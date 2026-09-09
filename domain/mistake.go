package domain

import (
	"strings"
	"time"
)

type MistakeStatus string

const (
	MistakeDetected    MistakeStatus = "detected"    // Mới phát hiện
	MistakeRemediating MistakeStatus = "remediating" // Đang có bài tập khắc phục (chưa giải quyết xong)
	MistakeResolved    MistakeStatus = "resolved"    // Đã giải quyết thành công
	MistakeCompounded  MistakeStatus = "compounded"  // Đẻ ra lỗi con sâu hơn (lỗ hổng tầng dưới)
)

type Mistake struct {
	ID                 int           `json:"id"`
	SourceAssignmentID int           `json:"source_assignment_id"` // Lỗi này bị bắt ở bài tập nào?
	ParentMistakeID    *int          `json:"parent_mistake_id"`    // Lỗi này là con của lỗi nào? (nil nếu là lỗi gốc)
	Depth              int           `json:"depth"`                // Độ sâu đệ quy (0 = lỗi gốc, 1, 2 = lỗi sâu)
	Topic              string        `json:"topic"`
	ErrorReason        string        `json:"error_reason"`
	Status             MistakeStatus `json:"status"`
	CreatedAt          time.Time     `json:"created_at"`
}

func (m *Mistake) StartRemediation() {
	m.Status = MistakeRemediating
}

func (m *Mistake) Resolve() {
	m.Status = MistakeResolved
}

// Hàm phụ trợ chuẩn hóa Mistake từ AI.
func extractMistake(dm *Mistake, assignmentID int, parentMistakeID *int, depth int, defaultTopic string) *Mistake {
	m := Mistake{
		SourceAssignmentID: assignmentID,
		ParentMistakeID:    parentMistakeID,
		Depth:              depth,
		Status:             MistakeDetected,
		CreatedAt:          time.Now().UTC(),
		Topic:              defaultTopic,
	}

	// Nếu AI có phát hiện và trả về topic/error_reason
	if dm != nil {
		if strings.TrimSpace(dm.Topic) != "" {
			m.Topic = dm.Topic
		}
		m.ErrorReason = dm.ErrorReason
	}

	return &m
}
