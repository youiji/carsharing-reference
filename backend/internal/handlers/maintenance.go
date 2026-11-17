package handlers

import (
	"context"
	"net/http"

	"carsharing/internal/dto"
	"carsharing/internal/middleware"
	"carsharing/internal/models"
	"carsharing/internal/repository"
	"carsharing/internal/util"
)

// MaintenanceService — методы сервиса журнала обслуживания (docs/api.md, раздел 8).
type MaintenanceService interface {
	List(ctx context.Context, f models.MaintenanceFilter) (dto.List[dto.Maintenance], error)
	Create(ctx context.Context, author models.User, req dto.CreateMaintenanceRequest) (dto.Maintenance, error)
	Update(ctx context.Context, user models.User, id int64, req dto.UpdateMaintenanceRequest) (dto.Maintenance, error)
	Delete(ctx context.Context, user models.User, id int64) error
}

type MaintenanceHandler struct {
	svc MaintenanceService
}

// List — GET /api/maintenance.
func (h *MaintenanceHandler) List(w http.ResponseWriter, r *http.Request) {
	q := util.NewListQuery(r.URL.Query())
	f := models.MaintenanceFilter{
		CarID:  q.PositiveInt("car_id"),
		Search: q.Search(),
	}
	f.Sort, f.Desc = q.Sort(repository.MaintenanceSortNames...)
	if err := q.Err(); err != nil {
		util.WriteError(w, r, err)
		return
	}
	list, err := h.svc.List(r.Context(), f)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, list)
}

// Create — POST /api/maintenance.
func (h *MaintenanceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateMaintenanceRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	user, _ := middleware.CurrentUser(r.Context())
	m, err := h.svc.Create(r.Context(), user, req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusCreated, m)
}

// Update — PATCH /api/maintenance/{id}.
func (h *MaintenanceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := util.PathID(r, "id")
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	var req dto.UpdateMaintenanceRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	user, _ := middleware.CurrentUser(r.Context())
	m, err := h.svc.Update(r.Context(), user, id, req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, m)
}

// Delete — DELETE /api/maintenance/{id}.
func (h *MaintenanceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := util.PathID(r, "id")
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	user, _ := middleware.CurrentUser(r.Context())
	if err := h.svc.Delete(r.Context(), user, id); err != nil {
		util.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
