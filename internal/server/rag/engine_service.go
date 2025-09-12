package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/schedule"
	"github.com/intel/aog/internal/types"
)

type EngineService struct {
	js datastore.JsonDatastore
	ds datastore.Datastore
}

// NewEngineService Create a new engine service instance
func NewEngineService() EngineServiceProvider {
	return &EngineService{
		js: datastore.GetDefaultJsonDatastore(),
		ds: datastore.GetDefaultDatastore(),
	}
}

// GenerateEmbedding Implement vector embedding generation function
func (e *EngineService) GenerateEmbedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	modelName := req.Model

	if len(req.Input) == 0 {
		return nil, fmt.Errorf("embedding input cannot be empty")
	}

	logger.EngineLogger.Info("[Embedding] 正在生成向量",
		"model", req.Model,
		"input_count", len(req.Input),
		"first_input_length", len(req.Input[0]))

	body, err := json.Marshal(req)
	if err != nil {
		logger.EngineLogger.Error("[Embedding] 请求体序列化失败", "error", err)
		return nil, err
	}

	inputSample := ""
	if len(req.Input) > 0 {
		sampleLength := 30
		if len(req.Input[0]) < sampleLength {
			sampleLength = len(req.Input[0])
		}
		inputSample = req.Input[0][:sampleLength] + "..."
	}

	logger.EngineLogger.Info("[Embedding] Request body for embedding",
		"model", req.Model,
		"input_count", len(req.Input),
		"input_sample", inputSample,
		"body_size", len(body))

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	hybridPolicy := "default"
	ds := datastore.GetDefaultDatastore()
	sp := &types.Service{
		Name:   "embed",
		Status: 1,
	}
	err = ds.Get(context.Background(), sp)
	if err != nil {
		logger.EngineLogger.Error("[Schedule] Failed to get service", "error", err, "service", "embed")
	} else {
		hybridPolicy = sp.HybridPolicy
	}
	hybridPolicy = sp.HybridPolicy

	serviceReq := &types.ServiceRequest{
		Service:      types.ServiceEmbed,
		Model:        modelName,
		FromFlavor:   constants.AppName,
		HybridPolicy: hybridPolicy,
		HTTP: types.HTTPContent{
			Header: headers,
			Body:   body,
		},
	}

	logger.EngineLogger.Info("[Embedding] 发送向量生成请求到调度器",
		"model", modelName,
		"body_length", len(body))

	_, ch := schedule.GetScheduler().Enqueue(serviceReq)
	select {
	case result := <-ch:
		if result.Error != nil {
			logger.EngineLogger.Error("[Embedding] Error from service provider", "error", result.Error)
			return nil, result.Error
		}

		responsePreview := "empty"
		if len(result.HTTP.Body) > 0 {
			previewLength := 100
			if len(result.HTTP.Body) < previewLength {
				previewLength = len(result.HTTP.Body)
			}
			responsePreview = string(result.HTTP.Body[:previewLength]) + "..."
		}

		logger.EngineLogger.Info("[Embedding] Received response from service provider",
			"response_size", len(result.HTTP.Body),
			"response_preview", responsePreview)

		var directData []map[string]interface{}
		if err := json.Unmarshal(result.HTTP.Body, &directData); err == nil && len(directData) > 0 {
			logger.EngineLogger.Info("[Embedding] 成功解析为直接数据数组",
				"array_length", len(directData))

			resp := &EmbeddingResponse{
				Object: "list", // 默认对象类型
				Model:  req.Model,
				Usage: EmbeddingUsage{
					PromptTokens: 0,
					TotalTokens:  0,
				},
			}

			for _, item := range directData {
				if embVector, ok := item["embedding"].([]interface{}); ok {
					embedding := make([]float32, len(embVector))
					for i, val := range embVector {
						if floatVal, ok := val.(float64); ok {
							embedding[i] = float32(floatVal)
						}
					}
					resp.Embeddings = append(resp.Embeddings, embedding)
				}
			}

			if len(resp.Embeddings) > 0 {
				logger.EngineLogger.Info("[Embedding] 成功从直接数据数组提取向量",
					"embedding_count", len(resp.Embeddings),
					"vector_dim", len(resp.Embeddings[0]))
				return resp, nil
			} else {
				logger.EngineLogger.Warn("[Embedding] 数据数组中未找到有效的向量数据")
			}
		}

		var aogResp AOGAPIResponse
		if err := json.Unmarshal(result.HTTP.Body, &aogResp); err != nil {
			logger.EngineLogger.Warn("[Embedding] 无法解析�?AOG 响应格式",
				"error", err.Error(),
				"response_preview", responsePreview)
		}

		if aogResp.BusinessCode > 0 && aogResp.BusinessCode != 10000 {
			logger.EngineLogger.Error("[Embedding] AOG API 返回错误",
				"business_code", aogResp.BusinessCode,
				"message", aogResp.Message)
			return nil, fmt.Errorf(" Embedding API 错误: %s (代码: %d)",
				aogResp.Message, aogResp.BusinessCode)
		}

		if aogResp.Data == nil {
			logger.EngineLogger.Error("[Embedding] AOG 响应�?data 字段为空")
			return nil, fmt.Errorf("AOG embedding 响应�?data 字段为空")
		}

		dataBytes, err := json.Marshal(aogResp.Data)
		if err != nil {
			logger.EngineLogger.Error("[Embedding] AOG data 字段序列化失败", "error", err)
			return nil, fmt.Errorf("AOG data 字段序列化失败: %w", err)
		}

		logger.EngineLogger.Debug("[Embedding] AOG 响应 data 字段内容",
			"data_json", string(dataBytes))

		var dataArray []map[string]interface{}
		parseArrayOk := false
		resp := &EmbeddingResponse{
			Object: "list",
			Model:  req.Model,
			Usage: EmbeddingUsage{
				PromptTokens: 0,
				TotalTokens:  0,
			},
		}
		if err := json.Unmarshal(dataBytes, &dataArray); err == nil && len(dataArray) > 0 {
			logger.EngineLogger.Info("[Embedding] data 字段是数组格式", "length", len(dataArray))
			for _, item := range dataArray {
				if embVector, ok := item["embedding"].([]interface{}); ok {
					embedding := make([]float32, len(embVector))
					for i, val := range embVector {
						if floatVal, ok := val.(float64); ok {
							embedding[i] = float32(floatVal)
						}
					}
					resp.Embeddings = append(resp.Embeddings, embedding)
				}
			}
			if len(resp.Embeddings) > 0 {
				logger.EngineLogger.Info("[Embedding] 成功从data 数组中提取向量",
					"embedding_count", len(resp.Embeddings),
					"vector_dim", len(resp.Embeddings[0]))
				parseArrayOk = true
				return resp, nil
			}
		}

		if !parseArrayOk {
			var embedResp AOGEmbeddingResponse
			if err := json.Unmarshal(dataBytes, &embedResp); err == nil {
				resp.Object = embedResp.Object
				resp.Model = embedResp.Model
				if embedResp.Usage != nil {
					resp.Usage.PromptTokens = embedResp.Usage["prompt_tokens"]
					resp.Usage.TotalTokens = embedResp.Usage["total_tokens"]
				}
				for _, d := range embedResp.Data {
					resp.Embeddings = append(resp.Embeddings, d.Embedding)
				}
				if len(resp.Embeddings) > 0 {
					logger.EngineLogger.Info("[Embedding] 成功解析 AOG embedding 响应",
						"embedding_count", len(resp.Embeddings),
						"vector_dim", len(resp.Embeddings[0]),
						"model", resp.Model)
					return resp, nil
				}
			} else {
				logger.EngineLogger.Warn("[Embedding] 无法解析为标�?AOGEmbeddingResponse",
					"error", err.Error())

				var directEmbeddings [][]float32
				if err := json.Unmarshal(dataBytes, &directEmbeddings); err == nil && len(directEmbeddings) > 0 {
					logger.EngineLogger.Info("[Embedding] �?AOG data 字段直接解析�?embeddings 数组",
						"embedding_count", len(directEmbeddings))
					resp.Embeddings = directEmbeddings
					return resp, nil
				}
				logger.EngineLogger.Error("[Embedding] 所有解析尝试均失败", "data_json", string(dataBytes))
				return nil, fmt.Errorf("AOG embedding 响应解析失败: 无法从数据中提取向量")
			}
		}
	case <-ctx.Done():
		logger.EngineLogger.Error("[Embedding] 上下文已取消", "error", ctx.Err())
		return nil, fmt.Errorf("向量生成请求被取�? %w", ctx.Err())
	}

	return nil, fmt.Errorf("embedding 处理过程中未能生成有效响应")
}

