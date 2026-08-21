package application

import (
	"context"
	"fmt"
	"time"

	"meet-attendance-clean/domain"
)

// ==================== 1. FILTER & VIEW DTOs ====================

type MeetingFilter struct {
	FromDate           time.Time `json:"from_date"`
	ToDate             time.Time `json:"to_date"`
	MinDurationMinutes int       `json:"min_duration_minutes"`
}

// 2. Bộ lọc DANH SÁCH HỌC SINH (Nhúng MeetingFilter + thêm SearchName)
type StudentFilter struct {
	SearchName    *string
	MeetingFilter // Embedded struct: Tự động có FromDate, ToDate, MinDurationMinutes
}

type DashboardView struct {
	TotalStudents        int              `json:"total_students"`
	TotalSessions        int              `json:"total_sessions"`
	TotalDurationMinutes int              `json:"total_duration_minutes"`
	Students             []StudentSummary `json:"students"`
}
type StudentSummary struct {
	ID                   int    `json:"id"`
	Class                string `json:"class"`
	Name                 string `json:"name"`
	CycleStartDay        int    `json:"cycle_start_day"`
	TotalSessions        int    `json:"total_sessions"`
	TotalDurationMinutes int    `json:"total_duration_minutes"`
}

type StudentDetailView struct {
	Student              domain.Student   `json:"student"`
	TotalSessions        int              `json:"total_sessions"`
	TotalDurationMinutes int              `json:"total_duration_minutes"`
	Meetings             []domain.Meeting `json:"meetings"`
}

type StudentQueryRepo interface {
	ListStudents(ctx context.Context, filter StudentFilter) ([]StudentSummary, error)
	GetStudent(ctx context.Context, id int) (domain.Student, error)
	ListClassMeetings(ctx context.Context, class string, filter MeetingFilter) ([]domain.Meeting, error)
}

// ==================== 3. QUERY HANDLER ====================

type MeetQuery struct {
	repo StudentQueryRepo
}

func NewMeetQuery(repo StudentQueryRepo) *MeetQuery {
	return &MeetQuery{repo: repo}
}

// Query 1: Lấy dữ liệu cho trang Dashboard (Màn hình 1)
func (q *MeetQuery) GetDashboard(ctx context.Context, filter StudentFilter) (*DashboardView, error) {
	students, err := q.repo.ListStudents(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("lấy danh sách học sinh: %w", err)
	}

	var totalSessions int
	var totalMinutes int

	// Tự động tính toán tổng số buổi và tổng thời gian cho 3 thẻ thống kê trên cùng
	for _, s := range students {
		totalSessions += s.TotalSessions
		totalMinutes += s.TotalDurationMinutes
	}

	return &DashboardView{
		TotalStudents:        len(students),
		TotalSessions:        totalSessions,
		TotalDurationMinutes: totalMinutes,
		Students:             students,
	}, nil
}

func (q *MeetQuery) GetStudentDetail(
	ctx context.Context,
	studentID int,
	filter MeetingFilter,
) (*StudentDetailView, error) {

	student, err := q.repo.GetStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy học sinh ID %d: %w", studentID, err)
	}

	meetings, err := q.repo.ListClassMeetings(ctx, student.Class, filter)
	if err != nil {
		return nil, fmt.Errorf("lấy danh sách buổi học của lớp %s: %w", student.Class, err)
	}

	// 3. Tính tổng thời lượng của riêng học sinh này
	var totalMinutes int
	for _, m := range meetings {
		totalMinutes += int(m.EndedAt.Sub(m.StartedAt).Minutes())
	}

	return &StudentDetailView{
		Student:              student,
		TotalSessions:        int(len(meetings)),
		TotalDurationMinutes: totalMinutes,
		Meetings:             meetings,
	}, nil
}

// Query 3: Lấy thông tin 1 học sinh để đổ vào Form chỉnh sửa
func (q *MeetQuery) GetStudent(ctx context.Context, id int) (domain.Student, error) {
	return q.repo.GetStudent(ctx, id)
}
