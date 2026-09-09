package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"meet-attendance-clean/application"
	"meet-attendance-clean/domain"
)

type geminiRoundTripFunc func(*http.Request) (*http.Response, error)

func (function geminiRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestEvaluateBatchMarksHandwritingOnlyAnswer(t *testing.T) {
	client := New("test-key", "")
	client.httpClient = &http.Client{Transport: geminiRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Contents) == 0 || len(payload.Contents[0].Parts) == 0 {
			t.Fatal("Gemini payload thiếu prompt")
		}
		var textParts strings.Builder
		for _, part := range payload.Contents[0].Parts {
			if text, ok := part["text"].(string); ok {
				textParts.WriteString(text)
			}
		}
		payloadText := textParts.String()
		if !strings.Contains(payloadText, "Phần trình bày") {
			t.Fatalf("prompt thiếu quy tắc nhận xét phần trình bày: %s", payloadText)
		}
		if !strings.Contains(payloadText, `"has_pasted_images":true`) {
			t.Fatalf("payload thiếu cờ ảnh viết tay: %s", payloadText)
		}

		response := `{"candidates":[{"content":{"parts":[{"text":"{\"results\":[{\"exercise_id\":1,\"result\":{\"is_correct\":true,\"feedback_html\":\"<p>Đúng</p>\"}}]}"}]}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(response)),
		}, nil
	})}

	results, err := client.EvaluateBatch(context.Background(), "gemini-test", "", nil, []application.GradingItem{{
		Exercise: domain.Exercise{ID: 1, Question: "1 + 1 = ?", Answer: "2"},
		Answer: domain.StudentAnswer{
			Images: [][]byte{{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ExerciseID != 1 {
		t.Fatalf("kết quả chấm không đúng: %#v", results)
	}
}

func TestGenerateRemediationSendsStrictLessonJSONSchema(t *testing.T) {
	client := New("test-key", "")
	client.httpClient = &http.Client{Transport: geminiRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
			GenerationConfig map[string]any `json:"generationConfig"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		schema, ok := payload.GenerationConfig["responseJsonSchema"].(map[string]any)
		if !ok {
			t.Fatal("request tạo bài khắc phục thiếu responseJsonSchema")
		}
		properties := schema["properties"].(map[string]any)
		sections := properties["sections"].(map[string]any)
		if sections["type"] != "array" {
			t.Fatalf("sections phải bị ép kiểu array, nhận %#v", sections)
		}
		var prompt strings.Builder
		for _, part := range payload.Contents[0].Parts {
			if text, ok := part["text"].(string); ok {
				prompt.WriteString(text)
			}
		}
		if !strings.Contains(prompt.String(), `"sections" BẮT BUỘC là mảng JSON`) {
			t.Fatal("prompt chưa nêu rõ sections không được là chuỗi")
		}

		response := `{"candidates":[{"content":{"parts":[{"text":"{\"title\":\"Khắc phục\",\"overview\":\"Không dùng\",\"sections\":[{\"section_title\":\"Ví dụ\",\"transition_intro\":\"\",\"detailed_content\":\"\",\"key_takeaway\":\"\",\"student_cloze_notes\":[],\"teacher_examples\":[]}],\"exercises\":[{\"id\":1,\"type\":\"Trắc nghiệm\",\"topic\":\"Hàm số\",\"difficulty\":\"Thông hiểu\",\"question\":\"Q\",\"options\":[\"A. 1\",\"B. 2\"],\"answer\":\"1\",\"explanation\":\"Lời giải\"}]}"}]}}]}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(response))}, nil
	})}

	lesson, err := client.GenerateRemediation(context.Background(), "gemini-test", "", application.MistakeContext{}, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if lesson.Title != "Khắc phục" || len(lesson.Exercises) != 1 {
		t.Fatalf("bài khắc phục parse sai: %#v", lesson)
	}
	if lesson.Overview != "" || len(lesson.Sections) != 0 || lesson.Exercises[0].Type != "Tự luận" || len(lesson.Exercises[0].Options) != 0 {
		t.Fatalf("bài khắc phục phải chỉ còn câu tự làm, nhận %#v", lesson)
	}
}
