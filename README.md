# Каршеринг — информационно-справочная система

Курсовая работа: трёхзвенная веб-система учёта автопарка, бронирования и аренды автомобилей.
Стек: Go, PostgreSQL, HTML/CSS/JavaScript, REST API.

## Возможности

| Роль | Что доступно |
|---|---|
| Гость | главная, вход, регистрация, «Об авторе» |
| Клиент | каталог свободных автомобилей в таблице и на карте, бронирование (действует 20 минут), начало и завершение поездки, история и стоимость поездок, профиль |
| Менеджер | автопарк (добавление, изменение, статусы, удаление), тарифы, журнал обслуживания, все аренды с отменой бронирований, статистика с диаграммами |
| Администратор | всё, что у менеджера, плюс пользователи и роли |

Подробно: [docs/role_matrix.md](docs/role_matrix.md) — матрица доступа, [docs/api.md](docs/api.md) — контракты REST API,
`docs/er-диаграмма серивиса каршеринг.xml` — ER-диаграмма (открывается в draw.io).

## Структура

```
backend/
  cmd/server/          точка входа
  internal/app/        сборка приложения (API + страницы), фоновая отмена просроченных бронирований
  internal/config/     настройки из переменных окружения
  internal/handlers/   HTTP-обработчики и маршруты API
  internal/middleware/ JWT-аутентификация, проверка ролей, лог, перехват паник
  internal/services/   бизнес-логика и валидация
  internal/repository/ SQL-запросы (pgx)
  internal/models/     структуры таблиц БД
  internal/dto/        структуры запросов и ответов API
  internal/ratelimit/  защита входа от подбора пароля
  internal/util/       ошибки API, JSON, JWT, bcrypt, деньги, валидация
  internal/web/        страницы из шаблонов, статика, заголовки безопасности
  e2e/                 сквозные тесты API
db/
  schema.sql           DDL
  seed.sql             тестовые данные
  checks.sql           проверка ограничений целостности
frontend/
  templates/           макет (layout.html) и шаблоны страниц (pages/)
  static/css, js, img  стили, модули страниц (js/pages), иконка
  static/fonts         шрифты IBM Plex (кириллица)
  static/vendor        Leaflet, MapLibre GL, Leaflet.markercluster, Chart.js
```

## Запуск

Требуется Go 1.25+ и PostgreSQL 14+.

1. **База данных.** Создать пользователя и базу, принадлежащую ему, и загрузить схему и данные
   **от имени этого пользователя** — иначе таблицы будут принадлежать `postgres`, и сервер
   получит «permission denied»:

   ```sh
   psql -U postgres -c "CREATE ROLE carsharing LOGIN PASSWORD 'carsharing'"
   createdb -U postgres -O carsharing carsharing
   psql -h localhost -U carsharing -d carsharing -f db/schema.sql
   psql -h localhost -U carsharing -d carsharing -f db/seed.sql
   ```

2. **Настройки.** Файл `backend/.env` (в git не попадает):

   ```sh
   cd backend
   cp .env.example .env
   # вписать JWT_SECRET: openssl rand -hex 32
   ```

3. **Сервер** — запускать из папки `backend` (оттуда читаются `.env` и `../frontend`):

   ```sh
   go run ./cmd/server
   ```

   В логе появится `server started ... url=http://localhost:8080` — откройте этот адрес.
   Остановка — `Ctrl+C`.

### Учётные записи тестовых данных

| Роль | Email | Пароль |
|---|---|---|
| Администратор | `admin@carsharing.local` | `Admin123!` |
| Менеджер | `manager1@carsharing.local`, `manager2@carsharing.local` | `Manager123!` |
| Клиент | `ivan.petrov@example.com` (в поездке), `maria.volkova@example.com` (бронирование), `dmitry.ivanov@example.com` (без аренды) | `Client123!` |
| Клиент без водительского удостоверения | `new.client@example.com` | `Client123!` |

### Перед демонстрацией

Текущие поездки и бронирования в тестовых данных отсчитываются от момента загрузки,
поэтому данные «стареют»: через сутки поездка Ивана будет длиться сутки. Перед показом
загрузите данные заново (таблицы очищаются, схема не меняется):

```sh
psql -h localhost -U carsharing -d carsharing -f db/seed.sql
```

Бронирование действует 20 минут: оба тестовых бронирования сервер отменит через 15–18 минут
после загрузки (со списанием ожидания).

### Что нужно браузеру

- **Интернет для карты.** Подложка грузится с OpenFreeMap (векторная, подписи на русском);
  без WebGL — растровая Stadia Maps. Без сети карта серая, остальное работает.
- **Stadia без ключа отдаёт тайлы только для `localhost`.** Для сайта в интернете домен
  регистрируется в кабинете Stadia (бесплатно); векторная карта OpenFreeMap ключа не требует.

### Если не запускается

| Сообщение | Что сделать |
|---|---|
| `DATABASE_URL не задана` | нет `backend/.env` или сервер запущен не из папки `backend` |
| `адрес :8080 уже занят` | сервер уже запущен в другом терминале: `lsof -i tcp:8080`, затем `kill <PID>` |
| `permission denied for table ...` | схема загружена не тем пользователем — шаг 1 |
| изменения в Go не видны | перезапустить сервер; изменения стилей и скриптов видны после обновления страницы |

## Тесты

| Уровень | Где | Что проверяет | Нужна БД |
|---|---|---|---|
| Unit | `internal/*/..._test.go` | расчёт стоимости и списаний, деньги, валидация, JWT, роли, ограничение входа, страницы | нет |
| Интеграционные | `internal/services/*_integration_test.go` | сервисы и SQL на реальной PostgreSQL, срок бронирования, гонки (одна машина двум клиентам, смена статуса или роли во время бронирования) | `TEST_DATABASE_URL` |
| Сквозные (e2e) | `e2e/` | весь API по HTTP: матрица доступа ролей, сценарии гостя, клиента, менеджера и администратора, единый формат ошибок, SQL-инъекции, защита входа, совпадение `/api/stats` с примером в `docs/api.md` | `E2E_DATABASE_URL` |

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
в имени которой есть `test`. Интеграционные тесты удаляют за собой созданные ими записи; кроме того,
они сдвигают время вперёд, поэтому тестовые бронирования из `seed.sql` в этой базе отменяются как просроченные.

Ограничения целостности БД (на свежих тестовых данных; каждый тест откатывается):

```sh
psql -h localhost -U carsharing -d carsharing -f db/checks.sql
```
