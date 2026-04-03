package pdf_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eguilde/egudoc/internal/pdf"
)

func TestConvertHTMLToPDFACallsCorrectEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("%PDF-1.4 fake pdf content"))
	}))
	defer srv.Close()

	g := pdf.NewGotenberg(srv.URL)
	result, err := g.ConvertHTMLToPDFA(context.Background(), "<html><body>Test</body></html>", "Test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(string(result), "%PDF") {
		t.Errorf("expected PDF content, got: %s", string(result))
	}
	if gotPath != "/forms/chromium/convert/html" {
		t.Errorf("expected HTML endpoint, got: %s", gotPath)
	}
}
