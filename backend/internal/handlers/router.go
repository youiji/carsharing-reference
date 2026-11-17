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
	DB          Pinger
	Tokens      *util.TokenManager
	Users       middleware.UserLoader
	Auth        AuthService
	Rentals     RentalService
	Cars        CarService
	Tariffs     TariffService
	Maintenance MaintenanceService
	UsersAdmin  UserService
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
	cars := &CarHandler{svc: d.Cars}
	tariffs := &TariffHandler{svc: d.Tariffs}
	maintenance := &MaintenanceHandler{svc: d.Maintenance}
	users := &UserHandler{svc: d.UsersAdmin}

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

	// Каталог и тарифы (чтение). Видимость автомобилей для клиента — в сервисе.
	mux.Handle("GET /api/cars", protect(cars.List, client, manager, admin))
	mux.Handle("GET /api/cars/{id}", protect(cars.Get, client, manager, admin))
	mux.Handle("GET /api/tariffs", protect(tariffs.List, client, manager, admin))

	// Управление автопарком и тарифами
	mux.Handle("POST /api/cars", protect(cars.Create, manager, admin))
	mux.Handle("PUT /api/cars/{id}", protect(cars.Update, manager, admin))
	mux.Handle("PATCH /api/cars/{id}/status", protect(cars.ChangeStatus, manager, admin))
	mux.Handle("DELETE /api/cars/{id}", protect(cars.Delete, manager, admin))
	mux.Handle("POST /api/tariffs", protect(tariffs.Create, manager, admin))
	mux.Handle("PUT /api/tariffs/{id}", protect(tariffs.Update, manager, admin))
	mux.Handle("DELETE /api/tariffs/{id}", protect(tariffs.Delete, manager, admin))

	// Аренды. Владелец аренды проверяется в сервисе.
	mux.Handle("POST /api/rentals", protect(rentals.Book, client))
	mux.Handle("POST /api/rentals/{id}/start", protect(rentals.Start, client))
	mux.Handle("POST /api/rentals/{id}/finish", protect(rentals.Finish, client))
	mux.Handle("POST /api/rentals/{id}/cancel", protect(rentals.Cancel, client, manager, admin))
	mux.Handle("GET /api/rentals/my", protect(rentals.ListMy, client))
	mux.Handle("GET /api/rentals", protect(rentals.List, manager, admin))
	mux.Handle("GET /api/rentals/{id}", protect(rentals.Get, client, manager, admin))

	// Журнал обслуживания. Автор записи проверяется в сервисе.
	mux.Handle("GET /api/maintenance", protect(maintenance.List, manager, admin))
	mux.Handle("POST /api/maintenance", protect(maintenance.Create, manager, admin))
	mux.Handle("PATCH /api/maintenance/{id}", protect(maintenance.Update, manager, admin))
	mux.Handle("DELETE /api/maintenance/{id}", protect(maintenance.Delete, manager, admin))

	// Пользователи и роли
	mux.Handle("GET /api/users", protect(users.List, admin))
	mux.Handle("PATCH /api/users/{id}/role", protect(users.ChangeRole, admin))
	mux.Handle("PATCH /api/users/{id}", protect(users.Update, admin))
	mux.Handle("DELETE /api/users/{id}", protect(users.Delete, admin))

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
