package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

type GCSClient struct {
	client     *storage.Client
	bucketName string
}

type UploadResult struct {
	ObjectName string
	PublicURL  string
	Size       int64
}

func NewGCSClient(bucketName, credentialsPath string) (*GCSClient, error) {
	ctx := context.Background()
	
	var client *storage.Client
	var err error
	
	if credentialsPath != "" {
		client, err = storage.NewClient(ctx, option.WithCredentialsFile(credentialsPath))
	} else {
		client, err = storage.NewClient(ctx)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &GCSClient{
		client:     client,
		bucketName: bucketName,
	}, nil
}

func (g *GCSClient) UploadFile(ctx context.Context, reader io.Reader, objectName string, contentType string) (*UploadResult, error) {
	bucket := g.client.Bucket(g.bucketName)
	obj := bucket.Object(objectName)

	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType
	writer.CacheControl = "public, max-age=86400"

	size, err := io.Copy(writer, reader)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("failed to write to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	return &UploadResult{
		ObjectName: objectName,
		PublicURL:  "", // Don't store public URL for private bucket
		Size:       size,
	}, nil
}

func (g *GCSClient) DeleteFile(ctx context.Context, objectName string) error {
	bucket := g.client.Bucket(g.bucketName)
	obj := bucket.Object(objectName)

	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object from GCS: %w", err)
	}

	return nil
}

func (g *GCSClient) GetSignedURL(objectName string, expiry time.Duration) (string, error) {
	bucket := g.client.Bucket(g.bucketName)
	
	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  "GET",
		Expires: time.Now().Add(expiry),
	}

	url, err := bucket.SignedURL(objectName, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return url, nil
}

func (g *GCSClient) Close() error {
	return g.client.Close()
}

func GenerateObjectName(templateID, originalFilename string) string {
	ext := filepath.Ext(originalFilename)
	timestamp := time.Now().Unix()
	return fmt.Sprintf("templates/%s/%d%s", templateID, timestamp, ext)
}