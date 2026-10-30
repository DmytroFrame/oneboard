.PHONY: dev-backend dev-frontend build run test infra-up infra-down infra-logs docker-build

# Dev: `make infra-up` first, then run both in separate terminals, open http://localhost:5173
dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

# Prod: build SPA and serve everything from the Go binary on :8080
build:
	cd frontend && npm install && npm run build
	cd backend && go build -o bin/server ./cmd/server

run: build
	cd backend && ./bin/server

test:
	cd backend && go vet ./... && go test ./...
	cd frontend && npm run build

# Production image (API + SPA) for AWS, see docs/aws.md
docker-build:
	docker build -t oneboard .

# Local infrastructure: Postgres, Redis, OpenSearch, RustFS, Redpanda (credentials: credentials.md)
infra-up:
	docker compose up -d

infra-down:
	docker compose down

infra-logs:
	docker compose logs -f
