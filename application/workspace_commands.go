package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"meet-attendance-clean/domain"
)

// WorkspaceTarget mô tả đích đến cho từng lần gọi Gateway
type WorkspaceTarget struct {
	WorkspaceID   *string `json:"workspace_id,omitempty"`
	WorkspaceName *string `json:"workspace_name,omitempty"`
	ChapterID     *string `json:"chapter_id,omitempty"`   // Section ID riêng của quyển vở đó
	ChapterName   *string `json:"chapter_name,omitempty"` // Tên Section riêng của quyển vở đó
	PageName      *string `json:"page_name"`
}

// ChapterTarget đại diện cho thông tin chương của từng đối tượng
type ChapterTarget struct {
	ChapterID   *string `json:"chapter_id,omitempty"`   // OneNote Section ID (nếu FE query được)
	ChapterName *string `json:"chapter_name,omitempty"` // Tên chương (nếu chưa có Section ID)
}

type PublishResult struct {
	PageID      string `json:"page_id"`
	WorkspaceID string `json:"workspace_id"`
}

type WorkspaceGateway interface {
	PublishSession(
		ctx context.Context,
		target WorkspaceTarget,
		audience domain.Audience,
		lesson domain.Lesson,
	) (pageID string, workspaceID string, err error)
}

type StudentRepository interface {
	GetStudent(ctx context.Context, studentID int) (domain.Student, error)
	UpdateStudent(ctx context.Context, student domain.Student) error
}

// PublishLessonCommand ĐƯỢC SỬA LẠI: Tách rõ Chapter của Học sinh và Giáo viên
type PublishLessonCommand struct {
	DraftID        int           `json:"draft_id"`
	StudentID      int           `json:"student_id"`
	PageName       string        `json:"page_name"`
	StudentChapter ChapterTarget `json:"student_chapter"` // Thông tin chương bên vở Học sinh
	TeacherChapter ChapterTarget `json:"teacher_chapter"` // Thông tin chương bên vở Giáo viên
}

type WorkspaceCommand struct {
	draftRepo      LessonDraftRepo
	gateway        WorkspaceGateway
	assignmentRepo AssignmentRepository
	studentRepo    StudentRepository
}

func NewWorkspaceCommand(
	draftRepo LessonDraftRepo,
	gateway WorkspaceGateway,
	assignmentRepo AssignmentRepository,
	studentRepo StudentRepository,
) *WorkspaceCommand {
	return &WorkspaceCommand{
		draftRepo:      draftRepo,
		gateway:        gateway,
		assignmentRepo: assignmentRepo,
		studentRepo:    studentRepo,
	}
}

