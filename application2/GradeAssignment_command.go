package application2

import (
	"context"
	"errors"
	"fmt"
	"meet-attendance-clean/domain2/assignment"
	"meet-attendance-clean/domain2/mistake"
	"uuid"
)

type AssignmentRepo interface {
	GetByID(ctx context.Context, assignmentID uuid.UUID) (*assignment.Assignment, error)
	Save(ctx context.Context, assignment *assignment.Assignment) error
}

type WorkSpaceGateway interface {
	FetchStudentSubmission(ctx context.Context, assignmentID uuid.UUID) (pageInk []byte, answers []assignment.Answer, err error)
	PatchFeedback(ctx context.Context, assignmentID uuid.UUID, items []assignment.AssignmentItem) error
}

type Grader interface {
	Evaluate(ctx context.Context, gradeItems []assignment.EvaluationRequest, pageInk []byte, prompt string, model string) ([]assignment.EvaluationResult, error)
}

type GradeAssignmentCommand struct {
	AssignmentID uuid.UUID
	ItemIDs      []uuid.UUID
	Prompt       string
	Model        string
}

type GradeAssignmentUsecase struct {
	assignmentRepo   AssignmentRepo
	remediationRepo  RemediationTaskRepo
	misktakeGrapRepo MistakeGraphRepo
	workspaceGateway WorkSpaceGateway
	grader           Grader
}

func NewGradeAssignmentUsecase(
	assignmentRepo AssignmentRepo,
	remediationRepo RemediationTaskRepo,
	misktakeGrapRepo MistakeGraphRepo,
	workspaceGateway WorkSpaceGateway,
	grader Grader,
) *GradeAssignmentUsecase {
	return &GradeAssignmentUsecase{
		assignmentRepo:   assignmentRepo,
		remediationRepo:  remediationRepo,
		misktakeGrapRepo: misktakeGrapRepo,
		workspaceGateway: workspaceGateway,
		grader:           grader,
	}
}

func (g *GradeAssignmentUsecase) Grade(ctx context.Context, cmd GradeAssignmentCommand) error {

	a, err := g.assignmentRepo.GetByID(ctx, cmd.AssignmentID)
	if err != nil {
		return fmt.Errorf("không thể tìm thấy bài tập: %w", err)
	}

	pageInk, answers, err := g.workspaceGateway.FetchStudentSubmission(ctx, a.ID())
	if err != nil {
		return fmt.Errorf("lỗi khi lấy bài làm của học sinh: %w", err)
	}

	if err := a.AddAnswers(answers); err != nil {
		return fmt.Errorf("dữ liệu bài làm không hợp lệ: %w", err)
	}

	gradeItemsToEvaluate := a.ListEvalReqs(cmd.ItemIDs)
	if len(gradeItemsToEvaluate) == 0 {
		return errors.New("không có câu hỏi nào hợp lệ để chấm")
	}

	evaluations, err := g.grader.Evaluate(ctx, gradeItemsToEvaluate, pageInk, cmd.Prompt, cmd.Model)
	if err != nil {
		return fmt.Errorf("lỗi khi chấm bài qua AI Grader: %w", err)
	}

	detectedMistakes, err := a.ApplyEvalResults(evaluations)
	if err != nil {
		return fmt.Errorf("lỗi khi áp dụng kết quả chấm điểm: %w", err)
	}

	if len(detectedMistakes) > 0 {
		graph, err := g.misktakeGrapRepo.GetByStudentID(ctx, a.StudentID())
		if err != nil {
			return fmt.Errorf("không thể lấy cây lỗi của học sinh: %w", err)
		}
		task, err := g.remediationRepo.GetByAssinmentID(ctx, a.ID())
		if err != nil {
			return fmt.Errorf("không thể lấy task chữa lỗi của học sinh: %w", err)
		}
		var parentMistakeID *uuid.UUID
		if task.AssignmentID() != nil {
			parentMistakeID = task.AssignmentID()
		}
		for _, mistake := range detectedMistakes {
			graph.RegisterMistake(parentMistakeID, mistake.Topic(), mistake.Reason(), mistake.AssignmentItemID())
		}
		if err := g.misktakeGrapRepo.Save(ctx, graph); err != nil {
			return fmt.Errorf("không thể lưu cây lỗi: %w", err)
		}
	}

	if err := g.assignmentRepo.Save(ctx, a); err != nil {
		return fmt.Errorf("không thể lưu kết quả bài tập: %w", err)
	}

	if err := g.workspaceGateway.PatchFeedback(ctx, a.ID(), a.Items()); err != nil {
		return fmt.Errorf("không thể đồng bộ nhận xét sang workspace: %w", err)
	}
	if a.IsCorrect() {
		err := g.resolveRemediationIfAssigned(ctx, a.ID())
		fmt.Printf("Cảnh báo: Không thể hoàn tất task chữa lỗi cho bài %s: %v\n", a.ID(), err)
	}
	return nil
}
func (g *GradeAssignmentUsecase) resolveRemediationIfAssigned(ctx context.Context, assignmentID uuid.UUID) error {

	task, err := g.remediationRepo.GetByAssinmentID(ctx, assignmentID)
	if err != nil {
		return err
	}
	if task == nil {
		return nil
	}
	if task.Status() == mistake.RemediationStatusResolved {
		return nil
	}
	graph, err := g.misktakeGrapRepo.GetByStudentID(ctx, task.StudentID())
	if err != nil {
		return fmt.Errorf("không tìm thấy cây lỗi của học sinh: %w", err)
	}

	if err := graph.Resolve(task.MistakeID()); err != nil {
		return fmt.Errorf("lỗ hổng chưa đủ điều kiện đóng: %w", err)
	}

	task.MarkAsCompleted()

	if err := g.remediationRepo.Save(ctx, task); err != nil {
		return fmt.Errorf("không thể lưu task: %w", err)
	}
	if err := g.misktakeGrapRepo.Save(ctx, graph); err != nil {
		return fmt.Errorf("không thể lưu cây lỗi: %w", err)
	}
	return nil
}
