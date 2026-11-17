package dto

import (
	"time"

	"carsharing/internal/models"
	"carsharing/internal/util"
)

// List — ответ эндпоинтов-списков: { "items": [...], "total": N }.
type List[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

// NewList гарантирует "items": [] вместо null для пустого списка.
func NewList[T any](items []T) List[T] {
	if items == nil {
		items = []T{}
	}
	return List[T]{Items: items, Total: len(items)}
}

// CreateRentalRequest — тело POST /api/rentals.
type CreateRentalRequest struct {
	CarID int64 `json:"car_id"`
}

// FinishRentalRequest — тело POST /api/rentals/{id}/finish (все поля необязательны).
type FinishRentalRequest struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	FuelLevel *int     `json:"fuel_level"`
}

type RentalUser struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
}

type RentalCar struct {
	ID          int64  `json:"id"`
	BrandModel  string `json:"brand_model"`
	PlateNumber string `json:"plate_number"`
}

// Rental — объект аренды (docs/api.md, раздел 9).
type Rental struct {
	ID            int64       `json:"id"`
	User          RentalUser  `json:"user"`
	Car           RentalCar   `json:"car"`
	TariffID      int64       `json:"tariff_id"`
	BookedAt      time.Time   `json:"booked_at"`
	StartedAt     *time.Time  `json:"started_at"`
	EndedAt       *time.Time  `json:"ended_at"`
	PricePerMin   util.Money  `json:"price_per_min"`
	WaitingPerMin util.Money  `json:"waiting_per_min"`
	TotalCost     *util.Money `json:"total_cost"`
	Status        string      `json:"status"`
}

func NewRental(r models.Rental) Rental {
	return Rental{
		ID:            r.ID,
		User:          RentalUser{ID: r.UserID, FullName: r.UserFullName},
		Car:           RentalCar{ID: r.CarID, BrandModel: r.CarBrandModel, PlateNumber: r.CarPlateNumber},
		TariffID:      r.TariffID,
		BookedAt:      r.BookedAt,
		StartedAt:     r.StartedAt,
		EndedAt:       r.EndedAt,
		PricePerMin:   r.PricePerMin,
		WaitingPerMin: r.WaitingPerMin,
		TotalCost:     r.TotalCost,
		Status:        r.Status,
	}
}

func NewRentals(rs []models.Rental) []Rental {
	out := make([]Rental, len(rs))
	for i, r := range rs {
		out[i] = NewRental(r)
	}
	return out
}
