// internal/workflow/engine.go
package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/eguilde/egudoc/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Engine executes validated workflow transitions on documents.
type Engine struct {
	db    *pgxpool.Pool
	audit *AuditService
}

func NewEngine(db *pgxpool.Pool) *Engine {
	return &Engine{db: db, audit: NewAuditService(db)}
}

// Advance processes a workflow action on a document.
// It validates the transition, updates the document, and logs the event atomically.
func (e *Engine) Advance(ctx context.Context, documentID uuid.UUID, req ActionRequest, claims *auth.Claims, ip string) (*WorkflowEvent, error) {
	tx, err := e.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Lock the document row
	var currentStatus string
	var institutionID uuid.UUID
	var lockedUntil *time.Time
	err = tx.QueryRow(ctx, `
		SELECT status, institution_id, workflow_locked_until
		FROM documente WHERE id = $1
		FOR UPDATE
	`, documentID).Scan(&currentStatus, &institutionID, &lockedUntil)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("document not found")
	}
	if err != nil {
		return nil, fmt.Errorf("lock document: %w", err)
	}

	// Check workflow lock
	if lockedUntil != nil && time.Now().Before(*lockedUntil) {
		return nil, fmt.Errorf("document is locked until %s", lockedUntil.Format(time.RFC3339))
	}

	// Validate transition
	transitions, ok := ValidTransitions[currentStatus]
	if !ok {
		return nil, fmt.Errorf("unknown document status: %s", currentStatus)
	}
	newStatus, ok := transitions[req.Action]
	if !ok {
		return nil, fmt.Errorf("action %q is not allowed when document status is %q", req.Action, currentStatus)
	}

	// Validate action-specific requirements
	switch req.Action {
	case ActionAssignCompartiment:
		if req.CompartimentID == nil {
			return nil, fmt.Errorf("compartiment_id required for ASSIGN_COMPARTIMENT")
		}
	case ActionAssignUser:
		if req.AssigneeSubject == "" {
			return nil, fmt.Errorf("assignee_subject required for ASSIGN_USER")
		}
	case ActionSendForApproval:
		if req.AssigneeSubject == "" {
			return nil, fmt.Errorf("assignee_subject (approver) required for SEND_FOR_APPROVAL")
		}
	case ActionCancel:
		if req.Motiv == "" {
			return nil, fmt.Errorf("motiv required for CANCEL")
		}
	}

	// Compute nullable fields for the UPDATE
	var newCompartimentID *uuid.UUID
	var newUserSubject *string
	var newApproverSubject *string
	var newDataFinalizare *time.Time
	var newMotivAnulare *string
	isReject := false

	switch req.Action {
	case ActionAssignCompartiment:
		newCompartimentID = req.CompartimentID
	case ActionAssignUser:
		s := req.AssigneeSubject
		newUserSubject = &s
	case ActionSendForApproval:
		s := req.AssigneeSubject
		newApproverSubject = &s
	case ActionApprove, ActionFinalize:
		now := time.Now()
		newDataFinalizare = &now
	case ActionReject:
		isReject = true
	case ActionCancel:
		s := req.Motiv
		newMotivAnulare = &s
	}

	_, err = tx.Exec(ctx, `
		UPDATE documente SET
			status = $1,
			compartiment_curent_id = CASE WHEN $2::uuid IS NOT NULL THEN $2::uuid ELSE compartiment_curent_id END,
			user_curent_subject = $3,
			awaiting_approver_subject = $4,
			workflow_locked_until = NULL,
			rejection_count = CASE WHEN $5 THEN rejection_count + 1 ELSE rejection_count END,
			data_finalizare = $6,
			motiv_anulare = $7,
			updated_at = NOW()
		WHERE id = $8
	`,
		newStatus,
		newCompartimentID,
		newUserSubject,
		newApproverSubject,
		isReject,
		newDataFinalizare,
		newMotivAnulare,
		documentID,
	)
	if err != nil {
		return nil, fmt.Errorf("update document: %w", err)
	}

	event := WorkflowEvent{
		DocumentID:    documentID,
		InstitutionID: institutionID,
		Action:        req.Action,
		OldStatus:     currentStatus,
		NewStatus:     newStatus,
		ActorSubject:  claims.Subject,
		ActorIP:       ip,
		Motiv:         req.Motiv,
		Visibility:    VisibilityWorkflowOnly,
	}
	if req.CompartimentID != nil {
		event.ToCompartimentID = req.CompartimentID
	}
	if req.AssigneeSubject != "" {
		event.AssignedUserSubject = req.AssigneeSubject
	}

	if err := e.audit.LogEventTx(ctx, tx, event); err != nil {
		return nil, fmt.Errorf("log event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transition: %w", err)
	}

	return &event, nil
}
