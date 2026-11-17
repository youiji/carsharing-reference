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

// RentalService — методы сервиса аренд (docs/api.md, раздел 9).
type RentalService interface {
	Book(ctx context.Context, user models.User, req dto.CreateRentalRequest) (dto.Rental, error)
	Start(ctx context.Context, user models.User, id int64) (dto.Rental, error)
	Finish(ctx context.Context, user models.User, id int64, req dto.FinishRentalRequest) (dto.Rental, error)
	Cancel(ctx context.Context, user models.User, id int64) (dto.Rental, error)
	Get(ctx context.Context, user models.User, id int64) (dto.Rental, error)
	ListMy(ctx context.Context, user models.User, f models.RentalFilter) (dto.List[dto.Rental], error)
	List(ctx context.Context, f models.RentalFilter) (dto.List[dto.Rental], error)
}

type RentalHandler struct {
	svc RentalService
}

// Book — POST /api/rentals.
func (h *RentalHandler) Book(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateRentalRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	user, _ := middleware.CurrentUser(r.Context())
	rental, err := h.svc.Book(r.Context(), user, req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusCreated, rental)
}

// Start — POST /api/rentals/{id}/start.
func (h *RentalHandler) Start(w http.ResponseWriter, r *http.Request) {
	h.action(w, r, h.svc.Start)
}

// Cancel — POST /api/rentals/{id}/cancel.
func (h *RentalHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	h.action(w, r, h.svc.Cancel)
}

// Get — GET /api/rentals/{id}.
func (h *RentalHandler) Get(w http.ResponseWriter, r *http.Request) {
	h.action(w, r, h.svc.Get)
}

// Finish — POST /api/rentals/{id}/finish. Тело необязательно.
func (h *RentalHandler) Finish(w http.ResponseWriter, r *http.Request) {
	id, err := util.PathID(r, "id")
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	var req dto.FinishRentalRequest
	if err := util.DecodeOptionalJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	user, _ := middleware.CurrentUser(r.Context())
	rental, err := h.svc.Finish(r.Context(), user, id, req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, rental)
}

// ListMy — GET /api/rentals/my.
func (h *RentalHandler) ListMy(w http.ResponseWriter, r *http.Request) {
	q := util.NewListQuery(r.URL.Query())
	f := models.RentalFilter{Status: q.Enum("status", models.RentalStatuses...)}
	f.Sort, f.Desc = q.Sort(repository.RentalSortNames...)
	if err := q.Err(); err != nil {
		util.WriteError(w, r, err)
		return
	}
	user, _ := middleware.CurrentUser(r.Context())
	list, err := h.svc.ListMy(r.Context(), user, f)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, list)
}

// List — GET /api/rentals.
func (h *RentalHandler) List(w http.ResponseWriter, r *http.Request) {
	q := util.NewListQuery(r.URL.Query())
	f := models.RentalFilter{
		Search: q.Search(),
		Status: q.Enum("status", models.RentalStatuses...),
		UserID: q.PositiveInt("user_id"),
		CarID:  q.PositiveInt("car_id"),
	}
	f.Sort, f.Desc = q.Sort(repository.RentalSortNames...)
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

// action — общий обработчик для действий над одной арендой без тела запроса.
func (h *RentalHandler) action(w http.ResponseWriter, r *http.Request,
	fn func(context.Context, models.User, int64) (dto.Rental, error)) {
	id, err := util.PathID(r, "id")
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	user, _ := middleware.CurrentUser(r.Context())
	rental, err := fn(r.Context(), user, id)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, rental)
}
