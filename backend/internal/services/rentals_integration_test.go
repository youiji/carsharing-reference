package services

// Интеграционные тесты аренд на реальной PostgreSQL.
// Запуск (на отдельной базе со схемой и тестовыми данными, данные seed не портятся —
// созданные тестом записи удаляются в конце):
//
//	TEST_DATABASE_URL=postgres://...  go test ./internal/services -run Integration -v
//
// Без TEST_DATABASE_URL тесты пропускаются.

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"carsharing/internal/dto"
	"carsharing/internal/models"
	"carsharing/internal/repository"
)

type rentalEnv struct {
	t     *testing.T
	ctx   context.Context
	pool  *pgxpool.Pool
	svc   *RentalService
	users *repository.UserRepository
	clock time.Time
	seq   int
}

func newRentalEnv(t *testing.T) *rentalEnv {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL не задана")
	}
	ctx := context.Background()
	pool, err := repository.NewPool(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	e := &rentalEnv{t: t, ctx: ctx, pool: pool,
		svc:   NewRentalService(repository.NewRentalRepository(pool)),
		users: repository.NewUserRepository(pool),
		clock: time.Now().Truncate(time.Second),
	}
	e.svc.now = func() time.Time { return e.clock }

	t.Cleanup(func() {
		// Удаляем всё, что создал тест, и возвращаем машины в исходное состояние.
		_, err := pool.Exec(ctx, `
			DELETE FROM rentals WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'it-%@test.local');
			DELETE FROM maintenance_logs WHERE manager_id IN (SELECT id FROM users WHERE email LIKE 'it-%@test.local');
			DELETE FROM users WHERE email LIKE 'it-%@test.local';`)
		if err != nil {
			t.Errorf("cleanup: %v", err)
		}
		pool.Close()
	})
	return e
}

// client создаёт клиента с ВУ (или без, если withLicense = false).
func (e *rentalEnv) client(withLicense bool) models.User {
	return e.user(models.RoleClient, withLicense)
}

