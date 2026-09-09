

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
INSERT INTO students (class, name, meeting_code, space_name, cycle_start_day)
VALUES (?, ?, ?, ?, ?);

-- name: UpdateStudentMeetIdentity :exec
UPDATE students
SET meeting_code = ?, space_name = ?
WHERE class = ?;

-- name: UpdateDefaultStudentName :exec
UPDATE students
SET name = CASE
    WHEN ? <> '' THEN ?
    ELSE printf('%s%d', ?, id)
END
WHERE class = ?
  AND (name = ? OR name GLOB ?);

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
SELECT id, class, name, meeting_code, space_name, cycle_start_day, student_workspace_id, teacher_workspace_id
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
	s.meeting_code,
	s.space_name,
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
	title, assignment_type, status, assigned_at, assignee_id, target_page_id, student_page_web_url, teacher_page_web_url, items_json,
    origin_mistake_id, depth
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(target_page_id) DO UPDATE SET
    title = excluded.title,
    assignment_type = excluded.assignment_type,
    status = excluded.status,
    assigned_at = excluded.assigned_at,
    assignee_id = excluded.assignee_id,
	student_page_web_url = excluded.student_page_web_url,
	teacher_page_web_url = excluded.teacher_page_web_url,
    items_json = excluded.items_json,
    origin_mistake_id = excluded.origin_mistake_id,
    depth = excluded.depth
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
	a.student_page_web_url,
	a.teacher_page_web_url,
    a.items_json,
    a.origin_mistake_id,
    a.depth,
    s.id AS student_id,
    s.class AS student_class,
    s.name AS student_name,
    s.cycle_start_day AS student_cycle_start_day,
    s.student_workspace_id,
    s.teacher_workspace_id
FROM assignments a
JOIN students s ON s.id = a.assignee_id
WHERE a.target_page_id = ?;

-- Persist one detected mistake, including its remediation ancestry.
-- name: CreateMistake :one
INSERT INTO mistakes (
    student_id, source_assignment_id, parent_mistake_id, depth,
    topic, error_reason, status, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- NEW UI INFRASTRUCTURE: aggregate assignments for the grading screen.
-- name: ListAssignmentsForGrading :many
SELECT
    a.id,
    a.title,
    a.status,
    a.assigned_at,
    a.target_page_id,
	a.student_page_web_url,
	a.teacher_page_web_url,
    a.items_json,
    s.id AS student_id,
    s.name AS student_name
FROM assignments a
JOIN students s ON s.id = a.assignee_id
ORDER BY a.assigned_at DESC;

-- List mistakes together with their student.
-- name: ListMistakesWithStudents :many
SELECT
    m.id,
    m.student_id,
    m.source_assignment_id,
    m.parent_mistake_id,
    m.depth,
    m.topic,
    m.error_reason,
    m.status,
    m.created_at,
    s.name AS student_name
FROM mistakes m
JOIN students s ON s.id = m.student_id
ORDER BY s.name ASC, m.created_at DESC;

-- Load the selected mistake and the student who owns it.
-- name: GetMistakeWithStudent :one
SELECT
    m.id,
    m.student_id,
    m.source_assignment_id,
    m.parent_mistake_id,
    m.depth,
    m.topic,
    m.error_reason,
    m.status,
    m.created_at,
    s.class AS student_class,
    s.name AS student_name,
    s.cycle_start_day AS student_cycle_start_day,
    s.student_workspace_id,
    s.teacher_workspace_id
FROM mistakes m
JOIN students s ON s.id = m.student_id
WHERE m.id = ?;

-- Load one mistake for status transitions.
-- name: GetMistakeByID :one
SELECT id, source_assignment_id, parent_mistake_id, depth, topic, error_reason, status, created_at
FROM mistakes
WHERE id = ?;

-- Load ancestors from root to the direct parent of the requested mistake.
-- name: ListMistakeAncestry :many
WITH RECURSIVE ancestry(id, source_assignment_id, parent_mistake_id, depth, topic, error_reason, status, created_at, distance) AS (
    SELECT m.id, m.source_assignment_id, m.parent_mistake_id, m.depth, m.topic, m.error_reason, m.status, m.created_at, 0
    FROM mistakes m
    WHERE m.id = ?
    UNION ALL
    SELECT parent.id, parent.source_assignment_id, parent.parent_mistake_id, parent.depth,
           parent.topic, parent.error_reason, parent.status, parent.created_at, ancestry.distance + 1
    FROM mistakes parent
    JOIN ancestry ON ancestry.parent_mistake_id = parent.id
)
SELECT id, source_assignment_id, parent_mistake_id, depth, topic, error_reason, status, created_at
FROM ancestry
WHERE distance > 0
ORDER BY distance DESC;

-- Persist all domain-owned fields after a lifecycle transition.
-- name: UpdateMistake :execrows
UPDATE mistakes
SET source_assignment_id = ?,
    parent_mistake_id = ?,
    depth = ?,
    topic = ?,
    error_reason = ?,
    status = ?,
    created_at = ?
WHERE id = ?;

-- NEW UI INFRASTRUCTURE: populate compact student selectors.
-- name: ListStudentChoices :many
SELECT id, name, class, meeting_code, space_name, student_workspace_id, teacher_workspace_id
FROM students
ORDER BY name ASC;
