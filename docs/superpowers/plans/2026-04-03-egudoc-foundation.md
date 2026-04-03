# EguDoc — Sub-plan A: Foundation & Infrastructure

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bootstrap the EguDoc Go mono-repo with full project structure, PostgreSQL schema, MinIO storage client, eguilde OIDC JWT validation middleware, granular RBAC system, and Kubernetes deployment skeleton — everything that subsequent sub-plans depend on.

**Architecture:** Single Go binary (`github.com/eguilde/egudoc`) using Chi v5 router, pgx v5 PostgreSQL driver, and MinIO Go SDK. Authentication delegates entirely to the existing eguilde OIDC provider — EguDoc is a resource server that validates JWTs and extracts claims. RBAC is enforced at the middleware layer using a `permissions` table with action+subject+condition triples.

**Tech Stack:** Go 1.24, Chi v5, pgx/v5, minio-go v7, go-jose/v4 (JWT), golang-migrate, Docker Compose (local), Kubernetes (prod)

---

## File Map

```
egudoc/
├── go.mod                                     # module github.com/eguilde/egudoc
├── go.sum
├── .env.example
├── Makefile
├── cmd/
│   └── server/
│       └── main.go                            # Entry point: wire all services, start HTTP server
├── internal/
│   ├── config/
│   │   └── config.go                         # All env vars loaded via os.Getenv with defaults
│   ├── database/
│   │   ├── database.go                       # pgx pool factory, health check
│   │   └── schema.go                         # EnsureSchema() - applies migrations on startup
│   ├── storage/
│   │   └── minio.go                          # MinIO client wrapper: Put, Get, Delete, PresignedURL
│   ├── auth/
│   │   ├── claims.go                         # Claims struct, ParseToken(), JWKS cache
│   │   ├── middleware.go                     # RequireAuth Chi middleware
│   │   └── context.go                       # GetClaims(ctx), GetUserID(ctx) helpers
│   ├── rbac/
│   │   ├── model.go                          # Role, Permission, UserRole structs
│   │   ├── service.go                        # RBACService: HasPermission(), GetUserRoles()
│   │   ├── middleware.go                     # Require(action, subject) Chi middleware factory
│   │   ├── seeder.go                         # SeedRoles() - seeds default roles+permissions
│   │   └── handler.go                        # Admin API: CRUD for roles, permissions, user-roles
│   ├── users/
│   │   ├── model.go                          # User struct (mirrors eguilde token claims + local profile)
│   │   ├── service.go                        # UserService: GetOrCreate(), GetByID(), List()
│   │   └── handler.go                        # GET /api/users/me, GET /api/users (admin)
│   └── health/
│       └── handler.go                        # GET /health, GET /health/ready
├── migrations/
│   ├── 000001_init_rbac.sql                  # roles, permissions, user_roles, role_permissions tables
│   ├── 000002_init_users.sql                 # users, institutions, departments tables
│   └── 000003_init_storage.sql              # stored_files table (MinIO references)
├── k8s/
│   ├── namespace.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── ingress.yaml
│   ├── configmap.yaml
│   ├── secret.yaml
│   └── hpa.yaml
├── docker-compose.yml                        # Local dev: postgres, minio, gotenberg
├── Dockerfile
└── .github/
    └── workflows/
        └── ci.yml
```

---

## Task 1: Go module and project skeleton

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `.env.example`
- Create: `cmd/server/main.go`

- [ ] **Step 1.1: Initialize Go module**

```bash
cd /c/dev/egudoc
go mod init github.com/eguilde/egudoc
```

Expected: `go.mod` created with `module github.com/eguilde/egudoc` and `go 1.24`

- [ ] **Step 1.2: Add core dependencies**

```bash
go get github.com/go-chi/chi/v5@v5.2.1
go get github.com/jackc/pgx/v5@v5.7.2
go get github.com/minio/minio-go/v7@v7.0.87
go get github.com/go-jose/go-jose/v4@v4.0.5
go get github.com/google/uuid@v1.6.0
go get github.com/golang-migrate/migrate/v4@v4.18.1
go get go.uber.org/zap@v1.27.0
go get github.com/go-chi/httprate@v0.15.0
go mod tidy
```

- [ ] **Step 1.3: Create minimal main.go**

```go
// cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/eguilde/egudoc/internal/config"
	"github.com/eguilde/egudoc/internal/database"
	"github.com/eguilde/egudoc/internal/health"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg := config.Load()

	pool, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("database connect failed", zap.Error(err))
	}
	defer pool.Close()

	if err := database.EnsureSchema(context.Background(), pool); err != nil {
		log.Fatal("schema migration failed", zap.Error(err))
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Mount("/", health.NewHandler(pool).Routes())

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("server starting", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Info("server stopped")
}
```

- [ ] **Step 1.4: Create Makefile**

```makefile
# Makefile
.PHONY: build test run migrate lint docker-up docker-down

build:
	go build -o bin/egudoc ./cmd/server

test:
	go test ./... -v -race -count=1

run:
	go run ./cmd/server

migrate:
	migrate -path ./migrations -database "$(DATABASE_URL)" up

lint:
	golangci-lint run ./...

docker-up:
	docker compose up -d

docker-down:
	docker compose down

generate-migration:
	@read -p "Migration name: " name; \
	n=$$(ls migrations/ | grep -c '\.sql' | xargs printf '%06d'); \
	touch migrations/$${n}_$${name}.sql
```

- [ ] **Step 1.5: Create .env.example**

