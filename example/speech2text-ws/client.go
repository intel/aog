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

// Package main demonstrates how to use AOG Speech-to-Text WebSocket service.
// This example shows how to connect to the AOG WebSocket endpoint, convert audio to PCM format,
// and send PCM data for real-time speech recognition results.
// IMPORTANT: AOG server only accepts PCM format - all input formats are converted to PCM.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hajimehoshi/go-mp3"
)

// Configuration constants
const (
	// WebSocket server URL for AOG Speech-to-Text service
	wsURL = "ws://localhost:16688/aog/v0.4/services/speech-to-text-ws"
	// Audio file path - replace with your audio file
	// IMPORTANT: Input formats PCM, WAV, and MP3 are supported for conversion
	// All formats are automatically converted to PCM before sending to AOG server
	// AOG server only accepts PCM format
	audioFile = "output.mp3"
	// Model to use for speech recognition
	defaultModel = "NamoLi/whisper-large-v3-ov"
	// Audio chunk size for streaming (100ms of 16kHz 16-bit mono PCM audio)
	audioChunkSize = 320000 // For PCM data: 10 * 16000 Hz * 0.1s * 1 bytes = 160000 bytes
	// Delay between audio chunks to simulate real-time streaming
	streamingDelay = 3000 * time.Millisecond // 3 seconds delay for demo purposes
	// Timeout for waiting task to start
	taskStartTimeout = 10 * time.Minute
)

// WebSocket dialer with default configuration
var dialer = websocket.DefaultDialer

// SpeechToTextClient represents a WebSocket client for speech-to-text service
type SpeechToTextClient struct {
	conn           *websocket.Conn
	taskID         string
	audioFormat    string // Detected input audio format (pcm/wav/mp3) - converted to PCM for server
	taskIDMutex    sync.Mutex
	taskIDReceived sync.WaitGroup
	taskStarted    chan bool
	taskDone       chan bool
}

// NewSpeechToTextClient creates a new speech-to-text client
func NewSpeechToTextClient() *SpeechToTextClient {
	client := &SpeechToTextClient{
		taskStarted: make(chan bool),
		taskDone:    make(chan bool),
	}
	client.taskIDReceived.Add(1)
	return client
}

func main() {
	fmt.Println("AOG Speech-to-Text WebSocket Client Demo")
	fmt.Println("========================================")

	// Create client instance
	client := NewSpeechToTextClient()

	// Connect to WebSocket service
	if err := client.connect(); err != nil {
		log.Fatal("Failed to connect to WebSocket:", err)
	}
	defer client.close()

	// Start message receiver in background
	client.startMessageReceiver()

	// Execute speech recognition workflow
	if err := client.runSpeechRecognition(); err != nil {
		log.Fatal("Speech recognition failed:", err)
	}

	fmt.Println("Speech recognition completed successfully!")
}

// Message structures for AOG Speech-to-Text WebSocket API

// Parameters represents the configuration parameters for speech recognition
type Parameters struct {
	Format       string `json:"format,omitempty"`        // Audio format: always "pcm" (server only accepts PCM)
	SampleRate   int    `json:"sample_rate,omitempty"`   // Sample rate, typically 16000
	Language     string `json:"language,omitempty"`      // Language code, e.g., "zh", "en"
	UseVAD       bool   `json:"use_vad,omitempty"`       // Whether to use Voice Activity Detection
	ReturnFormat string `json:"return_format,omitempty"` // Return format: "text", "json", "srt"
}

// ActionMessage represents a client-to-server message
type ActionMessage struct {
	Task       string      `json:"task"`                 // Task type, e.g., "speech-to-text-ws"
	Action     string      `json:"action"`               // Action type: "run-task" or "finish-task"
	TaskID     string      `json:"task_id,omitempty"`    // Task ID returned by server
	Model      string      `json:"model,omitempty"`      // Model name for recognition
	Parameters *Parameters `json:"parameters,omitempty"` // Optional parameters
}

// EventHeader represents the common header for all server events
type EventHeader struct {
	TaskID       string `json:"task_id"`                 // Task ID
	Event        string `json:"event"`                   // Event type
	ErrorCode    string `json:"error_code,omitempty"`    // Error code (only in task-failed events)
	ErrorMessage string `json:"error_message,omitempty"` // Error message (only in task-failed events)
}

