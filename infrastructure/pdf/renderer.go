package pdf

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"

	"meet-attendance-clean/domain"

	"github.com/go-pdf/fpdf"
)

//go:embed fonts/regular.ttf
var fontRegular []byte

//go:embed fonts/bold.ttf
var fontBold []byte

//go:embed fonts/italic.ttf
var fontItalic []byte

//go:embed fonts/bolditalic.ttf
var fontBoldItalic []byte

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

// Implement application.DocumentRenderer
func (r *Renderer) Render(title, subtitle, content string) (domain.Document, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 15, 20)
	pdf.SetAutoPageBreak(true, 15)

	// Nạp font tiếng Việt UTF-8
	pdf.AddUTF8FontFromBytes("VN", "", fontRegular)
	pdf.AddUTF8FontFromBytes("VN", "B", fontBold)
	pdf.AddUTF8FontFromBytes("VN", "I", fontItalic)
	pdf.AddUTF8FontFromBytes("VN", "BI", fontBoldItalic)

	if err := pdf.Error(); err != nil {
		return domain.Document{}, fmt.Errorf("lỗi nạp font tiếng Việt: %w", err)
	}

	pdf.AddPage()

	// 1. Vẽ Header đầu trang
	pdf.SetFont("VN", "B", 16)
	pdf.SetTextColor(25, 49, 80) // Xanh đen đậm
	pdf.MultiCell(0, 8, title, "", "C", false)

	if subtitle != "" {
		pdf.SetFont("VN", "I", 11)
		pdf.SetTextColor(100, 116, 139) // Xám nhạt
		pdf.MultiCell(0, 6, subtitle, "", "C", false)
	}
	pdf.Ln(3)

	// Đường kẻ phân cách Header
	pdf.SetDrawColor(203, 213, 225)
	pdf.Line(20, pdf.GetY(), 190, pdf.GetY())
	pdf.Ln(6)

	// 2. Phân tích từng dòng Markdown và vẽ lên PDF
	lines := strings.Split(content, "\n")
	pdf.SetTextColor(30, 41, 59)

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			pdf.Ln(3)
			continue
		}

		if strings.HasPrefix(trimmed, "# ") {
			// Heading 1
			pdf.SetFont("VN", "B", 14.5)
			pdf.SetTextColor(15, 23, 42)
			pdf.MultiCell(0, 8, cleanText(strings.TrimPrefix(trimmed, "# ")), "", "L", false)
		} else if strings.HasPrefix(trimmed, "## ") {
			// Heading 2
			pdf.Ln(2)
			pdf.SetFont("VN", "B", 13)
			pdf.SetTextColor(30, 58, 138) // Xanh dương
			pdf.MultiCell(0, 7, cleanText(strings.TrimPrefix(trimmed, "## ")), "", "L", false)
		} else if strings.HasPrefix(trimmed, "### ") {
			// Heading 3
			pdf.SetFont("VN", "B", 11.5)
			pdf.SetTextColor(51, 65, 85)
			pdf.MultiCell(0, 6, cleanText(strings.TrimPrefix(trimmed, "### ")), "", "L", false)
		} else if strings.HasPrefix(trimmed, "> ") {
			// Blockquote / Overview
			pdf.SetFont("VN", "I", 10.5)
			pdf.SetTextColor(71, 85, 105)
			pdf.MultiCell(0, 5.5, cleanText(strings.TrimPrefix(trimmed, "> ")), "", "L", false)
		} else if trimmed == "---" {
			// Đường kẻ ngang
			pdf.Ln(2)
			pdf.SetDrawColor(226, 232, 240)
			pdf.Line(20, pdf.GetY(), 190, pdf.GetY())
			pdf.Ln(4)
		} else if strings.HasPrefix(trimmed, "- ") {
			// Danh sách gạch đầu dòng
			pdf.SetFont("VN", "", 10.5)
			pdf.SetTextColor(30, 41, 59)
			pdf.CellFormat(6, 5.5, "•", "", 0, "L", false, 0, "")
			pdf.MultiCell(0, 5.5, cleanText(strings.TrimPrefix(trimmed, "- ")), "", "L", false)
		} else {
			// Đoạn văn bản thường
			pdf.SetFont("VN", "", 10.5)
			pdf.SetTextColor(30, 41, 59)
			pdf.MultiCell(0, 5.5, cleanText(trimmed), "", "L", false)
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return domain.Document{}, fmt.Errorf("lỗi xuất buffer PDF: %w", err)
	}

	return domain.Document{
		Title:       title,
		ContentType: "application/pdf",
		Data:        buf.Bytes(),
	}, nil
}

// Làm sạch các ký tự đặc biệt có thể làm lỗi font fpdf
func cleanText(text string) string {
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "`", "")
	return text
}