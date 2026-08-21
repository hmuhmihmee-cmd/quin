package main

import (
	"embed"
	"encoding/json"
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

	db, err := database.Open("./attendance.db")
	if err != nil {
		log.Fatalf("Lỗi mở SQLite: %v", err)
	}
	defer db.Close()

	meetClient, _ := googlemeet.New(embeddedGoogleCredentials, "token.json", cfg.GoogleRedirectURL)
	geminiClient := gemini.New(string(embeddedGeminiKey), cfg.GeminiModel)
	formatter := pdf.NewFormatter()
	renderer := pdf.NewRenderer()
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
		filepath.Join("", "token_microsoft.json"),
	)
	// 2. Khởi tạo Application UseCases
	meetCmd := application.NewMeetCommand(db, meetClient, db, db)
	meetQuery := application.NewMeetQuery(db)
	lessonCmd := application.NewLessonCommand(db, geminiClient, formatter, renderer)
	lessonQuery := application.NewLessonQuery(db, geminiClient)
	// HẠ TẦNG MỚI: nối các adapter Assignment/OneNote/Gemini vào application.
	assignmentCmd := application.NewAssignmentCommand(db, db, db, db, oneNoteClient, geminiClient)
	assignmentQuery := application.NewAssignmentQuery(db)
	workspaceCmd := application.NewWorkspaceCommand(db, oneNoteClient, db, db)
	workspaceQuery := application.NewWorkspaceQuery(oneNoteClient)
	// 3. Khởi tạo Wails Bridge App
	app := NewApp(meetCmd, meetQuery, lessonCmd, lessonQuery, geminiClient, workspaceCmd, workspaceQuery, assignmentCmd, assignmentQuery, oneNoteClient, meetClient)

	// 4. Khởi chạy Desktop App
	err = wails.Run(&options.App{
		Title:     "Lớp học 1–1 & Trợ lý Soạn bài",
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
	appDir := filepath.Join(dir, "MeetAttendanceApp")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return ".", err
	}
	return appDir, nil
}
