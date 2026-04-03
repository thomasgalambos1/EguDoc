// internal/rbac/middleware.go
package rbac

import (
	"net/http"

	"github.com/eguilde/egudoc/internal/auth"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Require returns a Chi middleware that checks the authenticated user has `action` on `subject`.
// Must be used AFTER auth.RequireAuth middleware (claims must be in context).
func (s *Service) Require(action, subject string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.GetClaims(r.Context())
			if claims == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// Extract institution from header if present
			var institutionID *uuid.UUID
			if raw := r.Header.Get("X-Institution-ID"); raw != "" {
				id, err := uuid.Parse(raw)
				if err != nil {
					http.Error(w, `{"error":"invalid X-Institution-ID"}`, http.StatusBadRequest)
					return
				}
				institutionID = &id
			}

			check := CheckContext{
				UserSubject:   claims.Subject,
				InstitutionID: institutionID,
			}

			allowed, err := s.HasPermission(r.Context(), check, action, subject)
			if err != nil {
				s.log.Error("rbac permission check failed", zap.Error(err))
				http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
				return
			}
			if !allowed {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole returns a Chi middleware that checks the user has at least one of the given role codes.
// This is a coarser check than Require() — use for admin-only routes.
func (s *Service) RequireRole(roles ...string) func(http.Handler) http.Handler {
	roleSet := make(map[string]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.GetClaims(r.Context())
			if claims == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			for _, role := range claims.Roles {
				if roleSet[role] {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":"forbidden - insufficient role"}`, http.StatusForbidden)
		})
	}
}
