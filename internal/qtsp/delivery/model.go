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
	StorageKey  string `json:"storage_key"`
}

type SubmitMessageResponse struct {
	ID          string        `json:"id"`
	Status      MessageStatus `json:"status"`
	SubmittedAt time.Time     `json:"submitted_at"`
	ExpiresAt   time.Time     `json:"expires_at"`
}

type MessageInfo struct {
	ID          string        `json:"id"`
	SenderID    string        `json:"sender_id"`
	RecipientID string        `json:"recipient_id"`
	Subject     string        `json:"subject"`
	Status      MessageStatus `json:"status"`
	SubmittedAt time.Time     `json:"submitted_at"`
	DeliveredAt *time.Time    `json:"delivered_at,omitempty"`
	RetrievedAt *time.Time    `json:"retrieved_at,omitempty"`
	ExpiresAt   time.Time     `json:"expires_at"`
}

type Evidence struct {
	ID           string    `json:"id"`
	MessageID    string    `json:"message_id"`
	EvidenceType string    `json:"evidence_type"`
	EvidenceJWT  string    `json:"evidence_jwt"`
	CreatedAt    time.Time `json:"created_at"`
}
