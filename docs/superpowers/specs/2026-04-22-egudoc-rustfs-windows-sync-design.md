# EguDoc — RustFS Storage + Windows Native Sync Client Design

**Date:** 2026-04-22  
**Status:** Approved  
**Author:** Thomas Galambos

---

## 1. Scope

This document specifies:

1. Replacing MinIO with RustFS as the S3-compatible object storage backend
2. A new Go backend `sync/`, `versioning/`, and `deviceflow/` modules
3. Two new database migrations
4. A Rust-based Windows native sync client (`egudoc-sync.exe`) using the Cloud Filter API
5. Targeted Angular frontend additions (version history, sync status badge, draft-completion notification)

Registratura, workflow, approval chains, and archiveworker pipelines are **not changed** by this spec.

---

## 2. Architecture Overview

Four layers. Presigned URL hybrid. RBAC enforced at every boundary.

```
┌─────────────────────────────────────────────────────────┐
│ CLIENT LAYER                                            │
│  Angular Web App (modified)  │  egudoc-sync.exe (new)   │
└────────────────────┬────────────────────┬───────────────┘
                     │ HTTPS / OAuth2 JWT  │
┌────────────────────▼────────────────────▼───────────────┐
│ GO API LAYER — egudoc backend                           │
│  Registratura (unchanged)  │  Sync API (new)            │
│  Versioning (new)          │  Device Flow (new)         │
└────────────────────┬────────────────────────────────────┘
        pgx metadata │        S3 presigned URLs (blobs)
┌───────────────────▼──────────────────────────────────┐
│ STORAGE LAYER                                        │
│  RustFS StatefulSet (replaces MinIO)  │  PostgreSQL  │
└──────────────────────────────────────────────────────┘
```

**Key principle:** File bytes never flow through the Go pod. Go API validates RBAC and issues short-lived presigned URLs; the client streams directly to/from RustFS.

---

## 3. RustFS Deployment

### 3.1 What changes

RustFS is a Rust-based S3-compatible object store. It is a drop-in replacement for MinIO using the same `minio-go/v7` SDK.

- `internal/storage/minio.go` — no code changes. The constructor reads `STORAGE_ENDPOINT` from env. Only that env var changes in the K8s secret.
- The MinIO Deployment and its PVC are removed. A new ArgoCD Application `rustfs` manages a StatefulSet.

### 3.2 K8s StatefulSet

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: rustfs
  namespace: egudoc
spec:
  replicas: 1
  selector:
    matchLabels:
      app: rustfs
  template:
    spec:
      containers:
        - name: rustfs
          image: rustfs/rustfs:latest
          ports:
            - containerPort: 9000   # S3 API
            - containerPort: 9001   # Web console
          volumeMounts:
            - name: data
              mountPath: /data
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

Node placement: Longhorn PVCs remain on egucluster3/egucluster4 (no change from MinIO).

### 3.3 Migration path

1. Stand up RustFS StatefulSet alongside MinIO (different service name `rustfs`)
2. Copy all existing objects: `mc mirror minio/egudoc rustfs/egudoc`
3. Switch `STORAGE_ENDPOINT` in the egudoc backend K8s secret → rolling restart
4. Verify existing document downloads via smoke test
5. Delete MinIO Deployment and old PVC

---

## 4. Database Migrations

Latest existing migration: `000008_approval_chains.up.sql`

### 4.1 `000009_attachment_versions.up.sql`

```sql
ALTER TABLE atasamente ADD COLUMN current_version_nr INTEGER NOT NULL DEFAULT 1;

CREATE TABLE atasament_versiuni (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    atasament_id    UUID        NOT NULL REFERENCES atasamente(id),
    version_nr      INTEGER     NOT NULL,
    storage_key     TEXT        NOT NULL,
    size_bytes      BIGINT      NOT NULL,
    sha256          TEXT        NOT NULL,
    uploaded_by     UUID        NOT NULL REFERENCES users(id),
    source          TEXT        NOT NULL CHECK (source IN ('web', 'windows_sync')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ON atasament_versiuni(atasament_id, version_nr);
```

When a new version is saved, a row is appended and `current_version_nr` is incremented atomically in one transaction.

### 4.2 `000010_sync_device_tokens.up.sql`

```sql
CREATE TABLE sync_device_tokens (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id),
    device_id       TEXT        NOT NULL,
    refresh_token   TEXT        NOT NULL,   -- stored as SHA-256 hash, never plaintext
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at      TIMESTAMPTZ
);

CREATE UNIQUE INDEX ON sync_device_tokens(user_id, device_id);
```

