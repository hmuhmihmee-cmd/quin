package document

import (
	"time"
	"uuid"
)

type DocumentStatus string

const (
	DocStatusProcessing DocumentStatus = "PROCESSING" // Đang convert / render ảnh
	DocStatusReady      DocumentStatus = "READY"      // Đã sẵn sàng để lướt xem
	DocStatusFailed     DocumentStatus = "FAILED"
)

// Aggregate Root: Tài liệu nguồn
type SourceDocument struct {
	id               uuid.UUID
	originalFileName string
	storagePath      string   // Đường dẫn lưu file PDF gốc
	totalPages       int      // Tổng số trang
	pagePreviewURLs  []string // Danh sách link ảnh từng trang để chị bạn lướt xem
	status           DocumentStatus
	createdAt        time.Time
	ErrReason        error
}

func NewSourceDocument(fileName string) *SourceDocument {
	return &SourceDocument{
		id:               uuid.New(),
		originalFileName: fileName,
		status:           DocStatusProcessing,
		createdAt:        time.Now().UTC(),
	}
}

// Cập nhật khi hạ tầng convert & render xong
func (d *SourceDocument) MarkAsReady(pdfPath string, totalPages int, previewURLs []string) {
	d.storagePath = pdfPath
	d.totalPages = totalPages
	d.pagePreviewURLs = previewURLs
	d.status = DocStatusReady
}

func (d *SourceDocument) MarkAsFailed(err error) {
	d.ErrReason = err
	d.status = DocStatusFailed
}

// Getters...
func (d *SourceDocument) ID() uuid.UUID             { return d.id }
func (d *SourceDocument) TotalPages() int           { return d.totalPages }
func (d *SourceDocument) PagePreviewURLs() []string { return d.pagePreviewURLs }
func (d *SourceDocument) StoragePath() string       { return d.storagePath }
func (d *SourceDocument) IsReady() bool             { return d.status == DocStatusReady }
