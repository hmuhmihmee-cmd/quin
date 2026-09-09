package application

import (
	"context"
	"testing"

	"meet-attendance-clean/domain"
)

type gradingRepoStub struct{ assignment *domain.Assignment }

func (s *gradingRepoStub) Save(context.Context, *domain.Assignment) error { return nil }
func (s *gradingRepoStub) GetByPageID(context.Context, string) (*domain.Assignment, error) {
	return s.assignment, nil
}

type mistakeRepoStub struct{}

func (*mistakeRepoStub) GetByID(context.Context, int) (*domain.Mistake, error)          { return nil, nil }
func (*mistakeRepoStub) SaveMistake(context.Context, domain.Mistake) error              { return nil }
func (*mistakeRepoStub) SaveMistakes(context.Context, int, int, []domain.Mistake) error { return nil }
func (*mistakeRepoStub) GetMistakeContext(context.Context, int) (*MistakeContext, error) {
	return nil, nil
}
func (*mistakeRepoStub) MarkMistakeResolved(context.Context, int) error { return nil }

type gradingOneNoteStub struct{}

func (*gradingOneNoteStub) PublishSession(context.Context, WorkspaceTarget, domain.Audience, domain.Lesson) (PublishResult, error) {
	return PublishResult{}, nil
}
func (*gradingOneNoteStub) FetchStudentAnswers(_ context.Context, assignment *domain.Assignment) error {
	for index := range assignment.Items {
		assignment.Items[index].StudentAnswer = &domain.StudentAnswer{Text: "Bài làm"}
	}
	return nil
}
func (*gradingOneNoteStub) PatchFeedback(context.Context, *domain.Assignment) error { return nil }

// preservedAnswerOneNoteStub mô phỏng OneNote đã bóc tách được trạng thái chọn đáp án.
type preservedAnswerOneNoteStub struct{}

func (*preservedAnswerOneNoteStub) PublishSession(context.Context, WorkspaceTarget, domain.Audience, domain.Lesson) (PublishResult, error) {
	return PublishResult{}, nil
}
func (*preservedAnswerOneNoteStub) FetchStudentAnswers(context.Context, *domain.Assignment) error {
	return nil
}
func (*preservedAnswerOneNoteStub) PatchFeedback(context.Context, *domain.Assignment) error {
	return nil
}

type batchAIStub struct{ calls int }

func (s *batchAIStub) EvaluateBatch(_ context.Context, _ string, _ string, _ []byte, items []GradingItem) ([]domain.AIEvaluationResult, error) {
	s.calls++
	results := make([]domain.AIEvaluationResult, 0, len(items))
	for _, item := range items {
		results = append(results, domain.AIEvaluationResult{
			ExerciseID: item.Exercise.ID,
			IsCorrect:  true,
			Feedback:   "<p>Cách giải.</p>",
		})
	}
	return results, nil
}
func (*batchAIStub) GenerateRemediation(context.Context, string, string, MistakeContext, int, int) (*domain.Lesson, error) {
	return nil, nil
}

func TestGradeEvaluatesExercisesInOneBatch(t *testing.T) {
	items := make([]domain.AssignedExercise, 8)
	for index := range items {
		items[index].Exercise = domain.Exercise{ID: index + 1, Question: "Câu hỏi"}
	}
	repo := &gradingRepoStub{assignment: &domain.Assignment{ID: 1, TargetPageID: "page", Assignee: domain.Student{ID: 1}, Items: items}}
	ai := &batchAIStub{}
	command := &AssignmentCommand{assignmentRepo: repo, mistakeRepo: &mistakeRepoStub{}, onenote: &gradingOneNoteStub{}, ai: ai}

	if err := command.Grade(context.Background(), GradeAssignmentCommand{PageID: "page"}); err != nil {
		t.Fatal(err)
	}
	if ai.calls != 1 {
		t.Fatalf("số lần gọi batch = %d, muốn 1", ai.calls)
	}
}

