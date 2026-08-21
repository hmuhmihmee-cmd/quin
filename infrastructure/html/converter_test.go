package html

import (
	"strings"
	"testing"
)

func TestExerciseMarkersAreStructuralOnlyBeforeGrading(t *testing.T) {
	document := New().MarkdownToDocument("Bài kiểm tra", `[[EXERCISE:1:START]]
**Câu 1:** Tính 1 + 1.
[[EXERCISE:1:ANSWER]]
*Bài làm:* 2
[[EXERCISE:1:FEEDBACK]]
[[EXERCISE:1:END]]`)

	for _, marker := range []string{"[[EXERCISE:1:START]]", "[[EXERCISE:1:ANSWER]]", "[[EXERCISE:1:FEEDBACK]]", "[[EXERCISE:1:END]]"} {
		if strings.Contains(document, marker) {
			t.Fatalf("marker %s không được xuất hiện trong HTML OneNote", marker)
		}
	}
	if !strings.Contains(document, `data-id="exercise-1-work"`) {
		t.Fatal("thiếu vùng bài làm có định danh")
	}
	if !strings.Contains(document, `data-id="exercise-1-region"`) {
		t.Fatal("thiếu wrapper toàn bộ câu để thu ảnh nằm ngoài ô bài làm")
	}
	if strings.Contains(document, "Nhận xét") {
		t.Fatal("trang chưa chấm không được hiện nhận xét")
	}
	if !strings.Contains(document, `data-id="exercise-1-work" width="760" border="0"`) || strings.Count(document, "<td") != 1 {
		t.Fatal("vùng làm bài ban đầu phải là bảng một ô toàn chiều rộng, không viền")
	}
}
