package dto

import "carsharing/internal/models"

// UserRoleRequest — тело PATCH /api/users/{id}/role.
type UserRoleRequest struct {
	Role string `json:"role"`
}

// UpdateUserRequest — тело PATCH /api/users/{id}: любое подмножество полей.
type UpdateUserRequest struct {
	FullName      *string `json:"full_name"`
	DriverLicense *string `json:"driver_license"`
	Email         *string `json:"email"`
}

func NewUsers(us []models.User) []User {
	out := make([]User, len(us))
	for i, u := range us {
		out[i] = NewUser(u)
	}
	return out
}
