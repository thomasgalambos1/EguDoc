# EguDoc — Windows Native Sync Client (egudoc-sync) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `egudoc-sync.exe` — a Rust Windows binary that presents EguDoc documents as a virtual drive in File Explorer using the Cloud Filter API (same model as OneDrive). The Windows client is **optional** — EguDoc works fully without it.

**Architecture:** The binary registers a sync root via CfApi, serves placeholder files for all visible documents, hydrates files on demand via presigned GET URLs, and uploads new versions via presigned PUT URLs whenever a file is closed after a write. RBAC is enforced at the `NOTIFY_RENAME` callback — cross-department moves are blocked before they complete.

**Tech Stack:** Rust (stable), windows-rs (CfApi + CredMan), reqwest + tokio (async HTTP), serde_json, toml

**Prerequisites:** Phase 1 Go backend must be deployed. The sync client talks to the Go API — it does not connect to RustFS directly.

---

## File Map

```
egudoc-sync/
├── Cargo.toml
└── src/
    ├── main.rs                      # entry: init logging, load config, start engine + provider
    ├── config.rs                    # read/write %APPDATA%\EguDoc\config.toml
    ├── api.rs                       # typed HTTP client wrapping all Go API calls
    ├── rbac.rs                      # validate-move with 5-min in-memory cache
    ├── provider.rs                  # CfApi sync root registration + callback dispatch
    ├── auth/
    │   ├── mod.rs
    │   ├── device_flow.rs           # POST /device/code, poll /device/token, store in CredMan
    │   └── token.rs                 # JWT cache + refresh on expiry
    ├── sync/
    │   ├── mod.rs
    │   ├── engine.rs                # 30s delta polling loop, reconcile placeholders
    │   ├── delta.rs                 # deserialize delta response
    │   ├── upload.rs                # upload-intent → PUT → upload-confirm
    │   └── download.rs              # download endpoint → presigned GET
    └── callbacks/
        ├── mod.rs
        ├── fetch_data.rs            # FETCH_DATA: hydrate placeholder on open
        ├── fetch_placeholders.rs    # FETCH_PLACEHOLDERS: populate folder on expand
        ├── notify_rename.rs         # NOTIFY_RENAME: block cross-dept moves
        ├── notify_close.rs          # NOTIFY_FILE_CLOSE_COMPLETION: detect save → upload
        └── notify_delete.rs         # NOTIFY_DELETE: permission check
```

---

### Task 1: Cargo project scaffold

**Files:**
- Create: `egudoc-sync/Cargo.toml`
- Create: `egudoc-sync/src/main.rs` (stub)

- [ ] **Step 1: Create the project**

```bash
cargo new egudoc-sync
cd egudoc-sync
```

- [ ] **Step 2: Write Cargo.toml**

```toml
[package]
name = "egudoc-sync"
version = "0.1.0"
edition = "2021"

[[bin]]
name = "egudoc-sync"
path = "src/main.rs"

[dependencies]
windows = { version = "0.58", features = [
    "Win32_Storage_CloudFilters",
    "Win32_Security_Credentials",
    "Win32_Foundation",
    "Win32_System_Com",
    "Win32_UI_Shell",
    "Win32_System_Threading",
] }
tokio = { version = "1", features = ["full"] }
reqwest = { version = "0.12", features = ["json", "rustls-tls"], default-features = false }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
toml = "0.8"
dirs = "5"
chrono = { version = "0.4", features = ["serde"] }
uuid = { version = "1", features = ["v4", "serde"] }
tracing = "0.1"
tracing-subscriber = { version = "0.3", features = ["env-filter"] }
anyhow = "1"
sha2 = "0.10"
hex = "0.4"
base64 = "0.22"

[target.'cfg(windows)'.dependencies]
windows-credentials = "0.1"  # wraps CredWrite/CredRead

[profile.release]
opt-level = 3
lto = true
codegen-units = 1
```

- [ ] **Step 3: Write minimal main.rs to prove it compiles**

```rust
#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(tracing_subscriber::EnvFilter::from_default_env())
        .init();
    tracing::info!("egudoc-sync starting");
    Ok(())
}
```

- [ ] **Step 4: Build**

```bash
cargo build
```
Expected: compiles with warnings only (unused imports acceptable at this stage).

- [ ] **Step 5: Commit**

```bash
git add egudoc-sync/
git commit -m "feat(sync-client): scaffold egudoc-sync Cargo project"
```

---

### Task 2: config.rs — read and write config.toml

**Files:**
- Create: `egudoc-sync/src/config.rs`

- [ ] **Step 1: Write the test**

```rust
// In egudoc-sync/src/config.rs, at bottom:
#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::tempdir;

    #[test]
    fn round_trip() {
        let dir = tempdir().unwrap();
        let path = dir.path().join("config.toml");

        let cfg = Config {
            api_base_url: "https://api.test".into(),
            sync_root: dir.path().join("EguDoc").to_string_lossy().into(),
            last_sync_ts: None,
            device_id: "test-device".into(),
        };
        cfg.save(&path).unwrap();

        let loaded = Config::load(&path).unwrap();
        assert_eq!(loaded.api_base_url, cfg.api_base_url);
        assert_eq!(loaded.device_id, cfg.device_id);
        assert!(loaded.last_sync_ts.is_none());
    }
}
```

Add `tempfile` to `[dev-dependencies]` in Cargo.toml:
```toml
[dev-dependencies]
tempfile = "3"
```

- [ ] **Step 2: Run to confirm failure**

```bash
cargo test config -- --nocapture 2>&1 | head -20
```
Expected: compile error — module not declared.

- [ ] **Step 3: Create config.rs**

```rust
use anyhow::{Context, Result};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Config {
    pub api_base_url: String,
    pub sync_root: String,
    pub last_sync_ts: Option<DateTime<Utc>>,
    pub device_id: String,
}

impl Config {
    /// Load config from path. Returns a default if the file doesn't exist.
    pub fn load(path: &Path) -> Result<Self> {
        if !path.exists() {
            return Ok(Self::default());
        }
        let raw = std::fs::read_to_string(path)
            .with_context(|| format!("read config {:?}", path))?;
        toml::from_str(&raw).context("parse config")
    }

    pub fn save(&self, path: &Path) -> Result<()> {
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent).context("create config dir")?;
        }
        let raw = toml::to_string_pretty(self).context("serialize config")?;
        std::fs::write(path, raw).context("write config")
    }

    /// Returns the default config path: %APPDATA%\EguDoc\config.toml
    pub fn default_path() -> PathBuf {
        dirs::config_dir()
            .unwrap_or_else(|| PathBuf::from("."))
            .join("EguDoc")
            .join("config.toml")
    }
}

impl Default for Config {
    fn default() -> Self {
        Self {
            api_base_url: "https://egudoc.ro".into(),
            sync_root: dirs::home_dir()
                .unwrap_or_default()
                .join("EguDoc")
                .to_string_lossy()
                .into(),
            last_sync_ts: None,
            device_id: uuid::Uuid::new_v4().to_string(),
        }
    }
}
```

