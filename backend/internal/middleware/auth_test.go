package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"carsharing/internal/models"
	"carsharing/internal/repository"
	"carsharing/internal/util"
)

type fakeUsers map[int64]models.User

func (f fakeUsers) GetByID(_ context.Context, id int64) (models.User, error) {
	if u, ok := f[id]; ok {
		return u, nil
	}
	return models.User{}, repository.ErrNotFound
}

func TestAuthAndRequireRole(t *testing.T) {
	tokens := util.NewTokenManager("test-secret-test-secret-test-secret", time.Minute)
	users := fakeUsers{
		1: {ID: 1, Role: models.RoleAdmin},
		2: {ID: 2, Role: models.RoleClient},
		// пользователь 3 повышен до manager в БД, но в старом токене роль client
		3: {ID: 3, Role: models.RoleManager},
	}
	tok := func(id int64, role string) string {
		s, err := tokens.Issue(id, role)
		if err != nil {
			t.Fatal(err)
		}
		return "Bearer " + s
	}

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := CurrentUser(r.Context())
		w.Write([]byte(u.Role))
	})
	h := Chain(ok, Auth(tokens, users), RequireRole(models.RoleManager, models.RoleAdmin))

	tests := []struct {
		name   string
		header string
		status int
	}{
		{"нет заголовка", "", 401},
		{"не Bearer", "Basic abc", 401},
		{"мусор вместо токена", "Bearer abc.def.ghi", 401},
		{"админ", tok(1, models.RoleAdmin), 200},
		{"клиент", tok(2, models.RoleClient), 403},
		{"роль берётся из БД, а не из токена", tok(3, models.RoleClient), 200},
		{"клиент подделал роль в claims (подпись та же)", tok(2, models.RoleAdmin), 403},
		{"удалённый пользователь", tok(99, models.RoleAdmin), 401},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
		if tt.header != "" {
			req.Header.Set("Authorization", tt.header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tt.status {
			t.Errorf("%s: status %d, want %d (body %s)", tt.name, rec.Code, tt.status, rec.Body)
		}
		if rec.Code >= 400 && !strings.Contains(rec.Body.String(), `"error"`) {
			t.Errorf("%s: ответ не в едином формате ошибок: %s", tt.name, rec.Body)
		}
	}
}

func TestRecover(t *testing.T) {
	h := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), "internal_error") {
		t.Errorf("got %d %s", rec.Code, rec.Body)
	}
}
