package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	h, err := New(filepath.Join(filepath.Dir(file), "..", "..", "..", "frontend"))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// Все страницы из перечня отдаются по своим адресам из общего макета.
func TestPages(t *testing.T) {
	h := newTestHandler(t)
	for _, p := range Pages {
		rec := get(h, p.Path)
		body := rec.Body.String()
		if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
			t.Errorf("%s: %d %s", p.Path, rec.Code, rec.Header().Get("Content-Type"))
			continue
		}
		for _, want := range []string{
			"<title>" + p.Title + " | Каршеринг</title>",
			`data-page="` + p.Name + `"`,
			`data-roles="` + strings.Join(p.Roles, " ") + `"`,
			`src="/static/js/app.js"`,
			`class="site-nav"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: нет %q", p.Path, want)
			}
		}
		if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
			t.Errorf("%s: CSP = %q", p.Path, csp)
		}
	}
}

func TestPageScriptsAndMenu(t *testing.T) {
	h := newTestHandler(t)
	login := get(h, "/login").Body.String()
	if !strings.Contains(login, `data-script="/static/js/pages/login.js"`) {
		t.Error("у страницы входа должен быть подключён модуль login.js")
	}
	if !strings.Contains(login, `<li data-roles="guest" hidden><a href="/login" aria-current="page">Вход</a></li>`) {
		t.Error("пункт меню текущей страницы должен быть отмечен aria-current")
	}
	if !strings.Contains(login, `<li data-roles="admin" hidden><a href="/users">Пользователи и роли</a></li>`) {
		t.Error("пункт «Пользователи и роли» — только для администратора")
	}
}

func TestNotFoundAndStatic(t *testing.T) {
	h := newTestHandler(t)
	rec := get(h, "/no-such-page")
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "Страница не найдена") {
		t.Errorf("404: %d", rec.Code)
	}

	rec = get(h, "/static/js/app.js")
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Errorf("app.js: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := get(h, "/static/css/main.css"); rec.Code != 200 {
		t.Errorf("main.css: %d", rec.Code)
	}
	for _, p := range []string{"/static/", "/static/js/", "/static/../templates/layout.html"} {
		if rec := get(h, p); rec.Code == 200 {
			t.Errorf("%s: содержимое каталогов и файлы вне static не отдаются, получено 200", p)
		}
	}
}

func TestNewMissingDir(t *testing.T) {
	if _, err := New(t.TempDir()); err == nil {
		t.Error("ожидалась ошибка для каталога без static")
	}
}
