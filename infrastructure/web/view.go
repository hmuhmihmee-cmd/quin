package web

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"strconv"
	"time"

	"meet-attendance-clean/application"
	"meet-attendance-clean/domain"
)

//go:embed templates/*.html
var templateFiles embed.FS

type ViewData struct {
	View               string
	Connected          bool
	Notice             string
	Error              string
	Name               string
	Month              string
	From               string
	To                 string
	MinDurationMinutes int64
	QueryString        string
	Summaries          []application.StudentSummary
	Student            domain.Student
	LessonDraft        domain.LessonDraft
	Meetings           []application.MeetingReport
	PeriodStart        time.Time
	PeriodEnd          time.Time
	TotalMeetings      int64
	TotalMinutes       int64
	Syncing            bool
	SyncMessage        string
}

type Renderer struct{ templates *template.Template }

func NewRenderer() (*Renderer, error) {
	functions := template.FuncMap{
		"dateTime": func(value time.Time) string { return value.Local().Format("02/01/2006 · 15:04") },
		"date":     func(value time.Time) string { return value.Local().Format("02/01/2006") },
		"duration": formatDuration,
		"periodEnd": func(value time.Time) time.Time {
			if value.Hour() == 0 && value.Minute() == 0 && value.Second() == 0 {
				return value.AddDate(0, 0, -1)
			}
			return value
		},
		"query": func(name, month, from, to string) string {
			values := url.Values{}
			if name != "" {
				values.Set("name", name)
			}
			if month != "" {
				values.Set("month", month)
			}
			if from != "" {
				values.Set("from", from)
			}
			if to != "" {
				values.Set("to", to)
			}
			return values.Encode()
		},
	}
	parsed, err := template.New("root").Funcs(functions).ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: parsed}, nil
}

func (r *Renderer) Render(writer io.Writer, data ViewData) error {
	return r.templates.ExecuteTemplate(writer, "layout", data)
}

func formatDuration(minutes int64) string {
	if minutes < 60 {
		return strconv.FormatInt(minutes, 10) + " phút"
	}
	hours := minutes / 60
	remaining := minutes % 60
	if remaining == 0 {
		return fmt.Sprintf("%d giờ", hours)
	}
	return fmt.Sprintf("%d giờ %d phút", hours, remaining)
}
