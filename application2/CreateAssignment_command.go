package application2

import (
	"context"
	"errors"
	"fmt"
	"meet-attendance-clean/domain2/lesson"
	"uuid"
)

type LessonDraftRepo interface {
	Save(ctx context.Context, draft *lesson.LessonDraft) error
}

type LessonGenerator interface {
	Generate(ctx context.Context, prompt string) (*lesson.Lesson, error)
}

type GenerateLessonCommand struct {
	Title    string
	Model    string
	Material lesson.StudyMaterial
	Prompt   string
}

type GenerateLessonUsecase struct {
	repo      LessonDraftRepo
	generator LessonGenerator
}

func NewGenerateLessonUsecase(repo LessonDraftRepo, gen LessonGenerator) *GenerateLessonUsecase {
	return &GenerateLessonUsecase{repo: repo, generator: gen}
}

func (u *GenerateLessonUsecase) Create(ctx context.Context, cmd GenerateLessonCommand) (uuid.UUID, error) {
	if cmd.Material == nil {
		return uuid.Nil(), errors.New("material không được để trống")
	}

	draft := lesson.NewLessonDraft(cmd.Title, cmd.Model, lesson.LessonDraftProcessing)

	if err := u.repo.Save(ctx, draft); err != nil {
		return uuid.Nil(), fmt.Errorf("không thể lưu Lesson Draft: %w", err)
	}

	go u.processGenerationInBackground(draft.ID, cmd)

	return draft.ID, nil
}

func (u *GenerateLessonUsecase) processGenerationInBackground(draftID uuid.UUID, cmd GenerateLessonCommand) {

	bgCtx := context.Background()

	var err error
	defer func() {
		if err != nil {
			u.markDraftAsFailed(bgCtx, draftID, err)
		}
	}()

	generatedLesson, err := u.generator.Generate(bgCtx, cmd.Prompt)
	if err != nil {
		err = fmt.Errorf("AI Grader lỗi khi tạo bài giảng: %w", err)
		return
	}

	u.markDraftAsCompleted(bgCtx, draftID, *generatedLesson)
}

func (u *GenerateLessonUsecase) markDraftAsFailed(ctx context.Context, draftID uuid.UUID, reason error) {
	draft := &lesson.LessonDraft{ID: draftID}
	draft.ApplyError(reason)
	_ = u.repo.Save(ctx, draft)
}

func (u *GenerateLessonUsecase) markDraftAsCompleted(ctx context.Context, draftID uuid.UUID, result lesson.Lesson) {
	draft := &lesson.LessonDraft{ID: draftID}
	draft.ApplyLesson(result)
	_ = u.repo.Save(ctx, draft)
}
