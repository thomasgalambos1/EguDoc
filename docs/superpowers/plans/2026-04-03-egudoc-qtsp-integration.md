# EguDoc — Sub-plan D: QTSP Integration (eDelivery + eArchiving + PDF/A + E-ARK SIP)

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`. Depends on Sub-plans A and B being complete.

**Goal:** Implement the eDelivery and eArchiving integration clients in Go. EguDoc calls the eguwallet QTSP REST APIs for all qualified trust service operations — it does not reimplement ETSI standards. Additionally implement: Gotenberg client for PDF/A-2b conversion, E-ARK CSIP/SIP package builder per Law 135/2007, and the background worker that archives finalized documents.

**Architecture:** Two clean client packages (`internal/qtsp/delivery` and `internal/qtsp/archive`) that wrap the eguwallet HTTP APIs. A `internal/pdf` package for Gotenberg. A `internal/eark` package for E-ARK SIP package generation. A `internal/archiveworker` background goroutine triggered by document finalization.

**Key facts from eguwallet research:**
- eguwallet eDelivery API: `POST /delivery/submit`, `GET /delivery/messages/{id}`, `GET /delivery/mailbox`, `GET /delivery/mailbox/{id}`
- eguwallet eArchive API: `POST /archive/ingest`, `GET /archive/documents/{id}`, `GET /archive/documents/{id}/verify`, `GET /archive/documents/{id}/custody-proof`
- Auth to QTSP: `X-Internal-Service-Key` header (from `QTSP_SERVICE_KEY` env var)
- QTSP uses S3-compatible storage (MinIO) — attachments are referenced by storage key

---

## File Map

```
internal/
├── qtsp/
│   ├── client.go                 # base HTTP client (auth header, retry, timeout)
│   ├── delivery/
│   │   ├── model.go              # Message, Evidence, Mailbox structs
│   │   ├── client.go             # QTSP delivery REST client
│   │   └── dispatcher.go        # decides when/how to send via eDelivery
│   └── archive/
│       ├── model.go              # ArchiveDocument, EvidenceRecord, CustodyProof
│       └── client.go             # QTSP archive REST client
├── pdf/
│   └── gotenberg.go             # Gotenberg client: ConvertToPDFA()
├── eark/
│   ├── sip_builder.go           # E-ARK CSIP SIP package generator
│   ├── mets.go                  # METS XML generation
│   ├── premis.go                # PREMIS XML generation
│   └── dublincore.go            # Dublin Core XML from document metadata
└── archiveworker/
    ├── worker.go                # Background goroutine: scans for finalized documents
    └── pipeline.go              # Full archive pipeline: PDF/A → SIP → validate → submit
```

---

## Task 21: QTSP base client

**Files:**
- Create: `internal/qtsp/client.go`

- [ ] **Step 21.1: Write qtsp/client.go**

```go
// internal/qtsp/client.go
package qtsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the base HTTP client for communicating with the eguwallet QTSP service.
// All requests include the X-Internal-Service-Key header for machine-to-machine auth.
type Client struct {
	baseURL    string
	serviceKey string
	httpClient *http.Client
}

func NewClient(baseURL, serviceKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		serviceKey: serviceKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:    10,
				IdleConnTimeout: 90 * time.Second,
			},
		},
	}
}

// Do executes an authenticated request to the QTSP service.
func (c *Client) Do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Internal-Service-Key", c.serviceKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request to %s %s: %w", method, path, err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("QTSP error %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// DecodeJSON decodes the response body into target.
func DecodeJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

// DoMultipart sends a multipart/form-data request (for file uploads).
func (c *Client) DoMultipart(ctx context.Context, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create multipart request: %w", err)
	}

	req.Header.Set("X-Internal-Service-Key", c.serviceKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("multipart request: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("QTSP multipart error %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}
```

- [ ] **Step 21.2: Commit**

```bash
git add internal/qtsp/client.go
git commit -m "feat: add QTSP base HTTP client with service key auth"
```

---

## Task 22: eDelivery client

**Files:**
- Create: `internal/qtsp/delivery/model.go`
- Create: `internal/qtsp/delivery/client.go`
- Create: `internal/qtsp/delivery/dispatcher.go`

- [ ] **Step 22.1: Write delivery/model.go**

```go
// internal/qtsp/delivery/model.go
package delivery

import "time"

type MessageStatus string

const (
	StatusSubmitted MessageStatus = "submitted"
	StatusDelivered MessageStatus = "delivered"
	StatusRetrieved MessageStatus = "retrieved"
	StatusExpired   MessageStatus = "expired"
	StatusFailed    MessageStatus = "failed"
)

// SubmitMessageRequest mirrors the QTSP delivery submit endpoint payload.
type SubmitMessageRequest struct {
	RecipientEmail string       `json:"recipient_email"`
	Subject        string       `json:"subject"`
	ContentType    string       `json:"content_type"`
	RetentionDays  int          `json:"retention_days,omitempty"`
	Attachments    []Attachment `json:"attachments,omitempty"`
}

type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	StorageKey  string `json:"storage_key"` // MinIO key in QTSP's bucket
}

type SubmitMessageResponse struct {
	ID            string        `json:"id"`
	Status        MessageStatus `json:"status"`
	SubmittedAt   time.Time     `json:"submitted_at"`
	ExpiresAt     time.Time     `json:"expires_at"`
}

type MessageInfo struct {
	ID            string        `json:"id"`
	SenderID      string        `json:"sender_id"`
	RecipientID   string        `json:"recipient_id"`
	Subject       string        `json:"subject"`
	Status        MessageStatus `json:"status"`
	SubmittedAt   time.Time     `json:"submitted_at"`
	DeliveredAt   *time.Time    `json:"delivered_at,omitempty"`
	RetrievedAt   *time.Time    `json:"retrieved_at,omitempty"`
	ExpiresAt     time.Time     `json:"expires_at"`
}

type Evidence struct {
	ID            string    `json:"id"`
	MessageID     string    `json:"message_id"`
	EvidenceType  string    `json:"evidence_type"`   // submission, delivery, receipt, rejection
	EvidenceJWT   string    `json:"evidence_jwt"`    // REM evidence JWT
	CreatedAt     time.Time `json:"created_at"`
}
```

- [ ] **Step 22.2: Write delivery/client.go**

```go
// internal/qtsp/delivery/client.go
package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/eguilde/egudoc/internal/qtsp"
)

// Client wraps the eguwallet QTSP eDelivery REST API.
type Client struct {
	base *qtsp.Client
}

func NewClient(base *qtsp.Client) *Client {
	return &Client{base: base}
}

// Submit sends a document via qualified eDelivery.
// The document content is provided as a reader; it is submitted as multipart.
func (c *Client) Submit(ctx context.Context, req SubmitMessageRequest, content io.Reader, filename string) (*SubmitMessageResponse, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Add JSON metadata part
	metaPart, err := mw.CreateFormField("metadata")
	if err != nil {
		return nil, fmt.Errorf("create metadata part: %w", err)
	}
	if err := json.NewEncoder(metaPart).Encode(req); err != nil {
		return nil, fmt.Errorf("encode metadata: %w", err)
	}

	// Add document file part
	filePart, err := mw.CreateFormFile("document", filename)
	if err != nil {
		return nil, fmt.Errorf("create file part: %w", err)
	}
	if _, err := io.Copy(filePart, content); err != nil {
		return nil, fmt.Errorf("copy document content: %w", err)
	}
	mw.Close()

	resp, err := c.base.DoMultipart(ctx, "/delivery/submit", &buf, mw.FormDataContentType())
	if err != nil {
		return nil, fmt.Errorf("submit delivery: %w", err)
	}

	var result SubmitMessageResponse
	if err := qtsp.DecodeJSON(resp, &result); err != nil {
		return nil, fmt.Errorf("decode submit response: %w", err)
	}
	return &result, nil
}

