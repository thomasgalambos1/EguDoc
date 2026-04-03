// internal/auth/context.go
package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const claimsKey contextKey = "claims"

// RequireAuth is a Chi middleware that validates the Bearer token and injects Claims into context.
func RequireAuth(cache *JWKSCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSON(w, http.StatusUnauthorized, `{"error":"missing authorization header"}`)
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				writeJSON(w, http.StatusUnauthorized, `{"error":"invalid authorization format"}`)
				return
			}

			claims, err := ParseToken(r.Context(), parts[1], cache)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, `{"error":"invalid or expired token"}`)
				return
			}

			next.ServeHTTP(w, r.WithContext(ContextWithClaims(r.Context(), claims)))
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(body))
}

func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// GetClaims returns the authenticated claims from the request context.
func GetClaims(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

// GetUserSubject returns the JWT subject from context.
func GetUserSubject(ctx context.Context) string {
	if c := GetClaims(ctx); c != nil {
		return c.Subject
	}
	return ""
}
