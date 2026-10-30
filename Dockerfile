# syntax=docker/dockerfile:1.7
# One image with the API and the built SPA, for ECS Fargate / App Runner / EKS.
#   docker build -t oneboard .
#   docker run -p 8080:8080 --env-file .env.aws oneboard

FROM node:24-alpine AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-alpine AS api
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/oneboard ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=api /out/oneboard /app/oneboard
COPY --from=web /src/frontend/dist /app/web
# CA bundle to verify Amazon RDS certificates (sslmode=verify-full).
ADD --chmod=444 https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem /etc/oneboard/rds-global-bundle.pem
ENV APP_ENV=production \
    ADDR=:8080 \
    STATIC_DIR=/app/web
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/oneboard"]
