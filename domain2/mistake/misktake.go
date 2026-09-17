package mistake

import (
	"errors"
	"time"
	"uuid"
)

type MistakeStatus string

const (
	MistakeStatusDetected    MistakeStatus = "DETECTED"    // Mới phát hiện
	MistakeStatusRemediating MistakeStatus = "REMEDIATING" // Đang làm bài chữa
	MistakeStatusResolved    MistakeStatus = "RESOLVED"    // Đã giải quyết xong
)

type Mistake struct {
	id               uuid.UUID
	topic            string
	reason           string
	assignmentItemID uuid.UUID
	children         []*Mistake
	createdAt        time.Time
	resolvedAt       *time.Time
	status           MistakeStatus
}

func (m *Mistake) ID() uuid.UUID {
	return m.id
}
func (m *Mistake) Topic() string {
	return m.topic
}
func (m *Mistake) Reason() string {
	return m.reason
}
func (m *Mistake) AssignmentItemID() uuid.UUID {
	return m.assignmentItemID
}
func (m *Mistake) Children() []*Mistake {
	return m.children
}
func (m *Mistake) CreatedAt() time.Time {
	return m.createdAt
}
func (m *Mistake) ResolvedAt() *time.Time {
	return m.resolvedAt
}
func (m *Mistake) Status() MistakeStatus {
	return m.status
}
func NewMistake(topic string, reason string, assignmentItemID uuid.UUID) Mistake {
	return Mistake{
		id:               uuid.New(),
		topic:            topic,
		reason:           reason,
		assignmentItemID: assignmentItemID,
		children:         make([]*Mistake, 0),
		createdAt:        time.Now().UTC(),
	}
}
func (m *Mistake) CanReslove() bool {
	for _, child := range m.children {
		if child.status == MistakeStatusResolved || !child.CanReslove() {
			return false
		}
	}
	return true
}
func (m *Mistake) Reslove() error {
	if !m.CanReslove() {
		return errors.New("không thể đóng lỗ hổng này vì các lỗ hổng tầng sâu hơn bên dưới chưa được giải quyết triệt để")
	}
	m.status = MistakeStatusResolved
	now := time.Now().UTC()
	m.resolvedAt = &now
	return nil
}
