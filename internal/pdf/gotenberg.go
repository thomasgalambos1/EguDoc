package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"
)

type Gotenberg struct {
	baseURL    string
	httpClient *http.Client
}

func NewGotenberg(baseURL string) *Gotenberg {
	return &Gotenberg{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ConvertToPDFA converts any supported document to PDF/A-2b.
func (g *Gotenberg) ConvertToPDFA(ctx context.Context, filename string, content io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	part, err := mw.CreateFormFile("files", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, content); err != nil {
		return nil, fmt.Errorf("copy file content: %w", err)
	}
	if err := mw.WriteField("pdfa", "PDF/A-2b"); err != nil {
		return nil, fmt.Errorf("write pdfa field: %w", err)
	}
	mw.Close()

	endpoint := g.endpointForFile(filename)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+endpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gotenberg conversion failed (status %d): %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// ConvertHTMLToPDFA converts an HTML string to PDF/A-2b.
func (g *Gotenberg) ConvertHTMLToPDFA(ctx context.Context, htmlContent string, title string) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	part, err := mw.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, fmt.Errorf("create html part: %w", err)
	}
	if _, err := io.WriteString(part, htmlContent); err != nil {
		return nil, fmt.Errorf("write html content: %w", err)
	}
	if err := mw.WriteField("pdfa", "PDF/A-2b"); err != nil {
		return nil, fmt.Errorf("write pdfa field: %w", err)
	}
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/forms/chromium/convert/html", &buf)
	if err != nil {
		return nil, fmt.Errorf("create html request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg html request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("html to pdfa failed: %s", string(body))
	}

	return io.ReadAll(resp.Body)
}

func (g *Gotenberg) endpointForFile(filename string) string {
	switch filepath.Ext(filename) {
	case ".html", ".htm":
		return "/forms/chromium/convert/html"
	default:
		return "/forms/libreoffice/convert"
	}
}
