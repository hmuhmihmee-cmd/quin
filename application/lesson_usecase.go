package application

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"meet-attendance-clean/domain"
)

type LessonUseCase struct {
	ai        LessonGenerator
	documents LessonDocumentGenerator
}

func NewLessonUseCase(ai LessonGenerator, documents LessonDocumentGenerator) *LessonUseCase {
	return &LessonUseCase{ai: ai, documents: documents}
}

func (u *LessonUseCase) GenerateDraft(ctx context.Context, youtubeURL, customPrompt string) (domain.LessonDraft, error) {
	youtubeURL = strings.TrimSpace(youtubeURL)
	parsed, err := url.ParseRequestURI(youtubeURL)
	host := ""
	if parsed != nil {
		host = strings.ToLower(parsed.Hostname())
	}
	validScheme := parsed != nil && (parsed.Scheme == "https" || parsed.Scheme == "http")
	validHost := host == "youtube.com" || strings.HasSuffix(host, ".youtube.com") || host == "youtu.be"
	if err != nil || !validScheme || !validHost {
		return domain.LessonDraft{}, errors.New("đường dẫn YouTube không hợp lệ")
	}
	customPrompt = strings.TrimSpace(customPrompt)
	if len(customPrompt) > 10000 {
		return domain.LessonDraft{}, errors.New("yêu cầu tùy chỉnh không được vượt quá 10.000 ký tự")
	}
	lesson, err := u.ai.Generate(ctx, youtubeURL, customPrompt)
	if err != nil {
		return domain.LessonDraft{}, err
	}
	return u.documents.Draft(lesson, youtubeURL), nil
}

func (u *LessonUseCase) ExportPDF(title, teacherMarkdown, studentMarkdown string) (domain.LessonFiles, error) {
	title = strings.TrimSpace(title)
	teacherMarkdown = strings.TrimSpace(teacherMarkdown)
	studentMarkdown = strings.TrimSpace(studentMarkdown)
	if title == "" || teacherMarkdown == "" || studentMarkdown == "" {
		return domain.LessonFiles{}, errors.New("hai bản Markdown không được để trống")
	}
	return u.documents.GeneratePDF(title, teacherMarkdown, studentMarkdown)
}
