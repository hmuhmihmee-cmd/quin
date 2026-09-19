package assignment

import (
	"errors"
	"fmt"
	exercise "meet-attendance-clean/domain2/excercise"
	"meet-attendance-clean/domain2/mistake"
	"uuid"
)

type Answer interface {
	ItemID() uuid.UUID
	IsAnswer()
}
type EvaluationRequest interface {
	ItemID() uuid.UUID
}
type DetectedMistake struct {
	topic  string
	reason string
}
type EvaluationResult interface {
	ItemID() uuid.UUID
	DetectedMistakes() []DetectedMistake
}
type AssignmentItem interface {
	ID() uuid.UUID
	OutputResult() string
	Comment() string
	Exercise() exercise.Exercise
	AddAnswer(raw Answer) error
	GenerateEvalRequest() EvaluationRequest
	ApplyEvalResult(res EvaluationResult) ([]mistake.Mistake, error)
	IsCorrect() bool
}

type TypedAssignmentItem[E exercise.Exercise, A Answer, Req EvaluationRequest, Res EvaluationResult] interface {
	ID() uuid.UUID
	OutputResult() string
	Comment() string
	Exercise() E
	AddAnswer(raw A)
	GenerateEvalRequest() Req
	ApplyEvalResult(res Res) []mistake.Mistake
	IsCorrect() bool
}
type wrapperAssignmentItem[E exercise.Exercise, A Answer, Req EvaluationRequest, Res EvaluationResult, T TypedAssignmentItem[E, A, Req, Res]] struct {
	inner T
}

func newWrapItem[E exercise.Exercise, A Answer, Req EvaluationRequest, Res EvaluationResult, T TypedAssignmentItem[E, A, Req, Res]](item T) *wrapperAssignmentItem[E, A, Req, Res, T] {
	return &wrapperAssignmentItem[E, A, Req, Res, T]{
		inner: item,
	}
}
func (w *wrapperAssignmentItem[E, A, Req, Res, T]) ID() uuid.UUID { return w.inner.ID() }
func (w *wrapperAssignmentItem[E, A, Req, Res, T]) OutputResult() string {
	return w.inner.OutputResult()
}
func (w *wrapperAssignmentItem[E, A, Req, Res, T]) Comment() string { return w.inner.Comment() }
func (w *wrapperAssignmentItem[E, A, Req, Res, T]) Exercise() exercise.Exercise {
	return w.inner.Exercise()
}
func (w *wrapperAssignmentItem[E, A, Req, Res, T]) AddAnswer(raw Answer) error {
	typed, ok := raw.(A)
	if !ok {
		return fmt.Errorf("item %s yêu cầu đáp án kiểu %T, nhận được %T", w.inner.ID(), *new(A), raw)
	}
	if typed.ItemID() != w.inner.ID() {
		return fmt.Errorf("đáp án thuộc về item khác: %s", typed.ItemID())
	}
	w.inner.AddAnswer(typed)
	return nil
}
func (w *wrapperAssignmentItem[E, A, Req, Res, T]) GenerateEvalRequest() EvaluationRequest {
	return w.inner.GenerateEvalRequest()
}
func (w *wrapperAssignmentItem[E, A, Req, Res, T]) ApplyEvalResult(res EvaluationResult) ([]mistake.Mistake, error) {
	typed, ok := res.(Res)
	if !ok {
		return nil, fmt.Errorf("item %s yêu cầu đáp án kiểu %T, nhận được %T", w.inner.ID(), *new(Res), res)
	}
	if typed.ItemID() != w.inner.ID() {
		return nil, fmt.Errorf("đáp án thuộc về item khác: %s", typed.ItemID())
	}
	return w.inner.ApplyEvalResult(typed), nil
}
func (w *wrapperAssignmentItem[E, A, Req, Res, T]) IsCorrect() bool {
	return w.inner.IsCorrect()
}

// Tạo một file factory.go hoặc để chung trong assignment.go
func NewAssignmentItem(ex exercise.Exercise) (AssignmentItem, error) {
	if ex == nil {
		return nil, errors.New("không thể tạo item từ exercise nil")
	}

	switch typedEx := ex.(type) {
	case exercise.EssayExercise:
		// Phải trả về pointer &EssayAssigmentItem
		var subItems []subItem
		for _, sub := range typedEx.Parts() {
			subItems = append(subItems, subItem{
				label:      sub.Label(),
				isSelected: false,
				isCorrect:  false,
				comment:    "",
			})
		}
		rawItem := &EssayAssigmentItem{
			id:       uuid.New(),
			exercise: typedEx,
			subItems: subItems,
			// Khởi tạo subItems dựa vào đề bài
		}
		return newWrapItem(rawItem), nil

	case exercise.MultipleChoiceExercise:
		// Phải trả về pointer &MCQAssignmentItem
		rawItem := &MCQAssignmentItem{
			id:        uuid.New(),
			exercise:  typedEx,
			isCorrect: false,
		}
		return newWrapItem(rawItem), nil

	default:
		return nil, fmt.Errorf("loại bài tập không hỗ trợ: %T", ex)
	}
}

var _ AssignmentItem = &wrapperAssignmentItem[exercise.Exercise, Answer, EvaluationRequest, EvaluationResult, TypedAssignmentItem[exercise.Exercise, Answer, EvaluationRequest, EvaluationResult]]{}
