package services

import (
	"strings"
	"sync"
	"testing"

	"carsharing/internal/dto"
	"carsharing/internal/models"
	"carsharing/internal/repository"
	"carsharing/internal/util"
)

// Интеграционные тесты журнала обслуживания и управления пользователями.
// Запуск — как у тестов аренд (см. rentals_integration_test.go).

func ptr[T any](v T) *T { return &v }

func TestIntegrationMaintenance(t *testing.T) {
	e := newRentalEnv(t)
	svc := NewMaintenanceService(repository.NewMaintenanceRepository(e.pool))
	m1, m2 := e.user(models.RoleManager, false), e.user(models.RoleManager, false)
	admin := e.user(models.RoleAdmin, false)
	carID := e.availableCar("Н999НН777")

	rec, err := svc.Create(e.ctx, m1, dto.CreateMaintenanceRequest{CarID: carID, Description: " Замена щёток ", Cost: ptr(util.Money(150000))})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.Manager.ID != m1.ID || rec.Description != "Замена щёток" || rec.Cost != 150000 || rec.Car.PlateNumber != "Н999НН777" {
		t.Fatalf("неожиданная запись: %+v", rec)
	}
	if e.carStatus(carID) != models.CarAvailable {
		t.Error("запись обслуживания не должна менять статус машины")
	}
	// Админ тоже может добавить запись (наследует права менеджера).
	adminRec, err := svc.Create(e.ctx, admin, dto.CreateMaintenanceRequest{CarID: carID, Description: "Осмотр", Cost: ptr(util.Money(0))})
	if err != nil {
		t.Fatalf("Create админом: %v", err)
	}

	_, err = svc.Create(e.ctx, m1, dto.CreateMaintenanceRequest{CarID: 999999, Description: "x", Cost: ptr(util.Money(1))})
	e.expectCode("несуществующая машина", err, "validation_error")
	if details(err)["car_id"] == "" {
		t.Errorf("ошибка должна указывать на car_id: %v", details(err))
	}
	_, err = svc.Create(e.ctx, m1, dto.CreateMaintenanceRequest{})
	if d := details(err); d["car_id"] == "" || d["description"] == "" || d["cost"] == "" {
		t.Errorf("пустой запрос: %v", d)
	}

	// Частичное изменение: только стоимость, описание сохраняется.
	upd, err := svc.Update(e.ctx, m1, rec.ID, dto.UpdateMaintenanceRequest{Cost: ptr(util.Money(180000))})
	if err != nil || upd.Cost != 180000 || upd.Description != "Замена щёток" {
		t.Fatalf("Update: %+v, %v", upd, err)
	}
	_, err = svc.Update(e.ctx, m1, rec.ID, dto.UpdateMaintenanceRequest{})
	e.expectCode("пустой PATCH", err, "validation_error")
	_, err = svc.Update(e.ctx, m2, rec.ID, dto.UpdateMaintenanceRequest{Description: ptr("чужое")})
	e.expectCode("менеджер правит чужую запись", err, "forbidden")
	e.expectCode("менеджер удаляет чужую запись", svc.Delete(e.ctx, m2, rec.ID), "forbidden")
	_, err = svc.Update(e.ctx, m1, adminRec.ID, dto.UpdateMaintenanceRequest{Description: ptr("x")})
	e.expectCode("менеджер правит запись админа", err, "forbidden")
	if _, err := svc.Update(e.ctx, admin, rec.ID, dto.UpdateMaintenanceRequest{Description: ptr("Исправлено админом")}); err != nil {
		t.Errorf("админ правит любую запись: %v", err)
	}
	_, err = svc.Update(e.ctx, admin, 999999, dto.UpdateMaintenanceRequest{Cost: ptr(util.Money(1))})
	e.expectCode("несуществующая запись", err, "not_found")

	// Фильтр по машине и поиск
	list, err := svc.List(e.ctx, models.MaintenanceFilter{CarID: &carID, Search: "админом"})
	if err != nil || list.Total != 1 || list.Items[0].ID != rec.ID {
		t.Fatalf("List: %+v, %v", list, err)
	}

	if err := svc.Delete(e.ctx, m1, rec.ID); err != nil {
		t.Fatalf("Delete своей записи: %v", err)
	}
	if err := svc.Delete(e.ctx, admin, adminRec.ID); err != nil {
		t.Fatalf("Delete админом: %v", err)
	}
	e.expectCode("повторное удаление", svc.Delete(e.ctx, m1, rec.ID), "not_found")
}

