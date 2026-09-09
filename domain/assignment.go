package domain

import (
	"fmt"
	"strings"
	"time"
)

type AssignmentType string

const (
	AssignmentTypeNormal      AssignmentType = "normal"
	AssignmentTypeRemediation AssignmentType = "remediation"
)

type AssignmentStatus string

const (
	AssignmentStatusPending AssignmentStatus = "pending"
	AssignmentStatusGraded  AssignmentStatus = "graded"
)

type AIExerciseStatus string

const (
	AIExerciseGraded            AIExerciseStatus = "graded"
	AIExerciseSkippedEmpty      AIExerciseStatus = "skipped_empty"
	AIExerciseAwaitingSelection AIExerciseStatus = "awaiting_selection"
	AIExerciseAlreadyGraded     AIExerciseStatus = "already_graded"
)

type ChoiceSelectionState string

const (
	ChoiceNotApplicable ChoiceSelectionState = "not_applicable"
	ChoiceUnselected    ChoiceSelectionState = "unselected"
	ChoiceSelected      ChoiceSelectionState = "selected"
	ChoiceMultiple      ChoiceSelectionState = "multiple"
)

type StudentAnswer struct {
	Text            string               `json:"text,omitempty"`
	Images          [][]byte             `json:"-"`
	ChoiceSelection ChoiceSelectionState `json:"choice_selection"`
	SelectedOption  string               `json:"selected_option,omitempty"`
	ExtractedAt     time.Time            `json:"extracted_at"`
}

type GradingResult struct {
	Status          AIExerciseStatus `json:"status"`
	IsCorrect       bool             `json:"is_correct"`
	Score           float64          `json:"score"`
	FeedbackHTML    string           `json:"feedback_html"`
	DetectedMistake *Mistake         `json:"detected_mistake,omitempty"`
}

type AIEvaluationResult struct {
	ExerciseID      int
	IsBlank         bool
	IsCorrect       bool
	Score           float64
	Feedback        string
	DetectedMistake *Mistake
}

type AssignedExercise struct {
	Exercise      Exercise       `json:"exercise"`
	StudentAnswer *StudentAnswer `json:"student_answer,omitempty"`
	Result        *GradingResult `json:"result,omitempty"`
}

// IsMultipleChoice kiểm tra xem đây có phải câu hỏi trắc nghiệm không
func (ae *AssignedExercise) IsMultipleChoice() bool {
	return len(ae.Exercise.Options) > 0
}

// PrepareForGrading thẩm định trước khi gửi AI:
// Trả về true nếu câu này cần gửi Gemini để phân tích phần trình bày/tự luận.
func (ae *AssignedExercise) PrepareForGrading() bool {
	if ae.StudentAnswer == nil {
		ae.StudentAnswer = &StudentAnswer{ChoiceSelection: ChoiceUnselected}
	}

	// Xử lý bài trắc nghiệm chưa chọn hoặc chọn nhiều
	if ae.IsMultipleChoice() && ae.StudentAnswer.ChoiceSelection != ChoiceSelected {
		feedback := "<p>Chưa chọn đáp án.</p>"
		if ae.StudentAnswer.ChoiceSelection == ChoiceMultiple {
			feedback = "<p>Không hợp lệ: chỉ được chọn một đáp án.</p>"
		}

		ae.Result = &GradingResult{
			Status:       AIExerciseAwaitingSelection,
			IsCorrect:    false,
			FeedbackHTML: feedback,
		}
		return false // Không cần gửi AI
	}

	return true // Hợp lệ để gửi AI chấm
}

