package application

import (
	"context"
	"fmt"
	"time"

	"meet-attendance-clean/domain"
)

type SyncRepo interface {
	LastSync(ctx context.Context) (*time.Time, error)
}
type ListMeetRepoForSync interface {
	ListMeetingsFrom(ctx context.Context, time time.Time) ([]domain.Meeting, error)
}
type SaveMeetRepo interface {
	SaveMeets(ctx context.Context, meets []domain.Meeting, students []domain.Student) error
}
type StudentRepo interface {
	Exits(ctx context.Context, class string) (bool, error)
	SaveStudent(ctx context.Context, student domain.Student) error
}

type MeetCommand struct {
	syncRepo     SyncRepo
	listMeetRepo ListMeetRepoForSync
	saveMeetRepo SaveMeetRepo
	studentRepo  StudentRepo
}

func NewMeetCommand(
	syncRepo SyncRepo,
	listMeetRepo ListMeetRepoForSync,
	saveMeetRepo SaveMeetRepo,
	studentRepo StudentRepo,
) *MeetCommand {
	return &MeetCommand{
		syncRepo:     syncRepo,
		listMeetRepo: listMeetRepo,
		saveMeetRepo: saveMeetRepo,
		studentRepo:  studentRepo,
	}
}
func (u *MeetCommand) Sync(ctx context.Context) error {
	lastSync, err := u.syncRepo.LastSync(ctx)
	var syncTime time.Time
	if err != nil {
		syncTime = time.Now().AddDate(0, -3, 0)
	} else {
		syncTime = (*lastSync).AddDate(0, 0, -3)
	}
	meets, err := u.listMeetRepo.ListMeetingsFrom(ctx, syncTime)

	if err != nil {
		return fmt.Errorf("fail to list meeting from %v :%w", syncTime, err)
	}

	finalMeet := []domain.Meeting{}
	finalStudent := []domain.Student{}
	for _, meet := range meets {
		if len(meet.Participants) == 0 {
			continue
		}
		exists, err := u.studentRepo.Exits(ctx, meet.Class)
		if err != nil {
			return fmt.Errorf("fail to check exist student %s", meet.Class)
		}
		if !exists {
			newStudent := domain.Student{
				Name:          "Học sinh mới",
				Class:         meet.Class,
				CycleStartDay: 1,
			}
			finalStudent = append(finalStudent, newStudent)
		}
		finalMeet = append(finalMeet, meet)
	}
	err = u.saveMeetRepo.SaveMeets(ctx, finalMeet, finalStudent)
	if err != nil {
		return fmt.Errorf("fail to save meets and student from %v :%w", syncTime, err)
	}
	return nil
}
func (u *MeetCommand) UpdateStudent(ctx context.Context, student domain.Student) error {
	return u.studentRepo.SaveStudent(ctx, student)
}
