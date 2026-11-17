package dto

import (
	"carsharing/internal/models"
	"carsharing/internal/util"
)

// Stats — ответ GET /api/stats (docs/api.md, раздел 11).
type Stats struct {
	UsersTotal      int             `json:"users_total"`
	UsersByRole     map[string]int  `json:"users_by_role"`
	CarsByStatus    map[string]int  `json:"cars_by_status"`
	RentalsByStatus map[string]int  `json:"rentals_by_status"`
	AvgWaitMinutes  *float64        `json:"avg_wait_minutes"`
	RevenueByTariff []TariffRevenue `json:"revenue_by_tariff"`
}

type TariffRevenue struct {
	Tariff       string     `json:"tariff"`
	RentalsCount int        `json:"rentals_count"`
	Revenue      util.Money `json:"revenue"`
}

// NewStats формирует ответ. Во всех разбивках присутствуют все значения
// (роли и статусы с нулём тоже), чтобы диаграммы на клиенте не зависели от данных.
func NewStats(s models.Stats) Stats {
	out := Stats{
		UsersByRole:     withZeros(s.UsersByRole, models.UserRoles),
		CarsByStatus:    withZeros(s.CarsByStatus, models.CarStatuses),
		RentalsByStatus: withZeros(s.RentalsByStatus, models.RentalStatuses),
		AvgWaitMinutes:  s.AvgWaitMinutes,
		RevenueByTariff: make([]TariffRevenue, len(s.RevenueByTariff)),
	}
	for _, n := range out.UsersByRole {
		out.UsersTotal += n
	}
	for i, r := range s.RevenueByTariff {
		out.RevenueByTariff[i] = TariffRevenue{Tariff: r.Tariff, RentalsCount: r.RentalsCount, Revenue: r.Revenue}
	}
	return out
}

func withZeros(counts map[string]int, keys []string) map[string]int {
	out := make(map[string]int, len(keys))
	for _, k := range keys {
		out[k] = counts[k]
	}
	return out
}
