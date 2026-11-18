package services

import (
	"context"
	"errors"
	"time"

	"carsharing/internal/dto"
	"carsharing/internal/models"
	"carsharing/internal/repository"
	"carsharing/internal/util"
)

// RentalService — бронирование, поездки и история аренд (docs/api.md, раздел 9).
//
// Каждое действие выполняется в одной транзакции: статус аренды и статус
// автомобиля меняются вместе или не меняются вовсе. Время всех событий
// (booked_at, started_at, ended_at) ставит сервер.
//
// Бронирование действует models.BookingTTL (20 минут). Отмена клиентом или истечение
// срока списывает ожидание; отмена менеджером или администратором бесплатна.
type RentalService struct {
	repo *repository.RentalRepository
	now  func() time.Time
}

func NewRentalService(repo *repository.RentalRepository) *RentalService {
	return &RentalService{repo: repo, now: time.Now}
}

// timestamp — текущее время с точностью до микросекунды (как в PostgreSQL),
// чтобы стоимость, посчитанная в Go, совпадала с сохранёнными датами.
func (s *RentalService) timestamp() time.Time {
	return s.now().Truncate(time.Microsecond)
}

// Book — POST /api/rentals: клиент бронирует свободный автомобиль.
func (s *RentalService) Book(ctx context.Context, user models.User, req dto.CreateRentalRequest) (dto.Rental, error) {
	if req.CarID <= 0 {
		return dto.Rental{}, util.ErrValidation.WithDetails(map[string]string{"car_id": "обязательное поле, положительное целое"})
	}
	// Роль проверена в маршруте, но ВУ может отсутствовать: например, у сотрудника,
	// переведённого в клиенты (docs/role_matrix.md, раздел 4, п. 5).
	if user.DriverLicense == nil || *user.DriverLicense == "" {
		return dto.Rental{}, util.ErrLicenseRequired
	}

	// Сначала снимаются просроченные бронирования (отдельной транзакцией), чтобы
	// машина или сам клиент не числились занятыми до прохода фоновой задачи.
	if _, err := s.ExpireBookings(ctx); err != nil {
		return dto.Rental{}, err
	}

	var rentalID int64
	err := s.repo.InTx(ctx, func(tx *repository.RentalTx) error {
		car, err := tx.LockCar(ctx, req.CarID)
		if errors.Is(err, repository.ErrNotFound) {
			return util.ErrNotFound.WithMessage("Автомобиль не найден")
		}
		if err != nil {
			return err
		}
		if car.Status != models.CarAvailable {
			return util.ErrCarUnavailable
		}

		// Роль и ВУ перечитываются под блокировкой: администратор мог изменить их
		// после того, как Auth загрузил пользователя.
		role, license, err := tx.LockUser(ctx, user.ID)
		if errors.Is(err, repository.ErrNotFound) {
			return util.ErrUnauthorized
		}
		if err != nil {
			return err
		}
		if role != models.RoleClient {
			return util.ErrForbidden
		}
		if license == nil || *license == "" {
			return util.ErrLicenseRequired
		}

		hasOpen, err := tx.UserHasOpenRental(ctx, user.ID)
		if err != nil {
			return err
		}
		if hasOpen {
			return util.ErrUserHasActiveRental
		}

		// Тариф и цены копируются из автомобиля: дальнейшее изменение тарифа
		// не влияет на эту аренду.
		rentalID, err = tx.Insert(ctx, models.Rental{
			UserID:        user.ID,
			TariffID:      car.TariffID,
			CarID:         car.ID,
			BookedAt:      s.timestamp(),
			PricePerMin:   car.PricePerMin,
			WaitingPerMin: car.WaitingPerMin,
		})
		if err != nil {
			return err
		}
		return tx.SetCarStatus(ctx, car.ID, models.CarAvailable, models.CarBooked)
	})
	if err != nil {
		return dto.Rental{}, err
	}
	return s.get(ctx, rentalID)
}

// Start — POST /api/rentals/{id}/start: booked → active, автомобиль → in_use.
// Если срок бронирования истёк, оно отменяется со списанием ожидания и возвращается
// 409 booking_expired (отмена при этом сохраняется).
func (s *RentalService) Start(ctx context.Context, user models.User, rentalID int64) (dto.Rental, error) {
	expired := false
	err := s.repo.InTx(ctx, func(tx *repository.RentalTx) error {
		rental, err := s.lockOwned(ctx, tx, user, rentalID, false)
		if err != nil {
			return err
		}
		if rental.Status != models.RentalBooked {
			return transitionErr(rental.Status, "начать поездку можно только по бронированию")
		}
		if s.isExpired(rental) {
			expired = true
			return s.expire(ctx, tx, rental)
		}
		startedAt := s.timestamp()
		if startedAt.Before(rental.BookedAt) {
			startedAt = rental.BookedAt // защита от расхождения часов
		}
		if err := tx.Start(ctx, rental.ID, startedAt); err != nil {
			return err
		}
		return tx.SetCarStatus(ctx, rental.CarID, models.CarBooked, models.CarInUse)
	})
	if err != nil {
		return dto.Rental{}, err
	}
	if expired {
		return dto.Rental{}, util.ErrBookingExpired
	}
	return s.get(ctx, rentalID)
}

