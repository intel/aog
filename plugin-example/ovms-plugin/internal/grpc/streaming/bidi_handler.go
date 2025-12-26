//*****************************************************************************
// Copyright 2024-2025 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//*****************************************************************************

package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/intel/aog/plugin-sdk/client"
	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/grpc/grpc_client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCBidiStreamHandler handles bidirectional gRPC streaming for speech-to-text-ws
type GRPCBidiStreamHandler struct {
	host      string
	port      int
	logger    LoggerAdapter
	modelName string // Store model name from first audio message
}

// NewGRPCBidiStreamHandler creates a new gRPC bidirectional stream handler
func NewGRPCBidiStreamHandler(host string, port int, logger LoggerAdapter) *GRPCBidiStreamHandler {
	return &GRPCBidiStreamHandler{
		host:   host,
		port:   port,
		logger: logger,
	}
}

// LoggerAdapter interface for logging
type LoggerAdapter interface {
	LogInfo(msg string)
	LogDebug(msg string)
	LogError(msg string, err error)
}

// HandleBidirectional manages the bidirectional gRPC stream
func (h *GRPCBidiStreamHandler) HandleBidirectional(
	ctx context.Context,
	wsConnID string,
	inStream <-chan client.BidiMessage,
	outStream chan<- client.BidiMessage,
) error {
	// 1. Establish gRPC connection to OVMS
	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", h.host, h.port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to OVMS gRPC: %w", err)
	}
	defer conn.Close()

	// 2. Create gRPC client
	gClient := grpc_client.NewGRPCInferenceServiceClient(conn)

	// 3. Create bidirectional stream
	stream, err := gClient.ModelStreamInfer(ctx)
	if err != nil {
		return fmt.Errorf("failed to create gRPC bidirectional stream: %w", err)
	}

	// 4. Channel for coordinating goroutines
	errChan := make(chan error, 2)

	// 5. Start receiver goroutine (gRPC stream -> outStream)
	go func() {
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				errChan <- nil
				return
			}
			if err != nil {
				h.logger.LogError("Failed to receive from gRPC stream", err)
				// Send error to outStream
				select {
				case outStream <- client.BidiMessage{
					Error:       err,
					MessageType: "error",
					Metadata: map[string]string{
						"conn_id": wsConnID,
						"source":  "grpc_recv",
					},
				}:
				case <-ctx.Done():
				}
				errChan <- err
				return
			}

			// Convert gRPC response to BidiMessage
			bidiMsg, err := h.convertGRPCResponseToBidiMessage(resp, wsConnID)
			if err != nil {
				continue
			}

			// Send to outStream
			select {
			case outStream <- bidiMsg:
			case <-ctx.Done():
				h.logger.LogError("Context cancelled while sending to outStream", ctx.Err())
				errChan <- ctx.Err()
				return
			}
		}
	}()

	// 6. Start sender goroutine (inStream -> gRPC stream)
	go func() {
		for {
			select {
			case msg, ok := <-inStream:
				if !ok {
					// Send finish request to gRPC
					finishReq := h.prepareFinishRequest()
					if err := stream.Send(finishReq); err != nil {
						h.logger.LogError("Failed to send finish request", err)
					}
					// Close send side of stream
					if err := stream.CloseSend(); err != nil {
						h.logger.LogError("Failed to close send stream", err)
					}
					errChan <- nil
					return
				}

				// Handle error messages from inStream
				if msg.Error != nil {
					h.logger.LogError("Received error from inStream", msg.Error)
					errChan <- msg.Error
					return
				}

				// Process different message types
				switch msg.MessageType {
				case "binary", "audio":
					// Extract and save model name from first audio message
					if h.modelName == "" {
						if model, ok := msg.Metadata["model"]; ok && model != "" {
							h.modelName = model
						} else {
							// No model specified, return error
							err := fmt.Errorf("model name not specified in metadata")
							h.logger.LogError("Missing model name", err)
							select {
							case outStream <- client.BidiMessage{
								Error:       err,
								MessageType: "error",
								Metadata: map[string]string{
									"conn_id": wsConnID,
									"source":  "model_name",
								},
							}:
							case <-ctx.Done():
							}
							continue
						}
					}

					// Ensure conn_id is in metadata for prepareAudioRequest
					if msg.Metadata == nil {
						msg.Metadata = make(map[string]string)
					}
					msg.Metadata["conn_id"] = wsConnID

					// Audio data - convert to gRPC request
					grpcReq, err := h.prepareAudioRequest(msg.Data, msg.Metadata)
					if err != nil {
						h.logger.LogError("Failed to prepare audio request", err)
						select {
						case outStream <- client.BidiMessage{
							Error:       err,
							MessageType: "error",
							Metadata: map[string]string{
								"conn_id": wsConnID,
								"source":  "audio_prepare",
							},
						}:
						case <-ctx.Done():
						}
						continue
					}

					if err := stream.Send(grpcReq); err != nil {
						h.logger.LogError("Failed to send audio to gRPC stream", err)
						select {
						case outStream <- client.BidiMessage{
							Error:       err,
							MessageType: "error",
							Metadata: map[string]string{
								"conn_id": wsConnID,
								"source":  "grpc_send",
							},
						}:
						case <-ctx.Done():
						}
						errChan <- err
						return
					}

				case "text":
					// Control message
					action := h.parseAction(msg.Data)
					if action == "finish-task" || action == "finish" {
						finishReq := h.prepareFinishRequest()
						if err := stream.Send(finishReq); err != nil {
							h.logger.LogError("Failed to send finish request", err)
						}
						if err := stream.CloseSend(); err != nil {
							h.logger.LogError("Failed to close send stream", err)
						}
						errChan <- nil
						return
					}

				default:
				}

			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}
		}
	}()

	// 7. Wait for either goroutine to finish
	err = <-errChan
	if err != nil && err != io.EOF {
		h.logger.LogError("Bidirectional stream error", err)
		return err
	}

	return nil
}

