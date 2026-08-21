package onenote

import (
	"bytes"
	"context"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"meet-attendance-clean/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestParseInkMLTracesIncludesNestedNamespacedTraces(t *testing.T) {
	inkML := []byte(`<?xml version="1.0"?>
		<ink:ink xmlns:ink="http://www.w3.org/2003/InkML">
			<ink:traceGroup>
				<ink:trace>10 20, 15 30, 25 35</ink:trace>
			</ink:traceGroup>
		</ink:ink>`)

	traces, err := parseInkMLTraces(inkML)
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) != 1 || len(traces[0].Points) != 3 {
		t.Fatalf("parse trace không đúng: %#v", traces)
	}
	if traces[0].MinX != 10 || traces[0].MinY != 20 || traces[0].MaxX != 25 || traces[0].MaxY != 35 {
		t.Fatalf("bounding box không đúng: %#v", traces[0])
	}
}

func TestParseInkTracePointsWithoutCommaOrWithNewlines(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "cách nhau bằng khoảng trắng", value: "100 200 105 202 110 205"},
		{name: "cách nhau bằng xuống dòng", value: "100 200\n105 202\n110 205"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			points := parseInkTracePoints(test.value)
			if len(points) != 3 || points[2] != (inkPoint{X: 110, Y: 205}) {
				t.Fatalf("tọa độ parse không đúng: %#v", points)
			}
		})
	}
}

func TestParseInkMLUsesTraceFormatForPressureChannel(t *testing.T) {
	inkML := []byte(`<ink xmlns="http://www.w3.org/2003/InkML">
		<definitions><traceFormat>
			<channel name="X"/><channel name="Y"/><channel name="F"/>
		</traceFormat></definitions>
		<trace>100 200 0.4 105 202 0.5 110 205 0.6</trace>
	</ink>`)

	traces, err := parseInkMLTraces(inkML)
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) != 1 || len(traces[0].Points) != 3 {
		t.Fatalf("trace X Y F bị đọc lệch: %#v", traces)
	}
	if traces[0].Points[2] != (inkPoint{X: 110, Y: 205}) {
		t.Fatalf("tọa độ cuối không đúng: %#v", traces[0].Points)
	}
}

func TestSplitAndRenderInkPerExerciseUsesYAxisBoundaries(t *testing.T) {
	// Trace đầu là một đường thẳng đứng (MinX == MaxX), đúng trường hợp nét
	// bút ngắn trong ảnh thực tế và vẫn phải render được.
	inkML := []byte(`<ink xmlns="http://www.w3.org/2003/InkML">
		<trace>300 150, 300 170, 300 195</trace>
		<trace>200 650, 240 675, 280 660</trace>
	</ink>`)
	boundaries := []inkExerciseBoundary{
		{ExerciseID: 5, Y: 100},
		{ExerciseID: 6, Y: 600},
	}

	images, err := splitAndRenderInkPerExercise(inkML, boundaries, []int{5, 6})
	if err != nil {
		t.Fatal(err)
	}
	for _, exerciseID := range []int{5, 6} {
		data := images[exerciseID]
		if len(data) == 0 {
			t.Fatalf("câu %d không có ảnh Ink", exerciseID)
		}
		if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
			t.Fatalf("ảnh Ink câu %d không phải JPEG hợp lệ: %v", exerciseID, err)
		}
	}
}

func TestSplitAndRenderInkWithoutBoundariesPreservesInkForEveryExercise(t *testing.T) {
	inkML := []byte(`<ink><trace>100 100, 120 120, 140 115</trace></ink>`)
	images, err := splitAndRenderInkPerExercise(inkML, nil, []int{5, 6, 7})
	if err != nil {
		t.Fatal(err)
	}
	for _, exerciseID := range []int{5, 6, 7} {
		if len(images[exerciseID]) == 0 {
			t.Fatalf("câu %d bị mất ảnh Ink toàn trang", exerciseID)
		}
		if !bytes.Equal(images[5], images[exerciseID]) {
			t.Fatalf("câu %d không nhận đúng ảnh Ink toàn trang", exerciseID)
		}
	}
}

func TestExtractInkExerciseBoundariesConvertsCSSPixelsToHimetric(t *testing.T) {
	page := `<html><body>
		<div style="position:absolute;top:100px"><div data-id="exercise-5-region"></div></div>
		<div style="position:absolute;top:600px"><div data-id="exercise-6-region"></div></div>
	</body></html>`

	boundaries := extractInkExerciseBoundaries(page, []int{5, 6})
	if len(boundaries) != 2 {
		t.Fatalf("số boundary không đúng: %#v", boundaries)
	}
	if boundaries[0].ExerciseID != 5 || boundaries[0].Y < 2645 || boundaries[0].Y > 2646 {
		t.Fatalf("boundary câu 5 không đúng: %#v", boundaries[0])
	}
	if boundaries[1].ExerciseID != 6 || boundaries[1].Y != 15875 {
		t.Fatalf("boundary câu 6 không đúng: %#v", boundaries[1])
	}
}

