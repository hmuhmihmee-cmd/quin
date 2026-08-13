package googlemeet

import (
	"encoding/json"
	"testing"
	"time"
)

func TestConferenceRecordKeepsGoogleSpaceName(t *testing.T) {
	var response ListConferenceRecordsResponse
	err := json.Unmarshal([]byte(`{"conferenceRecords":[{"name":"conferenceRecords/record-1","startTime":"2026-08-04T17:07:06.269Z","endTime":"2026-08-04T18:07:06.269Z","space":"spaces/space-1"}]}`), &response)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ConferenceRecords) != 1 || response.ConferenceRecords[0].Space != "spaces/space-1" {
		t.Fatalf("conference records: %#v", response.ConferenceRecords)
	}
}

func TestTrimGoogleResourcePrefix(t *testing.T) {
	if got := trimResourcePrefix("conferenceRecords/record-1", "conferenceRecords/"); got != "record-1" {
		t.Fatalf("record ID: %q", got)
	}
	if got := trimResourcePrefix("spaces/space-1", "spaces/"); got != "space-1" {
		t.Fatalf("space ID: %q", got)
	}
}

func TestParticipantKeepsGoogleIdentities(t *testing.T) {
	person := Participant{
		Name:         "conferenceRecords/record-1/participants/participant-1",
		SignedInUser: &SignedInUser{User: "users/user-1", DisplayName: "Minh Anh"},
	}
	if person.googleUserName() != "users/user-1" || person.displayName() != "Minh Anh" {
		t.Fatalf("participant identity: %#v", person)
	}
}

func TestMergedDurationDoesNotDoubleCountOverlappingSessions(t *testing.T) {
	meetingStart := time.Date(2026, time.August, 6, 6, 0, 0, 0, time.UTC)
	meetingEnd := meetingStart.Add(125 * time.Minute)
	joinedAt, leftAt, minutes := mergedDuration([]timeInterval{
		{start: meetingStart, end: meetingEnd},
		{start: meetingStart.Add(10 * time.Minute), end: meetingStart.Add(120 * time.Minute)},
	}, meetingStart, meetingEnd)
	if minutes != 125 || !joinedAt.Equal(meetingStart) || !leftAt.Equal(meetingEnd) {
		t.Fatalf("got %v - %v, %d minutes", joinedAt, leftAt, minutes)
	}
}

func TestMergedDurationClipsSessionsToConference(t *testing.T) {
	meetingStart := time.Date(2026, time.August, 11, 5, 57, 0, 0, time.UTC)
	meetingEnd := meetingStart.Add(101 * time.Minute)
	_, _, minutes := mergedDuration([]timeInterval{
		{start: meetingStart.Add(-time.Hour), end: meetingEnd.Add(2 * time.Hour)},
	}, meetingStart, meetingEnd)
	if minutes != 101 {
		t.Fatalf("got %d minutes, want 101", minutes)
	}
}

func TestMergedDurationAddsSeparatedSessions(t *testing.T) {
	meetingStart := time.Date(2026, time.August, 11, 5, 0, 0, 0, time.UTC)
	meetingEnd := meetingStart.Add(2 * time.Hour)
	_, _, minutes := mergedDuration([]timeInterval{
		{start: meetingStart, end: meetingStart.Add(30 * time.Minute)},
		{start: meetingStart.Add(45 * time.Minute), end: meetingStart.Add(75 * time.Minute)},
	}, meetingStart, meetingEnd)
	if minutes != 60 {
		t.Fatalf("got %d minutes, want 60", minutes)
	}
}
