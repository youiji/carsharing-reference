package models

import (
	"time"

	"carsharing/internal/util"
)

// Статусы аренды (rentals.status), docs/api.md, раздел 2.1.
const (
	RentalBooked    = "booked"
	RentalActive    = "active"
	RentalCompleted = "completed"
	RentalCancelled = "cancelled"
)

// RentalStatuses — все допустимые статусы аренды.
var RentalStatuses = []string{RentalBooked, RentalActive, RentalCompleted, RentalCancelled}

// Rental — строка таблицы rentals. Поля User* и Car* заполняются
// при чтении (JOIN с users и cars) и не хранятся в rentals.
type Rental struct {
	ID            int64
	UserID        int64
	TariffID      int64
	CarID         int64
	BookedAt      time.Time
	StartedAt     *time.Time
	EndedAt       *time.Time
	PricePerMin   util.Money // снимок цены тарифа на момент бронирования
	WaitingPerMin util.Money
	TotalCost     *util.Money
	Status        string

	UserFullName   string
	CarBrandModel  string
	CarPlateNumber string
}

// RentalFilter — условия выборки списка аренд.
type RentalFilter struct {
	UserID *int64
	CarID  *int64
	Status *string
	Search string // по ФИО пользователя и гос. номеру
	Sort   string // поле из repository.RentalSortFields
	Desc   bool
}

// CarLocation — место парковки после поездки (поля необязательны, передаются вместе).
type CarLocation struct {
	Latitude  *float64
	Longitude *float64
}
