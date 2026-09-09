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
	PageID            string `json:"page_id"`
	WorkspaceID       string `json:"workspace_id"`
	PageWebURL        string `json:"page_web_url"` // Giữ tương thích: URL trang học sinh.
	StudentPageWebURL string `json:"student_page_web_url"`
	TeacherPageWebURL string `json:"teacher_page_web_url"`
}

type WorkspaceGateway interface {
	PublishSession(
		ctx context.Context,
		target WorkspaceTarget,
		audience domain.Audience,
		lesson domain.Lesson,
	) (PublishResult, error)
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

func (c *WorkspaceCommand) PublishLesson(ctx context.Context, cmd PublishLessonCommand) (*PublishResult, error) {
	// 1. VALIDATE CƠ BẢN
	if cmd.DraftID <= 0 {
		return nil, errors.New("bản nháp không hợp lệ")
	}
	if cmd.StudentID <= 0 {
		return nil, errors.New("mã học sinh không hợp lệ")
	}
	cmd.PageName = strings.TrimSpace(cmd.PageName)
	if cmd.PageName == "" {
		return nil, errors.New("tên trang (page_name) không được để trống")
	}

	// Chuẩn hóa & validate Chapter của Học sinh
	cmd.StudentChapter.ChapterID = normalizeOptionalString(cmd.StudentChapter.ChapterID)
	cmd.StudentChapter.ChapterName = normalizeOptionalString(cmd.StudentChapter.ChapterName)
	if cmd.StudentChapter.ChapterID == nil && cmd.StudentChapter.ChapterName == nil {
		return nil, errors.New("cần ít nhất chapter_id hoặc chapter_name cho vở học sinh")
	}

	// Chuẩn hóa & validate Chapter của Giáo viên
	cmd.TeacherChapter.ChapterID = normalizeOptionalString(cmd.TeacherChapter.ChapterID)
	cmd.TeacherChapter.ChapterName = normalizeOptionalString(cmd.TeacherChapter.ChapterName)
	if cmd.TeacherChapter.ChapterID == nil && cmd.TeacherChapter.ChapterName == nil {
		return nil, errors.New("cần ít nhất chapter_id hoặc chapter_name cho vở giáo viên")
	}

	// 2. LẤY ENTITY TỪ DATABASE
	student, err := c.studentRepo.GetStudent(ctx, cmd.StudentID)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy học sinh ID %d: %w", cmd.StudentID, err)
	}

	draft, err := c.draftRepo.GetByID(ctx, cmd.DraftID)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy bài giảng ID %d: %w", cmd.DraftID, err)
	}
	if draft.LessonData == nil {
		return nil, errors.New("bài giảng chưa có dữ liệu cấu trúc hoàn chỉnh")
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
			return nil, errors.New("vở học sinh chưa được tạo nên cần student_chapter.chapter_name")
		}
		// Tên notebook chỉ được dùng khi tạo lần đầu. Sau đó WorkspaceID được lưu
		// và luôn được tái sử dụng, nên đổi tên học sinh sẽ không làm đổi notebook.
		notebookName := fmt.Sprintf("%s_HS", student.Name)
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
			return nil, errors.New("vở giáo viên chưa được tạo nên cần teacher_chapter.chapter_name")
		}
		notebookName := fmt.Sprintf("%s_GV", student.Name)
		teacherTarget.WorkspaceName = &notebookName
		teacherTarget.ChapterID = nil
		isNewTeacherWorkspace = true
	}

	// 5. GỌI GATEWAY ĐẨY BÀI CHO GIÁO VIÊN
	teacherResult, err := c.gateway.PublishSession(ctx, teacherTarget, domain.AudienceTeacher, *draft.LessonData)
	if err != nil {
		fmt.Println(fmt.Errorf("lỗi đẩy bài vào vở giáo viên: %w", err))
		return nil, fmt.Errorf("lỗi đẩy bài vào vở giáo viên: %w", err)
	}

	// 6. GỌI GATEWAY ĐẨY BÀI CHO HỌC SINH
	studentResult, err := c.gateway.PublishSession(ctx, studentTarget, domain.AudienceStudent, *draft.LessonData)
	if err != nil {
		fmt.Println(fmt.Errorf("lỗi đẩy bài vào vở học sinh: %w", err))
		return nil, fmt.Errorf("lỗi đẩy bài vào vở học sinh: %w", err)
	}
	fmt.Printf("Đẩy lên OneNote thành công - pageid = %s / notebookid = %s\n", studentResult.PageID, studentResult.WorkspaceID)
	// 7. ĐỒNG BỘ TỪ TỪ (LAZY SYNC) LƯU NOTEBOOK ID VÀO DB
	if isNewStudentWorkspace || isNewTeacherWorkspace {
		if isNewStudentWorkspace {
			student.StudentWorkspaceID = &studentResult.WorkspaceID
		}
		if isNewTeacherWorkspace {
			student.TeacherWorkspaceID = &teacherResult.WorkspaceID
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
		Title:             cmd.PageName,
		Type:              domain.AssignmentTypeNormal,
		Status:            domain.AssignmentStatusPending,
		AssignedAt:        time.Now().UTC(),
		Assignee:          student,
		TargetPageID:      studentResult.PageID,
		StudentPageWebURL: studentResult.PageWebURL,
		TeacherPageWebURL: teacherResult.PageWebURL,
		Items:             items,
	}

	if err := c.assignmentRepo.Save(ctx, &assignment); err != nil {
		return nil, fmt.Errorf("đã đẩy lên OneNote nhưng lỗi lưu Assignment vào Database: %w", err)
	}

	studentResult.StudentPageWebURL = studentResult.PageWebURL
	studentResult.TeacherPageWebURL = teacherResult.PageWebURL
	return &studentResult, nil
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
