package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"meet-attendance-clean/domain"
)

type GradingResultItem struct {
	ExerciseID int                  `json:"exercise_id"` // ID của câu hỏi tương ứng
	IsBlank    bool                 `json:"is_blank"`    // AI xác định học sinh có làm hay bỏ trống
	Result     domain.GradingResult `json:"result"`      // Chi tiết kết quả chấm (đúng/sai, điểm, nhận xét HTML, mistake)
}
type GradeAssignmentCommand struct {
	Model        string `json:"model"`
	CustomPrompt string `json:"custom_prompt"`
	PageID       string `json:"page_id"`
	ExerciseIDs  []int  `json:"exercise_ids,omitempty"`
}

type GradingItem struct {
	Exercise domain.Exercise      `json:"exercise"`
	Answer   domain.StudentAnswer `json:"answer"`
}

type AssignmentRepository interface {
	Save(ctx context.Context, assignment *domain.Assignment) error
	GetByPageID(ctx context.Context, pageID string) (*domain.Assignment, error)
}

type OneNoteGateway interface {
	PublishSession(ctx context.Context, target WorkspaceTarget, audience domain.Audience, lesson domain.Lesson) (PublishResult, error)
	FetchStudentAnswers(ctx context.Context, assignment *domain.Assignment) error
	PatchFeedback(ctx context.Context, assignment *domain.Assignment) error
}

// AIGradingService nhận thêm fullPageInkImage để AI có toàn bộ bức tranh viết tay của học sinh
type AIGradingService interface {
	EvaluateBatch(ctx context.Context, model string, customPrompt string, fullPageInkImage []byte, items []GradingItem) ([]domain.AIEvaluationResult, error)
	GenerateRemediation(ctx context.Context, model string, customPrompt string, ctxData MistakeContext, mcCount, essayCount int) (*domain.Lesson, error)
}

type AssignmentCommand struct {
	draftRepo      LessonDraftRepo
	assignmentRepo AssignmentRepository
	studentRepo    StudentRepository
	mistakeRepo    MistakeRepository
	onenote        OneNoteGateway
	ai             AIGradingService
}

func NewAssignmentCommand(
	draftRepo LessonDraftRepo,
	assignmentRepo AssignmentRepository,
	studentRepo StudentRepository,
	mistakeRepo MistakeRepository,
	onenote OneNoteGateway,
	ai AIGradingService,
) *AssignmentCommand {
	return &AssignmentCommand{
		draftRepo:      draftRepo,
		assignmentRepo: assignmentRepo,
		studentRepo:    studentRepo,
		mistakeRepo:    mistakeRepo,
		onenote:        onenote,
		ai:             ai,
	}
}
func (c *AssignmentCommand) Grade(ctx context.Context, cmd GradeAssignmentCommand) error {
	// 1. Validate Input
	if err := c.validateGradeCommand(cmd); err != nil {
		return err
	}

	// 2. Tải Aggregate từ Database
	assignment, err := c.assignmentRepo.GetByPageID(ctx, cmd.PageID)
	if err != nil {
		return fmt.Errorf("không tìm thấy Assignment: %w", err)
	}

	// 3. Đồng bộ bài làm học sinh từ OneNote
	if err := c.onenote.FetchStudentAnswers(ctx, assignment); err != nil {
		return fmt.Errorf("không thể lấy bài làm OneNote: %w", err)
	}
	err = c.assignmentRepo.Save(ctx, assignment) // Lưu checkpoint an toàn

	if err != nil {
		// log
	}

	// 4. Để Domain tự chọn lọc và tiền thẩm định các câu cần chấm
	eligibleExercises := assignment.FilterAndPrepareGradingBatch(cmd.ExerciseIDs)

	// 5. Gọi AI chấm batch nếu có câu hỏi đủ điều kiện
	if len(eligibleExercises) > 0 {
		aiItems := make([]GradingItem, len(eligibleExercises))
		for i, ex := range eligibleExercises {
			aiItems[i] = GradingItem{Exercise: ex.Exercise, Answer: *ex.StudentAnswer}
		}

		callCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		evaluations, err := c.ai.EvaluateBatch(callCtx, cmd.Model, cmd.CustomPrompt, assignment.PageInkImage, aiItems)
		cancel()

		if err != nil {
			return fmt.Errorf("lỗi khi gọi AI chấm bài: %w", err)
		}

		// 6. Aggregate áp dụng kết quả và tự động thu thập Mistakes
		newMistakes := assignment.ApplyAIEvaluations(evaluations)

		// Lưu Mistake Bank nếu có phát hiện lỗi
		if len(newMistakes) > 0 {
			err = c.mistakeRepo.SaveMistakes(ctx, assignment.Assignee.ID, assignment.ID, newMistakes)
			fmt.Printf("lỗi khi lưu mistaske vào db  :%v", err)
		}
	}
	if assignment.OriginMistakeID != nil {
		originMistake, err := c.mistakeRepo.GetByID(ctx, *assignment.OriginMistakeID)
		if err == nil && originMistake != nil {

			if assignment.IsRemediationSuccess() {
				originMistake.Resolve()
			}

			// Lưu lại struct Mistake
			_ = c.mistakeRepo.SaveMistake(ctx, *originMistake)
		}
	}

	// 7. Lưu toàn bộ trạng thái mới của Aggregate vào DB
	if err := c.assignmentRepo.Save(ctx, assignment); err != nil {
		return fmt.Errorf("lỗi cập nhật kết quả vào DB: %w", err)
	}

	// 8. Đẩy nhận xét trở lại OneNote
	if err := c.onenote.PatchFeedback(ctx, assignment); err != nil {
		return fmt.Errorf("lỗi đẩy nhận xét OneNote: %w", err)
	}

	return nil
}

func (c *AssignmentCommand) validateGradeCommand(cmd GradeAssignmentCommand) error {
	if strings.TrimSpace(cmd.PageID) == "" {
		return errors.New("page_id không được để trống")
	}
	for _, id := range cmd.ExerciseIDs {
		if id <= 0 {
			return fmt.Errorf("exercise_id không hợp lệ: %d", id)
		}
	}
	return nil
}
func (c *AssignmentCommand) PushFeedback(ctx context.Context, pageID string) error {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return errors.New("page_id không được để trống")
	}
	assignment, err := c.assignmentRepo.GetByPageID(ctx, pageID)
	if err != nil {
		return fmt.Errorf("không tìm thấy bài đã chấm: %w", err)
	}
	return c.onenote.PatchFeedback(ctx, assignment)
}
