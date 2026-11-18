// Package util содержит общие вспомогательные функции: ошибки API, JSON-ответы,
// JWT, хеширование паролей и валидацию полей.
package util

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError — ошибка, которую можно показать клиенту API.
// Формат ответа описан в docs/api.md, раздел 1.4.
type AppError struct {
	Status  int               // HTTP-код ответа
	Code    string            // стабильный машинный код (error.code)
	Message string            // текст для пользователя на русском языке
	Details map[string]string // необязательные подробности (поле → причина)
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Message)
}

// WithDetails возвращает копию ошибки с подробностями.
func (e *AppError) WithDetails(details map[string]string) *AppError {
	c := *e
	c.Details = details
	return &c
}

// WithMessage возвращает копию ошибки с другим текстом.
func (e *AppError) WithMessage(msg string) *AppError {
	c := *e
	c.Message = msg
	return &c
}

func newErr(status int, code, msg string) *AppError {
	return &AppError{Status: status, Code: code, Message: msg}
}

// Ошибки из таблицы кодов docs/api.md, раздел 1.4.
var (
	ErrValidation          = newErr(http.StatusBadRequest, "validation_error", "Некорректные данные запроса")
	ErrWrongPassword       = newErr(http.StatusBadRequest, "wrong_password", "Неверный текущий пароль")
	ErrUnauthorized        = newErr(http.StatusUnauthorized, "unauthorized", "Требуется вход в систему")
	ErrInvalidCredentials  = newErr(http.StatusUnauthorized, "invalid_credentials", "Неверный email или пароль")
	ErrForbidden           = newErr(http.StatusForbidden, "forbidden", "Недостаточно прав")
	ErrLicenseRequired     = newErr(http.StatusForbidden, "license_required", "Для бронирования нужно водительское удостоверение")
	ErrCannotChangeOwnRole = newErr(http.StatusForbidden, "cannot_change_own_role", "Нельзя изменить собственную роль")
	ErrCannotDeleteSelf    = newErr(http.StatusForbidden, "cannot_delete_self", "Нельзя удалить собственную учётную запись")
	ErrNotFound            = newErr(http.StatusNotFound, "not_found", "Объект не найден")
	ErrEmailTaken          = newErr(http.StatusConflict, "email_taken", "Пользователь с таким email уже зарегистрирован")
	ErrLicenseTaken        = newErr(http.StatusConflict, "license_taken", "Это водительское удостоверение уже зарегистрировано")
	ErrPlateTaken          = newErr(http.StatusConflict, "plate_taken", "Автомобиль с таким гос. номером уже есть")
	ErrNameTaken           = newErr(http.StatusConflict, "name_taken", "Тариф с таким названием уже есть")
	ErrCarUnavailable      = newErr(http.StatusConflict, "car_unavailable", "Автомобиль недоступен для бронирования")
	ErrUserHasActiveRental = newErr(http.StatusConflict, "user_has_active_rental", "У пользователя есть бронирование или активная аренда")
	ErrInvalidTransition   = newErr(http.StatusConflict, "invalid_status_transition", "Недопустимая смена статуса")
	ErrHasRelatedRecords   = newErr(http.StatusConflict, "has_related_records", "Нельзя удалить объект: на него ссылаются другие записи")
	ErrBookingExpired      = newErr(http.StatusConflict, "booking_expired", "Срок бронирования истёк: бронирование отменено, списано ожидание")
	ErrConflict            = newErr(http.StatusConflict, "conflict", "Значение уже используется")
	ErrTooManyAttempts     = newErr(http.StatusTooManyRequests, "too_many_attempts", "Слишком много неудачных попыток входа, повторите позже")
	ErrMethodNotAllowed    = newErr(http.StatusMethodNotAllowed, "method_not_allowed", "Метод не поддерживается для этого адреса")
	ErrInternal            = newErr(http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера")
)

// AsAppError извлекает AppError из цепочки ошибок.
func AsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
