package assignment

import (
	"errors"
	exercise "meet-attendance-clean/domain2/excercise"
	"meet-attendance-clean/domain2/mistake"
	"uuid"
)

type MCQAnswer struct {
	itemID         uuid.UUID
	selectedOption string
	workingText    string
	workingData    [][]byte
}

func (a MCQAnswer) ItemID() uuid.UUID { return a.itemID }
func (a MCQAnswer) IsAnswer()         {}

func (a MCQAnswer) IsValid(ex exercise.MultipleChoiceExercise) (bool, string) {
	if a.selectedOption == "" {
		return false, "Chưa chọn đáp án trắc nghiệm"
	}
	for _, option := range ex.Options() {
		if option == a.selectedOption {
			return true, ""
		}
	}
	return false, "Đáp án không hợp lệ"
}

type MCQEvalReq struct {
	itemID         uuid.UUID
	selectedOption string
	text           string
	data           [][]byte
}

func (e MCQEvalReq) ItemID() uuid.UUID { return e.itemID }

type MCQEvalRes struct {
	itemID  uuid.UUID
	comment string
}

func NewMCQEvalRes(itemID uuid.UUID, comment string) MCQEvalRes {
	return MCQEvalRes{
		itemID:  itemID,
		comment: comment,
	}
}
func (e MCQEvalRes) ItemID() uuid.UUID { return e.itemID }

type MCQAssignmentItem struct {
	id        uuid.UUID
	exercise  exercise.MultipleChoiceExercise
	answer    *MCQAnswer
	comment   string
	isCorrect bool
}

func (e *MCQAssignmentItem) OutputResult() string {
	if e.answer == nil {
		return "Chưa chọn đáp án"
	}
	isValid, rs := e.answer.IsValid(e.exercise)
	if !isValid {
		return rs
	}
	if e.answer.selectedOption == e.exercise.Answer() {
		return "Đúng"
	}
	return "Sai"
}
func (e *MCQAssignmentItem) ApplyEvalResult(res MCQEvalRes) []mistake.Mistake {
	if e.answer == nil {
		return nil
	}
	if e.answer.selectedOption == "" {
		e.isCorrect = false
		e.comment = "Chưa chọn đáp án"
		return nil
	}
	if e.answer.selectedOption != e.exercise.Answer() {
		e.comment = res.comment
		e.isCorrect = false
		return nil
	}
	e.isCorrect = true
	e.comment = res.comment
	return nil
}

func (e *MCQAssignmentItem) GenerateEvalRequest() MCQEvalReq {
	var selected string
	var workText string
	var workData [][]byte

	if e.answer != nil {
		selected = e.answer.selectedOption
		workText = e.answer.workingText
		workData = e.answer.workingData
	}

	return MCQEvalReq{
		itemID:         e.id,
		selectedOption: selected,
		text:           workText,
		data:           workData,
	}
}
func (e *MCQAssignmentItem) AddAnswer(anwser MCQAnswer) {
	e.answer = &anwser
}
func (e *MCQAssignmentItem) Exercise() exercise.MultipleChoiceExercise {
	return e.exercise
}
func (e *MCQAssignmentItem) ID() uuid.UUID { return e.id }
func (e *MCQAssignmentItem) Comment() string {
	return e.comment
}
func (e *MCQAssignmentItem) IsCorrect() bool {
	return e.isCorrect
}
func NewMCQAnswer(itemID uuid.UUID, selectedOption string, text string, data [][]byte) (*MCQAnswer, error) {
	if itemID == uuid.Nil() {
		return nil, errors.New("itemID không hợp lệ")
	}
	// Chú ý: Trả về Value (hoặc Pointer trỏ tới Value), nhưng bản thân nó là bất biến
	return &MCQAnswer{
		itemID:         itemID,
		selectedOption: selectedOption,
		workingText:    text,
		workingData:    data,
	}, nil
}

var _ TypedAssignmentItem[exercise.MultipleChoiceExercise, MCQAnswer, MCQEvalReq, MCQEvalRes] = &MCQAssignmentItem{}
