package services

import (
	"context"
	"errors"
	"slices"
	"strings"

	"carsharing/internal/dto"
	"carsharing/internal/models"
	"carsharing/internal/repository"
	"carsharing/internal/util"
)

// CarService — каталог и управление автопарком (docs/api.md, разделы 6–7).
type CarService struct {
	cars *repository.CarRepository
}

func NewCarService(cars *repository.CarRepository) *CarService {
	return &CarService{cars: cars}
}

// manualCarTransitions — переходы статуса, которые менеджер и админ могут
// выполнить вручную (docs/api.md, таблица 2.2). Статусы booked и in_use
// меняются только вместе с арендой.
var manualCarTransitions = map[string][]string{
	models.CarAvailable:      {models.CarMaintenance, models.CarDecommissioned},
	models.CarMaintenance:    {models.CarAvailable, models.CarDecommissioned},
	models.CarDecommissioned: {models.CarAvailable},
}

// ManualCarStatuses — значения, допустимые в PATCH /api/cars/{id}/status.
var ManualCarStatuses = []string{models.CarAvailable, models.CarMaintenance, models.CarDecommissioned}

var carStatusNames = map[string]string{
	models.CarAvailable:      "свободен",
	models.CarBooked:         "забронирован",
	models.CarInUse:          "в поездке",
	models.CarMaintenance:    "на обслуживании",
	models.CarDecommissioned: "списан",
}

var errCarNotFound = util.ErrNotFound.WithMessage("Автомобиль не найден")

// List — GET /api/cars. Клиент видит только свободные автомобили.
func (s *CarService) List(ctx context.Context, user models.User, f models.CarFilter) (dto.List[dto.Car], error) {
	if user.Role == models.RoleClient {
		available := models.CarAvailable
		f.Status = &available
	}
	cars, err := s.cars.List(ctx, f)
	if err != nil {
		return dto.List[dto.Car]{}, err
	}
	return dto.NewList(dto.NewCars(cars)), nil
}

// Get — GET /api/cars/{id}. Клиент получает автомобиль, только если он свободен
// или входит в его текущую аренду; иначе 404, чтобы не раскрывать чужие данные.
func (s *CarService) Get(ctx context.Context, user models.User, id int64) (dto.Car, error) {
	car, err := s.cars.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return dto.Car{}, errCarNotFound
	}
	if err != nil {
		return dto.Car{}, err
	}
	if user.Role == models.RoleClient && car.Status != models.CarAvailable {
		own, err := s.cars.InUserOpenRental(ctx, id, user.ID)
		if err != nil {
			return dto.Car{}, err
		}
		if !own {
			return dto.Car{}, errCarNotFound
		}
	}
	return dto.NewCar(car), nil
}

// Create — POST /api/cars. Начальный статус всегда available.
func (s *CarService) Create(ctx context.Context, req dto.CarRequest) (dto.Car, error) {
	car, err := validateCar(req)
	if err != nil {
		return dto.Car{}, err
	}
	id, err := s.cars.Create(ctx, car)
	if err != nil {
		return dto.Car{}, err
	}
	return s.get(ctx, id)
}

// Update — PUT /api/cars/{id}: полная замена полей, статус не меняется.
func (s *CarService) Update(ctx context.Context, id int64, req dto.CarRequest) (dto.Car, error) {
	car, err := validateCar(req)
	if err != nil {
		return dto.Car{}, err
	}
	car.ID = id
	if err := s.cars.Update(ctx, car); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return dto.Car{}, errCarNotFound
		}
		return dto.Car{}, err
	}
	return s.get(ctx, id)
}

// ChangeStatus — PATCH /api/cars/{id}/status по таблице переходов 2.2.
func (s *CarService) ChangeStatus(ctx context.Context, id int64, req dto.CarStatusRequest) (dto.Car, error) {
	to := strings.TrimSpace(req.Status)
	if !slices.Contains(ManualCarStatuses, to) {
		return dto.Car{}, util.ErrValidation.WithDetails(map[string]string{
			"status": "допустимые значения: " + strings.Join(ManualCarStatuses, ", "),
		})
	}
	err := s.cars.ChangeStatus(ctx, id, to, func(from string) error {
		if from == models.CarBooked || from == models.CarInUse {
			return util.ErrInvalidTransition.WithMessage(
				"Автомобиль " + carStatusNames[from] + ": статус меняется только через аренду")
		}
		if !slices.Contains(manualCarTransitions[from], to) {
			return util.ErrInvalidTransition.WithMessage(
				"Нельзя перевести автомобиль из статуса «" + carStatusNames[from] + "» в «" + carStatusNames[to] + "»")
		}
		return nil
	})
	if errors.Is(err, repository.ErrNotFound) {
		return dto.Car{}, errCarNotFound
	}
	if err != nil {
		return dto.Car{}, err
	}
	return s.get(ctx, id)
}

// Delete — DELETE /api/cars/{id}. Автомобиль с историей не удаляется, а списывается.
func (s *CarService) Delete(ctx context.Context, id int64) error {
	err := s.cars.Delete(ctx, id)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return errCarNotFound
	case errors.Is(err, util.ErrHasRelatedRecords):
		return util.ErrHasRelatedRecords.WithMessage(
			"Нельзя удалить автомобиль: у него есть аренды или записи обслуживания. Переведите его в статус «списан»")
	}
	return err
}

func (s *CarService) get(ctx context.Context, id int64) (dto.Car, error) {
	car, err := s.cars.GetByID(ctx, id)
	if err != nil {
		return dto.Car{}, err
	}
	return dto.NewCar(car), nil
}

// validateCar проверяет поля по docs/api.md, раздел 12, и нормализует гос. номер.
func validateCar(req dto.CarRequest) (models.Car, error) {
	var v util.Validator
	brand := strings.TrimSpace(req.BrandModel)
	plate := util.NormalizePlate(req.PlateNumber)

	switch {
	case req.TariffID == nil:
		v.Add("tariff_id", "обязательное поле")
	case *req.TariffID <= 0:
		v.Add("tariff_id", "должно быть положительным целым числом")
	}
	v.StringLen("brand_model", brand, 1, 100)
	v.StringLen("plate_number", plate, 1, 15)
	switch {
	case req.FuelLevel == nil:
		v.Add("fuel_level", "обязательное поле")
	case *req.FuelLevel < 0 || *req.FuelLevel > 100:
		v.Add("fuel_level", "должно быть от 0 до 100")
	}
	switch {
	case req.Latitude == nil:
		v.Add("latitude", "обязательное поле")
	case *req.Latitude < -90 || *req.Latitude > 90:
		v.Add("latitude", "должно быть от -90 до 90")
	}
	switch {
	case req.Longitude == nil:
		v.Add("longitude", "обязательное поле")
	case *req.Longitude < -180 || *req.Longitude > 180:
		v.Add("longitude", "должно быть от -180 до 180")
	}
	if err := v.Err(); err != nil {
		return models.Car{}, err
	}
	return models.Car{
		TariffID:    *req.TariffID,
		BrandModel:  brand,
		PlateNumber: plate,
		FuelLevel:   *req.FuelLevel,
		Latitude:    *req.Latitude,
		Longitude:   *req.Longitude,
	}, nil
}