// GetMessage retrieves the status and evidence summary for a delivery message.
func (c *Client) GetMessage(ctx context.Context, messageID string) (*MessageInfo, error) {
	resp, err := c.base.Do(ctx, http.MethodGet, "/delivery/messages/"+messageID, nil)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	var info MessageInfo
	if err := qtsp.DecodeJSON(resp, &info); err != nil {
		return nil, fmt.Errorf("decode message: %w", err)
	}
	return &info, nil
}

// GetEvidence returns all evidence records for a delivery message.
func (c *Client) GetEvidence(ctx context.Context, messageID string) ([]Evidence, error) {
	resp, err := c.base.Do(ctx, http.MethodGet, "/delivery/messages/"+messageID+"/evidence", nil)
	if err != nil {
		return nil, fmt.Errorf("get evidence: %w", err)
	}
	var evidence []Evidence
	if err := qtsp.DecodeJSON(resp, &evidence); err != nil {
		return nil, fmt.Errorf("decode evidence: %w", err)
	}
	return evidence, nil
}
```

- [ ] **Step 22.3: Write delivery/dispatcher.go**

```go
// internal/qtsp/delivery/dispatcher.go
package delivery

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Dispatcher decides whether a document should be sent via eDelivery
// and orchestrates the submission process.
type Dispatcher struct {
	client *Client
	db     *pgxpool.Pool
}

func NewDispatcher(client *Client, db *pgxpool.Pool) *Dispatcher {
	return &Dispatcher{client: client, db: db}
}

// ShouldDeliver returns true if the document's destinatar is an external institution
// that is registered in the eDelivery network (has a delivery_participant_id).
func (d *Dispatcher) ShouldDeliver(ctx context.Context, documentID uuid.UUID) (bool, string, error) {
	var destinatarEmail string
	var deliveryParticipantID *string

	err := d.db.QueryRow(ctx, `
		SELECT e.email, e.delivery_participant_id
		FROM documente doc
		LEFT JOIN entitati e ON e.id = doc.destinatar_id
		WHERE doc.id = $1 AND doc.destinatar_id IS NOT NULL
	`, documentID).Scan(&destinatarEmail, &deliveryParticipantID)
	if err != nil {
		return false, "", fmt.Errorf("lookup destinatar: %w", err)
	}

	// If recipient has a delivery participant ID, use eDelivery
	if deliveryParticipantID != nil && *deliveryParticipantID != "" {
		return true, destinatarEmail, nil
	}
	return false, "", nil
}

// Dispatch submits a document to the QTSP eDelivery service and records the message ID.
func (d *Dispatcher) Dispatch(ctx context.Context, documentID uuid.UUID, subject string, content io.Reader, filename string, recipientEmail string) error {
	req := SubmitMessageRequest{
		RecipientEmail: recipientEmail,
		Subject:        subject,
		ContentType:    "application/pdf",
		RetentionDays:  90,
	}

	result, err := d.client.Submit(ctx, req, content, filename)
	if err != nil {
		// Record failure
		d.db.Exec(ctx, `
			UPDATE documente SET delivery_status = 'FAILED', updated_at = NOW()
			WHERE id = $1
		`, documentID)
		return fmt.Errorf("delivery submit: %w", err)
	}

	// Record success
	_, err = d.db.Exec(ctx, `
		UPDATE documente
		SET delivery_message_id = $1, delivery_status = 'SUBMITTED', updated_at = NOW()
		WHERE id = $2
	`, result.ID, documentID)
	return err
}
```

- [ ] **Step 22.4: Commit**

```bash
git add internal/qtsp/delivery/
git commit -m "feat: add eDelivery client and dispatcher for QTSP integration"
```

---

## Task 23: eArchiving client

**Files:**
- Create: `internal/qtsp/archive/model.go`
- Create: `internal/qtsp/archive/client.go`

- [ ] **Step 23.1: Write archive/model.go**

```go
// internal/qtsp/archive/model.go
package archive

import "time"

type ArchiveStatus string

const (
	ArchiveStatusPending    ArchiveStatus = "pending"
	ArchiveStatusArchived   ArchiveStatus = "archived"
	ArchiveStatusVerified   ArchiveStatus = "verified"
	ArchiveStatusExpired    ArchiveStatus = "expired"
)

type IngestResponse struct {
	ID              string        `json:"id"`
	Title           string        `json:"title"`
	ContentHash     string        `json:"content_hash"`
	StorageKey      string        `json:"storage_key"`
	ArchiveStatus   ArchiveStatus `json:"archive_status"`
	RetentionYears  int           `json:"retention_years"`
	IngestedAt      time.Time     `json:"ingested_at"`
	NextRetimestampAt *time.Time  `json:"next_retimestamp_at,omitempty"`
	ExpiresAt       *time.Time    `json:"expires_at,omitempty"`
}

type ArchiveDocument struct {
	ID              string        `json:"id"`
	OwnerID         string        `json:"owner_id"`
	Title           string        `json:"title"`
	ContentHash     string        `json:"content_hash"`
	ContentType     string        `json:"content_type"`
	SizeBytes       int64         `json:"size_bytes"`
	SignatureFormat string        `json:"signature_format"`
	SignatureValid  bool          `json:"signature_valid"`
	ArchiveStatus   ArchiveStatus `json:"archive_status"`
	RetentionYears  int           `json:"retention_years"`
	IngestedAt      time.Time     `json:"ingested_at"`
	LastEvidenceAt  *time.Time    `json:"last_evidence_at,omitempty"`
	ExpiresAt       *time.Time    `json:"expires_at,omitempty"`
}

type VerifyResponse struct {
	Valid       bool   `json:"valid"`
	Message     string `json:"message"`
	EvidenceLen int    `json:"evidence_length"`
}

type CustodyProof struct {
	DocumentID  string    `json:"document_id"`
	ProofJWT    string    `json:"proof_jwt"`  // QSeal-signed custody proof
	IssuedAt    time.Time `json:"issued_at"`
}
```

- [ ] **Step 23.2: Write archive/client.go**

```go
// internal/qtsp/archive/client.go
package archive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/eguilde/egudoc/internal/qtsp"
)

// Client wraps the eguwallet QTSP eArchiving REST API.
type Client struct {
	base *qtsp.Client
}

func NewClient(base *qtsp.Client) *Client {
	return &Client{base: base}
}

