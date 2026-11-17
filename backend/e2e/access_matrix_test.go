package e2e

import (
	"net/http"
	"slices"
	"testing"
)

// TestAccessMatrix проверяет матрицу доступа (docs/role_matrix.md, раздел 2)
// для каждого защищённого эндпоинта и каждой роли:
//
//   - гость (без токена)        → 401 unauthorized;
//   - роль не из списка          → 403 forbidden;
//   - роль из списка             → запрос доходит до сервиса (любой ответ, кроме 401/403).
//
// Запросы используют несуществующие id и пустые тела, поэтому данные не меняются:
// разрешённая роль получает 200, 400 (валидация) или 404 (объекта нет).
func TestAccessMatrix(t *testing.T) {
	e := newEnv(t)
	const (
		C = "client"
		M = "manager"
		A = "admin"
	)
	users := map[string]*client{
		C: e.login(dmitryEmail, clientPass),
		M: e.login(manager1, managerPass),
		A: e.login(adminEmail, adminPass),
	}

	matrix := []struct {
		method, path string
		body         any
		allowed      []string
	}{
		// 2.1 Профиль
		{"GET", "/api/me", nil, []string{C, M, A}},
		{"PUT", "/api/me/password", map[string]any{}, []string{C, M, A}},
		// 2.2 Каталог и карта
		{"GET", "/api/cars", nil, []string{C, M, A}},
		{"GET", "/api/cars/999999", nil, []string{C, M, A}},
		{"GET", "/api/tariffs", nil, []string{C, M, A}},
		// 2.3 Автопарк и тарифы
		{"POST", "/api/cars", map[string]any{}, []string{M, A}},
		{"PUT", "/api/cars/999999", map[string]any{}, []string{M, A}},
		{"PATCH", "/api/cars/999999/status", map[string]any{}, []string{M, A}},
		{"DELETE", "/api/cars/999999", nil, []string{M, A}},
		{"POST", "/api/tariffs", map[string]any{}, []string{M, A}},
		{"PUT", "/api/tariffs/999999", map[string]any{}, []string{M, A}},
		{"DELETE", "/api/tariffs/999999", nil, []string{M, A}},
		// 2.4 Журнал обслуживания
		{"GET", "/api/maintenance", nil, []string{M, A}},
		{"POST", "/api/maintenance", map[string]any{}, []string{M, A}},
		{"PATCH", "/api/maintenance/999999", map[string]any{}, []string{M, A}},
		{"DELETE", "/api/maintenance/999999", nil, []string{M, A}},
		// 2.5 Аренды
		{"POST", "/api/rentals", map[string]any{}, []string{C}},
		{"POST", "/api/rentals/999999/start", nil, []string{C}},
		{"POST", "/api/rentals/999999/finish", nil, []string{C}},
		{"POST", "/api/rentals/999999/cancel", nil, []string{C, M, A}},
		{"GET", "/api/rentals/my", nil, []string{C}},
		{"GET", "/api/rentals", nil, []string{M, A}},
		{"GET", "/api/rentals/999999", nil, []string{C, M, A}},
		// 2.6 Пользователи и роли
		{"GET", "/api/users", nil, []string{A}},
		{"PATCH", "/api/users/999999/role", map[string]any{}, []string{A}},
		{"PATCH", "/api/users/999999", map[string]any{}, []string{A}},
		{"DELETE", "/api/users/999999", nil, []string{A}},
		// 2.7 Статистика
		{"GET", "/api/stats", nil, []string{M, A}},
	}

	for _, ep := range matrix {
		name := ep.method + " " + ep.path
		e.guest().call(ep.method, ep.path, ep.body).fails(http.StatusUnauthorized, "unauthorized")

		for _, role := range []string{C, M, A} {
			r := users[role].call(ep.method, ep.path, ep.body)
			if slices.Contains(ep.allowed, role) {
				if r.Status == http.StatusUnauthorized || r.Status == http.StatusForbidden {
					t.Errorf("%s: роль %s должна иметь доступ, получено %d %s", name, role, r.Status, r.Body)
				}
			} else if r.Status != http.StatusForbidden || r.code() != "forbidden" {
				t.Errorf("%s: роль %s не должна иметь доступа, получено %d %s", name, role, r.Status, r.Body)
			}
		}
	}

	// Публичные эндпоинты доступны без токена.
	e.guest().call("GET", "/api/health", nil).ok(200)
	e.guest().call("POST", "/api/auth/login", map[string]string{"email": "x@x.ru", "password": "x"}).
		fails(401, "invalid_credentials")
	e.guest().call("POST", "/api/auth/register", map[string]any{}).fails(400, "validation_error")
}
