// internal/workflow/audit.go
package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditService struct {
	db *pgxpool.Pool
}

func NewAuditService(db *pgxpool.Pool) *AuditService {
	return &AuditService{db: db}
}

// LogEventTx writes an immutable workflow event within an existing transaction.
func (s *AuditService) LogEventTx(ctx context.Context, tx pgx.Tx, event WorkflowEvent) error {
	metaJSON, _ := json.Marshal(event.Metadata)

	_, err := tx.Exec(ctx, `
		INSERT INTO workflow_events
		  (document_id, institution_id, action, old_status, new_status,
		   actor_subject, actor_ip,
		   from_compartiment_id, to_compartiment_id, assigned_user_subject,
		   motiv, metadata, visibility)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		event.DocumentID, event.InstitutionID, event.Action,
		event.OldStatus, event.NewStatus,
		event.ActorSubject, event.ActorIP,
		event.FromCompartimentID, event.ToCompartimentID, event.AssignedUserSubject,
		event.Motiv, metaJSON, event.Visibility,
	)
	if err != nil {
		return fmt.Errorf("insert workflow event: %w", err)
	}
	return nil
}

// GetAuditTrail returns the workflow history for a document.
func (s *AuditService) GetAuditTrail(ctx context.Context, documentID string) ([]WorkflowEvent, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, document_id, institution_id, action, old_status, new_status,
		       actor_subject, actor_ip,
		       from_compartiment_id, to_compartiment_id, assigned_user_subject,
		       motiv, metadata, visibility, created_at
		FROM workflow_events
		WHERE document_id = $1
		ORDER BY created_at ASC
	`, documentID)
	if err != nil {
		return nil, fmt.Errorf("query audit trail: %w", err)
	}
	defer rows.Close()

	var events []WorkflowEvent
	for rows.Next() {
		var e WorkflowEvent
		var metaJSON []byte
		if err := rows.Scan(
			&e.ID, &e.DocumentID, &e.InstitutionID, &e.Action, &e.OldStatus, &e.NewStatus,
			&e.ActorSubject, &e.ActorIP,
			&e.FromCompartimentID, &e.ToCompartimentID, &e.AssignedUserSubject,
			&e.Motiv, &metaJSON, &e.Visibility, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan workflow event: %w", err)
		}
		if len(metaJSON) > 0 {
			if err := json.Unmarshal(metaJSON, &e.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal metadata: %w", err)
			}
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
