-- 1. Bảng cài đặt hệ thống (Lưu mốc thời gian đồng bộ, v.v.)
CREATE TABLE IF NOT EXISTS system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- 2. Bảng học sinh (quản lý theo mã lớp `class`)
CREATE TABLE IF NOT EXISTS students (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    class TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
	student_workspace_id TEXT,
	teacher_workspace_id TEXT,
    cycle_start_day INTEGER NOT NULL DEFAULT 1,
    CHECK (cycle_start_day BETWEEN 1 AND 31)
);

-- 3. Bảng các buổi học
CREATE TABLE IF NOT EXISTS meetings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    class TEXT NOT NULL,
    code TEXT NOT NULL UNIQUE, -- Mã định danh buổi học
    started_at DATETIME NOT NULL,
    ended_at DATETIME,
    duration_minutes INTEGER NOT NULL DEFAULT 0
);

-- 4. Bảng người tham gia từng buổi học
CREATE TABLE IF NOT EXISTS participants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    meeting_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    first_joined_at DATETIME NOT NULL,
    last_left_at DATETIME,
    duration_minutes INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (meeting_id) REFERENCES meetings(id) ON DELETE CASCADE,
    UNIQUE (meeting_id, name) -- Mỗi người chỉ xuất hiện 1 lần trong 1 buổi
);

-- Tạo Index để tăng tốc độ truy vấn lọc theo ngày và theo lớp
CREATE INDEX IF NOT EXISTS idx_meetings_class_time ON meetings(class, started_at);
CREATE INDEX IF NOT EXISTS idx_participants_meeting ON participants(meeting_id);


CREATE TABLE IF NOT EXISTS lesson_drafts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_url TEXT NOT NULL,
    custom_prompt TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'processing', -- 'processing', 'completed', 'failed'
    error_message TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    teacher_content TEXT NOT NULL DEFAULT '',
    student_content TEXT NOT NULL DEFAULT '',
    lesson_data_json TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_lesson_drafts_status ON lesson_drafts(status);

-- HẠ TẦNG MỚI: lưu vòng đời phiếu bài tập và kết quả chấm theo PageID OneNote.
CREATE TABLE IF NOT EXISTS assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    assignment_type TEXT NOT NULL DEFAULT 'normal',
    status TEXT NOT NULL DEFAULT 'pending',
    assigned_at DATETIME NOT NULL,
    assignee_id INTEGER NOT NULL,
    target_page_id TEXT NOT NULL UNIQUE,
    items_json TEXT NOT NULL DEFAULT '[]',
    FOREIGN KEY (assignee_id) REFERENCES students(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_assignments_assignee ON assignments(assignee_id, assigned_at DESC);
CREATE INDEX IF NOT EXISTS idx_assignments_status ON assignments(status);

-- HẠ TẦNG MỚI: ngân hàng lỗi sai theo học sinh và phiếu bài tập.
CREATE TABLE IF NOT EXISTS mistakes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id INTEGER NOT NULL,
    assignment_id INTEGER NOT NULL,
    topic TEXT NOT NULL,
    error_reason TEXT NOT NULL,
    is_resolved INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (assignment_id) REFERENCES assignments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_mistakes_student_resolved ON mistakes(student_id, is_resolved, created_at DESC);
