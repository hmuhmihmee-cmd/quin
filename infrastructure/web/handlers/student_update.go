package handlers

import (
	"net/http"
	"strconv"

	"meet-attendance-clean/application"
)

type StudentUpdate struct {
	useCase *application.AttendanceUseCase
}

func NewStudentUpdate(useCase *application.AttendanceUseCase) *StudentUpdate {
	return &StudentUpdate{useCase: useCase}
}
func (h *StudentUpdate) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	id, idErr := strconv.ParseInt(request.PathValue("id"), 10, 64)
	cycleDay, dayErr := strconv.Atoi(request.FormValue("cycle_start_day"))
	if idErr != nil || dayErr != nil {
		redirectStudentError(writer, request, id, application.ErrInvalidStudent)
		return
	}
	student, err := h.useCase.GetStudent(request.Context(), id)
	if err == nil {
		student.Name = request.FormValue("name")
		student.CycleStartDay = cycleDay
		err = h.useCase.UpdateStudent(request.Context(), student)
	}
	if err != nil {
		redirectStudentError(writer, request, id, err)
		return
	}
	redirectStudentNotice(writer, request, id, "Đã cập nhật học sinh.")
}
