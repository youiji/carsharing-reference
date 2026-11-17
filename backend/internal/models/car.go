package models

import "carsharing/internal/util"

// Статусы автомобиля (cars.status), docs/api.md, раздел 2.2.
const (
	CarAvailable      = "available"
	CarBooked         = "booked"
	CarInUse          = "in_use"
	CarMaintenance    = "maintenance"
	CarDecommissioned = "decommissioned"
)

// CarStatuses — все допустимые статусы автомобиля.
var CarStatuses = []string{CarAvailable, CarBooked, CarInUse, CarMaintenance, CarDecommissioned}

// Tariff — строка таблицы tariffs.
type Tariff struct {
	ID            int64
	Name          string
	PricePerMin   util.Money
	WaitingPerMin util.Money
}

// Car — строка таблицы cars вместе с тарифом (JOIN tariffs).
type Car struct {
	ID          int64
	TariffID    int64
	BrandModel  string
	PlateNumber string
	FuelLevel   int
	Latitude    float64
	Longitude   float64
	Status      string
	Tariff      Tariff
}

// CarFilter — условия выборки каталога.
type CarFilter struct {
	Status   *string
	TariffID *int64
	Search   string // по марке/модели и гос. номеру
	Sort     string // поле из repository.CarSortNames
	Desc     bool
}
