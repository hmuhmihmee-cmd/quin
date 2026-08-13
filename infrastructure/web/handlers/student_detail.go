package handlers

import (
	"net/http"
	"strconv"
	"time"

	"meet-attendance-clean/application"
	webview "meet-attendance-clean/infrastructure/web"
)

type StudentDetail struct {
	useCase *application.AttendanceUseCase
	views   *webview.Renderer
}

func NewStudentDetail(useCase *application.AttendanceUseCase, views *webview.Renderer) *StudentDetail {
	return &StudentDetail{useCase: useCase, views: views}
}
func (h *StudentDetail) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	filter := filterFromRequest(request)
	if filter.Month == "" && filter.From == "" && filter.To == "" {
		filter.Month = time.Now().Format("2006-01")
	}
	student, from, to, meetings, err := h.useCase.StudentMeetings(request.Context(), id, filter, time.Now())
	if err != nil {
		renderError(writer, err)
		return
	}
	data := webview.ViewData{
		View: "student", Connected: h.useCase.IsGoogleConnected(), Student: student,
		Meetings: meetings, PeriodStart: from, PeriodEnd: to,
		Name: filter.Name, Month: filter.Month, From: filter.From, To: filter.To,
		MinDurationMinutes: filter.MinDurationMinutes,
		Notice:             request.URL.Query().Get("notice"), Error: request.URL.Query().Get("error"),
	}
	data.QueryString = filterQuery(filter)
	for _, meeting := range meetings {
		data.TotalMinutes += meeting.DurationMinutes
	}
	data.TotalMeetings = int64(len(meetings))
	renderHTML(writer, h.views, data)
}
