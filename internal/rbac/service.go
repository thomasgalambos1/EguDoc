// internal/rbac/service.go
package rbac

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// HasPermission checks if a user (identified by subject) has the given action on the given subject.
// It checks all active roles for the user, optionally scoped by institution and compartiment.
func (s *Service) HasPermission(ctx context.Context, check CheckContext, action, subject string) (bool, error) {
	query := `
		SELECT COUNT(*) > 0
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ur.user_subject = $1
		  AND ur.active = TRUE
		  AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
		  AND p.action = $2
		  AND p.subject = $3
		  AND (
		        ur.institution_id IS NULL
		        OR ur.institution_id = $4
		      )
	`
	var allowed bool
	err := s.db.QueryRow(ctx, query,
		check.UserSubject,
		action,
		subject,
		check.InstitutionID,
	).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("check permission: %w", err)
	}
	return allowed, nil
}

// GetUserRoles returns all active role codes for a user.
func (s *Service) GetUserRoles(ctx context.Context, userSubject string, institutionID *uuid.UUID) ([]string, error) {
	query := `
		SELECT r.code
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_subject = $1
		  AND ur.active = TRUE
		  AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
		  AND (ur.institution_id IS NULL OR ur.institution_id = $2)
	`
	rows, err := s.db.Query(ctx, query, userSubject, institutionID)
	if err != nil {
		return nil, fmt.Errorf("get user roles: %w", err)
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

// AssignRole grants a role to a user (with optional institution/compartiment scoping).
func (s *Service) AssignRole(ctx context.Context, grantedBy string, assignment UserRole) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_roles (user_subject, role_id, institution_id, compartiment_id, granted_by, active, expires_at)
		VALUES ($1, $2, $3, $4, $5, TRUE, $6)
		ON CONFLICT (user_subject, role_id,
		             COALESCE(institution_id::text,''),
		             COALESCE(compartiment_id::text,''))
		DO UPDATE SET active = TRUE, expires_at = EXCLUDED.expires_at, granted_by = EXCLUDED.granted_by
	`,
		assignment.UserSubject,
		assignment.RoleID,
		assignment.InstitutionID,
		assignment.CompartimentID,
		grantedBy,
		assignment.ExpiresAt,
	)
	return err
}

// RevokeRole deactivates a role assignment.
func (s *Service) RevokeRole(ctx context.Context, userSubject string, roleID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		UPDATE user_roles SET active = FALSE
		WHERE user_subject = $1 AND role_id = $2
	`, userSubject, roleID)
	return err
}

// GetRoleByCode returns a role by its code.
func (s *Service) GetRoleByCode(ctx context.Context, code string) (*Role, error) {
	var r Role
	err := s.db.QueryRow(ctx, `
		SELECT id, code, label, description, system, created_at, updated_at
		FROM roles WHERE code = $1
	`, code).Scan(&r.ID, &r.Code, &r.Label, &r.Description, &r.System, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get role by code %q: %w", code, err)
	}
	return &r, nil
}
