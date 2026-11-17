package handlers

import (
	"context"
	"net/http"

	"carsharing/internal/dto"
	"carsharing/internal/middleware"
	"carsharing/internal/util"
)

// AuthService — методы сервиса, нужные обработчикам раздела 5 API.
type AuthService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (dto.User, error)
	Login(ctx context.Context, req dto.LoginRequest) (dto.LoginResponse, error)
	ChangePassword(ctx context.Context, userID int64, req dto.ChangePasswordRequest) error
}

type AuthHandler struct {
	svc AuthService
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
	resp, err := h.svc.Login(r.Context(), req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
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
