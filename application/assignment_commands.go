package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"meet-attendance-clean/domain"
)

type GradeAssignmentCommand struct {
	Model        string `json:"model"`
	CustomPrompt string `json:"custom_prompt"`
	PageID       string `json:"page_id"`
	ExerciseIDs  []int  `json:"exercise_ids,omitempty"`
}

type GenerateRemediationCommand struct {
	MistakeID           int    `json:"mistake_id"`
	MultipleChoiceCount int    `json:"multiple_choice_count"`
	EssayCount          int    `json:"essay_count"`
	Model               string `json:"model"`
	CustomPrompt        string `json:"custom_prompt"`
}

type MistakeContext struct {
	Mistake domain.Mistake
	Student domain.Student
}

type GradingItem struct {
	Exercise domain.Exercise      `json:"exercise"`
	Answer   domain.StudentAnswer `json:"answer"`
}

type GradingResultItem struct {
	ExerciseID int                  `json:"exercise_id"`
	Result     domain.GradingResult `json:"result"`
}

type AssignmentRepository interface {
	Save(ctx context.Context, assignment *domain.Assignment) error
	GetByPageID(ctx context.Context, pageID string) (*domain.Assignment, error)
}

type MistakeRepository interface {
	SaveMistakes(ctx context.Context, studentID int, assignmentID int, mistakes []domain.Mistake) error
	GetMistakeContext(ctx context.Context, mistakeID int) (*MistakeContext, error)
	MarkMistakeResolved(ctx context.Context, mistakeID int) error
}

type OneNoteGateway interface {
	PublishStudentLesson(ctx context.Context, target WorkspaceTarget, lesson *domain.Lesson) (string, error)
	FetchStudentAnswers(ctx context.Context, assignment *domain.Assignment) error
	PatchFeedback(ctx context.Context, assignment *domain.Assignment) error
}

type AIGradingService interface {
	EvaluateBatch(ctx context.Context, model string, customPrompt string, items []GradingItem) ([]GradingResultItem, error)
	GenerateRemediation(ctx context.Context, model string, customPrompt string, mistake domain.Mistake, multipleChoiceCount, essayCount int) (*domain.Lesson, error)
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
	cmd.PageID = strings.TrimSpace(cmd.PageID)
	if cmd.PageID == "" {
		return errors.New("page_id không được để trống")
	}
	for _, exerciseID := range cmd.ExerciseIDs {
		if exerciseID <= 0 {
			return fmt.Errorf("exercise_id không hợp lệ: %d", exerciseID)
		}
	}
	// 1. Lấy Assignment từ DB
	assignment, err := c.assignmentRepo.GetByPageID(ctx, cmd.PageID)
	if err != nil {
		return fmt.Errorf("không tìm thấy Assignment: %w", err)
	}

	// 2. Bóc tách bài làm từ OneNote (Text + Image bytes trong RAM)
	if err := c.onenote.FetchStudentAnswers(ctx, assignment); err != nil {
		return fmt.Errorf("không thể lấy bài làm OneNote: %w", err)
	}

	// CHỐNG MẤT BÀI LÀM: Lưu Checkpoint ngay vào DB trước khi gọi AI
	if err := c.assignmentRepo.Save(ctx, assignment); err != nil {
		return fmt.Errorf("lỗi lưu checkpoint bài làm: %w", err)
	}

	// 3. Phân loại câu cần chấm và câu bỏ trống
	isGradeAll := len(cmd.ExerciseIDs) == 0
	targetMap := make(map[int]bool, len(cmd.ExerciseIDs))
	for _, id := range cmd.ExerciseIDs {
		targetMap[id] = true
	}

	batchItems := make([]GradingItem, 0, len(assignment.Items))
	exerciseIndexMap := make(map[int]int, len(assignment.Items))

	for i := range assignment.Items {
		item := &assignment.Items[i]

		shouldGrade := false
		if isGradeAll {
			if item.Result == nil || (item.Result.Status == domain.AIExerciseSkippedEmpty && item.StudentAnswer != nil && !item.StudentAnswer.IsBlank) {
				shouldGrade = true
			}
		} else {
			if targetMap[item.Exercise.ID] {
				shouldGrade = true
			}
		}

		if !shouldGrade {
			continue
		}

		// Xử lý câu bỏ trống: Gán kết quả tại chỗ, không tốn token AI
		if item.StudentAnswer == nil || item.StudentAnswer.IsBlank {
			item.Result = &domain.GradingResult{
				Status:       domain.AIExerciseSkippedEmpty,
				IsCorrect:    false,
				FeedbackHTML: "<p>Học sinh bỏ trống câu này.</p>",
			}
			continue
		}

		batchItems = append(batchItems, GradingItem{
			Exercise: item.Exercise,
			Answer:   *item.StudentAnswer,
		})
		exerciseIndexMap[item.Exercise.ID] = i
	}

	// 4. GỌI AI CHẤM BATCH 1 LẦN DUY NHẤT
	var newMistakes []domain.Mistake

	if len(batchItems) > 0 {
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		results, err := c.ai.EvaluateBatch(callCtx, cmd.Model, cmd.CustomPrompt, batchItems)
		cancel()

		if err != nil {
			return fmt.Errorf("lỗi khi gọi AI chấm bài hàng loạt: %w", err)
		}

		// Map kết quả AI trả về vào Assignment
		for _, res := range results {
			idx, exists := exerciseIndexMap[res.ExerciseID]
			if !exists {
				continue
			}

			item := &assignment.Items[idx]
			gradingRes := res.Result
			gradingRes.Status = domain.AIExerciseGraded
			item.Result = &gradingRes

			// Thu thập lỗi sai cho Mistake Bank
			if !gradingRes.IsCorrect && gradingRes.DetectedMistake != nil {
				m := *gradingRes.DetectedMistake
				m.CreatedAt = time.Now().UTC()
				newMistakes = append(newMistakes, m)
			}
		}
	}

	// 5. Cập nhật trạng thái hoàn thành
	allGraded := true
	for _, item := range assignment.Items {
		if item.Result == nil {
			allGraded = false
			break
		}
	}
	if allGraded {
		assignment.Status = domain.AssignmentStatusGraded
	}

	// 6. Lưu kết quả chấm điểm vào DB
	if err := c.assignmentRepo.Save(ctx, assignment); err != nil {
		return fmt.Errorf("lỗi cập nhật kết quả vào DB: %w", err)
	}

	// 7. Lưu Mistake Bank
	if len(newMistakes) > 0 {
		if err := c.mistakeRepo.SaveMistakes(ctx, assignment.Assignee.ID, assignment.ID, newMistakes); err != nil {
			return fmt.Errorf("lỗi lưu ngân hàng lỗi sai: %w", err)
		}
	}

	// 8. Đẩy nhận xét lên OneNote (Infra đảm bảo chỉ PATCH vào thẻ Feedback riêng)
	if err := c.onenote.PatchFeedback(ctx, assignment); err != nil {
		return fmt.Errorf("chấm bài xong nhưng lỗi đẩy nhận xét OneNote: %w", err)
	}

	return nil
}

