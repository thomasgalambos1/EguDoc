// internal/users/model.go
package users

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID  `json:"id"`
	Subject     string     `json:"subject"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone,omitempty"`
	Prenume     string     `json:"prenume,omitempty"`
	Nume        string     `json:"nume,omitempty"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
	Active      bool       `json:"active"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
