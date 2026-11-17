package models

import (
	"time"

	"carsharing/internal/util"
)

// MaintenanceLog — строка таблицы maintenance_logs. Поля Car* и ManagerFullName
// заполняются при чтении (JOIN с cars и users).
type MaintenanceLog struct {
	ID          int64
	CarID       int64
	ManagerID   int64 // автор записи: менеджер или администратор
	Description string
	Cost        util.Money
	LoggedAt    time.Time

	CarBrandModel   string
	CarPlateNumber  string
	ManagerFullName string
}

// MaintenanceFilter — условия выборки журнала обслуживания.
type MaintenanceFilter struct {
	CarID  *int64
	Search string // по описанию, марке/модели и гос. номеру
	Sort   string // поле из repository.MaintenanceSortNames
	Desc   bool
}

// UserFilter — условия выборки списка пользователей.
type UserFilter struct {
	Role   *string
	Search string // по email и ФИО
	Sort   string // поле из repository.UserSortNames
	Desc   bool
}

// UserRoles — все допустимые роли.
var UserRoles = []string{RoleClient, RoleManager, RoleAdmin}
