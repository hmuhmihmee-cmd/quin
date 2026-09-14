package workspace

import (
	"errors"
	"uuid"
)

type Audience string

const (
	AudienceTeacher Audience = "teacher"
	AudienceStudent Audience = "student"
)

type Workspace struct {
	id        uuid.UUID
	name      string
	studentID uuid.UUID
	audience  Audience
	chapters  []Chapter
}
type Chapter struct {
	id    uuid.UUID
	name  string
	pages []Page
}
type Page struct {
	id   uuid.UUID
	name string
}

func NewWorkspace(name string, studentID uuid.UUID, audience Audience) *Workspace {
	return &Workspace{
		id:        uuid.New(),
		name:      name,
		studentID: studentID,
		audience:  audience,
	}
}
func (w *Workspace) AddPage(chapterName string, pageName string) (chapterID uuid.UUID, pageID uuid.UUID, err error) {
	if chapterName == "" {
		return uuid.Nil(), uuid.Nil(), errors.New("tên chương không được rỗng")
	}
	if pageName == "" {
		return uuid.Nil(), uuid.Nil(), errors.New("tên page không được rỗng")
	}
	index := -1
	for i := 0; i < len(w.chapters); i++ {
		if w.chapters[i].name == chapterName {
			for j := 0; j < len(w.chapters[i].pages); j++ {
				if w.chapters[i].pages[j].name == pageName {
					return uuid.Nil(), uuid.Nil(), errors.New("tên page đã tồn tại")
				}
			}
			index = i
			chapterID = w.chapters[index].id
			break
		}
	}
	if index == -1 {
		chapterID = uuid.New()
		w.chapters = append(w.chapters, Chapter{id: chapterID, name: chapterName})
		index = len(w.chapters) - 1
	}
	pageID = uuid.New()
	w.chapters[index].pages = append(w.chapters[index].pages, Page{id: pageID, name: pageName})
	return chapterID, pageID, nil

}
func (w *Workspace) ID() uuid.UUID        { return w.id }
func (w *Workspace) Name() string         { return w.name }
func (w *Workspace) StudentID() uuid.UUID { return w.studentID }
func (w *Workspace) Audience() Audience   { return w.audience }
func (w *Workspace) Chapters() []Chapter  { return w.chapters }
func (c *Chapter) ID() uuid.UUID          { return c.id }
func (c *Chapter) Name() string           { return c.name }
func (c *Chapter) Pages() []Page          { return c.pages }
func (p *Page) ID() uuid.UUID             { return p.id }
func (p *Page) Name() string              { return p.name }
