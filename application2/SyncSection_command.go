package application2

import (
	"context"
	"fmt"
	"meet-attendance-clean/domain2/student"
	"time"
)

// ==========================================================
// 1. PORTS (GIAO DIỆN TẦNG APPLICATION)
// ==========================================================

type SyncConfigRepo interface {
	GetLastSyncTime(ctx context.Context) (*time.Time, error)
	UpdateLastSyncTime(ctx context.Context, t time.Time) error
}

// Kết quả mà Gateway trả về: Đã được Hạ tầng "rửa sạch" thành Domain Model
type MeetingSyncResult struct {
	Sessions    []student.ClassSession
	NewStudents []student.Student // Học sinh mới tự sinh nếu phát hiện phòng học mới
}

// MeetingProviderGateway: Tên khái quát, không dính chữ Google Meet
type MeetingProviderGateway interface {
	SyncMeetings(ctx context.Context, fromTime time.Time) (*MeetingSyncResult, error)
}

type ClassSessionRepo interface {
	SaveBatch(ctx context.Context, sessions []student.ClassSession) error
}

type StudentBatchRepo interface {
	SaveBatch(ctx context.Context, students []student.Student) error
}

// ==========================================================
// 2. USECASE (CHỈ ĐIỀU PHỐI - KHÔNG BIẾT SPACE LÀ GÌ)
// ==========================================================

type SyncMeetingUsecase struct {
	syncRepo    SyncConfigRepo
	meetGateway MeetingProviderGateway
	sessionRepo ClassSessionRepo
	studentRepo StudentBatchRepo
}

func NewSyncMeetingUsecase(
	syncRepo SyncConfigRepo,
	meetGateway MeetingProviderGateway,
	sessionRepo ClassSessionRepo,
	studentRepo StudentBatchRepo,
) *SyncMeetingUsecase {
	return &SyncMeetingUsecase{
		syncRepo:    syncRepo,
		meetGateway: meetGateway,
		sessionRepo: sessionRepo,
		studentRepo: studentRepo,
	}
}

func (u *SyncMeetingUsecase) Sync(ctx context.Context) error {
	// BƯỚC 1: Tính toán thời điểm cần đồng bộ
	syncFrom := u.determineSyncStartTime(ctx)

	// BƯỚC 2: Gateway tự kéo Google Meet, tự lọc rác, tự map với bảng external_identity_mappings
	// và trả về kết quả đã được gán StudentID (UUID) chuẩn mực!
	result, err := u.meetGateway.SyncMeetings(ctx, syncFrom)
	if err != nil {
		return fmt.Errorf("lỗi đồng bộ phiên học từ nền tảng ngoài: %w", err)
	}

	// BƯỚC 3: Lưu học sinh mới vào DB (nếu phát hiện có phòng học mới)
	if len(result.NewStudents) > 0 {
		if err := u.studentRepo.SaveBatch(ctx, result.NewStudents); err != nil {
			return fmt.Errorf("lỗi lưu học sinh mới: %w", err)
		}
	}

	// BƯỚC 4: Lưu các phiên học mới vào DB
	if len(result.Sessions) > 0 {
		if err := u.sessionRepo.SaveBatch(ctx, result.Sessions); err != nil {
			return fmt.Errorf("lỗi lưu phiên học: %w", err)
		}
	}

	// BƯỚC 5: Cập nhật thời điểm đồng bộ thành công
	return u.syncRepo.UpdateLastSyncTime(ctx, time.Now().UTC())
}

func (u *SyncMeetingUsecase) determineSyncStartTime(ctx context.Context) time.Time {
	lastSync, err := u.syncRepo.GetLastSyncTime(ctx)
	if err != nil || lastSync == nil {
		return time.Now().UTC().AddDate(0, -3, 0) // Mặc định 3 tháng trước
	}
	return lastSync.AddDate(0, 0, -3) // Lùi 3 ngày phòng trễ log
}
