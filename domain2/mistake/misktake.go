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

func NewMistake(topic string, reason string, assignmentItemID uuid.UUID) *Mistake {
	return &Mistake{
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

// func (m *Mistake)
// Trong MistakeGraph:
// Lấy toàn bộ chuỗi phả hệ từ Gốc đến Ngọn của một lỗi cụ thể
func (g *MistakeGraph) GetAncestryPath(targetMistakeID uuid.UUID) ([]*Mistake, error) {
	for _, root := range g.roots {
		var path []*Mistake
		if findPath(root, targetMistakeID, &path) {
			return path, nil
		}
	}
	return nil, errors.New("không tìm thấy lỗi sai trong phả hệ tri thức của học sinh")
}

// Thuật toán đệ quy tìm đường đi trên cây
func findPath(current *Mistake, targetID uuid.UUID, path *[]*Mistake) bool {
	// 1. Thêm nút hiện tại vào chuỗi
	*path = append(*path, current)

	// 2. Nếu đã tới đích -> Thành công!
	if current.id == targetID {
		return true
	}

	// 3. Rẽ nhánh tìm tiếp ở các nút con
	for _, child := range current.children {
		if findPath(child, targetID, path) {
			return true
		}
	}

	// 4. Backtrack: Nếu nhánh này không dẫn tới đích thì gỡ ra
	*path = (*path)[:len(*path)-1]
	return false
}
