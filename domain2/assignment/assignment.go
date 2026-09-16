package assignment

import (
	"fmt"
	exercise "meet-attendance-clean/domain2/excercise"
	"meet-attendance-clean/domain2/mistake"
)

type Answer[E exercise.Exercise] interface {
	IsValid(ex E) (bool, string)
}

type MCQAnswer struct {
	SelectedOption string
	WorkingText    string
	WorkingData    [][]byte
}

func (a MCQAnswer) IsValid(ex exercise.MultipleChoiceExercise) (bool, string) {
	if a.SelectedOption == "" {
		return false, "Chưa chọn đáp án trắc nghiệm"
	}
	for _, option := range ex.Options() {
		if option == a.SelectedOption {
			return true, ""
		}
	}
	return false, "Đáp án không hợp lệ"
}

type EssayAnswer struct {
	Text string
	Data [][]byte
}

func (a EssayAnswer) IsValid(ex exercise.EssayExercise) (bool, string) {
	return true, ""
}

var _ Answer[exercise.EssayExercise] = &EssayAnswer{}
var _ Answer[exercise.MultipleChoiceExercise] = &MCQAnswer{}

type ItemStatus string
type GradeItem[E exercise.Exercise] interface {
	Comment() string
}
type SubEssayGrade struct {
	Label     string
	IsCorrect bool
	Comment   string
}
type EssayGrade struct {
	SubItems []SubEssayGrade
}

func (e *EssayGrade) Comment() string {
	totalComment := ""
	for _, sub := range e.SubItems {
		totalComment += "\n" + sub.Comment
	}
	return totalComment
}

type MCQGrade struct {
	SelectedOption string
	comment        string
}

func (e *MCQGrade) Comment() string {
	return e.comment
}

type AssignmentItem[E exercise.Exercise, A Answer[E], G GradeItem[E]] interface {
	Status() ItemStatus
	OutputResult() string
	Comment() string
	ApplyGrade(GradeItem G) []mistake.Mistake
}
type subItem struct {
	label      string
	isSelected bool
	isCorrect  bool
	comment    string
}
type EssayAssigmentItem struct {
	exercise exercise.EssayExercise
	answer   *EssayAnswer
	subItems []subItem
}

func (e *EssayAssigmentItem) ListSelectedSubItem() []string {
	var selected []string
	for _, sub := range e.subItems {
		if sub.isSelected {
			selected = append(selected, sub.label)
		}
	}
	return selected
}
func (e *EssayAssigmentItem) ListSubItem() []string {
	var labels []string
	for _, sub := range e.subItems {
		labels = append(labels, sub.label)
	}
	return labels
}
func (e *EssayAssigmentItem) OuputResult() string {
	if e.answer == nil {
		return "Chưa làm bài tự luận"
	}
	isValid, rs := e.answer.IsValid(e.exercise)
	if !isValid {
		return rs
	}
	correct := 0
	selected := 0
	for _, sub := range e.subItems {
		if sub.isCorrect {
			correct++
		}
		if sub.isSelected {
			selected++
		}
	}
	return fmt.Sprintf("Đúng: %d/%d ý", correct, selected)
}
func (e *EssayAssigmentItem) ApplyGrade(grade EssayGrade) {
	for i := 0; i < len(e.subItems); i++ {
		sub := &e.subItems[i]
		if sub.isSelected {
			for _, subGrade := range grade.SubItems {
				if !sub.isSelected {
					continue
				}
				if subGrade.Label == sub.label {
					sub.comment = subGrade.Comment
					sub.isCorrect = subGrade.IsCorrect
				}
			}
		}
	}
}

type MCQAssignmentItem struct {
	exercise  exercise.MultipleChoiceExercise
	answer    *MCQAnswer
	comment   string
	isCorrect bool
}

func (e *MCQAssignmentItem) OuputResult() string {
	if e.answer == nil {
		return "Chưa chọn đáp án"
	}
	isValid, rs := e.answer.IsValid(e.exercise)
	if !isValid {
		return rs
	}
	if e.answer.SelectedOption == e.exercise.Answer() {
		return "Đúng"
	}
	return "Sai"
}
func (e *MCQAssignmentItem) ApplyGrade(grade MCQGrade) {
	if e.answer == nil {
		return
	}
	if e.answer.SelectedOption == "" {
		e.isCorrect = false
		e.comment = "Chưa chọn đáp án"
		return
	}
	if e.answer.SelectedOption != e.exercise.Answer() {
		e.comment = grade.comment
		e.isCorrect = false
		return
	}
	e.isCorrect = true
	e.comment = grade.comment
}
