// File: infrastructure/html/renderer.go
package html

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"meet-attendance-clean/domain"
)

type LessonRenderer struct{}

func NewLessonRenderer() *LessonRenderer {
	return &LessonRenderer{}
}

// RenderLessonHTML là hàm điều phối chính
func (r *LessonRenderer) RenderLessonHTML(pageTitle string, lesson *domain.Lesson, audience domain.Audience) string {
	var body strings.Builder

	if audience == domain.AudienceTeacher {
		r.renderTeacherLesson(&body, lesson)
	} else {
		r.renderStudentLesson(&body, lesson)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8" />
    <title>%s</title>
    <meta name="created" content="%s" />
    <style>
      body { font-family: Calibri, Arial, sans-serif; font-size: 14pt; color: #1e293b; line-height: 1.6; margin: 0; padding: 10pt; }
      p { orphans: 3; widows: 3; margin: 0 0 6pt 0; }
    </style>
  </head>
  <body>
    %s
  </body>
</html>`, escapeHTML(pageTitle), time.Now().Format(time.RFC3339), body.String())
}

// =========================================================================
// 1. PHIẾU HỌC SINH (AudienceStudent)
// =========================================================================
func (r *LessonRenderer) renderStudentLesson(body *strings.Builder, lesson *domain.Lesson) {
	// Header & Tổng quan (Huy hiệu 16pt, nội dung 14pt)
	body.WriteString(
		`<div style="margin-bottom:12pt;">` +
			`<span style="background:#0f766e; color:#fff; padding:4pt 10pt; border-radius:3px; font-size:16pt; font-weight:bold;">PHIẾU BÀI TẬP HỌC SINH</span>`,
	)
	if strings.TrimSpace(lesson.Overview) != "" {
		body.WriteString(renderTextBlocks(lesson.Overview, "color:#475569; font-size:14pt; font-style:italic; margin-top:8pt;"))
	}
	body.WriteString(`</div><hr style="border:0; border-top:1.5px solid #cbd5e1; margin:12pt 0;"/>`)

	// PHẦN 1: LÝ THUYẾT & VÍ DỤ TRÊN LỚP (H2: 20pt)
	if len(lesson.Sections) > 0 {
		body.WriteString(`<h2 style="color:#0f766e; font-size:20pt; margin:16pt 0 8pt 0;">PHẦN 1: LÝ THUYẾT & VÍ DỤ TRÊN LỚP</h2>`)

		for secIdx, sec := range lesson.Sections {
			sectionTitle := cleanSectionTitle(sec.SectionTitle)
			body.WriteString(fmt.Sprintf(
				`<div style="margin-bottom:14pt;">`+
					`<h3 style="color:#0f172a; font-size:18pt; font-weight:700; margin:12pt 0 5pt 0;">%d. %s</h3>`,
				secIdx+1, formatMathInline(sectionTitle),
			))

			if sec.TransitionIntro != "" {
				body.WriteString(renderTextBlocks(sec.TransitionIntro, "color:#334155; font-size:14pt; font-style:italic;"))
			}
			if sec.DetailedContent != "" {
				body.WriteString(formatDetailedContent(sec.DetailedContent))
			}

			// Ghi nhớ quan trọng (Key Takeaway: 15pt)
			if sec.KeyTakeaway != "" {
				body.WriteString(fmt.Sprintf(
					`<div style="background:#fef3c7; border:1px solid #fde68a; padding:6pt 10pt; border-radius:3px; margin:6pt 0 8pt 0; font-size:15pt;">`+
						`<strong style="color:#92400e;">💡 Cần nhớ:</strong> <span style="color:#78350f;">%s</span>`+
						`</div>`,
					formatMathInline(sec.KeyTakeaway),
				))
			}

			// Điền từ vào chỗ trống (Cloze Notes: 14pt)
			if len(sec.StudentClozeNotes) > 0 {
				body.WriteString(`<ul style="margin:4pt 0 10pt 16pt; padding:0; line-height:1.7;">`)
				for _, note := range sec.StudentClozeNotes {
					body.WriteString(fmt.Sprintf(`<li style="color:#1e293b; font-size:14pt; margin-bottom:4pt;">%s</li>`, formatMathInline(note)))
				}
				body.WriteString(`</ul>`)
			}

			// Ví dụ mẫu trên lớp (Tiêu đề ví dụ: 16pt, nội dung: 14pt)
			for _, eg := range sec.TeacherExamples {
				body.WriteString(fmt.Sprintf(
					`<div style="margin:10pt 0 14pt 0;">`+
						`<p style="font-size:16pt; margin:0 0 4pt 0;"><strong>Ví dụ %d:</strong> <span style="font-size:14pt;">%s</span></p>`,
					eg.ExampleNum, formatMathInline(eg.Problem),
				))

				if eg.StudentFriendlyExplanation != "" {
					body.WriteString(fmt.Sprintf(
						`<p style="color:#334155; font-size:14pt; font-style:italic; margin:0 0 4pt 0;"><em>Gợi ý suy nghĩ:</em> %s</p>`,
						formatMathInline(eg.StudentFriendlyExplanation),
					))
				}

				body.WriteString(
					`<p style="font-size:14pt; color:#475569; margin:4pt 0 2pt 0;">Bài làm:</p>` +
						`<p style="color:#94a3b8; font-size:14pt; letter-spacing:1.5px; margin:0 0 2pt 0;">....................................................................................................</p>` +
						`<p style="color:#94a3b8; font-size:14pt; letter-spacing:1.5px; margin:0 0 2pt 0;">....................................................................................................</p>`,
				)
				body.WriteString(`</div>`)
			}

			body.WriteString(`</div>`)
		}
		body.WriteString(`<hr style="border:0; border-top:1.5px solid #cbd5e1; margin:16pt 0;"/>`)
	}

	// PHẦN 2: BÀI TẬP TỰ LUYỆN (H2: 20pt)
	if len(lesson.Exercises) > 0 {
		body.WriteString(`<h2 style="color:#0f766e; font-size:20pt; margin:16pt 0 10pt 0;">PHẦN 2: BÀI TẬP TỰ LUYỆN</h2>`)

		for idx, ex := range lesson.Exercises {
			body.WriteString(fmt.Sprintf(`<div data-id="exercise-%d-region" style="margin: 16pt 0 24pt 0;">`, ex.ID))

			diffBadge := ""
			if ex.Difficulty != "" {
				diffBadge = fmt.Sprintf(`<span style="font-size:14pt; background:#e2e8f0; color:#475569; padding:2pt 6pt; border-radius:3px; margin-left:6pt;">%s</span>`, escapeHTML(ex.Difficulty))
			}

			// Câu hỏi: 16pt
			body.WriteString(fmt.Sprintf(
				`<p style="font-size:16pt; font-weight:bold; color:#1e293b; margin:0 0 6pt 0;">`+
					`Câu %d (%s):%s <span style="font-weight:normal;">%s</span>`+
					`</p>`,
				idx+1, escapeHTML(ex.Type), diffBadge, formatMathInline(ex.Question),
			))

			// Lựa chọn trắc nghiệm: 15pt
			if len(ex.Options) > 0 {
				body.WriteString(fmt.Sprintf(`<div style="margin:6pt 0 10pt 0;">`+
					`<p style="margin:0 0 4pt 0; color:#475569; font-size:15pt;"><strong>Chọn duy nhất 1 đáp án:</strong></p>`+
					`<table data-id="exercise-%d-choice" width="760" style="width:760px; border-collapse:separate; border-spacing:5pt;">`, ex.ID))
				for optionIdx, opt := range ex.Options {
					letter := optionLetter(optionIdx)
					if optionIdx%2 == 0 {
						body.WriteString(`<tr>`)
					}
					body.WriteString(fmt.Sprintf(`<td width="370" style="width:370px; padding:8pt 10pt; vertical-align:middle; background:#f8fafc; border:1px solid #cbd5e1;">`+
						`<p data-id="exercise-%d-option-%s" data-tag="to-do" style="margin:0; color:#1e293b; font-size:15pt; line-height:1.45;">`+
						`<strong>%s.</strong> %s</p></td>`, ex.ID, letter, letter, formatMathInline(optionTextWithoutLetter(opt, letter))))
					if optionIdx%2 == 1 || optionIdx == len(ex.Options)-1 {
						if optionIdx%2 == 0 {
							body.WriteString(`<td width="370" style="width:370px; padding:0; border:0;">&nbsp;</td>`)
						}
						body.WriteString(`</tr>`)
					}
				}
				body.WriteString(`</table></div>`)
			}

			// Vùng làm bài: hướng dẫn 14pt
			body.WriteString(fmt.Sprintf(
				`<table width="1050" style="width:1050px; border-collapse:collapse; margin:8pt 0 6pt 0;"><tr>`+
					`<td width="760" style="width:760px; padding:0; vertical-align:top;">`+
					`<table data-id="exercise-%d-work" width="760" style="width:760px; border-collapse:collapse;">`+
					`<tr><td height="180" style="height:180pt; padding:10pt; border:1.5px dashed #94a3b8; background-color:#f8fafc; vertical-align:top;">`+
					`<div data-id="exercise-%d-answer-content">`+
					`<p style="margin:0 0 4pt 0; color:#94a3b8; font-size:14pt;"><em>✍️ Bài làm (Gõ chữ hoặc viết/chèn ảnh vào khung này):</em></p>`+
					`<p style="margin:0; line-height:32pt;">&nbsp;</p><p style="margin:0; line-height:32pt;">&nbsp;</p><p style="margin:0; line-height:32pt;">&nbsp;</p>`+
					`</div></td></tr></table></td>`+
					`<td width="290" style="width:290px; padding:0 0 0 12pt; vertical-align:top;">`+
					`<div data-id="exercise-%d-feedback" style="width:270px; min-height:1px;">&nbsp;</div>`+
					`</td></tr></table>`,
				ex.ID, ex.ID, ex.ID,
			))

			body.WriteString(`</div>`)
		}
	}
}

// =========================================================================
// 2. PHIẾU GIÁO VIÊN (AudienceTeacher)
// =========================================================================
func (r *LessonRenderer) renderTeacherLesson(body *strings.Builder, lesson *domain.Lesson) {
	// Header & Tổng quan (Huy hiệu 16pt, nội dung 14pt)
	body.WriteString(
		`<div style="margin-bottom:12pt;">` +
			`<span style="background:#1e3a8a; color:#fff; padding:4pt 10pt; border-radius:3px; font-size:16pt; font-weight:bold;">GIÁO ÁN GIẢNG DẠY & ĐÁP ÁN (GIÁO VIÊN)</span>`,
	)
	if strings.TrimSpace(lesson.Overview) != "" {
		body.WriteString(renderTextBlocks(lesson.Overview, "color:#475569; font-size:14pt; font-style:italic; margin-top:8pt;"))
	}
	body.WriteString(`</div><hr style="border:0; border-top:1.5px solid #cbd5e1; margin:12pt 0;"/>`)

	// PHẦN 1: HƯỚNG DẪN GIẢNG DẠY (H2: 20pt)
	if len(lesson.Sections) > 0 {
		body.WriteString(`<h2 style="color:#1e3a8a; font-size:20pt; margin:16pt 0 8pt 0;">I. KIẾN THỨC TRỌNG TÂM & HƯỚNG DẪN GIẢNG DẠY</h2>`)

		for secIdx, sec := range lesson.Sections {
			sectionTitle := cleanSectionTitle(sec.SectionTitle)
			body.WriteString(fmt.Sprintf(
				`<div style="background:#f8fafc; border-left:3.5px solid #cbd5e1; padding:10pt 12pt; margin-bottom:14pt; border-radius:0 4px 4px 0;">`+
					`<h3 style="color:#0f172a; font-size:18pt; font-weight:700; margin:0 0 5pt 0;">%d. %s</h3>`,
				secIdx+1, formatMathInline(sectionTitle),
			))

			if sec.TransitionIntro != "" {
				body.WriteString(renderTextBlocks(sec.TransitionIntro, "color:#334155; font-size:14pt; font-style:italic;"))
			}
			if sec.DetailedContent != "" {
				body.WriteString(formatDetailedContent(sec.DetailedContent))
			}

			// Ghi nhớ (15pt)
			if sec.KeyTakeaway != "" {
				body.WriteString(fmt.Sprintf(
					`<div style="background:#fef3c7; border:1px solid #fde68a; padding:6pt 10pt; border-radius:3px; margin:6pt 0 8pt 0; font-size:15pt;">`+
						`<strong style="color:#92400e;">💡 Cần nhớ:</strong> <span style="color:#78350f;">%s</span>`+
						`</div>`,
					formatMathInline(sec.KeyTakeaway),
				))
			}

			// Ví dụ mẫu (Tiêu đề nhóm: 16pt, tiêu đề ví dụ: 16pt, lời giải: 14pt)
			if len(sec.TeacherExamples) > 0 {
				body.WriteString(`<div style="margin-top:10pt;"><strong style="color:#0f172a; font-size:16pt;">Ví dụ mẫu & Hướng dẫn giải:</strong>`)
				for _, eg := range sec.TeacherExamples {
					body.WriteString(fmt.Sprintf(
						`<div style="background:#ffffff; border:1px solid #e2e8f0; padding:8pt 10pt; margin:6pt 0; border-radius:4px;">`+
							`<p style="font-weight:bold; font-size:16pt; color:#1e293b; margin:0 0 4pt 0;">Ví dụ %d: <span style="font-weight:normal; font-size:14pt;">%s</span></p>`+
							`<div style="background:#f0fdf4; border-left:3px solid #16a34a; padding:6pt 8pt; margin:6pt 0 4pt 0;">`+
							`<strong style="color:#15803d; font-size:15pt;">Lời giải chi tiết:</strong><div style="color:#166534; font-size:14pt; margin-top:2pt;">%s</div>`+
							`</div>`,
						eg.ExampleNum, formatMathInline(eg.Problem), renderTextBlocks(eg.TeacherSolution, "color:#166534; font-size:14pt;"),
					))

					if eg.CommonMistake != "" {
						body.WriteString(fmt.Sprintf(
							`<p style="color:#dc2626; font-size:14pt; margin:4pt 0 0 0;"><strong>⚠️ Lỗi học sinh hay gặp:</strong> %s</p>`,
							formatMathInline(eg.CommonMistake),
						))
					}
					body.WriteString(`</div>`)
				}
				body.WriteString(`</div>`)
			}

			body.WriteString(`</div>`)
		}
		body.WriteString(`<hr style="border:0; border-top:1.5px solid #cbd5e1; margin:16pt 0;"/>`)
	}

	// PHẦN 2: BÀI TẬP & ĐÁP ÁN (H2: 20pt)
	if len(lesson.Exercises) > 0 {
		body.WriteString(`<h2 style="color:#1e3a8a; font-size:20pt; margin:16pt 0 10pt 0;">II. HỆ THỐNG BÀI TẬP & ĐÁP ÁN GIẢNG DẠY</h2>`)

		for idx, ex := range lesson.Exercises {
			body.WriteString(fmt.Sprintf(`<div data-id="exercise-%d-region" style="margin: 14pt 0 20pt 0;">`, ex.ID))

			diffBadge := ""
			if ex.Difficulty != "" {
				diffBadge = fmt.Sprintf(`<span style="font-size:14pt; background:#e2e8f0; color:#475569; padding:2pt 6pt; border-radius:3px; margin-left:6pt;">%s</span>`, escapeHTML(ex.Difficulty))
			}

			// Câu hỏi: 16pt
			body.WriteString(fmt.Sprintf(
				`<p style="font-size:16pt; font-weight:bold; color:#1e293b; margin:0 0 6pt 0;">`+
					`Câu %d (%s):%s <span style="font-weight:normal;">%s</span>`+
					`</p>`,
				idx+1, escapeHTML(ex.Type), diffBadge, formatMathInline(ex.Question),
			))

			// Lựa chọn: 15pt
			if len(ex.Options) > 0 {
				body.WriteString(`<div style="margin:4pt 0 8pt 12pt; display:flex; flex-direction:column; gap:4pt;">`)
				for _, opt := range ex.Options {
					body.WriteString(fmt.Sprintf(`<div style="color:#334155; font-size:15pt;">%s</div>`, formatMathInline(opt)))
				}
				body.WriteString(`</div>`)
			}

			// Đáp án: Nhãn 15pt, lời giải 14pt
			body.WriteString(fmt.Sprintf(
				`<div style="background:#f0fdf4; border:1px solid #bbf7d0; border-left:4px solid #16a34a; padding:8pt 12pt; border-radius:4px; margin:8pt 0 12pt 0;">`+
					`<p style="margin:0 0 4pt 0; color:#15803d; font-weight:bold; font-size:15pt;">`+
					`✅ Đáp án đúng: <span style="background:#16a34a; color:#ffffff; padding:2pt 8pt; border-radius:3px; font-size:15pt;">%s</span>`+
					`</p>`+
					`<div style="margin:0; color:#166534; font-size:14pt; line-height:1.5;"><strong>Lời giải chi tiết:</strong>%s</div>`+
					`</div>`,
				formatMathInline(ex.Answer), renderTextBlocks(ex.Explanation, "color:#166534; font-size:14pt; line-height:1.5;"),
			))

			body.WriteString(`</div>`)
		}
	}
}

func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	text = strings.ReplaceAll(text, `"`, "&quot;")
	return text
}

var (
	fractionRE           = regexp.MustCompile(`(-?\d+|\([^()\r\n]+\))\s*/\s*(\d+|\([^()\r\n]+\))`)
	superscriptRE        = regexp.MustCompile(`\^(\{([^{}\r\n]+)\}|[A-Za-z0-9()+\-]+)`)
	subscriptRE          = regexp.MustCompile(`_([A-Za-z0-9]+)`)
	inlineStepRE         = regexp.MustCompile(`(?i)[ \t]+[+\-−][ \t]+(bước[ \t]*\d+|các bước|nếu |trường hợp|kết luận|lập bảng|biểu diễn|vẽ |tính |thay )`)
	listItemRE           = regexp.MustCompile(`^(?:[•*+\-−][ \t]+|bước[ \t]*\d+[.:][ \t]*)`)
	sectionTitlePrefixRE = regexp.MustCompile(`(?i)^\s*(?:(?:mục|phần|bài)\s+\d+\s*[.:\-–)]*\s*|\d+\s*[.:\-–)]\s*|[ivxlcdm]+\s*[.:\-–)]\s*)`)
)

func formatMathInline(text string) string {
	value := escapeHTML(strings.TrimSpace(text))
	if value == "" {
		return ""
	}
	value = fractionRE.ReplaceAllStringFunc(value, func(match string) string {
		parts := fractionRE.FindStringSubmatch(match)
		return "<sup>" + strings.TrimSpace(parts[1]) + "</sup>⁄<sub>" + strings.TrimSpace(parts[2]) + "</sub>"
	})
	value = superscriptRE.ReplaceAllStringFunc(value, func(match string) string {
		parts := superscriptRE.FindStringSubmatch(match)
		exponent := parts[1]
		if parts[2] != "" {
			exponent = parts[2]
		}
		return "<sup>" + exponent + "</sup>"
	})
	value = subscriptRE.ReplaceAllString(value, `<sub>$1</sub>`)
	value = strings.ReplaceAll(value, " => ", " ⇒ ")
	value = strings.ReplaceAll(value, " -> ", " → ")
	return value
}

func renderTextBlocks(text, style string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = inlineStepRE.ReplaceAllString(text, "\n• $1")

	lines := strings.Split(text, "\n")
	var out strings.Builder
	inList := false
	closeList := func() {
		if inList {
			out.WriteString(`</ul>`)
			inList = false
		}
	}
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			closeList()
			continue
		}
		if listItemRE.MatchString(line) {
			if !inList {
				out.WriteString(`<ul style="margin:4pt 0 8pt 18pt; padding:0;">`)
				inList = true
			}
			line = listItemRE.ReplaceAllString(line, "")
			out.WriteString(fmt.Sprintf(`<li style="%s margin:0 0 3pt 0; font-size:14pt;">%s</li>`, style, formatMathInline(line)))
			continue
		}
		closeList()
		out.WriteString(fmt.Sprintf(`<p style="%s font-size:14pt;">%s</p>`, style, formatMathInline(line)))
	}
	closeList()
	return out.String()
}