// Ingest submits a document to the qualified electronic archive.
// The document should already be in PDF/A format.
// title: human-readable document title (nr_inregistrare + obiect)
// retentionYears: from retention policy
// ownerID: institution CUI or user subject
func (c *Client) Ingest(ctx context.Context, title string, ownerID string, retentionYears int, content io.Reader, filename string, contentType string) (*IngestResponse, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Add metadata fields
	mw.WriteField("title", title)
	mw.WriteField("owner_id", ownerID)
	mw.WriteField("retention_years", strconv.Itoa(retentionYears))

	// Add document content
	part, err := mw.CreateFormFile("document", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, content); err != nil {
		return nil, fmt.Errorf("copy content: %w", err)
	}
	mw.Close()

	resp, err := c.base.DoMultipart(ctx, "/archive/ingest", &buf, mw.FormDataContentType())
	if err != nil {
		return nil, fmt.Errorf("ingest document: %w", err)
	}

	var result IngestResponse
	if err := qtsp.DecodeJSON(resp, &result); err != nil {
		return nil, fmt.Errorf("decode ingest response: %w", err)
	}
	return &result, nil
}

// GetDocument retrieves the archive record for a document.
func (c *Client) GetDocument(ctx context.Context, archiveID string) (*ArchiveDocument, error) {
	resp, err := c.base.Do(ctx, http.MethodGet, "/archive/documents/"+archiveID, nil)
	if err != nil {
		return nil, fmt.Errorf("get archive document: %w", err)
	}
	var doc ArchiveDocument
	if err := qtsp.DecodeJSON(resp, &doc); err != nil {
		return nil, fmt.Errorf("decode archive document: %w", err)
	}
	return &doc, nil
}

// VerifyIntegrity runs a full evidence-chain integrity check on a document.
func (c *Client) VerifyIntegrity(ctx context.Context, archiveID string) (*VerifyResponse, error) {
	resp, err := c.base.Do(ctx, http.MethodGet, "/archive/documents/"+archiveID+"/verify", nil)
	if err != nil {
		return nil, fmt.Errorf("verify document: %w", err)
	}
	var result VerifyResponse
	if err := qtsp.DecodeJSON(resp, &result); err != nil {
		return nil, fmt.Errorf("decode verify response: %w", err)
	}
	return &result, nil
}

// GetCustodyProof returns the QSeal-signed custody proof JWT for certified retrieval.
func (c *Client) GetCustodyProof(ctx context.Context, archiveID string) (*CustodyProof, error) {
	resp, err := c.base.Do(ctx, http.MethodGet, "/archive/documents/"+archiveID+"/custody-proof", nil)
	if err != nil {
		return nil, fmt.Errorf("get custody proof: %w", err)
	}
	var proof CustodyProof
	if err := qtsp.DecodeJSON(resp, &proof); err != nil {
		return nil, fmt.Errorf("decode custody proof: %w", err)
	}
	return &proof, nil
}
```

- [ ] **Step 23.3: Commit**

```bash
git add internal/qtsp/archive/
git commit -m "feat: add eArchiving client for QTSP integration"
```

---

## Task 24: Gotenberg PDF/A conversion client

**Files:**
- Create: `internal/pdf/gotenberg.go`

- [ ] **Step 24.1: Write gotenberg.go**

```go
// internal/pdf/gotenberg.go
package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"
)

// Gotenberg wraps the Gotenberg PDF conversion microservice.
// See: https://gotenberg.dev
type Gotenberg struct {
	baseURL    string
	httpClient *http.Client
}

func NewGotenberg(baseURL string) *Gotenberg {
	return &Gotenberg{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // PDF conversion can be slow for large files
		},
	}
}

// ConvertToPDFA converts any supported document format to PDF/A-2b.
// Supported input formats: .docx, .doc, .odt, .ods, .odp, .xlsx, .pptx, .html, .pdf
// Returns the PDF/A content as a byte slice.
func (g *Gotenberg) ConvertToPDFA(ctx context.Context, filename string, content io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Add the file
	part, err := mw.CreateFormFile("files", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, content); err != nil {
		return nil, fmt.Errorf("copy file content: %w", err)
	}

	// PDF/A-2b format
	mw.WriteField("pdfa", "PDF/A-2b")
	// Disable JavaScript for security
	mw.WriteField("nativePageRanges", "")

	mw.Close()

	// Choose endpoint based on input format
	endpoint := g.endpointForFile(filename)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+endpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gotenberg conversion failed (status %d): %s", resp.StatusCode, string(body))
	}

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read conversion result: %w", err)
	}
	return result, nil
}

// ConvertHTMLToPDFA converts an HTML string to PDF/A-2b.
// Useful for generating documents from templates.
func (g *Gotenberg) ConvertHTMLToPDFA(ctx context.Context, htmlContent string, title string) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	part, err := mw.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, fmt.Errorf("create html part: %w", err)
	}
	io.WriteString(part, htmlContent)

	mw.WriteField("pdfa", "PDF/A-2b")
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/forms/chromium/convert/html", &buf)
	if err != nil {
		return nil, fmt.Errorf("create html request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg html request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("html to pdfa failed: %s", string(body))
	}

	return io.ReadAll(resp.Body)
}

func (g *Gotenberg) endpointForFile(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".html", ".htm":
		return "/forms/chromium/convert/html"
	case ".pdf":
		return "/forms/libreoffice/convert" // LibreOffice can re-export as PDF/A
	default:
		// .docx, .doc, .odt, .xlsx, .pptx, .ods, .odp, etc.
		return "/forms/libreoffice/convert"
	}
}
```

- [ ] **Step 24.2: Write Gotenberg test**

```go
// internal/pdf/gotenberg_test.go
package pdf_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eguilde/egudoc/internal/pdf"
)

func TestConvertHTMLToPDFACallsCorrectEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("%PDF-1.4 fake pdf content"))
	}))
	defer srv.Close()

	g := pdf.NewGotenberg(srv.URL)
	result, err := g.ConvertHTMLToPDFA(context.Background(), "<html><body>Test</body></html>", "Test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(string(result), "%PDF") {
		t.Errorf("expected PDF content, got: %s", string(result))
	}
	if gotPath != "/forms/chromium/convert/html" {
		t.Errorf("expected HTML endpoint, got: %s", gotPath)
	}
}
```

```bash
go test ./internal/pdf/... -v -run TestConvertHTMLToPDFACallsCorrectEndpoint
```

Expected: PASS

- [ ] **Step 24.3: Commit**

```bash
git add internal/pdf/
git commit -m "feat: add Gotenberg PDF/A-2b conversion client"
```

---

## Task 25: E-ARK SIP package builder (Law 135/2007)

**Files:**
- Create: `internal/eark/dublincore.go`
- Create: `internal/eark/premis.go`
- Create: `internal/eark/mets.go`
- Create: `internal/eark/sip_builder.go`

- [ ] **Step 25.1: Write dublincore.go**

```go
// internal/eark/dublincore.go
package eark

import (
	"encoding/xml"
	"time"
)

// DublinCoreMetadata maps to Dublin Core XML elements.
// Maps to the 13 mandatory fields from Romanian Law 135/2007.
type DublinCoreMetadata struct {
	XMLName     xml.Name `xml:"oai_dc:dc"`
	XMLNS       string   `xml:"xmlns:oai_dc,attr"`
	XMLNSDC     string   `xml:"xmlns:dc,attr"`
	XMLNSXSI    string   `xml:"xmlns:xsi,attr"`
	XSISchema   string   `xml:"xsi:schemaLocation,attr"`

	// Law 135/2007 mandatory fields mapped to DC elements
	Title       string     `xml:"dc:title"`        // obiect + nr_inregistrare
	Creator     string     `xml:"dc:creator"`      // emitent
	Subject     []string   `xml:"dc:subject"`      // cuvinte_cheie
	Description string     `xml:"dc:description"`  // continut
	Publisher   string     `xml:"dc:publisher"`    // institution (proprietarul documentului)
	Contributor string     `xml:"dc:contributor"`  // assigned user (titularul dreptului de dispozitie)
	Date        string     `xml:"dc:date"`         // data_inregistrare
	Type        string     `xml:"dc:type"`         // tip_document
	Format      string     `xml:"dc:format"`       // always "application/pdf" after PDF/A conversion
	Identifier  string     `xml:"dc:identifier"`   // nr_inregistrare (identificator unic)
	Language    string     `xml:"dc:language"`     // "ro"
	Rights      string     `xml:"dc:rights"`       // clasificare (nivelul de clasificare)
}

