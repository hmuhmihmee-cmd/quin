package onenote

import (
	"context"
	"strings"
	"testing"
	"time"

	"meet-attendance-clean/domain"
)

func TestExtractAnswerBlockFromStructuredWorkRegion(t *testing.T) {
	page := `<html><body><div data-id="exercise-4-work"><p><em>Bài làm:</em></p><p>x = 2</p></div></body></html>`
	answer, found := extractAnswerBlock(page, 4)
	if !found || !strings.Contains(answer, "x = 2") {
		t.Fatalf("không lấy được bài làm trong vùng cấu trúc: %q", answer)
	}
	if strings.Contains(answer, "EXERCISE") {
		t.Fatal("bài làm không được chứa marker nội bộ")
	}
}

func TestParseStudentAnswerReturnsPlainText(t *testing.T) {
	client := &Client{}
	answer, err := client.parseStudentAnswer(
		context.Background(),
		nil,
		`<p><em>Bài làm:</em></p><p>x = 2</p>`,
		time.Unix(100, 0).UTC(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if answer.IsBlank || !strings.Contains(answer.Text, "x = 2") || len(answer.Images) != 0 {
		t.Fatalf("bài làm parse không đúng: %#v", answer)
	}
}

func TestExtractAnswerBlockAfterFeedbackWasInserted(t *testing.T) {
	page := `<html><body><div data-id="exercise-5-work"><table><tr><td><div data-id="exercise-5-answer-content"><p>Bài làm: 12</p></div></td><td><div data-id="exercise-5-feedback"><p>Đúng</p></div></td></tr></table></div></body></html>`
	answer, found := extractAnswerBlock(page, 5)
	if !found || !strings.Contains(answer, "12") {
		t.Fatalf("không lấy được bài làm sau khi chấm: %q", answer)
	}
	if strings.Contains(answer, "Đúng") {
		t.Fatal("không được gửi nhận xét cũ trở lại Gemini")
	}
}

func TestFindGeneratedIDForReplaceTarget(t *testing.T) {
	page := `<html><body><table data-id="exercise-4-work" id="table:{page-guid}{42}"><tr><td>Bài làm</td></tr></table></body></html>`
	generatedID, found := findGeneratedIDByDataID(page, "exercise-4-work")
	if !found || generatedID != "table:{page-guid}{42}" {
		t.Fatalf("generated ID không đúng: %q", generatedID)
	}
}

func TestFindGeneratedTableIDsForLegacyPages(t *testing.T) {
	page := `<html><body><table id="table:{page}{10}"><tr><td>Câu 1</td></tr></table><table id="table:{page}{20}"><tr><td>Câu 2</td></tr></table></body></html>`
	targets := findGeneratedTableIDs(page)
	if len(targets) != 2 || targets[0] != "table:{page}{10}" || targets[1] != "table:{page}{20}" {
		t.Fatalf("danh sách target bảng cũ không đúng: %#v", targets)
	}
}

func TestBlankExerciseKeepsOneColumnWithoutFeedback(t *testing.T) {
	html := buildWorkTable("4", "<p>Bài làm:</p>", &domain.GradingResult{Status: domain.AIExerciseSkippedEmpty})
	if strings.Contains(html, "Nhận xét") || strings.Contains(html, "Chưa làm") {
		t.Fatalf("câu trống không được hiện nhận xét: %s", html)
	}
	if strings.Count(html, "<td") != 1 || !strings.Contains(html, `width="760"`) {
		t.Fatalf("câu trống phải giữ một cột toàn chiều rộng: %s", html)
	}
}

func TestGradedExerciseHasAnswerAndFeedbackColumns(t *testing.T) {
	html := buildWorkTable("1", "<p>A</p>", &domain.GradingResult{Status: domain.AIExerciseGraded, IsCorrect: true, FeedbackHTML: "<p>Cách giải.</p>"})
	if strings.Count(html, "<td") != 2 || !strings.Contains(html, "Nhận xét") || !strings.Contains(html, "Cách giải") {
		t.Fatalf("câu đã chấm phải có hai cột và nhận xét: %s", html)
	}
}

func TestSkippedEmptyExerciseIsNotPatched(t *testing.T) {
	page := `<html><body><table data-id="exercise-5-work" id="table:{page}{5}"><tr><td><p>Bài làm:</p></td></tr></table></body></html>`
	assignment := &domain.Assignment{Items: []domain.AssignedExercise{{
		Exercise: domain.Exercise{ID: 5},
		Result:   &domain.GradingResult{Status: domain.AIExerciseSkippedEmpty},
	}}}
	commands, err := buildFeedbackPatchCommands(page, assignment)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 0 {
		t.Fatalf("câu chưa làm không được PATCH replace: %#v", commands)
	}
}
