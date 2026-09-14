// File: app.go
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	goRuntime "runtime"
	"strings"
	"time"

	"meet-attendance-clean/application"
	"meet-attendance-clean/domain"
	"meet-attendance-clean/infrastructure/googlemeet"
	"meet-attendance-clean/infrastructure/onenote"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx             context.Context
	meetCmd         *application.MeetCommand
	meetQuery       *application.MeetQuery
	lessonCmd       *application.LessonCommand
	lessonQuery     *application.LessonQuery
	workspaceCmd    *application.WorkspaceCommand
	workspaceQuery  *application.WorkspaceQuery
	assignmentCmd   *application.AssignmentCommand
	assignmentQuery *application.AssignmentQuery
	oneNoteClient   *onenote.Client
	meetClient      *googlemeet.Client
}

func NewApp(
	meetCmd *application.MeetCommand,
	meetQuery *application.MeetQuery,
	lesCmd *application.LessonCommand,
	lesQuery *application.LessonQuery,
	ai application.LessonAI,
	workspaceCmd *application.WorkspaceCommand,
	workspaceQuery *application.WorkspaceQuery,
	assignmentCmd *application.AssignmentCommand,
	assignmentQuery *application.AssignmentQuery,
	oneNoteClient *onenote.Client,
	meetClient *googlemeet.Client,
) *App {
	return &App{
		meetCmd:         meetCmd,
		meetQuery:       meetQuery,
		lessonCmd:       lesCmd,
		lessonQuery:     lesQuery,
		workspaceCmd:    workspaceCmd,
		workspaceQuery:  workspaceQuery,
		assignmentCmd:   assignmentCmd,
		assignmentQuery: assignmentQuery,
		oneNoteClient:   oneNoteClient,
		meetClient:      meetClient,
	}
}

// startup được gọi khi cửa sổ ứng dụng Desktop bật lên
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ==================== MODULE 1: ĐIỂM DANH & LỚP HỌC ====================

// Lấy dữ liệu Dashboard (Sổ buổi học)
func (a *App) GetDashboard(filter application.StudentFilter) (*application.DashboardView, error) {
	return a.meetQuery.GetDashboard(a.ctx, filter)
}

// Lấy chi tiết buổi học của 1 học sinh
func (a *App) GetStudentDetail(studentID int, filter application.MeetingFilter) (*application.StudentDetailView, error) {
	return a.meetQuery.GetStudentDetail(a.ctx, studentID, filter)
}

// Đồng bộ Google Meet
func (a *App) SyncGoogleMeet() error {
	return a.meetCmd.Sync(a.ctx)
}

// Lưu chỉnh sửa học sinh
func (a *App) UpdateStudent(student domain.Student) error {
	return a.meetCmd.UpdateStudent(a.ctx, student)
}

// ==================== MODULE 2: TẠO BÀI GIẢNG AI ====================

// Lấy danh sách model Gemini (để đổ vào dropdown)
func (a *App) ListAIModels() ([]domain.AIModelInfo, error) {
	return a.lessonQuery.ListModels(a.ctx)
}

// Bấm tạo bài giảng từ video
func (a *App) CreateLessonJob(model, sourceURL, customPrompt string) (int, error) {
	return a.lessonCmd.CreateDraftJob(a.ctx, model, sourceURL, customPrompt)
}

// Lấy danh sách lịch sử các bài giảng đã tạo
func (a *App) ListLessonDrafts() ([]domain.LessonDraft, error) {
	return a.lessonQuery.ListDrafts(a.ctx)
}

// Lấy chi tiết 1 bản thảo để sửa
func (a *App) GetLessonDraft(id int) (*domain.LessonDraft, error) {
	return a.lessonQuery.GetDraft(a.ctx, id)
}

// Lưu nội dung giáo viên vừa sửa trên Textarea/ToastUI
func (a *App) SaveLessonDraft(draft domain.LessonDraft) error {
	return a.lessonCmd.SaveDraftContent(a.ctx, draft)
}

// // Xuất PDF: Mở hộp thoại chọn nơi lưu file của Windows/Mac
// func (a *App) ExportAndSavePDF(draftID int) error {
// 	teacherDoc, studentDoc, err := a.lessonCmd.ExportDocuments(a.ctx, draftID)
// 	if err != nil {
// 		return err
// 	}

// 	// Mở hộp thoại chọn thư mục lưu file native của hệ điều hành
// 	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
// 		Title: "Chọn thư mục để lưu 2 file PDF",
// 	})
// 	if err != nil || dir == "" {
// 		return nil // Người dùng bấm Hủy
// 	}

