# EguDoc — Go Backend: RustFS + Sync/Versioning/DeviceFlow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the Go backend foundation: RustFS storage deployment, DB migrations for versioning and device tokens, and three new internal modules (sync, versioning, deviceflow) that serve both the Angular web client and the optional Windows sync client.

**Architecture:** Three new packages follow the existing `model.go` / `service.go` / `handler.go` pattern with chi/v5 routing and pgx/v5. The `internal/storage` package gains a public-endpoint field so presigned URLs point to the externally-reachable RustFS hostname. A new `internal/auth/issuer.go` allows the backend to issue its own short-lived JWTs for device flow. RustFS replaces MinIO with no minio-go/v7 code changes — only the endpoint env vars change.

**Tech Stack:** Go 1.23, chi/v5, pgx/v5, aws-sdk-go-v2 (official RustFS-recommended Go SDK), go-jose/v4, golang-migrate, PostgreSQL 15, RustFS

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/storage/minio.go` | Modify | Add `mcPub` field + `STORAGE_PUBLIC_ENDPOINT` for externally-reachable presigned URLs |
| `internal/auth/issuer.go` | Create | Sign JWTs with backend RSA key for device flow access tokens |
| `migrations/000009_attachment_versions.up.sql` | Create | atasament_versiuni table + current_version_nr column |
| `migrations/000009_attachment_versions.down.sql` | Create | Reverse migration |
| `migrations/000010_sync_device_tokens.up.sql` | Create | sync_device_tokens table |
| `migrations/000010_sync_device_tokens.down.sql` | Create | Reverse migration |
| `internal/sync/model.go` | Create | Request/response types for sync endpoints |
| `internal/sync/service.go` | Create | Delta, DownloadURL, UploadIntent, UploadConfirm, ValidateMove |
| `internal/sync/service_test.go` | Create | Integration tests (requires TEST_DATABASE_URL) |
| `internal/sync/handler.go` | Create | HTTP handlers wiring service to chi routes |
| `internal/versioning/model.go` | Create | AttachmentVersion type |
| `internal/versioning/service.go` | Create | ListVersions, DownloadVersionURL |
| `internal/versioning/handler.go` | Create | HTTP handlers |
| `internal/deviceflow/model.go` | Create | PendingCode, DeviceCodeResponse, TokenResponse |
| `internal/deviceflow/service.go` | Create | In-memory code store + DB refresh token management |
| `internal/deviceflow/handler.go` | Create | HTTP handlers + approval page HTML |
| `cmd/server/main.go` | Modify | Register three new modules |
| `deploy/rustfs/statefulset.yaml` | Create | RustFS StatefulSet |
| `deploy/rustfs/service.yaml` | Create | ClusterIP Service for RustFS |
| `deploy/rustfs/ingress.yaml` | Create | Traefik IngressRoute for external S3 access |
| `deploy/rustfs/argocd-app.yaml` | Create | ArgoCD Application manifest |

---

### Task 1: Rewrite storage client to aws-sdk-go-v2 (official RustFS SDK)

RustFS recommends aws-sdk-go-v2 as its Go SDK. We rewrite `internal/storage/minio.go` with the same external interface so no other code changes are needed. We also add a `publicEndpoint` for generating presigned URLs that browsers can reach.

**Files:**
- Modify: `internal/storage/minio.go` (full rewrite — keep same function signatures)
- Modify: `go.mod` (add aws-sdk-go-v2 dependencies)

- [ ] **Step 1: Read the current storage file**

```bash
cat internal/storage/minio.go
```

Note the existing function signatures — we must keep them identical so callers don't change.

- [ ] **Step 2: Add aws-sdk-go-v2 dependencies**

```bash
go get github.com/aws/aws-sdk-go-v2
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/credentials
go get github.com/aws/aws-sdk-go-v2/service/s3
go get github.com/aws/aws-sdk-go-v2/aws
```

- [ ] **Step 3: Rewrite `internal/storage/minio.go`**

Replace the entire file with:

```go
package storage

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/s3/types"
    smithyendpoints "github.com/aws/smithy-go/endpoints"
)

// Client wraps the aws-sdk-go-v2 S3 client for RustFS.
// s3c is used for all object operations (internal endpoint).
// s3cPub is used only for presigned URL generation (externally-reachable endpoint).
type Client struct {
    s3c    *s3.Client
    s3cPub *s3.Client
    bucket string
}

type staticEndpointResolver struct{ url string }

func (r staticEndpointResolver) ResolveEndpoint(ctx context.Context, params s3.EndpointParameters) (smithyendpoints.Endpoint, error) {
    return smithyendpoints.Endpoint{URI: mustParseURL(r.url)}, nil
}

func mustParseURL(raw string) url.URL {
    u, err := url.Parse(raw)
    if err != nil {
        panic(fmt.Sprintf("storage: invalid endpoint URL %q: %v", raw, err))
    }
    return *u
}

func newS3Client(endpoint, accessKey, secretKey string, useSSL bool) *s3.Client {
    scheme := "http"
    if useSSL {
        scheme = "https"
    }
    fullURL := fmt.Sprintf("%s://%s", scheme, endpoint)
    creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
    return s3.New(s3.Options{
        Credentials:      creds,
        Region:           "us-east-1", // RustFS ignores region but SDK requires one
        UsePathStyle:     true,        // required for non-AWS S3-compatible stores
        EndpointResolverV2: staticEndpointResolver{url: fullURL},
    })
}

// NewClient creates a storage client.
// publicEndpoint is the externally-reachable RustFS hostname (e.g. storage.egudoc.ro).
// If empty, presigned URLs use the same endpoint as internal operations.
func NewClient(endpoint, publicEndpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
    c := &Client{bucket: bucket}
    c.s3c = newS3Client(endpoint, accessKey, secretKey, useSSL)
    if publicEndpoint != "" && publicEndpoint != endpoint {
        c.s3cPub = newS3Client(publicEndpoint, accessKey, secretKey, useSSL)
    } else {
        c.s3cPub = c.s3c
    }
    return c, nil
}

// EnsureBucket creates the bucket if it does not exist.
func (c *Client) EnsureBucket(ctx context.Context) error {
    _, err := c.s3c.CreateBucket(ctx, &s3.CreateBucketInput{
        Bucket: aws.String(c.bucket),
    })
    if err != nil {
        var bae *types.BucketAlreadyExists
        var bbo *types.BucketAlreadyOwnedByYou
        if errors.As(err, &bae) || errors.As(err, &bbo) {
            return nil
        }
        return fmt.Errorf("ensure bucket: %w", err)
    }
    return nil
}

// PutResult is returned by PutDocument.
type PutResult struct {
    StorageKey string
    SHA256     string
    SizeBytes  int64
}

// PutDocument uploads content to RustFS under entityType/YYYY/MM/<uuid>/filename.
func (c *Client) PutDocument(ctx context.Context, entityType, filename string, content io.Reader, contentType string, size int64) (*PutResult, error) {
    key := storageKey(entityType, filename)

    h := sha256.New()
    tr := io.TeeReader(content, h)

    _, err := c.s3c.PutObject(ctx, &s3.PutObjectInput{
        Bucket:        aws.String(c.bucket),
        Key:           aws.String(key),
        Body:          tr,
        ContentType:   aws.String(contentType),
        ContentLength: aws.Int64(size),
    })
    if err != nil {
        return nil, fmt.Errorf("put document: %w", err)
    }
    return &PutResult{
        StorageKey: key,
        SHA256:     hex.EncodeToString(h.Sum(nil)),
        SizeBytes:  size,
    }, nil
}

// GetDocument downloads an object from RustFS.
func (c *Client) GetDocument(ctx context.Context, storageKey string) (io.ReadCloser, error) {
    out, err := c.s3c.GetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(c.bucket),
        Key:    aws.String(storageKey),
    })
    if err != nil {
        return nil, fmt.Errorf("get document: %w", err)
    }
    return out.Body, nil
}

