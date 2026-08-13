package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"meet-attendance-clean/domain"
)

func TestRendererHasStudentSummaryAndDetailViews(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	for view, expected := range map[string]string{
		"students":      "Thống kê theo học sinh",
		"student":       "Các buổi học",
		"lessons":       "Tạo tài liệu từ video",
		"lesson_editor": "Bản nháp Markdown",
	} {
		var output bytes.Buffer
		if err := renderer.Render(&output, ViewData{View: view}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("view %s does not contain %q", view, expected)
		}
	}
}

func TestEmbeddedFrontendAssets(t *testing.T) {
	for path, expected := range map[string]string{
		"/assets/htmx.min.js":               "htmx",
		"/assets/toastui-editor-all.min.js": "Editor",
		"/assets/toastui-editor.min.css":    ".toastui-editor-defaultUI",
		"/assets/lesson-editor.js":          "toastui.Editor",
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			assetsHandler().ServeHTTP(response, request)
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), expected) {
				t.Fatalf("asset %s response: status=%d", path, response.Code)
			}
		})
	}
}

func TestStudentSpaceIsReadOnlyAndStudentsAreNotCreatedManually(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	var detail bytes.Buffer
	err = renderer.Render(&detail, ViewData{
		View:    "student",
		Student: domain.Student{ID: 1, Name: "Minh Anh", GoogleSpaceName: "space-1", CycleStartDay: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(detail.String(), `name="google_space_name"`) || !strings.Contains(detail.String(), `value="space-1" readonly`) {
		t.Fatal("Space ID phải hiển thị readonly và không được gửi trong form")
	}
	if strings.Contains(detail.String(), `/delete`) {
		t.Fatal("giao diện không được phép xóa student được quản lý theo Space")
	}

	var students bytes.Buffer
	if err := renderer.Render(&students, ViewData{View: "students"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(students.String(), "Thêm học sinh") {
		t.Fatal("student phải được tạo tự động khi đồng bộ")
	}
}

func TestBothAttendanceDashboardsShowDurationFilter(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range []string{"students", "student"} {
		var output bytes.Buffer
		if err := renderer.Render(&output, ViewData{View: view, MinDurationMinutes: 30}); err != nil {
			t.Fatal(err)
		}
		body := output.String()
		if !strings.Contains(body, `name="min_duration"`) || !strings.Contains(body, `value="30"`) {
			t.Fatalf("view %s is missing the 30-minute filter", view)
		}
	}
}