func cleanSectionTitle(title string) string {
	cleaned := strings.TrimSpace(title)
	for range 3 {
		next := strings.TrimSpace(sectionTitlePrefixRE.ReplaceAllString(cleaned, ""))
		if next == cleaned {
			break
		}
		cleaned = next
	}
	return cleaned
}

func formatDetailedContent(rawText string) string {
	text := strings.ReplaceAll(rawText, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = inlineStepRE.ReplaceAllString(text, "\n- $1")

	var out strings.Builder
	out.WriteString(`<div style="margin:6pt 0 10pt 0; line-height:1.65; color:#1e293b; font-size:14pt;">`)
	inRootList := false
	inChildList := false
	inRootItem := false
	closeChildList := func() {
		if inChildList {
			out.WriteString(`</ul>`)
			inChildList = false
		}
	}
	closeRootItem := func() {
		if inRootItem {
			closeChildList()
			out.WriteString(`</li>`)
			inRootItem = false
		}
	}
	closeRootList := func() {
		if inRootList {
			closeRootItem()
			out.WriteString(`</ul>`)
			inRootList = false
		}
	}
	openRootList := func() {
		if !inRootList {
			out.WriteString(`<ul style="margin:6pt 0 8pt 20pt; padding:0; list-style-type:disc;">`)
			inRootList = true
		}
	}
	for _, rawLine := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			closeRootList()
			continue
		}

		if strings.HasPrefix(trimmed, "* ") {
			content := strings.TrimSpace(strings.TrimPrefix(trimmed, "* "))
			openRootList()
			if !inRootItem {
				out.WriteString(`<li style="margin:0 0 3pt 0; color:#0f172a; font-size:14pt;">`)
				inRootItem = true
			}
			if !inChildList {
				out.WriteString(`<ul style="margin:3pt 0 3pt 18pt; padding:0; list-style-type:circle;">`)
				inChildList = true
			}
			out.WriteString(fmt.Sprintf(
				`<li style="margin:0 0 3pt 0; color:#1e293b; font-size:14pt;">%s</li>`,
				formatMathInline(content),
			))
			continue
		}

		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "• ") || strings.HasPrefix(trimmed, "+ ") {
			content := strings.TrimSpace(trimmed[2:])
			openRootList()
			closeRootItem()
			out.WriteString(fmt.Sprintf(
				`<li style="margin:0 0 5pt 0; color:#1e293b; font-weight:normal; font-size:14pt;">%s`,
				formatMathInline(content),
			))
			inRootItem = true
			continue
		}

		closeRootList()
		out.WriteString(fmt.Sprintf(`<p style="margin:4pt 0; font-size:14pt;">%s</p>`, formatMathInline(trimmed)))
	}
	closeRootList()
	out.WriteString(`</div>`)
	return out.String()
}

func optionLetter(index int) string {
	if index >= 0 && index < 26 {
		return string(rune('A' + index))
	}
	return fmt.Sprintf("%d", index+1)
}

func optionTextWithoutLetter(option, letter string) string {
	trimmed := strings.TrimSpace(option)
	prefixes := []string{letter + ".", letter + ")", letter + ":"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToUpper(trimmed), prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return trimmed
}