// Finish — POST /api/rentals/{id}/finish: active → completed, расчёт стоимости,
// автомобиль → available на месте парковки. Уровень топлива клиент не задаёт.
func (s *RentalService) Finish(ctx context.Context, user models.User, rentalID int64, req dto.FinishRentalRequest) (dto.Rental, error) {
	if err := validateFinish(req); err != nil {
		return dto.Rental{}, err
	}

	err := s.repo.InTx(ctx, func(tx *repository.RentalTx) error {
		rental, err := s.lockOwned(ctx, tx, user, rentalID, false)
		if err != nil {
			return err
		}
		if rental.Status != models.RentalActive || rental.StartedAt == nil {
			return transitionErr(rental.Status, "завершить можно только начатую поездку")
		}
		endedAt := s.timestamp()
		if endedAt.Before(*rental.StartedAt) {
			endedAt = *rental.StartedAt
		}
		cost := CalcRentalCost(rental.BookedAt, *rental.StartedAt, endedAt, rental.PricePerMin, rental.WaitingPerMin)
		if err := tx.Complete(ctx, rental.ID, endedAt, cost); err != nil {
			return err
		}
		return tx.ReleaseCar(ctx, rental.CarID, models.CarLocation{
			Latitude: req.Latitude, Longitude: req.Longitude,
		})
	})
	if err != nil {
		return dto.Rental{}, err
	}
	return s.get(ctx, rentalID)
}

// Cancel — POST /api/rentals/{id}/cancel: booked → cancelled, автомобиль → available.
// Клиент отменяет только своё бронирование, менеджер и админ — любое.
// Клиенту списывается ожидание с момента бронирования; отмена сотрудником бесплатна.
// Если срок бронирования уже истёк, оно отменяется как просроченное.
func (s *RentalService) Cancel(ctx context.Context, user models.User, rentalID int64) (dto.Rental, error) {
	err := s.repo.InTx(ctx, func(tx *repository.RentalTx) error {
		rental, err := s.lockOwned(ctx, tx, user, rentalID, isStaff(user))
		if err != nil {
			return err
		}
		if rental.Status != models.RentalBooked {
			return transitionErr(rental.Status, "отменить можно только бронирование")
		}
		if s.isExpired(rental) {
			return s.expire(ctx, tx, rental)
		}
		cancelledAt := s.timestamp()
		if cancelledAt.Before(rental.BookedAt) {
			cancelledAt = rental.BookedAt
		}
		fee := util.Money(0)
		if !isStaff(user) {
			fee = CancellationFee(rental.BookedAt, cancelledAt, rental.WaitingPerMin)
		}
		if err := tx.Cancel(ctx, rental.ID, cancelledAt, fee); err != nil {
			return err
		}
		return tx.SetCarStatus(ctx, rental.CarID, models.CarBooked, models.CarAvailable)
	})
	if err != nil {
		return dto.Rental{}, err
	}
	return s.get(ctx, rentalID)
}

// ExpireBookings отменяет все бронирования, срок которых истёк, со списанием
// ожидания за весь срок. Вызывается фоновой задачей сервера и перед бронированием.
// Возвращает число отменённых бронирований.
func (s *RentalService) ExpireBookings(ctx context.Context) (int, error) {
	n := 0
	err := s.repo.InTx(ctx, func(tx *repository.RentalTx) error {
		expired, err := tx.LockExpiredBookings(ctx, s.timestamp().Add(-models.BookingTTL))
		if err != nil {
			return err
		}
		for _, r := range expired {
			if err := s.expire(ctx, tx, r); err != nil {
				return err
			}
		}
		n = len(expired)
		return nil
	})
	return n, err
}

// isExpired — истёк ли срок бронирования к текущему моменту.
func (s *RentalService) isExpired(r models.Rental) bool {
	return !s.timestamp().Before(r.BookedAt.Add(models.BookingTTL))
}

// expire отменяет просроченное бронирование: момент отмены — конец срока,
// списывается ожидание за весь срок, автомобиль снова свободен.
func (s *RentalService) expire(ctx context.Context, tx *repository.RentalTx, r models.Rental) error {
	end := r.BookedAt.Add(models.BookingTTL)
	if err := tx.Cancel(ctx, r.ID, end, CancellationFee(r.BookedAt, end, r.WaitingPerMin)); err != nil {
		return err
	}
	return tx.SetCarStatus(ctx, r.CarID, models.CarBooked, models.CarAvailable)
}

