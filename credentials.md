# Credentials — local infrastructure

The services are defined in `compose.yaml`; the values come from `.env`. If you change a password,
change it in `.env` and here.

> These passwords are **for local development only**. Don't commit `.env` or this file to a public
> repository, and never use these values in production.

```bash
docker compose up -d      # or: make infra-up
docker compose ps         # every service should be healthy
docker compose down       # stop (data stays in the volumes)
docker compose down -v    # stop and delete all data
```

Passwords, logins and the database and bucket names only take effect on the **first** start, while the
volume is empty. To apply new values from `.env`, run `docker compose down -v`.

## PostgreSQL

| Setting | Value |
|---|---|
| Host | `localhost:5432` (inside the compose network: `postgres:5432`) |
| Database | `oneboard` |
| User | `oneboard` |
| Password | `H6pQsOiyVAmSNOgXFntI` |

```
postgres://oneboard:H6pQsOiyVAmSNOgXFntI@localhost:5432/oneboard?sslmode=disable
```

```bash
docker compose exec postgres psql -U oneboard -d oneboard
```

## Redis

| Setting | Value |
|---|---|
| Host | `localhost:6379` (inside the compose network: `redis:6379`) |
| User | `default` |
| Password | `8QsvjmiPYvIVmiTyXcnx` |

```
redis://default:8QsvjmiPYvIVmiTyXcnx@localhost:6379/0
```

```bash
docker compose exec redis redis-cli ping   # the password is already passed via REDISCLI_AUTH
```

## OpenSearch

| Setting | Value |
|---|---|
| REST API | `https://localhost:9200` (self-signed certificate) |
| Performance Analyzer | `http://localhost:9600` |
| User | `admin` |
| Password | `Os-5nkp3vajorWQFIn7gRl9!` |

```bash
curl -k -u 'admin:Os-5nkp3vajorWQFIn7gRl9!' https://localhost:9200/_cluster/health
```

The certificate is self-signed, so clients must skip TLS verification (`-k` / `InsecureSkipVerify`).
OpenSearch needs at least 4 GB of memory available to Docker.

## RustFS (S3)

| Setting | Value |
|---|---|
| S3 API | `http://localhost:9000` (inside the compose network: `http://rustfs:9000`) |
| Web console | http://localhost:9001 |
| Access key | `oneboard` |
| Secret key | `iKKMRSsrdb70Xv6gP9mL` |
| Bucket | `oneboard` (created automatically by the `rustfs-init` service) |
| Region | `us-east-1` |

S3 clients need path-style addressing (`UsePathStyle` / `forcePathStyle`).

## Redpanda (Kafka API)

| Setting | Value |
|---|---|
| Kafka bootstrap | `localhost:19092` (inside the compose network: `redpanda:9092`) |
| SASL mechanism | `SCRAM-SHA-256` (no TLS, `SASL_PLAINTEXT`) |
| User | `oneboard` (superuser) |
| Password | `vfxpjWzVpj5YsOJvhG9n` |
| Schema Registry | `http://localhost:18081` |
| HTTP Proxy | `http://localhost:18082` |
| Admin API | `http://localhost:19644` |

```bash
docker compose exec redpanda rpk topic create demo \
  -X user=oneboard -X pass=vfxpjWzVpj5YsOJvhG9n -X sasl.mechanism=SCRAM-SHA-256
```

## coturn (TURN/STUN for voice rooms)

| Setting | Value |
|---|---|
| STUN / TURN | `localhost:3478` (UDP and TCP), relay ports `49160–49200/udp` |
| Auth | `use-auth-secret`: no fixed users; the backend issues short-lived credentials (`GET /api/voice/ice`) |
| Shared secret | `TURN_SECRET` in `.env` |

Relay addresses are advertised as `127.0.0.1`, so locally only browsers on this machine can use the relay.

## JWT

| Setting | Value |
|---|---|
| Signing secret (HS256) | `JWT_SECRET` in `.env` (64 random characters) |
| Access token | 15 min (`ACCESS_TOKEN_TTL`) |
| Refresh token | 30 days (`REFRESH_TOKEN_TTL`), cookie `refresh_token` |

Changing `JWT_SECRET` invalidates every session and every signed avatar/file URL.

```bash
TOKEN=$(curl -s -X POST localhost:8080/api/auth/login -d '{"email":"demo@example.com","password":"demo1234"}' | jq -r .accessToken)
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/projects
```
