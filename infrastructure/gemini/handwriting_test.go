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
		prompt, _ := payload.Contents[0].Parts[0]["text"].(string)
		if !strings.Contains(prompt, "Học sinh làm bài bằng chữ viết tay") {
			t.Fatalf("prompt không đánh dấu bài viết tay: %s", prompt)
		}
		if !strings.Contains(prompt, `"has_handwriting_images":true`) {
			t.Fatalf("prompt thiếu cờ ảnh viết tay: %s", prompt)
		}

		response := `{"candidates":[{"content":{"parts":[{"text":"{\"results\":[{\"exercise_id\":1,\"result\":{\"is_correct\":true,\"feedback_html\":\"<p>Đúng</p>\"}}]}"}]}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(response)),
		}, nil
	})}

	results, err := client.EvaluateBatch(context.Background(), "gemini-test", "", []application.GradingItem{{
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
