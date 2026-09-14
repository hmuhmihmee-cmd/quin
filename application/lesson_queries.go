package application

import (
	"context"
	"meet-attendance-clean/domain"
)

type LessonAIClient interface {
	ListModels(ctx context.Context) ([]AIModelInfo, error)
}
type LessonQuery struct {
	repo LessonDraftRepo
	ai   LessonAIClient // hoặc LessonAI
}

func NewLessonQuery(repo LessonDraftRepo, ai LessonAIClient) *LessonQuery {
	return &LessonQuery{repo: repo, ai: ai}
}

// Lấy danh sách toàn bộ các bài giảng đã tạo (để hiển thị bảng lịch sử & trạng thái)
func (q *LessonQuery) ListDrafts(ctx context.Context) ([]domain.LessonDraft, error) {
	return q.repo.List(ctx)
}

// Lấy chi tiết 1 bài giảng theo ID (để đổ vào khung soạn thảo hoặc xem lỗi)
func (q *LessonQuery) GetDraft(ctx context.Context, id int) (*domain.LessonDraft, error) {
	return q.repo.GetByID(ctx, id)
}
func (q *LessonQuery) ListModels(ctx context.Context) ([]AIModelInfo, error) {
	return q.ai.ListModels(ctx)
}

type AIModelInfo struct {
	ID          string `json:"id"`           // Ví dụ: "gemini-1.5-flash", "gemini-2.0-flash"
	DisplayName string `json:"display_name"` // Ví dụ: "Gemini 1.5 Flash"
	Description string `json:"description"`  // Mô tả ngắn về model
}