// user создаёт пользователя с указанной ролью.
func (e *rentalEnv) user(role string, withLicense bool) models.User {
	e.seq++
	var license *string
	if withLicense {
		s := fmt.Sprintf("98%02d %06d", e.seq%100, time.Now().UnixNano()%1_000_000)
		license = &s
	}
	u, err := e.users.Create(e.ctx, models.User{
		Email:         fmt.Sprintf("it-%d-%d@test.local", e.seq, time.Now().UnixNano()),
		PasswordHash:  "x",
		FullName:      "Интеграционный Тест",
		DriverLicense: license,
		Role:          role,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return u
}

// availableCar находит свободный автомобиль по гос. номеру из seed.sql и
// запоминает его состояние для восстановления после теста.
func (e *rentalEnv) availableCar(plate string) int64 {
	var (
		id       int64
		status   string
		lat, lon float64
		fuel     int
	)
	err := e.pool.QueryRow(e.ctx, `SELECT id, status, latitude, longitude, fuel_level FROM cars WHERE plate_number = $1`, plate).
		Scan(&id, &status, &lat, &lon, &fuel)
	if err != nil {
		e.t.Fatalf("car %s: %v", plate, err)
	}
	if status != models.CarAvailable {
		e.t.Fatalf("car %s: ожидался статус available, в БД %s (перезагрузите seed.sql)", plate, status)
	}
	e.t.Cleanup(func() {
		e.pool.Exec(context.Background(),
			`UPDATE cars SET status = $2, latitude = $3, longitude = $4, fuel_level = $5 WHERE id = $1`,
			id, status, lat, lon, fuel)
	})
	return id
}

func (e *rentalEnv) carStatus(id int64) string {
	var s string
	if err := e.pool.QueryRow(e.ctx, `SELECT status FROM cars WHERE id = $1`, id).Scan(&s); err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *rentalEnv) expectCode(step string, err error, code string) {
	e.t.Helper()
	if got := errCode(err); got != code {
		e.t.Fatalf("%s: ошибка %q, ожидалась %q", step, got, code)
	}
}

func TestIntegrationRentalLifecycle(t *testing.T) {
	e := newRentalEnv(t)
	carID := e.availableCar("Н999НН777")    // Genesis G80, тариф Бизнес: 19.90 / 5.00
	otherCar := e.availableCar("М888ММ777") // Audi A6
	ivan, anna := e.client(true), e.client(true)
	manager := models.User{ID: 2, Role: models.RoleManager}

	// Без водительского удостоверения бронировать нельзя.
	_, err := e.svc.Book(e.ctx, e.client(false), dto.CreateRentalRequest{CarID: carID})
	e.expectCode("бронирование без ВУ", err, "license_required")

	_, err = e.svc.Book(e.ctx, ivan, dto.CreateRentalRequest{CarID: 999999})
	e.expectCode("несуществующая машина", err, "not_found")

	// Бронирование
	rental, err := e.svc.Book(e.ctx, ivan, dto.CreateRentalRequest{CarID: carID})
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	if rental.Status != models.RentalBooked || rental.PricePerMin != 1990 || rental.WaitingPerMin != 500 ||
		rental.StartedAt != nil || rental.TotalCost != nil || rental.User.ID != ivan.ID {
		t.Fatalf("неожиданная аренда: %+v", rental)
	}
	if s := e.carStatus(carID); s != models.CarBooked {
		t.Fatalf("статус машины после бронирования %s", s)
	}

	_, err = e.svc.Book(e.ctx, anna, dto.CreateRentalRequest{CarID: carID})
	e.expectCode("повторное бронирование машины", err, "car_unavailable")
	_, err = e.svc.Book(e.ctx, ivan, dto.CreateRentalRequest{CarID: otherCar})
	e.expectCode("вторая аренда клиента", err, "user_has_active_rental")

	_, err = e.svc.Start(e.ctx, anna, rental.ID)
	e.expectCode("старт чужой аренды", err, "forbidden")
	_, err = e.svc.Get(e.ctx, anna, rental.ID)
	e.expectCode("просмотр чужой аренды", err, "forbidden")
	if _, err = e.svc.Get(e.ctx, manager, rental.ID); err != nil {
		t.Fatalf("менеджер должен видеть любую аренду: %v", err)
	}
	_, err = e.svc.Finish(e.ctx, ivan, rental.ID, dto.FinishRentalRequest{})
	e.expectCode("завершение до старта", err, "invalid_status_transition")

	// Старт через 5 мин 30 с → 6 минут ожидания.
	e.clock = e.clock.Add(5*time.Minute + 30*time.Second)
	rental, err = e.svc.Start(e.ctx, ivan, rental.ID)
	if err != nil || rental.Status != models.RentalActive || rental.StartedAt == nil {
		t.Fatalf("Start: %+v, %v", rental, err)
	}
	if s := e.carStatus(carID); s != models.CarInUse {
		t.Fatalf("статус машины после старта %s", s)
	}
	_, err = e.svc.Cancel(e.ctx, ivan, rental.ID)
	e.expectCode("отмена начатой поездки", err, "invalid_status_transition")
	_, err = e.svc.Start(e.ctx, ivan, rental.ID)
	e.expectCode("повторный старт", err, "invalid_status_transition")

	// Невалидные данные парковки не меняют ничего.
	lat, lon, fuel := 55.76, 37.62, 61
	_, err = e.svc.Finish(e.ctx, ivan, rental.ID, dto.FinishRentalRequest{Latitude: &lat})
	e.expectCode("только одна координата", err, "validation_error")

	// Финиш через 34 мин 40 с после старта → 35 минут поездки.
	// 19.90 × 35 + 5.00 × 6 = 696.50 + 30.00 = 726.50
	e.clock = e.clock.Add(34*time.Minute + 40*time.Second)
	rental, err = e.svc.Finish(e.ctx, ivan, rental.ID, dto.FinishRentalRequest{Latitude: &lat, Longitude: &lon, FuelLevel: &fuel})
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if rental.Status != models.RentalCompleted || rental.TotalCost == nil || *rental.TotalCost != 72650 {
		t.Fatalf("неожиданный итог: status=%s cost=%v", rental.Status, rental.TotalCost)
	}
	var gotLat, gotLon float64
	var gotFuel int
	var status string
	e.pool.QueryRow(e.ctx, `SELECT status, latitude, longitude, fuel_level FROM cars WHERE id = $1`, carID).
		Scan(&status, &gotLat, &gotLon, &gotFuel)
	if status != models.CarAvailable || gotLat != lat || gotLon != lon || gotFuel != fuel {
		t.Fatalf("машина после поездки: %s %v %v %d", status, gotLat, gotLon, gotFuel)
	}

	// Бронирование и отмена менеджером
	booked, err := e.svc.Book(e.ctx, anna, dto.CreateRentalRequest{CarID: carID})
	if err != nil {
		t.Fatalf("Book после завершения: %v", err)
	}
	cancelled, err := e.svc.Cancel(e.ctx, manager, booked.ID)
	if err != nil || cancelled.Status != models.RentalCancelled {
		t.Fatalf("Cancel менеджером: %+v, %v", cancelled, err)
	}
	if s := e.carStatus(carID); s != models.CarAvailable {
		t.Fatalf("статус машины после отмены %s", s)
	}

	// История клиента
	my, err := e.svc.ListMy(e.ctx, ivan, models.RentalFilter{Sort: "id"})
	if err != nil || my.Total != 1 || my.Items[0].ID != rental.ID {
		t.Fatalf("ListMy: %+v, %v", my, err)
	}
	completed := models.RentalCompleted
	list, err := e.svc.List(e.ctx, models.RentalFilter{Status: &completed, Search: "интеграционный", Sort: "total_cost", Desc: true})
	if err != nil || list.Total != 1 {
		t.Fatalf("List с поиском и фильтром: %+v, %v", list, err)
	}
}

// Главный риск предметной области — выдача одной машины двум клиентам.
// Десять клиентов одновременно бронируют одну машину: успешно ровно одно бронирование.
func TestIntegrationConcurrentBooking(t *testing.T) {
	e := newRentalEnv(t)
	carID := e.availableCar("К777КК777")
	const n = 10
	clients := make([]models.User, n)
	for i := range clients {
		clients[i] = e.client(true)
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		success int
		codes   = map[string]int{}
	)
	for _, c := range clients {
		wg.Add(1)
		go func(u models.User) {
			defer wg.Done()
			_, err := e.svc.Book(e.ctx, u, dto.CreateRentalRequest{CarID: carID})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success++
			} else {
				codes[errCode(err)]++
			}
		}(c)
	}
	wg.Wait()

	if success != 1 || codes["car_unavailable"] != n-1 {
		t.Fatalf("успешных бронирований %d (ожидалось 1), ошибки: %v", success, codes)
	}
	var open int
	e.pool.QueryRow(e.ctx, `SELECT count(*) FROM rentals WHERE car_id = $1 AND status IN ('booked','active')`, carID).Scan(&open)
	if open != 1 {
		t.Fatalf("в БД %d открытых аренд на машину", open)
	}
	// Возвращаем машину: отменяем бронирование.
	var rentalID int64
	e.pool.QueryRow(e.ctx, `SELECT id FROM rentals WHERE car_id = $1 AND status = 'booked'`, carID).Scan(&rentalID)
	if _, err := e.svc.Cancel(e.ctx, models.User{Role: models.RoleAdmin}, rentalID); err != nil {
		t.Fatal(err)
	}
}

// Один клиент отправляет два бронирования разных машин одновременно —
// создаётся только одно (частичный уникальный индекс по user_id).
func TestIntegrationConcurrentSameUser(t *testing.T) {
	e := newRentalEnv(t)
	cars := []int64{e.availableCar("С909СС777"), e.availableCar("У222УУ777")}
	u := e.client(true)

	var wg sync.WaitGroup
	errs := make([]error, len(cars))
	for i, carID := range cars {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.svc.Book(e.ctx, u, dto.CreateRentalRequest{CarID: carID})
		}()
	}
	wg.Wait()

	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if errCode(err) != "user_has_active_rental" {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("успешных бронирований %d, ожидалось 1", ok)
	}
	// Машина, которую не удалось забронировать, должна остаться available.
	free := 0
	for _, id := range cars {
		if e.carStatus(id) == models.CarAvailable {
			free++
		}
	}
	if free != 1 {
		t.Fatalf("свободных машин %d, ожидалась 1", free)
	}
	var rentalID int64
	e.pool.QueryRow(e.ctx, `SELECT id FROM rentals WHERE user_id = $1 AND status = 'booked'`, u.ID).Scan(&rentalID)
	if _, err := e.svc.Cancel(e.ctx, u, rentalID); err != nil {
		t.Fatal(err)
	}
}