// convertGRPCResponseToBidiMessage converts gRPC response to BidiMessage
func (h *GRPCBidiStreamHandler) convertGRPCResponseToBidiMessage(
	resp *grpc_client.ModelStreamInferResponse,
	wsConnID string,
) (client.BidiMessage, error) {
	// Check for error in response
	if resp.ErrorMessage != "" {
		err := fmt.Errorf("gRPC error: %s", resp.ErrorMessage)
		h.logger.LogError("gRPC response contains error", err)
		return client.BidiMessage{
			Error:       err,
			MessageType: "error",
		}, nil
	}

	// Parse gRPC response and convert to JSON
	if resp.InferResponse == nil {
		return client.BidiMessage{}, fmt.Errorf("empty infer response")
	}

	// Check if response has data
	if len(resp.InferResponse.RawOutputContents) == 0 {
		return client.BidiMessage{}, fmt.Errorf("empty output contents")
	}

	rawOutput := string(resp.InferResponse.RawOutputContents[0])

	// Parse JSON response from whisper.py (speech-to-text-ws returns JSON-wrapped SRT)
	var whisperResp struct {
		Status        string `json:"status"`
		Content       string `json:"content"`
		IsFinal       bool   `json:"is_final"`
		CurrentChunks int    `json:"current_chunks"`
		TotalChunks   int    `json:"total_chunks"`
	}

	if err := json.Unmarshal(resp.InferResponse.RawOutputContents[0], &whisperResp); err != nil {
		// Fallback: treat as raw SRT (for compatibility)
		whisperResp.Content = rawOutput
	}

	// Get complete SRT content (matching built-in OVMS behavior)
	srtText := whisperResp.Content

	// Skip if no content
	if strings.TrimSpace(srtText) == "" {
		return client.BidiMessage{}, fmt.Errorf("empty SRT content")
	}

	// Parse timestamps from complete SRT content (extract earliest start and latest end)
	// This matches the behavior of utils.ParseSRTTimestamps in built-in OVMS
	beginTime, endTime := parseSRTTimestamps(srtText)

	// Use complete SRT content as text (matching built-in OVMS)
	text := srtText

	// Build response message matching client expectations
	resultMsg := map[string]interface{}{
		"header": map[string]string{
			"task_id": wsConnID,
			"event":   "result-generated",
		},
		"payload": map[string]interface{}{
			"output": map[string]interface{}{
				"sentence": map[string]interface{}{
					"text":       text,
					"begin_time": beginTime,
					"end_time":   endTime,
				},
			},
		},
	}

	data, err := json.Marshal(resultMsg)
	if err != nil {
		h.logger.LogError("Failed to marshal result message", err)
		return client.BidiMessage{}, fmt.Errorf("failed to marshal result: %w", err)
	}

	return client.BidiMessage{
		Data:        data,
		MessageType: "text",
		Metadata: map[string]string{
			"service": "speech-to-text-ws",
			"conn_id": wsConnID,
			"event":   "result-generated",
		},
	}, nil
}