// Get — GET /api/rentals/{id}: клиент видит только свои аренды, менеджер и админ — любые.
func (s *RentalService) Get(ctx context.Context, user models.User, rentalID int64) (dto.Rental, error) {
	rental, err := s.repo.GetByID(ctx, rentalID)
	if errors.Is(err, repository.ErrNotFound) {
		return dto.Rental{}, util.ErrNotFound.WithMessage("Аренда не найдена")
	}
	if err != nil {
		return dto.Rental{}, err
	}
	if !isStaff(user) && rental.UserID != user.ID {
		return dto.Rental{}, util.ErrForbidden.WithMessage("Это чужая аренда")
	}
	return dto.NewRental(rental), nil
}

// ListMy — GET /api/rentals/my: аренды текущего пользователя.
func (s *RentalService) ListMy(ctx context.Context, user models.User, f models.RentalFilter) (dto.List[dto.Rental], error) {
	f.UserID = &user.ID
	f.CarID, f.Search = nil, ""
	return s.list(ctx, f)
}

// List — GET /api/rentals: все аренды (менеджер, админ).
func (s *RentalService) List(ctx context.Context, f models.RentalFilter) (dto.List[dto.Rental], error) {
	return s.list(ctx, f)
}

func (s *RentalService) list(ctx context.Context, f models.RentalFilter) (dto.List[dto.Rental], error) {
	rentals, err := s.repo.List(ctx, f)
	if err != nil {
		return dto.List[dto.Rental]{}, err
	}
	return dto.NewList(dto.NewRentals(rentals)), nil
}

func (s *RentalService) get(ctx context.Context, id int64) (dto.Rental, error) {
	rental, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return dto.Rental{}, err
	}
	return dto.NewRental(rental), nil
}

// lockOwned блокирует аренду и проверяет, что она принадлежит пользователю
// (если anyOwner = false). Порядок ответов: 404 → 403 → проверка статуса.
func (s *RentalService) lockOwned(ctx context.Context, tx *repository.RentalTx, user models.User, id int64, anyOwner bool) (models.Rental, error) {
	rental, err := tx.LockRental(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return models.Rental{}, util.ErrNotFound.WithMessage("Аренда не найдена")
	}
	if err != nil {
		return models.Rental{}, err
	}
	if !anyOwner && rental.UserID != user.ID {
		return models.Rental{}, util.ErrForbidden.WithMessage("Это чужая аренда")
	}
	return rental, nil
}

func isStaff(u models.User) bool {
	return u.Role == models.RoleManager || u.Role == models.RoleAdmin
}

var rentalStatusNames = map[string]string{
	models.RentalBooked:    "забронирована",
	models.RentalActive:    "в поездке",
	models.RentalCompleted: "завершена",
	models.RentalCancelled: "отменена",
}

func transitionErr(status, hint string) error {
	return util.ErrInvalidTransition.WithMessage("Аренда " + rentalStatusNames[status] + ": " + hint)
}

func validateFinish(req dto.FinishRentalRequest) error {
	var v util.Validator
	if (req.Latitude == nil) != (req.Longitude == nil) {
		v.Add("latitude", "latitude и longitude передаются только вместе")
	}
	if req.Latitude != nil {
		v.Check(*req.Latitude >= -90 && *req.Latitude <= 90, "latitude", "должно быть от -90 до 90")
	}
	if req.Longitude != nil {
		v.Check(*req.Longitude >= -180 && *req.Longitude <= 180, "longitude", "должно быть от -180 до 180")
	}
	return v.Err()
}

// CalcRentalCost считает стоимость аренды (docs/api.md, раздел 2.3):
//
//	минуты_поездки  = max(1, ⌈ended_at − started_at⌉ в минутах)
//	минуты_ожидания = ⌈started_at − booked_at⌉ в минутах
//	стоимость       = цена_минуты × минуты_поездки + цена_ожидания × минуты_ожидания
//
// Цены в копейках, минуты целые, поэтому результат точный и округление не нужно.
func CalcRentalCost(bookedAt, startedAt, endedAt time.Time, pricePerMin, waitingPerMin util.Money) util.Money {
	rideMin := max(1, ceilMinutes(endedAt.Sub(startedAt)))
	waitMin := ceilMinutes(startedAt.Sub(bookedAt))
	return pricePerMin*util.Money(rideMin) + waitingPerMin*util.Money(waitMin)
}

// CancellationFee — списание при отмене бронирования (docs/api.md, раздел 2.3):
// цена_ожидания × ⌈отмена − бронирование⌉ в минутах, но не больше срока бронирования.
func CancellationFee(bookedAt, cancelledAt time.Time, waitingPerMin util.Money) util.Money {
	minutes := min(ceilMinutes(cancelledAt.Sub(bookedAt)), int64(models.BookingTTL/time.Minute))
	return waitingPerMin * util.Money(minutes)
}

// ceilMinutes округляет длительность вверх до целых минут (отрицательная → 0).
func ceilMinutes(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	return int64((d + time.Minute - 1) / time.Minute)
}