Declare the module in `main.rs`:
```rust
mod config;
```

- [ ] **Step 4: Run tests**

```bash
cargo test config
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add egudoc-sync/src/config.rs egudoc-sync/src/main.rs egudoc-sync/Cargo.toml
git commit -m "feat(sync-client): add config module with load/save support"
```

---

### Task 3: api.rs — typed HTTP client for all Go API calls

**Files:**
- Create: `egudoc-sync/src/api.rs`

- [ ] **Step 1: Create api.rs**

```rust
use anyhow::{bail, Context, Result};
use chrono::{DateTime, Utc};
use reqwest::{Client, StatusCode};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

// ── Response types ────────────────────────────────────────────────────────────

#[derive(Debug, Deserialize)]
pub struct DeltaAtasament {
    pub id: Uuid,
    pub filename: String,
    pub content_type: String,
    pub size_bytes: i64,
    pub current_version: i32,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Deserialize)]
pub struct DeltaDocument {
    pub id: Uuid,
    pub nr_inregistrare: String,
    pub registru_id: Uuid,
    pub registru_nume: String,
    pub tip: String,
    pub status: String,
    pub obiect: String,
    pub updated_at: DateTime<Utc>,
    pub atasamente: Vec<DeltaAtasament>,
}

#[derive(Debug, Deserialize)]
pub struct DownloadUrlResponse {
    pub url: String,
}

#[derive(Debug, Serialize)]
pub struct UploadIntentRequest {
    pub document_id: Uuid,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub atasament_id: Option<Uuid>,
    pub filename: String,
    pub content_type: String,
    pub size_bytes: i64,
}

#[derive(Debug, Deserialize)]
pub struct UploadIntentResponse {
    pub upload_url: String,
    pub storage_key: String,
}

#[derive(Debug, Serialize)]
pub struct UploadConfirmRequest {
    pub document_id: Uuid,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub atasament_id: Option<Uuid>,
    pub storage_key: String,
    pub filename: String,
    pub content_type: String,
    pub size_bytes: i64,
    pub sha256: String,
    pub source: String,
}

#[derive(Debug, Deserialize)]
pub struct UploadConfirmResponse {
    pub atasament_id: Uuid,
    pub version_nr: i32,
}

#[derive(Debug, Serialize)]
pub struct ValidateMoveRequest {
    pub attachment_id: Uuid,
    pub dest_document_id: Uuid,
}

#[derive(Debug, Deserialize)]
pub struct DeviceCodeResponse {
    pub device_code: String,
    pub user_code: String,
    pub verification_uri: String,
    pub expires_in: u64,
    pub interval: u64,
}

#[derive(Debug, Deserialize)]
pub struct TokenResponse {
    pub access_token: String,
    pub token_type: String,
    pub expires_in: u64,
    #[serde(default)]
    pub refresh_token: Option<String>,
}

// ── Client ────────────────────────────────────────────────────────────────────

pub struct ApiClient {
    http: Client,
    base_url: String,
    institution_id: String,
}

impl ApiClient {
    pub fn new(base_url: String, institution_id: String) -> Self {
        let http = Client::builder()
            .timeout(std::time::Duration::from_secs(30))
            .build()
            .expect("build reqwest client");
        Self { http, base_url, institution_id }
    }

    fn auth_headers(&self, token: &str) -> reqwest::header::HeaderMap {
        let mut h = reqwest::header::HeaderMap::new();
        h.insert("Authorization", format!("Bearer {token}").parse().unwrap());
        h.insert("X-Institution-ID", self.institution_id.parse().unwrap());
        h
    }

    pub async fn delta(
        &self,
        token: &str,
        since: Option<DateTime<Utc>>,
    ) -> Result<Vec<DeltaDocument>> {
        let mut url = format!("{}/api/sync/delta", self.base_url);
        if let Some(ts) = since {
            url = format!("{}?since={}", url, ts.to_rfc3339());
        }
        let resp = self.http.get(&url)
            .headers(self.auth_headers(token))
            .send().await.context("delta request")?;
        if !resp.status().is_success() {
            bail!("delta HTTP {}", resp.status());
        }
        resp.json().await.context("delta parse")
    }

    pub async fn download_url(&self, token: &str, attachment_id: Uuid) -> Result<String> {
        let url = format!("{}/api/sync/download/{}", self.base_url, attachment_id);
        let resp = self.http.get(&url)
            .headers(self.auth_headers(token))
            .send().await.context("download_url request")?;
        if !resp.status().is_success() {
            bail!("download_url HTTP {}", resp.status());
        }
        let body: DownloadUrlResponse = resp.json().await.context("download_url parse")?;
        Ok(body.url)
    }

    pub async fn upload_intent(&self, token: &str, req: &UploadIntentRequest) -> Result<UploadIntentResponse> {
        let url = format!("{}/api/sync/upload-intent", self.base_url);
        let resp = self.http.post(&url)
            .headers(self.auth_headers(token))
            .json(req)
            .send().await.context("upload_intent")?;
        if !resp.status().is_success() {
            bail!("upload_intent HTTP {}", resp.status());
        }
        resp.json().await.context("upload_intent parse")
    }

    pub async fn upload_confirm(&self, token: &str, req: &UploadConfirmRequest) -> Result<UploadConfirmResponse> {
        let url = format!("{}/api/sync/upload-confirm", self.base_url);
        let resp = self.http.post(&url)
            .headers(self.auth_headers(token))
            .json(req)
            .send().await.context("upload_confirm")?;
        if !resp.status().is_success() {
            bail!("upload_confirm HTTP {}", resp.status());
        }
        resp.json().await.context("upload_confirm parse")
    }

    pub async fn validate_move(&self, token: &str, req: &ValidateMoveRequest) -> Result<bool> {
        let url = format!("{}/api/sync/validate-move", self.base_url);
        let resp = self.http.post(&url)
            .headers(self.auth_headers(token))
            .json(req)
            .send().await.context("validate_move")?;
        Ok(resp.status() != StatusCode::FORBIDDEN)
    }

    pub async fn device_code(&self) -> Result<DeviceCodeResponse> {
        let url = format!("{}/api/auth/device/code", self.base_url);
        let resp = self.http.post(&url).send().await.context("device_code")?;
        resp.json().await.context("device_code parse")
    }

    /// Returns Some(TokenResponse) if approved, None if still pending.
    pub async fn device_token(&self, device_code: &str, device_id: &str) -> Result<Option<TokenResponse>> {
        let url = format!("{}/api/auth/device/token", self.base_url);
        let body = serde_json::json!({ "device_code": device_code, "device_id": device_id });
        let resp = self.http.post(&url).json(&body).send().await.context("device_token")?;
        if resp.status().as_u16() == 428 {
            return Ok(None); // still pending
        }
        if !resp.status().is_success() {
            bail!("device_token HTTP {}", resp.status());
        }
        Ok(Some(resp.json().await.context("device_token parse")?))
    }

    pub async fn refresh_token(&self, refresh_token: &str) -> Result<TokenResponse> {
        let url = format!("{}/api/auth/device/refresh", self.base_url);
        let body = serde_json::json!({ "refresh_token": refresh_token });
        let resp = self.http.post(&url).json(&body).send().await.context("refresh")?;
        if !resp.status().is_success() {
            bail!("refresh HTTP {}", resp.status());
        }
        resp.json().await.context("refresh parse")
    }
}
```