```bash
# .env.example
PORT=8090
DATABASE_URL=postgres://egudoc:egudoc@localhost:5432/egudoc?sslmode=disable

# eguilde OIDC provider
OIDC_ISSUER=http://localhost:3100/api/oidc
OIDC_CLIENT_ID=egudoc-spa
OIDC_JWKS_URL=http://localhost:3100/api/oidc/jwks

# MinIO / S3
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=egudoc
MINIO_SECRET_KEY=egudoc123
MINIO_BUCKET_DOCUMENTS=egudoc-documents
MINIO_USE_SSL=false

# eguwallet QTSP
QTSP_BASE_URL=http://localhost:3220
QTSP_SERVICE_KEY=change-me-in-production

# Gotenberg (PDF/A conversion)
GOTENBERG_URL=http://localhost:3000

# CORS
FRONTEND_URL=http://localhost:4200

# Logging
LOG_LEVEL=info
NODE_ENV=development
```

- [ ] **Step 1.6: Verify build**

```bash
cd /c/dev/egudoc
go build ./cmd/server
```

Expected: `bin/egudoc` produced with no errors.

- [ ] **Step 1.7: Commit**

```bash
cd /c/dev/egudoc
git add go.mod go.sum Makefile .env.example cmd/
git commit -m "feat: initialize Go module with Chi router and minimal server"
```

---

## Task 2: Configuration package

**Files:**
- Create: `internal/config/config.go`

- [ ] **Step 2.1: Write config**

```go
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
	LogLevel  string
	IsProd    bool
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
```

- [ ] **Step 2.2: Commit**

```bash
git add internal/config/
git commit -m "feat: add configuration loader"
```

---

## Task 3: Database package and migrations

**Files:**
- Create: `internal/database/database.go`
- Create: `internal/database/schema.go`
- Create: `migrations/000001_init_rbac.sql`
- Create: `migrations/000002_init_users.sql`
- Create: `migrations/000003_init_storage.sql`

- [ ] **Step 3.1: Write database.go**

```go
// internal/database/database.go
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 25
	cfg.MinConns = 5
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
```

- [ ] **Step 3.2: Write schema.go using golang-migrate**

```go
// internal/database/schema.go
package database

import (
	"context"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed ../../migrations/*.sql
var migrationsFS embed.FS

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	connStr := pool.Config().ConnString()

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migrations source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, connStr)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
```

- [ ] **Step 3.3: Write migration 000001_init_rbac.sql**

```sql
-- migrations/000001_init_rbac.sql
-- +migrate Up

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Roles define named bundles of permissions
CREATE TABLE roles (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code       VARCHAR(100) UNIQUE NOT NULL,    -- e.g. "registrar", "department_head", "admin"
    label      VARCHAR(200) NOT NULL,
    description TEXT,
    system     BOOLEAN NOT NULL DEFAULT FALSE,  -- system roles cannot be deleted
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Permissions: action + subject + optional condition
-- action: "create", "read", "update", "delete", "approve", "assign", "archive", "deliver"
-- subject: "document", "workflow", "registry", "user", "role", "institution", "report", "archive"
-- condition: JSON path constraint e.g. {"compartiment_id": "$user.compartiment_id"}
CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    action      VARCHAR(100) NOT NULL,
    subject     VARCHAR(100) NOT NULL,
    condition   JSONB,   -- NULL means no restriction (applies to all instances)
    description TEXT,
    UNIQUE(action, subject, COALESCE(condition::text, ''))
);

-- Junction: which permissions belong to which roles
CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- Institution-scoped role assignments for users (from eguilde JWT sub claim)
CREATE TABLE user_roles (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_subject   VARCHAR(255) NOT NULL,  -- JWT 'sub' claim from eguilde token
    role_id        UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    institution_id UUID,                   -- NULL = global, non-NULL = institution-scoped
    compartiment_id UUID,                  -- further scoping to a compartiment
    granted_by     VARCHAR(255),           -- subject of the user who granted this
    active         BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_subject, role_id, COALESCE(institution_id::text,''), COALESCE(compartiment_id::text,''))
);

CREATE INDEX idx_user_roles_subject ON user_roles(user_subject) WHERE active = TRUE;
CREATE INDEX idx_user_roles_institution ON user_roles(institution_id) WHERE active = TRUE;

-- +migrate Down
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
```

- [ ] **Step 3.4: Write migration 000002_init_users.sql**

```sql
-- migrations/000002_init_users.sql
-- +migrate Up

-- Local user profile cache (augments eguilde identity)
-- Populated on first login via JWT claims, updated on subsequent logins
CREATE TABLE users (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subject        VARCHAR(255) UNIQUE NOT NULL,  -- JWT 'sub' from eguilde
    email          VARCHAR(255) UNIQUE NOT NULL,
    phone          VARCHAR(50),
    prenume        VARCHAR(200),
    nume           VARCHAR(200),
    avatar_url     VARCHAR(500),
    active         BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at  TIMESTAMPTZ,
    last_login_ip  VARCHAR(45),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);

-- Public institutions (primarii, consilii judetene, ministere, etc.)
CREATE TABLE institutions (
    id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cui                   VARCHAR(20) UNIQUE NOT NULL,   -- Romanian CUI/CIF
    denumire              VARCHAR(500) NOT NULL,
    tip                   VARCHAR(100) NOT NULL,         -- PRIMARIE, CONSILIU_JUDETEAN, MINISTER, etc.
    adresa                TEXT,
    localitate            VARCHAR(200),
    judet                 VARCHAR(100),
    cod_siruta            VARCHAR(10),
    telefon               VARCHAR(50),
    email                 VARCHAR(255),
    website               VARCHAR(500),
    -- eDelivery participant ID for QTSP delivery
    delivery_participant_id VARCHAR(255),
    -- eArchive account ID at QTSP
    archive_account_id    VARCHAR(255),
    active                BOOLEAN NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_institutions_cui ON institutions(cui);

-- Departments (compartimente) within an institution
CREATE TABLE compartimente (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    denumire       VARCHAR(300) NOT NULL,
    cod            VARCHAR(50),    -- short code for the compartiment
    descriere      TEXT,
    parent_id      UUID REFERENCES compartimente(id),   -- hierarchical depts
    active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(institution_id, cod)
);

CREATE INDEX idx_compartimente_institution ON compartimente(institution_id);

-- Link users to institutions and their primary compartiment
CREATE TABLE user_institution_memberships (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_subject     VARCHAR(255) NOT NULL,
    institution_id   UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    compartiment_id  UUID REFERENCES compartimente(id),
    functie          VARCHAR(200),   -- job title
    primary_member   BOOLEAN NOT NULL DEFAULT TRUE,
    active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_subject, institution_id)
);

-- +migrate Down
DROP TABLE IF EXISTS user_institution_memberships;
DROP TABLE IF EXISTS compartimente;
DROP TABLE IF EXISTS institutions;
DROP TABLE IF EXISTS users;
```

