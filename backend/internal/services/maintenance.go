package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"carsharing/internal/dto"
	"carsharing/internal/models"
	"carsharing/internal/repository"
	"carsharing/internal/util"
)

// MaintenanceService — журнал технического обслуживания (docs/api.md, раздел 8).
type MaintenanceService struct {
	logs *repository.MaintenanceRepository
	now  func() time.Time
}

func NewMaintenanceService(logs *repository.MaintenanceRepository) *MaintenanceService {
	return &MaintenanceService{logs: logs, now: time.Now}
}

// maxMaintenanceCost — верхняя граница NUMERIC(10,2): 99999999.99.
const maxMaintenanceCost = util.Money(9_999_999_999)

var errMaintenanceNotFound = util.ErrNotFound.WithMessage("Запись обслуживания не найдена")

// List — GET /api/maintenance.
func (s *MaintenanceService) List(ctx context.Context, f models.MaintenanceFilter) (dto.List[dto.Maintenance], error) {
	logs, err := s.logs.List(ctx, f)
	if err != nil {
		return dto.List[dto.Maintenance]{}, err
	}
	return dto.NewList(dto.NewMaintenanceList(logs)), nil
}

// Create — POST /api/maintenance. Автор — текущий пользователь (менеджер или админ,
// проверено в маршруте), время ставит сервер. Статус автомобиля не меняется.
func (s *MaintenanceService) Create(ctx context.Context, author models.User, req dto.CreateMaintenanceRequest) (dto.Maintenance, error) {
	var v util.Validator
	v.Check(req.CarID > 0, "car_id", "обязательное поле, положительное целое")
	description := strings.TrimSpace(req.Description)
	v.StringLen("description", description, 1, 255)
	if req.Cost == nil {
		v.Add("cost", "обязательное поле")
	} else {
		checkCost(&v, *req.Cost)
	}
	if err := v.Err(); err != nil {
		return dto.Maintenance{}, err
	}

	id, err := s.logs.Create(ctx, models.MaintenanceLog{
		CarID:       req.CarID,
		ManagerID:   author.ID,
		Description: description,
		Cost:        *req.Cost,
		LoggedAt:    s.now().Truncate(time.Microsecond),
	})
	if err != nil {
		return dto.Maintenance{}, err
	}
	return s.get(ctx, id)
}

// Update — PATCH /api/maintenance/{id}: менеджер меняет только свои записи, админ — любые.
func (s *MaintenanceService) Update(ctx context.Context, user models.User, id int64, req dto.UpdateMaintenanceRequest) (dto.Maintenance, error) {
	var v util.Validator
	if req.Description == nil && req.Cost == nil {
		v.Add("description", "передайте description и/или cost")
	}
	var description *string
	if req.Description != nil {
		d := strings.TrimSpace(*req.Description)
		v.StringLen("description", d, 1, 255)
		description = &d
	}
	if req.Cost != nil {
		checkCost(&v, *req.Cost)
	}
	if err := v.Err(); err != nil {
		return dto.Maintenance{}, err
	}

	if err := s.checkOwner(ctx, user, id); err != nil {
		return dto.Maintenance{}, err
	}
	if err := s.logs.Update(ctx, id, description, req.Cost); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return dto.Maintenance{}, errMaintenanceNotFound
		}
		return dto.Maintenance{}, err
	}
	return s.get(ctx, id)
}

// Delete — DELETE /api/maintenance/{id}: менеджер удаляет только свои записи, админ — любые.
func (s *MaintenanceService) Delete(ctx context.Context, user models.User, id int64) error {
	if err := s.checkOwner(ctx, user, id); err != nil {
		return err
	}
	err := s.logs.Delete(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return errMaintenanceNotFound
	}
	return err
}

// checkOwner: 404, если записи нет; 403, если менеджер пытается изменить чужую.
// Автор записи не меняется, поэтому проверку можно делать до изменения.
func (s *MaintenanceService) checkOwner(ctx context.Context, user models.User, id int64) error {
	m, err := s.logs.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return errMaintenanceNotFound
	}
	if err != nil {
		return err
	}
	if user.Role != models.RoleAdmin && m.ManagerID != user.ID {
		return util.ErrForbidden.WithMessage("Менеджер может изменять только свои записи")
	}
	return nil
}

func (s *MaintenanceService) get(ctx context.Context, id int64) (dto.Maintenance, error) {
	m, err := s.logs.GetByID(ctx, id)
	if err != nil {
		return dto.Maintenance{}, err
	}
	return dto.NewMaintenance(m), nil
}

func checkCost(v *util.Validator, cost util.Money) {
	v.Check(cost >= 0 && cost <= maxMaintenanceCost, "cost", "должно быть от 0 до 99999999.99")
}
