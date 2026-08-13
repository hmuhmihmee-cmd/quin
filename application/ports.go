package application

import (
	"context"
	"time"

	"meet-attendance-clean/domain"
)

type ImportedMeeting struct {
	GoogleRecordName string
	GoogleSpaceName  string
	StartedAt        time.Time
	EndedAt          time.Time
	Participants     []domain.Participant
}

type StudentRepository interface {
	ListStudents(context.Context) ([]domain.Student, error)
	GetStudent(context.Context, int64) (domain.Student, error)
	UpdateStudent(context.Context, domain.Student) error
	CreateMissingStudents(context.Context) error
}

type AttendanceRepository interface {
	StudentMeetingStats(context.Context, string, time.Time, time.Time, int64) (int64, int64, error)
	StudentMeetings(context.Context, string, time.Time, time.Time, int64) ([]MeetingReport, error)
	SaveMeeting(context.Context, ImportedMeeting) error
}

type MeetGateway interface {
	IsConnected() bool
	AuthorizationURL(state string) (string, error)
	Exchange(context.Context, string) error
	Disconnect() error
	ListMeetings(context.Context, time.Time) ([]ImportedMeeting, error)
}

type LessonGenerator interface {
	Generate(context.Context, string, string) (*domain.LessonData, error)
}

type LessonDocumentGenerator interface {
	Draft(*domain.LessonData, string) domain.LessonDraft
	GeneratePDF(string, string, string) (domain.LessonFiles, error)
}
