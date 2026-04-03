// internal/auth/claims.go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
)

// jwksHTTPClient is a dedicated HTTP client with a timeout for JWKS fetches.
var jwksHTTPClient = &http.Client{Timeout: 10 * time.Second}

// Claims represents the JWT claims from the eguilde OIDC provider.
type Claims struct {
	Subject string   `json:"sub"`
	UID     int64    `json:"uid"`
	Email   string   `json:"email"`
	Roles   []string `json:"roles"`
	Exp     int64    `json:"exp"`
	Iat     int64    `json:"iat"`
}

// JWKSCache fetches and caches the OIDC provider's public keys.
type JWKSCache struct {
	mu        sync.RWMutex
	fetchMu   sync.Mutex
	keys      jose.JSONWebKeySet
	fetchedAt time.Time
	ttl       time.Duration
	jwksURL   string
}

func NewJWKSCache(jwksURL string, ttl time.Duration) *JWKSCache {
	return &JWKSCache{jwksURL: jwksURL, ttl: ttl}
}

func (c *JWKSCache) GetKeys(ctx context.Context) (jose.JSONWebKeySet, error) {
	c.mu.RLock()
	if time.Since(c.fetchedAt) < c.ttl {
		keys := c.keys
		c.mu.RUnlock()
		return keys, nil
	}

	// Release read lock, acquire fetch serialization lock
	c.mu.RUnlock()
	c.fetchMu.Lock()
	defer c.fetchMu.Unlock()

	// Re-check under read lock now that fetch lock is held
	c.mu.RLock()
	if time.Since(c.fetchedAt) < c.ttl {
		keys := c.keys
		c.mu.RUnlock()
		return keys, nil
	}
	c.mu.RUnlock()

	// Fetch without holding the read/write lock
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURL, nil)
	if err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("build jwks request: %w", err)
	}
	resp, err := jwksHTTPClient.Do(req)
	if err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return jose.JSONWebKeySet{}, fmt.Errorf("jwks endpoint returned %d", resp.StatusCode)
	}

	var keySet jose.JSONWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&keySet); err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("decode jwks: %w", err)
	}

	// Only acquire write lock for the brief cache update
	c.mu.Lock()
	c.keys = keySet
	c.fetchedAt = time.Now()
	c.mu.Unlock()

	return keySet, nil
}

// ParseToken validates a JWT Bearer token against the JWKS.
func ParseToken(ctx context.Context, tokenString string, cache *JWKSCache) (*Claims, error) {
	tok, err := josejwt.ParseSigned(tokenString, []jose.SignatureAlgorithm{jose.RS256, jose.ES256})
	if err != nil {
		return nil, fmt.Errorf("parse jwt: %w", err)
	}

	keySet, err := cache.GetKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("get jwks: %w", err)
	}

	var claims Claims
	for _, key := range keySet.Keys {
		if err := tok.Claims(key, &claims); err == nil {
			if claims.Exp < time.Now().Add(-10*time.Second).Unix() {
				return nil, fmt.Errorf("token expired")
			}
			if claims.Iat > time.Now().Add(10*time.Second).Unix() {
				return nil, fmt.Errorf("token issued in the future")
			}
			return &claims, nil
		}
	}
	return nil, fmt.Errorf("no matching key found or invalid signature")
}
