package pdf

import (
	"bytes"
	_ "embed"
	"fmt"
	"regexp"
	"strings"

	"meet-attendance-clean/domain"

	"github.com/go-pdf/fpdf"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gmtext "github.com/yuin/goldmark/text"
)

//go:embed fonts/regular.ttf
var regular []byte

//go:embed fonts/bold.ttf
var bold []byte

//go:embed fonts/italic.ttf
var italic []byte

//go:embed fonts/bolditalic.ttf
var boldItalic []byte

type Generator struct{}

func New() *Generator { return &Generator{} }

func (g *Generator) Draft(lesson *domain.LessonData, sourceURL string) domain.LessonDraft {
	return domain.LessonDraft{
		Title:           lesson.LessonTitle,
		SourceURL:       sourceURL,
		TeacherMarkdown: teacherMarkdown(lesson, sourceURL),
		StudentMarkdown: studentMarkdown(lesson),
	}
}

func (g *Generator) GeneratePDF(title, teacherMarkdown, studentMarkdown string) (domain.LessonFiles, error) {
	teacherMarkdown = normalizeGeneratedMarkdown(teacherMarkdown)
	studentMarkdown = normalizeGeneratedMarkdown(studentMarkdown)
	teacher, err := markdownToPDF(teacherMarkdown, "Giáo án: "+title)
	if err != nil {
		return domain.LessonFiles{}, fmt.Errorf("tạo PDF giáo viên: %w", err)
	}
	student, err := markdownToPDF(studentMarkdown, "Tài liệu học tập: "+title)
	if err != nil {
		return domain.LessonFiles{}, fmt.Errorf("tạo PDF học sinh: %w", err)
	}
	return domain.LessonFiles{Title: title, TeacherPDF: teacher, StudentPDF: student}, nil
}

func teacherMarkdown(data *domain.LessonData, sourceURL string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# GIÁO ÁN CHI TIẾT: %s\n\n", strings.ToUpper(data.LessonTitle))
	fmt.Fprintf(&out, "**Nguồn video bài giảng:** %s\n\n", sourceURL)
	if data.LessonOverview != "" {
		fmt.Fprintf(&out, "> %s\n\n", data.LessonOverview)
	}
	out.WriteString("---\n\n# PHẦN A: NỘI DUNG BÀI GIẢNG & VÍ DỤ MẪU\n\n")
	for _, section := range data.Sections {
		fmt.Fprintf(&out, "## %s\n\n", section.SectionTitle)
		if section.TransitionIntro != "" {
			fmt.Fprintf(&out, "*%s*\n\n", section.TransitionIntro)
		}
		fmt.Fprintf(&out, "%s\n\n", section.DetailedContent)
		for _, example := range section.TeacherExamples {
			fmt.Fprintf(&out, "**Ví dụ %d:** %s\n\n%s\n\n", example.ExampleNum, example.Problem, example.TeacherSolution)
			if example.CommonMistake != "" {
				fmt.Fprintf(&out, "*Lưu ý khi giảng: %s*\n\n", example.CommonMistake)
			}
		}
		if section.KeyTakeaway != "" {
			fmt.Fprintf(&out, "**Chốt lại:** %s\n\n", section.KeyTakeaway)
		}
		out.WriteString("---\n\n")
	}
	out.WriteString("# PHẦN B: BÀI TẬP RÈN LUYỆN — ĐÁP ÁN CHO GIÁO VIÊN\n\n")
	for _, exercise := range data.AIGeneratedExercises {
		writeExercise(&out, exercise, true)
	}
	return normalizeGeneratedMarkdown(out.String())
}

