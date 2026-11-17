# Каршеринг — информационно-справочная система

Курсовая работа: трёхзвенная веб-система учёта автопарка, бронирования и аренды автомобилей.
Стек: Go, PostgreSQL, HTML/CSS/JavaScript, REST API.

## Документация

- [docs/api.md](docs/api.md) — контракты REST API
- [docs/role_matrix.md](docs/role_matrix.md) — матрица доступа ролей
- `docs/er-диаграмма серивиса каршеринг.xml` — ER-диаграмма (открывается в draw.io)

## Структура

```
backend/
  cmd/server/          точка входа
  internal/config/     настройки из переменных окружения
  internal/handlers/   HTTP-обработчики и маршруты
  internal/middleware/ JWT-аутентификация, проверка ролей, лог, перехват паник
  internal/services/   бизнес-логика и валидация
  internal/repository/ SQL-запросы (pgx)
  internal/models/     структуры таблиц БД
  internal/dto/        структуры запросов и ответов API
  internal/util/       ошибки API, JSON, JWT, bcrypt, валидация
db/
  schema.sql           DDL
  seed.sql             тестовые данные
  checks.sql           проверка ограничений целостности
frontend/              шаблоны и статика
```

## Запуск

Требуется Go 1.25+ и PostgreSQL 14+.

1. Создать базу и загрузить схему и тестовые данные:

   ```sh
   createdb -U postgres carsharing
   psql -U postgres -d carsharing -f db/schema.sql
   psql -U postgres -d carsharing -f db/seed.sql
   ```

   Тестовые учётные записи перечислены в начале `db/seed.sql`.

2. Настроить окружение:

   ```sh
   cd backend
   cp .env.example .env
   # заполнить JWT_SECRET (openssl rand -hex 32) и при необходимости DATABASE_URL
   ```

3. Запустить сервер:

   ```sh
   go run ./cmd/server
   ```

   Проверка: `curl http://localhost:8080/api/health`.

## Тесты

Три уровня тестов:

| Уровень | Где | Что проверяет | Нужна БД |
|---|---|---|---|
| Unit | `internal/*/..._test.go` | расчёт стоимости, деньги, валидация, JWT, роли, разбор запросов | нет |
| Интеграционные | `internal/services/*_integration_test.go` | сервисы и SQL на реальной PostgreSQL, гонки (одна машина двум клиентам, смена статуса или роли во время бронирования) | `TEST_DATABASE_URL` |
| Сквозные (e2e) | `e2e/` | весь API по HTTP: матрица доступа ролей, сценарии гостя, клиента, менеджера и администратора, единый формат ошибок, SQL-инъекции, совпадение `/api/stats` с примером в `docs/api.md` | `E2E_DATABASE_URL` |

Без переменных окружения `go test ./...` запускает только unit-тесты, остальные пропускаются.

Полный прогон (базы — отдельные от рабочей):

```sh
createdb -U postgres carsharing_test
psql -U postgres -d carsharing_test -f db/schema.sql
psql -U postgres -d carsharing_test -f db/seed.sql
createdb -U postgres carsharing_e2e_test          # схему и данные тесты загружают сами

cd backend
TEST_DATABASE_URL=postgres://postgres@localhost:5432/carsharing_test?sslmode=disable \
E2E_DATABASE_URL=postgres://postgres@localhost:5432/carsharing_e2e_test?sslmode=disable \
  go test -race -cover ./...
```

Сквозные тесты перед каждым тестом **удаляют все данные** в базе `E2E_DATABASE_URL`
и загружают `schema.sql` и `seed.sql` заново, поэтому запускаются только на базе,
в имени которой есть `test`. Интеграционные тесты удаляют за собой только созданные ими записи.

Ограничения целостности БД проверяются отдельно:

```sh
psql -U postgres -d carsharing -f db/checks.sql
```

## Готовность API

| Раздел api.md | Статус |
|---|---|
| 4. Служебные (`/api/health`) | готово |
| 5. Аутентификация и профиль | готово |
| 6–7. Каталог, автопарк, тарифы | готово |
| 8. Журнал обслуживания | готово |
| 9. Аренды | готово |
| 10. Пользователи | готово |
| 11. Статистика | готово |
