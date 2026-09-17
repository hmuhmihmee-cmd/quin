package mistake

import (
	"errors"
	"meet-attendance-clean/domain2/lesson"
	"uuid"
)

type RemediationTask struct {
	mistakeID uuid.UUID
	studentID uuid.UUID
	lesson.LessonDraft
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

func (r *RemediationTask) MistakeID() uuid.UUID {
	return r.mistakeID
}
func (r *RemediationTask) StudentID() uuid.UUID {
	return r.studentID
}
