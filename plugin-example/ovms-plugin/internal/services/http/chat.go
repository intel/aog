package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// ChatHandler handles chat service requests
type ChatHandler struct {
	*BaseHandler
}

// NewChatHandler creates a new chat handler
func NewChatHandler(ovmsHost string, ovmsPort int, endpoint string) *ChatHandler {
	return &ChatHandler{
		BaseHandler: NewBaseHandler("chat", ovmsHost, ovmsPort, endpoint),
	}
}

// ChatRequest represents a chat request
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float32       `json:"temperature,omitempty"`
	TopP        float32       `json:"top_p,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse represents a chat response
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   ChatUsage    `json:"usage"`
}

// ChatChoice represents a chat choice
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatUsage represents token usage
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Handle handles a chat request
func (h *ChatHandler) Handle(ctx context.Context, request []byte) ([]byte, error) {
	log.Printf("[chat] Handling chat request")

	var chatReq ChatRequest
	if err := h.ParseRequest(request, &chatReq); err != nil {
		h.LogError(err)
		return nil, err
	}

	// Send to OVMS (using endpoint from manifest)
	respBody, err := h.SendRequest(ctx, "POST", h.Endpoint, chatReq)
	if err != nil {
		h.LogError(err)
		return nil, err
	}

	// Parse OVMS response
	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		h.LogError(err)
		return nil, fmt.Errorf("failed to parse OVMS response: %w", err)
	}

	// Marshal response
	response, err := h.MarshalResponse(chatResp)
	if err != nil {
		h.LogError(err)
		return nil, err
	}

	h.LogInfo("Chat request completed successfully")
	return response, nil
}

// HandleStream handles a streaming chat request with real-time SSE streaming
func (h *ChatHandler) HandleStream(ctx context.Context, request []byte) (<-chan []byte, error) {
	log.Printf("[chat] Handling streaming chat request")

	ch := make(chan []byte, 10)

	go func() {
		defer close(ch)

		var chatReq ChatRequest
		if err := json.Unmarshal(request, &chatReq); err != nil {
			log.Printf("[chat] Failed to parse request: %v", err)
			return
		}

		// Enable streaming
		chatReq.Stream = true

		// Send streaming request to OVMS (using endpoint from manifest)
		dataChan, errChan := h.SendStreamRequest(ctx, "POST", h.Endpoint, chatReq)

		// Process streaming response
		for {
			select {
			case data, ok := <-dataChan:
				if !ok {
					// Stream ended normally
					log.Printf("[chat] Stream completed")
					return
				}

				// Convert to SSE format: "data: {json}\n\n"
				sseData := fmt.Sprintf("data: %s\n\n", string(data))
				ch <- []byte(sseData)

			case err := <-errChan:
				if err != nil {
					log.Printf("[chat] Streaming error: %v", err)
				}
				return

			case <-ctx.Done():
				log.Printf("[chat] Context cancelled")
				return
			}
		}
	}()

	return ch, nil
}