- [ ] **Step 3.5: Write migration 000003_init_storage.sql**

```sql
-- migrations/000003_init_storage.sql
-- +migrate Up

-- Track all files stored in MinIO with metadata
CREATE TABLE stored_files (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    storage_key    VARCHAR(1000) NOT NULL UNIQUE, -- MinIO object key
    bucket         VARCHAR(255) NOT NULL,
    filename       VARCHAR(500) NOT NULL,
    content_type   VARCHAR(200) NOT NULL,
    size_bytes     BIGINT NOT NULL,
    sha256         VARCHAR(64) NOT NULL,           -- hex SHA-256 of file content
    uploaded_by    VARCHAR(255) NOT NULL,           -- user subject
    entity_type    VARCHAR(100),                   -- "document", "attachment", etc.
    entity_id      UUID,                           -- FK to the owning entity
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stored_files_entity ON stored_files(entity_type, entity_id);

-- +migrate Down
DROP TABLE IF EXISTS stored_files;
```

- [ ] **Step 3.6: Commit**

```bash
git add internal/database/ migrations/
git commit -m "feat: add database connection, migrations for RBAC, users, storage"
```

---

## Task 4: OIDC JWT authentication middleware

**Files:**
- Create: `internal/auth/claims.go`
- Create: `internal/auth/middleware.go`
- Create: `internal/auth/context.go`

- [ ] **Step 4.1: Write claims.go**

```go
// internal/auth/claims.go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
)

// Claims represents the JWT claims from the eguilde OIDC provider.
// The eguilde token includes: sub, uid, email, roles ([]string)
type Claims struct {
	Subject string   `json:"sub"`
	UID     int64    `json:"uid"`
	Email   string   `json:"email"`
	Roles   []string `json:"roles"`
	Exp     int64    `json:"exp"`
	Iat     int64    `json:"iat"`
}

// JWKSCache fetches and caches the OIDC provider's public keys.
type JWKSCache struct {
	mu       sync.RWMutex
	keys     jose.JSONWebKeySet
	fetchedAt time.Time
	ttl      time.Duration
	jwksURL  string
}

func NewJWKSCache(jwksURL string, ttl time.Duration) *JWKSCache {
	return &JWKSCache{jwksURL: jwksURL, ttl: ttl}
}

func (c *JWKSCache) GetKeys(ctx context.Context) (jose.JSONWebKeySet, error) {
	c.mu.RLock()
	if time.Since(c.fetchedAt) < c.ttl {
		keys := c.keys
		c.mu.RUnlock()
		return keys, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	// Double-check after acquiring write lock
	if time.Since(c.fetchedAt) < c.ttl {
		return c.keys, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURL, nil)
	if err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("build jwks request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	var keySet jose.JSONWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&keySet); err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("decode jwks: %w", err)
	}

	c.keys = keySet
	c.fetchedAt = time.Now()
	return c.keys, nil
}

// ParseToken validates a JWT Bearer token using the JWKS from the eguilde OIDC provider.
func ParseToken(ctx context.Context, tokenString string, cache *JWKSCache) (*Claims, error) {
	tok, err := josejwt.ParseSigned(tokenString, []jose.SignatureAlgorithm{jose.RS256, jose.ES256})
	if err != nil {
		return nil, fmt.Errorf("parse jwt: %w", err)
	}

	keySet, err := cache.GetKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("get jwks: %w", err)
	}

	var claims Claims
	for _, key := range keySet.Keys {
		if err := tok.Claims(key, &claims); err == nil {
			// Validate expiry
			if claims.Exp < time.Now().Unix() {
				return nil, fmt.Errorf("token expired")
			}
			return &claims, nil
		}
	}
	return nil, fmt.Errorf("no matching key found or invalid signature")
}
```

- [ ] **Step 4.2: Write middleware.go**

```go
// internal/auth/middleware.go
package auth

import (
	"net/http"
	"strings"
)

type contextKey string

const claimsKey contextKey = "claims"

// RequireAuth is a Chi middleware that validates the Bearer token and injects Claims into context.
// Returns 401 if missing/invalid, 403 if token is expired.
func RequireAuth(cache *JWKSCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}

			claims, err := ParseToken(r.Context(), parts[1], cache)
			if err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = contextWithClaims(ctx, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func contextWithClaims(ctx interface{ Value(any) any }, claims *Claims) interface{} {
	// Use standard context
	return nil // implemented below in context.go
}
```

- [ ] **Step 4.3: Write context.go (replaces stub in middleware.go)**

