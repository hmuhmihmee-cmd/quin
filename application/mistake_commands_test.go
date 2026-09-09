package application

import (
	"context"
	"testing"

	"meet-attendance-clean/domain"
)

type remediationAssignmentRepoStub struct{ saved *domain.Assignment }

func (s *remediationAssignmentRepoStub) Save(_ context.Context, assignment *domain.Assignment) error {
	s.saved = assignment
	return nil
}

func (*remediationAssignmentRepoStub) GetByPageID(context.Context, string) (*domain.Assignment, error) {
	return nil, nil
}

type remediationMistakeRepoStub struct {
	context *MistakeContext
	saved   domain.Mistake
}

func (s *remediationMistakeRepoStub) GetByID(context.Context, int) (*domain.Mistake, error) {
	return nil, nil
}
func (s *remediationMistakeRepoStub) SaveMistake(_ context.Context, mistake domain.Mistake) error {
	s.saved = mistake
	return nil
}
func (*remediationMistakeRepoStub) SaveMistakes(context.Context, int, int, []domain.Mistake) error {
	return nil
}
func (s *remediationMistakeRepoStub) GetMistakeContext(context.Context, int) (*MistakeContext, error) {
	return s.context, nil
}
func (*remediationMistakeRepoStub) MarkMistakeResolved(context.Context, int) error { return nil }

type remediationAIStub struct{}

func (*remediationAIStub) EvaluateBatch(context.Context, string, string, []byte, []GradingItem) ([]domain.AIEvaluationResult, error) {
	return nil, nil
}
func (*remediationAIStub) GenerateRemediation(context.Context, string, string, MistakeContext, int, int) (*domain.Lesson, error) {
	return &domain.Lesson{Exercises: []domain.Exercise{{ID: 1, Question: "Câu hỏi", Answer: "A", Explanation: "Lời giải"}}}, nil
}

type remediationOneNoteStub struct{ calls []domain.Audience }

func (s *remediationOneNoteStub) PublishSession(_ context.Context, _ WorkspaceTarget, audience domain.Audience, _ domain.Lesson) (PublishResult, error) {
	s.calls = append(s.calls, audience)
	if audience == domain.AudienceTeacher {
		return PublishResult{PageID: "teacher-page", PageWebURL: "https://onenote.example/teacher"}, nil
	}
	return PublishResult{PageID: "student-page", PageWebURL: "https://onenote.example/student"}, nil
}
func (*remediationOneNoteStub) FetchStudentAnswers(context.Context, *domain.Assignment) error {
	return nil
}
func (*remediationOneNoteStub) PatchFeedback(context.Context, *domain.Assignment) error { return nil }

func TestGenerateRemediationPublishesAndStoresBothOneNotePages(t *testing.T) {
	studentNotebook, teacherNotebook := "student-notebook", "teacher-notebook"
	mistakeRepo := &remediationMistakeRepoStub{context: &MistakeContext{
		CurrentMistake: domain.Mistake{ID: 12, Topic: "Phân số", Status: domain.MistakeDetected},
		Student:        domain.Student{ID: 7, Name: "An", StudentWorkspaceID: &studentNotebook, TeacherWorkspaceID: &teacherNotebook},
	}}
	assignmentRepo := &remediationAssignmentRepoStub{}
	oneNote := &remediationOneNoteStub{}
	command := &AssignmentCommand{assignmentRepo: assignmentRepo, mistakeRepo: mistakeRepo, onenote: oneNote, ai: &remediationAIStub{}}

	if err := command.GenerateRemediation(context.Background(), GenerateRemediationCommand{MistakeID: 12, MultipleChoiceCount: 1}); err != nil {
		t.Fatal(err)
	}

	if len(oneNote.calls) != 2 || oneNote.calls[0] != domain.AudienceTeacher || oneNote.calls[1] != domain.AudienceStudent {
		t.Fatalf("phải xuất lần lượt trang giáo viên và học sinh, nhận được %#v", oneNote.calls)
	}
	if assignmentRepo.saved == nil {
		t.Fatal("chưa lưu assignment")
	}
	if assignmentRepo.saved.TargetPageID != "student-page" || assignmentRepo.saved.StudentPageWebURL != "https://onenote.example/student" || assignmentRepo.saved.TeacherPageWebURL != "https://onenote.example/teacher" {
		t.Fatalf("assignment lưu sai liên kết OneNote: %#v", assignmentRepo.saved)
	}
	if mistakeRepo.saved.Status != domain.MistakeRemediating {
		t.Fatalf("trạng thái lỗi chưa chuyển sang remediating: %q", mistakeRepo.saved.Status)
	}
}
