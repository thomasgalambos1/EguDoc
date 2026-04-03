// internal/workflow/model.go
package workflow

import (
	"time"

	"github.com/google/uuid"
)

type Action string

const (
	ActionCreate             Action = "CREATE"
	ActionAssignCompartiment Action = "ASSIGN_COMPARTIMENT"
	ActionAssignUser         Action = "ASSIGN_USER"
	ActionSendForApproval    Action = "SEND_FOR_APPROVAL"
	ActionApprove            Action = "APPROVE"
	ActionReject             Action = "REJECT"
	ActionFinalize           Action = "FINALIZE"
	ActionCancel             Action = "CANCEL"
	ActionAddComment         Action = "ADD_COMMENT"
)

type Visibility string

const (
	VisibilityWorkflowOnly Visibility = "WORKFLOW_ONLY"
	VisibilityDepartment   Visibility = "DEPARTMENT"
	VisibilityPublic       Visibility = "PUBLIC"
)

type WorkflowEvent struct {
	ID            uuid.UUID `json:"id"`
	DocumentID    uuid.UUID `json:"document_id"`
	InstitutionID uuid.UUID `json:"institution_id"`

	Action    Action `json:"action"`
	OldStatus string `json:"old_status,omitempty"`
	NewStatus string `json:"new_status"`

	ActorSubject        string         `json:"actor_subject"`
	ActorIP             string         `json:"actor_ip,omitempty"`
	FromCompartimentID  *uuid.UUID     `json:"from_compartiment_id,omitempty"`
	ToCompartimentID    *uuid.UUID     `json:"to_compartiment_id,omitempty"`
	AssignedUserSubject string         `json:"assigned_user_subject,omitempty"`
	Motiv               string         `json:"motiv,omitempty"`
	Metadata            map[string]any `json:"metadata,omitempty"`
	Visibility          Visibility     `json:"visibility"`
	CreatedAt           time.Time      `json:"created_at"`
}

// ActionRequest is the payload for any workflow action.
type ActionRequest struct {
	Action          Action     `json:"action"`
	CompartimentID  *uuid.UUID `json:"compartiment_id,omitempty"`
	AssigneeSubject string     `json:"assignee_subject,omitempty"`
	Motiv           string     `json:"motiv,omitempty"`
}

// ValidTransitions defines the allowed state machine transitions.
// Key: (currentStatus) → map of (action → newStatus)
var ValidTransitions = map[string]map[Action]string{
	"INREGISTRAT": {
		ActionAssignCompartiment: "ALOCAT_COMPARTIMENT",
		ActionCancel:             "ANULAT",
	},
	"ALOCAT_COMPARTIMENT": {
		ActionAssignUser: "IN_LUCRU",
		ActionCancel:     "ANULAT",
	},
	"IN_LUCRU": {
		ActionSendForApproval: "FLUX_APROBARE",
		ActionFinalize:        "FINALIZAT",
		ActionCancel:          "ANULAT",
	},
	"FLUX_APROBARE": {
		ActionApprove: "FINALIZAT",
		ActionReject:  "IN_LUCRU",
		ActionCancel:  "ANULAT",
	},
	"FINALIZAT": {},
	"ARHIVAT":   {}, // terminal state — documents reach this via background archive worker
	"ANULAT":    {},
}
