// Package handlers принимает HTTP-запросы, разбирает JSON, вызывает сервисы
// и формирует ответы. Бизнес-логики здесь нет.
package handlers

import (
	"net/http"
	"strings"

	"carsharing/internal/middleware"
	"carsharing/internal/models"
	"carsharing/internal/util"
)

// Deps — зависимости, нужные обработчикам.
type Deps struct {
	DB      Pinger
	Tokens  *util.TokenManager
	Users   middleware.UserLoader
	Auth    AuthService
	Rentals RentalService
}

// NewRouter собирает маршруты API (docs/api.md, раздел 3).
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	// protect оборачивает обработчик проверками доступа (docs/role_matrix.md, раздел 4):
	// без ролей — любой вошедший пользователь, с ролями — только указанные.
	authed := middleware.Auth(d.Tokens, d.Users)
	protect := func(h http.HandlerFunc, roles ...string) http.Handler {
		if len(roles) == 0 {
			return middleware.Chain(h, authed)
		}
		return middleware.Chain(h, authed, middleware.RequireRole(roles...))
	}

	health := &HealthHandler{db: d.DB}
	auth := &AuthHandler{svc: d.Auth}
	rentals := &RentalHandler{svc: d.Rentals}

	const (
		client  = models.RoleClient
		manager = models.RoleManager
		admin   = models.RoleAdmin
	)

	// Служебные
	mux.HandleFunc("GET /api/health", health.Check)

	// Аутентификация и профиль
	mux.HandleFunc("POST /api/auth/register", auth.Register)
	mux.HandleFunc("POST /api/auth/login", auth.Login)
	mux.Handle("GET /api/me", protect(auth.Me))
	mux.Handle("PUT /api/me/password", protect(auth.ChangePassword))

	// Аренды. Владелец аренды проверяется в сервисе.
	mux.Handle("POST /api/rentals", protect(rentals.Book, client))
	mux.Handle("POST /api/rentals/{id}/start", protect(rentals.Start, client))
	mux.Handle("POST /api/rentals/{id}/finish", protect(rentals.Finish, client))
	mux.Handle("POST /api/rentals/{id}/cancel", protect(rentals.Cancel, client, manager, admin))
	mux.Handle("GET /api/rentals/my", protect(rentals.ListMy, client))
	mux.Handle("GET /api/rentals", protect(rentals.List, manager, admin))
	mux.Handle("GET /api/rentals/{id}", protect(rentals.Get, client, manager, admin))

	// Остальные запросы к /api/: путь есть, но метод другой — 405 с заголовком Allow,
	// пути нет — 404. Оба ответа в едином JSON-формате.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		if allowed := allowedMethods(mux, r); len(allowed) > 0 {
			w.Header().Set("Allow", strings.Join(allowed, ", "))
			util.WriteError(w, r, util.ErrMethodNotAllowed)
			return
		}
		util.WriteError(w, r, util.ErrNotFound.WithMessage("Метод API не найден"))
	})

	// Logger снаружи, чтобы в лог попадали и ответы 500 после паники.
	return middleware.Chain(mux, middleware.Logger, middleware.Recover)
}

// allowedMethods возвращает методы, для которых путь запроса зарегистрирован в mux.
func allowedMethods(mux *http.ServeMux, r *http.Request) []string {
	var allowed []string
	for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if m == r.Method {
			continue
		}
		probe := r.Clone(r.Context())
		probe.Method = m
		if _, pattern := mux.Handler(probe); pattern != "" && pattern != "/api/" {
			allowed = append(allowed, m)
		}
	}
	return allowed
}
