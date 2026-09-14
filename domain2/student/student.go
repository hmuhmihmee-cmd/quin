package student

import (
	"errors"
	"strings"
	"time"
	"uuid"
)

type Student struct {
	id                 uuid.UUID
	name               string
	class              string
	studentWorkspaceID uuid.UUID
	teacherWorkspaceID uuid.UUID
	cycleStartDay      int
	createdAt          time.Time
}

func NewStudent(name, class string, cycleStartDay int) (*Student, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("tên học sinh không được để trống")
	}

	class = strings.TrimSpace(class)
	if class == "" {
		return nil, errors.New("lớp học không được để trống")
	}

	if cycleStartDay < 1 || cycleStartDay > 31 {
		return nil, errors.New("ngày chu kỳ học phí phải từ ngày 1 đến ngày 31")
	}

	return &Student{
		id:            uuid.New(),
		name:          name,
		class:         class,
		cycleStartDay: cycleStartDay,
		createdAt:     time.Now().UTC(),
	}, nil
}
