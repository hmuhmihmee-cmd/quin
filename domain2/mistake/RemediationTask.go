package mistake

import (
	"errors"
	"meet-attendance-clean/domain2/lesson"
	"uuid"
)

type RemediationStatus string

const (
	RemediationStatusDeteted    RemediationStatus = "deteted"
	RemediationStatusInProgress RemediationStatus = "in_progress"
	RemediationStatusResolved   RemediationStatus = "resolved"
)

type RemediationTask struct {
	mistakeID uuid.UUID
	studentID uuid.UUID
	lesson.LessonDraft
	assignmentID *uuid.UUID
	status       RemediationStatus
}

func (t *RemediationTask) ID() uuid.UUID {
	return t.LessonDraft.ID()
}
func (t *RemediationTask) MistakeID() uuid.UUID {
	return t.mistakeID
}
func (t *RemediationTask) StudentID() uuid.UUID {
	return t.studentID
}

//	func (t *RemediationTask) LessonDraft() lesson.LessonDraft {
//		return t.LessonDraft
//	}
func (t *RemediationTask) AssignmentID() *uuid.UUID {
	return t.assignmentID
}
func (t *RemediationTask) Status() RemediationStatus {
	return t.status
}

func NewRemediationTask(mistakeID uuid.UUID, studentID uuid.UUID, draft lesson.LessonDraft) (*RemediationTask, error) {
	if mistakeID == uuid.Nil() {
		return nil, errors.New("mistake ID không được định nghĩa")
	}
	if studentID == uuid.Nil() {
		return nil, errors.New("student ID không được định nghĩa")
	}
	return &RemediationTask{
		mistakeID:   mistakeID,
		studentID:   studentID,
		LessonDraft: draft,
	}, nil
}

func (t *RemediationTask) MarkAsAssigned(assignmentID uuid.UUID) {
	t.assignmentID = &assignmentID
	t.status = RemediationStatusInProgress
}

// Khi bài tập được chấm điểm 100%, Task hoàn thành sứ mệnh!
func (t *RemediationTask) MarkAsCompleted() {
	t.status = RemediationStatusResolved
}
