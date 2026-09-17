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

// 1. ĐỊNH NGHĨA INTERFACE CHUNG
type StudyMaterial interface {
	Type() MaterialType
}

// 2. STRUCT DÀNH RIÊNG CHO YOUTUBE (Không chứa byte rác)
type YouTubeMaterial struct {
	sourceURL string
}

func (y YouTubeMaterial) Type() MaterialType { return MaterialYouTube }
func (y YouTubeMaterial) SourceURL() string  { return y.sourceURL }

func NewYouTubeMaterial(rawURL string) (YouTubeMaterial, error) {
	rawURL = strings.TrimSpace(rawURL)
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil || parsedURL.Host == "" {
		return YouTubeMaterial{}, errors.New("đường dẫn YouTube không hợp lệ")
	}

	host := strings.ToLower(parsedURL.Host)
	if !strings.Contains(host, "youtube.com") && !strings.Contains(host, "youtu.be") {
		return YouTubeMaterial{}, errors.New("chỉ chấp nhận link từ YouTube")
	}

	return YouTubeMaterial{sourceURL: rawURL}, nil
}

// 3. STRUCT DÀNH RIÊNG CHO PDF (Không chứa URL rác)
type PDFMaterial struct {
	binaryData []byte
}

func (p PDFMaterial) Type() MaterialType { return MaterialPDF }
func (p PDFMaterial) BinaryData() []byte { return p.binaryData }

func NewSlicedPDFMaterial(data []byte) (PDFMaterial, error) {
	if len(data) == 0 {
		return PDFMaterial{}, errors.New("dữ liệu PDF rỗng")
	}
	return PDFMaterial{binaryData: data}, nil
}
