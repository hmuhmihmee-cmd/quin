package domain

import "time"

type Audience string

const (
	AudienceTeacher Audience = "teacher"
	AudienceStudent Audience = "student"
)

type TeacherExample struct {
	ExampleNum                 int    `json:"example_num"`
	Problem                    string `json:"problem"`
	TeacherSolution            string `json:"teacher_solution"`
	StudentFriendlyExplanation string `json:"student_friendly_explanation"`
	CommonMistake              string `json:"common_mistake,omitempty"`
}

type Section struct {
	SectionTitle      string           `json:"section_title"`
	TransitionIntro   string           `json:"transition_intro"`
	DetailedContent   string           `json:"detailed_content"`
	KeyTakeaway       string           `json:"key_takeaway"`
	StudentClozeNotes []string         `json:"student_cloze_notes"`
	TeacherExamples   []TeacherExample `json:"teacher_examples"`
}

type Exercise struct {
	ID          int      `json:"id"`
	Type        string   `json:"type"`
	Difficulty  string   `json:"difficulty"`
	Question    string   `json:"question"`
	Options     []string `json:"options,omitempty"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
}

type Lesson struct {
	Title     string     `json:"title"`
	Overview  string     `json:"overview"`
	Sections  []Section  `json:"sections"`
	Exercises []Exercise `json:"exercises"`
}

type Document struct {
	Title       string
	ContentType string
	Data        []byte
}

type DraftStatus string

const (
	DraftStatusPending    DraftStatus = "pending"
	DraftStatusProcessing DraftStatus = "processing"
	DraftStatusCompleted  DraftStatus = "completed"
	DraftStatusFailed     DraftStatus = "failed"
)

type LessonDraft struct {
	ID           int         `json:"id"`
	SourceURL    string      `json:"source_url"`
	CustomPrompt string      `json:"custom_prompt"`
	Status       DraftStatus `json:"status"`
	ErrorMessage string      `json:"error_message,omitempty"`
	Title        string      `json:"title"` // Tên bài giảng (AI sinh ra hoặc GV sửa)
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    *time.Time  `json:"updated_at"`
	Model        string      `json:"model"` // Ví dụ: "gemini-1.5-flash"
	LessonData   *Lesson     `json:"lesson_data,omitempty"`
}

type AIModelInfo struct {
	ID          string `json:"id"`           // Ví dụ: "gemini-1.5-flash", "gemini-2.0-flash"
	DisplayName string `json:"display_name"` // Ví dụ: "Gemini 1.5 Flash"
	Description string `json:"description"`  // Mô tả ngắn về model
}
