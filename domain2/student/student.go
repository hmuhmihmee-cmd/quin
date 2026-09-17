package student

import (
	"errors"
	"fmt"
	"meet-attendance-clean/domain2/lesson"
	"strings"
	"time"
	"uuid"
)

type Student struct {
	id                 uuid.UUID
	name               string
	class              string
	studentWorkspaceID uuid.UUID
	teacherWorkspaceID uuid.UUID
	cycleStartDay      int
	createdAt          time.Time
}

func (s *Student) ID() uuid.UUID {
	return s.id
}
func (s *Student) Name() string {
	return s.name
}
func (s *Student) Class() string {
	return s.class
}
func (s *Student) StudentWorkspaceID() uuid.UUID {
	return s.studentWorkspaceID
}
func (s *Student) TeacherWorkspaceID() uuid.UUID {
	return s.teacherWorkspaceID
}
func (s *Student) CycleStartDay() int {
	return s.cycleStartDay
}
func (s *Student) CreatedAt() time.Time {
	return s.createdAt
}

func NewStudent(name, class string, cycleStartDay int) (*Student, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("tên học sinh không được để trống")
	}

	class = strings.TrimSpace(class)
	if class == "" {
		return nil, errors.New("lớp học không được để trống")
	}

	if cycleStartDay < 1 || cycleStartDay > 31 {
		return nil, errors.New("ngày chu kỳ học phí phải từ ngày 1 đến ngày 31")
	}

	return &Student{
		id:            uuid.New(),
		name:          name,
		class:         class,
		cycleStartDay: cycleStartDay,
		createdAt:     time.Now().UTC(),
	}, nil
}

type TargetLocation struct {
	WorkspaceID   *uuid.UUID
	WorkspaceName *string
	ChapterName   string
}

// GetPublishTarget tự động quyết định xem dùng ID cũ hay tạo Tên mới
func (s *Student) GetPublishTarget(audience lesson.Audience, chapterName string) TargetLocation {
	target := TargetLocation{
		ChapterName: chapterName,
	}

	if audience == lesson.AudienceStudent {
		if s.studentWorkspaceID != uuid.Nil() {
			target.WorkspaceID = &s.studentWorkspaceID
		} else {
			name := fmt.Sprintf("%s_HS", s.name) // Logic quy tắc đặt tên nằm ở đây!
			target.WorkspaceName = &name
		}
	} else {
		if s.teacherWorkspaceID != uuid.Nil() {
			target.WorkspaceID = &s.teacherWorkspaceID
		} else {
			name := fmt.Sprintf("%s_GV", s.name)
			target.WorkspaceName = &name
		}
	}
	return target
}

// SyncWorkspaceIDs lưu lại ID mới nếu OneNote vừa tạo vở xong
func (s *Student) SyncWorkspaceIDs(studentWsID, teacherWsID uuid.UUID) {
	if s.studentWorkspaceID == uuid.Nil() && studentWsID != uuid.Nil() {
		s.studentWorkspaceID = studentWsID
	}
	if s.teacherWorkspaceID == uuid.Nil() && teacherWsID != uuid.Nil() {
		s.teacherWorkspaceID = teacherWsID
	}
}
