-- name: ListStudents :many
SELECT id, name, google_space_name, cycle_start_day
FROM students
ORDER BY name COLLATE NOCASE;

-- name: GetStudent :one
SELECT id, name, google_space_name, cycle_start_day
FROM students
WHERE id = ?;

-- name: CreateStudent :exec
INSERT INTO students (name, google_space_name, cycle_start_day)
VALUES (?, ?, ?);

-- name: CountStudents :one
SELECT COUNT(*) FROM students;

-- name: ListUnassignedSpaces :many
SELECT DISTINCT meetings.google_space_name
FROM meetings
LEFT JOIN students ON students.google_space_name = meetings.google_space_name
WHERE students.id IS NULL
ORDER BY meetings.google_space_name;

-- name: DeleteStudentsWithoutMeetings :exec
DELETE FROM students
WHERE NOT EXISTS (
    SELECT 1
    FROM meetings
    WHERE meetings.google_space_name = students.google_space_name
);

-- name: UpdateStudent :execrows
UPDATE students
SET name = ?, cycle_start_day = ?
WHERE id = ?;

-- name: UpsertMeeting :one
INSERT INTO meetings (
    google_record_name, google_space_name, started_at, ended_at
) VALUES (?, ?, ?, ?)
ON CONFLICT(google_record_name) DO UPDATE SET
    google_space_name = excluded.google_space_name,
    started_at = excluded.started_at,
    ended_at = COALESCE(excluded.ended_at, meetings.ended_at)
RETURNING id;

-- name: UpsertParticipant :exec
INSERT INTO participants (
    meeting_id, google_participant_name, google_user_name,
    display_name, joined_at, left_at, duration_minutes
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(google_participant_name) DO UPDATE SET
    meeting_id = excluded.meeting_id,
    google_user_name = excluded.google_user_name,
    display_name = excluded.display_name,
    joined_at = excluded.joined_at,
    left_at = excluded.left_at,
    duration_minutes = excluded.duration_minutes;

-- name: ListStudentMeetings :many
SELECT id, google_record_name, google_space_name, started_at, ended_at
FROM meetings
WHERE google_space_name = ?
  AND started_at >= ?
  AND started_at < ?
  AND ended_at IS NOT NULL
  AND unixepoch(ended_at) - unixepoch(started_at) >= sqlc.arg(min_duration_minutes) * 60
ORDER BY started_at DESC;

-- name: ListMeetingParticipants :many
SELECT id, meeting_id, google_participant_name, google_user_name,
       display_name, joined_at, left_at, duration_minutes
FROM participants
WHERE meeting_id = ?
ORDER BY duration_minutes DESC, display_name COLLATE NOCASE;
