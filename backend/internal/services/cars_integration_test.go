package services

import (
	"context"
	"sync"
	"testing"

	"carsharing/internal/dto"
	"carsharing/internal/models"
	"carsharing/internal/repository"
	"carsharing/internal/util"
)

// Интеграционные тесты каталога, автопарка и тарифов. Запуск — как у тестов аренд
// (см. rentals_integration_test.go). Созданные записи помечаются префиксом "IT "
// и удаляются в конце.

type fleetEnv struct {
	*rentalEnv
	cars    *CarService
	tariffs *TariffService
}

func newFleetEnv(t *testing.T) *fleetEnv {
	e := newRentalEnv(t)
	f := &fleetEnv{
		rentalEnv: e,
		cars:      NewCarService(repository.NewCarRepository(e.pool)),
		tariffs:   NewTariffService(repository.NewTariffRepository(e.pool)),
	}
	t.Cleanup(func() {
		_, err := e.pool.Exec(context.Background(), `
			DELETE FROM rentals WHERE car_id IN (SELECT id FROM cars WHERE brand_model LIKE 'IT %');
			DELETE FROM maintenance_logs WHERE car_id IN (SELECT id FROM cars WHERE brand_model LIKE 'IT %');
			DELETE FROM cars WHERE brand_model LIKE 'IT %';
			DELETE FROM rentals WHERE tariff_id IN (SELECT id FROM tariffs WHERE name LIKE 'IT %');
			DELETE FROM tariffs WHERE name LIKE 'IT %';`)
		if err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return f
}

func (f *fleetEnv) tariff(name string, price, wait util.Money) dto.Tariff {
	tr, err := f.tariffs.Create(f.ctx, dto.TariffRequest{Name: name, PricePerMin: &price, WaitingPerMin: &wait})
	if err != nil {
		f.t.Fatalf("create tariff: %v", err)
	}
	return tr
}

func carReq(tariffID int64, brand, plate string) dto.CarRequest {
	fuel, lat, lon := 80, 55.75, 37.61
	return dto.CarRequest{TariffID: &tariffID, BrandModel: brand, PlateNumber: plate,
		FuelLevel: &fuel, Latitude: &lat, Longitude: &lon}
}

func (f *fleetEnv) car(tariffID int64, plate string) dto.Car {
	c, err := f.cars.Create(f.ctx, carReq(tariffID, "IT Test Car", plate))
	if err != nil {
		f.t.Fatalf("create car: %v", err)
	}
	return c
}

func TestIntegrationTariffs(t *testing.T) {
	f := newFleetEnv(t)
	tr := f.tariff("IT Тест", 1000, 100)
	if tr.PricePerMin != 1000 || tr.ID == 0 {
		t.Fatalf("Create: %+v", tr)
	}

	p, w := util.Money(1), util.Money(1)
	_, err := f.tariffs.Create(f.ctx, dto.TariffRequest{Name: "IT Тест", PricePerMin: &p, WaitingPerMin: &w})
	f.expectCode("дубликат названия", err, "name_taken")
	_, err = f.tariffs.Update(f.ctx, 999999, dto.TariffRequest{Name: "IT X", PricePerMin: &p, WaitingPerMin: &w})
	f.expectCode("изменение несуществующего", err, "not_found")

	// Снимок цен: изменение тарифа не меняет уже созданную аренду.
	car := f.car(tr.ID, "IT001")
	rental, err := f.svc.Book(f.ctx, f.client(true), dto.CreateRentalRequest{CarID: car.ID})
	if err != nil {
		t.Fatal(err)
	}
	newPrice := util.Money(5000)
	updated, err := f.tariffs.Update(f.ctx, tr.ID, dto.TariffRequest{Name: "IT Тест 2", PricePerMin: &newPrice, WaitingPerMin: &w})
	if err != nil || updated.PricePerMin != 5000 || updated.Name != "IT Тест 2" {
		t.Fatalf("Update: %+v, %v", updated, err)
	}
	after, _ := f.svc.Get(f.ctx, models.User{Role: models.RoleAdmin}, rental.ID)
	if after.PricePerMin != 1000 || after.WaitingPerMin != 100 {
		t.Errorf("цены аренды изменились вслед за тарифом: %+v", after)
	}
	carAfter, _ := f.cars.Get(f.ctx, models.User{Role: models.RoleAdmin}, car.ID)
	if carAfter.Tariff.PricePerMin != 5000 {
		t.Errorf("автомобиль должен показывать новую цену тарифа: %+v", carAfter.Tariff)
	}

	err = f.tariffs.Delete(f.ctx, tr.ID)
	f.expectCode("удаление используемого тарифа", err, "has_related_records")

	unused := f.tariff("IT Пустой", 0, 0)
	if err := f.tariffs.Delete(f.ctx, unused.ID); err != nil {
		t.Errorf("удаление неиспользуемого: %v", err)
	}
	f.expectCode("повторное удаление", f.tariffs.Delete(f.ctx, unused.ID), "not_found")
}

// cyrPlate — "IT002XA" после нормализации: Т, Х, А — кириллица, I остаётся латинской.
const cyrPlate = "I\u0422002\u0425\u0410"

func TestIntegrationCars(t *testing.T) {
	f := newFleetEnv(t)
	tr := f.tariff("IT Авто", 1000, 100)
	admin := models.User{Role: models.RoleAdmin}

	// Номер нормализуется: латиница и пробелы → кириллица без пробелов.
	car := f.car(tr.ID, "it 002 xa")
	if car.Status != models.CarAvailable || car.PlateNumber != cyrPlate || car.Tariff.ID != tr.ID {
		t.Fatalf("Create: %+v", car)
	}
	_, err := f.cars.Create(f.ctx, carReq(tr.ID, "IT Dup", cyrPlate))
	f.expectCode("дубликат номера (кириллица против латиницы)", err, "plate_taken")

	_, err = f.cars.Create(f.ctx, carReq(999999, "IT Bad", "IT003"))
	f.expectCode("несуществующий тариф", err, "validation_error")
	if details(err)["tariff_id"] == "" {
		t.Errorf("ошибка должна указывать на tariff_id: %v", details(err))
	}

	upd := carReq(tr.ID, "IT Test Car Updated", "IT002XA")
	fuel := 15
	upd.FuelLevel = &fuel
	got, err := f.cars.Update(f.ctx, car.ID, upd)
	if err != nil || got.BrandModel != "IT Test Car Updated" || got.FuelLevel != 15 || got.Status != models.CarAvailable {
		t.Fatalf("Update: %+v, %v", got, err)
	}
	_, err = f.cars.Update(f.ctx, 999999, upd)
	f.expectCode("изменение несуществующего", err, "not_found")

	// Ручные статусы по таблице 2.2
	steps := []struct{ to, code string }{
		{models.CarMaintenance, ""},
		{models.CarDecommissioned, ""},
		{models.CarMaintenance, "invalid_status_transition"}, // decommissioned → maintenance нельзя
		{models.CarAvailable, ""},
		{models.CarAvailable, "invalid_status_transition"}, // тот же статус
		{models.CarInUse, "validation_error"},              // вручную не задаётся
	}
	for _, s := range steps {
		_, err := f.cars.ChangeStatus(f.ctx, car.ID, dto.CarStatusRequest{Status: s.to})
		f.expectCode("статус → "+s.to, err, s.code)
	}

	// Видимость для клиента и запрет ручной смены статуса у забронированной машины.
	owner, stranger := f.client(true), f.client(true)
	rental, err := f.svc.Book(f.ctx, owner, dto.CreateRentalRequest{CarID: car.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.cars.Get(f.ctx, owner, car.ID); err != nil {
		t.Errorf("клиент должен видеть машину своей аренды: %v", err)
	}
	_, err = f.cars.Get(f.ctx, stranger, car.ID)
	f.expectCode("чужая забронированная машина для клиента", err, "not_found")
	if _, err := f.cars.Get(f.ctx, admin, car.ID); err != nil {
		t.Errorf("админ видит любую машину: %v", err)
	}
	_, err = f.cars.ChangeStatus(f.ctx, car.ID, dto.CarStatusRequest{Status: models.CarMaintenance})
	f.expectCode("обслуживание забронированной машины", err, "invalid_status_transition")

	// Каталог: клиент видит только свободные, параметр status для него игнорируется.
	booked := models.CarBooked
	list, err := f.cars.List(f.ctx, stranger, models.CarFilter{Status: &booked})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range list.Items {
		if c.Status != models.CarAvailable {
			t.Fatalf("клиент увидел машину в статусе %s", c.Status)
		}
	}
	staffList, _ := f.cars.List(f.ctx, admin, models.CarFilter{Search: "it002", TariffID: &tr.ID})
	if staffList.Total != 1 || staffList.Items[0].ID != car.ID {
		t.Errorf("поиск по номеру латиницей: %+v", staffList)
	}

	// Удаление: машину с историей — нельзя, новую — можно.
	f.svc.Cancel(f.ctx, owner, rental.ID)
	err = f.cars.Delete(f.ctx, car.ID)
	f.expectCode("удаление машины с арендами", err, "has_related_records")
	fresh := f.car(tr.ID, "IT004")
	if err := f.cars.Delete(f.ctx, fresh.ID); err != nil {
		t.Errorf("удаление новой машины: %v", err)
	}
}

// Менеджер переводит машину на обслуживание одновременно с бронированием клиентом.
// Благодаря блокировке строки возможен только один исход: либо аренда и отказ
// менеджеру, либо обслуживание и отказ клиенту. Аренды на машине «на обслуживании» быть не может.
func TestIntegrationStatusVsBookingRace(t *testing.T) {
	f := newFleetEnv(t)
	tr := f.tariff("IT Гонка", 1000, 100)
	for i := range 20 {
		car := f.car(tr.ID, "ITR"+string(rune('A'+i)))
		u := f.client(true)
		var wg sync.WaitGroup
		var bookErr, statusErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, bookErr = f.svc.Book(f.ctx, u, dto.CreateRentalRequest{CarID: car.ID})
		}()
		go func() {
			defer wg.Done()
			_, statusErr = f.cars.ChangeStatus(f.ctx, car.ID, dto.CarStatusRequest{Status: models.CarMaintenance})
		}()
		wg.Wait()

		final := f.carStatus(car.ID)
		switch {
		case bookErr == nil && errCode(statusErr) == "invalid_status_transition" && final == models.CarBooked:
		case statusErr == nil && errCode(bookErr) == "car_unavailable" && final == models.CarMaintenance:
		default:
			t.Fatalf("попытка %d: book=%v status=%v итоговый статус=%s", i, bookErr, statusErr, final)
		}
	}
}
