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

package schedule

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/intel/aog/internal/client"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
)

type BaseMiddleware struct {
	ServiceName string
}

// GetMiddleware Returns the corresponding middleware chain based on the service type
func GetMiddleware(serviceName string) []TaskMiddleware {
	switch serviceName {
	case types.ServiceTextToImage:
		return []TaskMiddleware{NewTextToImageMiddleware()}
	case types.ServiceSpeechToText:
		return []TaskMiddleware{NewSpeechToTextMiddleware()}
	case types.ServiceSpeechToTextWS:
		return []TaskMiddleware{NewSpeechToTextWSMiddleware()}
	case types.ServiceChat:
		return []TaskMiddleware{NewChatMiddleware()}
	case types.ServiceTextToSpeech:
		return []TaskMiddleware{NewTextToSpeechMiddleware()}
	case types.ServiceImageToVideo:
		return []TaskMiddleware{NewImageToVideoMiddleware()}
	case types.ServiceImageToImage:
		return []TaskMiddleware{NewImageToImageMiddleware()}
	default:
		return []TaskMiddleware{}
	}
}

// TextToImageMiddleware Handles image-related requests
type TextToImageMiddleware struct {
	BaseMiddleware
}

func (m *TextToImageMiddleware) Handle(st *ServiceTask) error {
	if st.Request.Service != m.ServiceName {
		return nil
	}

	body, err := utils.ParseRequestBody(st.Request.HTTP.Body)
	if err != nil {
		return fmt.Errorf("parse request body: %w", err)
	}

	if err := m.validateImageParams(body); err != nil {
		return err
	}

	return nil
}

func (m *TextToImageMiddleware) validateImageParams(body map[string]interface{}) error {
	size, sizeOk := body["size"].(string)
	batch, batchOk := body["n"].(float64)

	if sizeOk {
		matched, err := regexp.MatchString(`^\d+\*\d+$`, size)
		if err != nil {
			return fmt.Errorf("size parameter format validation error: %w", err)
		}
		if !matched {
			return fmt.Errorf("size parameter format error, should be like 512*512")
		}
		var w, h int
		_, err = fmt.Sscanf(size, "%d*%d", &w, &h)
		if err != nil {
			return fmt.Errorf("size parameter parsing failed: %w", err)
		}
		if w > 4096 || h > 4096 {
			return fmt.Errorf("size parameter cannot exceed 4096*4096")
		}
	}

	if batchOk {
		if batch < 1 || batch > 4 {
			return fmt.Errorf("image generation count must be between 1-4")
		}
	}

	return nil
}

func NewTextToImageMiddleware() *TextToImageMiddleware {
	return &TextToImageMiddleware{
		BaseMiddleware: BaseMiddleware{
			ServiceName: types.ServiceTextToImage,
		},
	}
}

// SpeechToTextMiddleware Handles SpeechToText-related requests
type SpeechToTextMiddleware struct {
	BaseMiddleware
}

func (m *SpeechToTextMiddleware) Handle(st *ServiceTask) error {
	if st.Request.Service != m.ServiceName {
		return nil
	}

	body, err := utils.ParseRequestBody(st.Request.HTTP.Body)
	if err != nil {
		return fmt.Errorf("parse request body: %w", err)
	}

	// Skip if no image-related fields
	if !m.hasFileFields(body) {
		return nil
	}

	if err := m.validateFileParams(body); err != nil {
		return err
	}

	if err := m.processFile(body, st.Target.Location); err != nil {
		return err
	}
	var newReqBody []byte
	if st.Request.Service == types.ServiceSpeechToText && st.Target.ServiceProvider.Flavor == types.FlavorAliYun {
		fileData := body["speech"].(string)
		newReqBody, err = base64.StdEncoding.DecodeString(fileData)
		st.Request.HTTP.Header.Set("Content-Type", "application/octet-stream")
	} else {
		newReqBody, err = json.Marshal(body)
	}
	if err != nil {
		return fmt.Errorf("marshal new request body: %w", err)
	}
	st.Request.HTTP.Body = newReqBody
	return nil
	logger.LogicLogger.Debug("[Middleware] Processing audio request")
	return nil
}

func (m *SpeechToTextMiddleware) hasFileFields(body map[string]interface{}) bool {
	_, typeExists := body["file_type"]
	_, imageExists := body["file"]
	return typeExists || imageExists
}

