package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"meet-attendance-clean/application"
	webview "meet-attendance-clean/infrastructure/web"
)

type GenerateLesson struct {
	useCase *application.LessonUseCase
	views   *webview.Renderer
}

func NewGenerateLesson(useCase *application.LessonUseCase, views *webview.Renderer) *GenerateLesson {
	return &GenerateLesson{useCase: useCase, views: views}
}

func (h *GenerateLesson) Page(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.views.Render(writer, webview.ViewData{View: "lessons", Notice: request.URL.Query().Get("notice"), Error: request.URL.Query().Get("error")}); err != nil {
		renderError(writer, err)
	}
}

func (h *GenerateLesson) Generate(writer http.ResponseWriter, request *http.Request) {
	draft, err := h.useCase.GenerateDraft(request.Context(), request.FormValue("youtube_url"), request.FormValue("custom_prompt"))
	if err != nil {
		redirectLessonError(writer, request, err)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.views.Render(writer, webview.ViewData{View: "lesson_editor", LessonDraft: draft}); err != nil {
		renderError(writer, err)
	}
}

func (h *GenerateLesson) Export(writer http.ResponseWriter, request *http.Request) {
	files, err := h.useCase.ExportPDF(request.FormValue("title"), request.FormValue("teacher_markdown"), request.FormValue("student_markdown"))
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if err != nil {
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(writer).Encode(map[string]string{"error": err.Error()})
		return
	}
	filename := slug(files.Title)
	if filename == "" {
		filename = "bai-giang"
	}
	_ = json.NewEncoder(writer).Encode(struct {
		TeacherFilename string `json:"teacher_filename"`
		StudentFilename string `json:"student_filename"`
		TeacherPDF      []byte `json:"teacher_pdf"`
		StudentPDF      []byte `json:"student_pdf"`
	}{
		TeacherFilename: filename + "-giao-vien.pdf",
		StudentFilename: filename + "-hoc-sinh.pdf",
		TeacherPDF:      files.TeacherPDF,
		StudentPDF:      files.StudentPDF,
	})
}

var unsafeFilename = regexp.MustCompile(`[^a-zA-Z0-9-]+`)

func slug(value string) string {
	return strings.Trim(unsafeFilename.ReplaceAllString(strings.ToLower(value), "-"), "-")
}
