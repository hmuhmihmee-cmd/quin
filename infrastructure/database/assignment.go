package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"meet-attendance-clean/application"
	"meet-attendance-clean/domain"
	"meet-attendance-clean/infrastructure/database/sqlcgen"
)

// ==================== HẠ TẦNG MỚI: AssignmentRepository ====================

// Save insert/update toàn bộ trạng thái Assignment theo TargetPageID. Items là
// aggregate con của Assignment nên được serialize thành JSON trong cùng một lần
// ghi; câu SQL thực tế vẫn được quản lý và sinh code hoàn toàn bởi sqlc.
func (s *SQLite) Save(ctx context.Context, assignment *domain.Assignment) error {
	if assignment == nil {
		return errors.New("assignment không được nil")
	}
	if strings.TrimSpace(assignment.TargetPageID) == "" {
		return errors.New("assignment thiếu target_page_id")
	}
	if assignment.Assignee.ID <= 0 {
		return errors.New("assignment thiếu học sinh")
	}

	itemsJSON, err := json.Marshal(assignment.Items)
	if err != nil {
		return fmt.Errorf("serialize assignment items: %w", err)
	}

	id, err := s.queries.SaveAssignment(ctx, sqlcgen.SaveAssignmentParams{
		Title:          strings.TrimSpace(assignment.Title),
		AssignmentType: string(assignment.Type),
		Status:         string(assignment.Status),
		AssignedAt:     assignment.AssignedAt,
		AssigneeID:     int64(assignment.Assignee.ID),
		TargetPageID:   strings.TrimSpace(assignment.TargetPageID),
		ItemsJson:      string(itemsJSON),
	})
	if err != nil {
		return fmt.Errorf("save assignment: %w", err)
	}
	assignment.ID = int(id)
	return nil
}

func (s *SQLite) GetByPageID(ctx context.Context, pageID string) (*domain.Assignment, error) {
	row, err := s.queries.GetAssignmentByPageID(ctx, strings.TrimSpace(pageID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("assignment page %q: %w", pageID, err)
		}
		return nil, err
	}

	var items []domain.AssignedExercise
	if err := json.Unmarshal([]byte(row.ItemsJson), &items); err != nil {
		return nil, fmt.Errorf("deserialize assignment %d items: %w", row.ID, err)
	}

	return &domain.Assignment{
		ID:         int(row.ID),
		Title:      row.Title,
		Type:       domain.AssignmentType(row.AssignmentType),
		Status:     domain.AssignmentStatus(row.Status),
		AssignedAt: row.AssignedAt,
		Assignee: domain.Student{
			ID:                 int(row.StudentID),
			Class:              row.StudentClass,
			Name:               row.StudentName,
			CycleStartDay:      int(row.StudentCycleStartDay),
			StudentWorkspaceID: stringPointer(row.StudentWorkspaceID),
			TeacherWorkspaceID: stringPointer(row.TeacherWorkspaceID),
		},
		TargetPageID: row.TargetPageID,
		Items:        items,
	}, nil
}

// ==================== HẠ TẦNG MỚI: MistakeRepository ====================

func (s *SQLite) SaveMistakes(ctx context.Context, studentID int, assignmentID int, mistakes []domain.Mistake) error {
	if len(mistakes) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)

	for i := range mistakes {
		createdAt := mistakes[i].CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		id, err := queries.CreateMistake(ctx, sqlcgen.CreateMistakeParams{
			StudentID:    int64(studentID),
			AssignmentID: int64(assignmentID),
			Topic:        strings.TrimSpace(mistakes[i].Topic),
			ErrorReason:  strings.TrimSpace(mistakes[i].ErrorReason),
			IsResolved:   boolToSQLite(mistakes[i].IsResolved),
			CreatedAt:    createdAt,
		})
		if err != nil {
			return fmt.Errorf("save mistake %d: %w", i+1, err)
		}
		mistakes[i].ID = int(id)
	}

	return tx.Commit()
}