func (m *SpeechToTextMiddleware) validateFileParams(body map[string]interface{}) error {
	imageType, typeOk := body["file_type"].(string)
	_, imageOk := body["file"].(string)

	switch {
	case !typeOk && imageOk:
		return errors.New("image_type request param lost")
	case typeOk && !imageOk:
		return errors.New("image request param lost")
	case !utils.Contains(types.SupportImageType, imageType):
		return errors.New("unsupported image type")
	}
	return nil
}

func (m *SpeechToTextMiddleware) processFile(body map[string]interface{}, location string) error {
	imageType := body["file_type"].(string)
	image := body["file"].(string)

	switch {
	case imageType == types.ImageTypePath && location == types.ServiceSourceRemote:
		return m.handleLocalToRemote(body, image)
	case imageType == types.ImageTypeUrl && location == types.ServiceSourceLocal:
		return m.handleRemoteToLocal(body, image)
	}
	return nil
}

func (m *SpeechToTextMiddleware) handleLocalToRemote(body map[string]interface{}, filePath string) error {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		logger.LogicLogger.Error("[Middleware] Failed to read image file", "error", err)
		return fmt.Errorf("read image file: %w", err)
	}
	body["speech"] = base64.StdEncoding.EncodeToString(fileData)
	body["len"] = len(fileData)
	return nil
}

func (m *SpeechToTextMiddleware) handleRemoteToLocal(body map[string]interface{}, imageUrl string) error {
	downLoadPath, err := utils.GetDownloadDir()
	if err != nil {
		return fmt.Errorf("get download directory: %w", err)
	}
	savePath, err := utils.DownloadFile(imageUrl, downLoadPath, false)
	if err != nil {
		return fmt.Errorf("download file: %w", err)
	}
	body["image"] = savePath
	return nil
}

func NewSpeechToTextMiddleware() *SpeechToTextMiddleware {
	return &SpeechToTextMiddleware{
		BaseMiddleware: BaseMiddleware{
			ServiceName: types.ServiceSpeechToText,
		},
	}
}

// SpeechToTextWSMiddleware handles WebSocket speech recognition requests
type SpeechToTextWSMiddleware struct {
	BaseMiddleware
}

func (m *SpeechToTextWSMiddleware) Handle(st *ServiceTask) error {
	// Check if it's the target service type
	if st.Request.Service != m.ServiceName {
		return nil
	}

	// Get WebSocket connection ID
	connID := st.Request.WebSocketConnID
	if connID == "" {
		return fmt.Errorf("missing WebSocket connection ID")
	}

	logger.LogicLogger.Debug("[SpeechToTextWS] Processing WebSocket request",
		"connID", connID,
		"taskID", st.Schedule.Id)

	// Get connection from WebSocketManager - add more debug information
	wsManager := client.GetGlobalWebSocketManager()
	logger.LogicLogger.Debug("[SpeechToTextWS] Attempting to get WebSocket connection",
		"connID", connID,
		"managerInstanceAddr", fmt.Sprintf("%p", wsManager),
		"connectionsCount", wsManager.GetActiveConnectionCount())

	wsConn, exists := wsManager.GetConnection(connID)
	if !exists {
		logger.LogicLogger.Error("[SpeechToTextWS] WebSocket connection not found",
			"connID", connID,
			"managerInstanceAddr", fmt.Sprintf("%p", wsManager),
			"connectionsCount", wsManager.GetActiveConnectionCount())
		return fmt.Errorf("WebSocket connection not found: %s", connID)
	}

	// Get and log STT parameters
	sttParams := wsConn.GetSTTParams()
	logger.LogicLogger.Debug("[SpeechToTextWS] Retrieved connection parameters",
		"connID", connID,
		"format", sttParams.AudioFormat,
		"sampleRate", sttParams.SampleRate,
		"language", sttParams.Language,
		"totalBytes", sttParams.TotalAudioBytes,
		"taskStarted", wsConn.IsTaskStarted(st.Schedule.Id))

	// Get task type
	taskType := wsConn.GetTaskType(st.Schedule.Id)
	logger.LogicLogger.Debug("[SpeechToTextWS] Task type",
		"connID", connID,
		"taskType", taskType)

	// Execute different parameter validation and processing based on task type
	switch taskType {
	case types.WSSTTTaskTypeAudio:
		return m.validateAudioData(st, wsConn)
	case types.WSSTTTaskTypeRunTask:
		return m.validateRunTask(st, wsConn)
	case types.WSSTTTaskTypeFinishTask:
		return m.validateFinishTask(st, wsConn)
	default:
		logger.LogicLogger.Warn("[SpeechToTextWS] Unknown task type",
			"taskType", taskType,
			"connID", wsConn.ID)
		return fmt.Errorf("unknown task type: %v", taskType)
	}
}

