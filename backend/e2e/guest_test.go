package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"carsharing/internal/util"
)

// Сценарии гостя: проверка сервера, регистрация, вход, защита токеном.
func TestGuest(t *testing.T) {
	e := newEnv(t)
	g := e.guest()

	e.run("health", func(t *testing.T) {
		var h map[string]string
		g.call("GET", "/api/health", nil).ok(200).into(&h)
		if h["status"] != "ok" || h["database"] != "ok" {
			t.Errorf("health: %v", h)
		}
	})

	e.run("регистрация", func(t *testing.T) {
		var u user
		g.call("POST", "/api/auth/register", map[string]any{
			"email": "new.user@example.com", "password": "Secret123!",
			"full_name": "Новиков Новый", "driver_license": "7799123456",
			"role": "admin", // должно игнорироваться
		}).ok(201).into(&u)
		if u.Role != "client" || u.DriverLicense == nil || *u.DriverLicense != "7799 123456" {
			t.Errorf("зарегистрирован: %+v", u)
		}

		g.call("POST", "/api/auth/register", map[string]any{
			"email": "NEW.USER@example.com", "password": "Secret123!", "full_name": "Дубль", "driver_license": "7799000000",
		}).fails(409, "email_taken")
		g.call("POST", "/api/auth/register", map[string]any{
			"email": "other@example.com", "password": "Secret123!", "full_name": "Дубль", "driver_license": "7712 345678",
		}).fails(409, "license_taken")

		d := g.call("POST", "/api/auth/register", map[string]any{"email": "bad", "password": "1"}).
			fails(400, "validation_error").details()
		for _, f := range []string{"email", "password", "full_name", "driver_license"} {
			if d[f] == "" {
				t.Errorf("нет ошибки по полю %s: %v", f, d)
			}
		}
		g.call("POST", "/api/auth/register", "{oops").fails(400, "validation_error")
		g.call("POST", "/api/auth/register", "").fails(400, "validation_error")
	})

	e.run("вход", func(t *testing.T) {
		var resp struct {
			Token     string `json:"token"`
			TokenType string `json:"token_type"`
			ExpiresIn int    `json:"expires_in"`
			User      user   `json:"user"`
		}
		g.call("POST", "/api/auth/login", map[string]string{"email": "Ivan.Petrov@Example.com", "password": clientPass}).
			ok(200).into(&resp)
		if resp.Token == "" || resp.TokenType != "Bearer" || resp.ExpiresIn != 1800 || resp.User.Email != ivanEmail {
			t.Errorf("вход (email без учёта регистра): %+v", resp)
		}
		// Неверный пароль и неизвестный email — одинаковый ответ.
		a := g.call("POST", "/api/auth/login", map[string]string{"email": ivanEmail, "password": "wrong-pass"}).
			fails(401, "invalid_credentials")
		b := g.call("POST", "/api/auth/login", map[string]string{"email": "nobody@example.com", "password": "wrong-pass"}).
			fails(401, "invalid_credentials")
		if string(a.Body) != string(b.Body) {
			t.Errorf("ответы различаются: %s / %s", a.Body, b.Body)
		}
		g.call("POST", "/api/auth/login", map[string]string{}).fails(400, "validation_error")
	})

	e.run("токены", func(t *testing.T) {
		bad := map[string]string{
			"мусор":         "abc.def.ghi",
			"чужая подпись": issue(t, "another-secret-another-secret-another", time.Minute, 4, "client"),
			"истёкший":      issue(t, jwtSecret, -time.Minute, 4, "client"),
			"alg=none":      "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ1c2VyX2lkIjoxLCJyb2xlIjoiYWRtaW4iLCJleHAiOjk5OTk5OTk5OTl9.",
			"удалённый id":  issue(t, jwtSecret, time.Minute, 999999, "admin"),
		}
		for name, tok := range bad {
			c := &client{e: e, who: "токен " + name, token: tok}
			c.call("GET", "/api/me", nil).fails(401, "unauthorized")
		}
		// Без схемы Bearer
		r := (&client{e: e, who: "без Bearer"}).call("GET", "/api/me", nil)
		r.fails(401, "unauthorized")
	})

	e.run("неизвестный путь и метод", func(t *testing.T) {
		g.call("GET", "/api/nope", nil).fails(404, "not_found")
		r := g.call("DELETE", "/api/health", nil).fails(405, "method_not_allowed")
		if r.Header.Get("Allow") != "GET" {
			t.Errorf("Allow = %q", r.Header.Get("Allow"))
		}
		big := `{"email":"` + strings.Repeat("a", 2<<20) + `"}`
		g.call("POST", "/api/auth/login", big).fails(400, "validation_error")
	})
}

func issue(t *testing.T, secret string, ttl time.Duration, userID int64, role string) string {
	t.Helper()
	tok, err := util.NewTokenManager(secret, ttl).Issue(userID, role)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// Клиентская часть отдаётся тем же сервером, что и API; заголовки безопасности — везде.
func TestSiteAndSecurityHeaders(t *testing.T) {
	e := newEnv(t)
	get := func(path string) *http.Response {
		resp, err := e.srv.Client().Get(e.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	for _, path := range []string{"/", "/login", "/about", "/catalog", "/static/js/app.js", "/api/health"} {
		resp := get(path)
		if resp.StatusCode != 200 {
			t.Errorf("%s: %d", path, resp.StatusCode)
		}
		if resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("X-Frame-Options") != "DENY" {
			t.Errorf("%s: нет заголовков безопасности: %v", path, resp.Header)
		}
	}
	if resp := get("/nope"); resp.StatusCode != 404 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Errorf("неизвестная страница: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if resp := get("/api/nope"); resp.StatusCode != 404 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Errorf("неизвестный путь API должен отвечать JSON: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}
