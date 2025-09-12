package rag

// GenerateRequest
type GenerateRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Temperature float32 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
	Think       bool    `json:"think"`
}

// GenerateResponse
type GenerateResponse struct {
	ID         string `json:"id"`
	Model      string `json:"model"`
	ModelName  string `json:"model_name,omitempty"` // 新增字段
	Content    string `json:"content"`
	IsComplete bool   `json:"is_complete"`        // 流式输出时，是否是最后一个块
	Thoughts   string `json:"thinking,omitempty"` // 深度思考的结果
}

type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type EmbeddingResponse struct {
	Object     string         `json:"object"`
	Embeddings [][]float32    `json:"embeddings"`
	Model      string         `json:"model"`
	Usage      EmbeddingUsage `json:"usage"`
}

type EmbeddingData struct {
	Object     string    `json:"object"`
	Embedding  []float32 `json:"embedding"`
	EmbedIndex int       `json:"index"`
}

type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type AOGAPIResponse struct {
	BusinessCode int         `json:"business_code"`
	Message      string      `json:"message"`
	Data         interface{} `json:"data"`
}

type AOGGenerateResponse struct {
	ID           string `json:"id"`
	Created      int64  `json:"created"`
	Model        string `json:"model"`
	Response     string `json:"response"`
	FinishReason string `json:"finish_reason"`
}

type AOGEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string         `json:"model"`
	Usage map[string]int `json:"usage"`
}
