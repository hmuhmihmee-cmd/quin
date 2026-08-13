package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"meet-attendance-clean/application"
	"meet-attendance-clean/domain"
	webview "meet-attendance-clean/infrastructure/web"
)

type lessonDocumentsStub struct{}

func (lessonDocumentsStub) Draft(*domain.LessonData, string) domain.LessonDraft {
	return domain.LessonDraft{}
}

func (lessonDocumentsStub) GeneratePDF(title, _, _ string) (domain.LessonFiles, error) {
	return domain.LessonFiles{Title: title, TeacherPDF: []byte("teacher-pdf"), StudentPDF: []byte("student-pdf")}, nil
}

func TestExportReturnsTwoSeparatePDFs(t *testing.T) {
	views, err := webview.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	useCase := application.NewLessonUseCase(nil, lessonDocumentsStub{})
	handler := NewGenerateLesson(useCase, views)
	form := url.Values{
		"title":            {"Bai giang"},
		"teacher_markdown": {"Noi dung giao vien"},
		"student_markdown": {"Noi dung hoc sinh"},
	}
	request := httptest.NewRequest(http.MethodPost, "/lessons/export", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.Export(response, request)

	if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("response status=%d content-type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	var result struct {
		TeacherFilename string `json:"teacher_filename"`
		StudentFilename string `json:"student_filename"`
		TeacherPDF      []byte `json:"teacher_pdf"`
		StudentPDF      []byte `json:"student_pdf"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.TeacherFilename != "bai-giang-giao-vien.pdf" || result.StudentFilename != "bai-giang-hoc-sinh.pdf" {
		t.Fatalf("filenames: %#v", result)
	}
	if string(result.TeacherPDF) != "teacher-pdf" || string(result.StudentPDF) != "student-pdf" {
		t.Fatalf("PDF payloads: %#v", result)
	}
}