func TestGradeRetriesPreviouslyBlankExerciseAfterStudentWrites(t *testing.T) {
	repo := &gradingRepoStub{assignment: &domain.Assignment{
		ID: 1, TargetPageID: "page", Assignee: domain.Student{ID: 1},
		Items: []domain.AssignedExercise{{
			Exercise: domain.Exercise{ID: 7, Question: "Câu hỏi"},
			Result:   &domain.GradingResult{Status: domain.AIExerciseSkippedEmpty},
		}},
	}}
	ai := &batchAIStub{}
	command := &AssignmentCommand{assignmentRepo: repo, mistakeRepo: &mistakeRepoStub{}, onenote: &gradingOneNoteStub{}, ai: ai}
	if err := command.Grade(context.Background(), GradeAssignmentCommand{PageID: "page"}); err != nil {
		t.Fatal(err)
	}
	if ai.calls != 1 || repo.assignment.Items[0].Result.Status != domain.AIExerciseGraded {
		t.Fatal("câu từng bỏ trống phải được chấm sau khi học sinh bổ sung bài")
	}
}

func TestGradeDoesNotSendInvalidMultipleChoiceSelectionToAI(t *testing.T) {
	answer := &domain.StudentAnswer{
		Text:            "Em có trình bày phép tính.",
		ChoiceSelection: domain.ChoiceMultiple,
	}
	repo := &gradingRepoStub{assignment: &domain.Assignment{
		ID: 1, TargetPageID: "page", Assignee: domain.Student{ID: 1},
		Items: []domain.AssignedExercise{{
			Exercise:      domain.Exercise{ID: 7, Question: "Câu hỏi", Options: []string{"A", "B", "C", "D"}, Answer: "B"},
			StudentAnswer: answer,
		}},
	}}
	ai := &batchAIStub{}
	command := &AssignmentCommand{assignmentRepo: repo, mistakeRepo: &mistakeRepoStub{}, onenote: &preservedAnswerOneNoteStub{}, ai: ai}

	if err := command.Grade(context.Background(), GradeAssignmentCommand{PageID: "page"}); err != nil {
		t.Fatal(err)
	}
	result := repo.assignment.Items[0].Result
	if ai.calls != 0 {
		t.Fatalf("AI không được gọi khi chọn nhiều đáp án, gọi %d lần", ai.calls)
	}
	if result == nil || result.Status != domain.AIExerciseAwaitingSelection {
		t.Fatalf("trạng thái = %#v, muốn awaiting_selection", result)
	}
	if result.FeedbackHTML != "<p>Không hợp lệ: chỉ được chọn một đáp án.</p>" {
		t.Fatalf("nhận xét không hợp lệ: %s", result.FeedbackHTML)
	}
	if repo.assignment.Status == domain.AssignmentStatusGraded {
		t.Fatal("bài có câu chọn nhiều không được coi là đã chấm xong")
	}
}

func TestGradeUsesSelectedOptionAsMultipleChoiceVerdict(t *testing.T) {
	repo := &gradingRepoStub{assignment: &domain.Assignment{
		ID: 1, TargetPageID: "page", Assignee: domain.Student{ID: 1},
		Items: []domain.AssignedExercise{{
			Exercise: domain.Exercise{ID: 8, Question: "Câu hỏi", Options: []string{"A", "B", "C", "D"}, Answer: "B"},
			StudentAnswer: &domain.StudentAnswer{
				ChoiceSelection: domain.ChoiceSelected,
				SelectedOption:  "A",
			},
		}},
	}}
	ai := &batchAIStub{} // Stub trả is_correct=true để chứng minh code không tin verdict của AI.
	command := &AssignmentCommand{assignmentRepo: repo, mistakeRepo: &mistakeRepoStub{}, onenote: &preservedAnswerOneNoteStub{}, ai: ai}

	if err := command.Grade(context.Background(), GradeAssignmentCommand{PageID: "page"}); err != nil {
		t.Fatal(err)
	}
	result := repo.assignment.Items[0].Result
	if ai.calls != 1 || result == nil || result.IsCorrect {
		t.Fatalf("đúng/sai trắc nghiệm phải theo lựa chọn A/B/C/D, result=%#v", result)
	}
}
