package grpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/grpc/grpc_client"
)

// SpeechToTextHandler handles speech-to-text service requests via gRPC
type SpeechToTextHandler struct {
	*GRPCBaseHandler
}

// NewSpeechToTextHandler creates a new speech-to-text handler
func NewSpeechToTextHandler(ovmsHost string, ovmsPort int, endpoint string) *SpeechToTextHandler {
	return &SpeechToTextHandler{
		GRPCBaseHandler: NewGRPCBaseHandler("speech-to-text", ovmsHost, ovmsPort),
	}
}

// SpeechToTextRequest represents a speech-to-text request
type SpeechToTextRequest struct {
	Model    string `json:"model"`
	Audio    string `json:"audio"` // base64 encoded audio
	Language string `json:"language,omitempty"`
}

// SpeechToTextParams represents parameters for speech-to-text (aligned with built-in OpenVINO)
type SpeechToTextParams struct {
	Service      string `json:"service"`
	Language     string `json:"language"`
	ReturnFormat string `json:"return_format"`
}

// SpeechToTextSegment represents a transcription segment with timestamps
type SpeechToTextSegment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// SpeechToTextResponse represents a speech-to-text response (aligned with built-in OpenVINO)
type SpeechToTextResponse struct {
	Segments []SpeechToTextSegment `json:"segments"`
	Model    string                `json:"model"`
}

// Handle handles a speech-to-text request via gRPC (aligned with built-in OpenVINO)
func (h *SpeechToTextHandler) Handle(ctx context.Context, request []byte) ([]byte, error) {
	log.Printf("[speech-to-text] Handling speech-to-text request via gRPC")

	// 1. Parse request
	var s2tReq SpeechToTextRequest
	if err := json.Unmarshal(request, &s2tReq); err != nil {
		h.LogError("Failed to parse request", err)
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	// 2. Decode base64 audio
	audioBytes, err := base64.StdEncoding.DecodeString(s2tReq.Audio)
	if err != nil {
		h.LogError("Failed to decode audio", err)
		return nil, fmt.Errorf("failed to decode audio: %w", err)
	}

	// 3. Prepare gRPC request (aligned with built-in OpenVINO)
	modelName := s2tReq.Model
	if modelName == "" {
		modelName = "whisper" // default model
	}

	// Prepare parameters - aligned with built-in OpenVINO format
	language := s2tReq.Language
	if language == "" {
		language = "zh"
	}
	// Format language as <|lang|> to match built-in OpenVINO
	languageFormatted := fmt.Sprintf("<|%s|>", language)

	params := SpeechToTextParams{
		Service:      "speech-to-text",
		Language:     languageFormatted,
		ReturnFormat: "text",
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		h.LogError("Failed to marshal params", err)
		paramsJSON = []byte("{}")
	}

	inputs := []*grpc_client.ModelInferRequest_InferInputTensor{
		{
			Name:     "audio",
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
		audioBytes,
		paramsJSON,
	}

	outputs := []*grpc_client.ModelInferRequest_InferRequestedOutputTensor{
		{Name: "result"},
	}

	// 4. Send gRPC request
	grpcResp, err := h.SendGRPCRequest(ctx, modelName, inputs, rawInputs, outputs)
	if err != nil {
		h.LogError("gRPC request failed", err)
		return nil, err
	}

	// 5. Parse gRPC response - aligned with built-in OpenVINO
	if len(grpcResp.RawOutputContents) == 0 {
		return nil, fmt.Errorf("empty response from OVMS")
	}

	// Parse SRT format: [start, end] Text
	srtText := string(grpcResp.RawOutputContents[0])
	segments := parseSRTFormat(srtText)

	// 6. Build response - aligned with built-in OpenVINO
	s2tResp := SpeechToTextResponse{
		Segments: segments,
		Model:    modelName,
	}

	response, err := json.Marshal(s2tResp)
	if err != nil {
		h.LogError("Failed to marshal response", err)
		return nil, err
	}

	h.LogInfo("Speech-to-text request completed successfully")
	return response, nil
}

// HandleStream handles a streaming speech-to-text request
func (h *SpeechToTextHandler) HandleStream(ctx context.Context, request []byte) (<-chan []byte, error) {
	log.Printf("[speech-to-text] Handling streaming speech-to-text request")

	ch := make(chan []byte, 10)

	go func() {
		defer close(ch)

		// For speech-to-text, streaming is not typically used
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

// parseSRTFormat parses SRT format text into segments (aligned with built-in OpenVINO)
// Format: [start, end] Text
func parseSRTFormat(srtText string) []SpeechToTextSegment {
	lines := strings.Split(strings.TrimSpace(srtText), "\n")
	segments := make([]SpeechToTextSegment, 0, len(lines))

	// Match format: [start time, end time] Text content
	timeRegex := regexp.MustCompile(`^\[(\d+\.\d+),\s*(\d+\.\d+)\]\s*(.+)$`)

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse timestamps and text
		matches := timeRegex.FindStringSubmatch(line)
		if len(matches) != 4 {
			continue
		}

		startTime, _ := strconv.ParseFloat(matches[1], 64)
		endTime, _ := strconv.ParseFloat(matches[2], 64)
		text := matches[3]

		segments = append(segments, SpeechToTextSegment{
			ID:    i,
			Start: startTime,
			End:   endTime,
			Text:  text,
		})
	}

	return segments
}
