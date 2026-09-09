package html

import (
	"strings"
	"testing"

	"meet-attendance-clean/domain"
)

func TestStudentExerciseHasCheckboxesAndStableAnswerLayout(t *testing.T) {
	document := NewLessonRenderer().RenderLessonHTML("Bài kiểm tra", &domain.Lesson{
		Exercises: []domain.Exercise{{
			ID:       1,
			Type:     "Trắc nghiệm",
			Question: "Tính 1 + 1.",
			Options:  []string{"A. 1", "B. 2", "C. 3", "D. 4"},
		}},
	}, domain.AudienceStudent)

	if !strings.Contains(document, `data-id="exercise-1-work" width="760"`) {
		t.Fatal("thiếu vùng bài làm có định danh")
	}
	if !strings.Contains(document, `data-id="exercise-1-feedback"`) {
		t.Fatal("thiếu vị trí nhận xét cạnh vùng bài làm")
	}
	for _, option := range []string{"A", "B", "C", "D"} {
		if !strings.Contains(document, `data-id="exercise-1-option-`+option+`" data-tag="to-do"`) {
			t.Fatalf("thiếu checkbox đáp án %s", option)
		}
	}
	if strings.Count(document, `font-size:17pt; line-height:1.45`) != 4 {
		t.Fatal("mỗi đáp án phải có vùng bấm lớn, dễ chọn")
	}
	if strings.Count(document, `line-height:32pt;`) != 3 {
		t.Fatal("vùng làm bài phải có tối thiểu ba dòng")
	}
}

func TestRendererUsesSupportedMathMarkupAndRealStepLists(t *testing.T) {
	document := NewLessonRenderer().RenderLessonHTML("Hàm số", &domain.Lesson{
		Sections: []domain.Section{{
			SectionTitle:    "Hàm số y = -3/2x^2",
			DetailedContent: "Xét hàm số y = -3/2x^2. + Bước 1: Tính x_1 = (1)/(4). - Bước 2: Kết luận y ≤ 0.",
			TeacherExamples: []domain.TeacherExample{{
				ExampleNum:      1,
				Problem:         "Tính (a+b)/(c-d)",
				TeacherSolution: "Bước 1: Thay x^2.\nBước 2: Rút gọn -3/2.",
			}},
		}},
	}, domain.AudienceTeacher)

	for _, fragment := range []string{`list-style-type:disc`, "x<sup>2</sup>", "x<sub>1</sub>", "<sup>-3</sup>⁄<sub>2</sub>", "<sup>(a+b)</sup>⁄<sub>(c-d)</sub>"} {
		if !strings.Contains(document, fragment) {
			t.Fatalf("thiếu định dạng OneNote cho %q trong:\n%s", fragment, document)
		}
	}
	if strings.Contains(document, " + Bước 1") {
		t.Fatal("các bước giải vẫn bị dồn trên cùng một dòng")
	}
}

func TestDetailedContentUsesTwoLevelsAndCleansGeneratedSectionOrdinal(t *testing.T) {
	document := NewLessonRenderer().RenderLessonHTML("Bài kiểm tra", &domain.Lesson{
		Sections: []domain.Section{{
			SectionTitle:    "MỤC 1. Định lí Vi-ét",
			DetailedContent: "- Bước lớn\n  * Công thức: x_1 + x_2 = -b/a\n- Kết luận",
		}},
	}, domain.AudienceTeacher)

	if strings.Contains(document, "1. MỤC 1.") {
		t.Fatal("tiêu đề vẫn bị lặp số thứ tự")
	}
	if !strings.Contains(document, "1. Định lí Vi-ét") {
		t.Fatal("tiêu đề đã bị làm sạch không đúng")
	}
	if !strings.Contains(document, "list-style-type:disc") || !strings.Contains(document, "list-style-type:circle") {
		t.Fatal("chi tiết hai cấp chưa được render thành danh sách thụt lề thật")
	}
}
