# Flowboard — Kanban (Go + PostgreSQL + React/TS)

```
backend/   Go, Clean Architecture: chi (HTTP), uber/fx (DI), PostgreSQL (pgx + goose),
           Redis (сесії), OpenSearch (пошук карток), RustFS/S3 (аватарки)
frontend/  Vite + React + TypeScript + react-router: лендінг "/", вхід/реєстрація, проєкти "/projects/:id"
```

## Архітектура бекенду

Залежності спрямовані лише всередину: `transport → usecase → domain`, адаптери реалізують порти use case-ів.

```
backend/
  cmd/server/main.go           точка входу: app.New().Run()
  internal/
    domain/                    сутності (User, Project, Column, Card…) і бізнес-помилки; без залежностей
    usecase/                   бізнес-логіка; кожен пакет оголошує свої порти (інтерфейси)
      auth/                      реєстрація, вхід, сесії
      user/                      користувачі, аватарки
      project/                   проєкти й учасники
      board/                     колонки, картки, пошук
    adapter/                   реалізації портів
      postgres/                  репозиторії, транзакції, міграції goose (migrations/*.sql), демо-дані
      redisstore/                сесії: session:<sha256(token)> → userId з TTL
      searchindex/               індекс карток в OpenSearch (edge n-gram, пошук за початком слова)
      s3store/                   аватарки в RustFS через AWS SDK v2
      bcrypthash/                хешування паролів
    transport/httpapi/         chi-роутер, middleware, DTO, обробники по модулях
    config/                    налаштування з оточення та ../.env
    app/                       composition root: збирає всі fx-модулі
```

Кожен пакет експортує свій `fx.Module`. Адаптер прив'язує себе до портів, наприклад
`func(r *CardRepo) board.Cards { return r }`. HTTP-обробники додаються в групу `routes`,
і сервер монтує їх усі. Щоб замінити реалізацію, наприклад пошук, достатньо підключити інший модуль в `internal/app`.

Postgres — джерело правди. OpenSearch лише повертає id карток, що відповідають запиту.
Сортування, фільтри й курсорна пагінація виконуються в Postgres. Під час старту індекс звіряється
з базою і перебудовується, якщо кількість документів не збігається.

## Запуск

Спершу підніміть інфраструктуру: `make infra-up` (див. нижче). Бекенд бере паролі з того ж `.env`.

**Розробка** (два термінали):
```bash
make dev-backend    # API на :8080
make dev-frontend   # http://localhost:5173 (проксі /api -> :8080)
```

**Продакшн** (один бінарник віддає і API, і SPA):
```bash
make run            # http://localhost:8080
```

При першому запуску створюються демо-дані: 2 проєкти (KAN, MOB) і 5 користувачів.
Вхід: `demo@example.com` / `demo1234` — власник обох проєктів.
Також `olena@`, `andrii@`, `maria@`, `taras@example.com` з тим самим паролем
(`taras` не входить у жоден проєкт — зручно перевірити додавання учасника).

Схема Postgres створюється міграціями goose під час старту. Дані зі старої SQLite (`backend/kanban.db`)
не переносяться, тому файл можна видалити.
У демо-проєкті KAN 48 карток у першій колонці — щоб було видно нескінченний скрол.

## Інфраструктура (Docker)

`compose.yaml` піднімає PostgreSQL, Redis, OpenSearch, RustFS і Redpanda. Дані для входу лежать у `.env`, адреси й рядки підключення — у [credentials.md](credentials.md).
```bash
make infra-up       # docker compose up -d
make infra-down     # docker compose down
```
Бекенд використовує PostgreSQL, Redis, OpenSearch і RustFS. Redpanda поки не використовується.

## Можливості
- Кілька проєктів, у кожного своя дошка і ключ задач (`KAN-1`, `MOB-3`), перемикання через сайдбар
- Учасники проєкту: власник додає/видаляє людей, учасник може вийти сам; виконавцем може бути лише учасник
- Видалити картку може автор або власник проєкту; видалити/перейменувати проєкт — лише власник
- Власні колонки: власник додає, перейменовує (подвійний клік або меню ⋯), переставляє й видаляє колонки
  (картки видаленої колонки переносяться в обрану)
