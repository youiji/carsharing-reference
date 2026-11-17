package e2e

import (
	"fmt"
	"net/url"
	"testing"
)

// Сценарии клиента: профиль, каталог, полный цикл аренды, история,
// защита чужих данных и попытки SQL-инъекций.
func TestClient(t *testing.T) {
	e := newEnv(t)
	dmitry := e.login(dmitryEmail, clientPass)
	ivan := e.login(ivanEmail, clientPass)

	e.run("профиль и смена пароля", func(t *testing.T) {
		var me user
		dmitry.call("GET", "/api/me", nil).ok(200).into(&me)
		if me.ID != dmitryID || me.Role != "client" {
			t.Errorf("me: %+v", me)
		}
		dmitry.call("PUT", "/api/me/password", map[string]string{"current_password": "bad", "new_password": "NewPass456!"}).
			fails(400, "wrong_password")
		dmitry.call("PUT", "/api/me/password", map[string]string{"current_password": clientPass, "new_password": "short"}).
			fails(400, "validation_error")
		dmitry.call("PUT", "/api/me/password", map[string]string{"current_password": clientPass, "new_password": "NewPass456!"}).
			ok(204)
		e.guest().call("POST", "/api/auth/login", map[string]string{"email": dmitryEmail, "password": clientPass}).
			fails(401, "invalid_credentials")
		dmitry = e.login(dmitryEmail, "NewPass456!")
	})

	e.run("каталог", func(t *testing.T) {
		var cars list[car]
		dmitry.call("GET", "/api/cars?status=in_use&sort=price_per_min&order=desc", nil).ok(200).into(&cars)
		if cars.Total != 12 {
			t.Errorf("клиент должен видеть 12 свободных машин, видит %d", cars.Total)
		}
		for i, c := range cars.Items {
			if c.Status != "available" {
				t.Errorf("клиент видит машину в статусе %s", c.Status)
			}
			if i > 0 && money(t, c.Tariff.PricePerMin) > money(t, cars.Items[i-1].Tariff.PricePerMin) {
				t.Errorf("сортировка по цене по убыванию нарушена")
			}
		}
		dmitry.call("GET", "/api/cars?q=a101", nil).ok(200).into(&cars) // латиница
		if cars.Total != 1 || cars.Items[0].ID != freeEconomy {
			t.Errorf("поиск по номеру латиницей: %+v", cars)
		}
		dmitry.call("GET", "/api/cars?sort=status", nil).fails(400, "validation_error")
		dmitry.call("GET", "/api/cars?tariff_id=abc", nil).fails(400, "validation_error")

		var c car
		dmitry.call("GET", fmt.Sprintf("/api/cars/%d", freeEconomy), nil).ok(200).into(&c)
		if c.PlateNumber != "А101АА777" || c.Tariff.Name != "Эконом" || money(t, c.Tariff.PricePerMin) != 890 {
			t.Errorf("карточка: %+v", c)
		}
		// Чужая забронированная машина для клиента «не существует».
		dmitry.call("GET", fmt.Sprintf("/api/cars/%d", mariaCar), nil).fails(404, "not_found")
		e.login(mariaEmail, clientPass).call("GET", fmt.Sprintf("/api/cars/%d", mariaCar), nil).ok(200)
		dmitry.call("GET", "/api/cars/abc", nil).fails(404, "not_found")

		var tariffs list[tariff]
		dmitry.call("GET", "/api/tariffs", nil).ok(200).into(&tariffs)
		if tariffs.Total != 3 {
			t.Errorf("тарифы: %+v", tariffs)
		}
	})

	e.run("полный цикл аренды", func(t *testing.T) {
		var r rental
		dmitry.call("POST", "/api/rentals", map[string]any{"car_id": freeEconomy}).ok(201).into(&r)
		if r.Status != "booked" || r.User.ID != dmitryID || r.StartedAt != nil || r.TotalCost != nil ||
			money(t, r.PricePerMin) != 890 || money(t, r.WaitingPerMin) != 200 {
			t.Fatalf("бронирование: %+v", r)
		}
		path := fmt.Sprintf("/api/rentals/%d", r.ID)

		// Вторая аренда, чужая машина, чужая аренда
		dmitry.call("POST", "/api/rentals", map[string]any{"car_id": freeComfort}).fails(409, "user_has_active_rental")
		e.login("oleg.novikov@example.com", clientPass).
			call("POST", "/api/rentals", map[string]any{"car_id": freeEconomy}).fails(409, "car_unavailable")
		ivan.call("GET", path, nil).fails(403, "forbidden")
		ivan.call("POST", path+"/start", nil).fails(403, "forbidden")
		ivan.call("POST", path+"/cancel", nil).fails(403, "forbidden")

		// Машина своей аренды видна клиенту, но не в каталоге свободных.
		var c car
		dmitry.call("GET", fmt.Sprintf("/api/cars/%d", freeEconomy), nil).ok(200).into(&c)
		if c.Status != "booked" {
			t.Errorf("статус машины после бронирования: %s", c.Status)
		}

		dmitry.call("POST", path+"/finish", nil).fails(409, "invalid_status_transition")
		dmitry.call("POST", path+"/start", nil).ok(200).into(&r)
		if r.Status != "active" || r.StartedAt == nil {
			t.Fatalf("старт: %+v", r)
		}
		dmitry.call("POST", path+"/cancel", nil).fails(409, "invalid_status_transition")
		dmitry.call("POST", path+"/finish", map[string]any{"latitude": 55.76}).fails(400, "validation_error")
		dmitry.call("POST", path+"/finish", map[string]any{"fuel_level": 150}).fails(400, "validation_error")

		dmitry.call("POST", path+"/finish", map[string]any{"latitude": 55.7601, "longitude": 37.6195, "fuel_level": 61}).
			ok(200).into(&r)
		// Поездка короче минуты: 1 минута поездки + 1 минута ожидания = 8.90 + 2.00
		if r.Status != "completed" || r.EndedAt == nil || r.TotalCost == nil || money(t, *r.TotalCost) != 1090 {
			t.Fatalf("завершение: %+v (total_cost %v)", r, r.TotalCost)
		}
		dmitry.call("GET", fmt.Sprintf("/api/cars/%d", freeEconomy), nil).ok(200).into(&c)
		if c.Status != "available" || c.FuelLevel != 61 || c.Latitude.String() != "55.7601" {
			t.Errorf("машина после поездки: %+v", c)
		}

		// Бронирование и отмена
		dmitry.call("POST", "/api/rentals", map[string]any{"car_id": freeComfort}).ok(201).into(&r)
		dmitry.call("POST", fmt.Sprintf("/api/rentals/%d/cancel", r.ID), nil).ok(200).into(&r)
		if r.Status != "cancelled" {
			t.Errorf("отмена: %+v", r)
		}

		var my list[rental]
		dmitry.call("GET", "/api/rentals/my?status=completed&sort=total_cost&order=desc", nil).ok(200).into(&my)
		for _, x := range my.Items {
			if x.User.ID != dmitryID || x.Status != "completed" {
				t.Errorf("чужая или не та аренда в истории: %+v", x)
			}
		}
		if my.Total != 2 { // одна из seed.sql и одна только что
			t.Errorf("завершённых аренд в истории %d, ожидалось 2", my.Total)
		}
		dmitry.call("GET", "/api/rentals/my?status=paused", nil).fails(400, "validation_error")
	})

	e.run("без водительского удостоверения", func(t *testing.T) {
		e.login(noLicense, clientPass).call("POST", "/api/rentals", map[string]any{"car_id": freeEconomy}).
			fails(403, "license_required")
	})

	e.run("SQL-инъекции", func(t *testing.T) {
		var cars list[car]
		for _, q := range []string{"' OR 1=1 --", "%", "_", `\`, "'; DROP TABLE cars; --"} {
			dmitry.call("GET", "/api/cars?q="+url.QueryEscape(q), nil).ok(200).into(&cars)
			if cars.Total != 0 {
				t.Errorf("q=%q нашёл %d машин", q, cars.Total)
			}
		}
		dmitry.call("GET", "/api/cars?sort="+url.QueryEscape("id; DROP TABLE cars"), nil).fails(400, "validation_error")
		dmitry.call("GET", "/api/cars", nil).ok(200).into(&cars)
		if cars.Total == 0 {
			t.Error("таблица cars пострадала")
		}
	})
}
