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

// CarService — методы сервиса автопарка (docs/api.md, разделы 6–7).
type CarService interface {
	List(ctx context.Context, user models.User, f models.CarFilter) (dto.List[dto.Car], error)
	Get(ctx context.Context, user models.User, id int64) (dto.Car, error)
	Create(ctx context.Context, req dto.CarRequest) (dto.Car, error)
	Update(ctx context.Context, id int64, req dto.CarRequest) (dto.Car, error)
	ChangeStatus(ctx context.Context, id int64, req dto.CarStatusRequest) (dto.Car, error)
	Delete(ctx context.Context, id int64) error
}

type CarHandler struct {
	svc CarService
}

// List — GET /api/cars.
func (h *CarHandler) List(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.CurrentUser(r.Context())
	q := util.NewListQuery(r.URL.Query())
	f := models.CarFilter{
		Search:   q.Search(),
		TariffID: q.PositiveInt("tariff_id"),
	}
	if user.Role != models.RoleClient { // для клиента status игнорируется
		f.Status = q.Enum("status", models.CarStatuses...)
	}
	f.Sort, f.Desc = q.Sort(repository.CarSortNames...)
	if err := q.Err(); err != nil {
		util.WriteError(w, r, err)
		return
	}
	list, err := h.svc.List(r.Context(), user, f)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, list)
}

// Get — GET /api/cars/{id}.
func (h *CarHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := util.PathID(r, "id")
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	user, _ := middleware.CurrentUser(r.Context())
	car, err := h.svc.Get(r.Context(), user, id)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, car)
}

// Create — POST /api/cars.
func (h *CarHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CarRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	car, err := h.svc.Create(r.Context(), req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusCreated, car)
}

// Update — PUT /api/cars/{id}.
func (h *CarHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := util.PathID(r, "id")
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	var req dto.CarRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	car, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, car)
}

// ChangeStatus — PATCH /api/cars/{id}/status.
func (h *CarHandler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	id, err := util.PathID(r, "id")
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	var req dto.CarStatusRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	car, err := h.svc.ChangeStatus(r.Context(), id, req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, car)
}

// Delete — DELETE /api/cars/{id}.
func (h *CarHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := util.PathID(r, "id")
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		util.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
