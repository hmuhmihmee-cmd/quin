package application

import (
	"context"
	"meet-attendance-clean/domain"
)

type WorkspaceRepo interface {
	ListWorkspaces(ctx context.Context) ([]domain.Workspace, error)
}
type WorkspaceQuery struct {
	repo WorkspaceRepo
}

func NewWorkspaceQuery(repo WorkspaceRepo) *WorkspaceQuery {
	return &WorkspaceQuery{
		repo: repo,
	}
}
func (w *WorkspaceQuery) ListWorkspaces(ctx context.Context) ([]domain.Workspace, error) {
	return w.repo.ListWorkspaces(ctx)
}
