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

// UserService — управление пользователями администратором (docs/api.md, раздел 10).
type UserService struct {
	users *repository.UserRepository
}

func NewUserService(users *repository.UserRepository) *UserService {
	return &UserService{users: users}
}

var errUserNotFound = util.ErrNotFound.WithMessage("Пользователь не найден")

// List — GET /api/users.
func (s *UserService) List(ctx context.Context, f models.UserFilter) (dto.List[dto.User], error) {
	users, err := s.users.List(ctx, f)
	if err != nil {
		return dto.List[dto.User]{}, err
	}
	return dto.NewList(dto.NewUsers(users)), nil
}

// ChangeRole — PATCH /api/users/{id}/role. Свою роль менять нельзя; пока у
// пользователя есть бронирование или поездка — тоже (клиент, переведённый
// в другую роль, не смог бы её завершить).
func (s *UserService) ChangeRole(ctx context.Context, admin models.User, id int64, req dto.UserRoleRequest) (dto.User, error) {
	role := strings.TrimSpace(req.Role)
	if !slices.Contains(models.UserRoles, role) {
		return dto.User{}, util.ErrValidation.WithDetails(map[string]string{
			"role": "допустимые значения: " + strings.Join(models.UserRoles, ", "),
		})
	}
	if id == admin.ID {
		return dto.User{}, util.ErrCannotChangeOwnRole
	}
	return s.modify(ctx, id, func(u *models.User) { u.Role = role })
}

// Update — PATCH /api/users/{id}: ФИО, ВУ и/или email (аналог обращения в поддержку).
func (s *UserService) Update(ctx context.Context, id int64, req dto.UpdateUserRequest) (dto.User, error) {
	var v util.Validator
	if req.FullName == nil && req.DriverLicense == nil && req.Email == nil {
		v.Add("full_name", "передайте хотя бы одно из полей: full_name, driver_license, email")
	}
	var fullName, license, email string
	if req.FullName != nil {
		fullName = strings.TrimSpace(*req.FullName)
		v.StringLen("full_name", fullName, 2, 100)
	}
	if req.DriverLicense != nil {
		var ok bool
		if license, ok = util.NormalizeDriverLicense(*req.DriverLicense); !ok {
			v.Add("driver_license", "10 цифр, например 7712 345678")
		}
	}
	if req.Email != nil {
		email = strings.TrimSpace(*req.Email)
		v.Email("email", email)
	}
	if err := v.Err(); err != nil {
		return dto.User{}, err
	}

	return s.modify(ctx, id, func(u *models.User) {
		if req.FullName != nil {
			u.FullName = fullName
		}
		if req.DriverLicense != nil {
			u.DriverLicense = &license
		}
		if req.Email != nil {
			u.Email = email
		}
	})
}

// Delete — DELETE /api/users/{id}. Себя удалить нельзя; пользователя с историей — тоже.
func (s *UserService) Delete(ctx context.Context, admin models.User, id int64) error {
	if id == admin.ID {
		return util.ErrCannotDeleteSelf
	}
	err := s.users.Delete(ctx, id)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return errUserNotFound
	case errors.Is(err, util.ErrHasRelatedRecords):
		return util.ErrHasRelatedRecords.WithMessage("Нельзя удалить пользователя: у него есть аренды или записи обслуживания")
	}
	return err
}

// modify в транзакции блокирует пользователя, проверяет отсутствие открытой
// аренды, применяет change и сохраняет результат.
func (s *UserService) modify(ctx context.Context, id int64, change func(u *models.User)) (dto.User, error) {
	var saved models.User
	err := s.users.InTx(ctx, func(tx *repository.UserTx) error {
		u, err := tx.Lock(ctx, id)
		if errors.Is(err, repository.ErrNotFound) {
			return errUserNotFound
		}
		if err != nil {
			return err
		}
		open, err := tx.HasOpenRental(ctx, id)
		if err != nil {
			return err
		}
		if open {
			return util.ErrUserHasActiveRental.WithMessage(
				"У пользователя есть бронирование или поездка: изменить данные можно после её завершения")
		}
		change(&u)
		saved, err = tx.Save(ctx, u)
		return err
	})
	if err != nil {
		return dto.User{}, err
	}
	return dto.NewUser(saved), nil
}
