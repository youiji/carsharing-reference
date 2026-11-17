// Package dto содержит структуры запросов и ответов REST API (docs/api.md).
package dto

import (
	"time"

	"carsharing/internal/models"
)

// RegisterRequest — тело POST /api/auth/register. Поле role не принимается.
type RegisterRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	FullName      string `json:"full_name"`
	DriverLicense string `json:"driver_license"`
}

// LoginRequest — тело POST /api/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ChangePasswordRequest — тело PUT /api/me/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// User — объект пользователя в ответах API. Пароль и хеш не передаются никогда.
type User struct {
	ID            int64     `json:"id"`
	Email         string    `json:"email"`
	FullName      string    `json:"full_name"`
	DriverLicense *string   `json:"driver_license"`
	Role          string    `json:"role"`
	CreatedAt     time.Time `json:"created_at"`
}

// LoginResponse — ответ POST /api/auth/login.
type LoginResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresIn int    `json:"expires_in"` // секунды
	User      User   `json:"user"`
}

// NewUser преобразует модель в объект ответа.
func NewUser(u models.User) User {
	return User{
		ID:            u.ID,
		Email:         u.Email,
		FullName:      u.FullName,
		DriverLicense: u.DriverLicense,
		Role:          u.Role,
		CreatedAt:     u.CreatedAt,
	}
}
