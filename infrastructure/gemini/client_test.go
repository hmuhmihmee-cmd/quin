package gemini

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPromptIncludesTeacherCustomization(t *testing.T) {
	prompt := promptWithCustomization("Thêm ba ví dụ hình học")
	if !strings.Contains(prompt, lessonPrompt) || !strings.Contains(prompt, "Thêm ba ví dụ hình học") {
		t.Fatal("prompt phải gồm cả yêu cầu hệ thống và tùy chỉnh của giáo viên")
	}
	if promptWithCustomization("  ") != lessonPrompt {
		t.Fatal("prompt trống không được thay đổi prompt hệ thống")
	}
}

func TestLessonJSONSchemaIsSentAsObject(t *testing.T) {
	if !json.Valid([]byte(lessonJSONSchema)) {
		t.Fatal("lesson JSON schema không hợp lệ")
	}
	payload, err := json.Marshal(generationConfig{
		ResponseMimeType:   "application/json",
		ResponseJSONSchema: json.RawMessage(lessonJSONSchema),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"responseJsonSchema":{"type":"object"`) {
		t.Fatalf("schema không được gửi đúng dạng JSON object: %s", payload)
	}
	if !strings.Contains(lessonJSONSchema, "student_cloze_notes") || !strings.Contains(lessonJSONSchema, "student_friendly_explanation") {
		t.Fatal("schema phải giữ ví dụ cũ và thêm riêng phần lý thuyết điền khuyết")
	}
	if !strings.Contains(lessonPrompt, "không dùng LaTeX") || !strings.Contains(lessonPrompt, "(a + b)/(c + d)") {
		t.Fatal("prompt phải bắt buộc định dạng toán thuần văn bản")
	}
}

func TestParseLessonAcceptsPreambleFenceAndRomanNumbering(t *testing.T) {
	response := "I. Đây là kết quả\n```json\n" + `{
  "lesson_title":"Mệnh đề",
  "lesson_overview":"Ôn tập",
  "sections":[{"section_title":"Khái niệm","transition_intro":"","detailed_content":"i) Ý thứ nhất; ii) Ý thứ hai","key_takeaway":"Ghi nhớ","teacher_examples":[]}],
  "ai_generated_exercises":[]
}` + "\n```"
	lesson, err := parseLesson(response)
	if err != nil {
		t.Fatal(err)
	}
	if lesson.LessonTitle != "Mệnh đề" || !strings.Contains(lesson.Sections[0].DetailedContent, "ii)") {
		t.Fatalf("lesson: %#v", lesson)
	}
}

func TestCandidateTextJoinsAllResponseParts(t *testing.T) {
	var value candidate
	value.Content.Parts = []responsePart{{Text: `{"lesson_title":"Mệnh đề",`}, {Text: `"lesson_overview":"","sections":[{"section_title":"A"}],"ai_generated_exercises":[]}`}}
	lesson, err := parseLesson(candidateText(value))
	if err != nil || lesson.LessonTitle != "Mệnh đề" {
		t.Fatalf("lesson: %#v, error: %v", lesson, err)
	}
}

func TestParseLessonRejectsTruncatedJSON(t *testing.T) {
	if _, err := parseLesson(`{"lesson_title":"Mệnh đề","sections":[`); err == nil {
		t.Fatal("JSON bị cắt phải trả lỗi")
	}
}
