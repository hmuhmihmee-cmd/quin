package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
		Title:             strings.TrimSpace(assignment.Title),
		AssignmentType:    string(assignment.Type),
		Status:            string(assignment.Status),
		AssignedAt:        assignment.AssignedAt,
		AssigneeID:        int64(assignment.Assignee.ID),
		TargetPageID:      strings.TrimSpace(assignment.TargetPageID),
		StudentPageWebUrl: strings.TrimSpace(assignment.StudentPageWebURL),
		TeacherPageWebUrl: strings.TrimSpace(assignment.TeacherPageWebURL),
		ItemsJson:         string(itemsJSON),
		OriginMistakeID:   nullableIntPointer(assignment.OriginMistakeID),
		Depth:             int64(assignment.Depth),
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
		ID:              int(row.ID),
		Title:           row.Title,
		Type:            domain.AssignmentType(row.AssignmentType),
		Status:          domain.AssignmentStatus(row.Status),
		AssignedAt:      row.AssignedAt,
		OriginMistakeID: intPointer(row.OriginMistakeID),
		Depth:           int(row.Depth),
		Assignee: domain.Student{
			ID:                 int(row.StudentID),
			Class:              row.StudentClass,
			Name:               row.StudentName,
			CycleStartDay:      int(row.StudentCycleStartDay),
			StudentWorkspaceID: stringPointer(row.StudentWorkspaceID),
			TeacherWorkspaceID: stringPointer(row.TeacherWorkspaceID),
		},
		TargetPageID:      row.TargetPageID,
		StudentPageWebURL: row.StudentPageWebUrl,
		TeacherPageWebURL: row.TeacherPageWebUrl,
		Items:             items,
	}, nil
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
			StudentPageWebURL: row.StudentPageWebUrl, TeacherPageWebURL: row.TeacherPageWebUrl,
			StudentID: int(row.StudentID), StudentName: row.StudentName,
			Status: domain.AssignmentStatus(row.Status), AssignedAt: row.AssignedAt,
			CorrectCount: correct, GradedCount: graded, TotalCount: len(items),
		})
	}
	return result, nil
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
