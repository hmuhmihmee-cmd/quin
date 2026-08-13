package handlers

import (
	"net/http"
	"time"

	"meet-attendance-clean/application"
	webview "meet-attendance-clean/infrastructure/web"
)

type Dashboard struct {
	useCase *application.AttendanceUseCase
	views   *webview.Renderer
	syncJob *SyncJob
}

func NewDashboard(useCase *application.AttendanceUseCase, views *webview.Renderer, syncJob *SyncJob) *Dashboard {
	return &Dashboard{useCase: useCase, views: views, syncJob: syncJob}
}

func (h *Dashboard) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	filter := filterFromRequest(request)
	if filter.Month == "" && filter.From == "" && filter.To == "" {
		filter.Month = time.Now().Format("2006-01")
	}
	summaries, err := h.useCase.StudentSummaries(request.Context(), filter, time.Now())
	if err != nil {
		renderError(writer, err)
		return
	}
	data := webview.ViewData{
		View: "students", Connected: h.useCase.IsGoogleConnected(),
		Notice: request.URL.Query().Get("notice"), Error: request.URL.Query().Get("error"),
		Name: filter.Name, Month: filter.Month, From: filter.From, To: filter.To,
		MinDurationMinutes: filter.MinDurationMinutes,
		Summaries:          summaries,
	}
	data.QueryString = filterQuery(filter)
	data.Syncing, data.SyncMessage = h.syncJob.State()
	for _, summary := range summaries {
		data.TotalMeetings += summary.MeetingCount
		data.TotalMinutes += summary.TotalMinutes
	}
	renderHTML(writer, h.views, data)
}
