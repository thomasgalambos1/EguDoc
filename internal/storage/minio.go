// internal/storage/minio.go
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	mc     *minio.Client
	bucket string
}

func NewClient(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	return &Client{mc: mc, bucket: bucket}, nil
}

// EnsureBucket creates the bucket if it doesn't exist.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if exists {
		return nil
	}
	return c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
}

// PutResult contains the storage key and SHA-256 of uploaded content.
type PutResult struct {
	StorageKey string
	SHA256     string
	SizeBytes  int64
}

// PutDocument stores a document file under a deterministic key.
// entityType: "document", "attachment", etc.
func (c *Client) PutDocument(ctx context.Context, entityType string, filename string, content io.Reader, contentType string, size int64) (*PutResult, error) {
	// Build key: entityType/YYYY/MM/uuid/filename
	now := time.Now()
	objectKey := path.Join(
		entityType,
		fmt.Sprintf("%d/%02d", now.Year(), now.Month()),
		uuid.New().String(),
		filename,
	)

	// Tee the reader to compute SHA-256 while uploading
	pr, pw := io.Pipe()
	hasher := sha256.New()
	tr := io.TeeReader(content, hasher)

	go func() {
		if _, err := io.Copy(pw, tr); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()

	info, err := c.mc.PutObject(ctx, c.bucket, objectKey, pr, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("put object: %w", err)
	}

	return &PutResult{
		StorageKey: objectKey,
		SHA256:     hex.EncodeToString(hasher.Sum(nil)),
		SizeBytes:  info.Size,
	}, nil
}

// GetDocument returns a reader for a stored document.
func (c *Client) GetDocument(ctx context.Context, storageKey string) (io.ReadCloser, *minio.ObjectInfo, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, storageKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("get object: %w", err)
	}
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, nil, fmt.Errorf("stat object: %w", err)
	}
	return obj, &info, nil
}

// PresignedURL generates a time-limited pre-signed GET URL.
func (c *Client) PresignedURL(ctx context.Context, storageKey string, expiry time.Duration, filename string) (string, error) {
	params := url.Values{}
	if filename != "" {
		params.Set("response-content-disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	}
	u, err := c.mc.PresignedGetObject(ctx, c.bucket, storageKey, expiry, params)
	if err != nil {
		return "", fmt.Errorf("presign: %w", err)
	}
	return u.String(), nil
}

// DeleteDocument removes a document from storage.
func (c *Client) DeleteDocument(ctx context.Context, storageKey string) error {
	return c.mc.RemoveObject(ctx, c.bucket, storageKey, minio.RemoveObjectOptions{})
}
