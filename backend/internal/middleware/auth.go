// Package middleware содержит обёртки HTTP-обработчиков: аутентификацию,
// проверку ролей, логирование и перехват паник.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"carsharing/internal/models"
	"carsharing/internal/repository"
	"carsharing/internal/util"
)

// UserLoader загружает пользователя по id из БД.
type UserLoader interface {
	GetByID(ctx context.Context, id int64) (models.User, error)
}

type ctxKey struct{}

// CurrentUser возвращает пользователя, которого Auth положил в контекст.
// В обработчиках за Auth второе значение всегда true.
func CurrentUser(ctx context.Context) (models.User, bool) {
	u, ok := ctx.Value(ctxKey{}).(models.User)
	return u, ok
}

// WithUser кладёт пользователя в контекст (используется в Auth и в тестах).
func WithUser(ctx context.Context, u models.User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

// Auth проверяет JWT из заголовка Authorization: Bearer <токен>
// и загружает пользователя из БД (docs/role_matrix.md, раздел 4, п. 4):
// удалённый пользователь получает 401, а роль берётся из БД, а не из токена.
func Auth(tokens *util.TokenManager, users UserLoader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				util.WriteError(w, r, util.ErrUnauthorized)
				return
			}
			claims, err := tokens.Parse(raw)
			if err != nil {
				util.WriteError(w, r, util.ErrUnauthorized.WithMessage("Сессия истекла или токен недействителен, войдите снова"))
				return
			}
			user, err := users.GetByID(r.Context(), claims.UserID)
			if errors.Is(err, repository.ErrNotFound) {
				util.WriteError(w, r, util.ErrUnauthorized)
				return
			}
			if err != nil {
				util.WriteError(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
		})
	}
}

// RequireRole пропускает только пользователей с одной из указанных ролей.
// Используется после Auth. Для функций менеджера: RequireRole("manager", "admin").
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := CurrentUser(r.Context())
			if !ok {
				util.WriteError(w, r, util.ErrUnauthorized)
				return
			}
			if !slices.Contains(roles, user.Role) {
				util.WriteError(w, r, util.ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}
