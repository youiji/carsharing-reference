// Package app собирает приложение из слоёв: репозитории → сервисы → обработчики.
// Используется сервером (cmd/server) и сквозными тестами (e2e), поэтому тесты
// проверяют ту же сборку маршрутов и зависимостей, что работает в продакшене.
package app

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"carsharing/internal/handlers"
	"carsharing/internal/repository"
	"carsharing/internal/services"
	"carsharing/internal/util"
)

// NewHandler возвращает HTTP-обработчик всего API.
func NewHandler(pool *pgxpool.Pool, tokens *util.TokenManager) http.Handler {
	userRepo := repository.NewUserRepository(pool)
	return handlers.NewRouter(handlers.Deps{
		DB:          pool,
		Tokens:      tokens,
		Users:       userRepo,
		Auth:        services.NewAuthService(userRepo, tokens),
		Rentals:     services.NewRentalService(repository.NewRentalRepository(pool)),
		Cars:        services.NewCarService(repository.NewCarRepository(pool)),
		Tariffs:     services.NewTariffService(repository.NewTariffRepository(pool)),
		Maintenance: services.NewMaintenanceService(repository.NewMaintenanceRepository(pool)),
		UsersAdmin:  services.NewUserService(userRepo),
		Stats:       services.NewStatsService(repository.NewStatsRepository(pool)),
	})
}