```go
// internal/auth/context.go
package auth

import (
	"context"
	"net/http"
	"strings"
)

// RequireAuth is a Chi middleware that validates the Bearer token and injects Claims into context.
func RequireAuth(cache *JWKSCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSON(w, http.StatusUnauthorized, `{"error":"missing authorization header"}`)
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				writeJSON(w, http.StatusUnauthorized, `{"error":"invalid authorization format"}`)
				return
			}

			claims, err := ParseToken(r.Context(), parts[1], cache)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, `{"error":"invalid or expired token"}`)
				return
			}

			next.ServeHTTP(w, r.WithContext(ContextWithClaims(r.Context(), claims)))
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(body))
}

func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// GetClaims returns the authenticated claims from the request context.
// Returns nil if not authenticated (should only be called after RequireAuth middleware).
func GetClaims(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

// GetUserSubject returns the JWT subject (user identifier) from context.
func GetUserSubject(ctx context.Context) string {
	if c := GetClaims(ctx); c != nil {
		return c.Subject
	}
	return ""
}
```

> **Note:** Delete the stub `middleware.go` from Step 4.2 — `context.go` contains the real `RequireAuth`. Only `context.go` and `claims.go` are needed.

- [ ] **Step 4.4: Write test**

Create `internal/auth/claims_test.go`:
```go
package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eguilde/egudoc/internal/auth"
)

func TestJWKSCacheRefreshesOnExpiry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
	}))
	defer srv.Close()

	cache := auth.NewJWKSCache(srv.URL, 100*time.Millisecond)

	// First call — fetches
	cache.GetKeys(context.Background())
	// Second call within TTL — uses cache
	cache.GetKeys(context.Background())
	if calls != 1 {
		t.Errorf("expected 1 fetch, got %d", calls)
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Third call — should re-fetch
	cache.GetKeys(context.Background())
	if calls != 2 {
		t.Errorf("expected 2 fetches after expiry, got %d", calls)
	}
}
```

- [ ] **Step 4.5: Run test**

```bash
cd /c/dev/egudoc
go test ./internal/auth/... -v -run TestJWKSCacheRefreshesOnExpiry
```

Expected: PASS

- [ ] **Step 4.6: Commit**

```bash
git add internal/auth/
git commit -m "feat: add JWT auth middleware with JWKS cache for eguilde OIDC"
```

---

## Task 5: RBAC system

**Files:**
- Create: `internal/rbac/model.go`
- Create: `internal/rbac/service.go`
- Create: `internal/rbac/middleware.go`
- Create: `internal/rbac/seeder.go`
- Create: `internal/rbac/handler.go`

- [ ] **Step 5.1: Write model.go**

```go
// internal/rbac/model.go
package rbac

import (
	"time"

	"github.com/google/uuid"
)

type Role struct {
	ID          uuid.UUID  `json:"id"`
	Code        string     `json:"code"`
	Label       string     `json:"label"`
	Description string     `json:"description,omitempty"`
	System      bool       `json:"system"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Permission struct {
	ID          uuid.UUID       `json:"id"`
	Action      string          `json:"action"`   // create, read, update, delete, approve, assign, archive, deliver
	Subject     string          `json:"subject"`  // document, workflow, registry, user, role, institution, report
	Condition   map[string]any  `json:"condition,omitempty"` // nil = no restriction
	Description string          `json:"description,omitempty"`
}

type UserRole struct {
	ID              uuid.UUID  `json:"id"`
	UserSubject     string     `json:"user_subject"`
	RoleID          uuid.UUID  `json:"role_id"`
	InstitutionID   *uuid.UUID `json:"institution_id,omitempty"`
	CompartimentID  *uuid.UUID `json:"compartiment_id,omitempty"`
	GrantedBy       string     `json:"granted_by,omitempty"`
	Active          bool       `json:"active"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// CheckContext carries the subject's context for condition evaluation
type CheckContext struct {
	UserSubject    string
	InstitutionID  *uuid.UUID
	CompartimentID *uuid.UUID
	ResourceID     *uuid.UUID // the specific resource being accessed
}

// DefaultRoles defines the system roles that are always seeded.
// Code must match what eguilde issues in token.roles claims.
var DefaultRoles = []Role{
	{Code: "superadmin",         Label: "Super Administrator",         System: true},
	{Code: "institution_admin",  Label: "Administrator Instituție",    System: true},
	{Code: "registrar",          Label: "Registrator",                 System: true},
	{Code: "department_head",    Label: "Șef Compartiment",           System: true},
	{Code: "department_staff",   Label: "Personal Compartiment",       System: true},
	{Code: "approver",           Label: "Aprobator",                  System: true},
	{Code: "archiver",           Label: "Arhivar",                    System: true},
	{Code: "citizen",            Label: "Cetățean",                   System: true},
	{Code: "external_entity",    Label: "Entitate Externă",           System: true},
}
```

- [ ] **Step 5.2: Write service.go**

```go
// internal/rbac/service.go
package rbac

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// HasPermission checks if a user (identified by subject) has the given action on the given subject.
// It checks all active roles for the user, optionally scoped by institution and compartiment.
func (s *Service) HasPermission(ctx context.Context, check CheckContext, action, subject string) (bool, error) {
	// Query: does any of the user's active roles have a permission matching action+subject?
	// For institution-scoped roles, ensure institutionID matches.
	query := `
		SELECT COUNT(*) > 0
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ur.user_subject = $1
		  AND ur.active = TRUE
		  AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
		  AND p.action = $2
		  AND p.subject = $3
		  AND (
		        ur.institution_id IS NULL
		        OR ur.institution_id = $4
		      )
	`
	var allowed bool
	err := s.db.QueryRow(ctx, query,
		check.UserSubject,
		action,
		subject,
		check.InstitutionID,
	).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("check permission: %w", err)
	}
	return allowed, nil
}

// GetUserRoles returns all active role codes for a user.
func (s *Service) GetUserRoles(ctx context.Context, userSubject string, institutionID *uuid.UUID) ([]string, error) {
	query := `
		SELECT r.code
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_subject = $1
		  AND ur.active = TRUE
		  AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
		  AND (ur.institution_id IS NULL OR ur.institution_id = $2)
	`
	rows, err := s.db.Query(ctx, query, userSubject, institutionID)
	if err != nil {
		return nil, fmt.Errorf("get user roles: %w", err)
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

// AssignRole grants a role to a user (with optional institution/compartiment scoping).
func (s *Service) AssignRole(ctx context.Context, grantedBy string, assignment UserRole) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_roles (user_subject, role_id, institution_id, compartiment_id, granted_by, active, expires_at)
		VALUES ($1, $2, $3, $4, $5, TRUE, $6)
		ON CONFLICT (user_subject, role_id,
		             COALESCE(institution_id::text,''),
		             COALESCE(compartiment_id::text,''))
		DO UPDATE SET active = TRUE, expires_at = EXCLUDED.expires_at, granted_by = EXCLUDED.granted_by
	`,
		assignment.UserSubject,
		assignment.RoleID,
		assignment.InstitutionID,
		assignment.CompartimentID,
		grantedBy,
		assignment.ExpiresAt,
	)
	return err
}

