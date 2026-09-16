package application2

import (
	"context"
	"errors"
	"meet-attendance-clean/domain2/assignment"
	"uuid"
)

type AssignmentRepo interface {
	GetByID(ctx context.Context, assignmentID uuid.UUID) (*assignment.Assignment, error)
	Save(ctx context.Context, assignment *assignment.Assignment) error
}
type WorkSpaceGateway interface {
	FetchStudentSubmission(ctx context.Context, assignmentID uuid.UUID) (pageInk []byte, answers []assignment.Answer, err error)
	PatchFeedback(ctx context.Context, assignmentID uuid.UUID, items []assignment.AssigmentItem) error
}
type Grader interface {
	Evaluate(ctx context.Context, gradeItems []assignment.GradeItem, pageInk []byte, prompt string, model string) ([]assignment.GradeItem, error)
}
type GradeAssignmentUsecase struct {
	assignmentRepo   AssignmentRepo
	workspaceGateway WorkSpaceGateway
	grader           Grader
}
type GradeAssignmentCommand struct {
	AssignmentID uuid.UUID
	ItemIDs      []uuid.UUID
	Prompt       string
	Model        string
}

func (g *GradeAssignmentUsecase) Grade(ctx context.Context, cmd GradeAssignmentCommand) error {
	assignment, err := g.assignmentRepo.GetByID(ctx, cmd.AssignmentID)
	if err != nil {
		return err
	}
	pageInk, answers, err := g.workspaceGateway.FetchStudentSubmission(ctx, assignment.ID())
	if err != nil {
		return err
	}
	assignment.AddAnwsers(answers)
	validItems := assignment.ListItem(cmd.ItemIDs)
	if len(validItems) == 0 {
		return errors.New("Không có bài nào hợp lệ")
	}
	evaluations, err := g.grader.Evaluate(ctx, validItems, pageInk, cmd.Prompt, cmd.Model)
	if err != nil {
		return err
	}
	assignment.ApplyGrade(evaluations)
	if err := g.assignmentRepo.Save(ctx, assignment); err != nil {
		return err
	}
	if err := g.workspaceGateway.PatchFeedback(ctx, assignment.ID(), assignment.Items()); err != nil {
		return err
	}
	return nil
}
