// internal/users/service.go
package users

import (
	"context"
	"fmt"
	"time"

	"github.com/eguilde/egudoc/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// GetOrCreate upserts a user from JWT claims on each login.
func (s *Service) GetOrCreate(ctx context.Context, claims *auth.Claims, ip string) (*User, error) {
	now := time.Now()
	var u User
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (subject, email, last_login_at, last_login_ip)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (subject) DO UPDATE
		SET email = EXCLUDED.email,
		    last_login_at = EXCLUDED.last_login_at,
		    last_login_ip = EXCLUDED.last_login_ip,
		    updated_at = NOW()
		RETURNING id, subject, email, COALESCE(phone, ''), COALESCE(prenume, ''), COALESCE(nume, ''), COALESCE(avatar_url, ''), active, last_login_at, created_at, updated_at
	`, claims.Subject, claims.Email, now, ip).Scan(
		&u.ID, &u.Subject, &u.Email, &u.Phone, &u.Prenume, &u.Nume,
		&u.AvatarURL, &u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return &u, nil
}

// GetBySubject retrieves a user by their JWT subject claim.
func (s *Service) GetBySubject(ctx context.Context, subject string) (*User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT id, subject, email, COALESCE(phone, ''), COALESCE(prenume, ''), COALESCE(nume, ''), COALESCE(avatar_url, ''), active, last_login_at, created_at, updated_at
		FROM users WHERE subject = $1
	`, subject).Scan(
		&u.ID, &u.Subject, &u.Email, &u.Phone, &u.Prenume, &u.Nume,
		&u.AvatarURL, &u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by subject: %w", err)
	}
	return &u, nil
}

// GetByID retrieves a user by UUID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT id, subject, email, COALESCE(phone, ''), COALESCE(prenume, ''), COALESCE(nume, ''), COALESCE(avatar_url, ''), active, last_login_at, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&u.ID, &u.Subject, &u.Email, &u.Phone, &u.Prenume, &u.Nume,
		&u.AvatarURL, &u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}
