package application

import (
	"context"
	"strings"
	"testing"

	"meet-attendance-clean/domain"
)

type lessonGeneratorStub struct{ customPrompt string }

func (s *lessonGeneratorStub) Generate(_ context.Context, _, customPrompt string) (*domain.LessonData, error) {
	s.customPrompt = customPrompt
	return &domain.LessonData{LessonTitle: "Mệnh đề", Sections: []domain.Section{{SectionTitle: "Khái niệm"}}}, nil
}

type lessonDocumentsStub struct{}

func (lessonDocumentsStub) Draft(lesson *domain.LessonData, sourceURL string) domain.LessonDraft {
	return domain.LessonDraft{Title: lesson.LessonTitle, SourceURL: sourceURL, TeacherMarkdown: "# Giáo viên", StudentMarkdown: "# Học sinh"}
}

func (lessonDocumentsStub) GeneratePDF(title, teacherMarkdown, studentMarkdown string) (domain.LessonFiles, error) {
	return domain.LessonFiles{Title: title, TeacherPDF: []byte(teacherMarkdown), StudentPDF: []byte(studentMarkdown)}, nil
}

func TestLessonFlowUsesCustomPromptThenExportsEditedMarkdown(t *testing.T) {
	ai := &lessonGeneratorStub{}
	useCase := NewLessonUseCase(ai, lessonDocumentsStub{})
	draft, err := useCase.GenerateDraft(context.Background(), "https://youtube.com/watch?v=test", "  Thêm ví dụ thực tế  ")
	if err != nil {
		t.Fatal(err)
	}
	if ai.customPrompt != "Thêm ví dụ thực tế" || draft.TeacherMarkdown == "" || draft.StudentMarkdown == "" {
		t.Fatalf("draft: %#v, prompt: %q", draft, ai.customPrompt)
	}
	files, err := useCase.ExportPDF(draft.Title, draft.TeacherMarkdown+" đã sửa", draft.StudentMarkdown+" đã sửa")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files.TeacherPDF), "đã sửa") || !strings.Contains(string(files.StudentPDF), "đã sửa") {
		t.Fatal("PDF generator phải nhận Markdown sau chỉnh sửa")
	}
}