func (c *WorkspaceCommand) PublishLesson(ctx context.Context, cmd PublishLessonCommand) error {
	// 1. VALIDATE CƠ BẢN
	if cmd.DraftID <= 0 {
		return errors.New("bản nháp không hợp lệ")
	}
	if cmd.StudentID <= 0 {
		return errors.New("mã học sinh không hợp lệ")
	}
	cmd.PageName = strings.TrimSpace(cmd.PageName)
	if cmd.PageName == "" {
		return errors.New("tên trang (page_name) không được để trống")
	}

	// Chuẩn hóa & validate Chapter của Học sinh
	cmd.StudentChapter.ChapterID = normalizeOptionalString(cmd.StudentChapter.ChapterID)
	cmd.StudentChapter.ChapterName = normalizeOptionalString(cmd.StudentChapter.ChapterName)
	if cmd.StudentChapter.ChapterID == nil && cmd.StudentChapter.ChapterName == nil {
		return errors.New("cần ít nhất chapter_id hoặc chapter_name cho vở học sinh")
	}

	// Chuẩn hóa & validate Chapter của Giáo viên
	cmd.TeacherChapter.ChapterID = normalizeOptionalString(cmd.TeacherChapter.ChapterID)
	cmd.TeacherChapter.ChapterName = normalizeOptionalString(cmd.TeacherChapter.ChapterName)
	if cmd.TeacherChapter.ChapterID == nil && cmd.TeacherChapter.ChapterName == nil {
		return errors.New("cần ít nhất chapter_id hoặc chapter_name cho vở giáo viên")
	}

	// 2. LẤY ENTITY TỪ DATABASE
	student, err := c.studentRepo.GetStudent(ctx, cmd.StudentID)
	if err != nil {
		return fmt.Errorf("không tìm thấy học sinh ID %d: %w", cmd.StudentID, err)
	}

	draft, err := c.draftRepo.GetByID(ctx, cmd.DraftID)
	if err != nil {
		return fmt.Errorf("không tìm thấy bài giảng ID %d: %w", cmd.DraftID, err)
	}
	if draft.LessonData == nil {
		return errors.New("bài giảng chưa có dữ liệu cấu trúc hoàn chỉnh")
	}

	// 3. THIẾT LẬP TARGET CHO HỌC SINH (_HS)
	studentTarget := WorkspaceTarget{
		ChapterID:   cmd.StudentChapter.ChapterID,
		ChapterName: cmd.StudentChapter.ChapterName,
		PageName:    &cmd.PageName,
	}
	isNewStudentWorkspace := false

	if student.StudentWorkspaceID != nil && *student.StudentWorkspaceID != "" {
		studentTarget.WorkspaceID = student.StudentWorkspaceID
	} else {
		if cmd.StudentChapter.ChapterName == nil {
			return errors.New("vở học sinh chưa được tạo nên cần student_chapter.chapter_name")
		}
		notebookName := fmt.Sprintf("%s_%s_HS", student.Name, student.Class)
		studentTarget.WorkspaceName = &notebookName
		studentTarget.ChapterID = nil
		isNewStudentWorkspace = true
	}

	// 4. THIẾT LẬP TARGET CHO GIÁO VIÊN (_GV)
	teacherTarget := WorkspaceTarget{
		ChapterID:   cmd.TeacherChapter.ChapterID,
		ChapterName: cmd.TeacherChapter.ChapterName,
		PageName:    &cmd.PageName,
	}
	isNewTeacherWorkspace := false

	if student.TeacherWorkspaceID != nil && *student.TeacherWorkspaceID != "" {
		teacherTarget.WorkspaceID = student.TeacherWorkspaceID
	} else {
		if cmd.TeacherChapter.ChapterName == nil {
			return errors.New("vở giáo viên chưa được tạo nên cần teacher_chapter.chapter_name")
		}
		notebookName := fmt.Sprintf("%s_%s_GV", student.Name, student.Class)
		teacherTarget.WorkspaceName = &notebookName
		teacherTarget.ChapterID = nil
		isNewTeacherWorkspace = true
	}

	// 5. GỌI GATEWAY ĐẨY BÀI CHO GIÁO VIÊN
	_, teacherWorkspaceID, err := c.gateway.PublishSession(ctx, teacherTarget, domain.AudienceTeacher, *draft.LessonData)
	if err != nil {
		return fmt.Errorf("lỗi đẩy bài vào vở giáo viên: %w", err)
	}

	// 6. GỌI GATEWAY ĐẨY BÀI CHO HỌC SINH
	studentPageID, studentWorkspaceID, err := c.gateway.PublishSession(ctx, studentTarget, domain.AudienceStudent, *draft.LessonData)
	if err != nil {
		return fmt.Errorf("lỗi đẩy bài vào vở học sinh: %w", err)
	}

	// 7. ĐỒNG BỘ TỪ TỪ (LAZY SYNC) LƯU NOTEBOOK ID VÀO DB
	if isNewStudentWorkspace || isNewTeacherWorkspace {
		if isNewStudentWorkspace {
			student.StudentWorkspaceID = &studentWorkspaceID
		}
		if isNewTeacherWorkspace {
			student.TeacherWorkspaceID = &teacherWorkspaceID
		}

		if err := c.studentRepo.UpdateStudent(ctx, student); err != nil {
			fmt.Printf("Cảnh báo: Lỗi lưu đồng bộ Notebook ID cho học sinh %d: %v\n", student.ID, err)
		}
	}

	// 8. TẠO VÀ LƯU ASSIGNMENT VÀO DATABASE
	items := make([]domain.AssignedExercise, 0, len(draft.LessonData.Exercises))
	for _, exercise := range draft.LessonData.Exercises {
		items = append(items, domain.AssignedExercise{Exercise: exercise})
	}

	assignment := domain.Assignment{
		Title:        cmd.PageName,
		Type:         domain.AssignmentTypeNormal,
		Status:       domain.AssignmentStatusPending,
		AssignedAt:   time.Now().UTC(),
		Assignee:     student,
		TargetPageID: studentPageID,
		Items:        items,
	}

	if err := c.assignmentRepo.Save(ctx, &assignment); err != nil {
		return fmt.Errorf("đã đẩy lên OneNote nhưng lỗi lưu Assignment vào Database: %w", err)
	}

	return nil
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
