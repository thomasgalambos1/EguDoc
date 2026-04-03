// internal/rbac/model.go
package rbac

import (
	"time"

	"github.com/google/uuid"
)

type Role struct {
	ID          uuid.UUID `json:"id"`
	Code        string    `json:"code"`
	Label       string    `json:"label"`
	Description string    `json:"description,omitempty"`
	System      bool      `json:"system"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Permission struct {
	ID          uuid.UUID      `json:"id"`
	Action      string         `json:"action"`             // create, read, update, delete, approve, assign, archive, deliver
	Subject     string         `json:"subject"`            // document, workflow, registry, user, role, institution, report
	Condition   map[string]any `json:"condition,omitempty"` // nil = no restriction
	Description string         `json:"description,omitempty"`
}

type UserRole struct {
	ID             uuid.UUID  `json:"id"`
	UserSubject    string     `json:"user_subject"`
	RoleID         uuid.UUID  `json:"role_id"`
	InstitutionID  *uuid.UUID `json:"institution_id,omitempty"`
	CompartimentID *uuid.UUID `json:"compartiment_id,omitempty"`
	GrantedBy      string     `json:"granted_by,omitempty"`
	Active         bool       `json:"active"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// CheckContext carries the subject's context for condition evaluation
type CheckContext struct {
	UserSubject    string
	InstitutionID  *uuid.UUID
	CompartimentID *uuid.UUID
	ResourceID     *uuid.UUID // the specific resource being accessed
}

// DefaultRoles defines the system roles that are always seeded.
// Code must match what eguilde issues in token.roles claims.
var DefaultRoles = []Role{
	{Code: "superadmin", Label: "Super Administrator", System: true},
	{Code: "institution_admin", Label: "Administrator Instituție", System: true},
	{Code: "registrar", Label: "Registrator", System: true},
	{Code: "department_head", Label: "Șef Compartiment", System: true},
	{Code: "department_staff", Label: "Personal Compartiment", System: true},
	{Code: "approver", Label: "Aprobator", System: true},
	{Code: "archiver", Label: "Arhivar", System: true},
	{Code: "citizen", Label: "Cetățean", System: true},
	{Code: "external_entity", Label: "Entitate Externă", System: true},
}
