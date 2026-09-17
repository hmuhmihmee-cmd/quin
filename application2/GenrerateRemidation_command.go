package application2

import (
	"context"
	"errors"
	"fmt"
	"meet-attendance-clean/domain2/lesson"
	"meet-attendance-clean/domain2/mistake"
	"strings"
	"uuid"
)

type MistakeGraphRepo interface {
	GetByStudentID(ctx context.Context, studentID uuid.UUID) (*mistake.MistakeGraph, error)
	Save(ctx context.Context, graph *mistake.MistakeGraph) error
}

type RemediationAIGenerator interface {
	GenerateRemediation(ctx context.Context, topic, reason string, ancestryContext string, model string, prompt string) (*lesson.Lesson, error)
}
type RemediationTaskRepo interface {
	Save(ctx context.Context, task *mistake.RemediationTask) error
}

type GenerateRemediationCommand struct {
	StudentID uuid.UUID
	MistakeID uuid.UUID
	Model     string
	Prompt    string
}

type GenerateRemediationLessonUsecase struct {
	remediationTaskRepo RemediationTaskRepo
	graphRepo           MistakeGraphRepo
	aiGenerator         RemediationAIGenerator
}

func NewGenerateRemediationLessonUsecase(
	remediationTaskRepo RemediationTaskRepo,
	graphRepo MistakeGraphRepo,
	aiGen RemediationAIGenerator,
) *GenerateRemediationLessonUsecase {
	return &GenerateRemediationLessonUsecase{
		remediationTaskRepo: remediationTaskRepo,
		graphRepo:           graphRepo,
		aiGenerator:         aiGen,
	}
}

func (u *GenerateRemediationLessonUsecase) Create(ctx context.Context, cmd GenerateRemediationCommand) (uuid.UUID, error) {
	// 1. Lấy cây phả hệ tri thức của học sinh (Aggregate Root: MistakeGraph)
	graph, err := u.graphRepo.GetByStudentID(ctx, cmd.StudentID)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("không tìm thấy hồ sơ lỗi của học sinh: %w", err)
	}

	// 2. Lấy toàn bộ chuỗi lỗi từ gốc đến ngọn để AI hiểu vì sao học sinh sai
	path, err := graph.GetAncestryPath(cmd.MistakeID)
	if err != nil {
		return uuid.Nil(), err
	}
	if len(path) == 0 {
		return uuid.Nil(), errors.New("không tìm thấy lỗi sai trong phả hệ tri thức của học sinh")
	}

	targetMistake := path[len(path)-1]
	if targetMistake.Status() == mistake.MistakeStatusResolved {
		return uuid.Nil(), errors.New("lỗ hổng này đã được khắc phục rồi, không cần tạo bài chữa")
	}

	// 3. Tạo Draft bài học
	draftTitle := fmt.Sprintf("Bài chữa: %s", targetMistake.Topic())
	draft := lesson.NewLessonDraft(draftTitle, cmd.Model, cmd.Prompt, lesson.LessonDraftProcessing)
	if draft == nil {
		return uuid.Nil(), errors.New("không tạo được bài học")
	}
	remediationTask, err := mistake.NewRemediationTask(cmd.StudentID, targetMistake.ID(), *draft)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("không thể tạo tác vụ chữa lỗi: %w", err)
	}
	if err := u.remediationTaskRepo.Save(ctx, remediationTask); err != nil {
		return uuid.Nil(), err
	}
	go u.processRemediationInBackground(remediationTask, graph, cmd.StudentID, targetMistake, path, cmd.Prompt, cmd.Model)

	return remediationTask.ID(), nil
}

func (u *GenerateRemediationLessonUsecase) processRemediationInBackground(
	remediationTask *mistake.RemediationTask,
	graph *mistake.MistakeGraph,
	studentID uuid.UUID,
	target *mistake.Mistake,
	ancestry []*mistake.Mistake,
	prompt string,
	model string,
) {
	bgCtx := context.Background()

	// 1. Chuẩn bị ngữ cảnh lỗi từ phả hệ
	ancestryContext := buildAncestryContextText(ancestry)

	// 2. Gọi AI sinh bài học khắc phục
	remediationLesson, err := u.aiGenerator.GenerateRemediation(bgCtx, target.Topic(), target.Reason(), ancestryContext, model, prompt)
	if err != nil {
		remediationTask.ApplyError(err)
		return
	}
	if remediationLesson == nil {
		remediationTask.ApplyError(errors.New("AI không tạo bài chữa nào"))
		return
	}
	remediationTask.ApplyLesson(*remediationLesson)

	// 4. Lưu lại cây lỗi và hoàn tất draft
	err = u.graphRepo.Save(bgCtx, graph)
	if err != nil {
		remediationTask.ApplyError(fmt.Errorf("không thể lưu cây lỗi: %w", err))
		return
	}
	err = u.remediationTaskRepo.Save(bgCtx, remediationTask)
}

func buildAncestryContextText(ancestry []*mistake.Mistake) string {
	// Ghép chuỗi các lỗi trước đó để làm prompt cho AI
	var sb strings.Builder
	for i, m := range ancestry {
		sb.WriteString(fmt.Sprintf("Tầng %d: Chủ đề '%s', Lý do sai: '%s'\n", i, m.Topic(), m.Reason()))
	}
	return sb.String()
}
