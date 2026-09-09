package application

import (
	"context"
	"errors"
	"fmt"
	"log"
	"meet-attendance-clean/domain"
	"net/url"
	"strings"
	"time"
)

// Interface tương tác Database SQLite cho LessonDraft
type LessonDraftRepo interface {
	Create(ctx context.Context, draft *domain.LessonDraft) (int, error)
	UpdateStatus(ctx context.Context, id int, status domain.DraftStatus, errorMsg string) error
	UpdateDraftLesson(ctx context.Context, draft domain.LessonDraft) error
	GetByID(ctx context.Context, id int) (*domain.LessonDraft, error)
	List(ctx context.Context) ([]domain.LessonDraft, error)
	Delete(ctx context.Context, id int) error
}

type LessonAI interface {
	GenerateLesson(ctx context.Context, modelName, sourceURL, prompt string) (*domain.Lesson, error)
}

type DocumentFormatter interface {
	Format(lesson *domain.Lesson, target domain.Audience, sourceURL string) string
}

type DocumentRenderer interface {
	Render(title, subtitle, content string) (domain.Document, error)
}
type LessonCommand struct {
	repo      LessonDraftRepo
	ai        LessonAI
	formatter DocumentFormatter
	renderer  DocumentRenderer
}

func NewLessonCommand(
	repo LessonDraftRepo,
	ai LessonAI,
	formatter DocumentFormatter,
	renderer DocumentRenderer,
) *LessonCommand {
	return &LessonCommand{
		repo:      repo,
		ai:        ai,
		formatter: formatter,
		renderer:  renderer,
	}
}

func (c *LessonCommand) CreateDraftJob(ctx context.Context, modelName, sourceURL, customPrompt string) (int, error) {
	sourceURL = strings.TrimSpace(sourceURL)
	parsed, err := url.ParseRequestURI(sourceURL)
	if err != nil || parsed.Host == "" {
		return 0, errors.New("đường dẫn video không hợp lệ")
	}
	if modelName == "" {
		modelName = "gemini-3.5-flash" // Mặc định nếu để trống
	}

	draft := &domain.LessonDraft{
		SourceURL:    sourceURL,
		CustomPrompt: strings.TrimSpace(customPrompt),
		Status:       domain.DraftStatusProcessing,
		CreatedAt:    time.Now(),
		// UpdatedAt:    time.Now(),
		Model: modelName,
	}

	draftID, err := c.repo.Create(ctx, draft)
	if err != nil {
		return 0, err
	}

	go c.processAIJob(draftID, modelName, sourceURL, customPrompt)

	return draftID, nil
}
func (c *LessonCommand) processAIJob(draftID int, modelName, sourceURL, customPrompt string) {

	aiCtx, aiCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer aiCancel()

	lesson, err := c.ai.GenerateLesson(aiCtx, modelName, sourceURL, customPrompt)
	if err != nil {
		log.Printf("[Draft #%d] Lỗi gọi AI: %v", draftID, err)
		c.markJobFailed(draftID, "Lỗi phân tích video từ AI: "+err.Error())
		return
	}

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()
	now := time.Now().UTC()
	err = c.repo.UpdateDraftLesson(dbCtx, domain.LessonDraft{
		ID:           draftID,
		SourceURL:    sourceURL,
		CustomPrompt: customPrompt,
		Status:       domain.DraftStatusCompleted,
		ErrorMessage: "",
		Title:        lesson.Title,
		UpdatedAt:    &now,
		Model:        modelName,
		LessonData:   lesson,
	})
	if err != nil {
		log.Printf("[Draft #%d] Lỗi lưu kết quả vào DB: %v", draftID, err)
		c.markJobFailed(draftID, "Lỗi không thể lưu nội dung vào cơ sở dữ liệu")
		return
	}

	log.Printf("[Draft #%d] Sinh giáo án thành công: %s", draftID, lesson.Title)
}

func (c *LessonCommand) markJobFailed(draftID int, errorReason string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.repo.UpdateStatus(ctx, draftID, domain.DraftStatusFailed, errorReason); err != nil {
		log.Printf("[Draft #%d] LỖI NGHIÊM TRỌNG: Không thể cập nhật trạng thái FAILED vào SQLite: %v (Lỗi gốc: %s)", draftID, err, errorReason)
	}
}

func (c *LessonCommand) SaveDraftContent(ctx context.Context, draft domain.LessonDraft) error {
	if strings.TrimSpace(draft.Title) == "" {
		return errors.New("tiêu đề không được để trống")
	}
	return c.repo.UpdateDraftLesson(ctx, draft)
}

// XUẤT PDF: Lôi LessonData có cấu trúc ra rồi Format thành Document
func (c *LessonCommand) ExportDocuments(ctx context.Context, draftID int) (domain.Document, domain.Document, error) {
	draft, err := c.repo.GetByID(ctx, draftID)
	if err != nil {
		return domain.Document{}, domain.Document{}, fmt.Errorf("không tìm thấy bản nháp: %w", err)
	}
	if draft.LessonData == nil {
		return domain.Document{}, domain.Document{}, errors.New("bản nháp chưa có dữ liệu bài giảng hoàn chỉnh")
	}

	// 1. Format ra Markdown theo từng vai trò
	teacherContent := c.formatter.Format(draft.LessonData, domain.AudienceTeacher, draft.SourceURL)
	studentContent := c.formatter.Format(draft.LessonData, domain.AudienceStudent, "")

	// 2. Render thành PDF Document
	teacherDoc, err := c.renderer.Render(draft.Title, "GIÁO ÁN GIÁO VIÊN", teacherContent)
	if err != nil {
		return domain.Document{}, domain.Document{}, fmt.Errorf("lỗi render PDF giáo viên: %w", err)
	}

	studentDoc, err := c.renderer.Render(draft.Title, "PHIẾU HỌC TẬP HỌC SINH", studentContent)
	if err != nil {
		return domain.Document{}, domain.Document{}, fmt.Errorf("lỗi render PDF học sinh: %w", err)
	}

	return teacherDoc, studentDoc, nil
}