// 	// Ghi 2 file PDF vào thư mục đã chọn
// 	teacherPath := fmt.Sprintf("%s/GiaoVien_%s.pdf", dir, teacherDoc.Title)
// 	studentPath := fmt.Sprintf("%s/HocSinh_%s.pdf", dir, studentDoc.Title)

// 	if err := os.WriteFile(teacherPath, teacherDoc.Data, 0644); err != nil {
// 		return fmt.Errorf("lỗi lưu file giáo viên: %w", err)
// 	}
// 	if err := os.WriteFile(studentPath, studentDoc.Data, 0644); err != nil {
// 		return fmt.Errorf("lỗi lưu file học sinh: %w", err)
// 	}

// 	return nil
// }

// HẠ TẦNG MỚI: bridge để frontend gọi use case chấm Assignment.
func (a *App) GradeAssignment(cmd application.GradeAssignmentCommand) error {
	return a.assignmentCmd.Grade(a.ctx, cmd)
}

func (a *App) PushAssignmentFeedback(pageID string) error {
	return a.assignmentCmd.PushFeedback(a.ctx, pageID)
}

// HẠ TẦNG MỚI: bridge dữ liệu gọn cho UI Chấm bài và Kho lỗi sai.
func (a *App) ListGradingPages(filter application.GradingPageFilter) ([]application.AssignmentSummary, error) {
	return a.assignmentQuery.ListGradingPages(a.ctx, filter)
}

func (a *App) GetAssignmentForGrading(pageID string) (*domain.Assignment, error) {
	return a.assignmentQuery.GetForGrading(a.ctx, pageID)
}

func (a *App) ListStudentMistakes() ([]application.StudentMistakeGroup, error) {
	return a.assignmentQuery.ListMistakes(a.ctx)
}

func (a *App) ListStudentChoices() ([]application.StudentChoice, error) {
	return a.assignmentQuery.ListStudents(a.ctx)
}

func (a *App) GenerateRemediation(cmd application.GenerateRemediationCommand) error {
	return a.assignmentCmd.GenerateRemediation(a.ctx, cmd)
}
func (a *App) IsOneNoteConnected() bool {
	return a.oneNoteClient.IsConnected()
}

func (a *App) ConnectOneNote() error {
	authURL := a.oneNoteClient.AuthorizationURL("onenote-auth")

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":9000",
		Handler: mux,
	}

	errChan := make(chan error, 1)

	// 1. Nếu mở thẳng localhost:9000 -> Tự động chuyển hướng sang trang đăng nhập Microsoft
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	})

	// 2. Nhận mã xác thực từ Microsoft trả về
	mux.HandleFunc("/oauth/microsoft/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Không tìm thấy mã xác thực từ Microsoft", http.StatusBadRequest)
			return
		}

		err := a.oneNoteClient.Exchange(context.Background(), code)
		if err != nil {
			http.Error(w, "Đổi token thất bại: "+err.Error(), http.StatusInternalServerError)
			errChan <- err
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `
			<html>
			<head><meta charset="utf-8"><title>Thành công</title></head>
			<body style="font-family: system-ui, sans-serif; text-align: center; padding: 60px 20px; background: #f8faf9;">
				<div style="max-width: 480px; margin: auto; background: #fff; padding: 30px; border-radius: 12px; border: 1px solid #e2e8f0; box-shadow: 0 4px 12px rgba(0,0,0,0.05);">
					<h2 style="color: #15803d; margin-top: 0;">Đăng nhập OneNote thành công!</h2>
					<p style="color: #64748b;">Hệ thống đã nhận được quyền truy cập. Bạn có thể đóng tab này và quay lại ứng dụng Desktop.</p>
				</div>
			</body>
			</html>
		`)
		errChan <- nil

		go func() {
			time.Sleep(1 * time.Second)
			_ = server.Shutdown(context.Background())
		}()
	})

	// 3. Khởi động server lắng nghe
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	// 4. Mở trình duyệt Windows
	openCrossPlatformBrowser(a.ctx, authURL)

	// 5. Đợi kết quả xác thực
	select {
	case err := <-errChan:
		return err
	case <-time.After(3 * time.Minute):
		_ = server.Shutdown(context.Background())
		return errors.New("quá thời gian đăng nhập (Timeout)")
	}
}

// Hàm mở trình duyệt Windows gọi trực tiếp từ đường dẫn gốc trong WSL
func openCrossPlatformBrowser(ctx context.Context, targetURL string) {
	// Danh sách các đường dẫn thực thi của Windows từ bên trong WSL
	executors := []string{
		"/mnt/c/Windows/System32/cmd.exe",
		"/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe",
		"wslview",
	}

	for _, exe := range executors {
		if strings.Contains(exe, "powershell") {
			cmd := exec.Command(exe, "-NoProfile", "-Command", fmt.Sprintf("Start-Process '%s'", targetURL))
			if err := cmd.Start(); err == nil {
				return
			}
		} else if strings.Contains(exe, "cmd.exe") {
			cmd := exec.Command(exe, "/c", "start", strings.ReplaceAll(targetURL, "&", "^&"))
			if err := cmd.Start(); err == nil {
				return
			}
		} else {
			cmd := exec.Command(exe, targetURL)
			if err := cmd.Start(); err == nil {
				return
			}
		}
	}

	// Mặc định fallback về Wails runtime
	runtime.BrowserOpenURL(ctx, targetURL)
}