Declare module in main.rs:
```rust
mod api;
```

- [ ] **Step 2: Build**

```bash
cargo build 2>&1 | grep -E "^error"
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add egudoc-sync/src/api.rs egudoc-sync/src/main.rs
git commit -m "feat(sync-client): add typed API client for all Go backend endpoints"
```

---

### Task 4: auth/token.rs — JWT cache and refresh

**Files:**
- Create: `egudoc-sync/src/auth/mod.rs`
- Create: `egudoc-sync/src/auth/token.rs`

- [ ] **Step 1: Create the token cache**

```rust
// egudoc-sync/src/auth/token.rs
use anyhow::Result;
use chrono::{DateTime, Utc};
use std::sync::Arc;
use tokio::sync::RwLock;
use crate::api::ApiClient;

#[derive(Clone)]
pub struct TokenCache {
    inner: Arc<RwLock<CacheInner>>,
    api: Arc<ApiClient>,
}

struct CacheInner {
    access_token: String,
    expires_at: DateTime<Utc>,
    refresh_token: String,
}

impl TokenCache {
    pub fn new(api: Arc<ApiClient>, access_token: String, expires_in_secs: u64, refresh_token: String) -> Self {
        let expires_at = Utc::now() + chrono::Duration::seconds(expires_in_secs as i64 - 60);
        Self {
            inner: Arc::new(RwLock::new(CacheInner { access_token, expires_at, refresh_token })),
            api,
        }
    }

    /// Returns a valid access token, refreshing if within 60s of expiry.
    pub async fn token(&self) -> Result<String> {
        {
            let inner = self.inner.read().await;
            if Utc::now() < inner.expires_at {
                return Ok(inner.access_token.clone());
            }
        }
        // Token expired — refresh.
        let mut inner = self.inner.write().await;
        // Re-check after acquiring write lock (another task may have refreshed).
        if Utc::now() < inner.expires_at {
            return Ok(inner.access_token.clone());
        }
        let resp = self.api.refresh_token(&inner.refresh_token).await?;
        inner.access_token = resp.access_token;
        inner.expires_at = Utc::now() + chrono::Duration::seconds(resp.expires_in as i64 - 60);
        Ok(inner.access_token.clone())
    }

    pub async fn refresh_token(&self) -> String {
        self.inner.read().await.refresh_token.clone()
    }
}
```

```rust
// egudoc-sync/src/auth/mod.rs
pub mod device_flow;
pub mod token;
pub use token::TokenCache;
```

Declare in main.rs:
```rust
mod auth;
```

- [ ] **Step 2: Build**

```bash
cargo build 2>&1 | grep "^error"
```

- [ ] **Step 3: Commit**

```bash
git add egudoc-sync/src/auth/
git commit -m "feat(sync-client): add JWT token cache with transparent refresh"
```

---

### Task 5: auth/device_flow.rs — OAuth2 device flow + Windows Credential Manager

**Files:**
- Create: `egudoc-sync/src/auth/device_flow.rs`

- [ ] **Step 1: Create device_flow.rs**

