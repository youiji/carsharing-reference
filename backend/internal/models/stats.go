package models

import "carsharing/internal/util"

// Stats — сводная статистика (docs/api.md, раздел 11).
type Stats struct {
	UsersByRole     map[string]int
	CarsByStatus    map[string]int
	RentalsByStatus map[string]int
	AvgWaitMinutes  *float64 // nil, если ни одна поездка не начиналась
	RevenueByTariff []TariffRevenue
}

// TariffRevenue — выручка по тарифу за завершённые аренды.
type TariffRevenue struct {
	Tariff       string
	RentalsCount int
	Revenue      util.Money
}