func studentMarkdown(data *domain.LessonData) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# TÀI LIỆU HỌC TẬP: %s\n\n", strings.ToUpper(data.LessonTitle))
	out.WriteString("Họ và tên: ............................................................\n\n")
	if data.LessonOverview != "" {
		fmt.Fprintf(&out, "> %s\n\n", data.LessonOverview)
	}
	out.WriteString("---\n\n")
	out.WriteString("# PHẦN I: LÝ THUYẾT & VÍ DỤ MINH HỌA\n\n")
	for _, section := range data.Sections {
		fmt.Fprintf(&out, "## %s\n\n", section.SectionTitle)
		for _, note := range section.StudentClozeNotes {
			if note = strings.TrimSpace(note); note != "" {
				fmt.Fprintf(&out, "- %s\n", note)
			}
		}
		if len(section.StudentClozeNotes) > 0 {
			out.WriteString("\n")
		}
		for _, example := range section.TeacherExamples {
			fmt.Fprintf(&out, "**Ví dụ %d:** %s\n\n%s\n\n", example.ExampleNum, example.Problem, example.StudentFriendlyExplanation)
			if example.CommonMistake != "" {
				fmt.Fprintf(&out, "*Lỗi thường gặp: %s*\n\n", example.CommonMistake)
			}
		}
		out.WriteString("---\n\n")
	}
	out.WriteString("# PHẦN II: BÀI TẬP TỰ LUYỆN\n\n")
	for _, exercise := range data.AIGeneratedExercises {
		writeExercise(&out, exercise, false)
	}
	return normalizeGeneratedMarkdown(out.String())
}

func writeExercise(out *strings.Builder, exercise domain.AIExercise, answers bool) {
	fmt.Fprintf(out, "**Câu %d (%s — %s):** %s\n\n", exercise.ID, exercise.Type, exercise.Difficulty, exercise.Question)
	for _, option := range exercise.Options {
		fmt.Fprintf(out, "%s\n", option)
	}
	if len(exercise.Options) > 0 {
		out.WriteString("\n")
	}
	if answers {
		fmt.Fprintf(out, "**Đáp án:** %s\n\n**Hướng dẫn:** %s\n\n---\n\n", exercise.Answer, exercise.Explanation)
		return
	}
	out.WriteString("Bài làm:\n....................................................................................................\n....................................................................................................\n....................................................................................................\n\n")
}

var internalArtifactLink = regexp.MustCompile(`(?i)\[[^\]]*(?:openai|codex|plugin|skill)[^\]]*\]\([^\)]*(?:\.codex|plugins|SKILL\.md)[^\)]*\)`)

func normalizeGeneratedMarkdown(value string) string {
	value = internalArtifactLink.ReplaceAllString(value, "")
	for strings.Contains(value, `\frac{`) {
		next := replaceLatexCommand(value, `\frac`, 2, func(args []string) string {
			return "(" + args[0] + ")/(" + args[1] + ")"
		})
		if next == value {
			break
		}
		value = next
	}
	value = replaceLatexCommand(value, `\sqrt`, 1, func(args []string) string { return "√(" + args[0] + ")" })
	for _, command := range []string{`\text`, `\mathrm`, `\mathbf`, `\overline`} {
		value = replaceLatexCommand(value, command, 1, func(args []string) string { return args[0] })
	}
	value = strings.NewReplacer(
		`\left`, "", `\right`, "", `\(`, "", `\)`, "", `\[`, "", `\]`, "", "$", "",
		`\times`, "·", `\cdot`, "·", `\pm`, "±", `\leq`, "≤", `\le`, "≤", `\geq`, "≥", `\ge`, "≥",
		`\neq`, "≠", `\in`, "∈", `\notin`, "∉", `\Rightarrow`, "⇒", `\Leftrightarrow`, "⇔",
		`\mathbb{R}`, "ℝ", `\mathbb{N}`, "ℕ", `\mathbb{Z}`, "ℤ", `\mathbb{Q}`, "ℚ",
		"^2", "²", "^3", "³",
	).Replace(value)
	return strings.TrimSpace(value)
}

func replaceLatexCommand(value, command string, argumentCount int, format func([]string) string) string {
	var output strings.Builder
	for {
		index := strings.Index(value, command)
		if index < 0 {
			output.WriteString(value)
			return output.String()
		}
		output.WriteString(value[:index])
		position := index + len(command)
		arguments := make([]string, 0, argumentCount)
		valid := true
		for len(arguments) < argumentCount {
			if position >= len(value) || value[position] != '{' {
				valid = false
				break
			}
			start, depth := position+1, 1
			position++
			for position < len(value) && depth > 0 {
				switch value[position] {
				case '{':
					depth++
				case '}':
					depth--
				}
				position++
			}
			if depth != 0 {
				valid = false
				break
			}
			arguments = append(arguments, value[start:position-1])
		}
		if !valid {
			output.WriteString(value[index : index+len(command)])
			value = value[index+len(command):]
			continue
		}
		output.WriteString(format(arguments))
		value = value[position:]
	}
}

