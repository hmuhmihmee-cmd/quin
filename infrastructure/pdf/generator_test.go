package pdf

import (
	"bytes"
	"strings"
	"testing"

	"meet-attendance-clean/domain"
)

func TestGenerateCreatesTwoPDFs(t *testing.T) {
	lesson := &domain.LessonData{
		LessonTitle:          "Mệnh đề Toán học",
		LessonOverview:       "Ôn tập mệnh đề và ký hiệu ∀, ∃.",
		Sections:             []domain.Section{{SectionTitle: "I. Mệnh đề", DetailedContent: "Một mệnh đề có giá trị đúng hoặc sai.", KeyTakeaway: "Xác định đúng giá trị chân lý."}},
		AIGeneratedExercises: []domain.AIExercise{{ID: 1, Type: "Tự luận", Difficulty: "Thông hiểu", Question: "Phủ định mệnh đề P.", Answer: "¬P", Explanation: "Dùng ký hiệu phủ định."}},
	}
	generator := New()
	draft := generator.Draft(lesson, "https://youtube.com/watch?v=test")
	if draft.TeacherMarkdown == "" || draft.StudentMarkdown == "" {
		t.Fatal("Markdown drafts must not be empty")
	}
	files, err := generator.GeneratePDF(draft.Title, draft.TeacherMarkdown, draft.StudentMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string][]byte{"teacher": files.TeacherPDF, "student": files.StudentPDF} {
		if len(contents) < 1000 || !bytes.HasPrefix(contents, []byte("%PDF")) {
			t.Fatalf("%s PDF is invalid", name)
		}
	}
}

func TestStudentDraftOnlyReplacesLongTheoryWithSmartClozeNotes(t *testing.T) {
	lesson := &domain.LessonData{
		LessonTitle:    "Phân số",
		LessonOverview: "Ôn tập phép cộng phân số.",
		Sections: []domain.Section{{
			SectionTitle: "Quy tắc", TransitionIntro: "Lời dẫn dài", DetailedContent: "Nội dung giảng chi tiết.",
			KeyTakeaway:       "Muốn cộng hai phân số, quy đồng mẫu số.",
			StudentClozeNotes: []string{"Muốn cộng hai phân số khác mẫu, trước tiên phải .................... mẫu số."},
			TeacherExamples: []domain.TeacherExample{{
				ExampleNum: 1, Problem: "Tính 1/2 + 1/3.", TeacherSolution: "Đáp án 5/6.",
				StudentFriendlyExplanation: "Quy đồng rồi cộng hai tử số.", CommonMistake: "Không cộng hai mẫu.",
			}},
		}},
		AIGeneratedExercises: []domain.AIExercise{{ID: 1, Type: "Tự luận", Difficulty: "Thông hiểu", Question: "Tính 2/3 + 1/3.", Answer: "1", Explanation: "Cộng tử số."}},
	}
	draft := New().Draft(lesson, "https://youtube.com/watch?v=test")
	for _, unwanted := range []string{"Lời dẫn dài", "Nội dung giảng chi tiết", "Muốn cộng hai phân số, quy đồng mẫu số.", "Đáp án 5/6.", "**Đáp án:** 1"} {
		if strings.Contains(draft.StudentMarkdown, unwanted) {
			t.Fatalf("student draft must not contain %q:\n%s", unwanted, draft.StudentMarkdown)
		}
	}
	for _, wanted := range []string{
		"Ôn tập phép cộng phân số.", "# PHẦN I: LÝ THUYẾT & VÍ DỤ MINH HỌA",
		"trước tiên phải .................... mẫu số", "**Ví dụ 1:** Tính 1/2 + 1/3.",
		"Quy đồng rồi cộng hai tử số.", "*Lỗi thường gặp: Không cộng hai mẫu.*",
		"# PHẦN II: BÀI TẬP TỰ LUYỆN", "**Câu 1 (Tự luận — Thông hiểu):**", "Bài làm:",
	} {
		if !strings.Contains(draft.StudentMarkdown, wanted) {
			t.Fatalf("student draft is missing %q:\n%s", wanted, draft.StudentMarkdown)
		}
	}
	for _, teacherOnly := range []string{"Nội dung giảng chi tiết.", "Đáp án 5/6.", "**Chốt lại:** Muốn cộng hai phân số, quy đồng mẫu số."} {
		if !strings.Contains(draft.TeacherMarkdown, teacherOnly) {
			t.Fatalf("teacher draft changed or is missing %q:\n%s", teacherOnly, draft.TeacherMarkdown)
		}
	}
}

func TestGeneratedMarkdownNormalizesMathAndRemovesInternalArtifactLinks(t *testing.T) {
	input := `Tính $\frac{x+1}{\sqrt{4}} \leq 3$ [$openai-templates:x](/home/user/.codex/plugins/x/SKILL.md)`
	got := normalizeGeneratedMarkdown(input)
	if got != "Tính (x+1)/(√(4)) ≤ 3" {
		t.Fatalf("normalized markdown = %q", got)
	}
}
