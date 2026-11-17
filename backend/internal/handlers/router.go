// Package handlers принимает HTTP-запросы, разбирает JSON, вызывает сервисы
// и формирует ответы. Бизнес-логики здесь нет.
package handlers

import (
	"net/http"

	"carsharing/internal/middleware"
	"carsharing/internal/util"
)

// Deps — зависимости, нужные обработчикам.
type Deps struct {
	DB     Pinger
	Tokens *util.TokenManager
	Users  middleware.UserLoader
	Auth   AuthService
}

// NewRouter собирает маршруты API (docs/api.md, раздел 3).
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	// protect оборачивает обработчик проверками доступа (docs/role_matrix.md, раздел 4):
	// без ролей — любой вошедший пользователь, с ролями — только указанные.
	// Пример для функций менеджера: protect(h, models.RoleManager, models.RoleAdmin).
	authed := middleware.Auth(d.Tokens, d.Users)
	protect := func(h http.HandlerFunc, roles ...string) http.Handler {
		if len(roles) == 0 {
			return middleware.Chain(h, authed)
		}
		return middleware.Chain(h, authed, middleware.RequireRole(roles...))
	}

	health := &HealthHandler{db: d.DB}
	auth := &AuthHandler{svc: d.Auth}

	// Служебные
	mux.HandleFunc("GET /api/health", health.Check)

	// Аутентификация и профиль
	mux.HandleFunc("POST /api/auth/register", auth.Register)
	mux.HandleFunc("POST /api/auth/login", auth.Login)
	mux.Handle("GET /api/me", protect(auth.Me))
	mux.Handle("PUT /api/me/password", protect(auth.ChangePassword))

	// Неизвестный путь API — 404 в едином JSON-формате.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		util.WriteError(w, r, util.ErrNotFound.WithMessage("Метод API не найден"))
	})

	// Logger снаружи, чтобы в лог попадали и ответы 500 после паники.
	return middleware.Chain(mux, middleware.Logger, middleware.Recover)
}
