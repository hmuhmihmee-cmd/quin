package domain

import "time"

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
	AIExerciseGraded        AIExerciseStatus = "graded"
	AIExerciseSkippedEmpty  AIExerciseStatus = "skipped_empty"
	AIExerciseAlreadyGraded AIExerciseStatus = "already_graded"
)



// StudentAnswer chứa bài làm thuần túy của học sinh
type StudentAnswer struct {
	Text        string    `json:"text,omitempty"` // Nội dung chữ học sinh gõ phím
	Images      [][]byte  `json:"-"`              // CHỈ LƯU TRONG RAM: Danh sách các ảnh vẽ tay/chụp bài làm
	IsBlank     bool      `json:"is_blank"`       // Đánh dấu nếu học sinh không làm bài
	ExtractedAt time.Time `json:"extracted_at"`
}


type Mistake struct {
	ID          int       `json:"id"`
	Topic       string    `json:"topic"`        // Dạng bài bị hỏng kiến thức
	ErrorReason string    `json:"error_reason"` // Nguyên nhân sai
	IsResolved  bool      `json:"is_resolved"`  // Đã làm bài khắc phục chưa
	CreatedAt   time.Time `json:"created_at"`
}

type GradingResult struct {
	Status          AIExerciseStatus `json:"status"`
	IsCorrect       bool             `json:"is_correct"`
	FeedbackHTML    string           `json:"feedback_html"`
	DetectedMistake *Mistake         `json:"detected_mistake,omitempty"`
}

type AssignedExercise struct {
	Exercise      Exercise       `json:"exercise"`
	StudentAnswer *StudentAnswer `json:"student_answer,omitempty"`
	Result        *GradingResult `json:"result,omitempty"`
}

type Assignment struct {
	ID           int                `json:"id"`
	Title        string             `json:"title"`
	Type         AssignmentType     `json:"type"`
	Status       AssignmentStatus   `json:"status"`
	AssignedAt   time.Time          `json:"assigned_at"`
	Assignee     Student            `json:"assignee"`
	TargetPageID string             `json:"target_page_id"`
	Items        []AssignedExercise `json:"items"`
}
