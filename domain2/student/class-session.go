package student

import (
	"errors"
	"strings"
	"time"
	"uuid"
)

type AttendanceRecord struct {
	studentName   string
	firstJoinedAt time.Time
	lastLeaveAt   time.Time
	durationMin   int
}

func newAttendanceRecord(name string, firstJoinedAt time.Time, lastLeaveAt time.Time, durationMin int) AttendanceRecord {
	return AttendanceRecord{
		studentName:   name,
		firstJoinedAt: firstJoinedAt,
		lastLeaveAt:   lastLeaveAt,
		durationMin:   durationMin,
	}
}
func (a *AttendanceRecord) StudentName() string {
	return a.studentName
}
func (a *AttendanceRecord) FirstJoinedAt() time.Time {
	return a.firstJoinedAt
}
func (a *AttendanceRecord) LastLeaveAt() time.Time {
	return a.lastLeaveAt
}
func (a *AttendanceRecord) DurationMin() int {
	return a.durationMin
}

type SessionPlatformRef struct {
	platform    string // "GOOGLE_MEET", "ZOOM", "OFFLINE"
	sessionCode string // Mã phòng học ngoài (abc-xyz hoặc Zoom ID)
}

type ClassSession struct {
	id          uuid.UUID
	startAt     time.Time
	endAt       time.Time
	attendance  []AttendanceRecord
	platformRef SessionPlatformRef
}

func NewClassSession(startTime time.Time, endTime time.Time) (*ClassSession, error) {
	if startTime.After(endTime) {
		return nil, errors.New("thời gian bắt đầu phải nhỏ hơn thời gian kết thúc")
	}
	return &ClassSession{
		id:         uuid.New(),
		startAt:    startTime,
		endAt:      endTime,
		attendance: make([]AttendanceRecord, 0),
	}, nil
}
func (s *ClassSession) AddAttendance(name string, firstJoinedAt time.Time, lastLeaveAt time.Time, durationMin int) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("tên học sinh không được để trống")
	}
	found := false
	for i := range s.attendance {
		if s.attendance[i].studentName == name {
			// Nếu thời điểm vào mới sớm hơn thời điểm vào cũ -> Cập nhật sớm hơn
			if firstJoinedAt.Before(s.attendance[i].firstJoinedAt) {
				s.attendance[i].firstJoinedAt = firstJoinedAt
			}
			// Nếu thời điểm rời mới muộn hơn thời điểm rời cũ -> Cập nhật muộn hơn
			if lastLeaveAt.After(s.attendance[i].lastLeaveAt) {
				s.attendance[i].lastLeaveAt = lastLeaveAt
			}
			// Cộng dồn thời lượng
			s.attendance[i].durationMin += durationMin
			return nil
		}
	}

	if !found {
		s.attendance = append(s.attendance, newAttendanceRecord(name, firstJoinedAt, lastLeaveAt, durationMin))
	}
	return nil
}
func ReconstituteClassSession(
	id uuid.UUID,
	startTime time.Time,
	endTime time.Time,
	attendance []AttendanceRecord,
) *ClassSession {
	return &ClassSession{
		id:         id, // 👈 Nhận ID đã có từ SQLite
		startAt:    startTime,
		endAt:      endTime,
		attendance: attendance,
	}
}

func (s *ClassSession) ID() uuid.UUID        { return s.id }
func (s *ClassSession) StartTime() time.Time { return s.startAt }
func (s *ClassSession) EndTime() time.Time   { return s.endAt }
func (s *ClassSession) Attendance() []AttendanceRecord {
	copied := make([]AttendanceRecord, len(s.attendance))
	copy(copied, s.attendance)
	return copied
}