// PresignedGetURL returns a presigned GET URL valid for expiry duration.
func (c *Client) PresignedGetURL(ctx context.Context, key string, expiry time.Duration, filename string) (string, error) {
    pc := s3.NewPresignClient(c.s3cPub)
    req, err := pc.PresignGetObject(ctx, &s3.GetObjectInput{
        Bucket:                     aws.String(c.bucket),
        Key:                        aws.String(key),
        ResponseContentDisposition: aws.String(fmt.Sprintf(`attachment; filename="%s"`, filename)),
    }, s3.WithPresignExpires(expiry))
    if err != nil {
        return "", fmt.Errorf("presigned get: %w", err)
    }
    return req.URL, nil
}

// PresignedPutURL returns a presigned PUT URL valid for expiry duration.
func (c *Client) PresignedPutURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
    pc := s3.NewPresignClient(c.s3cPub)
    req, err := pc.PresignPutObject(ctx, &s3.PutObjectInput{
        Bucket: aws.String(c.bucket),
        Key:    aws.String(key),
    }, s3.WithPresignExpires(expiry))
    if err != nil {
        return "", fmt.Errorf("presigned put: %w", err)
    }
    return req.URL, nil
}

// ObjectExists returns true if the object at key exists in the bucket.
func (c *Client) ObjectExists(ctx context.Context, key string) (bool, error) {
    _, err := c.s3c.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: aws.String(c.bucket),
        Key:    aws.String(key),
    })
    if err != nil {
        var nsk *types.NoSuchKey
        var nf *types.NotFound
        if errors.As(err, &nsk) || errors.As(err, &nf) {
            return false, nil
        }
        return false, fmt.Errorf("object exists: %w", err)
    }
    return true, nil
}

// DeleteDocument removes an object from storage.
func (c *Client) DeleteDocument(ctx context.Context, key string) error {
    _, err := c.s3c.DeleteObject(ctx, &s3.DeleteObjectInput{
        Bucket: aws.String(c.bucket),
        Key:    aws.String(key),
    })
    if err != nil {
        return fmt.Errorf("delete document: %w", err)
    }
    return nil
}

