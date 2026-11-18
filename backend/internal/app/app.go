// Package app собирает приложение из слоёв: репозитории → сервисы → обработчики.
// Используется сервером (cmd/server) и сквозными тестами (e2e), поэтому тесты
// проверяют ту же сборку маршрутов и зависимостей, что работает в продакшене.
package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"carsharing/internal/handlers"
	"carsharing/internal/middleware"
	"carsharing/internal/repository"
	"carsharing/internal/services"
	"carsharing/internal/util"
	"carsharing/internal/web"
)

// NewHandler возвращает HTTP-обработчик всего приложения: REST API под /api/
// и клиентскую часть (страницы и статику) из каталога frontendDir.
func NewHandler(pool *pgxpool.Pool, tokens *util.TokenManager, frontendDir string) (http.Handler, error) {
	site, err := web.New(frontendDir)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", newAPI(pool, tokens))
	mux.Handle("/", middleware.Chain(site, middleware.Logger, middleware.Recover))
	return web.SecurityHeaders(mux), nil
}

// newAPI собирает REST API: репозитории → сервисы → обработчики.
func newAPI(pool *pgxpool.Pool, tokens *util.TokenManager) http.Handler {
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

// RunBookingExpiry периодически отменяет просроченные бронирования (models.BookingTTL)
// со списанием ожидания, пока не отменён ctx. Запросы старта и отмены проверяют срок
// сами, поэтому задержка задачи на корректность не влияет: она лишь освобождает машину.
func RunBookingExpiry(ctx context.Context, pool *pgxpool.Pool, every time.Duration) {
	svc := services.NewRentalService(repository.NewRentalRepository(pool))
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		if n, err := svc.ExpireBookings(ctx); err != nil && ctx.Err() == nil {
			slog.Error("expire bookings", "err", err)
		} else if n > 0 {
			slog.Info("bookings expired", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
