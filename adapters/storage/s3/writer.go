// Package s3 provides an S3-backed storage adapter for canonical exports.
package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// minPartSize is the S3 minimum multipart upload part size (5MB per REQ-AUDIT-003).
const minPartSize = 5 * 1024 * 1024

// Writer uploads canonical export tar archives to S3 using multipart upload.
// Part size is 5MB minimum as required by S3. Retries failed parts with
// exponential backoff (3s, 6s, 12s) per REQ-AUDIT-003.
type Writer struct {
	client   *s3.Client
	uploader *manager.Uploader
	bucket   string
	prefix   string
}

// NewWriter creates a Writer for the given bucket and key prefix.
// The AWS config is loaded from the environment (IAM role, env vars, etc.).
func NewWriter(ctx context.Context, bucket, prefix string) (*Writer, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("s3 writer config: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = minPartSize
	})

	return &Writer{
		client:   client,
		uploader: uploader,
		bucket:   bucket,
		prefix:   prefix,
	}, nil
}

// Write uploads data to S3 at key with exponential-backoff retry.
// It returns the S3 object URL on success.
func (w *Writer) Write(ctx context.Context, key string, data io.Reader, size int64) (string, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * 3 * time.Second // 3, 6, 12s
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		result, err := w.uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket:        aws.String(w.bucket),
			Key:           aws.String(w.prefix + key),
			Body:          data,
			ContentLength: aws.Int64(size),
		})

		if err == nil {
			return result.Location, nil
		}
		lastErr = err
	}

	return "", fmt.Errorf("s3 upload after %d retries: %w", maxRetries, lastErr)
}

// WriteWithEncryption uploads data with SSE-KMS encryption using the provided
// KMS key ID (CMK). The key is referenced via internal/ports/kms.go (REQ-AUDIT-002).
func (w *Writer) WriteWithEncryption(ctx context.Context, key string, data io.Reader, size int64, kmsKeyID string) (string, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * 3 * time.Second
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		var buf bytes.Buffer
		tr := io.TeeReader(data, &buf)

		_, err := w.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:               aws.String(w.bucket),
			Key:                  aws.String(w.prefix + key),
			Body:                 tr,
			ContentLength:        aws.Int64(size),
			SSEKMSKeyId:          aws.String(kmsKeyID),
			ServerSideEncryption: types.ServerSideEncryptionAwsKms,
		})

		if err == nil {
			return fmt.Sprintf("s3://%s/%s", w.bucket, w.prefix+key), nil
		}
		lastErr = err
	}

	return "", fmt.Errorf("s3 kms upload after %d retries: %w", maxRetries, lastErr)
}

// UploadPartSize returns the minimum part size for multipart upload.
func (w *Writer) UploadPartSize() int64 {
	return minPartSize
}
