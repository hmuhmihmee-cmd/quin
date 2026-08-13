package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"meet-attendance-clean/domain"
)

var (
	ErrInvalidStudent = errors.New("thông tin học sinh không hợp lệ")
	ErrInvalidPeriod  = errors.New("khoảng thời gian không hợp lệ")
	ErrNotConnected   = errors.New("chưa liên kết tài khoản Google")
)

type AttendanceFilter struct {
	Name               string
	Month              string
	From               string
	To                 string
	MinDurationMinutes int64
}

type StudentSummary struct {
	StudentID       int64
	StudentName     string
	GoogleSpaceName string
	CycleStartDay   int
	PeriodStart     time.Time
	PeriodEnd       time.Time
	MeetingCount    int64
	TotalMinutes    int64
}

type MeetingReport struct {
	MeetingID       int64
	StartedAt       time.Time
	EndedAt         time.Time
	DurationMinutes int64
	Participants    []domain.Participant
}

type AttendanceUseCase struct {
	students       StudentRepository
	meetings       AttendanceRepository
	meet           MeetGateway
	lookbackMonths int
}

func NewAttendanceUseCase(students StudentRepository, meetings AttendanceRepository, meet MeetGateway, lookbackMonths int) *AttendanceUseCase {
	return &AttendanceUseCase{students: students, meetings: meetings, meet: meet, lookbackMonths: lookbackMonths}
}

func (u *AttendanceUseCase) IsGoogleConnected() bool { return u.meet.IsConnected() }
func (u *AttendanceUseCase) ListStudents(ctx context.Context) ([]domain.Student, error) {
	return u.students.ListStudents(ctx)
}
func (u *AttendanceUseCase) GetStudent(ctx context.Context, id int64) (domain.Student, error) {
	if id < 1 {
		return domain.Student{}, ErrInvalidStudent
	}
	return u.students.GetStudent(ctx, id)
}

func (u *AttendanceUseCase) UpdateStudent(ctx context.Context, student domain.Student) error {
	student = normalizeStudent(student)
	if student.ID < 1 || !validStudent(student) {
		return ErrInvalidStudent
	}
	if err := u.students.UpdateStudent(ctx, student); err != nil {
		return fmt.Errorf("cập nhật học sinh: %w", err)
	}
	return nil
}

func (u *AttendanceUseCase) StudentSummaries(ctx context.Context, filter AttendanceFilter, now time.Time) ([]StudentSummary, error) {
	students, err := u.students.ListStudents(ctx)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimSpace(filter.Name))
	result := make([]StudentSummary, 0, len(students))
	for _, student := range students {
		if needle != "" && !strings.Contains(strings.ToLower(student.Name), needle) {
			continue
		}
		from, to, err := periodForStudent(filter, student.CycleStartDay, now)
		if err != nil {
			return nil, err
		}
		count, minutes, err := u.meetings.StudentMeetingStats(ctx, student.GoogleSpaceName, from, to, filter.MinDurationMinutes)
		if err != nil {
			return nil, err
		}
		result = append(result, StudentSummary{
			StudentID: student.ID, StudentName: student.Name, GoogleSpaceName: student.GoogleSpaceName,
			CycleStartDay: student.CycleStartDay, PeriodStart: from, PeriodEnd: to,
			MeetingCount: count, TotalMinutes: minutes,
		})
	}
	return result, nil
}

func (u *AttendanceUseCase) StudentMeetings(ctx context.Context, studentID int64, filter AttendanceFilter, now time.Time) (domain.Student, time.Time, time.Time, []MeetingReport, error) {
	student, err := u.GetStudent(ctx, studentID)
	if err != nil {
		return domain.Student{}, time.Time{}, time.Time{}, nil, err
	}
	from, to, err := periodForStudent(filter, student.CycleStartDay, now)
	if err != nil {
		return domain.Student{}, time.Time{}, time.Time{}, nil, err
	}
	meetings, err := u.meetings.StudentMeetings(ctx, student.GoogleSpaceName, from, to, filter.MinDurationMinutes)
	return student, from, to, meetings, err
}

func (u *AttendanceUseCase) Sync(ctx context.Context, now time.Time) (int, error) {
	if !u.meet.IsConnected() {
		return 0, ErrNotConnected
	}
	items, err := u.meet.ListMeetings(ctx, now.AddDate(0, -u.lookbackMonths, 0))
	if err != nil {
		return 0, fmt.Errorf("đọc Google Meet: %w", err)
	}
	for _, item := range items {
		if err := u.meetings.SaveMeeting(ctx, item); err != nil {
			return 0, fmt.Errorf("lưu buổi học: %w", err)
		}
	}
	if err := u.students.CreateMissingStudents(ctx); err != nil {
		return 0, fmt.Errorf("tạo học sinh từ Google Space: %w", err)
	}
	return len(items), nil
}

func (u *AttendanceUseCase) AuthorizationURL(state string) (string, error) {
	return u.meet.AuthorizationURL(state)
}
func (u *AttendanceUseCase) ConnectGoogle(ctx context.Context, code string) error {
	if strings.TrimSpace(code) == "" {
		return errors.New("Google không trả authorization code")
	}
	return u.meet.Exchange(ctx, code)
}
func (u *AttendanceUseCase) DisconnectGoogle() error { return u.meet.Disconnect() }

func normalizeStudent(student domain.Student) domain.Student {
	student.Name = strings.TrimSpace(student.Name)
	student.GoogleSpaceName = strings.TrimSpace(student.GoogleSpaceName)
	if student.CycleStartDay == 0 {
		student.CycleStartDay = 1
	}
	return student
}

func validStudent(student domain.Student) bool {
	return student.Name != "" && student.GoogleSpaceName != "" && !strings.Contains(student.GoogleSpaceName, "/") && student.CycleStartDay >= 1 && student.CycleStartDay <= 31
}

func periodForStudent(filter AttendanceFilter, cycleDay int, now time.Time) (time.Time, time.Time, error) {
	location := now.Location()
	if filter.From != "" || filter.To != "" {
		if filter.From == "" || filter.To == "" {
			return time.Time{}, time.Time{}, ErrInvalidPeriod
		}
		from, err := time.ParseInLocation("2006-01-02", filter.From, location)
		if err != nil {
			return time.Time{}, time.Time{}, ErrInvalidPeriod
		}
		lastDay, err := time.ParseInLocation("2006-01-02", filter.To, location)
		if err != nil || lastDay.Before(from) {
			return time.Time{}, time.Time{}, ErrInvalidPeriod
		}
		return from, lastDay.AddDate(0, 0, 1), nil
	}
	month := filter.Month
	if month == "" {
		month = now.Format("2006-01")
	}
	base, err := time.ParseInLocation("2006-01", month, location)
	if err != nil {
		return time.Time{}, time.Time{}, ErrInvalidPeriod
	}
	from := clampedDate(base.Year(), base.Month(), cycleDay, location)
	if base.Year() == now.Year() && base.Month() == now.Month() && from.After(now) {
		previous := base.AddDate(0, -1, 0)
		from = clampedDate(previous.Year(), previous.Month(), cycleDay, location)
	}
	nextMonth := from.AddDate(0, 1, 0)
	to := clampedDate(nextMonth.Year(), nextMonth.Month(), cycleDay, location)
	if now.After(from) && now.Before(to) {
		to = now
	}
	return from, to, nil
}

func clampedDate(year int, month time.Month, day int, location *time.Location) time.Time {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, location).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, 0, 0, 0, 0, location)
}