```rust
// egudoc-sync/src/auth/device_flow.rs
use anyhow::{bail, Result};
use std::sync::Arc;
use tokio::time::{sleep, Duration};
use tracing::info;
use windows::core::PWSTR;
use windows::Win32::Security::Credentials::{
    CredReadW, CredWriteW, CREDENTIALW, CRED_PERSIST_LOCAL_MACHINE, CRED_TYPE_GENERIC,
};

use crate::api::ApiClient;
use super::token::TokenCache;

const CRED_TARGET_PREFIX: &str = "EguDoc:RefreshToken:";

/// Run the full device authorization flow. Blocks until the user approves or the code expires.
/// Returns a TokenCache loaded with the issued tokens.
pub async fn run_device_flow(api: Arc<ApiClient>, device_id: &str) -> Result<TokenCache> {
    let code_resp = api.device_code().await?;

    // Show instructions to the user via a Windows dialog or stdout.
    // For the initial implementation, write to stdout; tray notification comes later.
    println!(
        "\n🔐 Open {} and enter: {}\n",
        code_resp.verification_uri, code_resp.user_code
    );
    info!(user_code = %code_resp.user_code, uri = %code_resp.verification_uri, "Waiting for device approval");

    let interval = Duration::from_secs(code_resp.interval.max(5));
    let device_code = code_resp.device_code.clone();

    loop {
        sleep(interval).await;
        match api.device_token(&device_code, device_id).await? {
            None => {
                info!("Device flow: still pending");
            }
            Some(token_resp) => {
                let refresh = token_resp.refresh_token.clone().unwrap_or_default();
                if !refresh.is_empty() {
                    save_refresh_token(device_id, &refresh)?;
                }
                info!("Device flow: approved, tokens issued");
                return Ok(TokenCache::new(
                    api,
                    token_resp.access_token,
                    token_resp.expires_in,
                    refresh,
                ));
            }
        }
    }
}

/// Try to load a stored refresh token and exchange it for a new access token.
/// Returns None if no stored token exists.
pub async fn try_load_stored_token(api: Arc<ApiClient>, device_id: &str) -> Option<TokenCache> {
    let refresh = load_refresh_token(device_id).ok()??;
    match api.refresh_token(&refresh).await {
        Ok(resp) => Some(TokenCache::new(api, resp.access_token, resp.expires_in, refresh)),
        Err(_) => None,
    }
}

fn cred_target(device_id: &str) -> String {
    format!("{}{}", CRED_TARGET_PREFIX, device_id)
}

fn save_refresh_token(device_id: &str, token: &str) -> Result<()> {
    let target = cred_target(device_id);
    let target_wide: Vec<u16> = target.encode_utf16().chain(Some(0)).collect();
    let username_wide: Vec<u16> = "egudoc-sync\0".encode_utf16().collect();
    let token_bytes = token.as_bytes();

    let mut cred = CREDENTIALW {
        Type: CRED_TYPE_GENERIC,
        TargetName: PWSTR(target_wide.as_ptr() as _),
        UserName: PWSTR(username_wide.as_ptr() as _),
        CredentialBlobSize: token_bytes.len() as u32,
        CredentialBlob: token_bytes.as_ptr() as _,
        Persist: CRED_PERSIST_LOCAL_MACHINE,
        ..Default::default()
    };
    unsafe {
        CredWriteW(&mut cred, 0).ok()?;
    }
    Ok(())
}

fn load_refresh_token(device_id: &str) -> Result<Option<String>> {
    let target = cred_target(device_id);
    let target_wide: Vec<u16> = target.encode_utf16().chain(Some(0)).collect();
    let mut pcred = std::ptr::null_mut();
    unsafe {
        if CredReadW(
            PWSTR(target_wide.as_ptr() as _),
            CRED_TYPE_GENERIC,
            0,
            &mut pcred,
        )
        .is_err()
        {
            return Ok(None);
        }
        let blob = std::slice::from_raw_parts((*pcred).CredentialBlob, (*pcred).CredentialBlobSize as usize);
        let token = String::from_utf8_lossy(blob).into_owned();
        Ok(Some(token))
    }
}
```

- [ ] **Step 2: Export from auth/mod.rs**

```rust
// egudoc-sync/src/auth/mod.rs
pub mod device_flow;
pub mod token;
pub use device_flow::{run_device_flow, try_load_stored_token};
pub use token::TokenCache;
```

- [ ] **Step 3: Build**

```bash
cargo build 2>&1 | grep "^error"
```

- [ ] **Step 4: Commit**

```bash
git add egudoc-sync/src/auth/
git commit -m "feat(sync-client): add device flow auth + Windows Credential Manager storage"
```

---

### Task 6: sync/delta.rs + sync/download.rs + sync/upload.rs

**Files:**
- Create: `egudoc-sync/src/sync/mod.rs`
- Create: `egudoc-sync/src/sync/delta.rs`
- Create: `egudoc-sync/src/sync/download.rs`
- Create: `egudoc-sync/src/sync/upload.rs`

- [ ] **Step 1: Create sync/mod.rs**

```rust
pub mod delta;
pub mod download;
pub mod engine;
pub mod upload;
```

- [ ] **Step 2: Create sync/delta.rs**

```rust
// egudoc-sync/src/sync/delta.rs
use anyhow::Result;
use chrono::{DateTime, Utc};
use std::sync::Arc;
use crate::{api::ApiClient, auth::TokenCache};

pub use crate::api::DeltaDocument;

pub async fn fetch_delta(
    api: &Arc<ApiClient>,
    tokens: &TokenCache,
    since: Option<DateTime<Utc>>,
) -> Result<Vec<DeltaDocument>> {
    let token = tokens.token().await?;
    api.delta(&token, since).await
}
```

- [ ] **Step 3: Create sync/download.rs**

```rust
// egudoc-sync/src/sync/download.rs
use anyhow::{Context, Result};
use std::{path::Path, sync::Arc};
use uuid::Uuid;
use crate::{api::ApiClient, auth::TokenCache};

/// Download the bytes for an attachment into a local file path.
/// Called by fetch_data callback to hydrate a placeholder.
pub async fn download_to_file(
    api: &Arc<ApiClient>,
    tokens: &TokenCache,
    attachment_id: Uuid,
    dest: &Path,
) -> Result<()> {
    let token = tokens.token().await?;
    let url = api.download_url(&token, attachment_id).await?;

    let http = reqwest::Client::new();
    let resp = http.get(&url).send().await.context("download GET")?;
    resp.error_for_status_ref().context("download status")?;

    let bytes = resp.bytes().await.context("download body")?;
    if let Some(parent) = dest.parent() {
        std::fs::create_dir_all(parent).context("create dir")?;
    }
    std::fs::write(dest, &bytes).context("write download")?;
    Ok(())
}
```

- [ ] **Step 4: Create sync/upload.rs**

```rust
// egudoc-sync/src/sync/upload.rs
use anyhow::{Context, Result};
use sha2::{Digest, Sha256};
use std::{path::Path, sync::Arc};
use uuid::Uuid;
use crate::{
    api::{ApiClient, UploadConfirmRequest, UploadIntentRequest},
    auth::TokenCache,
};

/// Upload a local file as a new attachment or new version of an existing attachment.
/// - document_id: document this attachment belongs to
/// - attachment_id: None for new attachment, Some for new version
pub async fn upload_file(
    api: &Arc<ApiClient>,
    tokens: &TokenCache,
    document_id: Uuid,
    attachment_id: Option<Uuid>,
    local_path: &Path,
    filename: &str,
    content_type: &str,
) -> Result<Uuid> {
    let bytes = std::fs::read(local_path).context("read local file")?;
    let size_bytes = bytes.len() as i64;
    let sha256 = hex::encode(Sha256::digest(&bytes));

    let token = tokens.token().await?;

    // Step 1: get presigned PUT URL
    let intent = api.upload_intent(&token, &UploadIntentRequest {
        document_id,
        atasament_id: attachment_id,
        filename: filename.to_string(),
        content_type: content_type.to_string(),
        size_bytes,
    }).await?;

    // Step 2: PUT bytes directly to RustFS via presigned URL (no auth header needed)
    let http = reqwest::Client::new();
    let put_resp = http.put(&intent.upload_url)
        .header("Content-Type", content_type)
        .body(bytes)
        .send().await.context("presigned PUT")?;
    put_resp.error_for_status().context("presigned PUT status")?;

    // Step 3: confirm with Go backend
    let token = tokens.token().await?;
    let confirm = api.upload_confirm(&token, &UploadConfirmRequest {
        document_id,
        atasament_id: attachment_id,
        storage_key: intent.storage_key,
        filename: filename.to_string(),
        content_type: content_type.to_string(),
        size_bytes,
        sha256,
        source: "windows_sync".to_string(),
    }).await?;

    Ok(confirm.atasament_id)
}
```

