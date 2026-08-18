// Package kms provides a KMS-backed signing adapter for manifest verification.
package kms

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// Signer signs data using AWS KMS with the given key alias (e.g. alias/golem-export).
// It implements the signing port defined in internal/ports/kms.go.
type Signer struct {
	client   *kms.Client
	keyAlias string
}

// NewSigner creates a KMS signer for the given key alias.
func NewSigner(ctx context.Context, keyAlias string) (*Signer, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("kms signer config: %w", err)
	}
	return &Signer{
		client:   kms.NewFromConfig(cfg),
		keyAlias: keyAlias,
	}, nil
}

// Sign computes the HMAC-SHA256 of data using the KMS key identified by keyAlias.
// Returns the signature as a hex-encoded string.
func (s *Signer) Sign(ctx context.Context, data []byte) (string, error) {
	// Resolve alias to key ARN.
	keyID, err := s.resolveKeyID(ctx)
	if err != nil {
		return "", fmt.Errorf("kms sign: resolve key: %w", err)
	}

	sum := sha256.Sum256(data)
	input := &kms.SignInput{
		KeyId:            aws.String(keyID),
		Message:          sum[:],
		SigningAlgorithm: types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
	}

	result, err := s.client.Sign(ctx, input)
	if err != nil {
		return "", fmt.Errorf("kms sign: %w", err)
	}

	return hex.EncodeToString(result.Signature), nil
}

// Verify checks that sig matches the HMAC-SHA256 of data using the KMS key.
// Returns nil if valid, ErrInvalidSignature otherwise.
func (s *Signer) Verify(ctx context.Context, data []byte, sig string) error {
	expected, err := s.Sign(ctx, data)
	if err != nil {
		return err
	}
	if expected != sig {
		return ErrInvalidSignature
	}
	return nil
}

// resolveKeyID converts a key alias to the actual key ARN.
func (s *Signer) resolveKeyID(ctx context.Context) (string, error) {
	out, err := s.client.DescribeKey(ctx, &kms.DescribeKeyInput{
		KeyId: aws.String(s.keyAlias),
	})
	if err != nil {
		return "", fmt.Errorf("describe key %s: %w", s.keyAlias, err)
	}
	return aws.ToString(out.KeyMetadata.Arn), nil
}

// ErrInvalidSignature is returned when the signature verification fails.
var ErrInvalidSignature = fmt.Errorf("kms: invalid signature")

// KeyAlias returns the KMS key alias used by this signer.
func (s *Signer) KeyAlias() string {
	return s.keyAlias
}
