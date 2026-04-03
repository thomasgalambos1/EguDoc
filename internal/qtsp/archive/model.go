package archive

import "time"

type ArchiveStatus string

const (
	ArchiveStatusPending  ArchiveStatus = "pending"
	ArchiveStatusArchived ArchiveStatus = "archived"
	ArchiveStatusVerified ArchiveStatus = "verified"
	ArchiveStatusExpired  ArchiveStatus = "expired"
)

type IngestResponse struct {
	ID                string        `json:"id"`
	Title             string        `json:"title"`
	ContentHash       string        `json:"content_hash"`
	StorageKey        string        `json:"storage_key"`
	ArchiveStatus     ArchiveStatus `json:"archive_status"`
	RetentionYears    int           `json:"retention_years"`
	IngestedAt        time.Time     `json:"ingested_at"`
	NextRetimestampAt *time.Time    `json:"next_retimestamp_at,omitempty"`
	ExpiresAt         *time.Time    `json:"expires_at,omitempty"`
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
	DocumentID string    `json:"document_id"`
	ProofJWT   string    `json:"proof_jwt"`
	IssuedAt   time.Time `json:"issued_at"`
}
