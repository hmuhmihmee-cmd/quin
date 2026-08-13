package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"meet-attendance-clean/application"
	"meet-attendance-clean/domain"
)

func TestSQLiteStudentMeetingFlow(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "attendance.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Date(2026, time.August, 10, 13, 0, 0, 0, time.UTC)
	err = store.SaveMeeting(ctx, application.ImportedMeeting{
		GoogleRecordName: "abc", GoogleSpaceName: "room",
		StartedAt: start, EndedAt: start.Add(time.Hour),
		Participants: []domain.Participant{
			{GoogleParticipantName: "abc/participants/student", GoogleUserName: "users/123", DisplayName: "Minh Anh", JoinedAt: start, LeftAt: start.Add(58 * time.Minute), DurationMinutes: 58},
			{GoogleParticipantName: "abc/participants/teacher", GoogleUserName: "users/456", DisplayName: "Minh Anh", JoinedAt: start, LeftAt: start.Add(time.Hour), DurationMinutes: 60},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMeeting(ctx, application.ImportedMeeting{
		GoogleRecordName: "def", GoogleSpaceName: "another-room",
		StartedAt: start.Add(24 * time.Hour), EndedAt: start.Add(25 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMissingStudents(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMissingStudents(ctx); err != nil {
		t.Fatal(err)
	}
	students, err := store.ListStudents(ctx)
	if err != nil || len(students) != 2 {
		t.Fatalf("students: %#v, %v", students, err)
	}
	var student domain.Student
	for _, candidate := range students {
		if candidate.GoogleSpaceName == "room" {
			student = candidate
		}
		if candidate.CycleStartDay != 1 {
			t.Fatalf("cycle start day: %#v", candidate)
		}
	}
	if student.ID == 0 {
		t.Fatalf("missing student for room: %#v", students)
	}
	student.Name = "Minh Anh"
	student.GoogleSpaceName = "tampered-space"
	student.CycleStartDay = 5
	if err := store.UpdateStudent(ctx, student); err != nil {
		t.Fatal(err)
	}
	student, err = store.GetStudent(ctx, student.ID)
	if err != nil {
		t.Fatal(err)
	}
	if student.GoogleSpaceName != "room" || student.Name != "Minh Anh" || student.CycleStartDay != 5 {
		t.Fatalf("updated student: %#v", student)
	}

	count, minutes, err := store.StudentMeetingStats(ctx, student.GoogleSpaceName, start.Add(-time.Hour), start.Add(2*time.Hour), 0)
	if err != nil || count != 1 || minutes != 60 {
		t.Fatalf("stats: %d, %d, %v", count, minutes, err)
	}
	reports, err := store.StudentMeetings(ctx, student.GoogleSpaceName, start.Add(-time.Hour), start.Add(2*time.Hour), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Participants) != 2 || reports[0].DurationMinutes != 60 {
		t.Fatalf("reports: %#v", reports)
	}
	store.Close()

	// Schema initialization must be safe on every application restart.
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.GetStudent(ctx, student.ID); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRemovesGoogleResourcePrefixes(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "attendance.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.August, 10, 13, 0, 0, 0, time.UTC)
	result, err := store.db.ExecContext(ctx, `
		INSERT INTO meetings (google_record_name, google_space_name, started_at, ended_at)
		VALUES ('conferenceRecords/record-1', 'spaces/space-1', ?, ?);
		INSERT INTO students (name, google_space_name, cycle_start_day)
		VALUES ('Minh Anh', 'spaces/space-1', 1);`, start, start.Add(time.Hour))
	_ = result
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	students, err := store.ListStudents(ctx)
	if err != nil || len(students) != 1 || students[0].GoogleSpaceName != "space-1" {
		t.Fatalf("students after migration: %#v, %v", students, err)
	}
	meetings, err := store.StudentMeetings(ctx, "space-1", start.Add(-time.Hour), start.Add(2*time.Hour), 0)
	if err != nil || len(meetings) != 1 {
		t.Fatalf("meetings after migration: %#v, %v", meetings, err)
	}
}

func TestMeetingDurationFilterKeepsAllRowsButFiltersQueries(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "attendance.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	start := time.Date(2026, time.August, 10, 13, 0, 0, 0, time.UTC)
	for _, meeting := range []application.ImportedMeeting{
		{GoogleRecordName: "short", GoogleSpaceName: "room", StartedAt: start, EndedAt: start.Add(29 * time.Minute)},
		{GoogleRecordName: "boundary", GoogleSpaceName: "room", StartedAt: start.Add(time.Hour), EndedAt: start.Add(90 * time.Minute)},
	} {
		if err := store.SaveMeeting(ctx, meeting); err != nil {
			t.Fatal(err)
		}
	}

	from, to := start.Add(-time.Hour), start.Add(2*time.Hour)
	filtered, err := store.StudentMeetings(ctx, "room", from, to, 30)
	if err != nil || len(filtered) != 1 || filtered[0].DurationMinutes != 30 {
		t.Fatalf("filtered meetings: %#v, %v", filtered, err)
	}
	all, err := store.StudentMeetings(ctx, "room", from, to, 0)
	if err != nil || len(all) != 2 {
		t.Fatalf("all meetings: %#v, %v", all, err)
	}
	count, minutes, err := store.StudentMeetingStats(ctx, "room", from, to, 30)
	if err != nil || count != 1 || minutes != 30 {
		t.Fatalf("filtered stats: count=%d minutes=%d err=%v", count, minutes, err)
	}
}
