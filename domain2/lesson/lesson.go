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
	ExampleNum                 int
	Problem                    string
	TeacherSolution            string
	StudentFriendlyExplanation string
	CommonMistake              string
}

type Section struct {
	SectionTitle      string
	TransitionIntro   string
	DetailedContent   string
	KeyTakeaway       string
	StudentClozeNotes []string
	TeacherExamples   []TeacherExample
}

type Lesson struct {
	title     string
	overview  string
	sections  []Section
	exercises []exercise.Exercise
}

func (l *Lesson) Title() string {
	return l.title
}
func (l *Lesson) Overview() string {
	return l.overview
}
func (l *Lesson) Sections() []Section {
	return l.sections
}
func (l *Lesson) Exercises() []exercise.Exercise {
	return l.exercises
}

func NewLesson(title string, overview string, sections []Section, exercises []exercise.Exercise) (*Lesson, error) {
	if len(title) == 0 {
		title = "Không có tiêu đề"
	}

	if len(exercises) == 0 {
		return nil, ErrErcerciseEmpty
	}

	return &Lesson{
		title:     title,
		overview:  overview,
		sections:  sections,
		exercises: exercises,
	}, nil
}
