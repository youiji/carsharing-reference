package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"carsharing/internal/dto"
	"carsharing/internal/middleware"
	"carsharing/internal/ratelimit"
	"carsharing/internal/util"
)

// AuthService — методы сервиса, нужные обработчикам раздела 5 API.
type AuthService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (dto.User, error)
	Login(ctx context.Context, req dto.LoginRequest) (dto.LoginResponse, error)
	ChangePassword(ctx context.Context, userID int64, req dto.ChangePasswordRequest) error
}

type AuthHandler struct {
	svc     AuthService
	limiter *ratelimit.LoginLimiter
}

// Register — POST /api/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	user, err := h.svc.Register(r.Context(), req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusCreated, user)
}

// Login — POST /api/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	// Защита от подбора пароля: после серии неудачных попыток вход временно блокируется.
	ip := clientIP(r)
	if wait, ok := h.limiter.Check(ip, req.Email); !ok {
		minutes := int((wait + time.Minute - 1) / time.Minute)
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		util.WriteError(w, r, util.ErrTooManyAttempts.WithMessage(
			fmt.Sprintf("Слишком много неудачных попыток входа. Повторите через %d мин.", minutes)))
		return
	}
	resp, err := h.svc.Login(r.Context(), req)
	if err != nil {
		if appErr, ok := util.AsAppError(err); ok && appErr.Code == util.ErrInvalidCredentials.Code {
			h.limiter.Fail(ip, req.Email)
		}
		util.WriteError(w, r, err)
		return
	}
	h.limiter.Success(req.Email)
	util.WriteJSON(w, http.StatusOK, resp)
}

// Me — GET /api/me: профиль текущего пользователя (уже загружен в Auth).
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.CurrentUser(r.Context())
	util.WriteJSON(w, http.StatusOK, dto.NewUser(user))
}

// ChangePassword — PUT /api/me/password.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req dto.ChangePasswordRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	user, _ := middleware.CurrentUser(r.Context())
	if err := h.svc.ChangePassword(r.Context(), user.ID, req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clientIP — адрес клиента из соединения. Заголовкам X-Forwarded-For не доверяем:
// сервер работает без прокси, а заголовок клиент может подделать.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
