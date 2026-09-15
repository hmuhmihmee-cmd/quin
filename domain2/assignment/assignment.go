package assignment

import (
	"meet-attendance-clean/domain2/lesson"
	"time"
	"uuid"
)

type AssigmentType string

const (
	AssigmentTypeNormal       AssigmentType = "Normal"
	AssignmentTypeRemediation AssigmentType = "Remediation"
)

type ItemStatus string

const (
	ItemStatusPending ItemStatus = "Pending" // Chưa chấm
	ItemStatusSkipped ItemStatus = "Skipped" // Bỏ trống hoàn toàn
	ItemStatusInvalid ItemStatus = "Invalid" // Khoanh sai quy cách
	ItemStatusGraded  ItemStatus = "Graded"  // Đã có điểm & nhận xét
)

type Answer interface {
	ItemID() uuid.UUID
	IsEmpty() bool
	IsValid() (bool, string)
	Text() string
	Data() [][]byte
	Type() lesson.ExerciseType
}
type baseAnswer struct {
	itemID uuid.UUID
	text   string
	data   [][]byte
}

func (a *baseAnswer) ItemID() uuid.UUID {
	return a.itemID
}

type MCAnswer struct {
	baseAnswer
	selectedOptions []string
}

func (a *MCAnswer) IsValid() (bool, string) {
	if len(a.selectedOptions) == 1 {
		return true, ""
	} else {
		return false, "Chỉ được chọn 1 đáp án"
	}
}
func (a *MCAnswer) Type() lesson.ExerciseType {
	return lesson.ExerciseTypeMultipleChoice
}

type ESSAnswer struct {
	baseAnswer
}

func (a *ESSAnswer) IsValid() (bool, string) {
	return a.baseAnswer.IsValid(), ""
}
func (a *ESSAnswer) Type() lesson.ExerciseType {
	return lesson.ExerciseTypeEssay
}
func (a *baseAnswer) IsEmpty() bool {
	return a.text == "" && len(a.data) == 0
}
func (a *baseAnswer) Text() string {
	return a.text
}
func (a *baseAnswer) Data() [][]byte {
	return a.data
}

func (a *baseAnswer) IsValid() bool {
	return a.text != "" || len(a.data) > 0
}
func NewMCAnswer(text string, data [][]byte, selectedOptions []string) *MCAnswer {
	return &MCAnswer{
		text:            text,
		data:            data,
		selectedOptions: selectedOptions,
	}
}
func NewESSAnswer(text string, data [][]byte) *ESSAnswer {
	return &ESSAnswer{
		text: text,
		data: data,
	}
}

type ExerciseSnapshot struct {
}
type AssigmentItem struct {
	id              uuid.UUID
	exercise        lesson.Exercise
	answer          Answer
	numSubCorrect   int
	numSubCompleted int
	status          ItemStatus
	comment         string
}
type Assignment struct {
	id         uuid.UUID
	studentID  uuid.UUID
	title      string
	typ        AssigmentType
	assignedAt time.Time
	items      []AssigmentItem
	status     ItemStatus
}
type GradeItem struct {
	itemID          uuid.UUID
	exercise        lesson.Exercise
	answer          Answer
	numSubCorrect   int
	numSubCompleted int
	comment         string
	status          ItemStatus
}

func NewAssignment(id uuid.UUID, studentID uuid.UUID, title string, typ AssigmentType, assignedAt time.Time, items []AssigmentItem, status ItemStatus) *Assignment {
	return &Assignment{
		id:         id,
		studentID:  studentID,
		title:      title,
		typ:        typ,
		assignedAt: assignedAt,
		items:      items,
		status:     status,
	}
}
func (a *Assignment) AddAnwsers(answers []Answer) {
	for i := 0; i < len(a.items); i++ {
		item := &a.items[i]
		for j := 0; j < len(answers); j++ {
			if item.id == answers[j].ItemID() {
				if item.exercise.Type() != answers[j].Type() {
					item.comment = "Câu trả lời không đúng định dạng"
					continue
				}
				item.answer = answers[j]
			}
		}
	}
}
func (a *Assignment) ListItem(ids []uuid.UUID) []GradeItem {
	items := make([]GradeItem, 0, 10)
	for _, item := range a.items {
		isValid, _ := item.answer.IsValid()
		if !item.answer.IsEmpty() && isValid {
			check := false
			for _, id := range ids {
				if id == item.id {
					check = true
					break
				}
			}
			if !check {
				continue
			}
			gradeItem := GradeItem{
				itemID:   item.id,
				exercise: item.exercise,
				answer:   item.answer,
			}
			items = append(items, gradeItem)
		}
	}
	return items
}
func (a *Assignment) ApplyGrade(items []GradeItem) {
	for i := 0; i < len(a.items); i++ {
		item := &a.items[i]
		for j := 0; j < len(items); j++ {
			if items[j].itemID == item.id {
				item.status = items[j].status
				item.comment = items[j].comment
				item.numSubCompleted = items[j].numSubCompleted
				item.numSubCorrect = items[j].numSubCorrect
				break
			}
		}
	}
	for i := 0; i < len(a.items); i++ {
		item := &a.items[i]
		if item.answer.IsEmpty() {
			item.comment = "Chưa hoàn thành"
			continue
		}
		isValid, reason := item.answer.IsValid()
		if !isValid {
			item.comment = reason
			continue
		}
	}
}
func (a *Assignment) ID() uuid.UUID {
	return a.id
}
func (a *Assignment) StudentID() uuid.UUID {
	return a.studentID
}
func (a *Assignment) Title() string {
	return a.title
}

func (a *Assignment) Type() AssigmentType {
	return a.typ
}
func (a *Assignment) AssignedAt() time.Time {
	return a.assignedAt
}
func (a *Assignment) Items() []AssigmentItem {
	copied := make([]AssigmentItem, len(a.items))
	copy(copied, a.items)
	return copied
}
func (a *Assignment) Status() ItemStatus {
	return a.status
}
