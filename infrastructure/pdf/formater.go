package pdf

import (
	"fmt"
	"strings"

	"meet-attendance-clean/domain"
)

type Formatter struct{}

func NewFormatter() *Formatter {
	return &Formatter{}
}

// Implement application.DocumentFormatter
func (f *Formatter) Format(lesson *domain.Lesson, target domain.Audience, sourceURL string) string {
	if target == domain.AudienceStudent {
		return f.buildStudentMarkdown(lesson)
	}
	return f.buildTeacherMarkdown(lesson, sourceURL)
}

// Bản Giáo viên: Đầy đủ lời giải, mẹo giảng bài, đáp án chi tiết
func (f *Formatter) buildTeacherMarkdown(data *domain.Lesson, sourceURL string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# GIÁO ÁN: %s\n\n", strings.ToUpper(data.Title)))
	if sourceURL != "" {
		sb.WriteString(fmt.Sprintf("> Nguồn video bài giảng: %s\n\n", sourceURL))
	}
	if data.Overview != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", data.Overview))
	}
	sb.WriteString("---\n\n## PHẦN 1: NỘI DUNG BÀI GIẢNG & VÍ DỤ MINH HỌA\n\n")

	for _, sec := range data.Sections {
		sb.WriteString(fmt.Sprintf("### %s\n\n", sec.SectionTitle))
		if sec.TransitionIntro != "" {
			sb.WriteString(fmt.Sprintf("*Lời dẫn giảng:* %s\n\n", sec.TransitionIntro))
		}
		sb.WriteString(fmt.Sprintf("%s\n\n", sec.DetailedContent))

		for _, ex := range sec.TeacherExamples {
			sb.WriteString(fmt.Sprintf("**Ví dụ %d:** %s\n\n", ex.ExampleNum, ex.Problem))
			sb.WriteString(fmt.Sprintf("**Lời giải giáo viên:**\n%s\n\n", ex.TeacherSolution))
			if ex.CommonMistake != "" {
				sb.WriteString(fmt.Sprintf("*Lưu ý học sinh hay sai:* %s\n\n", ex.CommonMistake))
			}
		}

		if sec.KeyTakeaway != "" {
			sb.WriteString(fmt.Sprintf("**Trọng tâm cần nhớ:** %s\n\n", sec.KeyTakeaway))
		}
		sb.WriteString("---\n\n")
	}

	sb.WriteString("## PHẦN 2: BÀI TẬP VẬN DỤNG & ĐÁP ÁN\n\n")
	for _, ex := range data.Exercises {
		sb.WriteString(fmt.Sprintf("**Câu %d (%s - %s):** %s\n\n", ex.ID, ex.Type, ex.Difficulty, ex.Question))
		for _, opt := range ex.Options {
			sb.WriteString(fmt.Sprintf("- %s\n", opt))
		}
		if len(ex.Options) > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("**Đáp án:** %s\n\n**Hướng dẫn giải:** %s\n\n---\n\n", ex.Answer, ex.Explanation))
	}

	return sb.String()
}

// Bản Học sinh: Điền vào chỗ trống (cloze notes), không lộ đáp án
func (f *Formatter) buildStudentMarkdown(data *domain.Lesson) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# PHIẾU HỌC TẬP: %s\n\n", strings.ToUpper(data.Title)))
	sb.WriteString("Họ và tên học sinh: ............................................................ Lớp: ..............\n\n")
	if data.Overview != "" {
		sb.WriteString(fmt.Sprintf("> %s\n\n", data.Overview))
	}
	sb.WriteString("---\n\n## PHẦN 1: LÝ THUYẾT & VÍ DỤ TRÊN LỚP\n\n")

	for _, sec := range data.Sections {
		sb.WriteString(fmt.Sprintf("### %s\n\n", sec.SectionTitle))

		// Các câu đục lỗ để học sinh nghe giảng và điền vào
		for _, note := range sec.StudentClozeNotes {
			if strings.TrimSpace(note) != "" {
				sb.WriteString(fmt.Sprintf("- %s\n", note))
			}
		}
		sb.WriteString("\n")

		for _, ex := range sec.TeacherExamples {
			sb.WriteString(fmt.Sprintf("**Ví dụ %d:** %s\n\n", ex.ExampleNum, ex.Problem))
			if ex.StudentFriendlyExplanation != "" {
				sb.WriteString(fmt.Sprintf("*Gợi ý suy nghĩ:* %s\n\n", ex.StudentFriendlyExplanation))
			}
			sb.WriteString("Bài làm:\n....................................................................................................\n....................................................................................................\n\n")
		}
		sb.WriteString("---\n\n")
	}

	sb.WriteString("## PHẦN 2: BÀI TẬP TỰ LUYỆN\n\n")
	for _, ex := range data.Exercises {
		sb.WriteString(fmt.Sprintf("**Câu %d (%s - %s):** %s\n\n", ex.ID, ex.Type, ex.Difficulty, ex.Question))
		for _, opt := range ex.Options {
			sb.WriteString(fmt.Sprintf("- %s\n", opt))
		}
		sb.WriteString("\n*Bài làm:*\n....................................................................................................\n....................................................................................................\n\n")
	}

	return sb.String()
}