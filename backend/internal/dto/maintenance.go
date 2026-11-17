package dto

import (
	"time"

	"carsharing/internal/models"
	"carsharing/internal/util"
)

type MaintenanceCar struct {
	ID          int64  `json:"id"`
	BrandModel  string `json:"brand_model"`
	PlateNumber string `json:"plate_number"`
}

type MaintenanceManager struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
}

// Maintenance — объект записи журнала обслуживания (docs/api.md, раздел 8).
type Maintenance struct {
	ID          int64              `json:"id"`
	Car         MaintenanceCar     `json:"car"`
	Manager     MaintenanceManager `json:"manager"`
	Description string             `json:"description"`
	Cost        util.Money         `json:"cost"`
	LoggedAt    time.Time          `json:"logged_at"`
}

// CreateMaintenanceRequest — тело POST /api/maintenance. manager_id не принимается.
type CreateMaintenanceRequest struct {
	CarID       int64       `json:"car_id"`
	Description string      `json:"description"`
	Cost        *util.Money `json:"cost"`
}

// UpdateMaintenanceRequest — тело PATCH /api/maintenance/{id}: только изменяемые поля.
type UpdateMaintenanceRequest struct {
	Description *string     `json:"description"`
	Cost        *util.Money `json:"cost"`
}

func NewMaintenance(m models.MaintenanceLog) Maintenance {
	return Maintenance{
		ID:          m.ID,
		Car:         MaintenanceCar{ID: m.CarID, BrandModel: m.CarBrandModel, PlateNumber: m.CarPlateNumber},
		Manager:     MaintenanceManager{ID: m.ManagerID, FullName: m.ManagerFullName},
		Description: m.Description,
		Cost:        m.Cost,
		LoggedAt:    m.LoggedAt,
	}
}

func NewMaintenanceList(ms []models.MaintenanceLog) []Maintenance {
	out := make([]Maintenance, len(ms))
	for i, m := range ms {
		out[i] = NewMaintenance(m)
	}
	return out
}
