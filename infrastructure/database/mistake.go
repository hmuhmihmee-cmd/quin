package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"meet-attendance-clean/application"
	"meet-attendance-clean/domain"
	"meet-attendance-clean/infrastructure/database/sqlcgen"
)

// MistakeStore tách repository Mistake khỏi SQLite dùng chung cho LessonDraft.
// Nhờ vậy hai repository có thể cùng có hàm GetByID mà không đụng method Go.
type MistakeStore struct {
	sqlite *SQLite
}

var _ application.MistakeRepository = (*MistakeStore)(nil)

func (s *SQLite) Mistakes() *MistakeStore {
	return &MistakeStore{sqlite: s}
}

// SaveMistakes lưu một batch lỗi mới trong cùng transaction, đồng thời bảo toàn
// toàn bộ quan hệ cha-con và trạng thái do domain xác định.
func (s *MistakeStore) SaveMistakes(ctx context.Context, studentID int, assignmentID int, mistakes []domain.Mistake) error {
	if len(mistakes) == 0 {
		return nil
	}

	tx, err := s.sqlite.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	queries := s.sqlite.queries.WithTx(tx)

	for i := range mistakes {
		createdAt := mistakes[i].CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		sourceAssignmentID := mistakes[i].SourceAssignmentID
		if sourceAssignmentID == 0 {
			sourceAssignmentID = assignmentID
		}
		id, err := queries.CreateMistake(ctx, sqlcgen.CreateMistakeParams{
			StudentID:          int64(studentID),
			SourceAssignmentID: int64(sourceAssignmentID),
			ParentMistakeID:    nullableIntPointer(mistakes[i].ParentMistakeID),
			Depth:              int64(mistakes[i].Depth),
			Topic:              strings.TrimSpace(mistakes[i].Topic),
			ErrorReason:        strings.TrimSpace(mistakes[i].ErrorReason),
			Status:             string(mistakes[i].Status),
			CreatedAt:          createdAt,
		})
		if err != nil {
			return fmt.Errorf("save mistake %d: %w", i+1, err)
		}
		mistakes[i].ID = int(id)
	}

	return tx.Commit()
}

func (s *MistakeStore) GetByID(ctx context.Context, id int) (*domain.Mistake, error) {
	row, err := s.sqlite.queries.GetMistakeByID(ctx, int64(id))
	if err != nil {
		return nil, err
	}
	mistake := mistakeFromRow(row.ID, row.SourceAssignmentID, row.ParentMistakeID, row.Depth, row.Topic, row.ErrorReason, row.Status, row.CreatedAt)
	return &mistake, nil
}

func (s *MistakeStore) SaveMistake(ctx context.Context, mistake domain.Mistake) error {
	if mistake.ID <= 0 {
		return fmt.Errorf("mistake id không hợp lệ: %d", mistake.ID)
	}
	updated, err := s.sqlite.queries.UpdateMistake(ctx, sqlcgen.UpdateMistakeParams{
		SourceAssignmentID: int64(mistake.SourceAssignmentID),
		ParentMistakeID:    nullableIntPointer(mistake.ParentMistakeID),
		Depth:              int64(mistake.Depth),
		Topic:              strings.TrimSpace(mistake.Topic),
		ErrorReason:        strings.TrimSpace(mistake.ErrorReason),
		Status:             string(mistake.Status),
		CreatedAt:          mistake.CreatedAt,
		ID:                 int64(mistake.ID),
	})
	if err != nil {
		return err
	}
	if updated == 0 {
		return fmt.Errorf("không tìm thấy lỗi sai %d", mistake.ID)
	}
	return nil
}

func (s *MistakeStore) GetMistakeContext(ctx context.Context, mistakeID int) (*application.MistakeContext, error) {
	row, err := s.sqlite.queries.GetMistakeWithStudent(ctx, int64(mistakeID))
	if err != nil {
		return nil, err
	}
	ancestryRows, err := s.sqlite.queries.ListMistakeAncestry(ctx, int64(mistakeID))
	if err != nil {
		return nil, err
	}
	ancestry := make([]domain.Mistake, 0, len(ancestryRows))
	for _, ancestor := range ancestryRows {
		ancestry = append(ancestry, mistakeFromRow(
			ancestor.ID, ancestor.SourceAssignmentID, ancestor.ParentMistakeID, ancestor.Depth,
			ancestor.Topic, ancestor.ErrorReason, ancestor.Status, ancestor.CreatedAt,
		))
	}

	return &application.MistakeContext{
		CurrentMistake: mistakeFromRow(row.ID, row.SourceAssignmentID, row.ParentMistakeID, row.Depth, row.Topic, row.ErrorReason, row.Status, row.CreatedAt),
		Student: domain.Student{
			ID:                 int(row.StudentID),
			Class:              row.StudentClass,
			Name:               row.StudentName,
			CycleStartDay:      int(row.StudentCycleStartDay),
			StudentWorkspaceID: stringPointer(row.StudentWorkspaceID),
			TeacherWorkspaceID: stringPointer(row.TeacherWorkspaceID),
		},
		AncestryChain: ancestry,
	}, nil
}

func (s *MistakeStore) MarkMistakeResolved(ctx context.Context, mistakeID int) error {
	mistake, err := s.GetByID(ctx, mistakeID)
	if err != nil {
		return err
	}
	mistake.Resolve()
	return s.SaveMistake(ctx, *mistake)
}

func (s *MistakeStore) ListMistakeGroups(ctx context.Context) ([]application.StudentMistakeGroup, error) {
	rows, err := s.sqlite.queries.ListMistakesWithStudents(ctx)
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
		groups[groupIndex].Mistakes = append(groups[groupIndex].Mistakes, mistakeFromRow(
			row.ID, row.SourceAssignmentID, row.ParentMistakeID, row.Depth,
			row.Topic, row.ErrorReason, row.Status, row.CreatedAt,
		))
	}
	return groups, nil
}

func (s *SQLite) ListMistakeGroups(ctx context.Context) ([]application.StudentMistakeGroup, error) {
	return s.Mistakes().ListMistakeGroups(ctx)
}

func mistakeFromRow(id, sourceAssignmentID int64, parentMistakeID sql.NullInt64, depth int64, topic, errorReason, status string, createdAt time.Time) domain.Mistake {
	return domain.Mistake{
		ID:                 int(id),
		SourceAssignmentID: int(sourceAssignmentID),
		ParentMistakeID:    intPointer(parentMistakeID),
		Depth:              int(depth),
		Topic:              topic,
		ErrorReason:        errorReason,
		Status:             domain.MistakeStatus(status),
		CreatedAt:          createdAt,
	}
}

func nullableIntPointer(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func intPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}
