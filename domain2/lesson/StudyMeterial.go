package lesson

import (
	"errors"
	"net/url"
	"strings"
)

type MaterialType string

const (
	MaterialYouTube MaterialType = "YOUTUBE"
	MaterialPDF     MaterialType = "PDF"
)

// Value Object: Gọn gàng, bất biến, tự bảo vệ tính toàn vẹn
type StudyMaterial struct {
	materialType MaterialType
	sourceURL    string // Dùng cho YouTube (nếu là PDF thì bằng "")
	binaryData   []byte // Dùng cho PDF (nếu là YouTube thì nil)
}

func NewYouTubeMaterial(rawURL string) (*StudyMaterial, error) {
	rawURL = strings.TrimSpace(rawURL)
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil || parsedURL.Host == "" {
		return nil, errors.New("đường dẫn YouTube không hợp lệ")
	}

	host := strings.ToLower(parsedURL.Host)
	if !strings.Contains(host, "youtube.com") && !strings.Contains(host, "youtu.be") {
		return nil, errors.New("chỉ chấp nhận link từ YouTube")
	}

	return &StudyMaterial{
		materialType: MaterialYouTube,
		sourceURL:    rawURL,
		binaryData:   nil,
	}, nil
}

func NewSlicedPDFMaterial(data []byte) (*StudyMaterial, error) {
	if len(data) == 0 {
		return nil, errors.New("dữ liệu PDF sau khi cắt bị rỗng")
	}

	return &StudyMaterial{
		materialType: MaterialPDF,
		sourceURL:    "",
		binaryData:   data,
	}, nil
}

// Getters an toàn
func (m *StudyMaterial) Type() MaterialType { return m.materialType }
func (m *StudyMaterial) SourceURL() string  { return m.sourceURL }
func (m *StudyMaterial) BinaryData() []byte { return m.binaryData }