// RevokeRole deactivates a role assignment.
func (s *Service) RevokeRole(ctx context.Context, userSubject string, roleID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		UPDATE user_roles SET active = FALSE
		WHERE user_subject = $1 AND role_id = $2
	`, userSubject, roleID)
	return err
}

// GetRoleByCode returns a role by its code.
func (s *Service) GetRoleByCode(ctx context.Context, code string) (*Role, error) {
	var r Role
	err := s.db.QueryRow(ctx, `
		SELECT id, code, label, description, system, created_at, updated_at
		FROM roles WHERE code = $1
	`, code).Scan(&r.ID, &r.Code, &r.Label, &r.Description, &r.System, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get role by code %q: %w", code, err)
	}
	return &r, nil
}
```

- [ ] **Step 5.3: Write middleware.go**

```go
// internal/rbac/middleware.go
package rbac

import (
	"net/http"

	"github.com/eguilde/egudoc/internal/auth"
	"github.com/google/uuid"
)

// Require returns a Chi middleware that checks the authenticated user has `action` on `subject`.
// Must be used AFTER auth.RequireAuth middleware (claims must be in context).
func (s *Service) Require(action, subject string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.GetClaims(r.Context())
			if claims == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// Extract institution from query param or header if present
			var institutionID *uuid.UUID
			if raw := r.Header.Get("X-Institution-ID"); raw != "" {
				if id, err := uuid.Parse(raw); err == nil {
					institutionID = &id
				}
			}

			check := CheckContext{
				UserSubject:   claims.Subject,
				InstitutionID: institutionID,
			}

			allowed, err := s.HasPermission(r.Context(), check, action, subject)
			if err != nil || !allowed {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole returns a Chi middleware that checks the user has at least one of the given role codes.
// This is a coarser check than Require() — use for admin-only routes.
func (s *Service) RequireRole(roles ...string) func(http.Handler) http.Handler {
	roleSet := make(map[string]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.GetClaims(r.Context())
			if claims == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			for _, role := range claims.Roles {
				if roleSet[role] {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":"forbidden - insufficient role"}`, http.StatusForbidden)
		})
	}
}
```

- [ ] **Step 5.4: Write seeder.go**

```go
// internal/rbac/seeder.go
package rbac

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedDefaultRolesAndPermissions inserts the canonical roles and permissions on startup.
// Uses INSERT ... ON CONFLICT DO NOTHING so it is idempotent.
func SeedDefaultRolesAndPermissions(ctx context.Context, db *pgxpool.Pool) error {
	// Seed roles
	for _, role := range DefaultRoles {
		_, err := db.Exec(ctx, `
			INSERT INTO roles (code, label, system)
			VALUES ($1, $2, $3)
			ON CONFLICT (code) DO UPDATE SET label = EXCLUDED.label
		`, role.Code, role.Label, role.System)
		if err != nil {
			return fmt.Errorf("seed role %q: %w", role.Code, err)
		}
	}

	// Define all permissions
	type perm struct{ action, subject, desc string }
	perms := []perm{
		// Document permissions
		{"create", "document", "Create new documents in registry"},
		{"read", "document", "Read document details"},
		{"update", "document", "Update document fields"},
		{"delete", "document", "Delete/cancel a document"},
		{"archive", "document", "Submit document to qualified archive"},
		{"deliver", "document", "Send document via eDelivery"},
		// Workflow permissions
		{"assign", "workflow", "Assign documents to compartiments or users"},
		{"approve", "workflow", "Approve documents in approval chain"},
		{"reject", "workflow", "Reject documents in approval chain"},
		{"read", "workflow", "View workflow state and audit trail"},
		// Registry permissions
		{"create", "registry", "Create registries"},
		{"read", "registry", "View registries"},
		{"update", "registry", "Modify registry configuration"},
		// User/role management
		{"create", "user", "Create user accounts"},
		{"read", "user", "View user accounts"},
		{"update", "user", "Modify user accounts"},
		{"assign", "role", "Assign roles to users"},
		{"revoke", "role", "Revoke roles from users"},
		// Institution management
		{"create", "institution", "Create institutions"},
		{"read", "institution", "View institutions"},
		{"update", "institution", "Modify institution data"},
		// Reports
		{"read", "report", "Access reports and statistics"},
		{"export", "report", "Export reports"},
		// Archive management
		{"read", "archive", "View archive records"},
		{"manage", "archive", "Manage archive configuration"},
	}

	// Seed permissions
	for _, p := range perms {
		_, err := db.Exec(ctx, `
			INSERT INTO permissions (action, subject, description)
			VALUES ($1, $2, $3)
			ON CONFLICT (action, subject, COALESCE(condition::text, '')) DO NOTHING
		`, p.action, p.subject, p.desc)
		if err != nil {
			return fmt.Errorf("seed permission %s:%s: %w", p.action, p.subject, err)
		}
	}

	// Assign all permissions to superadmin
	_, err := db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
		WHERE r.code = 'superadmin'
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("seed superadmin permissions: %w", err)
	}

	// Assign registrar permissions
	_, err = db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.code = 'registrar'
		  AND ((p.action = 'create' AND p.subject = 'document')
		    OR (p.action = 'read'   AND p.subject = 'document')
		    OR (p.action = 'update' AND p.subject = 'document')
		    OR (p.action = 'read'   AND p.subject = 'registry')
		    OR (p.action = 'read'   AND p.subject = 'workflow')
		    OR (p.action = 'assign' AND p.subject = 'workflow'))
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("seed registrar permissions: %w", err)
	}

	// Assign department_head permissions
	_, err = db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.code = 'department_head'
		  AND ((p.action IN ('read', 'update') AND p.subject = 'document')
		    OR (p.action IN ('assign', 'approve', 'reject', 'read') AND p.subject = 'workflow')
		    OR (p.action = 'read' AND p.subject = 'report'))
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("seed department_head permissions: %w", err)
	}

	// Assign department_staff permissions
	_, err = db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.code = 'department_staff'
		  AND ((p.action IN ('read', 'update') AND p.subject = 'document')
		    OR (p.action = 'read' AND p.subject = 'workflow'))
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("seed department_staff permissions: %w", err)
	}

	// Assign archiver permissions
	_, err = db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.code = 'archiver'
		  AND ((p.action IN ('read', 'archive') AND p.subject = 'document')
		    OR (p.action IN ('read', 'manage') AND p.subject = 'archive')
		    OR (p.action = 'read' AND p.subject = 'report'))
		ON CONFLICT DO NOTHING
	`)
	return err
}
```

