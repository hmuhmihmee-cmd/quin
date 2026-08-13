package main

import (
	"context"

	_ "embed"
	"errors"
	"log"
	"meet-attendance-clean/application"
	"meet-attendance-clean/config"
	"meet-attendance-clean/infrastructure/database"
	"meet-attendance-clean/infrastructure/gemini"
	"meet-attendance-clean/infrastructure/googlemeet"
	pdfadapter "meet-attendance-clean/infrastructure/pdf"
	"meet-attendance-clean/infrastructure/web"
	"meet-attendance-clean/infrastructure/web/handlers"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

//go:embed config/google-oauth.json
var embeddedGoogleCredentials []byte
//go:embed config/gemini-key.txt
var embeddedGeminiKey []byte
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg,err:= config.NewConfig()
	if err!=nil{
		return err
	}
	dataDir, err := applicationDirectory()
	if err != nil {
		return err
	}

	store, err := database.Open(filepath.Join(dataDir, "attendance.db"))
	if err != nil {
		return err
	}
	defer store.Close()

	meetClient, err := googlemeet.New(embeddedGoogleCredentials, filepath.Join(dataDir, "token.json"), cfg.GoogleRedirectURL)
	if err != nil {
		return err
	}
	apiKey := strings.TrimSpace(string(embeddedGeminiKey))

	attendance := application.NewAttendanceUseCase(store, store, meetClient, cfg.MeetLookbackMonths)
	lessons := application.NewLessonUseCase(gemini.New(apiKey, cfg.GeminiModel), pdfadapter.New())
	views, err := web.NewRenderer()
	if err != nil {
		return err
	}

	syncJob := handlers.NewSyncJob(attendance)
	dashboard := handlers.NewDashboard(attendance, views, syncJob)
	studentDetail := handlers.NewStudentDetail(attendance, views)
	studentUpdate := handlers.NewStudentUpdate(attendance)
	syncMeet := handlers.NewSyncMeet(syncJob)
	oauth := handlers.NewOAuth(attendance)
	lesson := handlers.NewGenerateLesson(lessons, views)

	router := web.NewRouter(web.Routes{
		Dashboard: dashboard, StudentDetail: studentDetail,
		StudentUpdate: studentUpdate, SyncMeet: syncMeet,
		OAuthLogin: oauth.Login, OAuthCallback: oauth.Callback, OAuthDisconnect: oauth.Disconnect,
		LessonPage: lesson.Page, LessonGenerate: lesson.Generate, LessonExport: lesson.Export,
	})
	server := &http.Server{Addr: cfg.Address, Handler: router, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 15 * time.Minute, WriteTimeout: 20 * time.Minute, IdleTimeout: 60 * time.Second}

	serverError := make(chan error, 1)
	go func() {
		log.Printf("Ứng dụng đang chạy tại http://localhost:9000")
		log.Printf("Dữ liệu được lưu tại %s", dataDir)
		serverError <- server.ListenAndServe()
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-serverError:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
	return nil
}

func applicationDirectory() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	executableDir := filepath.Dir(executablePath)

	// `go run` places its executable in the operating system's temporary
	// directory. Keep development data in the project directory instead.
	if relative, err := filepath.Rel(os.TempDir(), executableDir); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		if workingDir, err := os.Getwd(); err == nil {
			return workingDir, nil
		}
	}
	return executableDir, nil
}
