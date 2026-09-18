package application2

import (
	"context"
	"errors"
	"fmt"
	"meet-attendance-clean/domain2/assignment"
	"meet-attendance-clean/domain2/lesson"
	"meet-attendance-clean/domain2/student"
	"uuid"
)

// ==========================================================
// 1. PORTS (INTERFACES CỦA TẦNG APPLICATION)
// ==========================================================

// WorkspacePublisherGateway: Cổng giao tiếp duy nhất ra bên ngoài (OneNote / Google Docs)
// Chú ý: Chỉ nhận các khái niệm thuần túy: StudentID (UUID), Lesson (Content), ChapterName, PageName.
type WorkspacePublisherGateway interface {
	PublishLesson(
		ctx context.Context,
		studentID uuid.UUID,
		studentName string, // Cần để OneNote tự tạo sổ nếu chưa có
		lsn lesson.Lesson,
		studentChapterName string,
		teacherChapterName string,
		pageName string,
	) error
}

type StudentRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*student.Student, error)
}

type DraftRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*lesson.LessonDraft, error)
}

// ==========================================================
// 2. COMMAND (DTO ĐẦU VÀO)
// ==========================================================

type PublishLessonCommand struct {
	DraftID            uuid.UUID
	StudentID          uuid.UUID
	PageName           string
	StudentChapterName string
	TeacherChapterName string
}

// ==========================================================
// 3. USECASE (ĐIỀU PHỐI TUYẾN TÍNH - KHÔNG RÒ RỈ HẠ TẦNG)
// ==========================================================

type PublishLessonUsecase struct {
	studentRepo    StudentRepo
	draftRepo      DraftRepo
	assignmentRepo AssignmentRepo
	publisher      WorkspacePublisherGateway
}

func NewPublishLessonUsecase(
	studentRepo StudentRepo,
	draftRepo DraftRepo,
	assignmentRepo AssignmentRepo,
	publisher WorkspacePublisherGateway,
) *PublishLessonUsecase {
	return &PublishLessonUsecase{
		studentRepo:    studentRepo,
		draftRepo:      draftRepo,
		assignmentRepo: assignmentRepo,
		publisher:      publisher,
	}
}

func (u *PublishLessonUsecase) Publish(ctx context.Context, cmd PublishLessonCommand) error {
	// BƯỚC 1: Lấy các Entity nội bộ từ DB (Chỉ dùng UUID)
	stu, err := u.studentRepo.GetByID(ctx, cmd.StudentID)
	if err != nil {
		return fmt.Errorf("không tìm thấy học sinh: %w", err)
	}

	draft, err := u.draftRepo.GetByID(ctx, cmd.DraftID)
	if err != nil {
		return fmt.Errorf("không tìm thấy bản nháp bài giảng: %w", err)
	}
	if draft.Lesson() == nil {
		return errors.New("bản nháp chưa hoàn thành việc sinh bài giảng")
	}

	// BƯỚC 2: Ra lệnh cho Gateway xuất bản lên nền tảng ngoài
	// Gateway (Hạ tầng) sẽ tự:
	// - Tra bảng mapping xem học sinh có sổ OneNote chưa (nếu chưa tự tạo Tên_HS, Tên_GV).
	// - Tạo trang OneNote cho cả thầy và trò.
	// - TỰ LƯU MỌI METADATA (URL, PageID) VÀO BẢNG external_identity_mappings.
	// 👉 UseCase không cần nhận lại URL hay ID rác nào của OneNote!
	err = u.publisher.PublishLesson(
		ctx,
		stu.ID(),
		stu.Name(),
		*draft.Lesson(),
		cmd.StudentChapterName,
		cmd.TeacherChapterName,
		cmd.PageName,
	)
	if err != nil {
		return fmt.Errorf("lỗi từ nền tảng xuất bản bên ngoài: %w", err)
	}

	// BƯỚC 3: Tạo Aggregate Assignment nội bộ (Chỉ quản lý UUID và câu hỏi)
	newAssignment, err := assignment.NewAssignmentFromLesson(stu.ID(), cmd.PageName, *draft.Lesson())
	if err != nil {
		return fmt.Errorf("lỗi khởi tạo assignment: %w", err)
	}

	// BƯỚC 4: Lưu Assignment mới vào Database
	if err := u.assignmentRepo.Save(ctx, newAssignment); err != nil {
		return fmt.Errorf("lỗi lưu assignment vào cơ sở dữ liệu: %w", err)
	}

	// Xong! Không cần u.studentRepo.Save() vì Student không bị ô nhiễm ID OneNote nữa.
	return nil
}
