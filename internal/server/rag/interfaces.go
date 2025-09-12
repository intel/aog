package rag

import (
	"context"
)

// EngineServiceInterface Defines the core interface of the engine service
type EngineServiceInterface interface {
	// Generate
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)

	// GenerateEmbedding
	GenerateEmbedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)
}

// EngineServiceProvider Defines the core interface of the engine service
type EngineServiceProvider interface {
	EngineServiceInterface
}
