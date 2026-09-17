package application2

import (
	"context"
	"fmt"
	"meet-attendance-clean/domain2/assignment"
	"meet-attendance-clean/domain2/lesson"
	"meet-attendance-clean/domain2/student"
	"uuid"
)

// ==========================================================
// 1. PORTS
// ==========================================================

// Thay thế WorkspaceTarget cũ bằng DTO sạch gọn
type PublishPayload struct {
	WorkspaceID   *uuid.UUID
	WorkspaceName *string
	ChapterName   string
	PageName      string
}

type PublishResult struct {
	PageID      uuid.UUID
	WorkspaceID uuid.UUID
}

type LessonPublisherGateway interface {
	Publish(ctx context.Context, payload PublishPayload, audience lesson.Audience, lsn lesson.Lesson) (PublishResult, error)
}

type StudentRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*student.Student, error)
	Save(ctx context.Context, s *student.Student) error
}

type DraftRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*lesson.LessonDraft, error)
}
type PublishLessonCommand struct {
	DraftID            uuid.UUID
	StudentID          uuid.UUID
	PageName           string
	StudentChapterName string
	TeacherChapterName string
}

type PublishLessonUsecase struct {
	studentRepo    StudentRepo
	draftRepo      DraftRepo
	assignmentRepo AssignmentRepo
	publisher      LessonPublisherGateway
}

func (u *PublishLessonUsecase) Publish(ctx context.Context, cmd PublishLessonCommand) error {

	stu, err := u.studentRepo.GetByID(ctx, cmd.StudentID)
	if err != nil {
		return fmt.Errorf("lỗi lấy thông tin học sinh: %w", err)
	}

	draft, err := u.draftRepo.GetByID(ctx, cmd.DraftID)
	if err != nil {
		return fmt.Errorf("lỗi lấy bản nháp bài giảng: %w", err)
	}
	if draft.Lesson() == nil {
		return fmt.Errorf("bản nháp chưa hoàn thành tạo bài giảng")
	}

	// BƯỚC 2: Domain Student TỰ QUYẾT ĐỊNH đích đến (WorkspaceID hay tạo Tên Mới)
	hsTarget := stu.GetPublishTarget(lesson.AudienceStudent, cmd.StudentChapterName)
	gvTarget := stu.GetPublishTarget(lesson.AudienceTeacher, cmd.TeacherChapterName)

	// BƯỚC 3: Gọi Gateway đẩy bài lên Workspace (OneNote)
	// Đẩy cho Học sinh

	hsResult, err := u.publisher.Publish(ctx, PublishPayload{
		WorkspaceID:   hsTarget.WorkspaceID,
		WorkspaceName: hsTarget.WorkspaceName,
		ChapterName:   hsTarget.ChapterName,
		PageName:      cmd.PageName,
	}, lesson.AudienceStudent, *draft.Lesson())
	if err != nil {
		return fmt.Errorf("lỗi đẩy bài cho học sinh: %w", err)
	}

	// Đẩy cho Giáo viên
	gvResult, err := u.publisher.Publish(ctx, PublishPayload{
		WorkspaceID:   gvTarget.WorkspaceID,
		WorkspaceName: gvTarget.WorkspaceName,
		ChapterName:   gvTarget.ChapterName,
		PageName:      cmd.PageName,
	}, lesson.AudienceTeacher, *draft.Lesson())
	if err != nil {
		return fmt.Errorf("lỗi đẩy bài cho giáo viên: %w", err)
	}

	// BƯỚC 4: Domain Student cập nhật lại trạng thái (Ghi nhận WorkspaceID mới nếu có)
	stu.SyncWorkspaceIDs(hsResult.WorkspaceID, gvResult.WorkspaceID)

	// BƯỚC 5: Domain Assignment TỰ KHỞI TẠO từ Lesson
	newAssignment, err := assignment.NewAssignmentFromLesson(stu.ID(), cmd.PageName, *draft.Lesson())
	if err != nil {
		return fmt.Errorf("lỗi tạo assignment từ lesson: %w", err)
	}

	// BƯỚC 6: Lưu toàn bộ trạng thái vào Database
	if err := u.studentRepo.Save(ctx, stu); err != nil {
		return fmt.Errorf("lỗi lưu trạng thái học sinh: %w", err)
	}
	if err := u.assignmentRepo.Save(ctx, newAssignment); err != nil {
		return fmt.Errorf("lỗi lưu assignment: %w", err)
	}

	return nil
}