- [ ] **Step 5: Build**

```bash
cargo build 2>&1 | grep "^error"
```

- [ ] **Step 6: Commit**

```bash
git add egudoc-sync/src/sync/
git commit -m "feat(sync-client): add delta, download, and upload sync operations"
```

---

### Task 7: rbac.rs — validate-move with 5-min cache

**Files:**
- Create: `egudoc-sync/src/rbac.rs`

- [ ] **Step 1: Create rbac.rs**

```rust
// egudoc-sync/src/rbac.rs
use anyhow::Result;
use chrono::{DateTime, Utc};
use std::{collections::HashMap, sync::Arc};
use tokio::sync::Mutex;
use uuid::Uuid;
use crate::{api::{ApiClient, ValidateMoveRequest}, auth::TokenCache};

struct CacheEntry {
    allowed: bool,
    expires_at: DateTime<Utc>,
}

pub struct RbacCache {
    api: Arc<ApiClient>,
    cache: Mutex<HashMap<(Uuid, Uuid), CacheEntry>>,
}

impl RbacCache {
    pub fn new(api: Arc<ApiClient>) -> Self {
        Self { api, cache: Mutex::new(HashMap::new()) }
    }

    /// Returns true if the move is allowed (same department), false if blocked.
    pub async fn validate_move(
        &self,
        tokens: &TokenCache,
        attachment_id: Uuid,
        dest_document_id: Uuid,
    ) -> Result<bool> {
        let key = (attachment_id, dest_document_id);
        {
            let cache = self.cache.lock().await;
            if let Some(entry) = cache.get(&key) {
                if Utc::now() < entry.expires_at {
                    return Ok(entry.allowed);
                }
            }
        }
        let token = tokens.token().await?;
        let allowed = self.api.validate_move(&token, &ValidateMoveRequest {
            attachment_id,
            dest_document_id,
        }).await?;

        let mut cache = self.cache.lock().await;
        cache.insert(key, CacheEntry {
            allowed,
            expires_at: Utc::now() + chrono::Duration::minutes(5),
        });
        Ok(allowed)
    }
}
```

Declare in main.rs:
```rust
mod rbac;
```

- [ ] **Step 2: Build**

```bash
cargo build 2>&1 | grep "^error"
```

- [ ] **Step 3: Commit**

```bash
git add egudoc-sync/src/rbac.rs egudoc-sync/src/main.rs
git commit -m "feat(sync-client): add RBAC validate-move with 5-min cache"
```

---

### Task 8: sync/engine.rs — delta polling loop

**Files:**
- Create: `egudoc-sync/src/sync/engine.rs`

- [ ] **Step 1: Create engine.rs**

```rust
// egudoc-sync/src/sync/engine.rs
use anyhow::Result;
use chrono::{DateTime, Utc};
use std::{path::PathBuf, sync::Arc, time::Duration};
use tokio::time::sleep;
use tracing::{error, info};

use crate::{
    api::ApiClient,
    auth::TokenCache,
    config::Config,
    sync::delta::fetch_delta,
};

pub struct Engine {
    api: Arc<ApiClient>,
    tokens: TokenCache,
    sync_root: PathBuf,
    config_path: PathBuf,
    last_sync_ts: Option<DateTime<Utc>>,
}

impl Engine {
    pub fn new(
        api: Arc<ApiClient>,
        tokens: TokenCache,
        config: &Config,
        config_path: PathBuf,
    ) -> Self {
        Self {
            api,
            tokens,
            sync_root: PathBuf::from(&config.sync_root),
            config_path,
            last_sync_ts: config.last_sync_ts,
        }
    }

    /// Run the polling loop forever. Call this in a background task.
    pub async fn run(&mut self, placeholder_tx: tokio::sync::mpsc::Sender<PlaceholderUpdate>) {
        loop {
            if let Err(e) = self.poll(&placeholder_tx).await {
                error!("sync poll error: {:#}", e);
            }
            sleep(Duration::from_secs(30)).await;
        }
    }

    async fn poll(&mut self, tx: &tokio::sync::mpsc::Sender<PlaceholderUpdate>) -> Result<()> {
        let docs = fetch_delta(&self.api, &self.tokens, self.last_sync_ts).await?;
        let now = Utc::now();

        for doc in &docs {
            for ata in &doc.atasamente {
                let virtual_path = self.virtual_path(doc, &ata.filename);
                tx.send(PlaceholderUpdate {
                    attachment_id: ata.id,
                    document_id: doc.id,
                    virtual_path,
                    filename: ata.filename.clone(),
                    size_bytes: ata.size_bytes,
                    current_version: ata.current_version,
                    updated_at: ata.updated_at,
                }).await?;
            }
        }

        self.last_sync_ts = Some(now);
        self.save_ts();
        Ok(())
    }

    fn virtual_path(&self, doc: &crate::api::DeltaDocument, filename: &str) -> std::path::PathBuf {
        let year = doc.updated_at.format("%Y").to_string();
        self.sync_root
            .join(&doc.registru_nume)
            .join(year)
            .join(format!("{} - {}", doc.nr_inregistrare, filename))
    }

    fn save_ts(&self) {
        // Best-effort — don't fail the polling loop on config save error.
        let path = Config::default_path();
        if let Ok(mut cfg) = Config::load(&self.config_path) {
            cfg.last_sync_ts = self.last_sync_ts;
            let _ = cfg.save(&path);
        }
    }
}

/// Message sent from engine to the CfApi provider to create/update a placeholder.
#[derive(Debug)]
pub struct PlaceholderUpdate {
    pub attachment_id: uuid::Uuid,
    pub document_id: uuid::Uuid,
    pub virtual_path: PathBuf,
    pub filename: String,
    pub size_bytes: i64,
    pub current_version: i32,
    pub updated_at: DateTime<Utc>,
}
```

- [ ] **Step 2: Build**

```bash
cargo build 2>&1 | grep "^error"
```

- [ ] **Step 3: Commit**

```bash
git add egudoc-sync/src/sync/engine.rs
git commit -m "feat(sync-client): add 30s delta polling engine with placeholder update channel"
```

