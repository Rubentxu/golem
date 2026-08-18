package memstore

import (
	"context"
	"fmt"

	"github.com/Rubentxu/golem/internal/ports"
)

// EmbeddingStore implements ports.EmbeddingProvider with deterministic replay.
// Vectors are indexed by sha256(text) for deterministic replay.
type EmbeddingStore struct {
	vectors map[string][]float64
	model   string
}

// NewEmbeddingStore creates a new EmbeddingStore with the given vectors.
func NewEmbeddingStore(vectors map[string][]float64) *EmbeddingStore {
	return &EmbeddingStore{vectors: vectors, model: "memstore-embedding"}
}

// NewEmbeddingStoreFromMap creates a new EmbeddingStore from a text-to-vector map.
func NewEmbeddingStoreFromMap(vectors map[string][]float64) *EmbeddingStore {
	return &EmbeddingStore{vectors: vectors, model: "memstore-embedding"}
}

// Embeddings returns the embedding for the given text.
func (s *EmbeddingStore) Embeddings(ctx context.Context, req ports.EmbeddingRequest) (ports.EmbeddingResponse, error) {
	if req.Text == "" {
		return ports.EmbeddingResponse{}, ports.ErrInvalidLLMRequest
	}
	hash := hashRequest(req.Text)
	vec, ok := s.vectors[hash]
	if !ok {
		return ports.EmbeddingResponse{}, ports.ErrProviderUnavailable
	}
	return ports.EmbeddingResponse{
		TenantID:  req.TenantID,
		Embedding: vec,
		Model:     s.model,
		Provider:  "memstore",
	}, nil
}

// AddEmbedding adds a text-vector pair to the store.
func (s *EmbeddingStore) AddEmbedding(text string, vector []float64) {
	hash := hashRequest(text)
	s.vectors[hash] = vector
}

// EmbeddingFor returns the embedding for a given text.
func (s *EmbeddingStore) EmbeddingFor(text string) ([]float64, error) {
	hash := hashRequest(text)
	vec, ok := s.vectors[hash]
	if !ok {
		return nil, fmt.Errorf("memstore: no embedding for text")
	}
	return vec, nil
}
