CREATE TABLE IF NOT EXISTS students (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    google_space_name TEXT NOT NULL UNIQUE,
    cycle_start_day INTEGER NOT NULL DEFAULT 1
        CHECK (cycle_start_day BETWEEN 1 AND 31)
);

CREATE TABLE IF NOT EXISTS meetings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    google_record_name TEXT NOT NULL UNIQUE,
    google_space_name TEXT NOT NULL,
    started_at DATETIME NOT NULL,
    ended_at DATETIME
);

CREATE TABLE IF NOT EXISTS participants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    meeting_id INTEGER NOT NULL,
    google_participant_name TEXT NOT NULL,
    google_user_name TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL,
    joined_at DATETIME NOT NULL,
    left_at DATETIME,
    duration_minutes INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (meeting_id) REFERENCES meetings(id) ON DELETE CASCADE,
    UNIQUE (google_participant_name)
);

-- The SQLite driver used by older builds wrote time.Time using Go's display
-- format (for example "2026-08-10 13:00:00 +0000 UTC"). Normalize those
-- values once so SQLite date functions can calculate meeting durations.
UPDATE meetings
SET started_at = substr(started_at, 1, 19) || substr(started_at, 21, 3) || ':' || substr(started_at, 24, 2)
WHERE length(started_at) >= 25 AND substr(started_at, 20, 1) = ' '
  AND substr(started_at, 21, 1) IN ('+', '-');
UPDATE meetings
SET ended_at = substr(ended_at, 1, 19) || substr(ended_at, 21, 3) || ':' || substr(ended_at, 24, 2)
WHERE length(ended_at) >= 25 AND substr(ended_at, 20, 1) = ' '
  AND substr(ended_at, 21, 1) IN ('+', '-');
UPDATE participants
SET joined_at = substr(joined_at, 1, 19) || substr(joined_at, 21, 3) || ':' || substr(joined_at, 24, 2)
WHERE length(joined_at) >= 25 AND substr(joined_at, 20, 1) = ' '
  AND substr(joined_at, 21, 1) IN ('+', '-');
UPDATE participants
SET left_at = substr(left_at, 1, 19) || substr(left_at, 21, 3) || ':' || substr(left_at, 24, 2)
WHERE length(left_at) >= 25 AND substr(left_at, 20, 1) = ' '
  AND substr(left_at, 21, 1) IN ('+', '-');

-- Older builds stored complete Google resource names. Keep only their IDs.
UPDATE meetings
SET google_record_name = substr(google_record_name, length('conferenceRecords/') + 1)
WHERE google_record_name LIKE 'conferenceRecords/%';

UPDATE meetings
SET google_space_name = substr(google_space_name, length('spaces/') + 1)
WHERE google_space_name LIKE 'spaces/%';

UPDATE students
SET google_space_name = substr(google_space_name, length('spaces/') + 1)
WHERE google_space_name LIKE 'spaces/%';

UPDATE participants
SET google_participant_name = substr(google_participant_name, length('conferenceRecords/') + 1)
WHERE google_participant_name LIKE 'conferenceRecords/%';

CREATE INDEX IF NOT EXISTS idx_meetings_space_time ON meetings(google_space_name, started_at);
CREATE INDEX IF NOT EXISTS idx_participants_meeting ON participants(meeting_id);