---

### Task 9: callbacks — all five CfApi callbacks

**Files:**
- Create: `egudoc-sync/src/callbacks/mod.rs`
- Create: `egudoc-sync/src/callbacks/fetch_data.rs`
- Create: `egudoc-sync/src/callbacks/fetch_placeholders.rs`
- Create: `egudoc-sync/src/callbacks/notify_rename.rs`
- Create: `egudoc-sync/src/callbacks/notify_close.rs`
- Create: `egudoc-sync/src/callbacks/notify_delete.rs`

Note: CfApi requires callbacks to complete synchronously (no async). Use `tokio::runtime::Handle::current().block_on(...)` inside callbacks that need to call async code.

- [ ] **Step 1: Create callbacks/mod.rs**

```rust
pub mod fetch_data;
pub mod fetch_placeholders;
pub mod notify_close;
pub mod notify_delete;
pub mod notify_rename;
```

- [ ] **Step 2: Create callbacks/fetch_data.rs**

```rust
// egudoc-sync/src/callbacks/fetch_data.rs
// Called when a placeholder file is opened for read. We download the bytes from RustFS
// and write them into the placeholder via CfExecute.

use std::{path::PathBuf, sync::Arc};
use uuid::Uuid;
use windows::Win32::Storage::CloudFilters::*;
use crate::{api::ApiClient, auth::TokenCache, sync::download::download_to_file};

pub fn handle(
    attachment_id: Uuid,
    local_path: PathBuf,
    api: Arc<ApiClient>,
    tokens: TokenCache,
    callback_info: &CF_CALLBACK_INFO,
    callback_parameters: &CF_CALLBACK_PARAMETERS,
) {
    let rt = tokio::runtime::Handle::current();
    let result = rt.block_on(async {
        let tmp = local_path.with_extension("tmp_download");
        download_to_file(&api, &tokens, attachment_id, &tmp).await?;
        anyhow::Ok(tmp)
    });

    match result {
        Ok(tmp_path) => {
            // Transfer the downloaded bytes into the placeholder via CfExecute.
            // This is the standard CloudFilter pattern from the CloudMirror sample.
            unsafe {
                let data = std::fs::read(&tmp_path).unwrap_or_default();
                let op = CF_OPERATION_INFO {
                    StructSize: std::mem::size_of::<CF_OPERATION_INFO>() as u32,
                    Type: CF_OPERATION_TYPE_TRANSFER_DATA,
                    ConnectionKey: callback_info.ConnectionKey,
                    TransferKey: callback_info.TransferKey,
                    ..Default::default()
                };
                let params = CF_OPERATION_PARAMETERS {
                    ParamSize: std::mem::size_of::<CF_OPERATION_PARAMETERS>() as u32,
                    Anonymous: CF_OPERATION_PARAMETERS_0 {
                        TransferData: CF_OPERATION_PARAMETERS_0_10 {
                            Offset: callback_parameters.Anonymous.FetchData.RequiredFileOffset,
                            Buffer: data.as_ptr() as _,
                            Length: data.len() as i64,
                            Flags: CF_OPERATION_TRANSFER_DATA_FLAG_NONE,
                            CompletionStatus: windows::core::NTSTATUS(0),
                        },
                    },
                };
                let _ = CfExecute(&op, &params as *const _ as _);
            }
            let _ = std::fs::remove_file(tmp_path);
        }
        Err(e) => {
            tracing::error!("fetch_data download failed: {:#}", e);
            // Signal failure to the OS so it shows an error to the user.
            unsafe {
                let op = CF_OPERATION_INFO {
                    StructSize: std::mem::size_of::<CF_OPERATION_INFO>() as u32,
                    Type: CF_OPERATION_TYPE_TRANSFER_DATA,
                    ConnectionKey: callback_info.ConnectionKey,
                    TransferKey: callback_info.TransferKey,
                    ..Default::default()
                };
                let params = CF_OPERATION_PARAMETERS {
                    ParamSize: std::mem::size_of::<CF_OPERATION_PARAMETERS>() as u32,
                    Anonymous: CF_OPERATION_PARAMETERS_0 {
                        TransferData: CF_OPERATION_PARAMETERS_0_10 {
                            Offset: 0,
                            Buffer: std::ptr::null(),
                            Length: 0,
                            Flags: CF_OPERATION_TRANSFER_DATA_FLAG_NONE,
                            CompletionStatus: windows::core::NTSTATUS(0xC000_0001u32 as i32), // STATUS_UNSUCCESSFUL
                        },
                    },
                };
                let _ = CfExecute(&op, &params as *const _ as _);
            }
        }
    }
}
```

- [ ] **Step 3: Create callbacks/notify_rename.rs**

This is the RBAC enforcement point. The callback fires BEFORE the move completes. Returning an error blocks it.

```rust
// egudoc-sync/src/callbacks/notify_rename.rs
use std::sync::Arc;
use uuid::Uuid;
use windows::Win32::Foundation::E_ACCESSDENIED;
use windows::Win32::Storage::CloudFilters::CF_CALLBACK_INFO;
use crate::{api::ApiClient, auth::TokenCache, rbac::RbacCache};

/// Returns Ok(()) if the move is allowed, Err(E_ACCESSDENIED) if blocked.
pub fn handle(
    attachment_id: Uuid,
    dest_document_id: Uuid,
    api: Arc<ApiClient>,
    tokens: TokenCache,
    rbac: Arc<RbacCache>,
) -> windows::core::Result<()> {
    let rt = tokio::runtime::Handle::current();
    let allowed = rt.block_on(async {
        rbac.validate_move(&tokens, attachment_id, dest_document_id).await
    });

    match allowed {
        Ok(true) => Ok(()),
        Ok(false) => {
            tracing::warn!(%attachment_id, %dest_document_id, "RBAC blocked cross-department move");
            Err(windows::core::Error::new(E_ACCESSDENIED, "Nu aveți permisiunea să mutați acest fișier în altă secțiune.".into()))
        }
        Err(e) => {
            tracing::error!("validate_move error: {:#}", e);
            Err(windows::core::Error::new(E_ACCESSDENIED, "Eroare la validarea permisiunilor.".into()))
        }
    }
}
```

- [ ] **Step 4: Create callbacks/notify_close.rs**

