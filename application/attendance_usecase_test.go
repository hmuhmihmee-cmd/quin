package application

import (
	"context"
	"testing"
	"time"

	"meet-attendance-clean/domain"
)

type syncRepositoryStub struct{}

func (syncRepositoryStub) ListStudents(context.Context) ([]domain.Student, error) { return nil, nil }
func (syncRepositoryStub) GetStudent(context.Context, int64) (domain.Student, error) {
	return domain.Student{}, nil
}
func (syncRepositoryStub) UpdateStudent(context.Context, domain.Student) error { return nil }
func (syncRepositoryStub) CreateMissingStudents(context.Context) error         { return nil }
func (syncRepositoryStub) StudentMeetingStats(context.Context, string, time.Time, time.Time, int64) (int64, int64, error) {
	return 0, 0, nil
}
func (syncRepositoryStub) StudentMeetings(context.Context, string, time.Time, time.Time, int64) ([]MeetingReport, error) {
	return nil, nil
}
func (syncRepositoryStub) SaveMeeting(context.Context, ImportedMeeting) error { return nil }

type syncMeetStub struct{ since time.Time }

func (*syncMeetStub) IsConnected() bool                       { return true }
func (*syncMeetStub) AuthorizationURL(string) (string, error) { return "", nil }
func (*syncMeetStub) Exchange(context.Context, string) error  { return nil }
func (*syncMeetStub) Disconnect() error                       { return nil }
func (m *syncMeetStub) ListMeetings(_ context.Context, since time.Time) ([]ImportedMeeting, error) {
	m.since = since
	return nil, nil
}

func TestStudentRequiresGoogleSpaceID(t *testing.T) {
	valid := domain.Student{Name: "Minh Anh", GoogleSpaceName: "abc123", CycleStartDay: 1}
	if !validStudent(valid) {
		t.Fatal("Space ID phải là định danh học sinh hợp lệ")
	}
	valid.GoogleSpaceName = "spaces/abc123"
	if validStudent(valid) {
		t.Fatal("không được lưu tiền tố spaces/ trong Student")
	}
}

func TestPeriodForStudentUsesActivePersonalCycle(t *testing.T) {
	location := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	now := time.Date(2026, time.August, 11, 10, 0, 0, 0, location)
	from, to, err := periodForStudent(AttendanceFilter{Month: "2026-08"}, 15, now)
	if err != nil {
		t.Fatal(err)
	}
	if from.Format("2006-01-02") != "2026-07-15" || !to.Equal(now) {
		t.Fatalf("period: %v - %v", from, to)
	}
}

func TestPeriodForStudentAcceptsCustomInclusiveDates(t *testing.T) {
	now := time.Date(2026, time.August, 11, 10, 0, 0, 0, time.UTC)
	from, to, err := periodForStudent(AttendanceFilter{From: "2026-08-02", To: "2026-08-09"}, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if from.Format("2006-01-02") != "2026-08-02" || to.Format("2006-01-02") != "2026-08-10" {
		t.Fatalf("period: %v - %v", from, to)
	}
}

func TestSyncListsThreeCalendarMonths(t *testing.T) {
	repository := syncRepositoryStub{}
	meet := &syncMeetStub{}
	useCase := NewAttendanceUseCase(repository, repository, meet, 3)
	now := time.Date(2026, time.August, 11, 10, 30, 0, 0, time.UTC)
	if _, err := useCase.Sync(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.May, 11, 10, 30, 0, 0, time.UTC)
	if !meet.since.Equal(want) {
		t.Fatalf("sync since %v, want %v", meet.since, want)
	}
}
