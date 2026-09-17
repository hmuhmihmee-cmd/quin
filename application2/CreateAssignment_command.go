package application2

import "context"

type CreateAssignmentCommand struct {
}
type CreateAssignmentUsecase struct {
}

func (u *CreateAssignmentUsecase) Create(ctx context.Context, cmd CreateAssignmentCommand) error {
	return nil
}
