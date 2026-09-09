package application

import (
	"context"
	"errors"
	"fmt"

	"meet-attendance-clean/domain"
)

type MistakeContext struct {
	CurrentMistake domain.Mistake   `json:"current_mistake"` // Lỗi hiện tại đang xét
	Student        domain.Student   `json:"student"`         // Học sinh
	AncestryChain  []domain.Mistake `json:"ancestry_chain"`  // Phả hệ lỗi từ gốc đến ngọn
}

type MistakeRepository interface {
	GetByID(ctx context.Context, id int) (*domain.Mistake, error)
	SaveMistake(ctx context.Context, mistake domain.Mistake) error
	SaveMistakes(ctx context.Context, studentID int, assignmentID int, mistakes []domain.Mistake) error
	GetMistakeContext(ctx context.Context, mistakeID int) (*MistakeContext, error)
	MarkMistakeResolved(ctx context.Context, mistakeID int) error
}

type GenerateRemediationCommand struct {
	MistakeID           int    `json:"mistake_id"`            // ID của lỗi sai cần khắc phục
	MultipleChoiceCount int    `json:"multiple_choice_count"` // Số lượng câu trắc nghiệm muốn tạo
	EssayCount          int    `json:"essay_count"`           // Số lượng câu tự luận muốn tạo
	Model               string `json:"model"`                 // Tên AI model (ví dụ: gemini-2.5-flash)
	CustomPrompt        string `json:"custom_prompt"`         // Lời dặn dò/prompt tùy chỉnh của giáo viên
}

// Validate tự kiểm tra tính hợp lệ của Command trước khi xử lý nghiệp vụ
func (cmd GenerateRemediationCommand) Validate() error {
	if cmd.MistakeID <= 0 {
		return errors.New("mistake_id không hợp lệ")
	}
	if cmd.MultipleChoiceCount < 0 || cmd.EssayCount < 0 {
		return errors.New("số lượng câu hỏi không được là số âm")
	}
	if cmd.MultipleChoiceCount+cmd.EssayCount == 0 {
		return errors.New("tổng số câu hỏi khắc phục phải lớn hơn 0")
	}
	return nil
}

func (c *AssignmentCommand) GenerateRemediation(ctx context.Context, cmd GenerateRemediationCommand) error {
	// 1. Kiểm tra tính hợp lệ của Command
	if err := cmd.Validate(); err != nil {
		return err
	}

	// 2. Lấy TOÀN BỘ ngữ cảnh phả hệ đệ quy (CurrentMistake + AncestryChain + Student)
	ctxData, err := c.mistakeRepo.GetMistakeContext(ctx, cmd.MistakeID)
	if err != nil {
		return fmt.Errorf("không tìm thấy ngữ cảnh lỗi sai: %w", err)
	}
	if ctxData == nil {
		return fmt.Errorf("context mistake null")
	}
	student := ctxData.Student
	if student.StudentWorkspaceID == nil || *student.StudentWorkspaceID == "" {
		return fmt.Errorf("học sinh %s chưa liên kết sổ OneNote", student.Name)
	}
	if student.TeacherWorkspaceID == nil || *student.TeacherWorkspaceID == "" {
		return fmt.Errorf("học sinh %s chưa liên kết sổ OneNote giáo viên", student.Name)
	}

	// 3. Gọi AI sinh bài tập khắc phục với TOÀN BỘ ngữ cảnh phả hệ
	lesson, err := c.ai.GenerateRemediation(ctx, cmd.Model, cmd.CustomPrompt, *ctxData, cmd.MultipleChoiceCount, cmd.EssayCount)
	if err != nil {
		return fmt.Errorf("AI không thể tạo bài khắc phục: %w", err)
	}

	// 4. Đẩy bài học khắc phục lên OneNote
	title := fmt.Sprintf("Bài tập khắc phục: %s", ctxData.CurrentMistake.Topic)
	chapterName := "Bài tập khắc phục"
	studentTarget := WorkspaceTarget{
		WorkspaceID: student.StudentWorkspaceID,
		ChapterName: &chapterName,
		PageName:    &title,
	}
	teacherTarget := WorkspaceTarget{
		WorkspaceID: student.TeacherWorkspaceID,
		ChapterName: &chapterName,
		PageName:    &title,
	}

	// Trang giáo viên dùng cùng Lesson nhưng renderer AudienceTeacher sẽ hiển thị
	// đáp án và lời giải; trang học sinh chỉ có đề cùng vùng làm bài.
	teacherResult, err := c.onenote.PublishSession(ctx, teacherTarget, domain.AudienceTeacher, *lesson)
	if err != nil {
		return fmt.Errorf("lỗi xuất bản bài khắc phục vào sổ giáo viên: %w", err)
	}
	studentResult, err := c.onenote.PublishSession(ctx, studentTarget, domain.AudienceStudent, *lesson)
	if err != nil {
		return fmt.Errorf("lỗi xuất bản bài khắc phục vào sổ học sinh: %w", err)
	}
	if studentResult.PageID == "" {
		return fmt.Errorf("OneNote không trả về PageID cho bài khắc phục của học sinh")
	}

	// 5. Khởi tạo Aggregate Assignment bằng Domain Factory (Tự động gắn OriginMistakeID và Depth)
	assignment := domain.NewRemediationAssignment(student, ctxData.CurrentMistake, studentResult.PageID, lesson.Exercises)
	assignment.StudentPageWebURL = studentResult.PageWebURL
	assignment.TeacherPageWebURL = teacherResult.PageWebURL

	// 6. Lưu Assignment mới vào DB
	if err := c.assignmentRepo.Save(ctx, assignment); err != nil {
		return fmt.Errorf("lỗi lưu bài tập khắc phục vào DB: %w", err)
	}

	// 7. Cập nhật trạng thái lỗi thành "ĐANG LUYỆN TẬP" (Chưa phải Resolved)
	ctxData.CurrentMistake.StartRemediation()

	// Lưu đúng struct domain.Mistake vào repository
	if err := c.mistakeRepo.SaveMistake(ctx, ctxData.CurrentMistake); err != nil {
		return fmt.Errorf("lỗi lưu trạng thái lỗi sai: %w", err)
	}

	return nil
}
