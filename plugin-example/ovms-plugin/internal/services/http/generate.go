package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// GenerateHandler handles text generation service requests
type GenerateHandler struct {
	*BaseHandler
}

// NewGenerateHandler creates a new generate handler
func NewGenerateHandler(ovmsHost string, ovmsPort int, endpoint string) *GenerateHandler {
	return &GenerateHandler{
		BaseHandler: NewBaseHandler("generate", ovmsHost, ovmsPort, endpoint),
	}
}

// GenerateRequest represents a generation request
type GenerateRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Temperature float32 `json:"temperature,omitempty"`
	TopP        float32 `json:"top_p,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
}

// GenerateResponse represents a generation response
type GenerateResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []GenerateChoice `json:"choices"`
	Usage   GenerateUsage    `json:"usage"`
}

// GenerateChoice represents a generation choice
type GenerateChoice struct {
	Index        int    `json:"index"`
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason"`
}

// GenerateUsage represents token usage for generation
type GenerateUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Handle handles a generation request
func (h *GenerateHandler) Handle(ctx context.Context, request []byte) ([]byte, error) {
	log.Printf("[generate] Handling generation request")

	var genReq GenerateRequest
	if err := h.ParseRequest(request, &genReq); err != nil {
		h.LogError(err)
		return nil, err
	}

	// Send to OVMS (using endpoint from manifest)
	respBody, err := h.SendRequest(ctx, "POST", h.Endpoint, genReq)
	if err != nil {
		h.LogError(err)
		return nil, err
	}

	// Parse OVMS response
	var genResp GenerateResponse
	if err := json.Unmarshal(respBody, &genResp); err != nil {
		h.LogError(err)
		return nil, fmt.Errorf("failed to parse OVMS response: %w", err)
	}

	// Marshal response
	response, err := h.MarshalResponse(genResp)
	if err != nil {
		h.LogError(err)
		return nil, err
	}

	h.LogInfo("Generation request completed successfully")
	return response, nil
}

// HandleStream handles a streaming generation request with real-time SSE streaming
func (h *GenerateHandler) HandleStream(ctx context.Context, request []byte) (<-chan []byte, error) {
	log.Printf("[generate] Handling streaming generation request")

	ch := make(chan []byte, 10)

	go func() {
		defer close(ch)

		var genReq GenerateRequest
		if err := json.Unmarshal(request, &genReq); err != nil {
			log.Printf("[generate] Failed to parse request: %v", err)
			return
		}

		// Enable streaming
		genReq.Stream = true

		// Send streaming request to OVMS (using endpoint from manifest)
		dataChan, errChan := h.SendStreamRequest(ctx, "POST", h.Endpoint, genReq)

		// Process streaming response
		for {
			select {
			case data, ok := <-dataChan:
				if !ok {
					// Stream ended normally
					log.Printf("[generate] Stream completed")
					return
				}

				// Convert to SSE format: "data: {json}\n\n"
				sseData := fmt.Sprintf("data: %s\n\n", string(data))
				ch <- []byte(sseData)

			case err := <-errChan:
				if err != nil {
					log.Printf("[generate] Streaming error: %v", err)
				}
				return

			case <-ctx.Done():
				log.Printf("[generate] Context cancelled")
				return
			}
		}
	}()

	return ch, nil
}
