// File: infrastructure/html/converter.go
package html

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	mathFont = "font-family:'Cambria Math','STIX Two Math','Times New Roman',serif;"
	mathCSS  = mathFont + "font-size:1.08em;letter-spacing:0.01em;white-space:nowrap;"
	fracCSS  = "display:inline-flex;flex-direction:column;vertical-align:middle;text-align:center;line-height:1.05;margin:0 0.14em;" + mathFont
)

var (
	boldPattern           = regexp.MustCompile(`\*\*(.+?)\*\*`)
	italicPattern         = regexp.MustCompile(`(?:^|\s)\*([^*]+?)\*`)
	codePattern           = regexp.MustCompile("`([^`]+)`")
	orderedPattern        = regexp.MustCompile(`^(\d+)[.)]\s+(.+)$`)
	exerciseMarkerPattern = regexp.MustCompile(`^\[\[EXERCISE:(\d+):(START|ANSWER|FEEDBACK|END)\]\]$`)
)

type Converter struct{}

func New() *Converter { return &Converter{} }

// MarkdownToDocument tạo HTML độc lập, tương thích Microsoft Graph/OneNote.
// Công thức dùng HTML thuần vì OneNote thường loại JavaScript MathJax/KaTeX.
func (c *Converter) MarkdownToDocument(title, markdownContent string) string {
	var body strings.Builder
	lines := strings.Split(strings.ReplaceAll(markdownContent, "\r\n", "\n"), "\n")
	openList := ""
	closeList := func() {
		if openList != "" {
			body.WriteString("</" + openList + ">")
			openList = ""
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if marker := exerciseMarkerPattern.FindStringSubmatch(trimmed); marker != nil {
			closeList()
			exerciseID, markerType := marker[1], marker[2]
			switch markerType {
			case "START":
				body.WriteString(`<div data-id="exercise-` + exerciseID + `-region">`)
			case "ANSWER":
				body.WriteString(`<table data-id="exercise-` + exerciseID + `-work" width="760" border="0" cellspacing="0" cellpadding="0" style="width:760px;border-collapse:collapse;margin:8pt 0 14pt;"><tr><td width="760" style="width:760px;padding:0;vertical-align:top;">`)
			case "FEEDBACK":
				body.WriteString(`</td></tr></table>`)
			case "END":
				body.WriteString(`</div>`)
			}
			continue
		}
		if trimmed == "" {
			closeList()
			continue
		}

		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			if openList != "ul" {
				closeList()
				body.WriteString(`<ul style="margin:5pt 0 8pt 20pt;padding-left:12pt;">`)
				openList = "ul"
			}
			body.WriteString(`<li style="margin:4pt 0;line-height:1.6;">` + formatInline(strings.TrimSpace(trimmed[2:])) + `</li>`)
			continue
		}
		if match := orderedPattern.FindStringSubmatch(trimmed); match != nil {
			if openList != "ol" {
				closeList()
				body.WriteString(`<ol style="margin:5pt 0 8pt 20pt;padding-left:12pt;">`)
				openList = "ol"
			}
			body.WriteString(`<li style="margin:4pt 0;line-height:1.6;">` + formatInline(match[2]) + `</li>`)
			continue
		}
		closeList()

		switch {
		case strings.HasPrefix(trimmed, "### "):
			body.WriteString(`<h3 style="color:#334155;font-size:12pt;font-weight:700;margin:12pt 0 4pt;">` + formatInline(strings.TrimPrefix(trimmed, "### ")) + `</h3>`)
		case strings.HasPrefix(trimmed, "## "):
			body.WriteString(`<h2 style="color:#0f766e;font-size:14pt;font-weight:700;margin:15pt 0 5pt;">` + formatInline(strings.TrimPrefix(trimmed, "## ")) + `</h2>`)
		case strings.HasPrefix(trimmed, "# "):
			body.WriteString(`<h1 style="color:#1e3a8a;font-size:18pt;font-weight:700;margin:18pt 0 7pt;">` + formatInline(strings.TrimPrefix(trimmed, "# ")) + `</h1>`)
		case strings.HasPrefix(trimmed, "> "):
			body.WriteString(`<blockquote style="background:#f1f5f9;padding:8pt 11pt;border-left:3pt solid #0f766e;color:#475569;margin:8pt 0;line-height:1.6;">` + formatInline(strings.TrimPrefix(trimmed, "> ")) + `</blockquote>`)
		case trimmed == "---" || trimmed == "***":
			body.WriteString(`<hr style="border:0;border-top:1px solid #cbd5e1;margin:14pt 0;"/>`)
		case isDisplayMath(trimmed):
			body.WriteString(`<div style="margin:10pt 0;text-align:center;font-size:13pt;` + mathFont + `">` + renderMath(stripMathDelimiters(trimmed)) + `</div>`)
		default:
			body.WriteString(`<p style="font-size:11.5pt;line-height:1.65;margin:5pt 0;color:#1e293b;">` + formatInline(trimmed) + `</p>`)
		}
	}
	closeList()

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8" />
    <title>%s</title>
    <meta name="created" content="%s" />
    <style>
      body { font-family: Calibri, Arial, sans-serif; font-size: 11.5pt; color: #1e293b; line-height: 1.6; }
      p { orphans: 3; widows: 3; }
      sup, sub { font-family: 'Cambria Math', 'STIX Two Math', 'Times New Roman', serif; line-height: 0; }
      code { font-family: Consolas, monospace; background: #f1f5f9; padding: 1px 4px; border-radius: 3px; }
    </style>
  </head>
  <body><div style="max-width:760px;margin:0 auto;padding:8pt 4pt;">%s</div></body>
</html>`, escapeHTML(title), time.Now().Format(time.RFC3339), body.String())
}

func formatInline(text string) string {
	var out strings.Builder
	for len(text) > 0 {
		start, open, close := nextMathDelimiter(text)
		if start < 0 {
			out.WriteString(formatText(text))
			break
		}
		out.WriteString(formatText(text[:start]))
		rest := text[start+len(open):]
		end := strings.Index(rest, close)
		if end < 0 {
			out.WriteString(formatText(text[start:]))
			break
		}
		out.WriteString(`<span style="` + mathCSS + `">` + renderMath(rest[:end]) + `</span>`)
		text = rest[end+len(close):]
	}
	return out.String()
}

func formatText(text string) string {
	text = escapeHTML(text)
	text = codePattern.ReplaceAllString(text, `<code>$1</code>`)
	text = boldPattern.ReplaceAllString(text, `<strong>$1</strong>`)
	text = italicPattern.ReplaceAllString(text, ` <em>$1</em>`)
	// Ngoài delimiter chỉ đổi số mũ/chỉ số và dấu căn, không đổi dấu / của URL.
	return renderMathEscaped(text, false)
}

func nextMathDelimiter(text string) (int, string, string) {
	best, open, close := -1, "", ""
	for _, delimiter := range [][2]string{{`\(`, `\)`}, {"$", "$"}} {
		if i := strings.Index(text, delimiter[0]); i >= 0 && (best < 0 || i < best) {
			best, open, close = i, delimiter[0], delimiter[1]
		}
	}
	return best, open, close
}

func isDisplayMath(text string) bool {
	return strings.HasPrefix(text, "$$") && strings.HasSuffix(text, "$$") && len(text) >= 4 ||
		strings.HasPrefix(text, `\[`) && strings.HasSuffix(text, `\]`) && len(text) >= 4
}

func stripMathDelimiters(text string) string {
	if strings.HasPrefix(text, "$$") && strings.HasSuffix(text, "$$") {
		return strings.TrimSpace(text[2 : len(text)-2])
	}
	if strings.HasPrefix(text, `\[`) && strings.HasSuffix(text, `\]`) {
		return strings.TrimSpace(text[2 : len(text)-2])
	}
	return text
}

func renderMath(text string) string {
	return renderMathEscaped(escapeHTML(strings.TrimSpace(text)), true)
}

func renderMathEscaped(text string, fractions bool) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		if html, next, ok := renderSpecialMath(text, i); ok {
			out.WriteString(html)
			i = next
			continue
		}
		if command, replacement, ok := mathCommandAt(text, i); ok {
			out.WriteString(replacement)
			i += len(command)
			continue
		}
		if fractions {
			if html, next, ok := renderSimpleFraction(text, i); ok {
				out.WriteString(html)
				i = next
				continue
			}
			if text[i] == '-' {
				out.WriteString("−")
				i++
				continue
			}
		} else {
			// Trong câu văn thường chỉ tự động đổi phân số thuần số (1/16,
			// 3/4...). Quy tắc hẹp này giúp công thức đẹp mà không làm hỏng URL.
			if html, next, ok := renderNumericFraction(text, i); ok {
				out.WriteString(html)
				i = next
				continue
			}
		}
		out.WriteByte(text[i])
		i++
	}
	return out.String()
}

func renderSpecialMath(text string, start int) (string, int, bool) {
	for _, command := range []string{`\dfrac`, `\tfrac`, `\frac`} {
		if strings.HasPrefix(text[start:], command) {
			return renderLatexFraction(text, start, len(command))
		}
	}
	if strings.HasPrefix(text[start:], `\sqrt`) {
		return renderLatexRoot(text, start)
	}
	if strings.HasPrefix(text[start:], "√") {
		return renderUnicodeRoot(text, start)
	}
	if text[start] == '^' || text[start] == '_' {
		return renderScript(text, start)
	}
	return "", start, false
}

func renderLatexFraction(text string, start, commandLength int) (string, int, bool) {
	i := skipSpaces(text, start+commandLength)
	numerator, next, ok := extractGroup(text, i, '{', '}')
	if !ok {
		return "", start, false
	}
	i = skipSpaces(text, next)
	denominator, next, ok := extractGroup(text, i, '{', '}')
	if !ok {
		return "", start, false
	}
	return fractionHTML(renderMathEscaped(numerator, true), renderMathEscaped(denominator, true)), next, true
}

func renderLatexRoot(text string, start int) (string, int, bool) {
	i := skipSpaces(text, start+len(`\sqrt`))
	content, next, ok := extractGroup(text, i, '{', '}')
	if !ok {
		return "", start, false
	}
	return rootHTML(renderMathEscaped(content, true)), next, true
}

func renderUnicodeRoot(text string, start int) (string, int, bool) {
	i := skipSpaces(text, start+len("√"))
	if i >= len(text) {
		return "√", i, true
	}
	var content string
	var next int
	var ok bool
	switch text[i] {
	case '(':
		content, next, ok = extractGroup(text, i, '(', ')')
	case '{':
		content, next, ok = extractGroup(text, i, '{', '}')
	default:
		content, next = extractToken(text, i)
		ok = content != ""
	}
	if !ok {
		return "", start, false
	}
	return rootHTML(renderMathEscaped(content, true)), next, true
}

func renderScript(text string, start int) (string, int, bool) {
	tag, vertical := "sup", "super"
	if text[start] == '_' {
		tag, vertical = "sub", "sub"
	}
	i := skipSpaces(text, start+1)
	if i >= len(text) {
		return "", start, false
	}
	var content string
	var next int
	var ok bool
	switch text[i] {
	case '{':
		content, next, ok = extractGroup(text, i, '{', '}')
	case '(':
		content, next, ok = extractGroup(text, i, '(', ')')
	default:
		content, next = extractScriptToken(text, i)
		ok = content != ""
	}
	if !ok {
		return "", start, false
	}
	return `<` + tag + ` style="font-size:0.76em;vertical-align:` + vertical + `;` + mathFont + `">` + renderMathEscaped(content, true) + `</` + tag + `>`, next, true
}

func renderSimpleFraction(text string, start int) (string, int, bool) {
	if start > 0 && isMathToken(text[start-1]) {
		return "", start, false
	}
	numerator, slash := extractToken(text, start)
	if numerator == "" || slash >= len(text) || text[slash] != '/' {
		return "", start, false
	}
	denominator, next := extractToken(text, slash+1)
	if denominator == "" {
		return "", start, false
	}
	return fractionHTML(numerator, denominator), next, true
}

func renderNumericFraction(text string, start int) (string, int, bool) {
	if start >= len(text) || text[start] < '0' || text[start] > '9' {
		return "", start, false
	}
	if start > 0 && (isMathToken(text[start-1]) || text[start-1] == '/') {
		return "", start, false
	}

	numerator, slash := extractNumber(text, start)
	if numerator == "" || slash >= len(text) || text[slash] != '/' {
		return "", start, false
	}
	denominator, next := extractNumber(text, slash+1)
	if denominator == "" || next < len(text) && (isMathToken(text[next]) || text[next] == '/') {
		return "", start, false
	}

	return fractionHTML(numerator, denominator), next, true
}

func fractionHTML(numerator, denominator string) string {
	return `<span style="` + fracCSS + `"><span style="display:block;padding:0 0.18em 0.08em;">` + numerator +
		`</span><span style="display:block;border-top:1.2px solid currentColor;padding:0.08em 0.18em 0;">` + denominator + `</span></span>`
}

func rootHTML(content string) string {
	return `<span style="display:inline-flex;align-items:flex-start;vertical-align:middle;` + mathFont + `"><span style="font-size:1.15em;line-height:1;">√</span><span style="display:inline-block;border-top:1px solid currentColor;padding:0 0.12em 0 0.08em;margin-left:0.03em;">` + content + `</span></span>`
}

func mathCommandAt(text string, start int) (string, string, bool) {
	commands := []struct{ command, replacement string }{
		{`\Leftrightarrow`, "⇔"}, {`\Rightarrow`, "⇒"},
		{`\mathbb{R}`, "ℝ"}, {`\mathbb{N}`, "ℕ"}, {`\mathbb{Z}`, "ℤ"}, {`\mathbb{Q}`, "ℚ"},
		{`\setminus`, "∖"}, {`\emptyset`, "∅"}, {`\infty`, "∞"},
		{`\times`, "×"}, {`\cdot`, "·"}, {`\div`, "÷"},
		{`\leq`, "≤"}, {`\le`, "≤"}, {`\geq`, "≥"}, {`\ge`, "≥"}, {`\neq`, "≠"}, {`\ne`, "≠"},
		{`\notin`, "∉"}, {`\in`, "∈"}, {`\subseteq`, "⊆"}, {`\subset`, "⊂"},
		{`\cup`, "∪"}, {`\cap`, "∩"}, {`\pm`, "±"},
		{`\alpha`, "α"}, {`\beta`, "β"}, {`\gamma`, "γ"}, {`\Delta`, "Δ"}, {`\delta`, "δ"},
		{`\theta`, "θ"}, {`\lambda`, "λ"}, {`\mu`, "μ"}, {`\pi`, "π"}, {`\sigma`, "σ"}, {`\omega`, "ω"},
		{`\left`, ""}, {`\right`, ""}, {`\,`, " "}, {`\;`, " "}, {`\ `, "∖ "},
	}
	for _, item := range commands {
		if strings.HasPrefix(text[start:], item.command) {
			return item.command, item.replacement, true
		}
	}
	return "", "", false
}

func extractGroup(text string, start int, open, close byte) (string, int, bool) {
	if start >= len(text) || text[start] != open {
		return "", start, false
	}
	depth := 0
	for i := start; i < len(text); i++ {
		if text[i] == open {
			depth++
		} else if text[i] == close {
			depth--
			if depth == 0 {
				return text[start+1 : i], i + 1, true
			}
		}
	}
	return "", start, false
}

func extractToken(text string, start int) (string, int) {
	i := start
	for i < len(text) && isMathToken(text[i]) {
		i++
	}
	return text[start:i], i
}

func extractNumber(text string, start int) (string, int) {
	i := start
	dotSeen := false
	for i < len(text) {
		if text[i] >= '0' && text[i] <= '9' {
			i++
			continue
		}
		if text[i] == '.' && !dotSeen {
			dotSeen = true
			i++
			continue
		}
		break
	}
	return text[start:i], i
}

func extractScriptToken(text string, start int) (string, int) {
	i := start
	if i < len(text) && (text[i] == '+' || text[i] == '-') {
		i++
	}
	for i < len(text) && (isMathToken(text[i]) || text[i] == '/') {
		i++
	}
	return text[start:i], i
}

func isMathToken(char byte) bool {
	return char >= '0' && char <= '9' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char == '.'
}

func skipSpaces(text string, start int) int {
	for start < len(text) && text[start] == ' ' {
		start++
	}
	return start
}

func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	text = strings.ReplaceAll(text, `"`, "&quot;")
	return text
}
