// Package models содержит структуры, соответствующие таблицам БД.
package models

import "time"

// Роли пользователей (users.role).
const (
	RoleClient  = "client"
	RoleManager = "manager"
	RoleAdmin   = "admin"
)

// User — строка таблицы users.
type User struct {
	ID            int64
	Email         string
	PasswordHash  string
	FullName      string
	DriverLicense *string // NULL у сотрудников и старых аккаунтов
	Role          string
	CreatedAt     time.Time
}