// prepareAudioRequest prepares gRPC request from audio data
func (h *GRPCBidiStreamHandler) prepareAudioRequest(
	audioData []byte,
	metadata map[string]string,
) (*grpc_client.ModelInferRequest, error) {
	// Use the model name saved from first audio message
	if h.modelName == "" {
		return nil, fmt.Errorf("model name not set")
	}

	// Build params JSON from metadata (matching built-in OVMS behavior)
	params := map[string]interface{}{
		"model":   h.modelName,
		"action":  "run-task", // WebSocket task action
		"task_id": metadata["conn_id"],
		"service": "speech-to-text-ws",
	}

	// Add optional parameters from metadata
	if language, ok := metadata["language"]; ok && language != "" {
		params["language"] = language
	}
	if format, ok := metadata["format"]; ok && format != "" {
		params["audio_format"] = format
	}
	if sampleRate, ok := metadata["sample_rate"]; ok && sampleRate != "" {
		params["sample_rate"] = sampleRate
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		h.logger.LogError("Failed to marshal params to JSON", err)
		paramsJSON = []byte("{}")
	}

	// Match built-in OVMS: two inputs (audio + params) and one output (result)
	req := &grpc_client.ModelInferRequest{
		ModelName: h.modelName,
		Inputs: []*grpc_client.ModelInferRequest_InferInputTensor{
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
		},
		Outputs: []*grpc_client.ModelInferRequest_InferRequestedOutputTensor{
			{
				Name: "result",
			},
		},
		RawInputContents: [][]byte{audioData, paramsJSON},
	}

	return req, nil
}

// prepareFinishRequest prepares finish request for gRPC stream
func (h *GRPCBidiStreamHandler) prepareFinishRequest() *grpc_client.ModelInferRequest {
	// Create finish request with empty inputs to signal end
	// Use the model name from the session
	modelName := h.modelName
	if modelName == "" {
		modelName = "whisper" // fallback if model name not set
	}
	return &grpc_client.ModelInferRequest{
		ModelName: modelName,
		Inputs:    []*grpc_client.ModelInferRequest_InferInputTensor{},
	}
}

// parseAction parses control message to extract action
func (h *GRPCBidiStreamHandler) parseAction(data []byte) string {
	var action struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(data, &action); err != nil {
		return ""
	}
	return action.Action
}

// parseSRTTimestamps extracts earliest begin time and latest end time from complete SRT content
// This matches the behavior of utils.ParseSRTTimestamps in built-in OVMS
func parseSRTTimestamps(srtContent string) (*int, *int) {
	if srtContent == "" {
		return nil, nil
	}

	// Split by lines
	lines := strings.Split(srtContent, "\n")

	var beginTime, endTime *int

	// Find timestamp lines (format: 00:00:00,000 --> 00:00:00,000)
	for _, line := range lines {
		if strings.Contains(line, " --> ") {
			parts := strings.Split(line, " --> ")
			if len(parts) == 2 {
				// Parse start time
				startMs := parseTimestamp(parts[0])
				if startMs >= 0 {
					if beginTime == nil || startMs < *beginTime {
						beginTime = &startMs
					}
				}

				// Parse end time
				endMs := parseTimestamp(parts[1])
				if endMs >= 0 {
					if endTime == nil || endMs > *endTime {
						endTime = &endMs
					}
				}
			}
		}
	}

	return beginTime, endTime
}

// parseTimestamp converts SRT timestamp format (HH:MM:SS,mmm) to milliseconds
func parseTimestamp(timestamp string) int {
	// Trim whitespace
	timestamp = strings.TrimSpace(timestamp)

	var hours, minutes, seconds, milliseconds int
	// Parse format: HH:MM:SS,mmm
	n, err := fmt.Sscanf(timestamp, "%d:%d:%d,%d", &hours, &minutes, &seconds, &milliseconds)
	if err != nil || n != 4 {
		return -1
	}

	return hours*3600000 + minutes*60000 + seconds*1000 + milliseconds
}
