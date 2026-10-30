# Deploying OneBoard to AWS

The app is a single stateless container (API + built SPA). All data lives in managed services, so you can
run as many instances as you need.

| Local (`compose.yaml`) | AWS | Notes |
|---|---|---|
| PostgreSQL 17 | **Amazon RDS for PostgreSQL 17** (or Aurora PostgreSQL) | TLS `verify-full`; the RDS CA bundle is in the image |
| RustFS | **Amazon S3** | accessed with the task's IAM role, no keys |
| OpenSearch 2.19 | **Amazon OpenSearch Service** (2.x) | SigV4 signing with the IAM role, or a master user |
| Redis 7.4 | **Amazon ElastiCache** (Valkey or Redis OSS ≥ 7) | **cluster mode disabled**, in-transit encryption |
| coturn | **coturn on EC2** (public IP) or a managed TURN service | voice rooms; credentials from `TURN_SECRET` |
| Redpanda | — | not used by the app yet |
| `make run` | **ECS Fargate** behind an **Application Load Balancer** | image from the `Dockerfile` |

## What the code already handles

- **`APP_ENV=production`** switches the defaults to production:
  - TLS to RDS and Redis;
  - AWS credentials from the SDK's default chain, i.e. the ECS task role;
  - secure cookies;
  - JSON logs for CloudWatch;
  - no demo data;
  - `OPENSEARCH_INSECURE` is refused.

  All variables are listed in [`.env.aws.example`](../.env.aws.example).
- **Migrations** run on start under a Postgres advisory lock (goose session locker), so several tasks can
  start at once during a rolling deploy without racing.
- **Health checks:**
  - `GET /healthz` — liveness, the process is up;
  - `GET /readyz` — readiness: checks Postgres and Redis, returns 503 with details on failure.

  Use `/readyz` for the ALB target group.
- **S3:** without `S3_ENDPOINT` the client talks to real S3 with virtual-hosted addressing. The bucket is
  not created on start (`S3_CREATE_BUCKET=0`); the app only checks that it exists.
- **OpenSearch:** without a password, requests are SigV4-signed (`es` or `aoss`). `OPENSEARCH_REPLICAS`
  sets the index replica count (1 by default in production). The index is created and rebuilt from
  Postgres automatically on start.
- **Board presence doesn't need `HEXPIRE`:** a hash plus a sorted set with Lua cleanup works on any
  ElastiCache version.
- **`Dockerfile`:** distroless image, non-root user, RDS CA bundle at `/etc/oneboard/rds-global-bundle.pem`.

## Infrastructure

1. **VPC:** private subnets for RDS, ElastiCache, OpenSearch and ECS; public subnets for the ALB.
2. **RDS:** PostgreSQL 17 with database and user `oneboard`. The security group allows 5432 only from the
   ECS tasks' security group.
3. **ElastiCache:** Valkey 8 or Redis OSS 7.x, **cluster mode disabled** (the refresh-token rotation Lua
   scripts touch keys they compute, which must live in one shard), Multi-AZ with replicas, in-transit
   encryption. Put the AUTH token or RBAC user into `REDIS_PASSWORD` / `REDIS_USERNAME`.
4. **OpenSearch Service:** a domain inside the VPC with HTTPS enforced. With fine-grained access control,
   map the ECS task role as a backend role of a role that can manage the `oneboard-cards` index, or use a
   master user (`OPENSEARCH_AUTH=basic`).
5. **S3:** a private bucket (Block Public Access on), SSE-S3 or SSE-KMS encryption. The app serves files
   through signed links, so no public access is needed.
6. **ECS Fargate:** a service with 2+ tasks.
   - Port 8080, ALB target group with the `/readyz` health check.
   - ALB idle timeout ≥ 120 s: the server pings WebSockets every 30 s, so connections stay open.
   - No sticky sessions needed: all tasks exchange events through Redis pub/sub.
7. **Secrets:** `JWT_SECRET`, `POSTGRES_PASSWORD`, `REDIS_PASSWORD` in Secrets Manager, injected as
   environment variables through `secrets` in the task definition.
8. **HTTPS:** an ACM certificate on the ALB, redirect 80 → 443.
9. **TURN for voice rooms:** audio goes browser to browser and never through the ALB, but people behind
   strict NATs need a relay. Run coturn on a small EC2 instance with an Elastic IP: `--use-auth-secret`
   with `--static-auth-secret` = `TURN_SECRET` (from Secrets Manager), `--external-ip` = the Elastic IP,
   and a security group open for 3478 UDP/TCP (plus 5349 TCP for `turns:` with a certificate) and the
   relay port range (e.g. 49152–65535 UDP). Set `STUN_URLS`, `TURN_URLS` and `TURN_SECRET` on the tasks.
   Any TURN service supporting the TURN REST API credential format works the same way.

### ECS task role IAM policy

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "Files",
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"],
      "Resource": "arn:aws:s3:::oneboard-files-prod/*"
    },
    {
      "Sid": "FilesBucket",
      "Effect": "Allow",
      "Action": ["s3:ListBucket"],
      "Resource": "arn:aws:s3:::oneboard-files-prod"
    },
    {
      "Sid": "Search",
      "Effect": "Allow",
      "Action": ["es:ESHttpGet", "es:ESHttpHead", "es:ESHttpPost", "es:ESHttpPut", "es:ESHttpDelete"],
      "Resource": "arn:aws:es:eu-central-1:123456789012:domain/oneboard/*"
    }
  ]
}
```

`s3:ListBucket` is needed for the bucket check on start (`HeadBucket`) and to get proper 404s instead of
403s. With SSE-KMS, also allow `kms:GenerateDataKey` and `kms:Decrypt` on the key.

## Moving data from the local environment

```bash
# Postgres → RDS
docker compose exec postgres pg_dump -U oneboard -Fc oneboard > oneboard.dump
pg_restore --no-owner -h <rds-endpoint> -U oneboard -d oneboard oneboard.dump

# RustFS → S3 (object keys stored in the database don't change)
aws --endpoint-url http://localhost:9000 s3 sync s3://oneboard ./files   # with the RustFS keys
aws s3 sync ./files s3://oneboard-files-prod
```

OpenSearch doesn't need to be moved: the index is rebuilt from Postgres on the first start. Neither does
Redis: it only holds sessions and short-lived data, so users simply sign in again after the move.

## Still to do

- **Infrastructure as code** (Terraform / CDK) for the setup above — not in the repository yet.
- **Serving files straight from S3** with presigned URLs or CloudFront. Today the app streams files
  itself, which is unnecessary load at scale.
- **CI/CD:** build and push the image to ECR (`docker build` + `docker push`), then
  `aws ecs update-service`.
