package onenote

import (
	"strings"
	"testing"

	"meet-attendance-clean/domain"
)

func TestFindFeedbackTargetID(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "current feedback placeholder",
			html: `<div data-id="exercise-3-feedback" id="current-target"></div>`,
			want: "current-target",
		},
		{
			name: "legacy feedback card inside its exercise region",
			html: `<div data-id="exercise-3-region"><div data-id="exercise-feedback-result" id="legacy-target"></div></div>` +
				`<div data-id="exercise-4-region"><div data-id="exercise-feedback-result" id="other-target"></div></div>`,
			want: "legacy-target",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, found := findFeedbackTargetID(test.html, 3)
			if !found || got != test.want {
				t.Fatalf("findFeedbackTargetID() = (%q, %v), want (%q, true)", got, found, test.want)
			}
		})
	}
}

func TestExtractSelectedOptionsFromOneNoteOutputHTML(t *testing.T) {
	html := `<span data-tag="to-do" data-id="exercise-9-option-A">A</span>` +
		`<span data-tag="to-do:completed" data-id="exercise-9-option-C">C</span>` +
		`<span data-tag="to-do" data-id="exercise-9-option-D">D</span>`

	selected := extractSelectedOptions(html)
	if len(selected) != 1 || selected[0] != "C" {
		t.Fatalf("extractSelectedOptions() = %v, want [C]", selected)
	}
}

func TestRenderFeedbackKeepsCorrectStatusWhenPresentationFeedbackExists(t *testing.T) {
	html := renderFeedbackCard(3, &domain.GradingResult{
		Status:       domain.AIExerciseGraded,
		IsCorrect:    true,
		FeedbackHTML: "<p>Đúng đáp án, nhưng sai dấu: phải là x ≤ 0.</p>",
	})
	if !strings.Contains(html, "✅ Đúng") || !strings.Contains(html, "phải là x ≤ 0") {
		t.Fatalf("trạng thái phải theo is_correct, nhưng vẫn hiện nhận xét: %s", html)
	}
}
