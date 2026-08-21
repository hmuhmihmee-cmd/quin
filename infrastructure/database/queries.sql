

-- name: GetSetting :one
SELECT value FROM system_settings WHERE key = ?;

-- name: SetSetting :exec
INSERT INTO system_settings (key, value)
VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

-- name: CheckStudentExists :one
SELECT EXISTS(
    SELECT 1 FROM students WHERE class = ?
);

-- name: CreateStudent :exec
INSERT INTO students (class, name, cycle_start_day)
VALUES (?, ?, ?);

-- name: UpsertMeeting :one
INSERT INTO meetings (
    class, code, started_at, ended_at, duration_minutes
) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(code) DO UPDATE SET
    class = excluded.class,
    started_at = excluded.started_at,
    ended_at = excluded.ended_at,
    duration_minutes = excluded.duration_minutes
RETURNING id;

-- name: UpsertParticipant :exec
INSERT INTO participants (
    meeting_id, name, first_joined_at, last_left_at, duration_minutes
) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(meeting_id, name) DO UPDATE SET
    first_joined_at = excluded.first_joined_at,
    last_left_at = excluded.last_left_at,
    duration_minutes = excluded.duration_minutes;




-- name: GetStudent :one
SELECT id, class, name, cycle_start_day, student_workspace_id, teacher_workspace_id
FROM students
WHERE id = ?;

-- name: UpdateStudent :execrows
UPDATE students
SET name = ?, cycle_start_day = ?, student_workspace_id = ?, teacher_workspace_id = ?
WHERE id = ?;

-- name: ListClassMeetings :many
SELECT id, class, code, started_at, ended_at, duration_minutes
FROM meetings
WHERE class = ?
  AND started_at >= ?
  AND started_at <= ?
  AND duration_minutes >= ?
ORDER BY started_at DESC;

-- name: ListMeetingParticipants :many
SELECT id, meeting_id, name, first_joined_at, last_left_at, duration_minutes
FROM participants
WHERE meeting_id = ?
ORDER BY duration_minutes DESC, name ASC;


-- name: ListStudentsWithStats :many
SELECT
    s.id,
    s.class,
    s.name,
    s.cycle_start_day,
    COUNT(m.id) AS total_sessions,
    CAST(COALESCE(SUM(m.duration_minutes), 0) AS INTEGER) AS total_duration_minutes
FROM students s
LEFT JOIN meetings m ON s.class = m.class
    AND m.started_at >= ?
    AND m.started_at <= ?
    AND m.duration_minutes >= ?
WHERE s.name LIKE '%' || COALESCE(?, '') || '%'
GROUP BY s.id
ORDER BY s.id ASC;


-- name: CreateLessonDraft :one
INSERT INTO lesson_drafts (
    source_url, custom_prompt, model, status, created_at
) VALUES (?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateLessonDraftStatus :exec
UPDATE lesson_drafts
SET status = ?, error_message = ?, updated_at = ?
WHERE id = ?;

-- name: UpdateLessonDraftGeneratedContent :exec
UPDATE lesson_drafts
SET status = 'completed',
    title = ?,
    teacher_content = ?,
    student_content = ?,
    error_message = '',
    updated_at = ?
WHERE id = ?;

-- name: UpdateLessonDraftUserContent :exec
UPDATE lesson_drafts
SET title = ?,
    teacher_content = ?,
    student_content = ?,
    updated_at = ?
WHERE id = ?;

-- NEW INFRASTRUCTURE: persist the structured Lesson as JSON.
-- name: UpdateLessonDraftLessonData :exec
UPDATE lesson_drafts
SET status = ?,
    title = ?,
    source_url = ?,
    custom_prompt = ?,
    model = ?,
    lesson_data_json = ?,
    error_message = ?,
    updated_at = ?
WHERE id = ?;

-- name: GetLessonDraft :one
SELECT id, source_url, custom_prompt, status, error_message,
       title, created_at, updated_at, model, lesson_data_json
FROM lesson_drafts
WHERE id = ?;

-- name: ListLessonDrafts :many
SELECT id, source_url, custom_prompt, model, status, error_message,
       title, created_at, updated_at, lesson_data_json
FROM lesson_drafts
ORDER BY created_at DESC;

-- name: DeleteLessonDraft :exec
DELETE FROM lesson_drafts
WHERE id = ?;

-- NEW INFRASTRUCTURE: insert/update Assignment by OneNote PageID.
-- name: SaveAssignment :one
INSERT INTO assignments (
    title, assignment_type, status, assigned_at, assignee_id, target_page_id, items_json
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(target_page_id) DO UPDATE SET
    title = excluded.title,
    assignment_type = excluded.assignment_type,
    status = excluded.status,
    assigned_at = excluded.assigned_at,
    assignee_id = excluded.assignee_id,
    items_json = excluded.items_json
RETURNING id;

-- NEW INFRASTRUCTURE: load Assignment and student by OneNote PageID.
-- name: GetAssignmentByPageID :one
SELECT
    a.id,
    a.title,
    a.assignment_type,
    a.status,
    a.assigned_at,
    a.target_page_id,
    a.items_json,
    s.id AS student_id,
    s.class AS student_class,
    s.name AS student_name,
    s.cycle_start_day AS student_cycle_start_day,
    s.student_workspace_id,
    s.teacher_workspace_id
FROM assignments a
JOIN students s ON s.id = a.assignee_id
WHERE a.target_page_id = ?;

-- NEW INFRASTRUCTURE: persist one detected mistake through sqlc.
-- name: CreateMistake :one
INSERT INTO mistakes (
    student_id, assignment_id, topic, error_reason, is_resolved, created_at
) VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- NEW UI INFRASTRUCTURE: aggregate assignments for the grading screen.
-- name: ListAssignmentsForGrading :many
SELECT
    a.id,
    a.title,
    a.status,
    a.assigned_at,
    a.target_page_id,
    a.items_json,
    s.id AS student_id,
    s.name AS student_name
FROM assignments a
JOIN students s ON s.id = a.assignee_id
ORDER BY a.assigned_at DESC;

-- NEW UI INFRASTRUCTURE: list mistakes together with their student.
-- name: ListMistakesWithStudents :many
SELECT
    m.id,
    m.student_id,
    m.assignment_id,
    m.topic,
    m.error_reason,
    m.is_resolved,
    m.created_at,
    s.name AS student_name
FROM mistakes m
JOIN students s ON s.id = m.student_id
ORDER BY s.name ASC, m.created_at DESC;

-- NEW UI INFRASTRUCTURE: load all data needed to create remediation work.
-- name: GetMistakeContext :one
SELECT
    m.id,
    m.student_id,
    m.assignment_id,
    m.topic,
    m.error_reason,
    m.is_resolved,
    m.created_at,
    s.class AS student_class,
    s.name AS student_name,
    s.cycle_start_day AS student_cycle_start_day,
    s.student_workspace_id,
    s.teacher_workspace_id
FROM mistakes m
JOIN students s ON s.id = m.student_id
WHERE m.id = ?;

-- NEW UI INFRASTRUCTURE: close a mistake after remediation is assigned.
-- name: MarkMistakeResolved :execrows
UPDATE mistakes
SET is_resolved = 1
WHERE id = ?;

-- NEW UI INFRASTRUCTURE: populate compact student selectors.
-- name: ListStudentChoices :many
SELECT id, name, class, student_workspace_id, teacher_workspace_id
FROM students
ORDER BY name ASC;
