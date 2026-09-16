package lesson

import (
	"errors"
	exercise "meet-attendance-clean/domain2/excercise"
)

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

type Lesson struct {
	Title     string              `json:"title"`
	Overview  string              `json:"overview"`
	Sections  []Section           `json:"sections"`
	Exercises []exercise.Exercise `json:"exercises"`
}

func NewLesson(title string, overview string, sections []Section, exercises []exercise.Exercise) (*Lesson, error) {
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
