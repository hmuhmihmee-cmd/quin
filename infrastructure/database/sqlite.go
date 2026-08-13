package database

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"meet-attendance-clean/application"
	"meet-attendance-clean/domain"
	"meet-attendance-clean/infrastructure/database/sqlcgen"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type SQLite struct {
	db      *sql.DB
	queries *sqlcgen.Queries
}

func Open(path string) (*SQLite, error) {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	db, err := sql.Open("sqlite", path+separator+"_time_format=sqlite")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec("PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000; PRAGMA journal_mode = WAL;"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("khởi tạo cơ sở dữ liệu: %w", err)
	}
	return &SQLite{db: db, queries: sqlcgen.New(db)}, nil
}

func (s *SQLite) Close() error { return s.db.Close() }

func (s *SQLite) ListStudents(ctx context.Context) ([]domain.Student, error) {
	rows, err := s.queries.ListStudents(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Student, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentFromRow(row.ID, row.Name, row.GoogleSpaceName, row.CycleStartDay))
	}
	return result, nil
}

func (s *SQLite) GetStudent(ctx context.Context, id int64) (domain.Student, error) {
	row, err := s.queries.GetStudent(ctx, id)
	if err != nil {
		return domain.Student{}, err
	}
	return studentFromRow(row.ID, row.Name, row.GoogleSpaceName, row.CycleStartDay), nil
}

func (s *SQLite) UpdateStudent(ctx context.Context, student domain.Student) error {
	n, err := s.queries.UpdateStudent(ctx, sqlcgen.UpdateStudentParams{Name: student.Name, CycleStartDay: int64(student.CycleStartDay), ID: student.ID})
	if err == nil && n == 0 {
		return errors.New("không tìm thấy học sinh")
	}
	return err
}

func (s *SQLite) CreateMissingStudents(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)
	if err := queries.DeleteStudentsWithoutMeetings(ctx); err != nil {
		return err
	}
	spaces, err := queries.ListUnassignedSpaces(ctx)
	if err != nil {
		return err
	}
	studentCount, err := queries.CountStudents(ctx)
	if err != nil {
		return err
	}
	for index, space := range spaces {
		if err := queries.CreateStudent(ctx, sqlcgen.CreateStudentParams{
			Name:            fmt.Sprintf("Học sinh mới %d", studentCount+int64(index)+1),
			GoogleSpaceName: space,
			CycleStartDay:   1,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLite) SaveMeeting(ctx context.Context, item application.ImportedMeeting) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)
	meetingID, err := queries.UpsertMeeting(ctx, sqlcgen.UpsertMeetingParams{
		GoogleRecordName: item.GoogleRecordName, GoogleSpaceName: item.GoogleSpaceName,
		StartedAt: item.StartedAt, EndedAt: nullableTime(item.EndedAt),
	})
	if err != nil {
		return err
	}
	for _, participant := range item.Participants {
		if participant.GoogleParticipantName == "" || participant.JoinedAt.IsZero() {
			continue
		}
		if err := queries.UpsertParticipant(ctx, sqlcgen.UpsertParticipantParams{
			MeetingID: meetingID, GoogleParticipantName: participant.GoogleParticipantName,
			GoogleUserName: participant.GoogleUserName, DisplayName: participant.DisplayName,
			JoinedAt: participant.JoinedAt, LeftAt: nullableTime(participant.LeftAt),
			DurationMinutes: participant.DurationMinutes,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLite) StudentMeetingStats(ctx context.Context, googleSpaceName string, from, to time.Time, minDurationMinutes int64) (int64, int64, error) {
	rows, err := s.queries.ListStudentMeetings(ctx, sqlcgen.ListStudentMeetingsParams{GoogleSpaceName: googleSpaceName, StartedAt: from, StartedAt_2: to, MinDurationMinutes: minDurationMinutes})
	if err != nil {
		return 0, 0, err
	}
	var totalMinutes int64
	for _, row := range rows {
		if row.EndedAt.Valid && row.EndedAt.Time.After(row.StartedAt) {
			totalMinutes += int64(row.EndedAt.Time.Sub(row.StartedAt).Minutes())
		}
	}
	return int64(len(rows)), totalMinutes, nil
}

func (s *SQLite) StudentMeetings(ctx context.Context, googleSpaceName string, from, to time.Time, minDurationMinutes int64) ([]application.MeetingReport, error) {
	rows, err := s.queries.ListStudentMeetings(ctx, sqlcgen.ListStudentMeetingsParams{GoogleSpaceName: googleSpaceName, StartedAt: from, StartedAt_2: to, MinDurationMinutes: minDurationMinutes})
	if err != nil {
		return nil, err
	}
	result := make([]application.MeetingReport, 0, len(rows))
	for _, row := range rows {
		report := application.MeetingReport{MeetingID: row.ID, StartedAt: row.StartedAt}
		if row.EndedAt.Valid {
			report.EndedAt = row.EndedAt.Time
			if row.EndedAt.Time.After(row.StartedAt) {
				report.DurationMinutes = int64(row.EndedAt.Time.Sub(row.StartedAt).Minutes())
			}
		}
		participants, err := s.queries.ListMeetingParticipants(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		for _, participant := range participants {
			value := domain.Participant{
				ID: participant.ID, MeetingID: participant.MeetingID,
				GoogleParticipantName: participant.GoogleParticipantName,
				GoogleUserName:        participant.GoogleUserName, DisplayName: participant.DisplayName,
				JoinedAt: participant.JoinedAt, DurationMinutes: participant.DurationMinutes,
			}
			if participant.LeftAt.Valid {
				value.LeftAt = participant.LeftAt.Time
			}
			report.Participants = append(report.Participants, value)
		}
		result = append(result, report)
	}
	return result, nil
}

func studentFromRow(id int64, name, googleSpaceName string, cycleStartDay int64) domain.Student {
	return domain.Student{ID: id, Name: name, GoogleSpaceName: googleSpaceName, CycleStartDay: int(cycleStartDay)}
}

func nullableTime(value time.Time) sql.NullTime {
	return sql.NullTime{Time: value, Valid: !value.IsZero()}
}