- Нескінченний скрол: кожна колонка підвантажує картки сторінками по 20 (курсорна пагінація);
  пошук (OpenSearch: ключ, назва, опис, за початком слова) і фільтри по людях виконуються на сервері
- Аватарки: клік по своїй аватарці у верхній панелі → завантажити/змінити/видалити фото.
  Браузер обрізає фото до квадрата 256×256, сервер перевіряє тип (PNG/JPEG/WebP/GIF, до 5 МБ)
  і зберігає його в RustFS. Віддається через `/api/users/{id}/avatar?v=…` з кешем «назавжди»
- Дизайн у стилі Liquid Glass (backdrop-filter, напівпрозорі шари) у світлій і темній темах
  (за замовчуванням — системна, вибір зберігається)

### Змінні оточення бекенду
Бекенд читає `../.env` (або файл з `ENV_FILE`). Змінні оточення мають пріоритет.

| Змінна | За замовчуванням | |
|---|---|---|
| `ADDR` | `:8080` | адреса сервера |
| `STATIC_DIR` | `../frontend/dist` | зібраний фронтенд |
| `SEED` | `1` | `0` — не створювати демо-дані в порожній базі |
| `SECURE_COOKIES` | `0` | `1` за HTTPS |
| `DATABASE_URL` | з `POSTGRES_USER/PASSWORD/DB` @ `localhost:5432` | PostgreSQL |
| `REDIS_ADDR`, `REDIS_PASSWORD` | `localhost:6379`, з `.env` | Redis |
| `OPENSEARCH_URL`, `OPENSEARCH_USER`, `OPENSEARCH_PASSWORD` | `https://localhost:9200`, `admin`, з `OPENSEARCH_ADMIN_PASSWORD` | OpenSearch |
| `OPENSEARCH_INSECURE`, `OPENSEARCH_INDEX` | `1`, `flowboard-cards` | не перевіряти самопідписаний сертифікат; назва індексу |
| `S3_ENDPOINT`, `S3_REGION` | `http://localhost:9000`, `us-east-1` | RustFS |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET` | з `RUSTFS_*` | бакет створюється автоматично |

## API
| Метод | Шлях | |
|---|---|---|
| POST | `/api/auth/register` | `{name, email, password}` |
| POST | `/api/auth/login` | `{email, password}` |
| POST | `/api/auth/logout` | |
| GET | `/api/auth/me` | поточний користувач |
| GET | `/api/users` | усі зареєстровані користувачі (з `avatarUrl`) |
| PUT | `/api/users/me/avatar` | `multipart/form-data`, поле `avatar` — PNG/JPEG/WebP/GIF до 5 МБ |
| DELETE | `/api/users/me/avatar` | видалити свою аватарку |
| GET | `/api/users/{id}/avatar?v=` | зображення аватарки |
| GET | `/api/projects` | проєкти поточного користувача |
| POST | `/api/projects` | `{name, key}` — творець стає власником |
| PUT | `/api/projects/{id}` | `{name}` — лише власник |
| DELETE | `/api/projects/{id}` | лише власник |
| GET | `/api/projects/{id}/members` | учасники проєкту |
| POST | `/api/projects/{id}/members` | `{userId}` — лише власник |
| DELETE | `/api/projects/{id}/members/{userId}` | власник — будь-кого, учасник — себе |
| GET | `/api/projects/{id}/columns` | колонки проєкту |
| POST | `/api/projects/{id}/columns` | `{name}` — лише власник |
| PUT | `/api/columns/{id}` | `{name}` — лише власник |
| POST | `/api/columns/{id}/move` | `{index}` — лише власник |
| DELETE | `/api/columns/{id}?moveTo={columnId}` | лише власник; `moveTo` обов'язковий, якщо в колонці є картки |
| GET | `/api/projects/{id}/cards?column=&cursor=&limit=&q=&assignees=1,2&unassigned=1` | сторінка карток колонки → `{cards, nextCursor, total}` |
| POST | `/api/projects/{id}/cards` | `{title, description, columnId, priority, assigneeId}` |
| PUT | `/api/cards/{id}` | `{title, description, priority, assigneeId}` |
| POST | `/api/cards/{id}/move` | `{columnId, beforeId?}` або `{columnId, afterId?}`; без них — в кінець колонки |
| DELETE | `/api/cards/{id}` | автор картки або власник проєкту |
