package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/eguilde/egudoc/internal/qtsp"
)

type Client struct {
	base *qtsp.Client
}

func NewClient(base *qtsp.Client) *Client {
	return &Client{base: base}
}

func (c *Client) Submit(ctx context.Context, req SubmitMessageRequest, content io.Reader, filename string) (*SubmitMessageResponse, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	metaPart, err := mw.CreateFormField("metadata")
	if err != nil {
		return nil, fmt.Errorf("create metadata part: %w", err)
	}
	if err := json.NewEncoder(metaPart).Encode(req); err != nil {
		return nil, fmt.Errorf("encode metadata: %w", err)
	}

	filePart, err := mw.CreateFormFile("document", filename)
	if err != nil {
		return nil, fmt.Errorf("create file part: %w", err)
	}
	if _, err := io.Copy(filePart, content); err != nil {
		return nil, fmt.Errorf("copy document content: %w", err)
	}
	mw.Close()

	resp, err := c.base.DoMultipart(ctx, "/delivery/submit", &buf, mw.FormDataContentType())
	if err != nil {
		return nil, fmt.Errorf("submit delivery: %w", err)
	}

	var result SubmitMessageResponse
	if err := qtsp.DecodeJSON(resp, &result); err != nil {
		return nil, fmt.Errorf("decode submit response: %w", err)
	}
	return &result, nil
}

func (c *Client) GetMessage(ctx context.Context, messageID string) (*MessageInfo, error) {
	resp, err := c.base.Do(ctx, http.MethodGet, "/delivery/messages/"+messageID, nil)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	var info MessageInfo
	if err := qtsp.DecodeJSON(resp, &info); err != nil {
		return nil, fmt.Errorf("decode message: %w", err)
	}
	return &info, nil
}

func (c *Client) GetEvidence(ctx context.Context, messageID string) ([]Evidence, error) {
	resp, err := c.base.Do(ctx, http.MethodGet, "/delivery/messages/"+messageID+"/evidence", nil)
	if err != nil {
		return nil, fmt.Errorf("get evidence: %w", err)
	}
	var evidence []Evidence
	if err := qtsp.DecodeJSON(resp, &evidence); err != nil {
		return nil, fmt.Errorf("decode evidence: %w", err)
	}
	return evidence, nil
}
