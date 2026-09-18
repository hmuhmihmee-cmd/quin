package assignment

import (
	"errors"
	"fmt"
	exercise "meet-attendance-clean/domain2/excercise"
	"meet-attendance-clean/domain2/mistake"
	"strings"
	"uuid"
)

type EssayAnswer struct {
	itemID      uuid.UUID
	text        string
	data        [][]byte
	targetParts []string
}

func (a EssayAnswer) IsValid(ex exercise.EssayExercise) (bool, string) {
	return true, ""
}
func (a EssayAnswer) ItemID() uuid.UUID { return a.itemID }
func (a EssayAnswer) IsAnswer()         {}
func (a EssayAnswer) TargetParts() []string {
	return a.targetParts
}

// var _ Answer[exercise.EssayExercise] = &EssayAnswer{}

type SubEssayReq struct {
	label string
}
type EssayEvalReq struct {
	itemID   uuid.UUID
	subItems []SubEssayReq
	text     string
	data     [][]byte
}

func (e EssayEvalReq) ItemID() uuid.UUID { return e.itemID }

type SubEssayRes struct {
	label     string
	isCorrect bool
	comment   string
}
type EssayEvalRes struct {
	itemID   uuid.UUID
	subItems []SubEssayRes
}

func NewSubEssayRes(label string, isCorrect bool, comment string) SubEssayRes {
	return SubEssayRes{
		label:     label,
		isCorrect: isCorrect,
		comment:   comment,
	}
}
func NewEssayEvalRes(itemID uuid.UUID, subItems []SubEssayRes) EssayEvalRes {
	return EssayEvalRes{
		itemID:   itemID,
		subItems: subItems,
	}
}

func (e EssayEvalRes) ItemID() uuid.UUID { return e.itemID }

func (e EssayEvalRes) Comment() string {
	totalComment := ""
	for _, sub := range e.subItems {
		totalComment += "\n" + sub.comment
	}
	return totalComment
}

type subItem struct {
	label      string
	isSelected bool
	isCorrect  bool
	comment    string
}
type EssayAssigmentItem struct {
	id       uuid.UUID
	exercise exercise.EssayExercise
	answer   *EssayAnswer
	subItems []subItem
}

func newEssayAssignmentItem(ex exercise.EssayExercise) EssayAssigmentItem {
	return EssayAssigmentItem{
		id:       uuid.New(),
		exercise: ex,
		// subItems: make([]subItem, len(ex.SubItems)),
	}
}
func (e *EssayAssigmentItem) Exercise() exercise.EssayExercise {
	return e.exercise
}
func (e *EssayAssigmentItem) ID() uuid.UUID { return e.id }

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
func (e *EssayAssigmentItem) IsCorrect() bool {
	isCorrect := true
	for _, sub := range e.subItems {
		if sub.isSelected && !sub.isCorrect {
			isCorrect = false
		}
	}
	return isCorrect
}
func (e *EssayAssigmentItem) OutputResult() string {
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
func (e *EssayAssigmentItem) ApplyEvalResult(res EssayEvalRes) []mistake.Mistake {
	for i := 0; i < len(e.subItems); i++ {
		sub := &e.subItems[i]
		if sub.isSelected {
			for _, subGrade := range res.subItems {
				if !sub.isSelected {
					continue
				}
				if subGrade.label == sub.label {
					sub.comment = subGrade.comment
					sub.isCorrect = subGrade.isCorrect
				}
			}
		}
	}
	return nil
}
func (e *EssayAssigmentItem) GenerateEvalRequest() EssayEvalReq {
	var subItems []SubEssayReq
	for i := 0; i < len(e.subItems); i++ {
		sub := &e.subItems[i]

		if sub.isSelected {
			subItems = append(subItems, SubEssayReq{
				label: sub.label,
			})
		}
	}
	var studentText string
	var studentData [][]byte
	if e.answer != nil {
		studentText = e.answer.text
		studentData = e.answer.data
	}
	return EssayEvalReq{
		itemID:   e.id,
		subItems: subItems,
		text:     studentText,
		data:     studentData,
	}
}
func (e *EssayAssigmentItem) AddAnswer(anwser EssayAnswer) {
	mapLabel := make(map[string]bool)
	for _, label := range anwser.TargetParts() {
		mapLabel[strings.ToLower(label)] = true
	}
	for i := 0; i < len(e.subItems); i++ {
		cleanLabel := strings.ToLower(strings.TrimSpace(e.subItems[i].label))
		e.subItems[i].isSelected = mapLabel[cleanLabel]
	}
	e.answer = &anwser
}
func (e *EssayAssigmentItem) Comment() string {
	return ""
}

var _ TypedAssignmentItem[exercise.EssayExercise, EssayAnswer, EssayEvalReq, EssayEvalRes] = &EssayAssigmentItem{}

func NewEssayAnswer(itemID uuid.UUID, text string, data [][]byte, targetParts []string) (*EssayAnswer, error) {
	// if text == "" && len(data) == 0 {
	// 	return nil, errors.New("bài làm tự luận không được để trống")
	// }
	for i := 0; i < len(targetParts); i++ {
		targetParts[i] = strings.TrimSpace(targetParts[i])
		if targetParts[i] == "" {
			return nil, errors.New("phần đề cập (parts) cần có tên")
		}
	}
	return &EssayAnswer{
		itemID:      itemID,
		text:        text,
		data:        data,
		targetParts: targetParts,
	}, nil
}
