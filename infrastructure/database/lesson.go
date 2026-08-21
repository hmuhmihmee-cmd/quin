package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"meet-attendance-clean/domain"
	"meet-attendance-clean/infrastructure/database/sqlcgen"
)

var vnZone = time.FixedZone("ICT", 7*3600)

func (s *SQLite) Create(ctx context.Context, draft *domain.LessonDraft) (int, error) {
	now := time.Now().In(vnZone)
	id, err := s.queries.CreateLessonDraft(ctx, sqlcgen.CreateLessonDraftParams{
		SourceUrl:    draft.SourceURL,
		CustomPrompt: draft.CustomPrompt,
		Status:       "processing",
		CreatedAt:    now,
		Model:        draft.Model,
	})
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func (s *SQLite) UpdateStatus(ctx context.Context, id int, status domain.DraftStatus, errorMsg string) error {
	return s.queries.UpdateLessonDraftStatus(ctx, sqlcgen.UpdateLessonDraftStatusParams{
		ID:           int64(id),
		Status:       string(status),
		ErrorMessage: errorMsg,
		UpdatedAt:    nullableTime(time.Now().In(vnZone)),
	})
}

func (s *SQLite) UpdateGeneratedContent(ctx context.Context, id int, title, teacherContent, studentContent string) error {
	return s.queries.UpdateLessonDraftGeneratedContent(ctx, sqlcgen.UpdateLessonDraftGeneratedContentParams{
		ID:             int64(id),
		Title:          title,
		TeacherContent: teacherContent,
		StudentContent: studentContent,
		UpdatedAt:      nullableTime(time.Now().In(vnZone)),
	})
}

func (s *SQLite) UpdateUserContent(ctx context.Context, id int, title, teacherContent, studentContent string) error {
	return s.queries.UpdateLessonDraftUserContent(ctx, sqlcgen.UpdateLessonDraftUserContentParams{
		ID:             int64(id),
		Title:          title,
		TeacherContent: teacherContent,
		StudentContent: studentContent,
		UpdatedAt:      nullableTime(time.Now().In(vnZone)),
	})
}

// ==================== HẠ TẦNG MỚI: LessonDraft có dữ liệu cấu trúc ====================

func (s *SQLite) UpdateDraftLesson(ctx context.Context, draft domain.LessonDraft) error {
	var lessonJSON sql.NullString
	if draft.LessonData != nil {
		data, err := json.Marshal(draft.LessonData)
		if err != nil {
			return fmt.Errorf("serialize lesson data: %w", err)
		}
		lessonJSON = sql.NullString{String: string(data), Valid: true}
	}

	updatedAt := time.Now().UTC()
	if draft.UpdatedAt != nil {
		updatedAt = *draft.UpdatedAt
	}
	return s.queries.UpdateLessonDraftLessonData(ctx, sqlcgen.UpdateLessonDraftLessonDataParams{
		Status:         string(draft.Status),
		Title:          draft.Title,
		SourceUrl:      draft.SourceURL,
		CustomPrompt:   draft.CustomPrompt,
		Model:          draft.Model,
		LessonDataJson: lessonJSON,
		ErrorMessage:   draft.ErrorMessage,
		UpdatedAt:      nullableTime(updatedAt),
		ID:             int64(draft.ID),
	})
}

func (s *SQLite) GetByID(ctx context.Context, id int) (*domain.LessonDraft, error) {
	row, err := s.queries.GetLessonDraft(ctx, int64(id))
	if err != nil {
		return nil, err
	}

	draft := &domain.LessonDraft{
		ID:           int(row.ID),
		SourceURL:    row.SourceUrl,
		CustomPrompt: row.CustomPrompt,
		Status:       domain.DraftStatus(row.Status),
		ErrorMessage: row.ErrorMessage,
		Title:        row.Title,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    nullableTime2(row.UpdatedAt),
		Model:        row.Model,
	}
	if row.LessonDataJson.Valid && row.LessonDataJson.String != "" {
		var lesson domain.Lesson
		if err := json.Unmarshal([]byte(row.LessonDataJson.String), &lesson); err != nil {
			return nil, fmt.Errorf("deserialize lesson draft %d: %w", id, err)
		}
		draft.LessonData = &lesson
	}
	return draft, nil
}

func (s *SQLite) List(ctx context.Context) ([]domain.LessonDraft, error) {
	rows, err := s.queries.ListLessonDrafts(ctx)
	if err != nil {
		return nil, err
	}

	drafts := make([]domain.LessonDraft, 0, len(rows))
	for _, r := range rows {
		draft := domain.LessonDraft{
			ID:           int(r.ID),
			SourceURL:    r.SourceUrl,
			CustomPrompt: r.CustomPrompt,
			Status:       domain.DraftStatus(r.Status),
			ErrorMessage: r.ErrorMessage,
			Title:        r.Title,
			CreatedAt:    r.CreatedAt,
			UpdatedAt:    nullableTime2(r.UpdatedAt),
			Model:        r.Model,
		}
		if r.LessonDataJson.Valid && r.LessonDataJson.String != "" {
			var lesson domain.Lesson
			if err := json.Unmarshal([]byte(r.LessonDataJson.String), &lesson); err != nil {
				return nil, fmt.Errorf("deserialize lesson draft %d: %w", r.ID, err)
			}
			draft.LessonData = &lesson
		}
		drafts = append(drafts, draft)
	}
	return drafts, nil
}
func nullableTime2(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func (s *SQLite) Delete(ctx context.Context, id int) error {
	return s.queries.DeleteLessonDraft(ctx, int64(id))
}