func markdownToPDF(markdown, title string) ([]byte, error) {
	doc := fpdf.New("P", "mm", "A4", "")
	doc.SetMargins(20, 18, 20)
	doc.SetAutoPageBreak(true, 20)
	doc.AddUTF8FontFromBytes("VN", "", regular)
	doc.AddUTF8FontFromBytes("VN", "B", bold)
	doc.AddUTF8FontFromBytes("VN", "I", italic)
	doc.AddUTF8FontFromBytes("VN", "BI", boldItalic)
	if err := doc.Error(); err != nil {
		return nil, err
	}
	doc.SetTitle(clean(title), true)
	doc.AddPage()
	doc.SetFont("VN", "B", 17)
	doc.SetTextColor(25, 49, 80)
	doc.MultiCell(0, 8.5, clean(title), "", "L", false)
	doc.SetTextColor(25, 32, 43)
	doc.Ln(3)
	source := []byte(markdown)
	tree := goldmark.New().Parser().Parse(gmtext.NewReader(source))
	renderBlocks(doc, tree, source)
	if err := doc.Error(); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := doc.Output(&output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func renderBlocks(doc *fpdf.Fpdf, node ast.Node, source []byte) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch value := child.(type) {
		case *ast.Heading:
			sizes := map[int]float64{1: 16, 2: 14, 3: 12.5}
			size := sizes[value.Level]
			if size == 0 {
				size = 11.5
			}
			doc.Ln(2)
			doc.SetTextColor(25, 49, 80)
			renderInline(doc, value, source, "B", size)
			doc.Ln(6)
			doc.SetTextColor(25, 32, 43)
		case *ast.Paragraph, *ast.TextBlock:
			renderInline(doc, child, source, "", 11.5)
			doc.Ln(8)
		case *ast.Blockquote:
			doc.SetLeftMargin(26)
			doc.SetX(26)
			doc.SetTextColor(71, 85, 105)
			renderBlocks(doc, child, source)
			doc.SetLeftMargin(20)
			doc.SetX(20)
			doc.SetTextColor(25, 32, 43)
		case *ast.List:
			renderList(doc, value, source)
		case *ast.ThematicBreak:
			y := doc.GetY() + 2
			doc.SetDrawColor(210, 216, 225)
			doc.Line(20, y, 190, y)
			doc.Ln(7)
		default:
			renderBlocks(doc, child, source)
		}
	}
}

func renderList(doc *fpdf.Fpdf, list *ast.List, source []byte) {
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		doc.SetFont("VN", "", 11.5)
		doc.CellFormat(6, 6, "•", "", 0, "L", false, 0, "")
		renderInline(doc, item, source, "", 11.5)
		doc.Ln(7)
	}
	doc.Ln(2)
}

func renderInline(doc *fpdf.Fpdf, node ast.Node, source []byte, baseStyle string, size float64) {
	var walk func(ast.Node, string)
	walk = func(parent ast.Node, style string) {
		for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
			switch value := child.(type) {
			case *ast.Text:
				doc.SetFont("VN", style, size)
				doc.Write(6, clean(string(value.Segment.Value(source))))
				if value.SoftLineBreak() || value.HardLineBreak() {
					doc.Write(6, "\n")
				}
			case *ast.String:
				doc.SetFont("VN", style, size)
				doc.Write(6, clean(string(value.Value)))
			case *ast.Emphasis:
				next := combineStyle(style, "I")
				if value.Level == 2 {
					next = combineStyle(style, "B")
				}
				walk(value, next)
			default:
				walk(child, style)
			}
		}
	}
	walk(node, baseStyle)
}

func combineStyle(a, b string) string {
	hasBold := strings.Contains(a, "B") || strings.Contains(b, "B")
	hasItalic := strings.Contains(a, "I") || strings.Contains(b, "I")
	if hasBold && hasItalic {
		return "BI"
	}
	if hasBold {
		return "B"
	}
	if hasItalic {
		return "I"
	}
	return ""
}

func clean(value string) string {
	value = strings.NewReplacer("\u0305", "", "\u203e", "", "\u0304", "").Replace(value)
	var out strings.Builder
	for _, r := range value {
		if (r >= 0x2600 && r <= 0x27bf) || (r >= 0x1f000 && r <= 0x1faff) || (r >= 0xfe00 && r <= 0xfe0f) {
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