---

## 5. Go Backend — New Modules

All new modules follow the existing pattern: `module.go` (routes), `handler.go` (HTTP layer), `service.go` (business logic).

### 5.1 `internal/sync/`

Registered under `/api/sync`.

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/sync/delta` | RBAC-filtered document+attachment index. `?since=<rfc3339>` for incremental. Returns stubs + attachment metadata including `sync_status`. |
| GET | `/api/sync/download/:aid` | Assert READ permission on attachment's document `entitate_id` → return presigned GET URL (5 min TTL). |
| POST | `/api/sync/upload-intent` | Assert WRITE permission → generate new storage key `atasamente/{doc_id}/{uuid}/{filename}` → return presigned PUT URL (15 min TTL) + `storage_key`. |
| POST | `/api/sync/upload-confirm` | HEAD-check object exists in RustFS → insert `atasament_versiuni` row + increment `current_version_nr` in one transaction. |
| POST | `/api/sync/validate-move` | Load source document's `entitate_id` and destination path's `entitate_id`. Return 403 if they differ or if user lacks WRITE on source. |

RBAC checks reuse the existing `rbac` package — no duplicated permission logic.

### 5.2 `internal/versioning/`

Registered under `/api/versioning`.

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/versioning/:aid/versions` | All `atasament_versiuni` rows for attachment, ordered `version_nr DESC`. Used by Angular version history panel. |
| GET | `/api/versioning/:aid/versions/:vn` | Presigned GET URL for a specific version's `storage_key`. |

### 5.3 `internal/deviceflow/`

Registered under `/api/auth/device`.

| Method | Route | Description |
|--------|-------|-------------|
| POST | `/api/auth/device/code` | Generate `device_code` (32-byte random, 5-min in-memory TTL) + `user_code` (8-char human-readable). Return both with `verification_uri`. |
| GET | `/api/auth/device/verify` | Browser endpoint — renders approval page, requires existing user session cookie. |
| POST | `/api/auth/device/approve` | Marks `device_code` as approved for `user_id`. |
| POST | `/api/auth/device/token` | Polling endpoint. If approved: issue JWT (15 min) + refresh token (opaque, 30d). Insert `sync_device_tokens` row with SHA-256 hash of refresh token. |
| POST | `/api/auth/device/refresh` | Validate refresh token hash against `sync_device_tokens` → issue new JWT. |
| POST | `/api/auth/device/revoke` | Set `revoked_at` on `sync_device_tokens` row. |

**Pending codes** are held in-memory (`sync.Map` with expiry goroutine). No Redis dependency. If the backend restarts during the 5-minute device flow window, the user retries — acceptable for this use case.

**JWT claims** match the existing RBAC middleware structure (`user_id`, `entitate_id`, `roles`). No middleware changes needed.

---

## 6. Windows Sync Client — `egudoc-sync`

Standalone Rust binary. Implements the same Cloud Filter API pattern as the Windows CloudMirror sample and OneDrive.

### 6.1 Project structure

```
egudoc-sync/
  Cargo.toml
  src/
    main.rs
    provider.rs                        # CfApi sync root registration + callback dispatch
    callbacks/
      fetch_data.rs                    # CF_CALLBACK_TYPE_FETCH_DATA (hydration)
      fetch_placeholders.rs            # CF_CALLBACK_TYPE_FETCH_PLACEHOLDERS
      notify_rename.rs                 # CF_CALLBACK_TYPE_NOTIFY_RENAME (RBAC move guard)
      notify_delete.rs                 # CF_CALLBACK_TYPE_NOTIFY_DELETE
      notify_close.rs                  # CF_CALLBACK_TYPE_NOTIFY_FILE_CLOSE_COMPLETION
    sync/
      engine.rs                        # delta polling loop, reconcile state
      delta.rs                         # /api/sync/delta client
      upload.rs                        # upload-intent → PUT → upload-confirm
      download.rs                      # download → presigned GET → hydrate
    auth/
      device_flow.rs                   # POST /device/code, poll /device/token, store in CredMan
      token.rs                         # JWT cache + refresh on expiry
    rbac.rs                            # validate-move call, 5-min department boundary cache
    api.rs                             # typed HTTP client (reqwest), all Go API calls
    config.rs                          # reads %APPDATA%\EguDoc\config.toml
```

**Cargo dependencies:**
- `windows` (windows-rs) — CfApi, CredMan
- `reqwest` — async HTTP
- `tokio` — async runtime
- `serde` / `serde_json`
- `toml`

