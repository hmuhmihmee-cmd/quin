package domain

import "time"

type Meeting struct {
	ID               int64     `json:"id"`
	GoogleRecordName string    `json:"google_record_name"`
	GoogleSpaceName  string    `json:"google_space_name"`
	StartedAt        time.Time `json:"started_at"`
	EndedAt          time.Time `json:"ended_at"`
}

type Participant struct {
	ID                    int64     `json:"id"`
	MeetingID             int64     `json:"meeting_id"`
	GoogleParticipantName string    `json:"google_participant_name"`
	GoogleUserName        string    `json:"google_user_name"`
	DisplayName           string    `json:"display_name"`
	JoinedAt              time.Time `json:"joined_at"`
	LeftAt                time.Time `json:"left_at"`
	DurationMinutes       int64     `json:"duration_minutes"`
}
