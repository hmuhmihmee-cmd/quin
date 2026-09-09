package application

import (
	"context"
	"strings"
	"time"

	"meet-attendance-clean/domain"
)

type GradingPageFilter struct {
	Search string `json:"search"`
}

type AssignmentSummary struct {
	AssignmentID      int                     `json:"assignment_id"`
	PageID            string                  `json:"page_id"`
	StudentPageWebURL string                  `json:"student_page_web_url"`
	TeacherPageWebURL string                  `json:"teacher_page_web_url"`
	Title             string                  `json:"title"`
	StudentID         int                     `json:"student_id"`
	StudentName       string                  `json:"student_name"`
	Status            domain.AssignmentStatus `json:"status"`
	AssignedAt        time.Time               `json:"assigned_at"`
	CorrectCount      int                     `json:"correct_count"`
	GradedCount       int                     `json:"graded_count"`
	TotalCount        int                     `json:"total_count"`
}

type StudentMistakeGroup struct {
	StudentID   int              `json:"student_id"`
	StudentName string           `json:"student_name"`
	Mistakes    []domain.Mistake `json:"mistakes"`
}

type StudentChoice struct {
	ID                 int     `json:"id"`
	Name               string  `json:"name"`
	Class              string  `json:"class"`
	StudentWorkspaceID *string `json:"student_workspace_id,omitempty"`
	TeacherWorkspaceID *string `json:"teacher_workspace_id,omitempty"`
}

type AssignmentQueryRepository interface {
	GetByPageID(ctx context.Context, pageID string) (*domain.Assignment, error)
	ListAssignmentSummaries(ctx context.Context) ([]AssignmentSummary, error)
	ListMistakeGroups(ctx context.Context) ([]StudentMistakeGroup, error)
	ListStudentChoices(ctx context.Context) ([]StudentChoice, error)
}

type AssignmentQuery struct {
	repo AssignmentQueryRepository
}

func NewAssignmentQuery(repo AssignmentQueryRepository) *AssignmentQuery {
	return &AssignmentQuery{repo: repo}
}

func (q *AssignmentQuery) ListGradingPages(ctx context.Context, filter GradingPageFilter) ([]AssignmentSummary, error) {
	assignments, err := q.repo.ListAssignmentSummaries(ctx)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimSpace(filter.Search))
	result := make([]AssignmentSummary, 0, len(assignments))
	for _, item := range assignments {
		haystack := strings.ToLower(strings.Join([]string{item.Title, item.StudentName}, " "))
		if needle != "" && !strings.Contains(haystack, needle) {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (q *AssignmentQuery) GetForGrading(ctx context.Context, pageID string) (*domain.Assignment, error) {
	return q.repo.GetByPageID(ctx, pageID)
}

func (q *AssignmentQuery) ListMistakes(ctx context.Context) ([]StudentMistakeGroup, error) {
	return q.repo.ListMistakeGroups(ctx)
}

func (q *AssignmentQuery) ListStudents(ctx context.Context) ([]StudentChoice, error) {
	return q.repo.ListStudentChoices(ctx)
}