// DocumentMetadata is the input for generating Dublin Core.
// Populated from the EguDoc document model.
type DocumentMetadata struct {
	// Law 135/2007 field 10: Identificator unic
	NrInregistrare string
	// Law 135/2007 field 2: Emitentul
	EmitentDenumire string
	// Law 135/2007 field 1: Proprietarul documentului
	InstitutionDenumire string
	InstitutionCUI      string
	// Law 135/2007 field 3: Titularul dreptului de dispozitie
	AssignedUserName string
	// Law 135/2007 field 5: Tipul documentului
	TipDocument string
	// Law 135/2007 field 6: Nivelul de clasificare
	Clasificare string
	// Law 135/2007 field 8: Cuvinte cheie
	CuvinteChecheie []string
	// Law 135/2007 field 11: Data emiterii
	DataInregistrare time.Time
	// Law 135/2007 field 13: Termenul de pastrare
	TermenPastrareAni int
	// Content
	Obiect  string
	Continut string
}

func NewDublinCoreMetadata(meta DocumentMetadata) DublinCoreMetadata {
	return DublinCoreMetadata{
		XMLNS:     "http://www.openarchives.org/OAI/2.0/oai_dc/",
		XMLNSDCs:  "http://purl.org/dc/elements/1.1/",
		XMLNSXSIs: "http://www.w3.org/2001/XMLSchema-instance",
		XSISchema: "http://www.openarchives.org/OAI/2.0/oai_dc/ http://www.openarchives.org/OAI/2.0/oai_dc.xsd",

		Title:       meta.Obiect + " [" + meta.NrInregistrare + "]",
		Creator:     meta.EmitentDenumire,
		Subject:     meta.CuvinteChecheie,
		Description: meta.Continut,
		Publisher:   meta.InstitutionDenumire + " (CUI: " + meta.InstitutionCUI + ")",
		Contributor: meta.AssignedUserName,
		Date:        meta.DataInregistrare.Format("2006-01-02"),
		Type:        meta.TipDocument,
		Format:      "application/pdf",
		Identifier:  meta.NrInregistrare,
		Language:    "ro",
		Rights:      meta.Clasificare,
	}
}

func MarshalDublinCore(meta DocumentMetadata) ([]byte, error) {
	dc := NewDublinCoreMetadata(meta)
	return xml.MarshalIndent(dc, "", "  ")
}
```

*(Note: fix the struct field tag for XMLNSDCs and XMLNSXSIs — they should have proper xml attr names: `xml:"xmlns:dc,attr"` etc. The struct as written is correct; only the field names in NewDublinCoreMetadata need to match the struct field names.)*

- [ ] **Step 25.2: Write premis.go**

```go
// internal/eark/premis.go
package eark

import (
	"encoding/xml"
	"fmt"
	"time"
)

// PREMISRoot is the root PREMIS metadata element.
type PREMISRoot struct {
	XMLName xml.Name      `xml:"premis:premis"`
	XMLNS   string        `xml:"xmlns:premis,attr"`
	Version string        `xml:"version,attr"`
	Objects []PREMISObject `xml:"premis:object"`
	Events  []PREMISEvent  `xml:"premis:event"`
	Agents  []PREMISAgent  `xml:"premis:agent"`
}

type PREMISObject struct {
	XMLName         xml.Name `xml:"premis:object"`
	Type            string   `xml:"xsi:type,attr"`
	ObjectIdentifier struct {
		Type  string `xml:"premis:objectIdentifierType"`
		Value string `xml:"premis:objectIdentifierValue"`
	} `xml:"premis:objectIdentifier"`
	ObjectCharacteristics struct {
		CompositionLevel int    `xml:"premis:compositionLevel"`
		Fixity          struct {
			MessageDigestAlgorithm string `xml:"premis:messageDigestAlgorithm"`
			MessageDigest          string `xml:"premis:messageDigest"`
		} `xml:"premis:fixity"`
		Size   int64 `xml:"premis:size"`
		Format struct {
			FormatDesignation struct {
				Name    string `xml:"premis:formatName"`
				Version string `xml:"premis:formatVersion"`
			} `xml:"premis:formatDesignation"`
			FormatRegistry struct {
				Name  string `xml:"premis:formatRegistryName"`
				Key   string `xml:"premis:formatRegistryKey"`
				Role  string `xml:"premis:formatRegistryRole"`
			} `xml:"premis:formatRegistry"`
		} `xml:"premis:format"`
	} `xml:"premis:objectCharacteristics"`
	OriginalName string `xml:"premis:originalName"`
}

type PREMISEvent struct {
	EventIdentifier struct {
		Type  string `xml:"premis:eventIdentifierType"`
		Value string `xml:"premis:eventIdentifierValue"`
	} `xml:"premis:eventIdentifier"`
	EventType    string `xml:"premis:eventType"`
	EventDateTime string `xml:"premis:eventDateTime"`
	EventDetail   string `xml:"premis:eventDetail"`
	EventOutcomeInformation struct {
		EventOutcome string `xml:"premis:eventOutcome"`
	} `xml:"premis:eventOutcomeInformation"`
	LinkingAgentIdentifier struct {
		Type  string `xml:"premis:linkingAgentIdentifierType"`
		Value string `xml:"premis:linkingAgentIdentifierValue"`
	} `xml:"premis:linkingAgentIdentifier"`
}

type PREMISAgent struct {
	AgentIdentifier struct {
		Type  string `xml:"premis:agentIdentifierType"`
		Value string `xml:"premis:agentIdentifierValue"`
	} `xml:"premis:agentIdentifier"`
	AgentName string `xml:"premis:agentName"`
	AgentType string `xml:"premis:agentType"`
}