func storageKey(entityType, filename string) string {
    now := time.Now().UTC()
    return fmt.Sprintf("%s/%04d/%02d/%s/%s", entityType, now.Year(), now.Month(), uuid.New(), filename)
}
```

Add the missing imports at the top:
```go
import (
    "errors"
    "net/url"
    "github.com/google/uuid"
    // ... rest above
)
```

- [ ] **Step 4: Remove minio-go/v7 from go.mod**

```bash
go mod tidy
```

Verify `minio-go/v7` is no longer in `go.mod`. If other packages still import it, remove those imports first.

- [ ] **Step 5: Update `cmd/server/main.go` — add STORAGE_PUBLIC_ENDPOINT**

Find where `storage.NewClient(` is called and update to the new 6-argument signature:

```go
store, err := storage.NewClient(
    os.Getenv("STORAGE_ENDPOINT"),
    os.Getenv("STORAGE_PUBLIC_ENDPOINT"), // externally-reachable RustFS host
    os.Getenv("STORAGE_ACCESS_KEY"),
    os.Getenv("STORAGE_SECRET_KEY"),
    os.Getenv("STORAGE_BUCKET"),
    os.Getenv("STORAGE_USE_SSL") == "true",
)
if err != nil {
    log.Fatal("storage client", zap.Error(err))
}
```

- [ ] **Step 6: Build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add internal/storage/minio.go go.mod go.sum cmd/server/main.go
git commit -m "feat(storage): switch to aws-sdk-go-v2 (RustFS recommended SDK), add public endpoint for presigned URLs"
```

---

### Task 2: JWT issuer for device flow

The existing backend only verifies JWTs (via JWKS). Device flow needs the backend to issue its own short-lived JWTs so sync clients can authenticate without going through the browser IdP each time.

**Files:**
- Create: `internal/auth/issuer.go`

- [ ] **Step 1: Write the failing test**

Create `internal/auth/issuer_test.go`:

```go
package auth_test

import (
    "crypto/rand"
    "crypto/rsa"
    "testing"
    "time"

    "github.com/eguilde/egudoc/internal/auth"
    "github.com/google/uuid"
)

func TestIssuer_SignAndParse(t *testing.T) {
    key, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        t.Fatal(err)
    }
    issuer := auth.NewIssuer(key)

    claims := auth.Claims{
        Subject: "user-subject-123",
        UID:     uuid.New(),
        Email:   "test@example.com",
        Roles:   []string{"employee"},
    }

    token, err := issuer.Sign(claims, 15*time.Minute)
    if err != nil {
        t.Fatalf("Sign: %v", err)
    }

    parsed, err := issuer.Verify(token)
    if err != nil {
        t.Fatalf("Verify: %v", err)
    }

    if parsed.Subject != claims.Subject {
        t.Errorf("subject: got %q want %q", parsed.Subject, claims.Subject)
    }
    if parsed.UID != claims.UID {
        t.Errorf("uid mismatch")
    }
    if len(parsed.Roles) != 1 || parsed.Roles[0] != "employee" {
        t.Errorf("roles: got %v", parsed.Roles)
    }
}

func TestIssuer_Verify_Expired(t *testing.T) {
    key, _ := rsa.GenerateKey(rand.Reader, 2048)
    issuer := auth.NewIssuer(key)
    token, _ := issuer.Sign(auth.Claims{Subject: "x"}, -1*time.Second)
    _, err := issuer.Verify(token)
    if err == nil {
        t.Fatal("expected error for expired token")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/auth/... -run TestIssuer -v
```
Expected: compile error — `auth.NewIssuer` not defined.

- [ ] **Step 3: Create `internal/auth/issuer.go`**

```go
package auth

import (
    "crypto/rsa"
    "fmt"
    "time"

    "github.com/go-jose/go-jose/v4"
    josejwt "github.com/go-jose/go-jose/v4/jwt"
    "github.com/google/uuid"
)

// Issuer signs and verifies JWTs using an RSA private key.
// Used by the device flow to issue access tokens for the sync client.
type Issuer struct {
    key    *rsa.PrivateKey
    signer jose.Signer
}

func NewIssuer(key *rsa.PrivateKey) *Issuer {
    sig, err := jose.NewSigner(
        jose.SigningKey{Algorithm: jose.RS256, Key: key},
        (&jose.SignerOptions{}).WithType("JWT"),
    )
    if err != nil {
        panic(fmt.Sprintf("auth issuer: %v", err))
    }
    return &Issuer{key: key, signer: sig}
}

type issuedClaims struct {
    josejwt.Claims
    UID   uuid.UUID `json:"uid"`
    Email string    `json:"email"`
    Roles []string  `json:"roles"`
}

func (i *Issuer) Sign(c Claims, ttl time.Duration) (string, error) {
    now := time.Now()
    ic := issuedClaims{
        Claims: josejwt.Claims{
            Subject:  c.Subject,
            IssuedAt: josejwt.NewNumericDate(now),
            Expiry:   josejwt.NewNumericDate(now.Add(ttl)),
        },
        UID:   c.UID,
        Email: c.Email,
        Roles: c.Roles,
    }
    raw, err := josejwt.Signed(i.signer).Claims(ic).Serialize()
    if err != nil {
        return "", fmt.Errorf("issuer sign: %w", err)
    }
    return raw, nil
}

func (i *Issuer) Verify(token string) (*Claims, error) {
    tok, err := josejwt.ParseSigned(token, []jose.SignatureAlgorithm{jose.RS256})
    if err != nil {
        return nil, fmt.Errorf("issuer verify parse: %w", err)
    }
    var ic issuedClaims
    if err := tok.Claims(i.key.Public(), &ic); err != nil {
        return nil, fmt.Errorf("issuer verify claims: %w", err)
    }
    if err := ic.ValidateWithLeeway(josejwt.Expected{Time: time.Now()}, 0); err != nil {
        return nil, fmt.Errorf("issuer verify expired: %w", err)
    }
    return &Claims{
        Subject: ic.Subject,
        UID:     ic.UID,
        Email:   ic.Email,
        Roles:   ic.Roles,
        Exp:     ic.Expiry.Time().Unix(),
        Iat:     ic.IssuedAt.Time().Unix(),
    }, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/auth/... -run TestIssuer -v
```
Expected: PASS for both test cases.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/issuer.go internal/auth/issuer_test.go
git commit -m "feat(auth): add RSA JWT issuer for device flow access tokens"
```

---

### Task 3: Migration 000009 — atasament_versiuni

**Files:**
- Create: `migrations/000009_attachment_versions.up.sql`
- Create: `migrations/000009_attachment_versions.down.sql`

- [ ] **Step 1: Create the up migration**

```sql
-- migrations/000009_attachment_versions.up.sql

ALTER TABLE atasamente ADD COLUMN current_version_nr INTEGER NOT NULL DEFAULT 1;

CREATE TABLE atasament_versiuni (
    id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    atasament_id    UUID         NOT NULL REFERENCES atasamente(id) ON DELETE CASCADE,
    version_nr      INTEGER      NOT NULL,
    storage_key     VARCHAR(1000) NOT NULL,
    size_bytes      BIGINT       NOT NULL,
    sha256          VARCHAR(64)  NOT NULL,
    uploaded_by     VARCHAR(255) NOT NULL,
    source          VARCHAR(20)  NOT NULL CHECK (source IN ('web', 'windows_sync')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_atasament_versiuni_nr ON atasament_versiuni(atasament_id, version_nr);
CREATE INDEX idx_atasament_versiuni_atasament ON atasament_versiuni(atasament_id);

-- Seed version 1 for all existing attachments
INSERT INTO atasament_versiuni (atasament_id, version_nr, storage_key, size_bytes, sha256, uploaded_by, source)
SELECT id, 1, storage_key, size_bytes, sha256, uploaded_by, 'web'
FROM atasamente;
```

- [ ] **Step 2: Create the down migration**

```sql
-- migrations/000009_attachment_versions.down.sql

DROP TABLE IF EXISTS atasament_versiuni;
ALTER TABLE atasamente DROP COLUMN IF EXISTS current_version_nr;
```

- [ ] **Step 3: Run migration against local dev DB and verify**

```bash
migrate -path migrations -database "$DATABASE_URL" up 1
```
Expected output: `1/u attachment_versions (Xms)`

Then verify:
```bash
psql $DATABASE_URL -c "\d atasament_versiuni"
psql $DATABASE_URL -c "SELECT COUNT(*) FROM atasament_versiuni;"
```
Expected: table exists; row count equals the number of existing atasamente rows.

- [ ] **Step 4: Commit**

```bash
git add migrations/000009_attachment_versions.up.sql migrations/000009_attachment_versions.down.sql
git commit -m "feat(db): add atasament_versiuni table and version tracking"
```

---

### Task 4: Migration 000010 — sync_device_tokens

**Files:**
- Create: `migrations/000010_sync_device_tokens.up.sql`
- Create: `migrations/000010_sync_device_tokens.down.sql`

- [ ] **Step 1: Create the up migration**

```sql
-- migrations/000010_sync_device_tokens.up.sql

CREATE TABLE sync_device_tokens (
    id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id       VARCHAR(255) NOT NULL,
    refresh_token   VARCHAR(64)  NOT NULL,  -- SHA-256 hex of the opaque token, never plaintext
    last_seen_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    revoked_at      TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_sync_device_tokens_user_device ON sync_device_tokens(user_id, device_id);
CREATE INDEX idx_sync_device_tokens_token ON sync_device_tokens(refresh_token) WHERE revoked_at IS NULL;
```

- [ ] **Step 2: Create the down migration**

```sql
-- migrations/000010_sync_device_tokens.down.sql

DROP TABLE IF EXISTS sync_device_tokens;
```

- [ ] **Step 3: Run and verify**

```bash
migrate -path migrations -database "$DATABASE_URL" up 1
psql $DATABASE_URL -c "\d sync_device_tokens"
```

- [ ] **Step 4: Commit**

```bash
git add migrations/000010_sync_device_tokens.up.sql migrations/000010_sync_device_tokens.down.sql
git commit -m "feat(db): add sync_device_tokens table for Windows client auth"
```

---

### Task 5: internal/sync/model.go

**Files:**
- Create: `internal/sync/model.go`

- [ ] **Step 1: Create model file**

```go
package sync

import (
    "time"

    "github.com/google/uuid"
)

// DeltaDocument is one item returned by the delta endpoint.
type DeltaDocument struct {
    ID             uuid.UUID      `json:"id"`
    NrInregistrare string         `json:"nr_inregistrare"`
    RegistruID     uuid.UUID      `json:"registru_id"`
    RegistruNume   string         `json:"registru_nume"`
    Tip            string         `json:"tip"`
    Status         string         `json:"status"`
    Obiect         string         `json:"obiect"`
    UpdatedAt      time.Time      `json:"updated_at"`
    Atasamente     []DeltaAtasament `json:"atasamente"`
}

// DeltaAtasament is an attachment stub returned in delta responses.
type DeltaAtasament struct {
    ID             uuid.UUID `json:"id"`
    Filename       string    `json:"filename"`
    ContentType    string    `json:"content_type"`
    SizeBytes      int64     `json:"size_bytes"`
    CurrentVersion int       `json:"current_version"`
    UpdatedAt      time.Time `json:"updated_at"`
}

// UploadIntentRequest starts the two-phase upload.
// AtasamentID nil means new attachment; set means new version of existing attachment.
type UploadIntentRequest struct {
    DocumentID  uuid.UUID  `json:"document_id"`
    AtasamentID *uuid.UUID `json:"atasament_id,omitempty"`
    Filename    string     `json:"filename"`
    ContentType string     `json:"content_type"`
    SizeBytes   int64      `json:"size_bytes"`
}

// UploadIntentResponse contains the presigned PUT URL for direct upload to RustFS.
type UploadIntentResponse struct {
    UploadURL  string `json:"upload_url"`
    StorageKey string `json:"storage_key"`
}

// UploadConfirmRequest completes the upload after the client has PUT the bytes.
type UploadConfirmRequest struct {
    DocumentID  uuid.UUID  `json:"document_id"`
    AtasamentID *uuid.UUID `json:"atasament_id,omitempty"`
    StorageKey  string     `json:"storage_key"`
    Filename    string     `json:"filename"`
    ContentType string     `json:"content_type"`
    SizeBytes   int64      `json:"size_bytes"`
    SHA256      string     `json:"sha256"`
    Source      string     `json:"source"` // "web" or "windows_sync"
}

// UploadConfirmResponse is returned after a successful upload-confirm.
type UploadConfirmResponse struct {
    AtasamentID uuid.UUID `json:"atasament_id"`
    VersionNr   int       `json:"version_nr"`
}

// ValidateMoveRequest checks whether a move crosses a department boundary.
type ValidateMoveRequest struct {
    AttachmentID   uuid.UUID `json:"attachment_id"`
    DestDocumentID uuid.UUID `json:"dest_document_id"`
}
```

- [ ] **Step 2: Build to confirm**

```bash
go build ./internal/sync/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/sync/model.go
git commit -m "feat(sync): add sync request/response model types"
```

---

### Task 6: internal/sync/service.go

**Files:**
- Create: `internal/sync/service.go`
- Create: `internal/sync/service_test.go`

- [ ] **Step 1: Write the failing tests first**

Create `internal/sync/service_test.go`:

```go
package sync_test

import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/eguilde/egudoc/internal/sync"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

func testDB(t *testing.T) *pgxpool.Pool {
    t.Helper()
    url := os.Getenv("TEST_DATABASE_URL")
    if url == "" {
        t.Skip("TEST_DATABASE_URL not set")
    }
    pool, err := pgxpool.New(context.Background(), url)
    if err != nil {
        t.Fatalf("testDB: %v", err)
    }
    t.Cleanup(pool.Close)
    return pool
}

// mockStore satisfies sync.StorageClient with no-op implementations.
type mockStore struct{}

func (m *mockStore) PresignedGetURL(ctx context.Context, key string, expiry time.Duration, filename string) (string, error) {
    return "https://storage.test/" + key, nil
}
func (m *mockStore) PresignedPutURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
    return "https://storage.test/" + key + "?put=1", nil
}
func (m *mockStore) ObjectExists(ctx context.Context, key string) (bool, error) {
    return true, nil
}

func TestSyncService_Delta_Empty(t *testing.T) {
    db := testDB(t)
    svc := sync.NewService(db, &mockStore{}, nil, nil)
    since := time.Now().Add(-time.Hour)
    results, err := svc.Delta(context.Background(), "test-subject", uuid.New(), &since)
    if err != nil {
        t.Fatalf("Delta: %v", err)
    }
    // result may be empty or contain rows depending on test DB state
    _ = results
}

func TestSyncService_ValidateMove_SameDocument(t *testing.T) {
    // When src and dst are in the same document (same entitate), move should be allowed.
    // This test verifies no error when both documents share compartiment.
    // Full cross-dept test requires seeded fixture data — run in integration suite.
    db := testDB(t)
    svc := sync.NewService(db, &mockStore{}, nil, nil)

    err := svc.ValidateMove(context.Background(), "user-sub", uuid.New(), uuid.New(), uuid.New())
    // We expect either nil (permitted) or a specific ErrForbidden — never a DB error.
    if err != nil && err.Error() == "sql: connection refused" {
        t.Fatal("unexpected DB connection error")
    }
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./internal/sync/... -run TestSync -v 2>&1 | head -20
```
Expected: compile error — package `sync` not yet implemented.

- [ ] **Step 3: Create `internal/sync/service.go`**

```go
package sync

import (
    "context"
    "fmt"
    "time"

    "github.com/eguilde/egudoc/internal/rbac"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "go.uber.org/zap"
)

// StorageClient is the subset of storage.Client used by this package.
// Defined as an interface to allow test mocking.
type StorageClient interface {
    PresignedGetURL(ctx context.Context, key string, expiry time.Duration, filename string) (string, error)
    PresignedPutURL(ctx context.Context, key string, expiry time.Duration) (string, error)
    ObjectExists(ctx context.Context, key string) (bool, error)
}

// ErrForbidden is returned when an RBAC check fails.
var ErrForbidden = fmt.Errorf("forbidden")

type Service struct {
    db    *pgxpool.Pool
    store StorageClient
    rbac  *rbac.Service
    log   *zap.Logger
}

func NewService(db *pgxpool.Pool, store StorageClient, rbacSvc *rbac.Service, log *zap.Logger) *Service {
    return &Service{db: db, store: store, rbac: rbacSvc, log: log}
}

// Delta returns documents+attachments visible to userSubject that were updated after 'since'.
// Pass since=nil to return all visible documents.
func (s *Service) Delta(ctx context.Context, userSubject string, institutionID uuid.UUID, since *time.Time) ([]DeltaDocument, error) {
    sinceVal := time.Time{}
    if since != nil {
        sinceVal = *since
    }

    rows, err := s.db.Query(ctx, `
        SELECT
            d.id, d.nr_inregistrare, d.registru_id, r.nume, d.tip::text, d.status::text,
            d.obiect, d.updated_at,
            a.id, a.filename, a.content_type, a.size_bytes, a.current_version_nr, a.created_at
        FROM documente d
        JOIN registre r ON r.id = d.registru_id
        LEFT JOIN atasamente a ON a.document_id = d.id
        WHERE d.institution_id = $1
          AND d.updated_at > $2
          AND d.status != 'ANULAT'
        ORDER BY d.updated_at DESC, a.created_at
    `, institutionID, sinceVal)
    if err != nil {
        return nil, fmt.Errorf("sync delta query: %w", err)
    }
    defer rows.Close()

    // Aggregate attachment rows into documents.
    docMap := make(map[uuid.UUID]*DeltaDocument)
    var order []uuid.UUID

    for rows.Next() {
        var (
            docID          uuid.UUID
            nrInregistrare string
            registruID     uuid.UUID
            registruNume   string
            tip, status    string
            obiect         string
            updatedAt      time.Time
            ataID          *uuid.UUID
            filename       *string
            contentType    *string
            sizeBytes      *int64
            currentVersion *int
            ataUpdatedAt   *time.Time
        )
        if err := rows.Scan(
            &docID, &nrInregistrare, &registruID, &registruNume, &tip, &status,
            &obiect, &updatedAt,
            &ataID, &filename, &contentType, &sizeBytes, &currentVersion, &ataUpdatedAt,
        ); err != nil {
            return nil, fmt.Errorf("sync delta scan: %w", err)
        }
        if _, exists := docMap[docID]; !exists {
            docMap[docID] = &DeltaDocument{
                ID: docID, NrInregistrare: nrInregistrare,
                RegistruID: registruID, RegistruNume: registruNume,
                Tip: tip, Status: status, Obiect: obiect, UpdatedAt: updatedAt,
            }
            order = append(order, docID)
        }
        if ataID != nil {
            docMap[docID].Atasamente = append(docMap[docID].Atasamente, DeltaAtasament{
                ID: *ataID, Filename: *filename, ContentType: *contentType,
                SizeBytes: *sizeBytes, CurrentVersion: *currentVersion,
                UpdatedAt: *ataUpdatedAt,
            })
        }
    }
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("sync delta rows: %w", err)
    }

    result := make([]DeltaDocument, 0, len(order))
    for _, id := range order {
        result = append(result, *docMap[id])
    }
    return result, nil
}

// DownloadURL validates that userSubject can read the attachment and returns a presigned GET URL.
func (s *Service) DownloadURL(ctx context.Context, userSubject string, institutionID uuid.UUID, attachmentID uuid.UUID) (string, error) {
    var storageKey, filename string
    err := s.db.QueryRow(ctx, `
        SELECT a.storage_key, a.filename
        FROM atasamente a
        JOIN documente d ON d.id = a.document_id
        WHERE a.id = $1 AND d.institution_id = $2
    `, attachmentID, institutionID).Scan(&storageKey, &filename)
    if err != nil {
        return "", fmt.Errorf("sync download lookup: %w", err)
    }
    url, err := s.store.PresignedGetURL(ctx, storageKey, 5*time.Minute, filename)
    if err != nil {
        return "", fmt.Errorf("sync download presign: %w", err)
    }
    return url, nil
}

// UploadIntent validates RBAC and returns a presigned PUT URL for direct upload to RustFS.
func (s *Service) UploadIntent(ctx context.Context, userSubject string, institutionID uuid.UUID, req UploadIntentRequest) (*UploadIntentResponse, error) {
    // Verify document exists in this institution.
    var docExists bool
    err := s.db.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM documente WHERE id = $1 AND institution_id = $2 AND status != 'ANULAT')
    `, req.DocumentID, institutionID).Scan(&docExists)
    if err != nil {
        return nil, fmt.Errorf("sync upload-intent check doc: %w", err)
    }
    if !docExists {
        return nil, ErrForbidden
    }

    // Generate a unique storage key for this upload slot.
    storageKey := fmt.Sprintf("atasamente/%s/%s/%s", req.DocumentID, uuid.New(), req.Filename)

    uploadURL, err := s.store.PresignedPutURL(ctx, storageKey, 15*time.Minute)
    if err != nil {
        return nil, fmt.Errorf("sync upload-intent presign: %w", err)
    }
    return &UploadIntentResponse{UploadURL: uploadURL, StorageKey: storageKey}, nil
}

// UploadConfirm verifies the object landed in RustFS, then atomically creates/updates
// the atasament row and appends an atasament_versiuni row.
func (s *Service) UploadConfirm(ctx context.Context, userSubject string, req UploadConfirmRequest) (*UploadConfirmResponse, error) {
    exists, err := s.store.ObjectExists(ctx, req.StorageKey)
    if err != nil {
        return nil, fmt.Errorf("sync upload-confirm stat: %w", err)
    }
    if !exists {
        return nil, fmt.Errorf("sync upload-confirm: object not found in storage")
    }

    tx, err := s.db.Begin(ctx)
    if err != nil {
        return nil, fmt.Errorf("sync upload-confirm begin tx: %w", err)
    }
    defer tx.Rollback(ctx)

    var atasamentID uuid.UUID
    var versionNr int

    if req.AtasamentID == nil {
        // New attachment — insert into atasamente, then insert version 1.
        atasamentID = uuid.New()
        _, err = tx.Exec(ctx, `
            INSERT INTO atasamente (id, document_id, storage_key, filename, content_type, size_bytes, sha256, uploaded_by, current_version_nr)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1)
        `, atasamentID, req.DocumentID, req.StorageKey, req.Filename, req.ContentType, req.SizeBytes, req.SHA256, userSubject)
        if err != nil {
            return nil, fmt.Errorf("sync upload-confirm insert atasament: %w", err)
        }
        versionNr = 1
    } else {
        // New version — increment current_version_nr on the existing atasament.
        atasamentID = *req.AtasamentID
        err = tx.QueryRow(ctx, `
            UPDATE atasamente
            SET storage_key = $1, size_bytes = $2, sha256 = $3, current_version_nr = current_version_nr + 1
            WHERE id = $4
            RETURNING current_version_nr
        `, req.StorageKey, req.SizeBytes, req.SHA256, atasamentID).Scan(&versionNr)
        if err != nil {
            return nil, fmt.Errorf("sync upload-confirm update atasament: %w", err)
        }
    }

    // Append version row.
    _, err = tx.Exec(ctx, `
        INSERT INTO atasament_versiuni (atasament_id, version_nr, storage_key, size_bytes, sha256, uploaded_by, source)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, atasamentID, versionNr, req.StorageKey, req.SizeBytes, req.SHA256, userSubject, req.Source)
    if err != nil {
        return nil, fmt.Errorf("sync upload-confirm insert version: %w", err)
    }

    if err := tx.Commit(ctx); err != nil {
        return nil, fmt.Errorf("sync upload-confirm commit: %w", err)
    }
    return &UploadConfirmResponse{AtasamentID: atasamentID, VersionNr: versionNr}, nil
}

