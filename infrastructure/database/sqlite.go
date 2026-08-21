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
		return nil, fmt.Errorf("khởi tạo schema: %w", err)
	}

	if err = ensureLessonDraftSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("cập nhật schema lesson_drafts: %w", err)
	}
	if err = ensureAssignmentInfrastructureSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("cập nhật schema assignment: %w", err)
	}

	return &SQLite{
		db:      db,
		queries: sqlcgen.New(db),
	}, nil
}

// HẠ TẦNG MỚI: bổ sung các cột liên kết OneNote/Assignment cho database cũ.
// CREATE TABLE IF NOT EXISTS không tự thêm cột vào bảng đã tồn tại nên migration
// nhỏ này vẫn cần chạy trước khi khởi tạo sqlc repository.
func ensureAssignmentInfrastructureSchema(db *sql.DB) error {
	columns := []struct {
		table      string
		column     string
		definition string
	}{
		{"students", "student_workspace_id", "TEXT"},
		{"students", "teacher_workspace_id", "TEXT"},
		{"lesson_drafts", "lesson_data_json", "TEXT"},
	}
	for _, item := range columns {
		exists, err := hasColumn(db, item.table, item.column)
		if err != nil {
			return err
		}
		if !exists {
			statement := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", item.table, item.column, item.definition)
			if _, err := db.Exec(statement); err != nil {
				return err
			}
		}
	}

	legacyWorkspaceColumn, err := hasColumn(db, "students", "workspace_id")
	if err != nil {
		return err
	}
	if legacyWorkspaceColumn {
		if _, err := db.Exec(`UPDATE students
			SET student_workspace_id = workspace_id
			WHERE (student_workspace_id IS NULL OR student_workspace_id = '')
			  AND workspace_id IS NOT NULL AND workspace_id <> ''`); err != nil {
			return err
		}
	}

	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_students_student_workspace_id
			ON students(student_workspace_id)
			WHERE student_workspace_id IS NOT NULL AND student_workspace_id <> '';
		CREATE UNIQUE INDEX IF NOT EXISTS idx_students_teacher_workspace_id
			ON students(teacher_workspace_id)
			WHERE teacher_workspace_id IS NOT NULL AND teacher_workspace_id <> '';`)
	return err
}

func ensureLessonDraftSchema(db *sql.DB) error {
	hasModel, err := hasColumn(db, "lesson_drafts", "model")
	if err != nil {
		return err
	}
	if !hasModel {
		if _, err := db.Exec("ALTER TABLE lesson_drafts ADD COLUMN model TEXT NOT NULL DEFAULT '';"); err != nil {
			return err
		}
	}
	return nil
}

func hasColumn(db *sql.DB, tableName, columnName string) (bool, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	return false, nil
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

// ==================== 1. IMPLEMENT SyncRepo ====================

func (s *SQLite) LastSync(ctx context.Context) (*time.Time, error) {
	val, err := s.queries.GetSetting(ctx, "last_sync_time")
	if err != nil {
		return nil, err
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ==================== 2. IMPLEMENT StudentRepo ====================

func (s *SQLite) Exits(ctx context.Context, class string) (bool, error) {
	exists, err := s.queries.CheckStudentExists(ctx, class)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *SQLite) SaveStudent(ctx context.Context, student domain.Student) error {
	n, err := s.queries.UpdateStudent(ctx, sqlcgen.UpdateStudentParams{
		Name:               student.Name,
		CycleStartDay:      int64(student.CycleStartDay),
		StudentWorkspaceID: nullableStringPointer(student.StudentWorkspaceID),
		TeacherWorkspaceID: nullableStringPointer(student.TeacherWorkspaceID),
		ID:                 int64(student.ID),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("Update khong thanh cong")
	}
	return nil
}

// ==================== 3. IMPLEMENT SaveMeetRepo ====================
var vnLocation = time.FixedZone("ICT", 7*3600)

func (s *SQLite) SaveMeets(ctx context.Context, meets []domain.Meeting, students []domain.Student) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := s.queries.WithTx(tx)

	// A. Lưu danh sách học sinh mới
	for _, student := range students {
		err := qtx.CreateStudent(ctx, sqlcgen.CreateStudentParams{
			Class:         student.Class,
			Name:          student.Name,
			CycleStartDay: int64(student.CycleStartDay),
		})
		if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("save student %s: %w", student.Class, err)
		}
	}

	// B. Lưu Meetings và Participants bằng Upsert của sqlc
	for _, meet := range meets {
		durationMinutes := int(meet.EndedAt.Sub(meet.StartedAt).Minutes())
		if durationMinutes < 0 {
			durationMinutes = 0
		}

		meetingID, err := qtx.UpsertMeeting(ctx, sqlcgen.UpsertMeetingParams{
			Class:           meet.Class,
			Code:            meet.ID,
			StartedAt:       meet.StartedAt,
			EndedAt:         nullableTime(meet.EndedAt),
			DurationMinutes: int64(durationMinutes),
		})
		if err != nil {
			return fmt.Errorf("upsert meeting %s: %w", meet.ID, err)
		}

		for _, p := range meet.Participants {
			if err := qtx.UpsertParticipant(ctx, sqlcgen.UpsertParticipantParams{
				MeetingID:       meetingID,
				Name:            p.Name,
				FirstJoinedAt:   p.FirstJoinedAt,
				LastLeftAt:      nullableTime(p.LastLeftAt),
				DurationMinutes: int64(p.Duration),
			}); err != nil {
				return fmt.Errorf("upsert participant %s: %w", p.Name, err)
			}
		}
	}

	// C. Cập nhật mốc thời gian vừa đồng bộ
	nowStr := time.Now().In(vnLocation).Format(time.RFC3339)
	if err := qtx.SetSetting(ctx, sqlcgen.SetSettingParams{
		Key:   "last_sync_time",
		Value: nowStr,
	}); err != nil {
		return fmt.Errorf("lưu mốc sync: %w", err)
	}

	return tx.Commit()
}

// ListStudents lấy danh sách học sinh kèm tổng số buổi và tổng thời lượng theo bộ lọc StudentFilter
func (s *SQLite) ListStudents(ctx context.Context, filter application.StudentFilter) ([]application.StudentSummary, error) {
	var searchParam sql.NullString
	if filter.SearchName != nil && *filter.SearchName != "" {
		searchParam = sql.NullString{String: *filter.SearchName, Valid: true}
	}

	rows, err := s.queries.ListStudentsWithStats(ctx, sqlcgen.ListStudentsWithStatsParams{
		StartedAt:       filter.FromDate,
		StartedAt_2:     filter.ToDate,
		DurationMinutes: int64(filter.MinDurationMinutes),
		Column4:         searchParam,
	})
	if err != nil {
		return nil, err
	}

	result := make([]application.StudentSummary, 0, len(rows))
	for _, r := range rows {
		result = append(result, application.StudentSummary{
			ID:                   int(r.ID),
			Class:                r.Class,
			Name:                 r.Name,
			CycleStartDay:        int(r.CycleStartDay),
			TotalSessions:        int(r.TotalSessions),
			TotalDurationMinutes: int(r.TotalDurationMinutes),
		})
	}
	return result, nil
}

func (s *SQLite) GetStudent(ctx context.Context, id int) (domain.Student, error) {
	r, err := s.queries.GetStudent(ctx, int64(id))
	if err != nil {
		return domain.Student{}, err
	}
	return domain.Student{
		ID:                 int(r.ID),
		Class:              r.Class,
		Name:               r.Name,
		CycleStartDay:      int(r.CycleStartDay),
		StudentWorkspaceID: stringPointer(r.StudentWorkspaceID),
		TeacherWorkspaceID: stringPointer(r.TeacherWorkspaceID),
	}, nil
}

func (s *SQLite) UpdateStudent(ctx context.Context, student domain.Student) error {
	rowsAffected, err := s.queries.UpdateStudent(ctx, sqlcgen.UpdateStudentParams{
		ID:                 int64(student.ID),
		Name:               student.Name,
		CycleStartDay:      int64(student.CycleStartDay),
		StudentWorkspaceID: nullableStringPointer(student.StudentWorkspaceID),
		TeacherWorkspaceID: nullableStringPointer(student.TeacherWorkspaceID),
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("không tìm thấy học sinh để cập nhật")
	}
	return nil
}

// ListClassMeetings lấy danh sách buổi học của 1 lớp theo MeetingFilter
func (s *SQLite) ListClassMeetings(ctx context.Context, class string, filter application.MeetingFilter) ([]domain.Meeting, error) {
	rows, err := s.queries.ListClassMeetings(ctx, sqlcgen.ListClassMeetingsParams{
		Class:           class,
		StartedAt:       filter.FromDate,
		StartedAt_2:     filter.ToDate,
		DurationMinutes: int64(filter.MinDurationMinutes),
	})
	if err != nil {
		return nil, err
	}

	meetings := make([]domain.Meeting, 0, len(rows))
	for _, r := range rows {
		pRows, err := s.queries.ListMeetingParticipants(ctx, r.ID)
		if err != nil {
			return nil, err
		}

		participants := make([]domain.Participant, 0, len(pRows))
		for _, p := range pRows {
			participants = append(participants, domain.Participant{
				Name:          p.Name,
				FirstJoinedAt: p.FirstJoinedAt,
				LastLeftAt:    p.LastLeftAt.Time,
				Duration:      int(p.DurationMinutes),
			})
		}

		meetings = append(meetings, domain.Meeting{
			ID:           r.Code,
			Class:        r.Class,
			StartedAt:    r.StartedAt,
			EndedAt:      r.EndedAt.Time,
			Participants: participants,
		})
	}

	return meetings, nil
}

// ==================== TIỆN ÍCH DÙNG CHUNG ====================

func nullableTime(value time.Time) sql.NullTime {
	return sql.NullTime{
		Time:  value,
		Valid: !value.IsZero(),
	}
}

func nullableStringPointer(value *string) sql.NullString {
	if value == nil || strings.TrimSpace(*value) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: strings.TrimSpace(*value), Valid: true}
}

func stringPointer(value sql.NullString) *string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	return &trimmed
}
