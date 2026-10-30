// Package config loads settings from the environment. Values not set explicitly are derived
// from the same .env that docker compose uses, so the backend and the containers share credentials.
//
// APP_ENV picks the defaults: "development" (local docker compose: RustFS, self-signed OpenSearch,
// no TLS, demo data) or "production" (AWS: RDS over verified TLS, S3 and OpenSearch with IAM
// credentials, secure cookies, JSON logs, no demo data). Every default can be overridden.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/fx"
)

var Module = fx.Module("config", fx.Provide(Load))

type Config struct {
	Env       string // "development" or "production"
	LogFormat string // "text" or "json"
	Addr      string // plain HTTP listen address, used when no TLS certificate is configured
	StaticDir string

	// HTTPS with your own certificate (e.g. Let's Encrypt). When both files exist the server listens on
	// HTTPSAddr and HTTPAddr only redirects to HTTPS (and serves ACME challenges from ACMEWebroot).
	TLSCertFile   string
	TLSKeyFile    string
	TLSEnabled    bool   // both files were found
	TLSProblem    string // why TLS is off although files were configured ("" = not configured or enabled)
	HTTPSAddr     string
	HTTPAddr      string
	ACMEWebroot   string
	Seed          bool
	SecureCookies bool

	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	DatabaseURL string

	RedisAddr     string
	RedisUsername string // ElastiCache RBAC user (optional)
	RedisPassword string
	RedisDB       int
	RedisTLS      bool

	AWSRegion string

	OpenSearchURL      string
	OpenSearchAuth     string // "basic" (user/password), "aws" (SigV4 with IAM credentials) or "none"
	OpenSearchService  string // SigV4 service: "es" (OpenSearch Service domain) or "aoss" (Serverless)
	OpenSearchUser     string
	OpenSearchPassword string
	OpenSearchInsecure bool // skip TLS verification: only for the local self-signed cluster
	OpenSearchIndex    string
	OpenSearchReplicas int

	S3Endpoint       string // "" = AWS S3
	S3Region         string
	S3AccessKey      string // "" = default AWS credential chain (IAM role, env, profile)
	S3SecretKey      string
	S3Bucket         string
	S3ForcePathStyle bool
	S3CreateBucket   bool // create the bucket on start if missing (local only: on AWS it is provisioned)
}