func (c *AssignmentCommand) GenerateRemediation(ctx context.Context, cmd GenerateRemediationCommand) error {
	if cmd.MistakeID <= 0 || cmd.MultipleChoiceCount < 0 || cmd.EssayCount < 0 || cmd.MultipleChoiceCount+cmd.EssayCount == 0 {
		return errors.New("số lượng câu hỏi khắc phục không hợp lệ")
	}

	contextData, err := c.mistakeRepo.GetMistakeContext(ctx, cmd.MistakeID)
	if err != nil {
		return fmt.Errorf("không tìm thấy lỗi sai: %w", err)
	}

	if contextData.Student.StudentWorkspaceID == nil || *contextData.Student.StudentWorkspaceID == "" {
		return fmt.Errorf("học sinh %s chưa được liên kết sổ OneNote", contextData.Student.Name)
	}

	lesson, err := c.ai.GenerateRemediation(ctx, cmd.Model, cmd.CustomPrompt, contextData.Mistake, cmd.MultipleChoiceCount, cmd.EssayCount)
	if err != nil {
		return fmt.Errorf("AI không thể tạo bài khắc phục: %w", err)
	}

	title := fmt.Sprintf("Bài khắc phục: %s", contextData.Mistake.Topic)
	chapterName := "Bài tập khắc phục"
	target := WorkspaceTarget{
		WorkspaceID: contextData.Student.StudentWorkspaceID,
		ChapterName: &chapterName,
		PageName:    &title,
	}

	pageID, err := c.onenote.PublishStudentLesson(ctx, target, lesson)
	if err != nil {
		return fmt.Errorf("lỗi đẩy bài khắc phục lên OneNote: %w", err)
	}

	items := make([]domain.AssignedExercise, 0, len(lesson.Exercises))
	for _, ex := range lesson.Exercises {
		items = append(items, domain.AssignedExercise{Exercise: ex})
	}

	assignment := &domain.Assignment{
		Title:        title,
		Type:         domain.AssignmentTypeRemediation,
		Status:       domain.AssignmentStatusPending,
		AssignedAt:   time.Now().UTC(),
		Assignee:     contextData.Student,
		TargetPageID: pageID,
		Items:        items,
	}

	if err := c.assignmentRepo.Save(ctx, assignment); err != nil {
		return fmt.Errorf("lỗi lưu bài khắc phục vào DB: %w", err)
	}

	return c.mistakeRepo.MarkMistakeResolved(ctx, cmd.MistakeID)
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