- [ ] **Step 5.5: Test HasPermission**

Create `internal/rbac/service_test.go`:
```go
package rbac_test

import (
	"context"
	"testing"

	"github.com/eguilde/egudoc/internal/rbac"
	"github.com/google/uuid"
)

// TestHasPermissionDeniedWhenNoRoles verifies that a user with no roles has no permissions.
// This is a unit test using a fake implementation; integration tests hit the real DB.
func TestCheckContextBuildsCorrectly(t *testing.T) {
	instID := uuid.New()
	compID := uuid.New()
	check := rbac.CheckContext{
		UserSubject:    "user-123",
		InstitutionID:  &instID,
		CompartimentID: &compID,
	}
	if check.UserSubject != "user-123" {
		t.Errorf("unexpected subject: %s", check.UserSubject)
	}
	if *check.InstitutionID != instID {
		t.Error("institution ID not set correctly")
	}
}
```

```bash
go test ./internal/rbac/... -v -run TestCheckContextBuildsCorrectly
```

Expected: PASS

- [ ] **Step 5.6: Commit**

```bash
git add internal/rbac/
git commit -m "feat: add RBAC service with role seeding and Chi middleware"
```

---

## Task 6: Users and health handlers

**Files:**
- Create: `internal/users/model.go`
- Create: `internal/users/service.go`
- Create: `internal/users/handler.go`
- Create: `internal/health/handler.go`

- [ ] **Step 6.1: Write users/model.go**

```go
// internal/users/model.go
package users

import (
	"time"
	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID  `json:"id"`
	Subject     string     `json:"subject"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone,omitempty"`
	Prenume     string     `json:"prenume,omitempty"`
	Nume        string     `json:"nume,omitempty"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
	Active      bool       `json:"active"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
```

- [ ] **Step 6.2: Write users/service.go**

```go
// internal/users/service.go
package users

import (
	"context"
	"fmt"
	"time"

	"github.com/eguilde/egudoc/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// GetOrCreate upserts a user from JWT claims on each login.
func (s *Service) GetOrCreate(ctx context.Context, claims *auth.Claims, ip string) (*User, error) {
	now := time.Now()
	var u User
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (subject, email, last_login_at, last_login_ip)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (subject) DO UPDATE
		SET email = EXCLUDED.email,
		    last_login_at = EXCLUDED.last_login_at,
		    last_login_ip = EXCLUDED.last_login_ip,
		    updated_at = NOW()
		RETURNING id, subject, email, phone, prenume, nume, active, last_login_at, created_at, updated_at
	`, claims.Subject, claims.Email, now, ip).Scan(
		&u.ID, &u.Subject, &u.Email, &u.Phone, &u.Prenume, &u.Nume,
		&u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return &u, nil
}

// GetBySubject retrieves a user by their JWT subject claim.
func (s *Service) GetBySubject(ctx context.Context, subject string) (*User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT id, subject, email, phone, prenume, nume, active, last_login_at, created_at, updated_at
		FROM users WHERE subject = $1
	`, subject).Scan(
		&u.ID, &u.Subject, &u.Email, &u.Phone, &u.Prenume, &u.Nume,
		&u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

// GetByID retrieves a user by UUID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT id, subject, email, phone, prenume, nume, active, last_login_at, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&u.ID, &u.Subject, &u.Email, &u.Phone, &u.Prenume, &u.Nume,
		&u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &u, err
}
```

- [ ] **Step 6.3: Write users/handler.go**

```go
// internal/users/handler.go
package users

import (
	"encoding/json"
	"net/http"

	"github.com/eguilde/egudoc/internal/auth"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/me", h.GetMe)
	return r
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.RemoteAddr
	}

	user, err := h.svc.GetOrCreate(r.Context(), claims, ip)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
```

- [ ] **Step 6.4: Write health/handler.go**

```go
// internal/health/handler.go
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/health", h.Health)
	r.Get("/health/ready", h.Ready)
	return r
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "egudoc"})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "degraded", "error": "database unreachable"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}
```

- [ ] **Step 6.5: Commit**

```bash
git add internal/users/ internal/health/
git commit -m "feat: add users service and health check endpoints"
```

---

## Task 7: MinIO storage client

**Files:**
- Create: `internal/storage/minio.go`

- [ ] **Step 7.1: Write minio.go**

```go
// internal/storage/minio.go
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	mc     *minio.Client
	bucket string
}