// EventMessage represents a server-to-client message
type EventMessage struct {
	Header  EventHeader `json:"header"`
	Payload struct {
		Output struct {
			Sentence struct {
				BeginTime *int   `json:"begin_time"` // Start time in milliseconds
				EndTime   *int   `json:"end_time"`   // End time in milliseconds
				Text      string `json:"text"`       // Recognized text
			} `json:"sentence"`
		} `json:"output"`
	} `json:"payload"`
}

// RecognitionResult represents a simplified recognition result for easier handling
type RecognitionResult struct {
	TaskID    string `json:"task_id"`
	Text      string `json:"text"`
	BeginTime *int   `json:"begin_time,omitempty"`
	EndTime   *int   `json:"end_time,omitempty"`
}

// connect establishes a WebSocket connection to the AOG service
func (c *SpeechToTextClient) connect() error {
	header := make(http.Header)
	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		return fmt.Errorf("failed to dial WebSocket: %w", err)
	}
	c.conn = conn
	fmt.Printf("Connected to AOG Speech-to-Text service at %s\n", wsURL)
	return nil
}

// close closes the WebSocket connection
func (c *SpeechToTextClient) close() {
	if c.conn != nil {
		c.conn.Close()
		fmt.Println("WebSocket connection closed")
	}
}

// startMessageReceiver starts a goroutine to receive and handle WebSocket messages
func (c *SpeechToTextClient) startMessageReceiver() {
	go func() {
		for {
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				log.Printf("Failed to read WebSocket message: %v", err)
				return
			}

			// Parse the message as an event
			var event EventMessage
			if err := json.Unmarshal(message, &event); err != nil {
				log.Printf("Failed to parse event message: %v", err)
				continue
			}

			// Handle the event and check if we should stop
			if c.handleEvent(event, message) {
				return
			}
		}
	}()
}

// runSpeechRecognition executes the complete speech recognition workflow
func (c *SpeechToTextClient) runSpeechRecognition() error {
	// Step 1: Validate input audio file format and prepare for PCM conversion
	fmt.Printf("Validating input audio file: %s\n", audioFile)
	format, err := c.validateAudioFormat(audioFile)
	if err != nil {
		return fmt.Errorf("audio validation failed: %w", err)
	}
	c.audioFormat = format

	// Step 2: Send run-task command with PCM format (server only accepts PCM)
	if err := c.sendRunTaskCommand(); err != nil {
		return fmt.Errorf("failed to send run-task command: %w", err)
	}

	// Step 3: Wait for task to start
	if err := c.waitForTaskStarted(); err != nil {
		return fmt.Errorf("task failed to start: %w", err)
	}

	// Step 4: Wait for task ID
	fmt.Println("Waiting for task ID from server...")
	c.taskIDReceived.Wait()
	fmt.Printf("Received task ID: %s\n", c.taskID)

	// Step 5: Send audio data
	if err := c.sendAudioData(); err != nil {
		return fmt.Errorf("failed to send audio data: %w", err)
	}

	// Step 6: Send finish-task command
	if err := c.sendFinishTaskCommand(); err != nil {
		return fmt.Errorf("failed to send finish-task command: %w", err)
	}

	// Step 7: Wait for task completion
	<-c.taskDone
	return nil
}