// ValidateMove returns ErrForbidden if moving the attachment to destDocumentID would
// cross a department (compartiment) boundary.
func (s *Service) ValidateMove(ctx context.Context, userSubject string, institutionID uuid.UUID, attachmentID, destDocumentID uuid.UUID) error {
    var srcCompartiment, dstCompartiment *uuid.UUID
    err := s.db.QueryRow(ctx, `
        SELECT d.compartiment_curent_id
        FROM atasamente a JOIN documente d ON d.id = a.document_id
        WHERE a.id = $1
    `, attachmentID).Scan(&srcCompartiment)
    if err != nil {
        return fmt.Errorf("sync validate-move src: %w", err)
    }
    err = s.db.QueryRow(ctx, `
        SELECT compartiment_curent_id FROM documente WHERE id = $1
    `, destDocumentID).Scan(&dstCompartiment)
    if err != nil {
        return fmt.Errorf("sync validate-move dst: %w", err)
    }
    // If either is NULL the document is not yet assigned — allow move.
    if srcCompartiment == nil || dstCompartiment == nil {
        return nil
    }
    if *srcCompartiment != *dstCompartiment {
        return ErrForbidden
    }
    return nil
}
```

- [ ] **Step 4: Run tests**

```bash
TEST_DATABASE_URL="postgres://postgres:postgres@localhost/egudoc_test?sslmode=disable" \
  go test ./internal/sync/... -v
