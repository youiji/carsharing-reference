package dto

import (
	"carsharing/internal/models"
	"carsharing/internal/util"
)

// Tariff — объект тарифа (docs/api.md, раздел 6).
type Tariff struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	PricePerMin   util.Money `json:"price_per_min"`
	WaitingPerMin util.Money `json:"waiting_per_min"`
}

// Car — объект автомобиля (docs/api.md, раздел 6).
type Car struct {
	ID          int64   `json:"id"`
	BrandModel  string  `json:"brand_model"`
	PlateNumber string  `json:"plate_number"`
	FuelLevel   int     `json:"fuel_level"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Status      string  `json:"status"`
	Tariff      Tariff  `json:"tariff"`
}

// CarRequest — тело POST /api/cars и PUT /api/cars/{id}.
// Указатели позволяют отличить отсутствующее поле от нуля (fuel_level = 0 допустим).
type CarRequest struct {
	TariffID    *int64   `json:"tariff_id"`
	BrandModel  string   `json:"brand_model"`
	PlateNumber string   `json:"plate_number"`
	FuelLevel   *int     `json:"fuel_level"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
}

// CarStatusRequest — тело PATCH /api/cars/{id}/status.
type CarStatusRequest struct {
	Status string `json:"status"`
}

// TariffRequest — тело POST /api/tariffs и PUT /api/tariffs/{id}.
type TariffRequest struct {
	Name          string      `json:"name"`
	PricePerMin   *util.Money `json:"price_per_min"`
	WaitingPerMin *util.Money `json:"waiting_per_min"`
}

func NewTariff(t models.Tariff) Tariff {
	return Tariff{ID: t.ID, Name: t.Name, PricePerMin: t.PricePerMin, WaitingPerMin: t.WaitingPerMin}
}

func NewTariffs(ts []models.Tariff) []Tariff {
	out := make([]Tariff, len(ts))
	for i, t := range ts {
		out[i] = NewTariff(t)
	}
	return out
}

func NewCar(c models.Car) Car {
	return Car{
		ID:          c.ID,
		BrandModel:  c.BrandModel,
		PlateNumber: c.PlateNumber,
		FuelLevel:   c.FuelLevel,
		Latitude:    c.Latitude,
		Longitude:   c.Longitude,
		Status:      c.Status,
		Tariff:      NewTariff(c.Tariff),
	}
}

func NewCars(cs []models.Car) []Car {
	out := make([]Car, len(cs))
	for i, c := range cs {
		out[i] = NewCar(c)
	}
	return out
}