// ApplyEvaluation tiếp nhận kết quả từ AI và kết hợp với logic nội tại của Domain
// ApplyEvaluation áp dụng kết quả từ AI theo ma trận nghiệp vụ sư phạm
// ApplyEvaluation nhận thêm ngữ cảnh từ Assignment cha
func (ae *AssignedExercise) ApplyEvaluation(eval AIEvaluationResult, assignmentID int, parentMistakeID *int, depth int) *Mistake {
	res := &GradingResult{
		FeedbackHTML: eval.Feedback,
	}

	var createdMistake *Mistake
	defaultTopic := ae.Exercise.Topic // Lấy topic từ câu hỏi

	// =========================================================================
	// 1. CÂU HỎI TRẮC NGHIỆM
	// =========================================================================
	if ae.IsMultipleChoice() {
		res.Status = AIExerciseGraded

		expected := ae.expectedChoiceLetter()
		actual := strings.TrimSpace(strings.ToUpper(ae.StudentAnswer.SelectedOption))

		choiceCorrect := (actual != "" && actual == expected)
		hasWorking := !eval.IsBlank
		workingCorrect := eval.IsCorrect

		switch {
		// TRƯỜNG HỢP 1: Chọn ĐÚNG - KHÔNG trình bày
		case choiceCorrect && !hasWorking:
			res.IsCorrect = true
			res.FeedbackHTML = "<p>✅ <strong>Đúng</strong> <em>(Thiếu lời giải)</em></p>"

		// TRƯỜNG HỢP 2: Chọn ĐÚNG - Bài giải ĐÚNG
		case choiceCorrect && hasWorking && workingCorrect:
			res.IsCorrect = true
			res.FeedbackHTML = eval.Feedback

		// TRƯỜNG HỢP 3: Chọn ĐÚNG - Bài giải SAI
		case choiceCorrect && hasWorking && !workingCorrect:
			res.IsCorrect = true
			prefix := "<p>✅ <strong>Đúng</strong>, nhưng <strong>phần bài giải sai</strong>:</p>"
			res.FeedbackHTML = prefix + eval.Feedback
			createdMistake = extractMistake(eval.DetectedMistake, assignmentID, parentMistakeID, depth, defaultTopic)

		// TRƯỜNG HỢP 4: Chọn SAI - KHÔNG trình bày
		case !choiceCorrect && !hasWorking:
			res.IsCorrect = false
			res.FeedbackHTML = fmt.Sprintf("<p>❌ <strong>Sai</strong> (Đáp án đúng là %s). Chưa có bài giải.</p>", expected)
			createdMistake = extractMistake(eval.DetectedMistake, assignmentID, parentMistakeID, depth, defaultTopic)
			if createdMistake.ErrorReason == "" {
				createdMistake.ErrorReason = fmt.Sprintf("Chọn sai đáp án (%s thay vì %s) và không có lời giải", actual, expected)
			}

		// TRƯỜNG HỢP 5: Chọn SAI - Trình bày ĐÚNG (Tick nhầm)
		case !choiceCorrect && hasWorking && workingCorrect:
			res.IsCorrect = false
			prefix := fmt.Sprintf("<p>❌ <strong>Sai</strong> (Đáp án là %s). Tuy nhiên <strong>phần bài giải đúng</strong> (chọn nhầm).</p>", expected)
			res.FeedbackHTML = prefix + eval.Feedback
			createdMistake = nil // Không ghi nhận mistake

		// TRƯỜNG HỢP 6: Chọn SAI - Trình bày SAI
		case !choiceCorrect && hasWorking && !workingCorrect:
			res.IsCorrect = false
			res.FeedbackHTML = eval.Feedback
			createdMistake = extractMistake(eval.DetectedMistake, assignmentID, parentMistakeID, depth, defaultTopic)
		}

		// =========================================================================
		// 2. CÂU HỎI TỰ LUẬN
		// =========================================================================
	} else {
		if eval.IsBlank {
			res.Status = AIExerciseSkippedEmpty
			res.IsCorrect = false
			res.FeedbackHTML = "<p>Chưa làm.</p>"
		} else {
			res.Status = AIExerciseGraded
			res.IsCorrect = eval.IsCorrect
			res.FeedbackHTML = eval.Feedback

			if !eval.IsCorrect {
				createdMistake = extractMistake(eval.DetectedMistake, assignmentID, parentMistakeID, depth, defaultTopic)
			}
		}
	}

	if createdMistake != nil {
		res.DetectedMistake = createdMistake
	}

	ae.Result = res
	return createdMistake
}
func (ae *AssignedExercise) expectedChoiceLetter() string {
	ans := strings.TrimSpace(strings.ToUpper(ae.Exercise.Answer))
	if ans != "" && ans[0] >= 'A' && ans[0] <= 'Z' {
		return string(ans[0])
	}
	return ""
}

// ================= AGGREGATE ROOT: Assignment =================

