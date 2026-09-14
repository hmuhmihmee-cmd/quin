package main

import (
	"embed"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"

	"meet-attendance-clean/application"
	"meet-attendance-clean/config"
	"meet-attendance-clean/infrastructure/database"
	"meet-attendance-clean/infrastructure/gemini"
	"meet-attendance-clean/infrastructure/googlemeet"
	"meet-attendance-clean/infrastructure/onenote"
	"meet-attendance-clean/infrastructure/pdf"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed frontend
var assets embed.FS

//go:embed config/google-oauth.json
var embeddedGoogleCredentials []byte

//go:embed config/gemini-key.txt
var embeddedGeminiKey []byte

//go:embed config/microsoft-oauth.json
var embeddedMicrosoftCredentials []byte

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		log.Printf("Cảnh báo config: %v", err)
	}

	dataDir, err := appDataDirectory()
	if err != nil {
		log.Fatalf("Lỗi tạo thư mục dữ liệu QUIN: %v", err)
	}
	if err := migrateLegacyUserFile(dataDir, "attendance.db"); err != nil {
		log.Printf("Cảnh báo chuyển dữ liệu cũ: %v", err)
	}
	if err := migrateLegacyUserFile(dataDir, "token.json"); err != nil {
		log.Printf("Cảnh báo chuyển đăng nhập Google cũ: %v", err)
	}
	if err := migrateLegacyUserFile(dataDir, "token_microsoft.json"); err != nil {
		log.Printf("Cảnh báo chuyển đăng nhập Microsoft cũ: %v", err)
	}

	db, err := database.Open(filepath.Join(dataDir, "attendance.db"))
	if err != nil {
		log.Fatalf("Lỗi mở SQLite: %v", err)
	}
	defer db.Close()

	meetClient, _ := googlemeet.New(embeddedGoogleCredentials, filepath.Join(dataDir, "token.json"), cfg.GoogleRedirectURL)
	geminiClient := gemini.New(string(embeddedGeminiKey), cfg.GeminiModel)
	formatter := pdf.NewFormatter()
	// renderer := pdf.NewRenderer()
	// Đọc cấu hình Microsoft OAuth
	var msCfg struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RedirectURL  string `json:"redirect_url"`
	}
	_ = json.Unmarshal(embeddedMicrosoftCredentials, &msCfg)
	if msCfg.RedirectURL == "" {
		msCfg.RedirectURL = "http://localhost:9000/oauth/microsoft/callback"
	}
	oneNoteClient := onenote.New(
		msCfg.ClientID,
		msCfg.ClientSecret,
		msCfg.RedirectURL,
		filepath.Join(dataDir, "token_microsoft.json"),
	)
	// 2. Khởi tạo Application UseCases
	meetCmd := application.NewMeetCommand(db, meetClient, db, db)
	meetQuery := application.NewMeetQuery(db)
	lessonCmd := application.NewLessonCommand(db, geminiClient, formatter)
	lessonQuery := application.NewLessonQuery(db, geminiClient)
	// HẠ TẦNG MỚI: nối các adapter Assignment/OneNote/Gemini vào application.
	assignmentCmd := application.NewAssignmentCommand(db, db, db, db.Mistakes(), oneNoteClient, geminiClient)
	assignmentQuery := application.NewAssignmentQuery(db)
	workspaceCmd := application.NewWorkspaceCommand(db, oneNoteClient, db, db)
	workspaceQuery := application.NewWorkspaceQuery(oneNoteClient)
	// 3. Khởi tạo Wails Bridge App
	app := NewApp(meetCmd, meetQuery, lessonCmd, lessonQuery, geminiClient, workspaceCmd, workspaceQuery, assignmentCmd, assignmentQuery, oneNoteClient, meetClient)

	// 4. Khởi chạy Desktop App
	err = wails.Run(&options.App{
		Title:     "QUIN",
		Width:     1280,
		Height:    820,
		MinWidth:  1024,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []any{
			app,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}

func appDataDirectory() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	appDir := filepath.Join(dir, "QUIN")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return ".", err
	}
	return appDir, nil
}

// migrateLegacyUserFile chỉ hỗ trợ môi trường phát triển cũ, nơi dữ liệu nằm
// cạnh executable. Bản cài mới luôn dùng AppData nên không ghi vào Program Files.
func migrateLegacyUserFile(dataDir, filename string) error {
	target := filepath.Join(dataDir, filename)
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	source := filename
	in, err := os.Open(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return nil
}