// Validate and process start task command
func (m *SpeechToTextWSMiddleware) validateRunTask(st *ServiceTask, session *client.WebSocketConnection) error {
	sttParams := session.GetSTTParams()

	logger.LogicLogger.Debug("[SpeechToTextWS] Validating run-task command",
		"connID", session.ID,
		"language", sttParams.Language,
		"sampleRate", sttParams.SampleRate)

	// Validate language parameter
	if sttParams.Language == "" {
		sttParams.Language = types.LanguageZh // Default to Chinese
		logger.LogicLogger.Info("[SpeechToTextWS] Using default language",
			"language", sttParams.Language,
			"connID", session.ID)
	}

	// Validate sample rate parameter
	validSampleRates := []int{8000, 16000, 22050, 44100, 48000}
	validRate := false
	for _, rate := range validSampleRates {
		if sttParams.SampleRate == rate {
			validRate = true
			break
		}
	}

	if !validRate {
		logger.LogicLogger.Warn("[SpeechToTextWS] Invalid sample rate, using default",
			"provided", sttParams.SampleRate,
			"default", 16000,
			"connID", session.ID)
		sttParams.SampleRate = 16000
	}

	// Validate return format
	validFormats := []string{"text", "json", "srt", "vtt"}
	if !utils.Contains(validFormats, sttParams.ReturnFormat) {
		logger.LogicLogger.Warn("[SpeechToTextWS] Invalid return format, using default",
			"provided", sttParams.ReturnFormat,
			"default", "text",
			"connID", session.ID)
		sttParams.ReturnFormat = "text"
	}

	logger.LogicLogger.Info("[SpeechToTextWS] Task validated",
		"connID", session.ID)

	return nil
}

// Validate and process audio data
func (m *SpeechToTextWSMiddleware) validateAudioData(st *ServiceTask, session *client.WebSocketConnection) error {
	sttParams := session.GetSTTParams()

	// Check if connection ID is valid
	if session.ID == "" {
		return fmt.Errorf("invalid WebSocket connection ID")
	}

	// Check audio data size
	dataSize := len(st.Request.HTTP.Body)
	if dataSize == 0 {
		return fmt.Errorf("empty audio data received")
	}

	if dataSize > 10*1024*1024 { // 10MB limit
		return fmt.Errorf("audio data too large: %d bytes (max 10MB)", dataSize)
	}

	// Update audio processing status in session
	sttParams.TotalAudioBytes += dataSize
	sttParams.LastAudioTime = time.Now().Unix()

	logger.LogicLogger.Debug("[SpeechToTextWS] Processed audio data chunk",
		"connID", session.ID,
		"size", dataSize,
		"totalBytes", sttParams.TotalAudioBytes)

	return nil
}

// Validate and process finish task command
func (m *SpeechToTextWSMiddleware) validateFinishTask(st *ServiceTask, session *client.WebSocketConnection) error {
	// Check if connection ID exists
	if session.ID == "" {
		return fmt.Errorf("invalid WebSocket connection ID")
	}

	// Log task completion status
	startTime, endTime := session.GetTaskTimes(st.Schedule.Id)
	duration := endTime - startTime
	sttParams := session.GetSTTParams()

	logger.LogicLogger.Info("[SpeechToTextWS] Task validated for finish",
		"connID", session.ID,
		"duration", duration,
		"totalBytes", sttParams.TotalAudioBytes)

	return nil
}

func NewSpeechToTextWSMiddleware() *SpeechToTextWSMiddleware {
	return &SpeechToTextWSMiddleware{
		BaseMiddleware: BaseMiddleware{
			ServiceName: types.ServiceSpeechToTextWS,
		},
	}
}

// ChatMiddleware Handles chat-related requests
type ChatMiddleware struct {
	BaseMiddleware
}

func (m *ChatMiddleware) Handle(st *ServiceTask) error {
	if st.Request.Service != m.ServiceName {
		return nil
	}

	// TODO: Implement chat processing logic
	// 1. Message format validation
	// 2. Sensitive word filtering
	// 3. Message length limitation, etc.
	logger.LogicLogger.Debug("[Middleware] Processing chat request")
	return nil
}

