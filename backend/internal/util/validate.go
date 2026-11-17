package util

import (
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"
)

// Validator накапливает ошибки по полям, чтобы вернуть их все одним ответом
// 400 validation_error с details (docs/api.md, раздел 12).
type Validator struct {
	errs map[string]string
}

// Add запоминает первую ошибку для поля.
func (v *Validator) Add(field, msg string) {
	if v.errs == nil {
		v.errs = make(map[string]string)
	}
	if _, exists := v.errs[field]; !exists {
		v.errs[field] = msg
	}
}

// Check добавляет ошибку, если условие не выполнено.
func (v *Validator) Check(ok bool, field, msg string) {
	if !ok {
		v.Add(field, msg)
	}
}

// Err возвращает ErrValidation с подробностями или nil, если ошибок нет.
func (v *Validator) Err() error {
	if len(v.errs) == 0 {
		return nil
	}
	return ErrValidation.WithDetails(v.errs)
}

// StringLen проверяет длину строки в символах (не в байтах).
func (v *Validator) StringLen(field, value string, min, max int) {
	n := utf8.RuneCountInString(value)
	switch {
	case n == 0 && min > 0:
		v.Add(field, "обязательное поле")
	case n < min || n > max:
		v.Add(field, fmt.Sprintf("должно быть от %d до %d символов", min, max))
	}
}

// Email проверяет формат адреса и длину до 100 символов.
func (v *Validator) Email(field, value string) {
	if value == "" {
		v.Add(field, "обязательное поле")
		return
	}
	if utf8.RuneCountInString(value) > 100 {
		v.Add(field, "не более 100 символов")
		return
	}
	addr, err := mail.ParseAddress(value)
	if err != nil || addr.Address != value || !strings.Contains(value[strings.LastIndex(value, "@")+1:], ".") {
		v.Add(field, "некорректный email")
	}
}

// Password проверяет длину пароля: от 8 символов и не более 72 байт
// (bcrypt учитывает только первые 72 байта).
func (v *Validator) Password(field, value string) {
	switch {
	case value == "":
		v.Add(field, "обязательное поле")
	case utf8.RuneCountInString(value) < 8:
		v.Add(field, "не менее 8 символов")
	case len(value) > 72:
		v.Add(field, "не более 72 символов (латиница) — ограничение bcrypt")
	}
}

// NormalizeDriverLicense приводит номер ВУ к виду "NNNN NNNNNN".
// Допустимы 10 цифр подряд или с одним пробелом после четвёртой.
// Второе значение false, если формат неверный.
func NormalizeDriverLicense(s string) (string, bool) {
	s = strings.TrimSpace(s)
	switch len(s) {
	case 10:
		// 10 цифр подряд
	case 11:
		if s[4] != ' ' {
			return "", false
		}
		s = s[:4] + s[5:]
	default:
		return "", false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return "", false
		}
	}
	return s[:4] + " " + s[4:], true
}