// Generate Implementing non-streaming text generation
func (e *EngineService) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	modelName := req.Model

	hybridPolicy := "default"
	ds := datastore.GetDefaultDatastore()
	sp := &types.Service{
		Name:   "generate",
		Status: 1,
	}
	err = ds.Get(context.Background(), sp)
	if err != nil {
		logger.EngineLogger.Error("[Schedule] Failed to get service", "error", err, "service", "embed")
	} else {
		hybridPolicy = sp.HybridPolicy
	}
	hybridPolicy = sp.HybridPolicy

	serviceReq := &types.ServiceRequest{
		Service:      types.ServiceGenerate,
		Model:        modelName,
		FromFlavor:   constants.AppName,
		HybridPolicy: hybridPolicy,
		Think:        req.Think,
		HTTP: types.HTTPContent{
			Header: http.Header{},
			Body:   body,
		},
	}
	_, ch := schedule.GetScheduler().Enqueue(serviceReq)
	select {
	case result := <-ch:
		if result.Error != nil {
			return nil, result.Error
		}

		// Log the raw response for debugging
		fmt.Printf("[Chat] Raw response received, length: %d\n", len(result.HTTP.Body))
		fmt.Printf("[Chat] Raw response content: %s\n", string(result.HTTP.Body))

		var aogResp AOGAPIResponse
		if err := json.Unmarshal(result.HTTP.Body, &aogResp); err == nil && aogResp.BusinessCode == 10000 {

			dataBytes, err := json.Marshal(aogResp.Data)
			if err != nil {
				return nil, fmt.Errorf("解析AOG响应data字段失败: %v", err)
			}

			var generateResp AOGGenerateResponse
			if err := json.Unmarshal(dataBytes, &generateResp); err != nil {
				return nil, fmt.Errorf("解析AOG聊天响应结构失败: %v", err)
			}

			response := &GenerateResponse{
				ID:         generateResp.ID,
				Model:      generateResp.Model,
				Content:    generateResp.Response,
				IsComplete: generateResp.FinishReason != "",
			}

			fmt.Printf("[Generate] AOG API解析成功，内容长度：%d\n", len(response.Content))
			return response, nil
		}

		// Attempt to resolve directly to a complete GenerateResponse
		var response GenerateResponse
		if err := json.Unmarshal(result.HTTP.Body, &response); err == nil && response.Content != "" {
			fmt.Printf("[Generate] 直接解析成功，内容长度：%d\n", len(response.Content))
			return &response, nil
		}

		// If all of the above methods fail, fall back to using generic map parsing
		fmt.Printf("[Generate] 标准解析方式失败，尝试通用map解析\n")
		var data map[string]interface{}
		if err := json.Unmarshal(result.HTTP.Body, &data); err != nil {
			return nil, fmt.Errorf("无法解析API响应: %v", err)
		}

		content := ""
		isComplete := false
		model := ""
		var thoughts string

		// Attempt to extract message.content
		if msg, ok := data["response"].(string); ok {
			content = msg
		}

		// If message.content is empty, try to extract the response
		if content == "" {
			if resp, ok := data["response"].(string); ok {
				content = resp
				fmt.Printf("[Generate] 从response提取内容，长�? %d\n", len(content))
			}
		}

		// Extraction completion flag
		if done, ok := data["done"].(bool); ok {
			isComplete = done
		}

		// Extract model name
		if m, ok := data["model"].(string); ok {
			model = m
		}

		// Create a response object
		resp := &GenerateResponse{
			Content:    content,
			Model:      model,
			IsComplete: isComplete,
			Thoughts:   thoughts,
		}
		fmt.Printf("[Generate] 通用解析成功，内容长�? %d\n", len(content))
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
