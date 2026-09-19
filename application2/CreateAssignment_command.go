package application2

import (
	"context"
	"errors"
	"fmt"
	"meet-attendance-clean/domain2/lesson"
	"time"
	"uuid"
)

type LessonDraftRepo interface {
	Save(ctx context.Context, draft *lesson.LessonDraft) error
}

type LessonGenerator interface {
	Generate(ctx context.Context, material lesson.StudyMaterial, model string, prompt string) (*lesson.Lesson, error)
}

type GenerateLessonCommand struct {
	Title    string
	Model    string
	Prompt   string
	Material lesson.StudyMaterial
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

	draft := lesson.NewLessonDraft(cmd.Title, cmd.Model, cmd.Prompt, lesson.LessonDraftProcessing)

	if err := u.repo.Save(ctx, draft); err != nil {
		return uuid.Nil(), fmt.Errorf("không thể lưu Lesson Draft: %w", err)
	}

	go u.processGenerationInBackground(draft, cmd)

	return draft.ID(), nil
}

func (u *GenerateLessonUsecase) processGenerationInBackground(draft *lesson.LessonDraft, cmd GenerateLessonCommand) {

	bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var err error
	defer func() {
		if err != nil {
			draft.ApplyError(err)
			_ = u.repo.Save(bgCtx, draft)
		}
	}()

	generatedLesson, err := u.generator.Generate(bgCtx, cmd.Material, cmd.Model, cmd.Prompt)
	if err != nil {
		err = fmt.Errorf("AI Grader lỗi khi tạo bài giảng: %w", err)
		return
	}

	draft.ApplyLesson(*generatedLesson)
	err = u.repo.Save(bgCtx, draft)
}
