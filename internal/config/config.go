// internal/config/config.go
package config

import (
	"os"
	"strings"
)

type Config struct {
	Port        string
	DatabaseURL string

	// OIDC
	OIDCIssuer   string
	OIDCClientID string
	OIDCJWKSURL  string

	// MinIO
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool

	// eguwallet QTSP
	QTSPBaseURL    string
	QTSPServiceKey string

	// Gotenberg
	GotenbergURL string

	// CORS
	FrontendURL string

	// Misc
	LogLevel string
	IsProd   bool
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8090"),
		DatabaseURL:    mustGetEnv("DATABASE_URL"),
		OIDCIssuer:     mustGetEnv("OIDC_ISSUER"),
		OIDCClientID:   getEnv("OIDC_CLIENT_ID", "egudoc-spa"),
		OIDCJWKSURL:    mustGetEnv("OIDC_JWKS_URL"),
		MinioEndpoint:  mustGetEnv("MINIO_ENDPOINT"),
		MinioAccessKey: mustGetEnv("MINIO_ACCESS_KEY"),
		MinioSecretKey: mustGetEnv("MINIO_SECRET_KEY"),
		MinioBucket:    getEnv("MINIO_BUCKET_DOCUMENTS", "egudoc-documents"),
		MinioUseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",
		QTSPBaseURL:    mustGetEnv("QTSP_BASE_URL"),
		QTSPServiceKey: mustGetEnv("QTSP_SERVICE_KEY"),
		GotenbergURL:   getEnv("GOTENBERG_URL", "http://gotenberg:3000"),
		FrontendURL:    getEnv("FRONTEND_URL", "http://localhost:4200"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		IsProd:         strings.EqualFold(getEnv("NODE_ENV", "development"), "production"),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required environment variable not set: " + key)
	}
	return v
}
