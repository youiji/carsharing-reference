package util

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNormalizeDriverLicense(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"7712 345678", "7712 345678", true},
		{"7712345678", "7712 345678", true},
		{"  7712345678 ", "7712 345678", true},
		{"771 2345678", "", false},
		{"7712  345678", "", false},
		{"77AB345678", "", false},
		{"771234567", "", false},
		{"77123456789", "", false},
		{"", "", false},
		{"７７１２３４５６７８", "", false}, // полноширинные цифры
	}
	for _, tt := range tests {
		got, ok := NormalizeDriverLicense(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("NormalizeDriverLicense(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestValidatorEmail(t *testing.T) {
	valid := []string{"ivan.petrov@example.com", "admin@carsharing.local"}
	invalid := []string{"", "ivan", "ivan@", "Ivan <ivan@example.com>", "ivan@localhost"}
	for _, e := range valid {
		var v Validator
		v.Email("email", e)
		if v.Err() != nil {
			t.Errorf("email %q должен быть корректным", e)
		}
	}
	for _, e := range invalid {
		var v Validator
		v.Email("email", e)
		if v.Err() == nil {
			t.Errorf("email %q должен быть отклонён", e)
		}
	}
}

func TestValidatorPassword(t *testing.T) {
	cases := map[string]bool{
		"Client123!":             true,
		"short":                  false,
		"":                       false,
		string(make([]byte, 73)): false,
		"пароль12":               true, // 8 символов, 14 байт
		"ппппппппппппппппппппппппппппппппппппп": false, // 37 символов, 74 байта
	}
	for pw, wantOK := range cases {
		var v Validator
		v.Password("password", pw)
		if (v.Err() == nil) != wantOK {
			t.Errorf("Password(%q): ok=%v, want %v", pw, v.Err() == nil, wantOK)
		}
	}
}

func TestValidatorDetails(t *testing.T) {
	var v Validator
	v.StringLen("full_name", "", 2, 100)
	v.StringLen("full_name", "x", 2, 100) // вторая ошибка по тому же полю не перезаписывает первую
	v.Check(false, "fuel_level", "должно быть от 0 до 100")

	appErr, ok := AsAppError(v.Err())
	if !ok || appErr.Code != "validation_error" {
		t.Fatalf("ожидалась validation_error, получено %v", v.Err())
	}
	if appErr.Details["full_name"] != "обязательное поле" || appErr.Details["fuel_level"] == "" {
		t.Errorf("неожиданные details: %v", appErr.Details)
	}
	if ErrValidation.Details != nil {
		t.Error("WithDetails не должен менять общий ErrValidation")
	}
}

func TestMapPgError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"unique email", &pgconn.PgError{Code: "23505", ConstraintName: "uq_users_email"}, "email_taken"},
		{"unique active car", &pgconn.PgError{Code: "23505", ConstraintName: "uq_rentals_one_active_per_car"}, "car_unavailable"},
		{"unique active user", &pgconn.PgError{Code: "23505", ConstraintName: "uq_rentals_one_active_per_user"}, "user_has_active_rental"},
		{"fk on delete", &pgconn.PgError{Code: "23503", Message: `update or delete on table "cars" violates foreign key constraint "fk_rentals_car" on table "rentals"`}, "has_related_records"},
		{"fk on insert", &pgconn.PgError{Code: "23503", Message: `insert or update on table "cars" violates foreign key constraint "fk_cars_tariff"`}, "validation_error"},
		{"check", &pgconn.PgError{Code: "23514", ConstraintName: "chk_cars_fuel_level"}, "validation_error"},
		{"wrapped", fmt.Errorf("create: %w", &pgconn.PgError{Code: "23505", ConstraintName: "uq_users_driver_license"}), "license_taken"},
	}
	for _, tt := range tests {
		appErr, ok := AsAppError(MapPgError(tt.err))
		if !ok || appErr.Code != tt.want {
			t.Errorf("%s: got %v, want %s", tt.name, MapPgError(tt.err), tt.want)
		}
	}

	plain := errors.New("connection refused")
	if MapPgError(plain) != plain {
		t.Error("ошибки не из PostgreSQL должны возвращаться без изменений")
	}
}

func TestTokenManager(t *testing.T) {
	secret := "test-secret-test-secret-test-secret"
	m := NewTokenManager(secret, 30*time.Minute)

	token, err := m.Issue(42, "manager")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := m.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.UserID != 42 || claims.Role != "manager" {
		t.Errorf("claims = %+v", claims)
	}

	// Чужой секрет
	if _, err := NewTokenManager("another-secret-another-secret-xx", time.Minute).Parse(token); err == nil {
		t.Error("токен с чужой подписью должен отклоняться")
	}

	// Истёкший токен
	expired := NewTokenManager(secret, 30*time.Minute)
	expired.now = func() time.Time { return time.Now().Add(-31 * time.Minute) }
	old, _ := expired.Issue(42, "client")
	if _, err := m.Parse(old); err == nil {
		t.Error("истёкший токен должен отклоняться")
	}

	// Алгоритм none
	none := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ1c2VyX2lkIjoxLCJyb2xlIjoiYWRtaW4iLCJleHAiOjk5OTk5OTk5OTl9."
	if _, err := m.Parse(none); err == nil {
		t.Error("токен с alg=none должен отклоняться")
	}
}

func TestPasswordHash(t *testing.T) {
	hash, err := HashPassword("Client123!")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := CheckPassword(hash, "Client123!"); !ok {
		t.Error("верный пароль не принят")
	}
	if ok, _ := CheckPassword(hash, "wrong"); ok {
		t.Error("неверный пароль принят")
	}
	// Хеш из db/seed.sql (формат $2b$) должен читаться Go-библиотекой.
	seedHash := "$2b$10$phmzkJ45HB9CrehS61MakeYJjHbBapcE.5wT8rBpyaxKDFDjVqO0S"
	if ok, err := CheckPassword(seedHash, "Client123!"); !ok || err != nil {
		t.Errorf("хеш из seed.sql не принят: ok=%v err=%v", ok, err)
	}
}
