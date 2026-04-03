// internal/rbac/seeder.go
package rbac

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedDefaultRolesAndPermissions inserts the canonical roles and permissions on startup.
// Uses INSERT ... ON CONFLICT DO NOTHING so it is idempotent.
func SeedDefaultRolesAndPermissions(ctx context.Context, db *pgxpool.Pool) error {
	// Seed roles
	for _, role := range DefaultRoles {
		_, err := db.Exec(ctx, `
			INSERT INTO roles (code, label, system)
			VALUES ($1, $2, $3)
			ON CONFLICT (code) DO UPDATE SET label = EXCLUDED.label
		`, role.Code, role.Label, role.System)
		if err != nil {
			return fmt.Errorf("seed role %q: %w", role.Code, err)
		}
	}

	// Define all permissions
	type perm struct{ action, subject, desc string }
	perms := []perm{
		// Document permissions
		{"create", "document", "Create new documents in registry"},
		{"read", "document", "Read document details"},
		{"update", "document", "Update document fields"},
		{"delete", "document", "Delete/cancel a document"},
		{"archive", "document", "Submit document to qualified archive"},
		{"deliver", "document", "Send document via eDelivery"},
		// Workflow permissions
		{"assign", "workflow", "Assign documents to compartiments or users"},
		{"approve", "workflow", "Approve documents in approval chain"},
		{"reject", "workflow", "Reject documents in approval chain"},
		{"read", "workflow", "View workflow state and audit trail"},
		// Registry permissions
		{"create", "registry", "Create registries"},
		{"read", "registry", "View registries"},
		{"update", "registry", "Modify registry configuration"},
		// User/role management
		{"create", "user", "Create user accounts"},
		{"read", "user", "View user accounts"},
		{"update", "user", "Modify user accounts"},
		{"assign", "role", "Assign roles to users"},
		{"revoke", "role", "Revoke roles from users"},
		// Institution management
		{"create", "institution", "Create institutions"},
		{"read", "institution", "View institutions"},
		{"update", "institution", "Modify institution data"},
		// Reports
		{"read", "report", "Access reports and statistics"},
		{"export", "report", "Export reports"},
		// Archive management
		{"read", "archive", "View archive records"},
		{"manage", "archive", "Manage archive configuration"},
	}

	// Seed permissions
	for _, p := range perms {
		_, err := db.Exec(ctx, `
			INSERT INTO permissions (action, subject, description)
			VALUES ($1, $2, $3)
			ON CONFLICT ON CONSTRAINT uq_permissions_action_subject DO NOTHING
		`, p.action, p.subject, p.desc)
		if err != nil {
			return fmt.Errorf("seed permission %s:%s: %w", p.action, p.subject, err)
		}
	}

	// Assign all permissions to superadmin
	_, err := db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
		WHERE r.code = 'superadmin'
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("seed superadmin permissions: %w", err)
	}

	// Assign registrar permissions
	_, err = db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.code = 'registrar'
		  AND ((p.action = 'create' AND p.subject = 'document')
		    OR (p.action = 'read'   AND p.subject = 'document')
		    OR (p.action = 'update' AND p.subject = 'document')
		    OR (p.action = 'read'   AND p.subject = 'registry')
		    OR (p.action = 'read'   AND p.subject = 'workflow')
		    OR (p.action = 'assign' AND p.subject = 'workflow'))
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("seed registrar permissions: %w", err)
	}

	// Assign department_head permissions
	_, err = db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.code = 'department_head'
		  AND ((p.action IN ('read', 'update') AND p.subject = 'document')
		    OR (p.action IN ('assign', 'approve', 'reject', 'read') AND p.subject = 'workflow')
		    OR (p.action = 'read' AND p.subject = 'report'))
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("seed department_head permissions: %w", err)
	}

	// Assign department_staff permissions
	_, err = db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.code = 'department_staff'
		  AND ((p.action IN ('read', 'update') AND p.subject = 'document')
		    OR (p.action = 'read' AND p.subject = 'workflow'))
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("seed department_staff permissions: %w", err)
	}

	// Assign archiver permissions
	_, err = db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.code = 'archiver'
		  AND ((p.action IN ('read', 'archive') AND p.subject = 'document')
		    OR (p.action IN ('read', 'manage') AND p.subject = 'archive')
		    OR (p.action = 'read' AND p.subject = 'report'))
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("seed archiver permissions: %w", err)
	}
	return nil
}
