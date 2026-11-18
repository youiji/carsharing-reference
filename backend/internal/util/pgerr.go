package util

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// Коды ошибок PostgreSQL (SQLSTATE).
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgCheckViolation      = "23514"
)

// uniqueConstraintErrors сопоставляет уникальные ограничения из db/schema.sql
// с ошибками API.
var uniqueConstraintErrors = map[string]*AppError{
	"uq_users_email":                 ErrEmailTaken,
	"uq_users_driver_license":        ErrLicenseTaken,
	"uq_cars_plate_number":           ErrPlateTaken,
	"uq_tariffs_name":                ErrNameTaken,
	"uq_rentals_one_active_per_car":  ErrCarUnavailable,
	"uq_rentals_one_active_per_user": ErrUserHasActiveRental,
}

// foreignKeyFields сопоставляет внешние ключи из db/schema.sql с полями запроса:
// при вставке ссылки на несуществующий объект ошибка указывает на поле.
var foreignKeyFields = map[string]string{
	"fk_cars_tariff":    "tariff_id",
	"fk_rentals_user":   "user_id",
	"fk_rentals_tariff": "tariff_id",
	"fk_rentals_car":    "car_id",
	"fk_ml_car":         "car_id",
	"fk_ml_manager":     "manager_id",
}

// MapPgError переводит ошибку PostgreSQL в AppError (docs/api.md, раздел 1.4).
// Ошибки, не относящиеся к ограничениям целостности, возвращаются без изменений.
//
//   - 23505 (уникальность)  → 409 с кодом по имени ограничения (неизвестное — 409 conflict);
//   - 23503 (внешний ключ)  → 400 validation_error (ссылка на несуществующий объект);
//   - 23514 (CHECK)         → 400 validation_error.
//
// Для DELETE используется MapPgDeleteError: там 23503 означает связанные записи.
func MapPgError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case pgUniqueViolation:
		if appErr, ok := uniqueConstraintErrors[pgErr.ConstraintName]; ok {
			return appErr
		}
		return ErrConflict
	case pgForeignKeyViolation:
		if field, ok := foreignKeyFields[pgErr.ConstraintName]; ok {
			return ErrValidation.WithDetails(map[string]string{field: "объект не найден"})
		}
		return ErrValidation.WithMessage("Ссылка на несуществующий объект")
	case pgCheckViolation:
		return ErrValidation
	}
	return err
}

// MapPgDeleteError переводит ошибку DELETE в AppError. Любое нарушение внешнего
// ключа (23503) при удалении означает, что на строку ссылаются другие записи,
// → 409 has_related_records. Решение принимается только по коду SQLSTATE, поэтому
// не зависит от языка сообщений сервера (lc_messages). Остальные ошибки — как в MapPgError.
func MapPgDeleteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		return ErrHasRelatedRecords
	}
	return MapPgError(err)
}
