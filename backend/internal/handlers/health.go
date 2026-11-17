package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"carsharing/internal/util"
)

// Pinger проверяет доступность БД.
type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	db Pinger
}

// Check — GET /api/health: сервер запущен и видит БД.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		slog.Warn("health check: database unavailable", "err", err)
		util.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "degraded", "database": "unavailable",
		})
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "ok"})
}