func NewClient(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	return &Client{mc: mc, bucket: bucket}, nil
}

// EnsureBucket creates the bucket if it doesn't exist.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if exists {
		return nil
	}
	return c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
}

// PutResult contains the storage key and SHA-256 of uploaded content.
type PutResult struct {
	StorageKey string
	SHA256     string
	SizeBytes  int64
}

// PutDocument stores a document file under a deterministic key.
// entityType: "document", "attachment", etc.
func (c *Client) PutDocument(ctx context.Context, entityType string, filename string, content io.Reader, contentType string, size int64) (*PutResult, error) {
	// Build key: entityType/YYYY/MM/uuid/filename
	now := time.Now()
	objectKey := path.Join(
		entityType,
		fmt.Sprintf("%d/%02d", now.Year(), now.Month()),
		uuid.New().String(),
		filename,
	)

	// Tee the reader to compute SHA-256 while uploading
	pr, pw := io.Pipe()
	hasher := sha256.New()
	tr := io.TeeReader(content, hasher)

	go func() {
		io.Copy(pw, tr)
		pw.Close()
	}()

	info, err := c.mc.PutObject(ctx, c.bucket, objectKey, pr, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("put object: %w", err)
	}

	return &PutResult{
		StorageKey: objectKey,
		SHA256:     hex.EncodeToString(hasher.Sum(nil)),
		SizeBytes:  info.Size,
	}, nil
}

// GetDocument returns a reader for a stored document.
func (c *Client) GetDocument(ctx context.Context, storageKey string) (io.ReadCloser, *minio.ObjectInfo, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, storageKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("get object: %w", err)
	}
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, nil, fmt.Errorf("stat object: %w", err)
	}
	return obj, &info, nil
}

