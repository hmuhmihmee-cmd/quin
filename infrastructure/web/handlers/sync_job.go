package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"meet-attendance-clean/application"
)

type SyncJob struct {
	useCase *application.AttendanceUseCase
	mu      sync.RWMutex
	running bool
	message string
}

func NewSyncJob(useCase *application.AttendanceUseCase) *SyncJob { return &SyncJob{useCase: useCase} }
func (j *SyncJob) Start() bool {
	j.mu.Lock()
	if j.running {
		j.mu.Unlock()
		return false
	}
	j.running = true
	j.message = "Đang đọc các buổi học và người tham gia từ Google…"
	j.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		count, err := j.useCase.Sync(ctx, time.Now())
		j.mu.Lock()
		defer j.mu.Unlock()
		j.running = false
		if err != nil {
			j.message = "Đồng bộ thất bại: " + err.Error()
			return
		}
		j.message = fmt.Sprintf("Đã đồng bộ %d buổi học từ Google Meet.", count)
	}()
	return true
}
func (j *SyncJob) State() (bool, string) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.running, j.message
}
