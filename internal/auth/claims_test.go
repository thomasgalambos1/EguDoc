// internal/auth/claims_test.go
package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eguilde/egudoc/internal/auth"
)

func TestJWKSCacheRefreshesOnExpiry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
	}))
	defer srv.Close()

	cache := auth.NewJWKSCache(srv.URL, 100*time.Millisecond)

	// First call — fetches
	cache.GetKeys(context.Background())
	// Second call within TTL — uses cache
	cache.GetKeys(context.Background())
	if calls != 1 {
		t.Errorf("expected 1 fetch, got %d", calls)
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Third call — should re-fetch
	cache.GetKeys(context.Background())
	if calls != 2 {
		t.Errorf("expected 2 fetches after expiry, got %d", calls)
	}
}