func NewChatMiddleware() *ChatMiddleware {
	return &ChatMiddleware{
		BaseMiddleware: BaseMiddleware{
			ServiceName: types.ServiceChat,
		},
	}
}

// ExecuteMiddleware Executes the middleware chain
func ExecuteMiddleware(st *ServiceTask) error {
	middlewares := GetMiddleware(st.Request.Service)
	for _, m := range middlewares {
		if err := m.Handle(st); err != nil {
			return fmt.Errorf("middleware execution failed: %w", err)
		}
	}
	return nil
}

func NewTextToSpeechMiddleware() *TextToSpeechMiddleware {
	return &TextToSpeechMiddleware{
		BaseMiddleware: BaseMiddleware{
			ServiceName: types.ServiceTextToSpeech,
		},
	}
}

type TextToSpeechMiddleware struct {
	BaseMiddleware
}

func (m *TextToSpeechMiddleware) Handle(st *ServiceTask) error {
	if st.Request.Service != m.ServiceName {
		return nil
	}

	if st.Request.HTTP.Body == nil {
		return fmt.Errorf("request body is missing for %s service", m.ServiceName)
	}

	req := new(types.TextToSpeechRequest)

	err := json.Unmarshal(st.Request.HTTP.Body, req)
	if err != nil {
		return err
	}

	if req.Text == "" {
		return fmt.Errorf("text field is required for %s service", m.ServiceName)
	}
	if st.Target.Location == types.ServiceSourceLocal {
		if !utils.Contains(types.SupportVoiceType, req.Voice) {
			return fmt.Errorf("invalid voice type: %s, must be one of %v", req.Voice, types.SupportVoiceType)
		}

		matched, err := regexp.MatchString("\\p{Han}", req.Text)
		if err != nil {
			return fmt.Errorf("an error occurred while verifying the text: %w", err)
		}
		if matched {
			return fmt.Errorf("currently, only English speech generation is supported")
		}
	} else {
		uuid := uuid.New().String()
		var reqData map[string]interface{}
		err = json.Unmarshal(st.Request.HTTP.Body, &reqData)
		if err != nil {
			return err
		}
		reqData["uuid"] = uuid
		reqBody, err := json.Marshal(reqData)
		if err != nil {
			return err
		}
		st.Request.HTTP.Body = reqBody
	}

	logger.LogicLogger.Debug("[Middleware] Processing audio request")
	return nil
}

func NewImageToVideoMiddleware() *ImageToVideoMiddleware {
	return &ImageToVideoMiddleware{
		BaseMiddleware: BaseMiddleware{
			ServiceName: types.ServiceImageToVideo,
		},
	}
}

type ImageToVideoMiddleware struct {
	BaseMiddleware
}

func (m *ImageToVideoMiddleware) Handle(st *ServiceTask) error {
	if st.Request.Service != m.ServiceName {
		return nil
	}

	body, err := utils.ParseRequestBody(st.Request.HTTP.Body)
	if err != nil {
		return fmt.Errorf("parse request body: %w", err)
	}

	// Skip if no image-related fields
	if !m.hasImageFields(body) {
		return nil
	}

	if err := m.validateImageParams(body); err != nil {
		return err
	}

	if err := m.processImage(body, st.Target.Location); err != nil {
		return err
	}

	newReqBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal new request body: %w", err)
	}
	st.Request.HTTP.Body = newReqBody
	return nil
}

func (m *ImageToVideoMiddleware) hasImageFields(body map[string]interface{}) bool {
	_, typeExists := body["image_type"]
	_, imageExists := body["image"]
	return typeExists || imageExists
}

func (m *ImageToVideoMiddleware) validateImageParams(body map[string]interface{}) error {
	imageType, typeOk := body["image_type"].(string)
	_, imageOk := body["image"].(string)

	switch {
	case !typeOk && imageOk:
		return errors.New("image_type request param lost")
	case typeOk && !imageOk:
		return errors.New("image request param lost")
	case !utils.Contains(types.SupportImageType, imageType):
		return errors.New("unsupported image type")
	}
	return nil
}

