package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// EmbedHandler handles embedding service requests
type EmbedHandler struct {
	*BaseHandler
}

// NewEmbedHandler creates a new embed handler
func NewEmbedHandler(ovmsHost string, ovmsPort int, endpoint string) *EmbedHandler {
	return &EmbedHandler{
		BaseHandler: NewBaseHandler("embed", ovmsHost, ovmsPort, endpoint),
	}
}

// EmbedRequest represents an embedding request
type EmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbedResponse represents an embedding response
type EmbedResponse struct {
	Object string      `json:"object"`
	Data   []EmbedData `json:"data"`
	Model  string      `json:"model"`
	Usage  EmbedUsage  `json:"usage"`
}

// EmbedData represents embedding data
type EmbedData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// EmbedUsage represents token usage for embeddings
type EmbedUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// Handle handles an embedding request
func (h *EmbedHandler) Handle(ctx context.Context, request []byte) ([]byte, error) {
	log.Printf("[embed] Handling embedding request")

	var embedReq EmbedRequest
	if err := h.ParseRequest(request, &embedReq); err != nil {
		h.LogError(err)
		return nil, err
	}

	// Send to OVMS (using endpoint from manifest)
	respBody, err := h.SendRequest(ctx, "POST", h.Endpoint, embedReq)
	if err != nil {
		h.LogError(err)
		return nil, err
	}

	// Parse OVMS response
	var embedResp EmbedResponse
	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		h.LogError(err)
		return nil, fmt.Errorf("failed to parse OVMS response: %w", err)
	}

	// Marshal response
	response, err := h.MarshalResponse(embedResp)
	if err != nil {
		h.LogError(err)
		return nil, err
	}

	h.LogInfo("Embedding request completed successfully")
	return response, nil
}

// HandleStream handles a streaming embedding request
func (h *EmbedHandler) HandleStream(ctx context.Context, request []byte) (<-chan []byte, error) {
	log.Printf("[embed] Handling streaming embedding request")

	ch := make(chan []byte, 10)

	go func() {
		defer close(ch)

		// For embeddings, streaming is not typically used
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