// GỌI QUA WORKSPACE QUERY:
func (a *App) ListWorkspaces() ([]domain.Workspace, error) {
	return a.workspaceQuery.ListWorkspaces(a.ctx)
}

// GỌI QUA WORKSPACE COMMAND:
func (a *App) PublishLessonToOneNote(cmd application.PublishLessonCommand) (*application.PublishResult, error) {
	return a.workspaceCmd.PublishLesson(a.ctx, cmd)
}

// OpenExternalURL mở đúng trang OneNote bằng trình duyệt mặc định của hệ điều hành.
// Chỉ nhận HTTP(S) để UI không thể khởi chạy lệnh hoặc giao thức tùy ý.
func (a *App) OpenExternalURL(targetURL string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(targetURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return errors.New("đường dẫn OneNote không hợp lệ")
	}

	// OneNote uses ! inside resource IDs. Wails' Windows URL sanitizer rejects a
	// literal ! even though it is valid in a query value, so percent-encode it
	// before handing the URL to the system browser. OneDrive decodes %21 back to !.
	safeURL := strings.ReplaceAll(parsed.String(), "!", "%21")
	if goRuntime.GOOS == "windows" {
		runtime.BrowserOpenURL(a.ctx, safeURL)
		return nil
	}

	openCrossPlatformBrowser(a.ctx, safeURL)
	return nil
}

// / 1. Kiểm tra xem đã đăng nhập tài khoản Google chưa
func (a *App) IsGoogleConnected() bool {
	return a.meetClient.IsConnected()
}

// 2. Mở trình duyệt để đăng nhập Google Meet (dùng chung cơ chế với OneNote)
func (a *App) ConnectGoogleMeet() error {
	authURL, err := a.meetClient.AuthorizationURL("google-auth")
	if err != nil {
		return fmt.Errorf("tạo link đăng nhập Google: %w", err)
	}

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":9000",
		Handler: mux,
	}

	errChan := make(chan error, 1)

	// Hàm xử lý khi nhận mã xác thực
	handleCallback := func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Không tìm thấy mã xác thực từ Google", http.StatusBadRequest)
			return
		}

		err := a.meetClient.Exchange(context.Background(), code)
		if err != nil {
			http.Error(w, "Đổi token thất bại: "+err.Error(), http.StatusInternalServerError)
			errChan <- err
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `
			<html>
			<head><meta charset="utf-8"><title>Thành công</title></head>
			<body style="font-family: system-ui, sans-serif; text-align: center; padding: 60px 20px; background: #f8faf9;">
				<div style="max-width: 480px; margin: auto; background: #fff; padding: 30px; border-radius: 12px; border: 1px solid #e2e8f0; box-shadow: 0 4px 12px rgba(0,0,0,0.05);">
					<h2 style="color: #15803d; margin-top: 0;">Đăng nhập Google Meet thành công!</h2>
					<p style="color: #64748b;">Hệ thống đã kết nối tài khoản Google. Bạn có thể đóng tab này và quay lại ứng dụng Desktop.</p>
				</div>
			</body>
			</html>
		`)
		errChan <- nil

		go func() {
			time.Sleep(1 * time.Second)
			_ = server.Shutdown(context.Background())
		}()
	}

	// ĐĂNG KÝ CẢ CÁC ĐƯỜNG DẪN ĐỂ KHÔNG BAO GIỜ BỊ 404:
	mux.HandleFunc("/oauth2callback", handleCallback) // Bắt đúng đường dẫn Google đang gửi về!
	mux.HandleFunc("/oauth/callback", handleCallback)
	mux.HandleFunc("/oauth/google/callback", handleCallback)

	// Chuyển hướng nếu mở thẳng localhost:9000
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	openCrossPlatformBrowser(a.ctx, authURL)

	select {
	case err := <-errChan:
		return err
	case <-time.After(3 * time.Minute):
		_ = server.Shutdown(context.Background())
		return errors.New("quá thời gian đăng nhập Google (Timeout)")
	}
}

// 3. Đăng xuất / Ngắt kết nối Google Meet (Xóa file token.json)
func (a *App) DisconnectGoogleMeet() error {
	return a.meetClient.Disconnect()
}