type Assignment struct {
	ID                int                `json:"id"`
	Title             string             `json:"title"`
	Type              AssignmentType     `json:"type"`
	Status            AssignmentStatus   `json:"status"`
	AssignedAt        time.Time          `json:"assigned_at"`
	Assignee          Student            `json:"assignee"`
	TargetPageID      string             `json:"target_page_id"`
	StudentPageWebURL string             `json:"student_page_web_url"`
	TeacherPageWebURL string             `json:"teacher_page_web_url"`
	Items             []AssignedExercise `json:"items"`
	PageInkImage      []byte             `json:"page_ink_image"`
	OriginMistakeID   *int               `json:"origin_mistake_id"` // nil nếu là bài học chính, có ID nếu là bài chữa lỗi
	Depth             int                `json:"depth"`
}

// FilterAndPrepareGradingBatch chọn lọc các câu cần chấm theo scope và xử lý tiền thẩm định
func (a *Assignment) FilterAndPrepareGradingBatch(targetIDs []int) []*AssignedExercise {
	isGradeAll := len(targetIDs) == 0
	targetMap := make(map[int]bool, len(targetIDs))
	for _, id := range targetIDs {
		targetMap[id] = true
	}

	var toSend []*AssignedExercise
	for i := range a.Items {
		item := &a.Items[i]

		// Kiểm tra scope
		if !isGradeAll && !targetMap[item.Exercise.ID] {
			continue
		}

		// Cho entity tự thẩm định logic nghiệp vụ
		if item.PrepareForGrading() {
			toSend = append(toSend, item)
		}
	}
	return toSend
}

// ApplyAIEvaluations áp dụng kết quả từ AI, gom lỗi sai và tự động cập nhật trạng thái Assignment
func (a *Assignment) ApplyAIEvaluations(evaluations []AIEvaluationResult) []Mistake {
	evalMap := make(map[int]AIEvaluationResult, len(evaluations))
	for _, eval := range evaluations {
		evalMap[eval.ExerciseID] = eval
	}

	// Xác định độ sâu cho các lỗi mới sinh ra
	// Nếu bài này là bài khắc phục (đang ở Depth = 1), lỗi mới sinh ra sẽ ở Depth = 2
	newMistakeDepth := a.Depth
	if a.OriginMistakeID != nil {
		newMistakeDepth = a.Depth + 1
	}

	var mistakes []Mistake
	for i := range a.Items {
		item := &a.Items[i]
		if eval, exists := evalMap[item.Exercise.ID]; exists {
			// TRUYỀN PHẢ HỆ TỪ a VÀO:
			// a.ID -> SourceAssignmentID
			// a.OriginMistakeID -> ParentMistakeID (cha)
			// newMistakeDepth -> Depth
			if mistake := item.ApplyEvaluation(eval, a.ID, a.OriginMistakeID, newMistakeDepth); mistake != nil {
				mistakes = append(mistakes, *mistake)
			}
		}
	}

	a.recalculateStatus()
	return mistakes
}

func (a *Assignment) recalculateStatus() {
	for _, item := range a.Items {
		if item.Result == nil || item.Result.Status == AIExerciseAwaitingSelection {
			a.Status = AssignmentStatusPending
			return
		}
	}
	a.Status = AssignmentStatusGraded
}
func (a *Assignment) IsRemediationSuccess() bool {
	// Chỉ xét nếu đây đúng là bài tập khắc phục và đã chấm xong
	if a.Type != AssignmentTypeRemediation || a.Status != AssignmentStatusGraded {
		return false
	}

	// Tiêu chuẩn sư phạm: Phải làm đúng TOÀN BỘ các câu trong bài khắc phục thì mới coi là hết mất gốc
	for _, item := range a.Items {
		if item.Result == nil || !item.Result.IsCorrect {
			return false
		}
	}
	return true
}
func NewRemediationAssignment(
	student Student,
	originMistake Mistake, // Truyền cả struct Mistake thay vì chỉ string topic
	targetPageID string,
	exercises []Exercise,
) *Assignment {
	title := fmt.Sprintf("Bài tập khắc phục: %s", originMistake.Topic)

	items := make([]AssignedExercise, len(exercises))
	for i, ex := range exercises {
		items[i] = AssignedExercise{
			Exercise: ex,
		}
	}

	originID := originMistake.ID

	return &Assignment{
		Title:           title,
		Type:            AssignmentTypeRemediation,
		Status:          AssignmentStatusPending,
		AssignedAt:      time.Now().UTC(),
		Assignee:        student,
		TargetPageID:    targetPageID,
		Items:           items,
		OriginMistakeID: &originID,               // Gắn ID của lỗi cha vào đây
		Depth:           originMistake.Depth + 1, // Bài tập con sâu thêm 1 bậc
	}
}
