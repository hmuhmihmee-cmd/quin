package lesson

import "errors"

type Audience string

const (
	AudienceTeacher Audience = "teacher"
	AudienceStudent Audience = "student"
)

var (
	ErrErcerciseEmpty = errors.New("exercises is empty")
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

type exercise struct {
	ID          int      `json:"id"`
	Type        string   `json:"type"`
	Topic       string   `json:"topic"`
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
	Exercises []exercise `json:"exercises"`
}

func NewLesson(title string, overview string, sections []Section, exercises []exercise) (*Lesson, error) {
	if len(title) == 0 {
		title = "Không có tiêu đề"
	}

	if len(exercises) == 0 {
		return nil, ErrErcerciseEmpty
	}

	return &Lesson{
		Title:     title,
		Overview:  overview,
		Sections:  sections,
		Exercises: exercises,
	}, nil
}
func NewExecise(id int, topic string, difficulty string, question string, options []string, answer string, explanation string) exercise {
	return exercise{
		ID:          id,
		Type:        "multiple-choice",
		Topic:       topic,
		Difficulty:  difficulty,
		Question:    question,
		Options:     options,
		Answer:      answer,
		Explanation: explanation,
	}
}