func TestIntegrationUsers(t *testing.T) {
	e := newRentalEnv(t)
	svc := NewUserService(e.users)
	admin := e.user(models.RoleAdmin, false)
	client := e.client(true)
	other := e.client(true)

	_, err := svc.ChangeRole(e.ctx, admin, admin.ID, dto.UserRoleRequest{Role: models.RoleClient})
	e.expectCode("смена своей роли", err, "cannot_change_own_role")
	_, err = svc.ChangeRole(e.ctx, admin, client.ID, dto.UserRoleRequest{Role: "root"})
	e.expectCode("неизвестная роль", err, "validation_error")
	_, err = svc.ChangeRole(e.ctx, admin, 999999, dto.UserRoleRequest{Role: models.RoleManager})
	e.expectCode("несуществующий пользователь", err, "not_found")

	// Пока есть бронирование — ни роль, ни данные не меняются.
	carID := e.availableCar("Е666ЕЕ777")
	rental, err := e.svc.Book(e.ctx, client, dto.CreateRentalRequest{CarID: carID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.ChangeRole(e.ctx, admin, client.ID, dto.UserRoleRequest{Role: models.RoleManager})
	e.expectCode("смена роли при бронировании", err, "user_has_active_rental")
	_, err = svc.Update(e.ctx, client.ID, dto.UpdateUserRequest{FullName: ptr("Новое Имя")})
	e.expectCode("смена данных при бронировании", err, "user_has_active_rental")
	if _, err := e.svc.Cancel(e.ctx, client, rental.ID); err != nil {
		t.Fatal(err)
	}

	// После отмены — можно. ВУ нормализуется.
	u, err := svc.Update(e.ctx, client.ID, dto.UpdateUserRequest{FullName: ptr(" Новое Имя "), DriverLicense: ptr("9799000111")})
	if err != nil || u.FullName != "Новое Имя" || u.DriverLicense == nil || *u.DriverLicense != "9799 000111" || u.Email != client.Email {
		t.Fatalf("Update: %+v, %v", u, err)
	}
	_, err = svc.Update(e.ctx, other.ID, dto.UpdateUserRequest{DriverLicense: ptr("9799 000111")})
	e.expectCode("чужое ВУ", err, "license_taken")
	_, err = svc.Update(e.ctx, other.ID, dto.UpdateUserRequest{Email: ptr(strings.ToUpper(client.Email))})
	e.expectCode("чужой email в другом регистре", err, "email_taken")
	_, err = svc.Update(e.ctx, other.ID, dto.UpdateUserRequest{})
	e.expectCode("пустой PATCH", err, "validation_error")

	m, err := svc.ChangeRole(e.ctx, admin, client.ID, dto.UserRoleRequest{Role: models.RoleManager})
	if err != nil || m.Role != models.RoleManager {
		t.Fatalf("ChangeRole: %+v, %v", m, err)
	}
	// Бывший клиент, ставший менеджером, бронировать уже не может, даже со старым объектом пользователя.
	_, err = e.svc.Book(e.ctx, client, dto.CreateRentalRequest{CarID: carID})
	e.expectCode("бронирование после смены роли", err, "forbidden")

	list, err := svc.List(e.ctx, models.UserFilter{Role: ptr(models.RoleManager), Search: "новое имя"})
	if err != nil || list.Total != 1 || list.Items[0].ID != client.ID {
		t.Fatalf("List: %+v, %v", list, err)
	}

	e.expectCode("удаление себя", svc.Delete(e.ctx, admin, admin.ID), "cannot_delete_self")
	e.expectCode("удаление пользователя с арендами", svc.Delete(e.ctx, admin, client.ID), "has_related_records")
	if err := svc.Delete(e.ctx, admin, other.ID); err != nil {
		t.Errorf("удаление пользователя без истории: %v", err)
	}
	e.expectCode("повторное удаление", svc.Delete(e.ctx, admin, other.ID), "not_found")
}

// Администратор переводит клиента в менеджеры в тот же момент, когда клиент
// бронирует машину. Блокировка строки пользователя допускает только два исхода:
// бронирование прошло и смена роли отклонена — или роль сменилась и бронирование отклонено.
// Менеджера с открытой арендой быть не может.
func TestIntegrationRoleChangeVsBookingRace(t *testing.T) {
	e := newRentalEnv(t)
	svc := NewUserService(e.users)
	admin := e.user(models.RoleAdmin, false)
	carID := e.availableCar("В555ВВ777")

	for i := range 20 {
		u := e.client(true)
		var wg sync.WaitGroup
		var bookErr, roleErr error
		var rental dto.Rental
		wg.Add(2)
		go func() {
			defer wg.Done()
			rental, bookErr = e.svc.Book(e.ctx, u, dto.CreateRentalRequest{CarID: carID})
		}()
		go func() {
			defer wg.Done()
			_, roleErr = svc.ChangeRole(e.ctx, admin, u.ID, dto.UserRoleRequest{Role: models.RoleManager})
		}()
		wg.Wait()

		switch {
		case bookErr == nil && errCode(roleErr) == "user_has_active_rental":
			if _, err := e.svc.Cancel(e.ctx, u, rental.ID); err != nil {
				t.Fatal(err)
			}
		case roleErr == nil && errCode(bookErr) == "forbidden":
		default:
			t.Fatalf("попытка %d: book=%v role=%v", i, bookErr, roleErr)
		}
	}
}
