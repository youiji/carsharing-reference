package handlers

import (
	"context"
	"net/http"

	"carsharing/internal/dto"
	"carsharing/internal/util"
)

// StatsService — сводная статистика (docs/api.md, раздел 11).
type StatsService interface {
	Get(ctx context.Context) (dto.Stats, error)
}

type StatsHandler struct {
	svc StatsService
}

// Get — GET /api/stats.
func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.Get(r.Context())
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, stats)
}
