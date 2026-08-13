package handlers

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"meet-attendance-clean/application"
	webview "meet-attendance-clean/infrastructure/web"
)

var errInvalidOAuthState = errors.New("phiên xác thực không hợp lệ; vui lòng thử lại")

func filterFromRequest(request *http.Request) application.AttendanceFilter {
	minDurationMinutes := int64(30)
	if raw := request.FormValue("min_duration"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed >= 0 {
			minDurationMinutes = parsed
		}
	}
	return application.AttendanceFilter{
		Name: request.FormValue("name"), Month: request.FormValue("month"),
		From: request.FormValue("from"), To: request.FormValue("to"),
		MinDurationMinutes: minDurationMinutes,
	}
}

func filterQuery(filter application.AttendanceFilter) string {
	values := url.Values{}
	if filter.Name != "" {
		values.Set("name", filter.Name)
	}
	if filter.Month != "" {
		values.Set("month", filter.Month)
	}
	if filter.From != "" {
		values.Set("from", filter.From)
	}
	if filter.To != "" {
		values.Set("to", filter.To)
	}
	values.Set("min_duration", strconv.FormatInt(filter.MinDurationMinutes, 10))
	return values.Encode()
}

func redirectNotice(writer http.ResponseWriter, request *http.Request, message string) {
	redirectDashboard(writer, request, "notice", message)
}
func redirectError(writer http.ResponseWriter, request *http.Request, err error) {
	if err == nil {
		err = errors.New("yêu cầu không hợp lệ")
	}
	redirectDashboard(writer, request, "error", err.Error())
}
func redirectDashboard(writer http.ResponseWriter, request *http.Request, key, message string) {
	values := url.Values{key: []string{message}}
	filter := filterFromRequest(request)
	filterValues := url.Values{
		"name": {filter.Name}, "month": {filter.Month}, "from": {filter.From}, "to": {filter.To},
		"min_duration": {strconv.FormatInt(filter.MinDurationMinutes, 10)},
	}
	for name, entries := range filterValues {
		if entries[0] != "" {
			values.Set(name, entries[0])
		}
	}
	http.Redirect(writer, request, "/?"+values.Encode(), http.StatusSeeOther)
}
func redirectStudentNotice(writer http.ResponseWriter, request *http.Request, id int64, message string) {
	redirectStudent(writer, request, id, "notice", message)
}
func redirectStudentError(writer http.ResponseWriter, request *http.Request, id int64, err error) {
	if err == nil {
		err = errors.New("yêu cầu không hợp lệ")
	}
	redirectStudent(writer, request, id, "error", err.Error())
}
func redirectStudent(writer http.ResponseWriter, request *http.Request, id int64, key, message string) {
	values := url.Values{key: []string{message}}
	filter := filterFromRequest(request)
	if filter.Month != "" {
		values.Set("month", filter.Month)
	}
	if filter.From != "" {
		values.Set("from", filter.From)
	}
	if filter.To != "" {
		values.Set("to", filter.To)
	}
	values.Set("min_duration", strconv.FormatInt(filter.MinDurationMinutes, 10))
	http.Redirect(writer, request, "/students/"+strconv.FormatInt(id, 10)+"?"+values.Encode(), http.StatusSeeOther)
}
func redirectLessonError(writer http.ResponseWriter, request *http.Request, err error) {
	http.Redirect(writer, request, "/lessons?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
}
func renderHTML(writer http.ResponseWriter, views *webview.Renderer, data webview.ViewData) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.Render(writer, data); err != nil {
		renderError(writer, err)
	}
}
func renderError(writer http.ResponseWriter, err error) {
	log.Printf("http error: %v", err)
	http.Error(writer, "Có lỗi xảy ra. Vui lòng thử lại.", http.StatusInternalServerError)
}
