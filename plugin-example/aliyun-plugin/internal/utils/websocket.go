package utils

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// WebSocket message constants
const (
	// Client Action types
	WSActionRunTask    = "run-task"
	WSActionFinishTask = "finish-task"

	WSSTTTaskTypeUnknown    = "unknown"
	WSSTTTaskTypeRunTask    = WSActionRunTask    // Start recognition task
	WSSTTTaskTypeAudio      = "audio"            // Audio data
	WSSTTTaskTypeFinishTask = WSActionFinishTask // End recognition task

	// Server Event types
	WSEventTaskStarted        = "task-started"
	WSEventTaskFinished       = "task-finished"
	WSEventResultGenerated    = "result-generated"
	WSEventTaskFailed         = "task-failed"
	WSEventTaskResultGenerate = "result-generated"

	// Error codes
	WSErrorCodeClientError = "CLIENT_ERROR"
	WSErrorCodeServerError = "SERVER_ERROR"
	WSErrorCodeModelError  = "MODEL_ERROR"
)

var (
	WSRemoteLocalMap      = make(map[string]*websocket.Conn)
	wsRemoteLocalMapMutex sync.RWMutex
)

// GetWSRemoteConnection Thread-safe acquisition of WebSocket connections
func GetWSRemoteConnection(connID string) (*websocket.Conn, bool) {
	wsRemoteLocalMapMutex.RLock()
	defer wsRemoteLocalMapMutex.RUnlock()
	conn, exists := WSRemoteLocalMap[connID]
	return conn, exists
}

// SetWSRemoteConnection Thread-Safe Setup of WebSocket Connections
func SetWSRemoteConnection(connID string, conn *websocket.Conn) {
	wsRemoteLocalMapMutex.Lock()
	defer wsRemoteLocalMapMutex.Unlock()
	WSRemoteLocalMap[connID] = conn
}

// RemoveWSRemoteConnection Thread-safe removal of WebSocket connections
func RemoveWSRemoteConnection(connID string) {
	wsRemoteLocalMapMutex.Lock()
	defer wsRemoteLocalMapMutex.Unlock()
	delete(WSRemoteLocalMap, connID)
}

func ConnectWebSocket(wsURL, apiKey string) (*websocket.Conn, error) {
	header := make(http.Header)
	header.Add("X-DashScope-DataInspection", "enable")
	header.Add("Authorization", fmt.Sprintf("bearer %s", apiKey))
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	return conn, err
}

// WebSocketParameters Common parameter structure
type WebSocketParameters struct {
	Service      string `json:"service,omitempty"`       // service name，example: "speech-to-text" "speech-to-text-ws"
	Format       string `json:"format,omitempty"`        // format: pcm/wav/mp3
	SampleRate   int    `json:"sample_rate,omitempty"`   // sample_rate，default 16000
	Language     string `json:"language,omitempty"`      // language，如"zh"、"en"
	UseVAD       bool   `json:"use_vad,omitempty"`       // Whether to use VAD
	ReturnFormat string `json:"return_format,omitempty"` // return_format，example"text"、"json"、"srt"
}

type Header struct {
	Action       string                 `json:"action"`
	TaskID       string                 `json:"task_id"`
	Streaming    string                 `json:"streaming"`
	Event        string                 `json:"event"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Attributes   map[string]interface{} `json:"attributes"`
}

type Output struct {
	Sentence struct {
		BeginTime int64  `json:"begin_time"`
		EndTime   *int64 `json:"end_time"`
		Text      string `json:"text"`
		Words     []struct {
			BeginTime   int64  `json:"begin_time"`
			EndTime     *int64 `json:"end_time"`
			Text        string `json:"text"`
			Punctuation string `json:"punctuation"`
		} `json:"words"`
	} `json:"sentence"`
	Usage interface{} `json:"usage"`
}

type Payload struct {
	TaskGroup  string     `json:"task_group"`
	Task       string     `json:"task"`
	Function   string     `json:"function"`
	Model      string     `json:"model"`
	Parameters Params     `json:"parameters"`
	Resources  []Resource `json:"resources,omitempty"`
	Input      Input      `json:"input"`
	Output     Output     `json:"output,omitempty"`
}

type Params struct {
	Format                   string   `json:"format"`
	SampleRate               int      `json:"sample_rate"`
	VocabularyID             string   `json:"vocabulary_id,omitempty"`
	DisfluencyRemovalEnabled bool     `json:"disfluency_removal_enabled"`
	LanguageHints            []string `json:"language_hints,omitempty"`
}

type Input struct{}

type Resource struct {
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
}

type Event struct {
	Header  Header  `json:"header"`
	Payload Payload `json:"payload"`
}
