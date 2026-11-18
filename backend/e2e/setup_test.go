// Package e2e — сквозные тесты REST API: настоящий маршрутизатор, настоящие
// сервисы и PostgreSQL, запросы идут по HTTP. Тесты повторяют сценарии
// функционального тестирования для каждой роли (гость, клиент, менеджер, админ).
//
// Каждый тест пересоздаёт схему и загружает db/schema.sql и db/seed.sql,
// поэтому для тестов нужна ОТДЕЛЬНАЯ база, в имени которой есть "test":
//
//	createdb -U postgres carsharing_e2e_test
//	E2E_DATABASE_URL=postgres://postgres@localhost:5432/carsharing_e2e_test?sslmode=disable \
//	  go test ./e2e -v
//
// Без E2E_DATABASE_URL тесты пропускаются.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"carsharing/internal/app"
	"carsharing/internal/util"
)

const jwtSecret = "e2e-secret-e2e-secret-e2e-secret-e2e"

// Тестовые учётные записи из db/seed.sql (id назначаются в порядке строк).
const (
	adminEmail   = "admin@carsharing.local"
	adminPass    = "Admin123!"
	manager1     = "manager1@carsharing.local"
	manager2     = "manager2@carsharing.local"
	managerPass  = "Manager123!"
	ivanEmail    = "ivan.petrov@example.com" // id 4, активная поездка (аренда 11, машина 3)
	annaEmail    = "anna.sidorova@example.com"
	dmitryEmail  = "dmitry.ivanov@example.com" // id 6, нет открытых аренд
	mariaEmail   = "maria.volkova@example.com" // id 7, бронирование (аренда 13, машина 4)
	noLicense    = "new.client@example.com"    // id 9, без водительского удостоверения
	clientPass   = "Client123!"
	dmitryID     = 6
	newClientID  = 9
	ivanRental   = 11
	mariaRental  = 13
	olegRental   = 14
	mariaCar     = 4
	freeEconomy  = 1 // Kia Rio, А101АА777, Эконом, available
	freeComfort  = 9 // Toyota Camry, Комфорт, available
	maintenance1 = 1 // запись manager1 по машине 1
	maintenance3 = 3 // запись manager2
)

type env struct {
	t    *testing.T
	srv  *httptest.Server
	pool *pgxpool.Pool
}