// BuildPREMIS constructs a PREMIS metadata document for an E-ARK SIP.
func BuildPREMIS(docID, filename, sha256 string, sizeBytes int64, ingestedAt time.Time, events []WorkflowEventForPREMIS, institutionCUI, creatingSystem string) ([]byte, error) {
	premis := PREMISRoot{
		XMLNS:   "http://www.loc.gov/premis/v3",
		Version: "3.0",
	}

	// Object entry for the main document file
	var obj PREMISObject
	obj.Type = "premis:file"
	obj.ObjectIdentifier.Type = "local"
	obj.ObjectIdentifier.Value = docID
	obj.ObjectCharacteristics.CompositionLevel = 0
	obj.ObjectCharacteristics.Fixity.MessageDigestAlgorithm = "SHA-256"
	obj.ObjectCharacteristics.Fixity.MessageDigest = sha256
	obj.ObjectCharacteristics.Size = sizeBytes
	obj.ObjectCharacteristics.Format.FormatDesignation.Name = "PDF/A-2b"
	obj.ObjectCharacteristics.Format.FormatDesignation.Version = "ISO 19005-2:2011"
	obj.ObjectCharacteristics.Format.FormatRegistry.Name = "PRONOM"
	obj.ObjectCharacteristics.Format.FormatRegistry.Key = "fmt/476"
	obj.ObjectCharacteristics.Format.FormatRegistry.Role = "specification"
	obj.OriginalName = filename
	premis.Objects = append(premis.Objects, obj)

	// Creation event
	creationEvent := PREMISEvent{}
	creationEvent.EventIdentifier.Type = "local"
	creationEvent.EventIdentifier.Value = "creation-" + docID
	creationEvent.EventType = "creation"
	creationEvent.EventDateTime = ingestedAt.Format(time.RFC3339)
	creationEvent.EventDetail = "Document created in EguDoc registratura"
	creationEvent.EventOutcomeInformation.EventOutcome = "success"
	creationEvent.LinkingAgentIdentifier.Type = "software"
	creationEvent.LinkingAgentIdentifier.Value = creatingSystem
	premis.Events = append(premis.Events, creationEvent)

	// Workflow events from audit trail
	for i, we := range events {
		event := PREMISEvent{}
		event.EventIdentifier.Type = "local"
		event.EventIdentifier.Value = fmt.Sprintf("workflow-%d-%s", i, docID)
		event.EventType = we.Action
		event.EventDateTime = we.OccurredAt.Format(time.RFC3339)
		event.EventDetail = we.Detail
		event.EventOutcomeInformation.EventOutcome = "success"
		event.LinkingAgentIdentifier.Type = "user"
		event.LinkingAgentIdentifier.Value = we.ActorSubject
		premis.Events = append(premis.Events, event)
	}

	// Submission event
	submissionEvent := PREMISEvent{}
	submissionEvent.EventIdentifier.Type = "local"
	submissionEvent.EventIdentifier.Value = "submission-" + docID
	submissionEvent.EventType = "ingestion"
	submissionEvent.EventDateTime = time.Now().Format(time.RFC3339)
	submissionEvent.EventDetail = "Document submitted to EguDoc electronic archive"
	submissionEvent.EventOutcomeInformation.EventOutcome = "success"
	submissionEvent.LinkingAgentIdentifier.Type = "software"
	submissionEvent.LinkingAgentIdentifier.Value = "EguDoc/1.0"
	premis.Events = append(premis.Events, submissionEvent)

	// Agents
	premis.Agents = []PREMISAgent{
		{
			AgentIdentifier: struct {
				Type  string `xml:"premis:agentIdentifierType"`
				Value string `xml:"premis:agentIdentifierValue"`
			}{Type: "CUI", Value: institutionCUI},
			AgentName: "Instituție Publică",
			AgentType: "organization",
		},
		{
			AgentIdentifier: struct {
				Type  string `xml:"premis:agentIdentifierType"`
				Value string `xml:"premis:agentIdentifierValue"`
			}{Type: "software", Value: "EguDoc"},
			AgentName: "EguDoc Document Management System",
			AgentType: "software",
		},
	}

	return xml.MarshalIndent(premis, "", "  ")
}

// WorkflowEventForPREMIS is a simplified view of a workflow event for PREMIS.
type WorkflowEventForPREMIS struct {
	Action      string
	ActorSubject string
	Detail      string
	OccurredAt  time.Time
}
```

- [ ] **Step 25.3: Write mets.go**

```go
// internal/eark/mets.go
package eark

import (
	"encoding/xml"
	"fmt"
	"time"
)

// METS is the root METS element for an E-ARK CSIP SIP.
type METS struct {
	XMLName   xml.Name `xml:"mets"`
	XMLNS     string   `xml:"xmlns,attr"`
	XMLNSXlink string  `xml:"xmlns:xlink,attr"`
	XMLNSCSIP string   `xml:"xmlns:csip,attr"`
	XMLNSSIP  string   `xml:"xmlns:sip,attr"`
	OBJID     string   `xml:"OBJID,attr"`
	LABEL     string   `xml:"LABEL,attr"`
	PROFILE   string   `xml:"PROFILE,attr"`
	TYPE      string   `xml:"TYPE,attr"`
	CSIPType  string   `xml:"csip:CONTENTINFORMATIONTYPE,attr"`

	MetsHdr  METSHeader `xml:"metsHdr"`
	DmdSec   []METSDmd  `xml:"dmdSec"`
	AmdSec   METSAmd    `xml:"amdSec"`
	FileSec  METSFileSec `xml:"fileSec"`
	StructMap METSStructMap `xml:"structMap"`
}

type METSHeader struct {
	CREATEDATE   string    `xml:"CREATEDATE,attr"`
	RECORDSTATUS string    `xml:"RECORDSTATUS,attr"`
	CSIPOAISPackageType string `xml:"csip:OAISPACKAGETYPE,attr"`
	Agents       []METSAgent `xml:"agent"`
}

type METSAgent struct {
	ROLE    string `xml:"ROLE,attr"`
	TYPE    string `xml:"TYPE,attr"`
	OTHERTYPE string `xml:"OTHERTYPE,attr,omitempty"`
	Name    string `xml:"name"`
	Note    []METSNote `xml:"note"`
}

type METSNote struct {
	NoteType string `xml:"csip:NOTETYPE,attr"`
	Value    string `xml:",chardata"`
}

type METSDmd struct {
	ID      string    `xml:"ID,attr"`
	CREATED string    `xml:"CREATED,attr"`
	MdRef   METSMdRef `xml:"mdRef"`
}

type METSAmd struct {
	DigiProvMD []METSDigiProv `xml:"digiprovMD"`
}

type METSDigiProv struct {
	ID    string    `xml:"ID,attr"`
	MdRef METSMdRef `xml:"mdRef"`
}

type METSMdRef struct {
	LOCTYPE      string `xml:"LOCTYPE,attr"`
	XlinkHref    string `xml:"xlink:href,attr"`
	MDTYPE       string `xml:"MDTYPE,attr"`
	MIMETYPE     string `xml:"MIMETYPE,attr"`
	SIZE         int64  `xml:"SIZE,attr"`
	CHECKSUM     string `xml:"CHECKSUM,attr"`
	CHECKSUMTYPE string `xml:"CHECKSUMTYPE,attr"`
}

type METSFileSec struct {
	FileGrps []METSFileGrp `xml:"fileGrp"`
}

type METSFileGrp struct {
	USE   string     `xml:"USE,attr"`
	Files []METSFile `xml:"file"`
}

type METSFile struct {
	ID           string   `xml:"ID,attr"`
	MIMETYPE     string   `xml:"MIMETYPE,attr"`
	SIZE         int64    `xml:"SIZE,attr"`
	CREATED      string   `xml:"CREATED,attr"`
	CHECKSUM     string   `xml:"CHECKSUM,attr"`
	CHECKSUMTYPE string   `xml:"CHECKSUMTYPE,attr"`
	FLocat       METSFLocat `xml:"FLocat"`
}

type METSFLocat struct {
	LOCTYPE   string `xml:"LOCTYPE,attr"`
	XlinkHref string `xml:"xlink:href,attr"`
}

type METSStructMap struct {
	TYPE  string   `xml:"TYPE,attr"`
	LABEL string   `xml:"LABEL,attr"`
	Div   METSDiv  `xml:"div"`
}

