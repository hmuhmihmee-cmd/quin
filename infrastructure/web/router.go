package web

import "net/http"

type Routes struct {
	Dashboard       http.Handler
	StudentDetail   http.Handler
	StudentUpdate   http.Handler
	SyncMeet        http.Handler
	OAuthLogin      http.HandlerFunc
	OAuthCallback   http.HandlerFunc
	OAuthDisconnect http.HandlerFunc
	LessonPage      http.HandlerFunc
	LessonGenerate  http.HandlerFunc
	LessonExport    http.HandlerFunc
}

func NewRouter(routes Routes) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", assetsHandler())
	mux.Handle("GET /{$}", routes.Dashboard)
	mux.Handle("GET /students/{id}", routes.StudentDetail)
	mux.Handle("POST /students/{id}", routes.StudentUpdate)
	mux.Handle("POST /meet/sync", routes.SyncMeet)
	mux.HandleFunc("GET /oauth/login", routes.OAuthLogin)
	mux.HandleFunc("GET /oauth2callback", routes.OAuthCallback)
	mux.HandleFunc("POST /oauth/disconnect", routes.OAuthDisconnect)
	mux.HandleFunc("GET /lessons", routes.LessonPage)
	mux.HandleFunc("POST /lessons/generate", routes.LessonGenerate)
	mux.HandleFunc("POST /lessons/export", routes.LessonExport)
	return securityHeaders(limitFormSize(mux))
}

func limitFormSize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
		}
		next.ServeHTTP(writer, request)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "same-origin")
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(writer, request)
	})
}
