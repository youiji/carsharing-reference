package handlers

import (
	"context"
	"net/http"

	"carsharing/internal/dto"
	"carsharing/internal/util"
)

// TariffService — методы сервиса тарифов (docs/api.md, разделы 6–7).
type TariffService interface {
	List(ctx context.Context) (dto.List[dto.Tariff], error)
	Create(ctx context.Context, req dto.TariffRequest) (dto.Tariff, error)
	Update(ctx context.Context, id int64, req dto.TariffRequest) (dto.Tariff, error)
	Delete(ctx context.Context, id int64) error
}

type TariffHandler struct {
	svc TariffService
}

// List — GET /api/tariffs.
func (h *TariffHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context())
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, list)
}

// Create — POST /api/tariffs.
func (h *TariffHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.TariffRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	t, err := h.svc.Create(r.Context(), req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusCreated, t)
}

// Update — PUT /api/tariffs/{id}.
func (h *TariffHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := util.PathID(r, "id")
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	var req dto.TariffRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	t, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, t)
}

// Delete — DELETE /api/tariffs/{id}.
func (h *TariffHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