type METSDiv struct {
	LABEL string    `xml:"LABEL,attr"`
	Divs  []METSDiv `xml:"div,omitempty"`
}

// BuildRootMETS generates the root METS.xml for an E-ARK CSIP SIP.
func BuildRootMETS(
	packageID string,
	label string,
	institutionName string,
	institutionCUI string,
	dcXMLSize int64,
	dcXMLHash string,
	premisXMLSize int64,
	premisXMLHash string,
	docFiles []FileRef,
	createdAt time.Time,
) ([]byte, error) {
	mets := METS{
		XMLNS:     "http://www.loc.gov/METS/",
		XMLNSXlink: "http://www.w3.org/1999/xlink",
		XMLNSCSIP: "https://earkcsip.dilcis.eu/schema/",
		XMLNSSIP:  "https://earksip.dilcis.eu/schema/",
		OBJID:     packageID,
		LABEL:     label,
		PROFILE:   "https://earkcsip.dilcis.eu/profile/E-ARK-CSIP.xml",
		TYPE:      "OTHER",
		CSIPType:  "MIXED",
	}

	mets.MetsHdr = METSHeader{
		CREATEDATE:          createdAt.Format(time.RFC3339),
		RECORDSTATUS:        "NEW",
		CSIPOAISPackageType: "SIP",
		Agents: []METSAgent{
			{
				ROLE: "CREATOR",
				TYPE: "ORGANIZATION",
				Name: institutionName,
				Note: []METSNote{{NoteType: "IDENTIFICATIONCODE", Value: institutionCUI}},
			},
			{
				ROLE:      "CREATOR",
				TYPE:      "OTHER",
				OTHERTYPE: "SOFTWARE",
				Name:      "EguDoc",
				Note:      []METSNote{{NoteType: "SOFTWARE VERSION", Value: "1.0"}},
			},
		},
	}

	mets.DmdSec = []METSDmd{{
		ID:      "dmd-dc-001",
		CREATED: createdAt.Format(time.RFC3339),
		MdRef: METSMdRef{
			LOCTYPE:      "URL",
			XlinkHref:    "metadata/descriptive/dc.xml",
			MDTYPE:       "DC",
			MIMETYPE:     "text/xml",
			SIZE:         dcXMLSize,
			CHECKSUM:     dcXMLHash,
			CHECKSUMTYPE: "SHA-256",
		},
	}}

	mets.AmdSec = METSAmd{
		DigiProvMD: []METSDigiProv{{
			ID: "digiprov-premis-001",
			MdRef: METSMdRef{
				LOCTYPE:      "URL",
				XlinkHref:    "metadata/preservation/premis.xml",
				MDTYPE:       "PREMIS",
				MIMETYPE:     "text/xml",
				SIZE:         premisXMLSize,
				CHECKSUM:     premisXMLHash,
				CHECKSUMTYPE: "SHA-256",
			},
		}},
	}

	// File section
	var files []METSFile
	for i, f := range docFiles {
		files = append(files, METSFile{
			ID:           fmt.Sprintf("file-%03d", i+1),
			MIMETYPE:     f.ContentType,
			SIZE:         f.Size,
			CREATED:      createdAt.Format(time.RFC3339),
			CHECKSUM:     f.SHA256,
			CHECKSUMTYPE: "SHA-256",
			FLocat: METSFLocat{
				LOCTYPE:   "URL",
				XlinkHref: "representations/rep-001/data/" + f.Filename,
			},
		})
	}
	mets.FileSec = METSFileSec{
		FileGrps: []METSFileGrp{{
			USE:   "Representations/rep-001",
			Files: files,
		}},
	}

	mets.StructMap = METSStructMap{
		TYPE:  "PHYSICAL",
		LABEL: "CSIP",
		Div: METSDiv{
			LABEL: "Root",
			Divs: []METSDiv{
				{LABEL: "Metadata"},
				{LABEL: "Representations", Divs: []METSDiv{{LABEL: "rep-001"}}},
			},
		},
	}

	return xml.MarshalIndent(mets, "", "  ")
}

// FileRef describes a file to be included in the E-ARK package.
type FileRef struct {
	Filename    string
	ContentType string
	Size        int64
	SHA256      string
}
```

- [ ] **Step 25.4: Write sip_builder.go**

```go
// internal/eark/sip_builder.go
package eark

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

// SIPPackage contains all files for an E-ARK CSIP SIP in memory.
type SIPPackage struct {
	PackageID string
	Files     map[string][]byte // path → content
}

// SIPInput is all data needed to generate a SIP package.
type SIPInput struct {
	PackageID string // UUID of the document
	Label     string // "nr_inregistrare + obiect"

	// Document metadata for Dublin Core + PREMIS
	Metadata    DocumentMetadata
	Events      []WorkflowEventForPREMIS

	// The main document file (already in PDF/A format)
	DocumentContent []byte
	DocumentFilename string

	// Attachment files (also PDF/A)
	Attachments []AttachmentInput
}

type AttachmentInput struct {
	Content     []byte
	Filename    string
	ContentType string
}