// sendRunTaskCommand sends the run-task command to start speech recognition
func (c *SpeechToTextClient) sendRunTaskCommand() error {
	// Always use PCM format for server communication (server only accepts PCM)
	// Input files are automatically converted to PCM before transmission

	runTaskMsg := ActionMessage{
		Task:   "speech-to-text-ws",
		Action: "run-task",
		Model:  defaultModel,
		Parameters: &Parameters{
			Format:     "pcm", // Server only accepts PCM format
			SampleRate: 16000,
			Language:   "<|zh|>", // Chinese language
			UseVAD:     true,
		},
	}

	data, err := json.Marshal(runTaskMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal run-task message: %w", err)
	}

	fmt.Printf("Sending run-task command with PCM format: %s\n", string(data))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// waitForTaskStarted waits for the task-started event
func (c *SpeechToTextClient) waitForTaskStarted() error {
	select {
	case <-c.taskStarted:
		fmt.Println("Task started successfully")
		return nil
	case <-time.After(taskStartTimeout):
		return fmt.Errorf("timeout waiting for task to start")
	}
}

// validateAudioFormat checks if the input audio file format is supported for PCM conversion
func (c *SpeechToTextClient) validateAudioFormat(audioPath string) (string, error) {
	// Check if file exists first
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return "", fmt.Errorf("audio file not found: %s", audioPath)
	}

	// Open file to read header
	file, err := os.Open(audioPath)
	if err != nil {
		return "", fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	// Read first 12 bytes to detect format
	header := make([]byte, 12)
	n, err := file.Read(header)
	if err != nil {
		return "", fmt.Errorf("failed to read audio file header: %w", err)
	}
	if n < 4 {
		return "", fmt.Errorf("audio file too small or corrupted")
	}

	// Detect actual audio format by examining file signature
	format, err := c.detectAudioFormat(header, audioPath)
	if err != nil {
		return "", err
	}

	fmt.Printf("Input audio format validated: %s (will be converted to PCM for server)\n", format)
	return format, nil
}

// detectAudioFormat detects audio format from file header bytes
func (c *SpeechToTextClient) detectAudioFormat(header []byte, filePath string) (string, error) {
	// Check for WAV format (RIFF header)
	if len(header) >= 12 &&
		string(header[0:4]) == "RIFF" &&
		string(header[8:12]) == "WAVE" {

		// Additional validation for WAV format
		if err := c.validateWAVFormat(filePath); err != nil {
			return "", fmt.Errorf("WAV format validation failed: %w", err)
		}
		return "wav", nil
	}

	// Check for MP3 format (ID3 tag or MP3 frame sync)
	if len(header) >= 3 {
		// ID3v2 tag
		if string(header[0:3]) == "ID3" {
			if err := c.validateMP3Format(filePath); err != nil {
				return "", fmt.Errorf("MP3 format validation failed: %w", err)
			}
			return "mp3", nil
		}
		// MP3 frame sync (0xFF followed by 0xFB, 0xFA, 0xF3, 0xF2, etc.)
		if header[0] == 0xFF && (header[1]&0xE0) == 0xE0 {
			if err := c.validateMP3Format(filePath); err != nil {
				return "", fmt.Errorf("MP3 format validation failed: %w", err)
			}
			return "mp3", nil
		}
	}

	// For PCM files, check file extension as they don't have standard headers
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".pcm" {
		fmt.Printf("Warning: PCM format detected by extension. Ensure audio is 16kHz, 16-bit, mono PCM\n")
		return "pcm", nil
	}

	// If we can't detect the format, provide helpful error
	ext = strings.ToLower(filepath.Ext(filePath))
	return "", fmt.Errorf("unsupported or unrecognized audio format. File extension: %s. Supported input formats for PCM conversion: PCM (.pcm), WAV (RIFF/WAVE), MP3 (with ID3 or frame sync)", ext)
}

// validateWAVFormat performs additional validation for WAV files
func (c *SpeechToTextClient) validateWAVFormat(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read WAV header (44 bytes minimum)
	header := make([]byte, 44)
	n, err := file.Read(header)
	if err != nil {
		return fmt.Errorf("failed to read WAV header: %w", err)
	}
	if n < 44 {
		return fmt.Errorf("WAV file too small, incomplete header")
	}

	// Validate WAV structure
	if string(header[0:4]) != "RIFF" {
		return fmt.Errorf("invalid WAV file: missing RIFF header")
	}
	if string(header[8:12]) != "WAVE" {
		return fmt.Errorf("invalid WAV file: missing WAVE identifier")
	}
	if string(header[12:16]) != "fmt " {
		return fmt.Errorf("invalid WAV file: missing format chunk")
	}

	// Extract audio format information
	audioFormat := uint16(header[20]) | uint16(header[21])<<8
	channels := uint16(header[22]) | uint16(header[23])<<8
	sampleRate := uint32(header[24]) | uint32(header[25])<<8 | uint32(header[26])<<16 | uint32(header[27])<<24
	bitsPerSample := uint16(header[34]) | uint16(header[35])<<8

	// Validate audio format (1 = PCM)
	if audioFormat != 1 {
		return fmt.Errorf("unsupported WAV audio format: %d (only PCM format is supported)", audioFormat)
	}

	// Log audio specifications
	fmt.Printf("WAV file specifications: %d channels, %d Hz, %d bits per sample\n",
		channels, sampleRate, bitsPerSample)

	// Warn about non-optimal settings
	if sampleRate != 16000 {
		fmt.Printf("Warning: Sample rate is %d Hz. Recommended: 16000 Hz for optimal recognition\n", sampleRate)
	}
	if channels != 1 {
		fmt.Printf("Warning: Audio has %d channels. Mono (1 channel) is recommended\n", channels)
	}
	if bitsPerSample != 16 {
		fmt.Printf("Warning: Bit depth is %d bits. 16-bit is recommended\n", bitsPerSample)
	}

	return nil
}

// loadAudioFile loads and validates audio file, converting to PCM if needed
func (c *SpeechToTextClient) loadAudioFile(audioPath string) ([]byte, string, error) {
	// Validate audio format first
	format, err := c.validateAudioFormat(audioPath)
	if err != nil {
		return nil, "", err
	}

	var audioData []byte

	// Convert audio to PCM format based on detected format
	switch format {
	case "mp3":
		audioData, err = c.convertMP3ToPCM(audioPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to convert MP3 to PCM: %w", err)
		}
	case "wav":
		// For WAV files, we assume they are already in PCM format after validation
		audioData, err = c.loadWAVAsPCM(audioPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load WAV as PCM: %w", err)
		}
	case "pcm":
		// For PCM files, read directly
		audioData, err = c.loadPCMFile(audioPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load PCM file: %w", err)
		}
	default:
		return nil, "", fmt.Errorf("unsupported format for PCM conversion: %s", format)
	}

	fmt.Printf("Loaded and converted audio file: %d bytes PCM data (original format: %s)\n", len(audioData), format)
	return audioData, format, nil
}

// sendAudioData sends PCM audio data to the server in chunks
func (c *SpeechToTextClient) sendAudioData() error {
	fmt.Printf("Loading audio file: %s\n", audioFile)
	pcmData, _, err := c.loadAudioFile(audioFile)
	if err != nil {
		return err
	}

	fmt.Println("Streaming PCM audio data to server...")
	// Send PCM audio data in chunks to simulate real-time streaming
	for offset := 0; offset < len(pcmData); offset += audioChunkSize {
		end := offset + audioChunkSize
		if end > len(pcmData) {
			end = len(pcmData)
		}

		chunk := pcmData[offset:end]
		if err := c.conn.WriteMessage(websocket.BinaryMessage, chunk); err != nil {
			return fmt.Errorf("failed to send audio chunk: %w", err)
		}

		// Add delay to simulate real-time streaming
		time.Sleep(streamingDelay)
	}

	fmt.Println("Audio streaming completed")
	return nil
}

// sendFinishTaskCommand sends the finish-task command to complete the recognition
func (c *SpeechToTextClient) sendFinishTaskCommand() error {
	c.taskIDMutex.Lock()
	taskID := c.taskID
	c.taskIDMutex.Unlock()

	finishTaskMsg := ActionMessage{
		Task:   "speech-to-text-ws",
		Action: "finish-task",
		TaskID: taskID,
	}

	data, err := json.Marshal(finishTaskMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal finish-task message: %w", err)
	}

	fmt.Printf("Sending finish-task command: %s\n", string(data))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// handleEvent processes incoming WebSocket events and returns true if the connection should be closed
func (c *SpeechToTextClient) handleEvent(event EventMessage, rawMessage []byte) bool {
	switch event.Header.Event {
	case "task-started":
		fmt.Printf("Task started successfully, Task ID: %s\n", event.Header.TaskID)

		// Save the task ID returned by server
		c.taskIDMutex.Lock()
		c.taskID = event.Header.TaskID
		c.taskIDMutex.Unlock()

		// Notify that task ID has been received
		c.taskIDReceived.Done()

		// Signal that task has started
		c.taskStarted <- true

	case "result-generated":
		// Handle recognition results
		result := c.parseRecognitionResult(rawMessage)
		if result != nil && result.Text != "" {
			fmt.Printf("Recognition result: %s\n", result.Text)
			if result.BeginTime != nil && result.EndTime != nil {
				fmt.Printf("  Time: %dms - %dms\n", *result.BeginTime, *result.EndTime)
			}
		}

	case "task-finished":
		fmt.Printf("Task completed successfully, Task ID: %s\n", event.Header.TaskID)
		// Send normal WebSocket close frame
		c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.taskDone <- true
		return true

	case "task-failed":
		fmt.Printf("Task failed - Error Code: %s, Error Message: %s\n",
			event.Header.ErrorCode, event.Header.ErrorMessage)
		c.taskDone <- true
		return true

	default:
		log.Printf("Unknown event type: %s", event.Header.Event)
	}

	return false
}

// parseRecognitionResult parses the recognition result from raw message
func (c *SpeechToTextClient) parseRecognitionResult(rawMessage []byte) *RecognitionResult {
	var resultEvent EventMessage
	if err := json.Unmarshal(rawMessage, &resultEvent); err != nil {
		log.Printf("Failed to parse recognition result: %v", err)
		return nil
	}

	sentence := resultEvent.Payload.Output.Sentence
	if sentence.Text == "" {
		return nil
	}

	return &RecognitionResult{
		TaskID:    resultEvent.Header.TaskID,
		Text:      sentence.Text,
		BeginTime: sentence.BeginTime,
		EndTime:   sentence.EndTime,
	}
}

// validateMP3Format performs basic validation for MP3 files
func (c *SpeechToTextClient) validateMP3Format(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read first 1024 bytes to analyze MP3 structure
	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read MP3 file: %w", err)
	}
	if n < 10 {
		return fmt.Errorf("MP3 file too small")
	}

	// Check for ID3v2 tag
	if string(buffer[0:3]) == "ID3" {
		// Extract ID3v2 version
		majorVersion := buffer[3]
		minorVersion := buffer[4]
		fmt.Printf("MP3 file has ID3v2.%d.%d tag\n", majorVersion, minorVersion)

		// Calculate ID3v2 tag size
		tagSize := int(buffer[6])<<21 | int(buffer[7])<<14 | int(buffer[8])<<7 | int(buffer[9])
		fmt.Printf("ID3v2 tag size: %d bytes\n", tagSize)

		// Skip ID3 tag and look for MP3 frame
		if tagSize+10 < len(buffer) {
			frameStart := tagSize + 10
			if frameStart < len(buffer)-1 &&
				buffer[frameStart] == 0xFF &&
				(buffer[frameStart+1]&0xE0) == 0xE0 {
				fmt.Printf("Valid MP3 frame found after ID3 tag\n")
			}
		}
	} else if buffer[0] == 0xFF && (buffer[1]&0xE0) == 0xE0 {
		// Direct MP3 frame without ID3 tag
		fmt.Printf("MP3 frame sync found (no ID3 tag)\n")

		// Extract basic MP3 frame info
		version := (buffer[1] >> 3) & 0x03
		layer := (buffer[1] >> 1) & 0x03
		bitrate := (buffer[2] >> 4) & 0x0F
		sampleRate := (buffer[2] >> 2) & 0x03

		fmt.Printf("MP3 info - Version: %d, Layer: %d, Bitrate index: %d, Sample rate index: %d\n",
			version, layer, bitrate, sampleRate)
	} else {
		return fmt.Errorf("no valid MP3 frame sync found")
	}

	fmt.Printf("MP3 format validation passed\n")
	return nil
}

// convertMP3ToPCM converts MP3 audio file to PCM data
func (c *SpeechToTextClient) convertMP3ToPCM(mp3Path string) ([]byte, error) {
	file, err := os.Open(mp3Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open MP3 file: %w", err)
	}
	defer file.Close()

	decoder, err := mp3.NewDecoder(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create MP3 decoder: %w", err)
	}

	// Read all PCM data from MP3
	pcmData, err := io.ReadAll(decoder)
	if err != nil {
		return nil, fmt.Errorf("failed to read PCM data from MP3: %w", err)
	}

	fmt.Printf("Converted MP3 to PCM: %d bytes\n", len(pcmData))
	return pcmData, nil
}

// loadWAVAsPCM loads WAV file and extracts PCM data
func (c *SpeechToTextClient) loadWAVAsPCM(wavPath string) ([]byte, error) {
	file, err := os.Open(wavPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAV file: %w", err)
	}
	defer file.Close()

	// Skip WAV header (44 bytes) and read PCM data
	header := make([]byte, 44)
	_, err = file.Read(header)
	if err != nil {
		return nil, fmt.Errorf("failed to read WAV header: %w", err)
	}

	// Read the remaining PCM data
	pcmData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read PCM data from WAV: %w", err)
	}

	fmt.Printf("Extracted PCM from WAV: %d bytes\n", len(pcmData))
	return pcmData, nil
}

// loadPCMFile loads raw PCM file
func (c *SpeechToTextClient) loadPCMFile(pcmPath string) ([]byte, error) {
	file, err := os.Open(pcmPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PCM file: %w", err)
	}
	defer file.Close()

	pcmData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read PCM file: %w", err)
	}

	fmt.Printf("Loaded PCM file: %d bytes\n", len(pcmData))
	return pcmData, nil
}