// newEnv пересоздаёт БД из schema.sql и seed.sql и запускает API на тестовом HTTP-сервере.
func newEnv(t *testing.T) *env {
	t.Helper()
	url := os.Getenv("E2E_DATABASE_URL")
	if url == "" {
		t.Skip("E2E_DATABASE_URL не задана")
	}
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cfg.Database, "test") {
		t.Fatalf("E2E_DATABASE_URL указывает на базу %q: тесты удаляют все данные, "+
			"поэтому имя базы должно содержать \"test\"", cfg.Database)
	}

	ctx := context.Background()
	resetDB(t, ctx, url)

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	_, thisFile, _, _ := runtime.Caller(0)
	handler, err := app.NewHandler(pool, util.NewTokenManager(jwtSecret, 30*time.Minute),
		filepath.Join(filepath.Dir(thisFile), "..", "..", "frontend"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(func() {
		srv.Close()
		pool.Close()
	})
	return &env{t: t, srv: srv, pool: pool}
}

func resetDB(t *testing.T, ctx context.Context, url string) {
	t.Helper()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	_, thisFile, _, _ := runtime.Caller(0)
	dbDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "db")
	exec := func(name, sql string) {
		// Без аргументов pgx выполняет запрос простым протоколом,
		// поэтому файл может содержать несколько команд.
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	exec("drop schema", `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	exec("schema.sql", readSQL(t, filepath.Join(dbDir, "schema.sql")))
	exec("seed.sql", readSQL(t, filepath.Join(dbDir, "seed.sql")))
}

// readSQL читает SQL-файл без мета-команд psql (строки, начинающиеся с \).
func readSQL(t *testing.T, path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), `\`) {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// run запускает подтест. На время подтеста помощники (call, ok, fails, into)
// сообщают об ошибках в него, а не в родительский тест.
func (e *env) run(name string, f func(t *testing.T)) {
	e.t.Run(name, func(t *testing.T) {
		parent := e.t
		e.t = t
		defer func() { e.t = parent }()
		f(t)
	})
}

// ---------- HTTP-клиент ----------

type client struct {
	e     *env
	who   string
	token string
}

func (e *env) guest() *client { return &client{e: e, who: "гость"} }

// login входит под учётной записью и возвращает клиента с токеном.
func (e *env) login(email, password string) *client {
	e.t.Helper()
	var out struct {
		Token string `json:"token"`
	}
	e.guest().call("POST", "/api/auth/login", map[string]string{"email": email, "password": password}).
		ok(200).into(&out)
	return &client{e: e, who: email, token: out.Token}
}

type response struct {
	t      *testing.T
	req    string
	Status int
	Body   []byte
	Header http.Header
}

// call отправляет запрос. body: nil — без тела, string — как есть, иначе JSON.
func (c *client) call(method, path string, body any) *response {
	c.e.t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		r = strings.NewReader(b)
	default:
		data, err := json.Marshal(b)
		if err != nil {
			c.e.t.Fatal(err)
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.e.srv.URL+path, r)
	if err != nil {
		c.e.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.e.srv.Client().Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	res := &response{t: c.e.t, req: c.who + ": " + method + " " + path, Status: resp.StatusCode, Body: data, Header: resp.Header}
	res.checkFormat()
	return res
}

// checkFormat проверяет общие правила API для любого ответа:
// JSON с правильным Content-Type, ошибки — в едином формате с кодом и текстом.
func (r *response) checkFormat() {
	r.t.Helper()
	if r.Status == http.StatusNoContent {
		if len(r.Body) != 0 {
			r.t.Errorf("%s: 204 с телом %s", r.req, r.Body)
		}
		return
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		r.t.Errorf("%s: Content-Type %q, ожидался JSON", r.req, ct)
	}
	if r.Status >= 400 {
		var e struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(r.Body, &e); err != nil || e.Error.Code == "" || e.Error.Message == "" {
			r.t.Errorf("%s: ошибка не в едином формате: %s", r.req, r.Body)
		}
	}
	if bytes.Contains(r.Body, []byte("password_hash")) || bytes.Contains(r.Body, []byte("$2b$")) {
		r.t.Errorf("%s: в ответе есть хеш пароля: %s", r.req, r.Body)
	}
}

// ok проверяет код успешного ответа.
func (r *response) ok(status int) *response {
	r.t.Helper()
	if r.Status != status {
		r.t.Fatalf("%s: код %d, ожидался %d; ответ: %s", r.req, r.Status, status, r.Body)
	}
	return r
}

// fails проверяет код и error.code ошибки.
func (r *response) fails(status int, code string) *response {
	r.t.Helper()
	if r.Status != status || r.code() != code {
		r.t.Fatalf("%s: получено %d %q, ожидалось %d %q; ответ: %s", r.req, r.Status, r.code(), status, code, r.Body)
	}
	return r
}

func (r *response) code() string {
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(r.Body, &e)
	return e.Error.Code
}

// details возвращает error.details.
func (r *response) details() map[string]string {
	var e struct {
		Error struct {
			Details map[string]string `json:"details"`
		} `json:"error"`
	}
	json.Unmarshal(r.Body, &e)
	return e.Error.Details
}

// into разбирает JSON-ответ. Числа сохраняются как json.Number,
// чтобы проверять денежный формат ("10.90") без потери точности.
func (r *response) into(v any) *response {
	r.t.Helper()
	dec := json.NewDecoder(bytes.NewReader(r.Body))
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		r.t.Fatalf("%s: разбор ответа: %v; ответ: %s", r.req, err, r.Body)
	}
	return r
}

// ---------- Объекты ответов ----------

type list[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type user struct {
	ID            int64   `json:"id"`
	Email         string  `json:"email"`
	FullName      string  `json:"full_name"`
	DriverLicense *string `json:"driver_license"`
	Role          string  `json:"role"`
}

type tariff struct {
	ID            int64       `json:"id"`
	Name          string      `json:"name"`
	PricePerMin   json.Number `json:"price_per_min"`
	WaitingPerMin json.Number `json:"waiting_per_min"`
}

type car struct {
	ID          int64       `json:"id"`
	BrandModel  string      `json:"brand_model"`
	PlateNumber string      `json:"plate_number"`
	FuelLevel   int         `json:"fuel_level"`
	Latitude    json.Number `json:"latitude"`
	Longitude   json.Number `json:"longitude"`
	Status      string      `json:"status"`
	Tariff      tariff      `json:"tariff"`
}

type rental struct {
	ID            int64              `json:"id"`
	User          struct{ ID int64 } `json:"user"`
	Car           struct{ ID int64 } `json:"car"`
	BookedAt      string             `json:"booked_at"`
	ExpiresAt     *string            `json:"expires_at"`
	StartedAt     *string            `json:"started_at"`
	EndedAt       *string            `json:"ended_at"`
	PricePerMin   json.Number        `json:"price_per_min"`
	WaitingPerMin json.Number        `json:"waiting_per_min"`
	TotalCost     *json.Number       `json:"total_cost"`
	Status        string             `json:"status"`
}

type maintenance struct {
	ID          int64              `json:"id"`
	Car         struct{ ID int64 } `json:"car"`
	Manager     struct{ ID int64 } `json:"manager"`
	Description string             `json:"description"`
	Cost        json.Number        `json:"cost"`
}

// money переводит денежное значение API в копейки и проверяет формат "N.NN".
func money(t *testing.T, n json.Number) int64 {
	t.Helper()
	s := n.String()
	if i := strings.IndexByte(s, '.'); i < 0 || len(s)-i-1 != 2 {
		t.Fatalf("денежное значение %q не в формате с двумя знаками после запятой", s)
	}
	m, err := util.ParseMoney(s)
	if err != nil {
		t.Fatal(err)
	}
	return int64(m)
}

// at разбирает время из ответа API (RFC 3339).
func at(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("время %q: %v", s, err)
	}
	return v
}

// ceilMinutes — округление длительности вверх до минут, как в docs/api.md, раздел 2.3.
func ceilMinutes(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	return int64((d + time.Minute - 1) / time.Minute)
}