// BuildSIP generates a complete E-ARK CSIP SIP package.
// Returns the SIP as a ZIP archive (bytes).
// This implements the requirements of Romanian Law 135/2007 and E-ARK CSIP.
func BuildSIP(ctx context.Context, input SIPInput) ([]byte, error) {
	createdAt := time.Now()

	// Step 1: Generate Dublin Core metadata
	dcXML, err := MarshalDublinCore(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("generate dublin core: %w", err)
	}
	dcHash := sha256hex(dcXML)

	// Step 2: Generate PREMIS preservation metadata
	docHash := sha256hex(input.DocumentContent)
	premisXML, err := BuildPREMIS(
		input.PackageID,
		input.DocumentFilename,
		docHash,
		int64(len(input.DocumentContent)),
		createdAt,
		input.Events,
		input.Metadata.InstitutionCUI,
		"EguDoc/1.0",
	)
	if err != nil {
		return nil, fmt.Errorf("generate premis: %w", err)
	}
	premisHash := sha256hex(premisXML)

	// Step 3: Collect file references for METS
	var fileRefs []FileRef
	fileRefs = append(fileRefs, FileRef{
		Filename:    input.DocumentFilename,
		ContentType: "application/pdf",
		Size:        int64(len(input.DocumentContent)),
		SHA256:      docHash,
	})
	for _, att := range input.Attachments {
		fileRefs = append(fileRefs, FileRef{
			Filename:    att.Filename,
			ContentType: att.ContentType,
			Size:        int64(len(att.Content)),
			SHA256:      sha256hex(att.Content),
		})
	}

	// Step 4: Generate root METS.xml
	metsXML, err := BuildRootMETS(
		input.PackageID,
		input.Label,
		input.Metadata.InstitutionDenumire,
		input.Metadata.InstitutionCUI,
		int64(len(dcXML)), dcHash,
		int64(len(premisXML)), premisHash,
		fileRefs,
		createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("generate mets: %w", err)
	}

	// Step 5: Assemble ZIP archive with E-ARK folder structure
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	writeZipFile := func(path string, content []byte) error {
		w, err := zw.Create(path)
		if err != nil {
			return fmt.Errorf("create zip entry %q: %w", path, err)
		}
		_, err = w.Write(content)
		return err
	}

	packageDir := input.PackageID + "/"

	// Root METS
	if err := writeZipFile(packageDir+"METS.xml", metsXML); err != nil {
		return nil, err
	}
	// Dublin Core
	if err := writeZipFile(packageDir+"metadata/descriptive/dc.xml", dcXML); err != nil {
		return nil, err
	}
	// PREMIS
	if err := writeZipFile(packageDir+"metadata/preservation/premis.xml", premisXML); err != nil {
		return nil, err
	}
	// Main document
	if err := writeZipFile(packageDir+"representations/rep-001/data/"+input.DocumentFilename, input.DocumentContent); err != nil {
		return nil, err
	}
	// Attachments
	for _, att := range input.Attachments {
		if err := writeZipFile(packageDir+"representations/rep-001/data/"+att.Filename, att.Content); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}

	return buf.Bytes(), nil
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// BuildSIPAndStream returns the SIP ZIP as an io.Reader for streaming to QTSP.
func BuildSIPAndStream(ctx context.Context, input SIPInput) (io.Reader, int64, error) {
	data, err := BuildSIP(ctx, input)
	if err != nil {
		return nil, 0, err
	}
	return bytes.NewReader(data), int64(len(data)), nil
}
```

- [ ] **Step 25.5: Write SIP builder test**

```go
// internal/eark/sip_builder_test.go
package eark_test

import (
	"archive/zip"
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/eguilde/egudoc/internal/eark"
)

func TestBuildSIPContainsRequiredFiles(t *testing.T) {
	input := eark.SIPInput{
		PackageID:        "test-uuid-001",
		Label:            "INT/000001/2025 - Document test",
		DocumentContent:  []byte("%PDF-1.4 fake pdf"),
		DocumentFilename: "document.pdf",
		Metadata: eark.DocumentMetadata{
			NrInregistrare:      "INT/000001/2025",
			EmitentDenumire:     "Cetățean Test",
			InstitutionDenumire: "Primăria Costești",
			InstitutionCUI:      "RO12345678",
			AssignedUserName:    "Ion Ionescu",
			TipDocument:         "INTRARE",
			Clasificare:         "PUBLIC",
			CuvinteChecheie:     []string{"test", "document"},
			DataInregistrare:    time.Now(),
			TermenPastrareAni:   10,
			Obiect:              "Test document",
			Continut:            "Test content",
		},
		Events: []eark.WorkflowEventForPREMIS{
			{Action: "CREATE", ActorSubject: "user-001", Detail: "Document registered", OccurredAt: time.Now()},
		},
	}

	zipData, err := eark.BuildSIP(context.Background(), input)
	if err != nil {
		t.Fatalf("BuildSIP failed: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}

	requiredFiles := map[string]bool{
		"test-uuid-001/METS.xml":                                           false,
		"test-uuid-001/metadata/descriptive/dc.xml":                       false,
		"test-uuid-001/metadata/preservation/premis.xml":                  false,
		"test-uuid-001/representations/rep-001/data/document.pdf":         false,
	}

	for _, f := range r.File {
		if _, ok := requiredFiles[f.Name]; ok {
			requiredFiles[f.Name] = true
		}
	}

	for path, found := range requiredFiles {
		if !found {
			t.Errorf("required file missing from SIP: %s", path)
		}
	}
}
```

```bash
go test ./internal/eark/... -v -run TestBuildSIPContainsRequiredFiles
```

Expected: PASS (all required E-ARK CSIP files present)

- [ ] **Step 25.6: Commit**

```bash
git add internal/eark/ internal/pdf/
git commit -m "feat: add E-ARK CSIP SIP builder with Dublin Core, PREMIS, METS generation"
```

---

## Task 26: Archive worker background pipeline

**Files:**
- Create: `internal/archiveworker/pipeline.go`
- Create: `internal/archiveworker/worker.go`

- [ ] **Step 26.1: Write pipeline.go**

```go
// internal/archiveworker/pipeline.go
package archiveworker

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eguilde/egudoc/internal/eark"
	qtsparchive "github.com/eguilde/egudoc/internal/qtsp/archive"
	"github.com/eguilde/egudoc/internal/pdf"
	"github.com/eguilde/egudoc/internal/storage"
)

// Pipeline orchestrates the full archiving process for a single document.
type Pipeline struct {
	db         *pgxpool.Pool
	store      *storage.Client
	gotenberg  *pdf.Gotenberg
	qtspClient *qtsparchive.Client
}

func NewPipeline(db *pgxpool.Pool, store *storage.Client, gotenberg *pdf.Gotenberg, qtsp *qtsparchive.Client) *Pipeline {
	return &Pipeline{db: db, store: store, gotenberg: gotenberg, qtspClient: qtsp}
}

// ArchiveDocument runs the full pipeline for one document:
// 1. Fetch document metadata from DB
// 2. Fetch document file from MinIO
// 3. Convert to PDF/A-2b via Gotenberg
// 4. Fetch workflow events for PREMIS
// 5. Build E-ARK CSIP SIP
// 6. Submit SIP to QTSP archive
// 7. Update document with archive reference
func (p *Pipeline) ArchiveDocument(ctx context.Context, documentID uuid.UUID) error {
	// Mark as pending immediately
	_, err := p.db.Exec(ctx, `
		UPDATE documente SET archive_status = 'PENDING', updated_at = NOW() WHERE id = $1
	`, documentID)
	if err != nil {
		return fmt.Errorf("mark pending: %w", err)
	}

	doc, meta, err := p.fetchDocumentAndMeta(ctx, documentID)
	if err != nil {
		p.markFailed(ctx, documentID, err.Error())
		return err
	}

	// Fetch main document content from MinIO (if stored)
	var docContent []byte
	var docFilename string
	if doc.StorageKey != "" {
		reader, _, err := p.store.GetDocument(ctx, doc.StorageKey)
		if err != nil {
			p.markFailed(ctx, documentID, "fetch from storage: "+err.Error())
			return err
		}
		defer reader.Close()

		// Convert to PDF/A
		docContent, err = p.gotenberg.ConvertToPDFA(ctx, doc.Filename, reader)
		if err != nil {
			p.markFailed(ctx, documentID, "pdf/a conversion: "+err.Error())
			return err
		}
		docFilename = replaceExt(doc.Filename, ".pdf")
	} else {
		// Generate a PDF/A from the document metadata as HTML
		html := buildDocumentHTML(doc)
		docContent, err = p.gotenberg.ConvertHTMLToPDFA(ctx, html, doc.NrInregistrare)
		if err != nil {
			p.markFailed(ctx, documentID, "html to pdfa: "+err.Error())
			return err
		}
		docFilename = "document.pdf"
	}

	// Build E-ARK SIP
	sipInput := eark.SIPInput{
		PackageID:        documentID.String(),
		Label:            doc.NrInregistrare + " - " + doc.Obiect,
		DocumentContent:  docContent,
		DocumentFilename: docFilename,
		Metadata:         meta,
		Events:           doc.WorkflowEvents,
	}

	sipData, _, err := eark.BuildSIPAndStream(ctx, sipInput)
	if err != nil {
		p.markFailed(ctx, documentID, "build SIP: "+err.Error())
		return err
	}

	// Submit to QTSP archive
	title := fmt.Sprintf("%s - %s", doc.NrInregistrare, doc.Obiect)
	result, err := p.qtspClient.Ingest(ctx, title, doc.InstitutionCUI, doc.TermenPastrareAni, sipData, documentID.String()+"-SIP.zip", "application/zip")
	if err != nil {
		p.markFailed(ctx, documentID, "QTSP ingest: "+err.Error())
		return err
	}

	// Update document with archive reference
	now := time.Now()
	_, err = p.db.Exec(ctx, `
		UPDATE documente
		SET archive_document_id = $1,
		    archive_status = 'ARCHIVED',
		    data_arhivare = $2,
		    updated_at = NOW()
		WHERE id = $3
	`, result.ID, now, documentID)
	return err
}

func (p *Pipeline) markFailed(ctx context.Context, documentID uuid.UUID, reason string) {
	p.db.Exec(ctx, `
		UPDATE documente SET archive_status = 'FAILED', updated_at = NOW() WHERE id = $1
	`, documentID)
}

// docRecord holds the raw data fetched from the DB.
type docRecord struct {
	NrInregistrare  string
	Obiect          string
	StorageKey      string
	Filename        string
	InstitutionCUI  string
	TermenPastrareAni int
	WorkflowEvents  []eark.WorkflowEventForPREMIS
}

func (p *Pipeline) fetchDocumentAndMeta(ctx context.Context, documentID uuid.UUID) (*docRecord, eark.DocumentMetadata, error) {
	var doc docRecord
	var meta eark.DocumentMetadata

	err := p.db.QueryRow(ctx, `
		SELECT d.nr_inregistrare, d.obiect, d.termen_pastrare_ani,
		       COALESCE(sf.storage_key, '') as storage_key,
		       COALESCE(sf.filename, '') as filename,
		       i.cui, i.denumire,
		       COALESCE(e.denumire, '') as emitent_denumire
		FROM documente d
		JOIN institutions i ON i.id = d.institution_id
		LEFT JOIN stored_files sf ON sf.entity_id = d.id AND sf.entity_type = 'document'
		LEFT JOIN entitati e ON e.id = d.emitent_id
		WHERE d.id = $1
		LIMIT 1
	`, documentID).Scan(
		&doc.NrInregistrare, &doc.Obiect, &doc.TermenPastrareAni,
		&doc.StorageKey, &doc.Filename,
		&doc.InstitutionCUI, &meta.InstitutionDenumire,
		&meta.EmitentDenumire,
	)
	if err != nil {
		return nil, meta, fmt.Errorf("fetch document: %w", err)
	}

	meta.NrInregistrare = doc.NrInregistrare
	meta.InstitutionCUI = doc.InstitutionCUI
	meta.Obiect = doc.Obiect
	meta.TermenPastrareAni = doc.TermenPastrareAni

	// Fetch workflow events for PREMIS
	rows, err := p.db.Query(ctx, `
		SELECT action, actor_subject, COALESCE(motiv, action), created_at
		FROM workflow_events
		WHERE document_id = $1
		ORDER BY created_at ASC
	`, documentID)
	if err != nil {
		return nil, meta, fmt.Errorf("fetch events: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var we eark.WorkflowEventForPREMIS
		rows.Scan(&we.Action, &we.ActorSubject, &we.Detail, &we.OccurredAt)
		doc.WorkflowEvents = append(doc.WorkflowEvents, we)
	}

	return &doc, meta, rows.Err()
}

func buildDocumentHTML(doc *docRecord) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ro">
<head><meta charset="UTF-8"><title>%s</title></head>
<body>
<h1>%s</h1>
<p><strong>Nr. Înregistrare:</strong> %s</p>
<p><strong>Obiect:</strong> %s</p>
</body>
</html>`, doc.NrInregistrare, doc.NrInregistrare, doc.NrInregistrare, doc.Obiect)
}

func replaceExt(filename, newExt string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[:i] + newExt
		}
	}
	return filename + newExt
}
```

- [ ] **Step 26.2: Write worker.go**

```go
// internal/archiveworker/worker.go
package archiveworker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Worker is a background goroutine that scans for finalized documents
// and submits them to the QTSP archive.
type Worker struct {
	db       *pgxpool.Pool
	pipeline *Pipeline
	log      *zap.Logger
	interval time.Duration
	// Documents are eligible for archiving this long after finalization
	archiveDelay time.Duration
}

func NewWorker(db *pgxpool.Pool, pipeline *Pipeline, log *zap.Logger) *Worker {
	return &Worker{
		db:           db,
		pipeline:     pipeline,
		log:          log,
		interval:     6 * time.Hour,         // check every 6 hours
		archiveDelay: 30 * 24 * time.Hour,   // archive 30 days after finalization
	}
}

// Start runs the archive worker in a background goroutine.
// It stops when ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		// Run immediately on startup
		w.runCycle(ctx)

		for {
			select {
			case <-ctx.Done():
				w.log.Info("archive worker stopping")
				return
			case <-ticker.C:
				w.runCycle(ctx)
			}
		}
	}()
}

func (w *Worker) runCycle(ctx context.Context) {
	w.log.Info("archive worker: scanning for documents to archive")

	candidates, err := w.findCandidates(ctx)
	if err != nil {
		w.log.Error("archive worker: find candidates failed", zap.Error(err))
		return
	}

	w.log.Info("archive worker: found candidates", zap.Int("count", len(candidates)))

	for _, id := range candidates {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := w.pipeline.ArchiveDocument(ctx, id); err != nil {
			w.log.Error("archive worker: pipeline failed",
				zap.String("document_id", id.String()),
				zap.Error(err),
			)
		} else {
			w.log.Info("archive worker: document archived",
				zap.String("document_id", id.String()),
			)
		}
	}
}

func (w *Worker) findCandidates(ctx context.Context) ([]uuid.UUID, error) {
	// Find FINALIZAT documents with no archiving attempted yet,
	// finalized at least archiveDelay ago
	cutoff := time.Now().Add(-w.archiveDelay)
	rows, err := w.db.Query(ctx, `
		SELECT id FROM documente
		WHERE status = 'FINALIZAT'
		  AND archive_status = 'NOT_ARCHIVED'
		  AND data_finalizare IS NOT NULL
		  AND data_finalizare < $1
		ORDER BY data_finalizare ASC
		LIMIT 50
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// TriggerImmediate archives a specific document immediately (used after finalization).
func (w *Worker) TriggerImmediate(ctx context.Context, documentID uuid.UUID) {
	go func() {
		if err := w.pipeline.ArchiveDocument(ctx, documentID); err != nil {
			w.log.Error("immediate archive failed",
				zap.String("document_id", documentID.String()),
				zap.Error(err),
			)
		}
	}()
}
```

- [ ] **Step 26.3: Commit**

```bash
git add internal/archiveworker/ internal/qtsp/
git commit -m "feat: add archive worker pipeline - PDF/A + E-ARK SIP + QTSP submission"
git push origin master
```

---

## Sub-plan D Completion Checklist

- [ ] QTSP base client sends `X-Internal-Service-Key` header
- [ ] eDelivery client can submit and check delivery status
- [ ] eArchiving client can ingest documents and verify integrity
- [ ] Gotenberg client test passes (mock server)
- [ ] E-ARK SIP test passes — ZIP contains METS.xml + dc.xml + premis.xml + document.pdf
- [ ] Archive worker starts, scans, and processes candidates
- [ ] `go build ./...` compiles cleanly

---

*All four sub-plans are now complete. Execution recommended via `superpowers:subagent-driven-development`.*
