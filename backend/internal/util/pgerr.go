package util

import (
	"errors"
	"strings"

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

// MapPgError переводит ошибку PostgreSQL в AppError (docs/api.md, раздел 1.4).
// Ошибки, не относящиеся к ограничениям целостности, возвращаются без изменений.
//
//   - 23505 (уникальность)  → 409 с кодом по имени ограничения;
//   - 23503 при удалении    → 409 has_related_records;
//   - 23503 при вставке     → 400 validation_error (ссылка на несуществующий объект);
//   - 23514 (CHECK)         → 400 validation_error.
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
		return ErrValidation.WithMessage("Значение уже используется")
	case pgForeignKeyViolation:
		// PostgreSQL: "update or delete on table ... violates foreign key constraint ..."
		if strings.HasPrefix(pgErr.Message, "update or delete") {
			return ErrHasRelatedRecords
		}
		return ErrValidation.WithMessage("Ссылка на несуществующий объект")
	case pgCheckViolation:
		return ErrValidation
	}
	return err
}
