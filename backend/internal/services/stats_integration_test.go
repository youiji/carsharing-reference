package services

import (
	"testing"
	"time"

	"carsharing/internal/dto"
	"carsharing/internal/models"
	"carsharing/internal/repository"
	"carsharing/internal/util"
)

// Статистика проверяется по приращениям: снимок до, одна завершённая поездка,
// снимок после. Так тест не зависит от текущего содержимого базы.
func TestIntegrationStats(t *testing.T) {
	e := newRentalEnv(t)
	stats := NewStatsService(repository.NewStatsRepository(e.pool))

	before, err := stats.Get(e.ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Lada Vesta, тариф Эконом (8.90 / 2.00): ожидание 2 мин, поездка 10 мин → 93.00
	carID := e.availableCar("Н606НН777")
	u := e.client(true)
	r, err := e.svc.Book(e.ctx, u, dto.CreateRentalRequest{CarID: carID})
	if err != nil {
		t.Fatal(err)
	}
	e.clock = e.clock.Add(2 * time.Minute)
	if _, err := e.svc.Start(e.ctx, u, r.ID); err != nil {
		t.Fatal(err)
	}
	e.clock = e.clock.Add(10 * time.Minute)
	if _, err := e.svc.Finish(e.ctx, u, r.ID, dto.FinishRentalRequest{}); err != nil {
		t.Fatal(err)
	}

	after, err := stats.Get(e.ctx)
	if err != nil {
		t.Fatal(err)
	}

	if after.UsersTotal != before.UsersTotal+1 || after.UsersByRole[models.RoleClient] != before.UsersByRole[models.RoleClient]+1 {
		t.Errorf("пользователи: до %v, после %v", before.UsersByRole, after.UsersByRole)
	}
	if after.RentalsByStatus[models.RentalCompleted] != before.RentalsByStatus[models.RentalCompleted]+1 {
		t.Errorf("завершённые аренды: до %v, после %v", before.RentalsByStatus, after.RentalsByStatus)
	}
	for _, st := range models.CarStatuses {
		if after.CarsByStatus[st] != before.CarsByStatus[st] {
			t.Errorf("машина вернулась в available — разбивка по статусам не должна измениться: %v → %v",
				before.CarsByStatus, after.CarsByStatus)
			break
		}
	}
	if after.AvgWaitMinutes == nil {
		t.Error("avg_wait_minutes не должно быть null после начатой поездки")
	}

	econ := func(s dto.Stats) dto.TariffRevenue {
		for _, r := range s.RevenueByTariff {
			if r.Tariff == "Эконом" {
				return r
			}
		}
		t.Fatalf("нет тарифа Эконом в %v", s.RevenueByTariff)
		return dto.TariffRevenue{}
	}
	b, a := econ(before), econ(after)
	if a.RentalsCount != b.RentalsCount+1 || a.Revenue != b.Revenue+util.Money(9300) {
		t.Errorf("выручка Эконом: до %+v, после %+v (ожидалось +1 аренда и +93.00)", b, a)
	}
	for i := 1; i < len(after.RevenueByTariff); i++ {
		if after.RevenueByTariff[i].Revenue > after.RevenueByTariff[i-1].Revenue {
			t.Errorf("выручка должна быть отсортирована по убыванию: %v", after.RevenueByTariff)
		}
	}
}
