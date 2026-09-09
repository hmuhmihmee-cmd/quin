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
	if err = ensureMistakeSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("cập nhật schema mistake: %w", err)
	}
	if err = ensureAssignmentInfrastructureSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("cập nhật schema assignment: %w", err)
	}
	if err = normalizeMeetTimestampsToUTC(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("chuẩn hoá thời gian Meet sang UTC: %w", err)
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
		{"students", "meeting_code", "TEXT NOT NULL DEFAULT ''"},
		{"students", "space_name", "TEXT NOT NULL DEFAULT ''"},
		{"lesson_drafts", "lesson_data_json", "TEXT"},
		{"assignments", "origin_mistake_id", "INTEGER"},
		{"assignments", "depth", "INTEGER NOT NULL DEFAULT 0"},
		{"assignments", "student_page_web_url", "TEXT NOT NULL DEFAULT ''"},
		{"assignments", "teacher_page_web_url", "TEXT NOT NULL DEFAULT ''"},
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
	if err != nil {
		return err
	}

	// Các bài khắc phục đã tạo trước migration chưa lưu origin_mistake_id.
	// Khôi phục liên kết chỉ khi tiêu đề khớp duy nhất topic của một lỗi đang
	// khắc phục, để tránh gán nhầm dữ liệu lịch sử.
	_, err = db.Exec(`UPDATE assignments
		SET origin_mistake_id = (
			SELECT m.id
			FROM mistakes m
			WHERE m.status = 'remediating'
			  AND assignments.title = 'Bài khắc phục : ' || m.topic
			  AND (SELECT COUNT(*) FROM mistakes m2
			       WHERE m2.status = 'remediating'
			         AND assignments.title = 'Bài khắc phục : ' || m2.topic) = 1
		),
		depth = (
			SELECT m.depth + 1
			FROM mistakes m
			WHERE m.status = 'remediating'
			  AND assignments.title = 'Bài khắc phục : ' || m.topic
			  AND (SELECT COUNT(*) FROM mistakes m2
			       WHERE m2.status = 'remediating'
			         AND assignments.title = 'Bài khắc phục : ' || m2.topic) = 1
		)
		WHERE assignment_type = 'remediation'
		  AND origin_mistake_id IS NULL
		  AND EXISTS (
			SELECT 1 FROM mistakes m
			WHERE m.status = 'remediating'
			  AND assignments.title = 'Bài khắc phục : ' || m.topic
			  AND (SELECT COUNT(*) FROM mistakes m2
			       WHERE m2.status = 'remediating'
			         AND assignments.title = 'Bài khắc phục : ' || m2.topic) = 1
		);`)
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

// ensureMistakeSchema nâng cấp bảng mistakes cũ sang mô hình có phả hệ và
// trạng thái domain. CREATE TABLE IF NOT EXISTS không thể bổ sung các cột này
// cho database đã tồn tại, nên migration được giữ tại hạ tầng SQLite.
func ensureMistakeSchema(db *sql.DB) error {
	columns := []struct {
		column     string
		definition string
	}{
		{"source_assignment_id", "INTEGER NOT NULL DEFAULT 0"},
		{"parent_mistake_id", "INTEGER"},
		{"depth", "INTEGER NOT NULL DEFAULT 0"},
		{"status", "TEXT NOT NULL DEFAULT 'detected'"},
	}
	for _, item := range columns {
		exists, err := hasColumn(db, "mistakes", item.column)
		if err != nil {
			return err
		}
		if !exists {
			statement := fmt.Sprintf("ALTER TABLE mistakes ADD COLUMN %s %s", item.column, item.definition)
			if _, err := db.Exec(statement); err != nil {
				return err
			}
		}
	}

	hasLegacyAssignmentID, err := hasColumn(db, "mistakes", "assignment_id")
	if err != nil {
		return err
	}
	if hasLegacyAssignmentID {
		if _, err := db.Exec(`UPDATE mistakes
			SET source_assignment_id = assignment_id
			WHERE source_assignment_id = 0`); err != nil {
			return err
		}
	}
	hasLegacyIsResolved, err := hasColumn(db, "mistakes", "is_resolved")
	if err != nil {
		return err
	}
	if hasLegacyIsResolved {
		if _, err := db.Exec(`UPDATE mistakes
			SET status = 'resolved'
			WHERE is_resolved <> 0 AND status = 'detected'`); err != nil {
			return err
		}
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_mistakes_student_status
		ON mistakes(student_id, status, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_mistakes_parent ON mistakes(parent_mistake_id);`)
	return err
}

// Các phiên bản cũ lưu thời gian Meet dưới offset +07:00. Chúng vẫn biểu diễn
// đúng instant, nhưng migration này chuẩn hoá biểu diễn vật lý trong SQLite về
// UTC để toàn bộ dữ liệu Meet có cùng quy ước với dữ liệu mới.
func normalizeMeetTimestampsToUTC(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var migrated string
	err = tx.QueryRow(`SELECT value FROM system_settings WHERE key = 'meet_timestamps_utc_v1'`).Scan(&migrated)
	if err == nil && migrated == "1" {
		return tx.Commit()
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	type meetingTimestamp struct {
		id        int64
		startedAt time.Time
		endedAt   sql.NullTime
	}
	meetingRows, err := tx.Query(`SELECT id, started_at, ended_at FROM meetings`)
	if err != nil {
		return err
	}
	var meetings []meetingTimestamp
	for meetingRows.Next() {
		var item meetingTimestamp
		if err := meetingRows.Scan(&item.id, &item.startedAt, &item.endedAt); err != nil {
			meetingRows.Close()
			return err
		}
		meetings = append(meetings, item)
	}
	if err := meetingRows.Err(); err != nil {
		meetingRows.Close()
		return err
	}
	if err := meetingRows.Close(); err != nil {
		return err
	}
	for _, item := range meetings {
		if _, err := tx.Exec(`UPDATE meetings SET started_at = ?, ended_at = ? WHERE id = ?`,
			item.startedAt.UTC(), nullableTime(item.endedAt.Time.UTC()), item.id); err != nil {
			return err
		}
	}

	type participantTimestamp struct {
		id            int64
		firstJoinedAt time.Time
		lastLeftAt    sql.NullTime
	}
	participantRows, err := tx.Query(`SELECT id, first_joined_at, last_left_at FROM participants`)
	if err != nil {
		return err
	}
	var participants []participantTimestamp
	for participantRows.Next() {
		var item participantTimestamp
		if err := participantRows.Scan(&item.id, &item.firstJoinedAt, &item.lastLeftAt); err != nil {
			participantRows.Close()
			return err
		}
		participants = append(participants, item)
	}
	if err := participantRows.Err(); err != nil {
		participantRows.Close()
		return err
	}
	if err := participantRows.Close(); err != nil {
		return err
	}
	for _, item := range participants {
		if _, err := tx.Exec(`UPDATE participants SET first_joined_at = ?, last_left_at = ? WHERE id = ?`,
			item.firstJoinedAt.UTC(), nullableTime(item.lastLeftAt.Time.UTC()), item.id); err != nil {
			return err
		}
	}

	var lastSync string
	err = tx.QueryRow(`SELECT value FROM system_settings WHERE key = 'last_sync_time'`).Scan(&lastSync)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		if parsed, err := time.Parse(time.RFC3339, lastSync); err == nil {
			if _, err := tx.Exec(`UPDATE system_settings SET value = ? WHERE key = 'last_sync_time'`, parsed.UTC().Format(time.RFC3339)); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`INSERT INTO system_settings (key, value) VALUES ('meet_timestamps_utc_v1', '1')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		return err
	}

	return tx.Commit()
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
			MeetingCode:   student.MeetingCode,
			SpaceName:     student.SpaceName,
			CycleStartDay: int64(student.CycleStartDay),
		})
		if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("save student %s: %w", student.Class, err)
		}
	}

	// B. Lưu Meetings và Participants bằng Upsert của sqlc
	for _, meet := range meets {
		if err := qtx.UpdateStudentMeetIdentity(ctx, sqlcgen.UpdateStudentMeetIdentityParams{
			MeetingCode: meet.MeetingCode,
			SpaceName:   meet.SpaceName,
			Class:       meet.Class,
		}); err != nil {
			return fmt.Errorf("lưu nhận diện Meet %s: %w", meet.Class, err)
		}
		if err := qtx.UpdateDefaultStudentName(ctx, sqlcgen.UpdateDefaultStudentNameParams{
			Column1: strings.TrimSpace(meet.SpaceName),
			Name:    strings.TrimSpace(meet.SpaceName),
			PRINTF:  "Học sinh mới ",
			Class:   meet.Class,
			Name_2:  "Học sinh mới",
			Name_3:  "Học sinh mới [0-9]*",
		}); err != nil {
			return fmt.Errorf("đặt tên mặc định học sinh %s: %w", meet.Class, err)
		}
		durationMinutes := int(meet.EndedAt.Sub(meet.StartedAt).Minutes())
		if durationMinutes < 0 {
			durationMinutes = 0
		}

		meetingID, err := qtx.UpsertMeeting(ctx, sqlcgen.UpsertMeetingParams{
			Class:           meet.Class,
			Code:            meet.ID,
			StartedAt:       meet.StartedAt.UTC(),
			EndedAt:         nullableTime(meet.EndedAt.UTC()),
			DurationMinutes: int64(durationMinutes),
		})
		if err != nil {
			return fmt.Errorf("upsert meeting %s: %w", meet.ID, err)
		}

		for _, p := range meet.Participants {
			if err := qtx.UpsertParticipant(ctx, sqlcgen.UpsertParticipantParams{
				MeetingID:       meetingID,
				Name:            p.Name,
				FirstJoinedAt:   p.FirstJoinedAt.UTC(),
				LastLeftAt:      nullableTime(p.LastLeftAt.UTC()),
				DurationMinutes: int64(p.Duration),
			}); err != nil {
				return fmt.Errorf("upsert participant %s: %w", p.Name, err)
			}
		}
	}

	// C. Cập nhật mốc thời gian vừa đồng bộ
	nowStr := time.Now().UTC().Format(time.RFC3339)
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
		StartedAt:       filter.FromDate.UTC(),
		StartedAt_2:     filter.ToDate.UTC(),
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
			MeetingCode:          r.MeetingCode,
			SpaceName:            r.SpaceName,
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
		MeetingCode:        r.MeetingCode,
		SpaceName:          r.SpaceName,
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
		StartedAt:       filter.FromDate.UTC(),
		StartedAt_2:     filter.ToDate.UTC(),
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