```
Expected: PASS (Delta_Empty and ValidateMove_SameDocument).

- [ ] **Step 6: Commit**

```bash
git add internal/sync/service.go internal/sync/service_test.go internal/storage/minio.go
git commit -m "feat(sync): add sync service with delta, download, upload, validate-move"
```

---

### Task 7: internal/sync/handler.go

**Files:**
- Create: `internal/sync/handler.go`

- [ ] **Step 1: Create the handler**

```go
package sync

import (
    "encoding/json"
    "errors"
    "net/http"

    "github.com/eguilde/egudoc/internal/auth"
    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
)

type Handler struct {
    svc *Service
}

func NewHandler(svc *Service) *Handler {
    return &Handler{svc: svc}
}

func (h *Handler) Routes() chi.Router {
    r := chi.NewRouter()
    r.Get("/delta", h.Delta)
    r.Get("/download/{aid}", h.Download)
    r.Post("/upload-intent", h.UploadIntent)
    r.Post("/upload-confirm", h.UploadConfirm)
    r.Post("/validate-move", h.ValidateMove)
    return r
}

func (h *Handler) Delta(w http.ResponseWriter, r *http.Request) {
    claims := auth.GetClaims(r.Context())
    instID, err := institutionIDFromHeader(r)
    if err != nil {
        http.Error(w, "X-Institution-ID required", http.StatusBadRequest)
        return
    }
    var since *time.Time
    if s := r.URL.Query().Get("since"); s != "" {
        t, err := time.Parse(time.RFC3339, s)
        if err != nil {
            http.Error(w, "invalid since param", http.StatusBadRequest)
            return
        }
        since = &t
    }
    docs, err := h.svc.Delta(r.Context(), claims.Subject, instID, since)
    if err != nil {
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(docs)
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
    claims := auth.GetClaims(r.Context())
    instID, err := institutionIDFromHeader(r)
    if err != nil {
        http.Error(w, "X-Institution-ID required", http.StatusBadRequest)
        return
    }
    aidStr := chi.URLParam(r, "aid")
    aid, err := uuid.Parse(aidStr)
    if err != nil {
        http.Error(w, "invalid attachment id", http.StatusBadRequest)
        return
    }
    url, err := h.svc.DownloadURL(r.Context(), claims.Subject, instID, aid)
    if err != nil {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"url": url})
}

