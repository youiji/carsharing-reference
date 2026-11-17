package util

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

// maxBodyBytes ограничивает размер тела запроса (защита от слишком больших запросов).
const maxBodyBytes = 1 << 20 // 1 МБ

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// WriteJSON отправляет ответ в формате JSON.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "err", err)
	}
}

// WriteError отправляет ошибку в едином формате.
// Ошибки, не являющиеся AppError, пишутся в лог, а клиент получает 500 internal_error
// без подробностей.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	appErr, ok := AsAppError(err)
	if !ok {
		slog.Error("unhandled error", "method", r.Method, "path", r.URL.Path, "err", err)
		appErr = ErrInternal
	}
	WriteJSON(w, appErr.Status, errorBody{Error: errorPayload{
		Code:    appErr.Code,
		Message: appErr.Message,
		Details: appErr.Details,
	}})
}

// DecodeJSON читает тело запроса в dst. Пустое тело, некорректный JSON
// или лишние данные после объекта дают 400 validation_error.
// Неизвестные поля игнорируются (например, role при регистрации).
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return ErrValidation.WithMessage("Пустое тело запроса")
		}
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return ErrValidation.WithMessage("Слишком большое тело запроса")
		}
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			return ErrValidation.WithDetails(map[string]string{typeErr.Field: "неверный тип значения"})
		}
		return ErrValidation.WithMessage("Некорректный JSON в теле запроса")
	}
	if dec.More() {
		return ErrValidation.WithMessage("Некорректный JSON в теле запроса")
	}
	return nil
}