func TestRequestPageContentReadsMultipartHTMLAndInkML(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	htmlPart, err := writer.CreateFormField("presentation")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = htmlPart.Write([]byte(`<html><body>lesson</body></html>`))
	inkPart, err := writer.CreateFormField("presentation-onenote-inkml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = inkPart.Write([]byte(`<ink><trace>1 2, 3 4</trace></ink>`))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	responseBody := append([]byte(nil), body.Bytes()...)
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{writer.FormDataContentType()}},
			Body:       io.NopCloser(bytes.NewReader(responseBody)),
		}, nil
	})}

	htmlContent, inkContent, err := requestPageContent(context.Background(), client, "https://example.test/content")
	if err != nil {
		t.Fatal(err)
	}
	if htmlContent != `<html><body>lesson</body></html>` {
		t.Fatalf("HTML multipart không đúng: %q", htmlContent)
	}
	if string(inkContent) != `<ink><trace>1 2, 3 4</trace></ink>` {
		t.Fatalf("InkML multipart không đúng: %q", inkContent)
	}
}

func TestRequestPageContentRecognizesTextXMLInkPart(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	inkPart, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": []string{`form-data; name="drawing"`},
		"Content-Type":        []string{"text/xml"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = inkPart.Write([]byte(`<ink><trace>1 2, 3 4</trace></ink>`))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{writer.FormDataContentType()}},
			Body:       io.NopCloser(bytes.NewReader(body.Bytes())),
		}, nil
	})}
	_, inkContent, err := requestPageContent(context.Background(), client, "https://example.test/content")
	if err != nil {
		t.Fatal(err)
	}
	if len(inkContent) == 0 {
		t.Fatal("multipart text/xml không được nhận diện là InkML")
	}
}

func TestFetchPageHTMLAndInkUsesMicrosoftGraphBetaFallback(t *testing.T) {
	requests := make([]string, 0, 2)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.URL.String())
		if strings.Contains(request.URL.Path, "/beta/") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/inkml+xml"}},
				Body:       io.NopCloser(strings.NewReader(`<ink><trace>1 2, 3 4</trace></ink>`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body:       io.NopCloser(strings.NewReader(`<html><body>lesson</body></html>`)),
		}, nil
	})}

	htmlContent, inkContent, err := (&Client{}).fetchPageHTMLAndInk(context.Background(), client, "page-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(htmlContent, "lesson") || len(inkContent) == 0 {
		t.Fatalf("fallback không giữ đủ HTML và InkML: html=%q ink=%q", htmlContent, inkContent)
	}
	if len(requests) != 2 || !strings.Contains(requests[0], "graph.microsoft.com/beta/me/onenote") {
		t.Fatalf("không gọi đúng Graph beta fallback: %#v", requests)
	}
}

func TestPopulateStudentAnswersHandlesTextInkCombinedAndBlank(t *testing.T) {
	extractedAt := time.Unix(200, 0).UTC()
	embeddedImage := []byte("embedded-image")
	inkImage := []byte("rendered-ink-image")
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Hostname() != "graph.microsoft.com" {
			t.Fatalf("tải ảnh từ host ngoài dự kiến: %s", request.URL.Hostname())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(embeddedImage)),
		}, nil
	})}

	tests := []struct {
		name       string
		content    string
		inkImages  map[int][]byte
		wantText   string
		wantImages int
		wantBlank  bool
	}{
		{
			name:      "chỉ có chữ gõ",
			content:   `<div data-id="exercise-1-work"><p><em>Bài làm:</em></p><p>x = 2</p></div>`,
			wantText:  "x = 2",
			wantBlank: false,
		},
		{
			name:       "chỉ có nét bút",
			content:    `<html><body></body></html>`,
			inkImages:  map[int][]byte{1: inkImage},
			wantImages: 1,
			wantBlank:  false,
		},
		{
			name: "kết hợp chữ ảnh nhúng và nét bút",
			content: `<div data-id="exercise-1-region"><div data-id="exercise-1-work">` +
				`<p><em>Bài làm:</em></p><p>x = 2</p><img src="https://graph.microsoft.com/onenote/image/1"></div></div>`,
			inkImages:  map[int][]byte{1: inkImage},
			wantText:   "x = 2",
			wantImages: 2,
			wantBlank:  false,
		},
		{
			name:      "bỏ trống",
			content:   `<div data-id="exercise-1-work"><p><em>Bài làm:</em></p></div>`,
			wantBlank: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assignment := &domain.Assignment{Items: []domain.AssignedExercise{{Exercise: domain.Exercise{ID: 1}}}}
			gateway := &Client{}
			if err := gateway.populateStudentAnswers(context.Background(), client, assignment, test.content, test.inkImages, extractedAt); err != nil {
				t.Fatal(err)
			}
			answer := assignment.Items[0].StudentAnswer
			if answer == nil {
				t.Fatal("StudentAnswer không được nil")
			}
			if !strings.Contains(answer.Text, test.wantText) {
				t.Fatalf("text không đúng: %q", answer.Text)
			}
			if len(answer.Images) != test.wantImages {
				t.Fatalf("số ảnh không đúng: got %d, want %d", len(answer.Images), test.wantImages)
			}
			if answer.IsBlank != test.wantBlank {
				t.Fatalf("IsBlank không đúng: got %t, want %t", answer.IsBlank, test.wantBlank)
			}
			if !answer.ExtractedAt.Equal(extractedAt) {
				t.Fatalf("ExtractedAt không đúng: %s", answer.ExtractedAt)
			}
		})
	}
}
