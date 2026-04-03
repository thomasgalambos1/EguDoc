package delivery

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dispatcher struct {
	client *Client
	db     *pgxpool.Pool
}

func NewDispatcher(client *Client, db *pgxpool.Pool) *Dispatcher {
	return &Dispatcher{client: client, db: db}
}

// ShouldDeliver returns true if the document's destinatar is registered in the eDelivery network.
func (d *Dispatcher) ShouldDeliver(ctx context.Context, documentID uuid.UUID) (bool, string, error) {
	var destinatarEmail string
	var deliveryParticipantID *string

	err := d.db.QueryRow(ctx, `
		SELECT COALESCE(e.email, ''), e.delivery_participant_id
		FROM documente doc
		LEFT JOIN entitati e ON e.id = doc.destinatar_id
		WHERE doc.id = $1 AND doc.destinatar_id IS NOT NULL
	`, documentID).Scan(&destinatarEmail, &deliveryParticipantID)
	if err != nil {
		return false, "", fmt.Errorf("lookup destinatar: %w", err)
	}

	if deliveryParticipantID != nil && *deliveryParticipantID != "" {
		return true, destinatarEmail, nil
	}
	return false, "", nil
}

// Dispatch submits a document via qualified eDelivery and records the message ID.
func (d *Dispatcher) Dispatch(ctx context.Context, documentID uuid.UUID, subject string, content io.Reader, filename string, recipientEmail string) error {
	req := SubmitMessageRequest{
		RecipientEmail: recipientEmail,
		Subject:        subject,
		ContentType:    "application/pdf",
		RetentionDays:  90,
	}

	result, err := d.client.Submit(ctx, req, content, filename)
	if err != nil {
		_, dbErr := d.db.Exec(ctx, `UPDATE documente SET delivery_status = 'FAILED', updated_at = NOW() WHERE id = $1`, documentID)
		if dbErr != nil {
			return fmt.Errorf("delivery submit: %w (also failed to record FAILED status: %v)", err, dbErr)
		}
		return fmt.Errorf("delivery submit: %w", err)
	}

	_, err = d.db.Exec(ctx, `
		UPDATE documente
		SET delivery_message_id = $1, delivery_status = 'SUBMITTED', updated_at = NOW()
		WHERE id = $2
	`, result.ID, documentID)
	if err != nil {
		return fmt.Errorf("update delivery status: %w", err)
	}
	return nil
}