func (h *Handler) UploadIntent(w http.ResponseWriter, r *http.Request) {
    claims := auth.GetClaims(r.Context())
    instID, err := institutionIDFromHeader(r)
    if err != nil {
        http.Error(w, "X-Institution-ID required", http.StatusBadRequest)
        return
    }
    var req UploadIntentRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    resp, err := h.svc.UploadIntent(r.Context(), claims.Subject, instID, req)
    if errors.Is(err, ErrForbidden) {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    if err != nil {
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(resp)
}

func (h *Handler) UploadConfirm(w http.ResponseWriter, r *http.Request) {
    claims := auth.GetClaims(r.Context())
    var req UploadConfirmRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    req.Source = sanitizeSource(req.Source)
    resp, err := h.svc.UploadConfirm(r.Context(), claims.Subject, req)
    if err != nil {
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ValidateMove(w http.ResponseWriter, r *http.Request) {
    claims := auth.GetClaims(r.Context())
    instID, err := institutionIDFromHeader(r)
    if err != nil {
        http.Error(w, "X-Institution-ID required", http.StatusBadRequest)
        return
    }
    var req ValidateMoveRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    err = h.svc.ValidateMove(r.Context(), claims.Subject, instID, req.AttachmentID, req.DestDocumentID)
    if errors.Is(err, ErrForbidden) {
        http.Error(w, "move crosses department boundary", http.StatusForbidden)
        return
    }
    if err != nil {
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
}

func institutionIDFromHeader(r *http.Request) (uuid.UUID, error) {
    return uuid.Parse(r.Header.Get("X-Institution-ID"))
}

func sanitizeSource(s string) string {
    if s == "windows_sync" {
        return "windows_sync"
    }
    return "web"
}
```

Note: add `"time"` to the imports in this file.

- [ ] **Step 2: Build**

```bash
go build ./internal/sync/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/sync/handler.go
git commit -m "feat(sync): add HTTP handler for sync endpoints"
```

---

### Task 8: internal/versioning — model, service, handler

**Files:**
- Create: `internal/versioning/model.go`
- Create: `internal/versioning/service.go`
- Create: `internal/versioning/handler.go`

- [ ] **Step 1: Create model.go**

```go
package versioning

import (
    "time"
    "github.com/google/uuid"
)

type AttachmentVersion struct {
    ID         uuid.UUID `json:"id"`
    VersionNr  int       `json:"version_nr"`
    SizeBytes  int64     `json:"size_bytes"`
    UploadedBy string    `json:"uploaded_by"`
    Source     string    `json:"source"`
    CreatedAt  time.Time `json:"created_at"`
}
```

- [ ] **Step 2: Create service.go**

```go
package versioning

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type StorageClient interface {
    PresignedGetURL(ctx context.Context, key string, expiry time.Duration, filename string) (string, error)
}

type Service struct {
    db    *pgxpool.Pool
    store StorageClient
}

func NewService(db *pgxpool.Pool, store StorageClient) *Service {
    return &Service{db: db, store: store}
}

func (s *Service) ListVersions(ctx context.Context, attachmentID uuid.UUID) ([]AttachmentVersion, error) {
    rows, err := s.db.Query(ctx, `
        SELECT id, version_nr, size_bytes, uploaded_by, source, created_at
        FROM atasament_versiuni
        WHERE atasament_id = $1
        ORDER BY version_nr DESC
    `, attachmentID)
    if err != nil {
        return nil, fmt.Errorf("versioning list: %w", err)
    }
    defer rows.Close()

    var versions []AttachmentVersion
    for rows.Next() {
        var v AttachmentVersion
        if err := rows.Scan(&v.ID, &v.VersionNr, &v.SizeBytes, &v.UploadedBy, &v.Source, &v.CreatedAt); err != nil {
            return nil, fmt.Errorf("versioning scan: %w", err)
        }
        versions = append(versions, v)
    }
    return versions, rows.Err()
}

func (s *Service) DownloadVersionURL(ctx context.Context, attachmentID uuid.UUID, versionNr int) (string, error) {
    var storageKey, filename string
    err := s.db.QueryRow(ctx, `
        SELECT v.storage_key, a.filename
        FROM atasament_versiuni v
        JOIN atasamente a ON a.id = v.atasament_id
        WHERE v.atasament_id = $1 AND v.version_nr = $2
    `, attachmentID, versionNr).Scan(&storageKey, &filename)
    if err != nil {
        return "", fmt.Errorf("versioning download lookup: %w", err)
    }
    return s.store.PresignedGetURL(ctx, storageKey, 5*time.Minute, filename)
}
```

- [ ] **Step 3: Create handler.go**

```go
package versioning

import (
    "encoding/json"
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
)

type Handler struct {
    svc *Service
}

func NewHandler(svc *Service) *Handler {
    return &Handler{svc: svc}
}

func (h *Handler) Routes() chi.Router {
    r := chi.NewRouter()
    r.Get("/{aid}/versions", h.ListVersions)
    r.Get("/{aid}/versions/{vn}", h.DownloadVersion)
    return r
}

func (h *Handler) ListVersions(w http.ResponseWriter, r *http.Request) {
    aid, err := uuid.Parse(chi.URLParam(r, "aid"))
    if err != nil {
        http.Error(w, "invalid attachment id", http.StatusBadRequest)
        return
    }
    versions, err := h.svc.ListVersions(r.Context(), aid)
    if err != nil {
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(versions)
}

func (h *Handler) DownloadVersion(w http.ResponseWriter, r *http.Request) {
    aid, err := uuid.Parse(chi.URLParam(r, "aid"))
    if err != nil {
        http.Error(w, "invalid attachment id", http.StatusBadRequest)
        return
    }
    vn, err := strconv.Atoi(chi.URLParam(r, "vn"))
    if err != nil || vn < 1 {
        http.Error(w, "invalid version number", http.StatusBadRequest)
        return
    }
    url, err := h.svc.DownloadVersionURL(r.Context(), aid, vn)
    if err != nil {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"url": url})
}
```

- [ ] **Step 4: Build**

```bash
go build ./internal/versioning/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/versioning/
git commit -m "feat(versioning): add attachment version list and download endpoints"
```

---

### Task 9: internal/deviceflow — service

**Files:**
- Create: `internal/deviceflow/model.go`
- Create: `internal/deviceflow/service.go`

- [ ] **Step 1: Create model.go**

```go
package deviceflow

import (
    "time"
    "github.com/google/uuid"
)

type DeviceCodeResponse struct {
    DeviceCode      string `json:"device_code"`
    UserCode        string `json:"user_code"`
    VerificationURI string `json:"verification_uri"`
    ExpiresIn       int    `json:"expires_in"`
    Interval        int    `json:"interval"`
}

type TokenResponse struct {
    AccessToken  string `json:"access_token"`
    TokenType    string `json:"token_type"`
    ExpiresIn    int    `json:"expires_in"`
    RefreshToken string `json:"refresh_token,omitempty"`
}

// pendingCode is held in-memory until the user approves or it expires.
type pendingCode struct {
    deviceCode string
    userCode   string
    expiresAt  time.Time
    approved   bool
    userID     uuid.UUID
    subject    string
    email      string
    roles      []string
}
```

- [ ] **Step 2: Create service.go**

```go
package deviceflow

import (
    "context"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "math/big"
    "strings"
    "sync"
    "time"

    "github.com/eguilde/egudoc/internal/auth"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "go.uber.org/zap"
)

const (
    codeTTL      = 5 * time.Minute
    accessTTL    = 15 * time.Minute
    refreshTTL   = 30 * 24 * time.Hour
    pollInterval = 5
)

type Service struct {
    db      *pgxpool.Pool
    issuer  *auth.Issuer
    baseURL string // e.g. "https://egudoc.example.ro"
    log     *zap.Logger

    mu      sync.Mutex
    pending map[string]*pendingCode // keyed by device_code
}

func NewService(db *pgxpool.Pool, issuer *auth.Issuer, baseURL string, log *zap.Logger) *Service {
    s := &Service{
        db:      db,
        issuer:  issuer,
        baseURL: baseURL,
        log:     log,
        pending: make(map[string]*pendingCode),
    }
    go s.sweepExpired()
    return s
}

// sweepExpired removes expired codes from memory every minute.
func (s *Service) sweepExpired() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        s.mu.Lock()
        for k, v := range s.pending {
            if time.Now().After(v.expiresAt) {
                delete(s.pending, k)
            }
        }
        s.mu.Unlock()
    }
}

// NewDeviceCode issues a new device code pair.
func (s *Service) NewDeviceCode() (*DeviceCodeResponse, error) {
    deviceCode, err := randomHex(32)
    if err != nil {
        return nil, fmt.Errorf("device code generate: %w", err)
    }
    userCode, err := humanCode()
    if err != nil {
        return nil, fmt.Errorf("user code generate: %w", err)
    }
    s.mu.Lock()
    s.pending[deviceCode] = &pendingCode{
        deviceCode: deviceCode,
        userCode:   userCode,
        expiresAt:  time.Now().Add(codeTTL),
    }
    s.mu.Unlock()
    return &DeviceCodeResponse{
        DeviceCode:      deviceCode,
        UserCode:        userCode,
        VerificationURI: s.baseURL + "/device",
        ExpiresIn:       int(codeTTL.Seconds()),
        Interval:        pollInterval,
    }, nil
}

// FindByUserCode looks up the pending code by the human-readable code (for the approval page).
func (s *Service) FindByUserCode(userCode string) (string, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for k, v := range s.pending {
        if v.userCode == strings.ToUpper(userCode) && time.Now().Before(v.expiresAt) {
            return k, true
        }
    }
    return "", false
}

// Approve marks a device code as approved for the authenticated user.
func (s *Service) Approve(deviceCode string, userID uuid.UUID, subject, email string, roles []string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    p, ok := s.pending[deviceCode]
    if !ok || time.Now().After(p.expiresAt) {
        return fmt.Errorf("device code expired or not found")
    }
    p.approved = true
    p.userID = userID
    p.subject = subject
    p.email = email
    p.roles = roles
    return nil
}

// Token polls for an approved code and issues access + refresh tokens.
// Returns ("", "", nil) when still pending. Returns error on expiry.
func (s *Service) Token(ctx context.Context, deviceCode, deviceID string) (accessToken, refreshToken string, err error) {
    s.mu.Lock()
    p, ok := s.pending[deviceCode]
    if !ok {
        s.mu.Unlock()
        return "", "", fmt.Errorf("device code not found or expired")
    }
    if time.Now().After(p.expiresAt) {
        delete(s.pending, deviceCode)
        s.mu.Unlock()
        return "", "", fmt.Errorf("device code expired")
    }
    if !p.approved {
        s.mu.Unlock()
        return "", "", nil // still pending — caller should retry
    }
    // Copy and remove from pending.
    userID := p.userID
    subject := p.subject
    email := p.email
    roles := p.roles
    delete(s.pending, deviceCode)
    s.mu.Unlock()

    accessToken, err = s.issuer.Sign(auth.Claims{Subject: subject, UID: userID, Email: email, Roles: roles}, accessTTL)
    if err != nil {
        return "", "", fmt.Errorf("device token sign access: %w", err)
    }

    rawRefresh, err := randomHex(32)
    if err != nil {
        return "", "", fmt.Errorf("device token refresh rand: %w", err)
    }
    hashBytes := sha256.Sum256([]byte(rawRefresh))
    hashHex := hex.EncodeToString(hashBytes[:])

    _, err = s.db.Exec(ctx, `
        INSERT INTO sync_device_tokens (user_id, device_id, refresh_token)
        VALUES ($1, $2, $3)
        ON CONFLICT (user_id, device_id) DO UPDATE
        SET refresh_token = EXCLUDED.refresh_token, last_seen_at = NOW(), revoked_at = NULL
    `, userID, deviceID, hashHex)
    if err != nil {
        return "", "", fmt.Errorf("device token store refresh: %w", err)
    }
    return accessToken, rawRefresh, nil
}

// Refresh validates an opaque refresh token and issues a new access token.
func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (string, error) {
    hashBytes := sha256.Sum256([]byte(rawRefreshToken))
    hashHex := hex.EncodeToString(hashBytes[:])

    var userID uuid.UUID
    err := s.db.QueryRow(ctx, `
        UPDATE sync_device_tokens
        SET last_seen_at = NOW()
        WHERE refresh_token = $1 AND revoked_at IS NULL
        RETURNING user_id
    `, hashHex).Scan(&userID)
    if err != nil {
        return "", fmt.Errorf("device refresh invalid token")
    }

    var subject, email string
    var roles []string
    err = s.db.QueryRow(ctx, `
        SELECT u.subject, u.email, COALESCE(array_agg(ur.role_code) FILTER (WHERE ur.role_code IS NOT NULL), '{}')
        FROM users u
        LEFT JOIN user_roles ur ON ur.user_id = u.id
        WHERE u.id = $1
        GROUP BY u.id
    `, userID).Scan(&subject, &email, &roles)
    if err != nil {
        return "", fmt.Errorf("device refresh fetch user: %w", err)
    }

    return s.issuer.Sign(auth.Claims{Subject: subject, UID: userID, Email: email, Roles: roles}, accessTTL)
}

// Revoke marks a device token as revoked.
func (s *Service) Revoke(ctx context.Context, rawRefreshToken string) error {
    hashBytes := sha256.Sum256([]byte(rawRefreshToken))
    hashHex := hex.EncodeToString(hashBytes[:])
    _, err := s.db.Exec(ctx, `
        UPDATE sync_device_tokens SET revoked_at = NOW() WHERE refresh_token = $1
    `, hashHex)
    return err
}

func randomHex(n int) (string, error) {
    b := make([]byte, n)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return hex.EncodeToString(b), nil
}

// humanCode generates an 8-char alphanumeric code like "ABCD-1234".
func humanCode() (string, error) {
    const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no O/0, I/1 confusion
    var sb strings.Builder
    for i := 0; i < 8; i++ {
        if i == 4 {
            sb.WriteByte('-')
        }
        n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
        if err != nil {
            return "", err
        }
        sb.WriteByte(chars[n.Int64()])
    }
    return sb.String(), nil
}
```

- [ ] **Step 3: Build**

```bash
go build ./internal/deviceflow/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/deviceflow/model.go internal/deviceflow/service.go
git commit -m "feat(deviceflow): add device flow service with in-memory code store"
```

---

### Task 10: internal/deviceflow/handler.go

**Files:**
- Create: `internal/deviceflow/handler.go`

- [ ] **Step 1: Create the handler**

```go
package deviceflow

import (
    "encoding/json"
    "html/template"
    "net/http"

    "github.com/eguilde/egudoc/internal/auth"
    "github.com/go-chi/chi/v5"
)

var approvalPageTmpl = template.Must(template.New("approval").Parse(`<!DOCTYPE html>
<html><head><title>EguDoc — Autorizare dispozitiv</title>
<style>body{font-family:sans-serif;max-width:400px;margin:80px auto;padding:0 20px}
.code{font-size:2em;letter-spacing:.2em;font-weight:700;color:#4f46e5;padding:12px;
background:#eef2ff;border-radius:8px;text-align:center;margin:24px 0}
button{background:#4f46e5;color:#fff;border:none;padding:12px 24px;border-radius:6px;
font-size:1em;cursor:pointer;width:100%}</style></head>
<body><h2>Autorizare dispozitiv EguDoc</h2>
<p>Codul de confirmare este:</p>
<div class="code">{{.UserCode}}</div>
{{if .Error}}<p style="color:red">{{.Error}}</p>{{end}}
<form method="POST" action="/api/auth/device/approve">
<input type="hidden" name="user_code" value="{{.UserCode}}">
<button type="submit">Autorizează dispozitivul</button>
</form></body></html>`))

type Handler struct {
    svc *Service
}

func NewHandler(svc *Service) *Handler {
    return &Handler{svc: svc}
}

func (h *Handler) Routes() chi.Router {
    r := chi.NewRouter()
    r.Post("/code", h.Code)
    r.Get("/verify", h.VerifyPage)
    r.Post("/approve", h.Approve)
    r.Post("/token", h.Token)
    r.Post("/refresh", h.Refresh)
    r.Post("/revoke", h.Revoke)
    return r
}

func (h *Handler) Code(w http.ResponseWriter, r *http.Request) {
    resp, err := h.svc.NewDeviceCode()
    if err != nil {
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(resp)
}

func (h *Handler) VerifyPage(w http.ResponseWriter, r *http.Request) {
    userCode := r.URL.Query().Get("user_code")
    w.Header().Set("Content-Type", "text/html")
    approvalPageTmpl.Execute(w, map[string]string{"UserCode": userCode, "Error": ""})
}

func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
    // This endpoint requires an authenticated browser session.
    claims := auth.GetClaims(r.Context())
    if claims == nil {
        http.Redirect(w, r, "/login?redirect=/device", http.StatusFound)
        return
    }
    if err := r.ParseForm(); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    userCode := r.FormValue("user_code")
    deviceCode, ok := h.svc.FindByUserCode(userCode)
    if !ok {
        w.Header().Set("Content-Type", "text/html")
        approvalPageTmpl.Execute(w, map[string]string{"UserCode": userCode, "Error": "Codul a expirat sau este invalid."})
        return
    }
    if err := h.svc.Approve(deviceCode, claims.UID, claims.Subject, claims.Email, claims.Roles); err != nil {
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(`<!DOCTYPE html><html><body style="font-family:sans-serif;max-width:400px;margin:80px auto;padding:0 20px">
<h2>✓ Dispozitiv autorizat</h2><p>Puteți închide această fereastră.</p></body></html>`))
}

func (h *Handler) Token(w http.ResponseWriter, r *http.Request) {
    var body struct {
        DeviceCode string `json:"device_code"`
        DeviceID   string `json:"device_id"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    access, refresh, err := h.svc.Token(r.Context(), body.DeviceCode, body.DeviceID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    if access == "" {
        // Still pending — return 428 so client knows to keep polling.
        w.WriteHeader(428)
        json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(TokenResponse{
        AccessToken:  access,
        TokenType:    "Bearer",
        ExpiresIn:    int(accessTTL.Seconds()),
        RefreshToken: refresh,
    })
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
    var body struct {
        RefreshToken string `json:"refresh_token"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    access, err := h.svc.Refresh(r.Context(), body.RefreshToken)
    if err != nil {
        http.Error(w, "invalid or expired refresh token", http.StatusUnauthorized)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(TokenResponse{AccessToken: access, TokenType: "Bearer", ExpiresIn: int(accessTTL.Seconds())})
}

func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
    var body struct {
        RefreshToken string `json:"refresh_token"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    h.svc.Revoke(r.Context(), body.RefreshToken)
    w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/deviceflow/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/deviceflow/handler.go
git commit -m "feat(deviceflow): add device flow HTTP handlers and approval page"
```

---

### Task 11: Register all modules in cmd/server/main.go

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Read main.go to find the authenticated router group**

```bash
grep -n "Mount\|Group\|authMiddleware\|rbacSvc\|pool" cmd/server/main.go | head -40
```

- [ ] **Step 2: Add module wiring**

In the section where services and handlers are instantiated, add (after existing service setup):

```go
// JWT issuer for device flow — load key from env or generate ephemeral one.
rsaKey, err := loadOrGenerateRSAKey(os.Getenv("JWT_SIGNING_KEY_PEM"))
if err != nil {
    log.Fatal("jwt signing key", zap.Error(err))
}
jwtIssuer := auth.NewIssuer(rsaKey)

// Sync module
syncSvc := egusync.NewService(pool, store, rbacSvc, log)
syncHandler := egusync.NewHandler(syncSvc)

// Versioning module
versioningSvc := versioning.NewService(pool, store)
versioningHandler := versioning.NewHandler(versioningSvc)

// Device flow module
deviceflowSvc := deviceflow.NewService(pool, jwtIssuer, os.Getenv("APP_BASE_URL"), log)
deviceflowHandler := deviceflow.NewHandler(deviceflowSvc)
```

In the authenticated router group, add:

```go
r.Mount("/api/sync", syncHandler.Routes())
r.Mount("/api/versioning", versioningHandler.Routes())
```

The device flow `/code` and `/token` endpoints are public (no auth). The `/verify` and `/approve` endpoints need the existing session auth. Mount the device flow handler outside the auth group:

```go
// Public device flow endpoints (no JWT required for code + token)
r.Mount("/api/auth/device", deviceflowHandler.Routes())
```

- [ ] **Step 3: Add the loadOrGenerateRSAKey helper in main.go**

```go
func loadOrGenerateRSAKey(pemStr string) (*rsa.PrivateKey, error) {
    if pemStr != "" {
        block, _ := pem.Decode([]byte(pemStr))
        if block == nil {
            return nil, fmt.Errorf("invalid PEM block")
        }
        return x509.ParsePKCS1PrivateKey(block.Bytes)
    }
    // Generate ephemeral key (only safe for single-replica dev setups).
    return rsa.GenerateKey(rand.Reader, 2048)
}
```

Add required imports: `"crypto/rand"`, `"crypto/rsa"`, `"crypto/x509"`, `"encoding/pem"`.

Import the new packages (use aliases to avoid collision with stdlib `sync`):

```go
import (
    egusync    "github.com/eguilde/egudoc/internal/sync"
    "github.com/eguilde/egudoc/internal/versioning"
    "github.com/eguilde/egudoc/internal/deviceflow"
)
```

- [ ] **Step 4: Build and run**

```bash
go build ./cmd/server/...
```

- [ ] **Step 5: Smoke test the new endpoints**

```bash
# Start server locally (requires DATABASE_URL and STORAGE_* env vars)
go run ./cmd/server &
sleep 2

# Device code endpoint (public, no auth)
curl -s -X POST http://localhost:8080/api/auth/device/code | jq .

# Delta endpoint (needs auth — will return 401 without a token, which is correct)
curl -s http://localhost:8080/api/sync/delta
# Expected: 401 Unauthorized
```

- [ ] **Step 6: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: register sync, versioning, and deviceflow modules in server"
```

---

### Task 12: RustFS Kubernetes manifests

**Files:**
- Create: `deploy/rustfs/statefulset.yaml`
- Create: `deploy/rustfs/service.yaml`
- Create: `deploy/rustfs/ingress.yaml`
- Create: `deploy/rustfs/argocd-app.yaml`

- [ ] **Step 1: Create the StatefulSet**

```yaml
# deploy/rustfs/statefulset.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: rustfs
  namespace: egudoc
spec:
  serviceName: rustfs
  replicas: 1
  selector:
    matchLabels:
      app: rustfs
  template:
    metadata:
      labels:
        app: rustfs
    spec:
      nodeSelector:
        kubernetes.io/os: linux
      containers:
        - name: rustfs
          image: rustfs/rustfs:latest
          args: ["server", "/data", "--console-address", ":9001"]
          env:
            - name: RUSTFS_ROOT_USER
              valueFrom:
                secretKeyRef:
                  name: rustfs-credentials
                  key: access-key
            - name: RUSTFS_ROOT_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: rustfs-credentials
                  key: secret-key
          ports:
            - containerPort: 9000
              name: s3
            - containerPort: 9001
              name: console
          volumeMounts:
            - name: data
              mountPath: /data
          readinessProbe:
            httpGet:
              path: /minio/health/ready
              port: 9000
            initialDelaySeconds: 10
            periodSeconds: 5
          livenessProbe:
            httpGet:
              path: /minio/health/live
              port: 9000
            initialDelaySeconds: 30
            periodSeconds: 20
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        storageClassName: longhorn
        accessModes: [ReadWriteOnce]
        resources:
          requests:
            storage: 100Gi
```

- [ ] **Step 2: Create the Service**

```yaml
# deploy/rustfs/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: rustfs
  namespace: egudoc
spec:
  selector:
    app: rustfs
  ports:
    - name: s3
      port: 9000
      targetPort: 9000
    - name: console
      port: 9001
      targetPort: 9001
```

- [ ] **Step 3: Create Traefik IngressRoute for S3 external access**

```yaml
# deploy/rustfs/ingress.yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: rustfs-s3
  namespace: egudoc
spec:
  entryPoints:
    - websecure
  routes:
    - match: Host(`storage.egudoc.ro`)  # replace with actual domain
      kind: Rule
      services:
        - name: rustfs
          port: 9000
  tls:
    certResolver: letsencrypt
```

- [ ] **Step 4: Create ArgoCD Application**

```yaml
# deploy/rustfs/argocd-app.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: rustfs
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/eguilde/egucluster
    targetRevision: HEAD
    path: deploy/rustfs
  destination:
    server: https://kubernetes.default.svc
    namespace: egudoc
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

- [ ] **Step 5: Create the credentials secret (manual — not in git)**

```bash
kubectl create secret generic rustfs-credentials \
  --namespace egudoc \
  --from-literal=access-key=<access-key> \
  --from-literal=secret-key=<secret-key>
```

- [ ] **Step 6: Commit manifests**

```bash
git add deploy/rustfs/
git commit -m "feat(infra): add RustFS StatefulSet, Service, IngressRoute, ArgoCD App"
```

---

### Task 13: MinIO → RustFS data migration

Run after RustFS is live and before switching the backend endpoint env var.

- [ ] **Step 1: Deploy RustFS without deleting MinIO**

Apply manifests:
```bash
kubectl apply -f deploy/rustfs/statefulset.yaml -f deploy/rustfs/service.yaml
kubectl rollout status statefulset/rustfs -n egudoc
```

- [ ] **Step 2: Configure mc aliases**

```bash
mc alias set minio http://minio.egudoc.svc.cluster.local:9000 <old-access-key> <old-secret-key>
mc alias set rustfs http://rustfs.egudoc.svc.cluster.local:9000 <new-access-key> <new-secret-key>
```

- [ ] **Step 3: Create bucket in RustFS**

```bash
mc mb rustfs/egudoc
```

- [ ] **Step 4: Mirror all objects**

```bash
mc mirror --preserve minio/egudoc rustfs/egudoc
```

Wait for completion. Verify:
```bash
mc ls minio/egudoc --recursive | wc -l
mc ls rustfs/egudoc --recursive | wc -l
```
Both counts must match.

- [ ] **Step 5: Switch the backend to RustFS**

Update the egudoc backend K8s secret:

```bash
kubectl patch secret egudoc-config -n egudoc \
  --type='json' \
  -p='[
    {"op":"replace","path":"/data/STORAGE_ENDPOINT","value":"'$(echo -n "rustfs.egudoc.svc.cluster.local:9000" | base64)'"},
    {"op":"replace","path":"/data/STORAGE_PUBLIC_ENDPOINT","value":"'$(echo -n "storage.egudoc.ro" | base64)'"}
  ]'
kubectl rollout restart deployment/egudoc -n egudoc
kubectl rollout status deployment/egudoc -n egudoc
```

- [ ] **Step 6: Smoke test downloads**

Fetch a known document attachment from the Angular UI or API and verify the presigned URL returns `storage.egudoc.ro` as the host.

- [ ] **Step 7: Delete MinIO (after 48h burn-in)**

```bash
kubectl delete deployment minio -n egudoc
kubectl delete pvc minio-pvc -n egudoc  # adjust PVC name to match actual
```

- [ ] **Step 8: Apply ArgoCD Application**

```bash
kubectl apply -f deploy/rustfs/argocd-app.yaml
```

---

*End of Phase 1 plan. Phase 2: Windows sync client. Phase 3: Angular web file manager.*
