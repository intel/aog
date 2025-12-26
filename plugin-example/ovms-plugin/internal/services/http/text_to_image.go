package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// TextToImageHandler handles text-to-image service requests
type TextToImageHandler struct {
	*BaseHandler
}

// NewTextToImageHandler creates a new text-to-image handler
func NewTextToImageHandler(ovmsHost string, ovmsPort int, endpoint string) *TextToImageHandler {
	return &TextToImageHandler{
		BaseHandler: NewBaseHandler("text-to-image", ovmsHost, ovmsPort, endpoint),
	}
}

// TextToImageRequest represents a text-to-image request
type TextToImageRequest struct {
	Model             string  `json:"model"`
	Prompt            string  `json:"prompt"`
	NegativePrompt    string  `json:"negative_prompt,omitempty"`
	Width             int     `json:"width,omitempty"`
	Height            int     `json:"height,omitempty"`
	NumInferenceSteps int     `json:"num_inference_steps,omitempty"`
	GuidanceScale     float32 `json:"guidance_scale,omitempty"`
}

// TextToImageResponse represents a text-to-image response
type TextToImageResponse struct {
	Images []string `json:"images"`
	Model  string   `json:"model"`
}

// Handle handles a text-to-image request
func (h *TextToImageHandler) Handle(ctx context.Context, request []byte) ([]byte, error) {
	log.Printf("[text-to-image] Handling text-to-image request")

	var t2iReq TextToImageRequest
	if err := h.ParseRequest(request, &t2iReq); err != nil {
		h.LogError(err)
		return nil, err
	}

	// Send to OVMS
	respBody, err := h.SendRequest(ctx, "POST", "/v1/text-to-image", t2iReq)
	if err != nil {
		h.LogError(err)
		return nil, err
	}

	// Parse OVMS response
	var t2iResp TextToImageResponse
	if err := json.Unmarshal(respBody, &t2iResp); err != nil {
		h.LogError(err)
		return nil, fmt.Errorf("failed to parse OVMS response: %w", err)
	}

	// Marshal response
	response, err := h.MarshalResponse(t2iResp)
	if err != nil {
		h.LogError(err)
		return nil, err
	}

	h.LogInfo("Text-to-image request completed successfully")
	return response, nil
}

// HandleStream handles a streaming text-to-image request
func (h *TextToImageHandler) HandleStream(ctx context.Context, request []byte) (<-chan []byte, error) {
	log.Printf("[text-to-image] Handling streaming text-to-image request")

	ch := make(chan []byte, 10)

	go func() {
		defer close(ch)

		// For text-to-image, streaming is not typically used
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
