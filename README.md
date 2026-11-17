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

```sh
cd backend && go test ./...                              # unit-тесты
psql -U postgres -d carsharing -f db/checks.sql          # ограничения БД
```

Интеграционные тесты (все разделы API и гонки: одновременные бронирования, смена статуса или роли во время бронирования)
работают с реальной БД, в которой загружены `schema.sql` и `seed.sql`. Созданные тестом
записи удаляются в конце, но лучше использовать отдельную базу:

```sh
cd backend
TEST_DATABASE_URL=postgres://postgres@localhost:5432/carsharing_test?sslmode=disable \
  go test -race -run Integration -v ./internal/services
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
