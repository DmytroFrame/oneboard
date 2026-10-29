# Дані для входу — локальна інфраструктура

Сервіси описані в `compose.yaml`, значення беруться з `.env`. Якщо змінюєте пароль, змінюйте його в `.env` і тут.

> Це паролі **лише для локальної розробки**. Не комітьте `.env` і цей файл у публічний репозиторій і не використовуйте ці значення на продакшені.

```bash
docker compose up -d      # або: make infra-up
docker compose ps         # усі сервіси мають бути healthy
docker compose down       # зупинити (дані залишаться у volume)
docker compose down -v    # зупинити й видалити всі дані
```

Паролі, логіни та назви бази й бакета застосовуються лише при **першому** старті, коли volume ще порожній. Щоб нові значення з `.env` підхопилися, виконайте `docker compose down -v`.

## PostgreSQL

| Параметр | Значення |
|---|---|
| Хост | `localhost:5432` (у мережі compose: `postgres:5432`) |
| База | `flowboard` |
| Користувач | `flowboard` |
| Пароль | `H6pQsOiyVAmSNOgXFntI` |

```
postgres://flowboard:H6pQsOiyVAmSNOgXFntI@localhost:5432/flowboard?sslmode=disable
```

```bash
docker compose exec postgres psql -U flowboard -d flowboard
```

## Redis

| Параметр | Значення |
|---|---|
| Хост | `localhost:6379` (у мережі compose: `redis:6379`) |
| Користувач | `default` |
| Пароль | `8QsvjmiPYvIVmiTyXcnx` |

```
redis://default:8QsvjmiPYvIVmiTyXcnx@localhost:6379/0
```

```bash
docker compose exec redis redis-cli ping   # пароль уже передано через REDISCLI_AUTH
```

## OpenSearch

| Параметр | Значення |
|---|---|
| REST API | `https://localhost:9200` (самопідписаний сертифікат) |
| Performance Analyzer | `http://localhost:9600` |
| Користувач | `admin` |
| Пароль | `Os-5nkp3vajorWQFIn7gRl9!` |

```bash
curl -k -u 'admin:Os-5nkp3vajorWQFIn7gRl9!' https://localhost:9200/_cluster/health
```

Сертифікат самопідписаний, тому клієнту треба вимкнути перевірку TLS (`-k` / `InsecureSkipVerify`). Для OpenSearch потрібно щонайменше 4 ГБ пам'яті в Docker.

## RustFS (S3)

| Параметр | Значення |
|---|---|
| S3 API | `http://localhost:9000` (у мережі compose: `http://rustfs:9000`) |
| Веб-консоль | http://localhost:9001 |
| Access key (логін) | `flowboard` |
| Secret key (пароль) | `iKKMRSsrdb70Xv6gP9mL` |
| Бакет | `flowboard` (створюється автоматично сервісом `rustfs-init`) |
| Регіон | `us-east-1` |

Для S3-клієнтів увімкніть path-style адресацію (`UsePathStyle` / `forcePathStyle`).

## Redpanda (Kafka API)

| Параметр | Значення |
|---|---|
| Kafka bootstrap | `localhost:19092` (у мережі compose: `redpanda:9092`) |
| SASL механізм | `SCRAM-SHA-256` (без TLS, `SASL_PLAINTEXT`) |
| Користувач | `flowboard` (суперкористувач) |
| Пароль | `vfxpjWzVpj5YsOJvhG9n` |
| Schema Registry | `http://localhost:18081` |
| HTTP Proxy | `http://localhost:18082` |
| Admin API | `http://localhost:19644` |

```bash
docker compose exec redpanda rpk topic create demo \
  -X user=flowboard -X pass=vfxpjWzVpj5YsOJvhG9n -X sasl.mechanism=SCRAM-SHA-256
```
