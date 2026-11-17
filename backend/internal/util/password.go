package util

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost совпадает со стоимостью хешей в db/seed.sql.
const bcryptCost = 10

// dummyHash используется при входе с несуществующим email, чтобы время ответа
// не выдавало, зарегистрирован ли адрес.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcryptCost)

// HashPassword возвращает bcrypt-хеш пароля.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword сравнивает пароль с хешем. Возвращает false при несовпадении
// и ошибку только при повреждённом хеше.
func CheckPassword(hash, password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	return false, fmt.Errorf("compare password: %w", err)
}

// SpendPasswordCheck выполняет сравнение с фиктивным хешем, выравнивая время ответа.
func SpendPasswordCheck(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}