```rust
// egudoc-sync/src/callbacks/notify_close.rs
// Fires when a file is closed after a write (e.g. Word saves a .docx).
// We upload the new version to Go backend → RustFS.

use std::{path::PathBuf, sync::Arc};
use uuid::Uuid;
use crate::{api::ApiClient, auth::TokenCache, sync::upload::upload_file};

pub fn handle(
    attachment_id: Option<Uuid>,  // Some = existing attachment, None = new
    document_id: Uuid,
    filename: String,
    local_path: PathBuf,
    api: Arc<ApiClient>,
    tokens: TokenCache,
) {
    // Spawn a detached task — the CfApi callback must return quickly.
    tokio::spawn(async move {
        let content_type = mime_guess::from_path(&filename)
            .first_or_octet_stream()
            .to_string();
        match upload_file(&api, &tokens, document_id, attachment_id, &local_path, &filename, &content_type).await {
            Ok(aid) => tracing::info!(%aid, %filename, "upload on close: success"),
            Err(e) => tracing::error!(%filename, "upload on close: {:#}", e),
        }
    });
}
```

Add `mime_guess = "2"` to Cargo.toml dependencies.

- [ ] **Step 5: Create callbacks/notify_delete.rs (stub)**

```rust
// egudoc-sync/src/callbacks/notify_delete.rs
// Currently a no-op. Deletion from Explorer is allowed but does not propagate to the server.
// A future iteration can add a DELETE API call here.

use windows::Win32::Storage::CloudFilters::CF_CALLBACK_INFO;

pub fn handle(_callback_info: &CF_CALLBACK_INFO) -> windows::core::Result<()> {
    Ok(())
}
```

- [ ] **Step 6: Create callbacks/fetch_placeholders.rs (stub)**

```rust
// egudoc-sync/src/callbacks/fetch_placeholders.rs
// Called when a folder is expanded. The engine polling loop handles placeholder creation,
// so this callback is a no-op for folders already populated by the engine.
// For on-demand folder expansion, signal the engine to run an immediate delta.

use windows::Win32::Storage::CloudFilters::CF_CALLBACK_INFO;

pub fn handle(_callback_info: &CF_CALLBACK_INFO) {
    // Engine's next 30s poll will populate any missing placeholders.
}
```

- [ ] **Step 7: Build**

```bash
cargo build 2>&1 | grep "^error"
```

- [ ] **Step 8: Commit**

```bash
git add egudoc-sync/src/callbacks/
git commit -m "feat(sync-client): add all CfApi callbacks (fetch_data, notify_rename/close/delete)"
```

---

### Task 10: provider.rs — CfApi sync root registration

**Files:**
- Create: `egudoc-sync/src/provider.rs`

- [ ] **Step 1: Create provider.rs**

```rust
// egudoc-sync/src/provider.rs
// Registers the EguDoc sync root with the Windows Cloud Filter API and
// connects the callback dispatch table.

use anyhow::{Context, Result};
use std::{path::Path, sync::Arc};
use windows::{
    core::HSTRING,
    Win32::Storage::CloudFilters::*,
};
use crate::{api::ApiClient, auth::TokenCache, rbac::RbacCache};

/// Register the sync root and start serving CfApi callbacks.
/// This function blocks until the provider is stopped.
pub fn register_and_run(
    sync_root_path: &Path,
    provider_name: &str,
    api: Arc<ApiClient>,
    tokens: TokenCache,
    rbac: Arc<RbacCache>,
) -> Result<()> {
    let sync_root_id = HSTRING::from(format!("EguDoc!{}", provider_name));
    let sync_root_path_w = HSTRING::from(sync_root_path.to_string_lossy().as_ref());

    // Register sync root (idempotent — safe to call on every startup).
    unsafe {
        let info = CF_SYNC_ROOT_PROVIDER_INFO {
            ProviderVersion: 1,
            SyncRootIdentity: sync_root_id.as_ptr() as _,
            SyncRootIdentityLength: (sync_root_id.len() * 2) as u32,
            ..Default::default()
        };
        // CfRegisterSyncRoot registers the virtual drive root in the registry.
        // Explorer will show the drive with cloud overlay icons after this call.
        CfRegisterSyncRoot(
            &sync_root_path_w,
            &info,
            std::ptr::null(),
            CF_REGISTER_FLAG_NONE,
        ).context("CfRegisterSyncRoot")?;
    }

    // Connect callback table.
    // Each entry maps a callback type to a function pointer.
    // The actual dispatch (looking up attachment_id from identity blob, calling
    // the typed callback handlers) happens in the dispatch function below.
    let callbacks = [
        CF_CALLBACK_REGISTRATION {
            Type: CF_CALLBACK_TYPE_FETCH_DATA,
            Callback: Some(dispatch_fetch_data),
        },
        CF_CALLBACK_REGISTRATION {
            Type: CF_CALLBACK_TYPE_NOTIFY_FILE_CLOSE_COMPLETION,
            Callback: Some(dispatch_notify_close),
        },
        CF_CALLBACK_REGISTRATION {
            Type: CF_CALLBACK_TYPE_NOTIFY_RENAME,
            Callback: Some(dispatch_notify_rename),
        },
        CF_CALLBACK_REGISTRATION {
            Type: CF_CALLBACK_TYPE_NOTIFY_DELETE,
            Callback: Some(dispatch_notify_delete),
        },
        CF_CALLBACK_REGISTRATION { Type: CF_CALLBACK_TYPE_NONE, Callback: None },
    ];

    let mut connection_key = CF_CONNECTION_KEY::default();
    unsafe {
        CfConnectSyncRoot(
            &sync_root_path_w,
            callbacks.as_ptr(),
            std::ptr::null(),
            CF_CONNECT_FLAG_NONE,
            &mut connection_key,
        ).context("CfConnectSyncRoot")?;
    }

    tracing::info!("CfApi sync root connected — drive is live");

    // Block until the process exits. Windows will call our callbacks.
    // A production implementation would use a message loop or wait handle.
    loop {
        std::thread::sleep(std::time::Duration::from_secs(60));
    }
}

// ── Dispatch trampolines ──────────────────────────────────────────────────────
// These are unsafe extern "system" fns required by the CfApi ABI.
// They decode the identity blob (which stores atasament_id + document_id as UUIDs)
// and call the typed callback handlers.

unsafe extern "system" fn dispatch_fetch_data(
    _info: *const CF_CALLBACK_INFO,
    _params: *const CF_CALLBACK_PARAMETERS,
) {
    // Identity blob layout: [16 bytes atasament_id][16 bytes document_id]
    // Populated by the engine when creating placeholders via CfCreatePlaceholders.
    let info = &*_info;
    let params = &*_params;
    if let Some((aid, _doc_id)) = parse_identity(info) {
        // In production, retrieve Arc<ApiClient> and TokenCache from a global/thread-local.
        // For the implementation plan, the provider stores these in a thread-local set
        // during CfConnectSyncRoot setup. Exact wiring is left to the implementor
        // based on their chosen global state pattern (lazy_static, once_cell, etc.).
        tracing::debug!(%aid, "FETCH_DATA callback");
        // crate::callbacks::fetch_data::handle(aid, local_path, api, tokens, info, params);
    }
}

unsafe extern "system" fn dispatch_notify_rename(
    _info: *const CF_CALLBACK_INFO,
    _params: *const CF_CALLBACK_PARAMETERS,
) {
    let info = &*_info;
    if let Some((aid, _doc_id)) = parse_identity(info) {
        tracing::debug!(%aid, "NOTIFY_RENAME callback");
        // Decode dest path from params, determine dest_document_id, call notify_rename::handle.
    }
}

unsafe extern "system" fn dispatch_notify_close(
    _info: *const CF_CALLBACK_INFO,
    _params: *const CF_CALLBACK_PARAMETERS,
) {
    let info = &*_info;
    if let Some((aid, doc_id)) = parse_identity(info) {
        tracing::debug!(%aid, %doc_id, "NOTIFY_FILE_CLOSE_COMPLETION callback");
        // Check if file was modified (via flag in params), then call notify_close::handle.
    }
}

unsafe extern "system" fn dispatch_notify_delete(
    _info: *const CF_CALLBACK_INFO,
    _params: *const CF_CALLBACK_PARAMETERS,
) {
    let info = &*_info;
    if let Some((aid, _)) = parse_identity(info) {
        tracing::debug!(%aid, "NOTIFY_DELETE callback");
    }
}

/// Parse the identity blob stored in a placeholder: [atasament_id 16B][document_id 16B]
fn parse_identity(info: &CF_CALLBACK_INFO) -> Option<(uuid::Uuid, uuid::Uuid)> {
    let blob = unsafe {
        std::slice::from_raw_parts(
            info.FileIdentity as *const u8,
            info.FileIdentityLength as usize,
        )
    };
    if blob.len() < 32 {
        return None;
    }
    let aid = uuid::Uuid::from_slice(&blob[0..16]).ok()?;
    let did = uuid::Uuid::from_slice(&blob[16..32]).ok()?;
    Some((aid, did))
}
```

