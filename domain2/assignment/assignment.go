package assignment

import (
	"errors"
	"meet-attendance-clean/domain2/lesson"
	"meet-attendance-clean/domain2/mistake"
	"uuid"
)

type AssignmentType string

const (
	AssignmentTypeNormal      AssignmentType = "Normal"
	AssignmentTypeRemediation AssignmentType = "Remediation"
)

type AssignmentStatus string

const (
	AssignmentStatusPending   AssignmentStatus = "Pending"
	AssignmentStatusGraded    AssignmentStatus = "Graded"
	AssignmentStatusCompleted AssignmentStatus = "Completed"
)

type Assignment struct {
	id        uuid.UUID
	studentID uuid.UUID
	items     []AssignmentItem
	title     string
	typ       AssignmentType
	status    AssignmentStatus
}

func NewAssignment(studentID uuid.UUID, title string, typ AssignmentType, items ...AssignmentItem) (*Assignment, error) {
	if studentID == uuid.Nil() {
		return nil, errors.New("student ID không được định nghĩa")
	}
	if title == "" {
		return nil, errors.New("tiêu đề không được định nghĩa")
	}
	if len(items) == 0 {
		return nil, errors.New("không có item nào được định nghĩa")
	}
	return &Assignment{
		id:        uuid.New(),
		studentID: studentID,
		title:     title,
		typ:       typ,
		items:     items,
	}, nil
}
func (a *Assignment) ID() uuid.UUID { return a.id }
func (a *Assignment) StudentID() uuid.UUID {
	return a.studentID
}
func (a *Assignment) Items() []AssignmentItem {
	return a.items
}
func (a *Assignment) Title() string {
	return a.title
}
func (a *Assignment) Type() AssignmentType {
	return a.typ
}
func (a *Assignment) Status() AssignmentStatus {
	return a.status
}
func (a *Assignment) AddAnswers(anwsers []Answer) error {
	mapAnsers := make(map[uuid.UUID]Answer)
	for _, ans := range anwsers {
		mapAnsers[ans.ItemID()] = ans
	}
	for i := 0; i < len(a.items); i++ {
		item := a.items[i]
		if _, ok := mapAnsers[item.ID()]; ok {
			if err := item.AddAnswer(mapAnsers[item.ID()]); err != nil {
				return err
			}
		}
	}
	return nil
}
func (a *Assignment) ApplyEvalResults(evalResults []EvaluationResult) ([]mistake.Mistake, error) {
	mapEvalRes := make(map[uuid.UUID]EvaluationResult)
	for _, evalRes := range evalResults {
		mapEvalRes[evalRes.ItemID()] = evalRes
	}
	var mistakes []mistake.Mistake
	for i := 0; i < len(a.items); i++ {
		item := a.items[i]
		if _, ok := mapEvalRes[item.ID()]; ok {
			newMistakes, err := item.ApplyEvalResult(mapEvalRes[item.ID()])
			if err != nil {
				return nil, err
			}
			mistakes = append(mistakes, newMistakes...)
		}
	}
	a.status = AssignmentStatusGraded

	return mistakes, nil
}
func (a *Assignment) ListEvalReqs(itemIDs []uuid.UUID) []EvaluationRequest {
	mapID := make(map[uuid.UUID]bool)
	for _, id := range itemIDs {
		mapID[id] = true
	}
	var evalReqs []EvaluationRequest
	for i := 0; i < len(a.items); i++ {
		item := a.items[i]
		if _, ok := mapID[item.ID()]; ok {
			evalReqs = append(evalReqs, item.GenerateEvalRequest())
		}
	}
	return evalReqs
}
func (a *Assignment) IsCorrect() bool {
	for _, item := range a.items {
		if !item.IsCorrect() {
			return false
		}
	}
	return true
}

// NewAssignmentFromLesson là Factory tự động rã Lesson thành các AssignmentItem
func NewAssignmentFromLesson(
	studentID uuid.UUID,
	title string,
	lsn lesson.Lesson,
) (*Assignment, error) {

	// Tự động map từ exercise.Exercise sang AssignmentItem wrapper
	items := make([]AssignmentItem, 0, len(lsn.Exercises()))
	for _, ex := range lsn.Exercises() {
		item, err := NewAssignmentItem(ex)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return NewAssignment(studentID, title, AssignmentTypeNormal, items...)
}
