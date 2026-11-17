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

// UserService — методы управления пользователями (docs/api.md, раздел 10).
type UserService interface {
	List(ctx context.Context, f models.UserFilter) (dto.List[dto.User], error)
	ChangeRole(ctx context.Context, admin models.User, id int64, req dto.UserRoleRequest) (dto.User, error)
	Update(ctx context.Context, id int64, req dto.UpdateUserRequest) (dto.User, error)
	Delete(ctx context.Context, admin models.User, id int64) error
}

type UserHandler struct {
	svc UserService
}

// List — GET /api/users.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	q := util.NewListQuery(r.URL.Query())
	f := models.UserFilter{
		Search: q.Search(),
		Role:   q.Enum("role", models.UserRoles...),
	}
	f.Sort, f.Desc = q.Sort(repository.UserSortNames...)
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

// ChangeRole — PATCH /api/users/{id}/role.
func (h *UserHandler) ChangeRole(w http.ResponseWriter, r *http.Request) {
	id, err := util.PathID(r, "id")
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	var req dto.UserRoleRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	admin, _ := middleware.CurrentUser(r.Context())
	u, err := h.svc.ChangeRole(r.Context(), admin, id, req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, u)
}

// Update — PATCH /api/users/{id}.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := util.PathID(r, "id")
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	var req dto.UpdateUserRequest
	if err := util.DecodeJSON(w, r, &req); err != nil {
		util.WriteError(w, r, err)
		return
	}
	u, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, u)
}

// Delete — DELETE /api/users/{id}.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := util.PathID(r, "id")
	if err != nil {
		util.WriteError(w, r, err)
		return
	}
	admin, _ := middleware.CurrentUser(r.Context())
	if err := h.svc.Delete(r.Context(), admin, id); err != nil {
		util.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
