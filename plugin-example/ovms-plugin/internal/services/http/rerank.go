package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// RerankHandler handles reranking service requests
type RerankHandler struct {
	*BaseHandler
}

// NewRerankHandler creates a new rerank handler
func NewRerankHandler(ovmsHost string, ovmsPort int, endpoint string) *RerankHandler {
	return &RerankHandler{
		BaseHandler: NewBaseHandler("rerank", ovmsHost, ovmsPort, endpoint),
	}
}

// RerankRequest represents a reranking request
type RerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopK      int      `json:"top_k,omitempty"`
}

// RerankResponse represents a reranking response
type RerankResponse struct {
	Results []RerankResult `json:"results"`
	Model   string         `json:"model"`
}

// RerankResult represents a reranking result
type RerankResult struct {
	Index    int     `json:"index"`
	Score    float32 `json:"score"`
	Document string  `json:"document"`
}

// Handle handles a reranking request
func (h *RerankHandler) Handle(ctx context.Context, request []byte) ([]byte, error) {
	log.Printf("[rerank] Handling reranking request")

	var rerankReq RerankRequest
	if err := h.ParseRequest(request, &rerankReq); err != nil {
		h.LogError(err)
		return nil, err
	}

	// Send to OVMS (using endpoint from manifest)
	respBody, err := h.SendRequest(ctx, "POST", h.Endpoint, rerankReq)
	if err != nil {
		h.LogError(err)
		return nil, err
	}

	// Parse OVMS response
	var rerankResp RerankResponse
	if err := json.Unmarshal(respBody, &rerankResp); err != nil {
		h.LogError(err)
		return nil, fmt.Errorf("failed to parse OVMS response: %w", err)
	}

	// Marshal response
	response, err := h.MarshalResponse(rerankResp)
	if err != nil {
		h.LogError(err)
		return nil, err
	}

	h.LogInfo("Reranking request completed successfully")
	return response, nil
}

// HandleStream handles a streaming reranking request
func (h *RerankHandler) HandleStream(ctx context.Context, request []byte) (<-chan []byte, error) {
	log.Printf("[rerank] Handling streaming reranking request")

	ch := make(chan []byte, 10)

	go func() {
		defer close(ch)

		// For reranking, streaming is not typically used
		// But we'll implement it for consistency
		response, err := h.Handle(ctx, request)
		if err != nil {
			h.LogError(err)
			return
		}

		ch <- response
	}()

	return ch, nil
}
