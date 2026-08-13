package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestMeetingDurationFilterDefaultsToThirtyMinutes(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	if got := filterFromRequest(request).MinDurationMinutes; got != 30 {
		t.Fatalf("default minimum duration = %d, want 30", got)
	}

	request = httptest.NewRequest("GET", "/?min_duration=0", nil)
	if got := filterFromRequest(request).MinDurationMinutes; got != 0 {
		t.Fatalf("selected minimum duration = %d, want 0", got)
	}
}
