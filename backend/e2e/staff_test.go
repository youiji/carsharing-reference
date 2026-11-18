package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"testing"
)

// Сценарии менеджера автопарка: автомобили, статусы, тарифы,
// журнал обслуживания, все аренды.
func TestManager(t *testing.T) {
	e := newEnv(t)
	m1 := e.login(manager1, managerPass)
	m2 := e.login(manager2, managerPass)

	e.run("каталог со всеми статусами", func(t *testing.T) {
		var cars list[car]
		m1.call("GET", "/api/cars", nil).ok(200).into(&cars)
		if cars.Total != 18 {
			t.Errorf("менеджер должен видеть все 18 машин, видит %d", cars.Total)
		}
		m1.call("GET", "/api/cars?status=maintenance", nil).ok(200).into(&cars)
		if cars.Total != 1 || cars.Items[0].ID != 7 {
			t.Errorf("фильтр по статусу: %+v", cars)
		}
		m1.call("GET", "/api/cars/4", nil).ok(200) // чужая для клиентов забронированная машина
	})

	var newCar car
	e.run("добавление и изменение автомобиля", func(t *testing.T) {
		body := map[string]any{"tariff_id": 2, "brand_model": "Lada Granta", "plate_number": "t 123 ok 77",
			"fuel_level": 0, "latitude": 55.7, "longitude": 37.6, "status": "in_use"}
		m1.call("POST", "/api/cars", body).ok(201).into(&newCar)
		if newCar.PlateNumber != "Т123ОК77" || newCar.Status != "available" || newCar.Tariff.Name != "Комфорт" {
			t.Errorf("новая машина (номер нормализуется, статус всегда available): %+v", newCar)
		}
		body["plate_number"] = "Т123ОК77"
		m1.call("POST", "/api/cars", body).fails(409, "plate_taken")
		body["plate_number"], body["tariff_id"] = "Т124ОК77", 999
		if d := m1.call("POST", "/api/cars", body).fails(400, "validation_error").details(); d["tariff_id"] == "" {
			t.Errorf("несуществующий тариф: %v", d)
		}
		d := m1.call("POST", "/api/cars", map[string]any{}).fails(400, "validation_error").details()
		if len(d) != 6 {
			t.Errorf("пустой запрос: ожидались ошибки по 6 полям, получено %v", d)
		}

		path := fmt.Sprintf("/api/cars/%d", newCar.ID)
		body["tariff_id"], body["plate_number"], body["fuel_level"] = 3, "Т123ОК77", 90
		m1.call("PUT", path, body).ok(200).into(&newCar)
		if newCar.Tariff.Name != "Бизнес" || newCar.FuelLevel != 90 || newCar.Status != "available" {
			t.Errorf("изменение: %+v", newCar)
		}
		m1.call("PUT", "/api/cars/999999", body).fails(404, "not_found")
	})

	e.run("ручная смена статуса", func(t *testing.T) {
		path := fmt.Sprintf("/api/cars/%d/status", newCar.ID)
		for _, st := range []string{"maintenance", "decommissioned", "available"} {
			var c car
			m1.call("PATCH", path, map[string]string{"status": st}).ok(200).into(&c)
			if c.Status != st {
				t.Errorf("статус %s: %+v", st, c)
			}
		}
		m1.call("PATCH", path, map[string]string{"status": "available"}).fails(409, "invalid_status_transition")
		m1.call("PATCH", path, map[string]string{"status": "booked"}).fails(400, "validation_error")
		m1.call("PATCH", "/api/cars/3/status", map[string]string{"status": "maintenance"}). // машина в поездке
													fails(409, "invalid_status_transition")
	})

	e.run("удаление автомобиля", func(t *testing.T) {
		m1.call("DELETE", "/api/cars/1", nil).fails(409, "has_related_records")
		m1.call("DELETE", fmt.Sprintf("/api/cars/%d", newCar.ID), nil).ok(204)
		m1.call("DELETE", fmt.Sprintf("/api/cars/%d", newCar.ID), nil).fails(404, "not_found")
	})

	e.run("тарифы и снимок цен в аренде", func(t *testing.T) {
		var tr tariff
		m1.call("POST", "/api/tariffs", map[string]any{"name": "Ночной", "price_per_min": 5.5, "waiting_per_min": 1}).
			ok(201).into(&tr)
		if tr.PricePerMin.String() != "5.50" || tr.WaitingPerMin.String() != "1.00" {
			t.Errorf("новый тариф: %+v", tr)
		}
		m1.call("POST", "/api/tariffs", map[string]any{"name": "Ночной", "price_per_min": 1, "waiting_per_min": 1}).
			fails(409, "name_taken")
		m1.call("POST", "/api/tariffs", map[string]any{"name": "X", "price_per_min": 5.555, "waiting_per_min": 1}).
			fails(400, "validation_error")
		m1.call("POST", "/api/tariffs", map[string]any{"name": "X", "price_per_min": "5.50", "waiting_per_min": 1}).
			fails(400, "validation_error")

		// Цена Эконома растёт, а завершённая аренда 1 хранит старую цену.
		m1.call("PUT", "/api/tariffs/1", map[string]any{"name": "Эконом", "price_per_min": 9.9, "waiting_per_min": 2}).ok(200)
		var r rental
		m1.call("GET", "/api/rentals/1", nil).ok(200).into(&r)
		if r.PricePerMin.String() != "8.90" || r.TotalCost == nil || r.TotalCost.String() != "331.50" {
			t.Errorf("аренда должна хранить снимок цен: %+v", r)
		}
		var c car
		m1.call("GET", "/api/cars/1", nil).ok(200).into(&c)
		if c.Tariff.PricePerMin.String() != "9.90" {
			t.Errorf("машина показывает текущую цену тарифа: %+v", c.Tariff)
		}

		m1.call("DELETE", "/api/tariffs/1", nil).fails(409, "has_related_records")
		m1.call("DELETE", fmt.Sprintf("/api/tariffs/%d", tr.ID), nil).ok(204)
		m1.call("PUT", fmt.Sprintf("/api/tariffs/%d", tr.ID), map[string]any{"name": "Y", "price_per_min": 1, "waiting_per_min": 1}).
			fails(404, "not_found")
	})

	e.run("журнал обслуживания", func(t *testing.T) {
		var logs list[maintenance]
		m1.call("GET", "/api/maintenance?car_id=1", nil).ok(200).into(&logs)
		if logs.Total != 1 || logs.Items[0].ID != maintenance1 {
			t.Errorf("записи по машине 1: %+v", logs)
		}
		m1.call("GET", "/api/maintenance?sort=cost&order=desc", nil).ok(200).into(&logs)
		if logs.Total != 8 || logs.Items[0].Cost.String() != "12000.00" {
			t.Errorf("сортировка по стоимости: %+v", logs.Items[0])
		}

		var m maintenance
		m1.call("POST", "/api/maintenance", map[string]any{"car_id": 7, "description": "Замена колодок",
			"cost": 6200, "manager_id": 3}).ok(201).into(&m)
		if m.Manager.ID != 2 || m.Cost.String() != "6200.00" {
			t.Errorf("автор записи берётся из токена, а не из тела: %+v", m)
		}
		if d := m1.call("POST", "/api/maintenance", map[string]any{"car_id": 999999, "description": "x", "cost": 1}).
			fails(400, "validation_error").details(); d["car_id"] == "" {
			t.Errorf("несуществующая машина: %v", d)
		}

		path := fmt.Sprintf("/api/maintenance/%d", m.ID)
		m1.call("PATCH", path, map[string]any{"cost": 11800}).ok(200).into(&m)
		if m.Cost.String() != "11800.00" || m.Description != "Замена колодок" {
			t.Errorf("частичное изменение: %+v", m)
		}
		m1.call("PATCH", path, map[string]any{}).fails(400, "validation_error")
		m2.call("PATCH", path, map[string]any{"cost": 1}).fails(403, "forbidden")
		m2.call("DELETE", path, nil).fails(403, "forbidden")
		m1.call("PATCH", fmt.Sprintf("/api/maintenance/%d", maintenance3), map[string]any{"cost": 1}).fails(403, "forbidden")
		m1.call("DELETE", path, nil).ok(204)
		m1.call("DELETE", path, nil).fails(404, "not_found")
	})

	e.run("все аренды и отмена бронирования клиента", func(t *testing.T) {
		var rs list[rental]
		m1.call("GET", "/api/rentals", nil).ok(200).into(&rs)
		if rs.Total != 14 {
			t.Errorf("всего аренд %d, ожидалось 14", rs.Total)
		}
		m1.call("GET", "/api/rentals?q="+"%D0%9F%D0%B5%D1%82%D1%80%D0%BE%D0%B2&sort=booked_at&order=desc", nil).ok(200).into(&rs)
		if rs.Total != 3 {
			t.Errorf("поиск «Петров»: %d аренд, ожидалось 3", rs.Total)
		}
		m1.call("GET", "/api/rentals?status=booked&car_id=12", nil).ok(200).into(&rs)
		if rs.Total != 1 || rs.Items[0].ID != olegRental {
			t.Errorf("фильтры status и car_id: %+v", rs)
		}

		// Поиск по номеру латиницей: «x333» находит Х333ХХ777.
		m1.call("GET", "/api/rentals?q=x333", nil).ok(200).into(&rs)
		if rs.Total != 1 || rs.Items[0].ID != olegRental {
			t.Errorf("поиск по номеру латиницей: %+v", rs)
		}

		var r rental
		m1.call("POST", fmt.Sprintf("/api/rentals/%d/cancel", olegRental), nil).ok(200).into(&r)
		if r.Status != "cancelled" || r.TotalCost == nil || r.TotalCost.String() != "0.00" {
			t.Errorf("отмена менеджером бесплатна для клиента: %+v", r)
		}
		var c car
		m1.call("GET", "/api/cars/12", nil).ok(200).into(&c)
		if c.Status != "available" {
			t.Errorf("машина после отмены: %s", c.Status)
		}
		m1.call("POST", fmt.Sprintf("/api/rentals/%d/cancel", ivanRental), nil).fails(409, "invalid_status_transition")
	})
}