func (s *SQLite) ListAssignmentSummaries(ctx context.Context) ([]application.AssignmentSummary, error) {
	rows, err := s.queries.ListAssignmentsForGrading(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]application.AssignmentSummary, 0, len(rows))
	for _, row := range rows {
		var items []domain.AssignedExercise
		if err := json.Unmarshal([]byte(row.ItemsJson), &items); err != nil {
			return nil, fmt.Errorf("deserialize assignment %d items: %w", row.ID, err)
		}
		graded, correct := 0, 0
		for _, item := range items {
			if item.Result != nil {
				graded++
				if item.Result.IsCorrect {
					correct++
				}
			}
		}
		result = append(result, application.AssignmentSummary{
			AssignmentID: int(row.ID), PageID: row.TargetPageID, Title: row.Title,
			StudentID: int(row.StudentID), StudentName: row.StudentName,
			Status: domain.AssignmentStatus(row.Status), AssignedAt: row.AssignedAt,
			CorrectCount: correct, GradedCount: graded, TotalCount: len(items),
		})
	}
	return result, nil
}

func (s *SQLite) ListMistakeGroups(ctx context.Context) ([]application.StudentMistakeGroup, error) {
	rows, err := s.queries.ListMistakesWithStudents(ctx)
	if err != nil {
		return nil, err
	}
	groups := make([]application.StudentMistakeGroup, 0)
	index := make(map[int]int)
	for _, row := range rows {
		studentID := int(row.StudentID)
		groupIndex, ok := index[studentID]
		if !ok {
			groupIndex = len(groups)
			index[studentID] = groupIndex
			groups = append(groups, application.StudentMistakeGroup{StudentID: studentID, StudentName: row.StudentName})
		}
		groups[groupIndex].Mistakes = append(groups[groupIndex].Mistakes, domain.Mistake{
			ID: int(row.ID), Topic: row.Topic, ErrorReason: row.ErrorReason,
			IsResolved: row.IsResolved != 0, CreatedAt: row.CreatedAt,
		})
	}
	return groups, nil
}

func (s *SQLite) ListStudentChoices(ctx context.Context) ([]application.StudentChoice, error) {
	rows, err := s.queries.ListStudentChoices(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]application.StudentChoice, 0, len(rows))
	for _, row := range rows {
		result = append(result, application.StudentChoice{
			ID:                 int(row.ID),
			Name:               row.Name,
			Class:              row.Class,
			StudentWorkspaceID: stringPointer(row.StudentWorkspaceID),
			TeacherWorkspaceID: stringPointer(row.TeacherWorkspaceID),
		})
	}
	return result, nil
}

func (s *SQLite) GetMistakeContext(ctx context.Context, mistakeID int) (*application.MistakeContext, error) {
	row, err := s.queries.GetMistakeContext(ctx, int64(mistakeID))
	if err != nil {
		return nil, err
	}
	return &application.MistakeContext{
		Mistake: domain.Mistake{ID: int(row.ID), Topic: row.Topic, ErrorReason: row.ErrorReason, IsResolved: row.IsResolved != 0, CreatedAt: row.CreatedAt},
		Student: domain.Student{
			ID:                 int(row.StudentID),
			Class:              row.StudentClass,
			Name:               row.StudentName,
			CycleStartDay:      int(row.StudentCycleStartDay),
			StudentWorkspaceID: stringPointer(row.StudentWorkspaceID),
			TeacherWorkspaceID: stringPointer(row.TeacherWorkspaceID),
		},
	}, nil
}

func (s *SQLite) MarkMistakeResolved(ctx context.Context, mistakeID int) error {
	affected, err := s.queries.MarkMistakeResolved(ctx, int64(mistakeID))
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("không tìm thấy lỗi sai %d", mistakeID)
	}
	return nil
}

func boolToSQLite(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