Declare in main.rs:
```rust
mod provider;
```

- [ ] **Step 2: Build**

```bash
cargo build 2>&1 | grep "^error"
```

- [ ] **Step 3: Commit**

```bash
git add egudoc-sync/src/provider.rs egudoc-sync/src/main.rs
git commit -m "feat(sync-client): add CfApi sync root provider with callback dispatch"
```

---

### Task 11: main.rs — wire everything together

**Files:**
- Modify: `egudoc-sync/src/main.rs`

- [ ] **Step 1: Write main.rs**

```rust
mod api;
mod auth;
mod callbacks;
mod config;
mod provider;
mod rbac;
mod sync;

use std::sync::Arc;
use anyhow::Result;
use config::Config;
use api::ApiClient;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "egudoc_sync=info".into()),
        )
        .init();

    let config_path = Config::default_path();
    let config = Config::load(&config_path)?;

    tracing::info!(api = %config.api_base_url, sync_root = %config.sync_root, "Starting egudoc-sync");

    // TODO: read institution_id from config (add to Config struct)
    let institution_id = std::env::var("EGUDOC_INSTITUTION_ID")
        .unwrap_or_else(|_| "00000000-0000-0000-0000-000000000000".to_string());

    let api = Arc::new(ApiClient::new(config.api_base_url.clone(), institution_id));

    // Authenticate: try stored refresh token first, fall back to device flow.
    let tokens = match auth::try_load_stored_token(api.clone(), &config.device_id).await {
        Some(t) => {
            tracing::info!("Authenticated via stored refresh token");
            t
        }
        None => {
            tracing::info!("No stored token — starting device flow");
            auth::run_device_flow(api.clone(), &config.device_id).await?
        }
    };

    let rbac = Arc::new(rbac::RbacCache::new(api.clone()));

    // Create the sync root directory if it doesn't exist.
    let sync_root = std::path::PathBuf::from(&config.sync_root);
    std::fs::create_dir_all(&sync_root)?;

    // Start the delta polling engine in a background task.
    let (placeholder_tx, mut placeholder_rx) =
        tokio::sync::mpsc::channel::<sync::engine::PlaceholderUpdate>(256);

    let mut engine = sync::engine::Engine::new(api.clone(), tokens.clone(), &config, config_path.clone());
    tokio::spawn(async move {
        engine.run(placeholder_tx).await;
    });

    // Handle placeholder updates from the engine (create/update CfApi placeholders).
    tokio::spawn(async move {
        while let Some(update) = placeholder_rx.recv().await {
            tracing::debug!(path = ?update.virtual_path, version = update.current_version, "placeholder update");
            // CfCreatePlaceholders is called here in a blocking thread.
            // For the full implementation, use tokio::task::spawn_blocking.
        }
    });

    // Register sync root and run CfApi provider (blocks until exit).
    // This must run on a dedicated thread — it contains an infinite loop.
    let tokens_for_provider = tokens.clone();
    let rbac_for_provider = rbac.clone();
    let api_for_provider = api.clone();
    let sync_root_for_provider = sync_root.clone();

    tokio::task::spawn_blocking(move || {
        provider::register_and_run(
            &sync_root_for_provider,
            "EguDoc",
            api_for_provider,
            tokens_for_provider,
            rbac_for_provider,
        )
    }).await??;

    Ok(())
}
```

- [ ] **Step 2: Build release binary**

```bash
cargo build --release 2>&1 | grep "^error"
```
Expected: no errors. Binary at `target/release/egudoc-sync.exe`.

- [ ] **Step 3: Smoke test on Windows**

```bash
EGUDOC_INSTITUTION_ID="<your-institution-uuid>" \
RUST_LOG=egudoc_sync=debug \
./target/release/egudoc-sync.exe
```
Expected: "Starting egudoc-sync" + device flow prompt or "Authenticated via stored refresh token".

- [ ] **Step 4: Commit**

```bash
git add egudoc-sync/src/main.rs
git commit -m "feat(sync-client): wire all modules in main — egudoc-sync.exe is functional"
```

---

*End of Phase 2 plan. Phase 3: Angular web file manager.*
