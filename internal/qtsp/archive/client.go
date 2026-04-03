package archive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/eguilde/egudoc/internal/qtsp"
)

type Client struct {
	base *qtsp.Client
}

func NewClient(base *qtsp.Client) *Client {
	return &Client{base: base}
}

// Ingest submits a document to the qualified electronic archive.
func (c *Client) Ingest(ctx context.Context, title string, ownerID string, retentionYears int, content io.Reader, filename string, contentType string) (*IngestResponse, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	if err := mw.WriteField("title", title); err != nil {
		return nil, fmt.Errorf("write title field: %w", err)
	}
	if err := mw.WriteField("owner_id", ownerID); err != nil {
		return nil, fmt.Errorf("write owner_id field: %w", err)
	}
	if err := mw.WriteField("retention_years", strconv.Itoa(retentionYears)); err != nil {
		return nil, fmt.Errorf("write retention_years field: %w", err)
	}

	part, err := mw.CreateFormFile("document", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, content); err != nil {
		return nil, fmt.Errorf("copy content: %w", err)
	}
	mw.Close()

	resp, err := c.base.DoMultipart(ctx, "/archive/ingest", &buf, mw.FormDataContentType())
	if err != nil {
		return nil, fmt.Errorf("ingest document: %w", err)
	}

	var result IngestResponse
	if err := qtsp.DecodeJSON(resp, &result); err != nil {
		return nil, fmt.Errorf("decode ingest response: %w", err)
	}
	return &result, nil
}

func (c *Client) GetDocument(ctx context.Context, archiveID string) (*ArchiveDocument, error) {
	resp, err := c.base.Do(ctx, http.MethodGet, "/archive/documents/"+archiveID, nil)
	if err != nil {
		return nil, fmt.Errorf("get archive document: %w", err)
	}
	var doc ArchiveDocument
	if err := qtsp.DecodeJSON(resp, &doc); err != nil {
		return nil, fmt.Errorf("decode archive document: %w", err)
	}
	return &doc, nil
}

func (c *Client) VerifyIntegrity(ctx context.Context, archiveID string) (*VerifyResponse, error) {
	resp, err := c.base.Do(ctx, http.MethodGet, "/archive/documents/"+archiveID+"/verify", nil)
	if err != nil {
		return nil, fmt.Errorf("verify document: %w", err)
	}
	var result VerifyResponse
	if err := qtsp.DecodeJSON(resp, &result); err != nil {
		return nil, fmt.Errorf("decode verify response: %w", err)
	}
	return &result, nil
}

func (c *Client) GetCustodyProof(ctx context.Context, archiveID string) (*CustodyProof, error) {
	resp, err := c.base.Do(ctx, http.MethodGet, "/archive/documents/"+archiveID+"/custody-proof", nil)
	if err != nil {
		return nil, fmt.Errorf("get custody proof: %w", err)
	}
	var proof CustodyProof
	if err := qtsp.DecodeJSON(resp, &proof); err != nil {
		return nil, fmt.Errorf("decode custody proof: %w", err)
	}
	return &proof, nil
}
