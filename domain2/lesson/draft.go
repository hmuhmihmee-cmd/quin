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
	id        uuid.UUID
	title     string
	model     string
	prompt    string
	status    LessonDraftStatus
	lesson    *Lesson
	createdAt time.Time
	updatedAt *time.Time
}

func NewLessonDraft(title string, model string, prompt string, status LessonDraftStatus) *LessonDraft {
	return &LessonDraft{
		id:        uuid.New(),
		title:     title,
		model:     model,
		prompt:    prompt,
		status:    status,
		createdAt: time.Now().UTC(),
	}
}
func (d *LessonDraft) ApplyLesson(lesson Lesson) {
	d.title = lesson.title
	d.status = LessonDraftCompleted
	now := time.Now().UTC()
	d.updatedAt = &now
	d.lesson = &lesson
}
func (d *LessonDraft) ApplyError(err error) {
	d.status = LessonDraftFailed
	now := time.Now().UTC()
	d.updatedAt = &now
}

func (d *LessonDraft) ID() uuid.UUID {
	return d.id
}
func (d *LessonDraft) Title() string {
	return d.title
}
func (d *LessonDraft) Model() string {
	return d.model
}
func (d *LessonDraft) Prompt() string {
	return d.prompt
}
func (d *LessonDraft) Status() LessonDraftStatus {
	return d.status
}
func (d *LessonDraft) Lesson() *Lesson {
	return d.lesson
}
func (d *LessonDraft) CreatedAt() time.Time {
	return d.createdAt
}
func (d *LessonDraft) UpdatedAt() *time.Time {
	return d.updatedAt
}
