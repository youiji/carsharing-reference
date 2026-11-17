package services

import (
	"context"
	"testing"
	"time"

	"carsharing/internal/dto"
	"carsharing/internal/models"
	"carsharing/internal/repository"
	"carsharing/internal/util"
)

// memUsers — хранилище пользователей в памяти для тестов.
type memUsers struct {
	byID map[int64]models.User
	next int64
}

func newMemUsers() *memUsers { return &memUsers{byID: map[int64]models.User{}, next: 1} }

func (m *memUsers) Create(_ context.Context, u models.User) (models.User, error) {
	for _, x := range m.byID {
		if x.Email == u.Email {
			return models.User{}, util.ErrEmailTaken
		}
	}
	u.ID, u.CreatedAt = m.next, time.Now()
	m.next++
	m.byID[u.ID] = u
	return u, nil
}

func (m *memUsers) GetByID(_ context.Context, id int64) (models.User, error) {
	if u, ok := m.byID[id]; ok {
		return u, nil
	}
	return models.User{}, repository.ErrNotFound
}

func (m *memUsers) GetByEmail(_ context.Context, email string) (models.User, error) {
	for _, u := range m.byID {
		if u.Email == email {
			return u, nil
		}
	}
	return models.User{}, repository.ErrNotFound
}

func (m *memUsers) UpdatePasswordHash(_ context.Context, id int64, hash string) error {
	u := m.byID[id]
	u.PasswordHash = hash
	m.byID[id] = u
	return nil
}

func errCode(err error) string {
	if appErr, ok := util.AsAppError(err); ok {
		return appErr.Code
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func TestAuthFlow(t *testing.T) {
	ctx := context.Background()
	svc := NewAuthService(newMemUsers(), util.NewTokenManager("test-secret-test-secret-test-secret", 30*time.Minute))

	// Регистрация: роль всегда client, ВУ нормализуется.
	u, err := svc.Register(ctx, dto.RegisterRequest{
		Email: " ivan.petrov@example.com ", Password: "Client123!",
		FullName: "Петров Иван Сергеевич", DriverLicense: "7712345678",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if u.Role != models.RoleClient || u.DriverLicense == nil || *u.DriverLicense != "7712 345678" || u.Email != "ivan.petrov@example.com" {
		t.Errorf("неожиданный пользователь: %+v", u)
	}

	// Ошибки валидации собираются по всем полям сразу.
	_, err = svc.Register(ctx, dto.RegisterRequest{Email: "bad", Password: "123", DriverLicense: "12"})
	appErr, _ := util.AsAppError(err)
	if appErr == nil || len(appErr.Details) != 4 {
		t.Errorf("ожидались ошибки по 4 полям, получено %v", err)
	}

	// Вход
	if _, err := svc.Login(ctx, dto.LoginRequest{Email: "ivan.petrov@example.com", Password: "wrong-pass"}); errCode(err) != "invalid_credentials" {
		t.Errorf("неверный пароль: %v", err)
	}
	if _, err := svc.Login(ctx, dto.LoginRequest{Email: "nobody@example.com", Password: "Client123!"}); errCode(err) != "invalid_credentials" {
		t.Errorf("неизвестный email: %v", err)
	}
	resp, err := svc.Login(ctx, dto.LoginRequest{Email: "ivan.petrov@example.com", Password: "Client123!"})
	if err != nil || resp.Token == "" || resp.ExpiresIn != 1800 || resp.TokenType != "Bearer" {
		t.Fatalf("Login: %+v, %v", resp, err)
	}

	// Смена пароля
	if err := svc.ChangePassword(ctx, u.ID, dto.ChangePasswordRequest{CurrentPassword: "nope-nope", NewPassword: "NewPass456!"}); errCode(err) != "wrong_password" {
		t.Errorf("неверный текущий пароль: %v", err)
	}
	if err := svc.ChangePassword(ctx, u.ID, dto.ChangePasswordRequest{CurrentPassword: "Client123!", NewPassword: "short"}); errCode(err) != "validation_error" {
		t.Errorf("слабый новый пароль: %v", err)
	}
	if err := svc.ChangePassword(ctx, u.ID, dto.ChangePasswordRequest{CurrentPassword: "Client123!", NewPassword: "NewPass456!"}); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, err := svc.Login(ctx, dto.LoginRequest{Email: "ivan.petrov@example.com", Password: "NewPass456!"}); err != nil {
		t.Errorf("вход с новым паролем: %v", err)
	}
}