func (c Config) Production() bool { return c.Env == "production" }

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

	appEnv := env("APP_ENV", "development")
	if appEnv != "development" && appEnv != "production" {
		return Config{}, fmt.Errorf("APP_ENV must be development or production, got %q", appEnv)
	}
	prod := appEnv == "production"
	// pick returns the development or the production default.
	pick := func(dev, production string) string {
		if prod {
			return production
		}
		return dev
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
	redisDB, err := strconv.Atoi(env("REDIS_DB", "0"))
	if err != nil {
		return Config{}, fmt.Errorf("REDIS_DB: %w", err)
	}
	replicas, err := strconv.Atoi(env("OPENSEARCH_REPLICAS", pick("0", "1")))
	if err != nil {
		return Config{}, fmt.Errorf("OPENSEARCH_REPLICAS: %w", err)
	}

	// RDS: verify the server certificate against the AWS RDS CA bundle (shipped in the Docker image).
	pgQuery := url.Values{"sslmode": {env("POSTGRES_SSLMODE", pick("disable", "verify-full"))}}
	if root := env("POSTGRES_SSLROOTCERT", pick("", "/etc/oneboard/rds-global-bundle.pem")); root != "" {
		pgQuery.Set("sslrootcert", root)
	}
	pgURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(env("POSTGRES_USER", "oneboard"), env("POSTGRES_PASSWORD", "")),
		Host:     env("POSTGRES_HOST", "localhost:5432"),
		Path:     env("POSTGRES_DB", "oneboard"),
		RawQuery: pgQuery.Encode(),
	}

	certFile, keyFile := os.Getenv("TLS_CERT_FILE"), os.Getenv("TLS_KEY_FILE")
	tlsEnabled, tlsProblem := false, ""
	if certFile != "" || keyFile != "" {
		switch {
		case certFile == "" || keyFile == "":
			tlsProblem = "both TLS_CERT_FILE and TLS_KEY_FILE must be set"
		case !fileExists(certFile):
			tlsProblem = "certificate not found: " + certFile
		case !fileExists(keyFile):
			tlsProblem = "key not found: " + keyFile
		default:
			tlsEnabled = true
		}
	}
	secureDefault := pick("0", "1")
	if tlsEnabled {
		secureDefault = "1"
	}

	region := env("AWS_REGION", env("AWS_DEFAULT_REGION", "us-east-1"))
	s3Endpoint := env("S3_ENDPOINT", pick("http://localhost:9000", ""))
	osPassword := env("OPENSEARCH_PASSWORD", env("OPENSEARCH_ADMIN_PASSWORD", ""))
	osAuth := "basic"
	if prod && osPassword == "" {
		osAuth = "aws" // no master user configured: sign requests with the task's IAM role
	}

	c := Config{
		Env:           appEnv,
		LogFormat:     env("LOG_FORMAT", pick("text", "json")),
		Addr:          env("ADDR", ":8080"),
		StaticDir:     env("STATIC_DIR", "../frontend/dist"),
		Seed:          env("SEED", pick("1", "0")) == "1",
		SecureCookies: env("SECURE_COOKIES", secureDefault) == "1",

		TLSCertFile: certFile,
		TLSKeyFile:  keyFile,
		TLSEnabled:  tlsEnabled,
		TLSProblem:  tlsProblem,
		HTTPSAddr:   env("HTTPS_ADDR", ":443"),
		HTTPAddr:    env("HTTP_ADDR", ":80"),
		ACMEWebroot: env("ACME_WEBROOT", ""),

		JWTSecret:       []byte(secret),
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,

		DatabaseURL: env("DATABASE_URL", pgURL.String()),

		RedisAddr:     env("REDIS_ADDR", "localhost:6379"),
		RedisUsername: env("REDIS_USERNAME", ""),
		RedisPassword: env("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,
		RedisTLS:      env("REDIS_TLS", pick("0", "1")) == "1",

		AWSRegion: region,

		OpenSearchURL:      env("OPENSEARCH_URL", "https://localhost:9200"),
		OpenSearchAuth:     env("OPENSEARCH_AUTH", osAuth),
		OpenSearchService:  env("OPENSEARCH_SERVICE", "es"),
		OpenSearchUser:     env("OPENSEARCH_USER", "admin"),
		OpenSearchPassword: osPassword,
		OpenSearchInsecure: env("OPENSEARCH_INSECURE", pick("1", "0")) == "1",
		OpenSearchIndex:    env("OPENSEARCH_INDEX", "oneboard-cards"),
		OpenSearchReplicas: replicas,

		S3Endpoint:       s3Endpoint,
		S3Region:         env("S3_REGION", region),
		S3AccessKey:      env("S3_ACCESS_KEY", pick(env("RUSTFS_ACCESS_KEY", ""), "")),
		S3SecretKey:      env("S3_SECRET_KEY", pick(env("RUSTFS_SECRET_KEY", ""), "")),
		S3Bucket:         env("S3_BUCKET", env("RUSTFS_BUCKET", "oneboard")),
		S3ForcePathStyle: env("S3_FORCE_PATH_STYLE", boolStr(s3Endpoint != "")) == "1",
		S3CreateBucket:   env("S3_CREATE_BUCKET", pick("1", "0")) == "1",
	}

	switch c.OpenSearchAuth {
	case "basic", "aws", "none":
	default:
		return Config{}, fmt.Errorf("OPENSEARCH_AUTH must be basic, aws or none, got %q", c.OpenSearchAuth)
	}
	if c.Production() && c.OpenSearchInsecure {
		return Config{}, errors.New("OPENSEARCH_INSECURE=1 is not allowed in production")
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// fileExists also follows symlinks, as Let's Encrypt's live/ directory consists of them.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