// Сценарии администратора: пользователи и роли; смена роли и удаление
// действуют сразу, даже для уже выданных токенов.
func TestAdmin(t *testing.T) {
	e := newEnv(t)
	admin := e.login(adminEmail, adminPass)

	e.run("список пользователей", func(t *testing.T) {
		var us list[user]
		admin.call("GET", "/api/users", nil).ok(200).into(&us)
		if us.Total != 9 {
			t.Errorf("пользователей %d, ожидалось 9", us.Total)
		}
		admin.call("GET", "/api/users?role=manager&sort=full_name&order=desc", nil).ok(200).into(&us)
		if us.Total != 2 || us.Items[0].FullName != "Смирнов Алексей Петрович" {
			t.Errorf("фильтр и сортировка: %+v", us)
		}
		admin.call("GET", "/api/users?q=VOLKOVA", nil).ok(200).into(&us)
		if us.Total != 1 || us.Items[0].Email != mariaEmail {
			t.Errorf("поиск по email: %+v", us)
		}
		admin.call("GET", "/api/users?role=root", nil).fails(400, "validation_error")
	})

	e.run("смена роли действует сразу", func(t *testing.T) {
		dmitry := e.login(dmitryEmail, clientPass) // токен выдан, пока он клиент
		dmitry.call("GET", "/api/stats", nil).fails(403, "forbidden")

		var u user
		admin.call("PATCH", fmt.Sprintf("/api/users/%d/role", dmitryID), map[string]string{"role": "manager"}).ok(200).into(&u)
		if u.Role != "manager" {
			t.Fatalf("роль: %+v", u)
		}
		dmitry.call("GET", "/api/stats", nil).ok(200) // старый токен, новые права
		dmitry.call("POST", "/api/rentals", map[string]any{"car_id": freeEconomy}).fails(403, "forbidden")

		admin.call("PATCH", fmt.Sprintf("/api/users/%d/role", dmitryID), map[string]string{"role": "client"}).ok(200)
		dmitry.call("GET", "/api/stats", nil).fails(403, "forbidden")

		admin.call("PATCH", "/api/users/1/role", map[string]string{"role": "client"}).fails(403, "cannot_change_own_role")
		admin.call("PATCH", "/api/users/4/role", map[string]string{"role": "manager"}).fails(409, "user_has_active_rental")
		// Та же роль при открытой аренде — ничего не меняется, ошибки нет.
		admin.call("PATCH", "/api/users/4/role", map[string]string{"role": "client"}).ok(200)
		admin.call("PATCH", "/api/users/4/role", map[string]string{"role": "root"}).fails(400, "validation_error")
		admin.call("PATCH", "/api/users/999999/role", map[string]string{"role": "client"}).fails(404, "not_found")
	})

	e.run("изменение данных пользователя", func(t *testing.T) {
		path := fmt.Sprintf("/api/users/%d", dmitryID)
		admin.call("PATCH", path, map[string]string{"driver_license": "7708112233"}).fails(409, "license_taken")
		admin.call("PATCH", path, map[string]string{"email": "IVAN.PETROV@example.com"}).fails(409, "email_taken")
		admin.call("PATCH", path, map[string]string{}).fails(400, "validation_error")
		admin.call("PATCH", "/api/users/4", map[string]string{"full_name": "Петров И. С."}).fails(409, "user_has_active_rental")

		var u user
		admin.call("PATCH", path, map[string]string{"full_name": "Иванов Дмитрий", "driver_license": "7715000111"}).
			ok(200).into(&u)
		if u.FullName != "Иванов Дмитрий" || *u.DriverLicense != "7715 000111" || u.Email != dmitryEmail {
			t.Errorf("изменение: %+v", u)
		}
		var me user
		e.login(dmitryEmail, clientPass).call("GET", "/api/me", nil).ok(200).into(&me)
		if me.FullName != "Иванов Дмитрий" {
			t.Errorf("пользователь видит новые данные: %+v", me)
		}
	})

	e.run("удаление пользователя", func(t *testing.T) {
		victim := e.login(noLicense, clientPass)
		admin.call("DELETE", "/api/users/1", nil).fails(403, "cannot_delete_self")
		admin.call("DELETE", "/api/users/4", nil).fails(409, "has_related_records")
		admin.call("DELETE", "/api/users/2", nil).fails(409, "has_related_records") // менеджер с записями обслуживания
		admin.call("DELETE", fmt.Sprintf("/api/users/%d", newClientID), nil).ok(204)
		victim.call("GET", "/api/me", nil).fails(401, "unauthorized") // токен удалённого пользователя
		admin.call("DELETE", fmt.Sprintf("/api/users/%d", newClientID), nil).fails(404, "not_found")
	})

	e.run("права менеджера у администратора", func(t *testing.T) {
		admin.call("PATCH", fmt.Sprintf("/api/maintenance/%d", maintenance3), map[string]any{"description": "Проверено"}).ok(200)
		admin.call("DELETE", fmt.Sprintf("/api/maintenance/%d", maintenance3), nil).ok(204)
		var r rental
		admin.call("POST", fmt.Sprintf("/api/rentals/%d/cancel", mariaRental), nil).ok(200).into(&r)
		if r.Status != "cancelled" {
			t.Errorf("отмена админом: %+v", r)
		}
	})
}

// TestStatsMatchesDocs: ответ /api/stats на данных seed.sql совпадает
// с примером в docs/api.md, раздел 11 — документация и код не расходятся.
func TestStatsMatchesDocs(t *testing.T) {
	e := newEnv(t)
	var got map[string]any
	e.login(manager1, managerPass).call("GET", "/api/stats", nil).ok(200).into(&got)

	_, thisFile, _, _ := runtime.Caller(0)
	doc, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "..", "..", "docs", "api.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile("(?s)### `GET /api/stats`.*?```json\n(.*?)```").FindSubmatch(doc)
	if m == nil {
		t.Fatal("в docs/api.md не найден пример ответа GET /api/stats")
	}
	var want map[string]any
	dec := json.NewDecoder(bytes.NewReader(m[1]))
	dec.UseNumber()
	if err := dec.Decode(&want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		t.Errorf("ответ отличается от примера в api.md:\n получено: %s\n в api.md: %s", gotJSON, wantJSON)
	}
}