// PresignedURL generates a time-limited pre-signed GET URL.
func (c *Client) PresignedURL(ctx context.Context, storageKey string, expiry time.Duration, filename string) (string, error) {
	params := url.Values{}
	if filename != "" {
		params.Set("response-content-disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	}
	u, err := c.mc.PresignedGetObject(ctx, c.bucket, storageKey, expiry, params)
	if err != nil {
		return "", fmt.Errorf("presign: %w", err)
	}
	return u.String(), nil
}

// DeleteDocument removes a document from storage.
func (c *Client) DeleteDocument(ctx context.Context, storageKey string) error {
	return c.mc.RemoveObject(ctx, c.bucket, storageKey, minio.RemoveObjectOptions{})
}
```

- [ ] **Step 7.2: Commit**

```bash
git add internal/storage/
git commit -m "feat: add MinIO storage client with SHA-256 tracking"
```

---

## Task 8: Docker Compose for local development

**Files:**
- Create: `docker-compose.yml`
- Create: `Dockerfile`

- [ ] **Step 8.1: Write docker-compose.yml**

```yaml
# docker-compose.yml
version: "3.9"

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: egudoc
      POSTGRES_PASSWORD: egudoc
      POSTGRES_DB: egudoc
    ports:
      - "5434:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U egudoc"]
      interval: 5s
      timeout: 5s
      retries: 5

  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: egudoc
      MINIO_ROOT_PASSWORD: egudoc123
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - miniodata:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 10s
      timeout: 5s
      retries: 3

  gotenberg:
    image: gotenberg/gotenberg:8
    ports:
      - "3000:3000"
    command:
      - "gotenberg"
      - "--chromium-disable-routes=true"
      - "--libreoffice-restart-after=10"

volumes:
  pgdata:
  miniodata:
```

- [ ] **Step 8.2: Write Dockerfile**

```dockerfile
# Dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/egudoc ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/bin/egudoc /egudoc
USER nonroot:nonroot
EXPOSE 8090
ENTRYPOINT ["/egudoc"]
```

- [ ] **Step 8.3: Start local services and verify**

```bash
cd /c/dev/egudoc
docker compose up -d postgres minio gotenberg
docker compose ps
```

Expected: all 3 services healthy.

- [ ] **Step 8.4: Commit**

```bash
git add docker-compose.yml Dockerfile
git commit -m "chore: add docker-compose for local dev (postgres, minio, gotenberg)"
```

---

## Task 9: Kubernetes deployment skeleton

**Files:**
- Create: `k8s/namespace.yaml`
- Create: `k8s/configmap.yaml`
- Create: `k8s/secret.yaml`
- Create: `k8s/deployment.yaml`
- Create: `k8s/service.yaml`
- Create: `k8s/ingress.yaml`
- Create: `k8s/hpa.yaml`

- [ ] **Step 9.1: Write namespace.yaml**

```yaml
# k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: egudoc
  labels:
    app.kubernetes.io/name: egudoc
```

- [ ] **Step 9.2: Write configmap.yaml**

```yaml
# k8s/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: egudoc-config
  namespace: egudoc
data:
  PORT: "8090"
  MINIO_BUCKET_DOCUMENTS: "egudoc-documents"
  MINIO_USE_SSL: "true"
  LOG_LEVEL: "info"
  NODE_ENV: "production"
  GOTENBERG_URL: "http://gotenberg.gotenberg.svc.cluster.local:3000"
```

- [ ] **Step 9.3: Write secret.yaml (template — values injected by CI)**

```yaml
# k8s/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: egudoc-secrets
  namespace: egudoc
type: Opaque
stringData:
  DATABASE_URL: "postgres://egudoc:CHANGE_ME@postgres:5432/egudoc?sslmode=require"
  OIDC_ISSUER: "https://eguilde.example.com/api/oidc"
  OIDC_JWKS_URL: "https://eguilde.example.com/api/oidc/jwks"
  OIDC_CLIENT_ID: "egudoc-spa"
  MINIO_ENDPOINT: "minio.minio.svc.cluster.local:9000"
  MINIO_ACCESS_KEY: "CHANGE_ME"
  MINIO_SECRET_KEY: "CHANGE_ME"
  QTSP_BASE_URL: "https://qtsp.eguwallet.example.com"
  QTSP_SERVICE_KEY: "CHANGE_ME"
  FRONTEND_URL: "https://egudoc.example.com"
```

- [ ] **Step 9.4: Write deployment.yaml**

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: egudoc
  namespace: egudoc
  labels:
    app: egudoc
spec:
  replicas: 2
  selector:
    matchLabels:
      app: egudoc
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    metadata:
      labels:
        app: egudoc
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8090"
    spec:
      serviceAccountName: egudoc
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
      containers:
        - name: egudoc
          image: ghcr.io/eguilde/egudoc:latest
          imagePullPolicy: Always
          ports:
            - name: http
              containerPort: 8090
          envFrom:
            - configMapRef:
                name: egudoc-config
            - secretRef:
                name: egudoc-secrets
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "1000m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8090
            initialDelaySeconds: 10
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8090
            initialDelaySeconds: 5
            periodSeconds: 10
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchExpressions:
                    - key: app
                      operator: In
                      values: [egudoc]
                topologyKey: kubernetes.io/hostname
```

- [ ] **Step 9.5: Write service.yaml and ingress.yaml**

```yaml
# k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: egudoc
  namespace: egudoc
spec:
  selector:
    app: egudoc
  ports:
    - name: http
      port: 80
      targetPort: 8090
  type: ClusterIP
---
# k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: egudoc
  namespace: egudoc
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/use-regex: "true"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - egudoc.example.com
      secretName: egudoc-tls
  rules:
    - host: egudoc.example.com
      http:
        paths:
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: egudoc
                port:
                  name: http
          - path: /
            pathType: Prefix
            backend:
              service:
                name: egudoc-frontend
                port:
                  name: http
```

- [ ] **Step 9.6: Write hpa.yaml**

```yaml
# k8s/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: egudoc
  namespace: egudoc
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: egudoc
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

- [ ] **Step 9.7: Commit**

```bash
git add k8s/
git commit -m "chore: add Kubernetes deployment manifests"
```

---

## Task 10: Wire everything in main.go and final build verification

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 10.1: Update main.go with all services wired**

```go
// cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"go.uber.org/zap"

	"github.com/eguilde/egudoc/internal/auth"
	"github.com/eguilde/egudoc/internal/config"
	"github.com/eguilde/egudoc/internal/database"
	"github.com/eguilde/egudoc/internal/health"
	"github.com/eguilde/egudoc/internal/rbac"
	"github.com/eguilde/egudoc/internal/storage"
	"github.com/eguilde/egudoc/internal/users"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg := config.Load()

	// Database
	pool, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("database connect failed", zap.Error(err))
	}
	defer pool.Close()

	if err := database.EnsureSchema(context.Background(), pool); err != nil {
		log.Fatal("schema migration failed", zap.Error(err))
	}

	// Seed RBAC default roles and permissions
	if err := rbac.SeedDefaultRolesAndPermissions(context.Background(), pool); err != nil {
		log.Fatal("RBAC seed failed", zap.Error(err))
	}

	// MinIO storage
	store, err := storage.NewClient(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioBucket, cfg.MinioUseSSL)
	if err != nil {
		log.Fatal("minio client failed", zap.Error(err))
	}
	if err := store.EnsureBucket(context.Background()); err != nil {
		log.Fatal("minio bucket setup failed", zap.Error(err))
	}

	// Auth
	jwksCache := auth.NewJWKSCache(cfg.OIDCJWKSURL, 5*time.Minute)
	authMiddleware := auth.RequireAuth(jwksCache)

	// RBAC
	rbacSvc := rbac.NewService(pool)

	// Users
	userSvc := users.NewService(pool)
	userHandler := users.NewHandler(userSvc)

	// Health
	healthHandler := health.NewHandler(pool)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(httprate.LimitByIP(100, time.Minute))

	// CORS
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", cfg.FrontendURL)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Institution-ID, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Public routes
	r.Mount("/", healthHandler.Routes())

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Mount("/api/users", userHandler.Routes())

		// Admin routes — superadmin only
		r.Group(func(r chi.Router) {
			r.Use(rbacSvc.RequireRole("superadmin", "institution_admin"))
			// RBAC admin, institution management — mounted in later sub-plans
		})
	})

	_ = rbacSvc // will be used by feature handlers in sub-plans B, C, D

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("egudoc starting", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
```

- [ ] **Step 10.2: Build and verify**

```bash
cd /c/dev/egudoc
go build ./cmd/server
echo "Exit code: $?"
```

Expected: exit code 0, no compilation errors.

- [ ] **Step 10.3: Run tests**

```bash
go test ./... -v -count=1 2>&1 | tail -20
```

Expected: all tests PASS (or SKIP if DB not running).

- [ ] **Step 10.4: Final commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire all services into main.go - foundation complete"
git push origin master
```

---

## Sub-plan A Completion Checklist

- [ ] Go module compiles cleanly
- [ ] Three database migrations applied idempotently
- [ ] JWKS cache tested with mock server
- [ ] RBAC roles and permissions seeded
- [ ] Health endpoints respond correctly
- [ ] Docker Compose starts postgres + minio + gotenberg
- [ ] Kubernetes manifests syntactically valid (`kubectl apply --dry-run=client -f k8s/`)

---

*Next: Sub-plan B — Registratura & Workflow Engine*
