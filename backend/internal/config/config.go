// Package config loads settings from the environment. Values not set explicitly are derived
// from the same .env that docker compose uses, so the backend and the containers share credentials.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/fx"
)

var Module = fx.Module("config", fx.Provide(Load))

type Config struct {
	Addr          string
	StaticDir     string
	Seed          bool
	SecureCookies bool

	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	DatabaseURL string

	RedisAddr     string
	RedisPassword string

	OpenSearchURL      string
	OpenSearchUser     string
	OpenSearchPassword string
	OpenSearchInsecure bool // the dev cluster uses a self-signed certificate
	OpenSearchIndex    string

	S3Endpoint  string
	S3Region    string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
}

func Load() (Config, error) {
	// Real environment variables win; ENV_FILE (default ../.env, then .env) only fills the gaps.
	files := []string{"../.env", ".env"}
	if f := os.Getenv("ENV_FILE"); f != "" {
		files = []string{f}
	}
	for _, f := range files {
		if _, err := os.Stat(f); err == nil {
			if err := godotenv.Load(f); err != nil {
				return Config{}, fmt.Errorf("load %s: %w", f, err)
			}
		}
	}

	pgURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(env("POSTGRES_USER", "flowboard"), env("POSTGRES_PASSWORD", "")),
		Host:     env("POSTGRES_HOST", "localhost:5432"),
		Path:     env("POSTGRES_DB", "flowboard"),
		RawQuery: "sslmode=disable",
	}

	accessTTL, err := time.ParseDuration(env("ACCESS_TOKEN_TTL", "15m"))
	if err != nil {
		return Config{}, fmt.Errorf("ACCESS_TOKEN_TTL: %w", err)
	}
	refreshTTL, err := time.ParseDuration(env("REFRESH_TOKEN_TTL", "720h"))
	if err != nil {
		return Config{}, fmt.Errorf("REFRESH_TOKEN_TTL: %w", err)
	}
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return Config{}, errors.New("JWT_SECRET must be set to at least 32 characters (see .env)")
	}

	c := Config{
		Addr:          env("ADDR", ":8080"),
		StaticDir:     env("STATIC_DIR", "../frontend/dist"),
		Seed:          env("SEED", "1") == "1",
		SecureCookies: env("SECURE_COOKIES", "0") == "1",

		JWTSecret:       []byte(secret),
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,

		DatabaseURL: env("DATABASE_URL", pgURL.String()),

		RedisAddr:     env("REDIS_ADDR", "localhost:6379"),
		RedisPassword: env("REDIS_PASSWORD", ""),

		OpenSearchURL:      env("OPENSEARCH_URL", "https://localhost:9200"),
		OpenSearchUser:     env("OPENSEARCH_USER", "admin"),
		OpenSearchPassword: env("OPENSEARCH_PASSWORD", env("OPENSEARCH_ADMIN_PASSWORD", "")),
		OpenSearchInsecure: env("OPENSEARCH_INSECURE", "1") == "1",
		OpenSearchIndex:    env("OPENSEARCH_INDEX", "flowboard-cards"),

		S3Endpoint:  env("S3_ENDPOINT", "http://localhost:9000"),
		S3Region:    env("S3_REGION", "us-east-1"),
		S3AccessKey: env("S3_ACCESS_KEY", env("RUSTFS_ACCESS_KEY", "")),
		S3SecretKey: env("S3_SECRET_KEY", env("RUSTFS_SECRET_KEY", "")),
		S3Bucket:    env("S3_BUCKET", env("RUSTFS_BUCKET", "flowboard")),
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
