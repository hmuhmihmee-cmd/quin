package domain

import "time"

type Meeting struct {
	ID           string        `json:"id"`
	Class        string        `json:"class"` // Stable Google Meet space ID (legacy column name).
	MeetingCode  string        `json:"meeting_code"`
	SpaceName    string        `json:"space_name"`
	StartedAt    time.Time     `json:"started_at"`
	EndedAt      time.Time     `json:"ended_at"`
	Participants []Participant `json:"participants"`
}

type Participant struct {
	Name          string    `json:"name"`
	FirstJoinedAt time.Time `json:"first_joined_at"`
	LastLeftAt    time.Time `json:"last_left_at"`
	Duration      int       `json:"duration"`
}
type Student struct {
	ID                 int     `json:"id"`
	Name               string  `json:"name"`
	Class              string  `json:"class"`
	MeetingCode        string  `json:"meeting_code"`
	SpaceName          string  `json:"space_name"`
	CycleStartDay      int     `json:"cycle_start_day"`
	StudentWorkspaceID *string `json:"student_workspace_id,omitempty"`
	TeacherWorkspaceID *string `json:"teacher_workspace_id,omitempty"`
}
