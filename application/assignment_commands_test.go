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

func (*mistakeRepoStub) SaveMistakes(context.Context, int, int, []domain.Mistake) error { return nil }
func (*mistakeRepoStub) GetMistakeContext(context.Context, int) (*MistakeContext, error) {
	return nil, nil
}
func (*mistakeRepoStub) MarkMistakeResolved(context.Context, int) error { return nil }

type gradingOneNoteStub struct{}

func (*gradingOneNoteStub) PublishStudentLesson(context.Context, WorkspaceTarget, *domain.Lesson) (string, error) {
	return "", nil
}
func (*gradingOneNoteStub) FetchStudentAnswers(_ context.Context, assignment *domain.Assignment) error {
	for index := range assignment.Items {
		assignment.Items[index].StudentAnswer = &domain.StudentAnswer{Text: "Bài làm", IsBlank: false}
	}
	return nil
}
func (*gradingOneNoteStub) PatchFeedback(context.Context, *domain.Assignment) error { return nil }

type batchAIStub struct{ calls int }

func (s *batchAIStub) EvaluateBatch(_ context.Context, _ string, _ string, items []GradingItem) ([]GradingResultItem, error) {
	s.calls++
	results := make([]GradingResultItem, 0, len(items))
	for _, item := range items {
		results = append(results, GradingResultItem{
			ExerciseID: item.Exercise.ID,
			Result:     domain.GradingResult{IsCorrect: true, FeedbackHTML: "<p>Cách giải.</p>"},
		})
	}
	return results, nil
}
func (*batchAIStub) GenerateRemediation(context.Context, string, string, domain.Mistake, int, int) (*domain.Lesson, error) {
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