### 6.2 CfApi callbacks

**`notify_rename.rs`** — fires before the move completes. Calls `POST /api/sync/validate-move` synchronously. If 403 → returns `HRESULT` error to Windows → Explorer shows "You don't have permission to move this item." File stays in place.

**`notify_close.rs`** — fires when a process closes a file it had open for write (Word/Excel/PowerPoint saving). Reads local file → `POST /api/sync/upload-intent` → streams bytes to presigned PUT URL → `POST /api/sync/upload-confirm`. This is how in-place editing creates new versions with no user action required.

**`fetch_data.rs`** — fires when a placeholder is opened for read. Downloads bytes from presigned GET URL → writes into placeholder via `CfExecute(CF_OPERATION_TYPE_TRANSFER_DATA)`. File becomes "local" after this.

**`fetch_placeholders.rs`** — fires when a folder is expanded. Calls `/api/sync/delta` for that folder scope → `CfCreatePlaceholders` for any files not yet present.

### 6.3 Sync engine

- Polls `/api/sync/delta?since={last_sync_ts}` every 30 seconds
- New/updated attachments → create or update placeholders (dehydrate if content changed server-side)
- Deleted attachments → `CfDeletePlaceholders`
- Stores `last_sync_ts` in `config.toml`

### 6.4 OAuth2 device flow sequence

```
1. egudoc-sync.exe  →  POST /api/auth/device/code
                    ←  { device_code, user_code, verification_uri, expires_in: 300 }

2. Windows notification: "Open https://<host>/device and enter: ABCD-1234"

3. User authenticates in browser, enters user_code, approves

4. egudoc-sync.exe  →  POST /api/auth/device/token { device_code }
   (polls every 5s until approved or expired)
                    ←  { access_token (JWT 15min), refresh_token (opaque 30d) }

5. Client stores refresh_token in Windows Credential Manager
   (CredWrite — keyed to "EguDoc:{user_id}")
```

### 6.5 Virtual folder structure

```
EguDoc (sync root)
├── Documentele mele/                      ← virtual: docs where assignee_id = current user
│   └── RG-2026-0043 - Contract servicii.docx
├── Registrul General/
│   └── 2026/
│       └── RG-2026-0043 - Contract servicii.docx
└── Registrul Contracte/
    └── 2026/
        └── CT-2026-0001 - Contract.docx
```

"Documentele mele" is populated from the delta response filtering `user_curent_subject = current_user_subject` (the `documente.user_curent_subject` field, which tracks the user currently responsible for the document). The same physical placeholder exists in both locations — no duplication. The sync engine maps both virtual paths to the same `atasament_id` via the CfApi identity blob.

### 6.6 RBAC enforcement

- Move/rename across department folders: blocked at `notify_rename` callback via `POST /api/sync/validate-move`
- Delete: `notify_delete` callback calls a permission check; returns error if user lacks DELETE permission
- Department boundaries are cached in-memory for 5 minutes to avoid a round-trip per explorer action

---

## 7. Angular Frontend Changes

Minimal additions. No new modules, no routing changes, no auth changes.

### 7.1 Version history panel

Added to the existing attachment detail view. Calls `GET /api/versioning/:aid/versions`. Displays version list with version number, date, uploader, and source (`web` / `windows_sync`). Each row has a "Download" button that calls `GET /api/versioning/:aid/versions/:vn`.

### 7.2 Sync status badge

Small badge on each attachment row showing `sync_status` from the delta response:
- `placeholder` — file is virtual on Windows client
- `local` — file is hydrated locally
- `uploading` — upload in progress
- `conflict` — server version newer than local (manual resolution required)

Badge is informational only for web users — no action.

### 7.3 Draft-completion notification

When `upload-confirm` creates a new version, the Angular app picks it up on the next document poll cycle and shows a toast: `"Fișier actualizat de pe Windows — versiunea {n} disponibilă"`.

If `document.status` is `INREGISTRAT` at upload-confirm time, the backend sets it to `IN_LUCRU`. The Angular app already handles `IN_LUCRU` — no new state needed.

---

## 8. What Does NOT Change

- `internal/registratura/` — untouched
- `internal/workflow/` — untouched
- `internal/archiveworker/` — untouched (already uses `storage.Client` which will point to RustFS via env var)
- `internal/storage/minio.go` — no code changes, only env var
- Angular routing, auth, registratura CRUD — untouched
- Kubernetes: Traefik, Istio, ArgoCD, Longhorn node placement — unchanged

---

## 9. Open Questions

None. All decisions have been made and approved.
