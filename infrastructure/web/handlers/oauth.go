package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"meet-attendance-clean/application"
)

type OAuth struct {
	useCase *application.AttendanceUseCase
}

func NewOAuth(useCase *application.AttendanceUseCase) *OAuth { return &OAuth{useCase: useCase} }

func (h *OAuth) Login(writer http.ResponseWriter, request *http.Request) {
	stateBytes := make([]byte, 24)
	if _, err := rand.Read(stateBytes); err != nil {
		renderError(writer, err)
		return
	}
	state := hex.EncodeToString(stateBytes)
	loginURL, err := h.useCase.AuthorizationURL(state)
	if err != nil {
		redirectError(writer, request, err)
		return
	}
	http.SetCookie(writer, &http.Cookie{Name: "oauth_state", Value: state, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 600, Expires: time.Now().Add(10 * time.Minute)})
	http.Redirect(writer, request, loginURL, http.StatusFound)
}

func (h *OAuth) Callback(writer http.ResponseWriter, request *http.Request) {
	cookie, err := request.Cookie("oauth_state")
	if err != nil || cookie.Value == "" || request.URL.Query().Get("state") != cookie.Value {
		redirectError(writer, request, errInvalidOAuthState)
		return
	}
	if err := h.useCase.ConnectGoogle(request.Context(), request.URL.Query().Get("code")); err != nil {
		redirectError(writer, request, err)
		return
	}
	http.SetCookie(writer, &http.Cookie{Name: "oauth_state", Value: "", Path: "/", HttpOnly: true, MaxAge: -1, SameSite: http.SameSiteLaxMode})
	redirectNotice(writer, request, "Đã liên kết tài khoản Google.")
}

func (h *OAuth) Disconnect(writer http.ResponseWriter, request *http.Request) {
	if err := h.useCase.DisconnectGoogle(); err != nil {
		redirectError(writer, request, err)
		return
	}
	redirectNotice(writer, request, "Đã ngắt liên kết tài khoản Google.")
}
