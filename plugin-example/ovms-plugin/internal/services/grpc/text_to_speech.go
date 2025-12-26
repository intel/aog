package grpc

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/grpc/grpc_client"
)

// TextToSpeechHandler handles text-to-speech service requests via gRPC
type TextToSpeechHandler struct {
	*GRPCBaseHandler
}

// NewTextToSpeechHandler creates a new text-to-speech handler
func NewTextToSpeechHandler(ovmsHost string, ovmsPort int, endpoint string) *TextToSpeechHandler {
	return &TextToSpeechHandler{
		GRPCBaseHandler: NewGRPCBaseHandler("text-to-speech", ovmsHost, ovmsPort),
	}
}

// TextToSpeechRequest represents a text-to-speech request
type TextToSpeechRequest struct {
	Model    string  `json:"model"`
	Text     string  `json:"text"`
	Voice    string  `json:"voice,omitempty"`
	Speed    float32 `json:"speed,omitempty"`
	Language string  `json:"language,omitempty"`
}

// TextToSpeechResponse represents a text-to-speech response (aligned with built-in OpenVINO)
type TextToSpeechResponse struct {
	URL string `json:"url"` // file path to generated audio
}

// Handle handles a text-to-speech request via gRPC (aligned with built-in OpenVINO)
func (h *TextToSpeechHandler) Handle(ctx context.Context, request []byte) ([]byte, error) {
	log.Printf("[text-to-speech] Handling text-to-speech request via gRPC")

	// 1. Parse request
	var ttsReq TextToSpeechRequest
	if err := json.Unmarshal(request, &ttsReq); err != nil {
		h.LogError("Failed to parse request", err)
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	// 2. Prepare gRPC request (aligned with built-in OpenVINO)
	modelName := ttsReq.Model
	if modelName == "" {
		modelName = "speecht5" // default model
	}

	// Voice parameter - aligned with built-in OpenVINO (separate input)
	voice := ttsReq.Voice
	if voice == "" {
		voice = "male" // default voice
	}

	// Prepare parameters (empty for now, aligned with built-in OpenVINO)
	params := []byte("{}")

	// Aligned with built-in OpenVINO: 3 inputs (text, voice, params)
	inputs := []*grpc_client.ModelInferRequest_InferInputTensor{
		{
			Name:     "text",
			Datatype: "BYTES",
			Shape:    []int64{1},
		},
		{
			Name:     "voice",
			Datatype: "BYTES",
			Shape:    []int64{1},
		},
		{
			Name:     "params",
			Datatype: "BYTES",
			Shape:    []int64{1},
		},
	}

	rawInputs := [][]byte{
		[]byte(ttsReq.Text),
		[]byte(voice),
		params,
	}

	outputs := []*grpc_client.ModelInferRequest_InferRequestedOutputTensor{
		{Name: "audio"},
	}

	// 3. Send gRPC request
	grpcResp, err := h.SendGRPCRequest(ctx, modelName, inputs, rawInputs, outputs)
	if err != nil {
		h.LogError("gRPC request failed", err)
		return nil, err
	}

	// 4. Parse gRPC response
	if len(grpcResp.RawOutputContents) == 0 {
		return nil, fmt.Errorf("empty response from OVMS")
	}

	audioData := grpcResp.RawOutputContents[0]

	// 5. Save audio to file (aligned with built-in OpenVINO)
	audioPath, err := saveAudioFile(audioData)
	if err != nil {
		h.LogError("Failed to save audio file", err)
		return nil, fmt.Errorf("failed to save audio file: %w", err)
	}

	// 6. Build response (aligned with built-in OpenVINO)
	ttsResp := TextToSpeechResponse{
		URL: audioPath,
	}

	response, err := json.Marshal(ttsResp)
	if err != nil {
		h.LogError("Failed to marshal response", err)
		return nil, err
	}

	h.LogInfo("Text-to-speech request completed successfully")
	return response, nil
}

// HandleStream handles a streaming text-to-speech request
func (h *TextToSpeechHandler) HandleStream(ctx context.Context, request []byte) (<-chan []byte, error) {
	log.Printf("[text-to-speech] Handling streaming text-to-speech request")

	ch := make(chan []byte, 10)

	go func() {
		defer close(ch)

		// For text-to-speech, streaming is not typically used
		// But we'll implement it for consistency by calling Handle
		response, err := h.Handle(ctx, request)
		if err != nil {
			h.LogError("HandleStream failed", err)
			return
		}

		ch <- response
	}()

	return ch, nil
}

// saveAudioFile saves audio data to a WAV file (aligned with built-in OpenVINO)
func saveAudioFile(audioData []byte) (string, error) {
	// Get user's Downloads directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	downloadDir := filepath.Join(homeDir, "Downloads")

	// Create Downloads directory if it doesn't exist
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create download directory: %w", err)
	}

	// Generate unique filename with timestamp
	now := time.Now()

	// Generate cryptographically secure random number
	var randBytes [2]byte
	if _, err := rand.Read(randBytes[:]); err != nil {
		return "", fmt.Errorf("failed to generate random number: %w", err)
	}
	randNum := binary.BigEndian.Uint16(randBytes[:]) % 10000

	audioName := fmt.Sprintf("%d%02d%02d%02d%02d%02d%04d.wav",
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second(), randNum)
	audioPath := filepath.Join(downloadDir, audioName)

	// Write audio data to file
	if err := os.WriteFile(audioPath, audioData, 0o644); err != nil {
		return "", fmt.Errorf("failed to write audio file: %w", err)
	}

	return audioPath, nil
}