func (m *ImageToVideoMiddleware) processImage(body map[string]interface{}, location string) error {
	imageType := body["image_type"].(string)
	image := body["image"].(string)

	switch {
	case imageType == types.ImageTypePath && location == types.ServiceSourceRemote:
		return m.handleLocalToRemote(body, image)
	case imageType == types.ImageTypeUrl && location == types.ServiceSourceLocal:
		return m.handleRemoteToLocal(body, image)
	}
	return nil
}

func (m *ImageToVideoMiddleware) handleLocalToRemote(body map[string]interface{}, imagePath string) error {
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		logger.LogicLogger.Error("[Middleware] Failed to read image file", "error", err)
		return fmt.Errorf("read image file: %w", err)
	}
	ext := filepath.Ext(imagePath) // .jpg
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = http.DetectContentType(imgData) // 更保险的 MIME 检测方式
	}
	encoded := base64.StdEncoding.EncodeToString(imgData)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
	body["image"] = dataURI
	return nil
}

func (m *ImageToVideoMiddleware) handleRemoteToLocal(body map[string]interface{}, imageUrl string) error {
	downLoadPath, err := utils.GetDownloadDir()
	if err != nil {
		return fmt.Errorf("get download directory: %w", err)
	}
	savePath, err := utils.DownloadFile(imageUrl, downLoadPath, false)
	if err != nil {
		return fmt.Errorf("download file: %w", err)
	}
	body["image"] = savePath
	return nil
}

func NewImageToImageMiddleware() *ImageToImageMiddleware {
	return &ImageToImageMiddleware{
		BaseMiddleware: BaseMiddleware{
			ServiceName: types.ServiceImageToImage,
		},
	}
}

type ImageToImageMiddleware struct {
	BaseMiddleware
}

func (m *ImageToImageMiddleware) Handle(st *ServiceTask) error {
	if st.Request.Service != m.ServiceName {
		return nil
	}

	body, err := utils.ParseRequestBody(st.Request.HTTP.Body)
	if err != nil {
		return fmt.Errorf("parse request body: %w", err)
	}

	// Skip if no image-related fields
	if !m.hasImageFields(body) {
		return nil
	}

	if err := m.validateImageParams(body); err != nil {
		return err
	}

	if err := m.processImage(body, st.Target.Location); err != nil {
		return err
	}

	newReqBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal new request body: %w", err)
	}
	st.Request.HTTP.Body = newReqBody
	return nil
}

func (m *ImageToImageMiddleware) hasImageFields(body map[string]interface{}) bool {
	_, typeExists := body["image_type"]
	_, imageExists := body["image"]
	return typeExists || imageExists
}

func (m *ImageToImageMiddleware) validateImageParams(body map[string]interface{}) error {
	imageType, typeOk := body["image_type"].(string)
	_, imageOk := body["image"].(string)

	switch {
	case !typeOk && imageOk:
		return errors.New("image_type request param lost")
	case typeOk && !imageOk:
		return errors.New("image request param lost")
	case !utils.Contains(types.SupportImageType, imageType):
		return errors.New("unsupported image type")
	}
	return nil
}

func (m *ImageToImageMiddleware) processImage(body map[string]interface{}, location string) error {
	imageType := body["image_type"].(string)
	image := body["image"].(string)

	switch {
	case imageType == types.ImageTypePath && location == types.ServiceSourceRemote:
		return m.handleLocalToRemote(body, image)
	case imageType == types.ImageTypeUrl && location == types.ServiceSourceLocal:
		return m.handleRemoteToLocal(body, image)
	}
	return nil
}

func (m *ImageToImageMiddleware) handleLocalToRemote(body map[string]interface{}, imagePath string) error {
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		logger.LogicLogger.Error("[Middleware] Failed to read image file", "error", err)
		return fmt.Errorf("read image file: %w", err)
	}
	ext := filepath.Ext(imagePath) // .jpg
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = http.DetectContentType(imgData) // 更保险的 MIME 检测方式
	}
	encoded := base64.StdEncoding.EncodeToString(imgData)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
	body["image"] = dataURI
	return nil
}

func (m *ImageToImageMiddleware) handleRemoteToLocal(body map[string]interface{}, imageUrl string) error {
	downLoadPath, err := utils.GetDownloadDir()
	if err != nil {
		return fmt.Errorf("get download directory: %w", err)
	}
	savePath, err := utils.DownloadFile(imageUrl, downLoadPath, false)
	if err != nil {
		return fmt.Errorf("download file: %w", err)
	}
	body["image"] = savePath
	return nil
}
