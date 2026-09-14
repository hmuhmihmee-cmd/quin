package lesson

import (
	"time"
	"uuid"
)

type LessonDraftStatus string

const (
	LessonDraftCompleted  LessonDraftStatus = "completed"
	LessonDraftFailed     LessonDraftStatus = "failed"
	LessonDraftProcessing LessonDraftStatus = "processing"
)

type LessonDraft struct {
	ID        uuid.UUID
	Title     string
	Model     string
	Status    LessonDraftStatus
	Lesson    *Lesson
	CreatedAt time.Time
	UpdatedAt *time.Time
}

func (d *LessonDraft) ApplyLesson(lesson Lesson) {
	d.Title = lesson.Title
	d.Status = LessonDraftCompleted
	now := time.Now().UTC()
	d.UpdatedAt = &now
	d.Lesson = &lesson
}
func (d *LessonDraft) ApplyError(err error) {
	d.Status = LessonDraftFailed
	now := time.Now().UTC()
	d.UpdatedAt = &now
}
