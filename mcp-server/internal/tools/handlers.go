// Package tools implements MCP tool handlers
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aog/mcp-server/internal/client"
	"github.com/aog/mcp-server/internal/types"
)

// ToolHandlers tool handlers collection
type ToolHandlers struct {
	aogClient *client.AOGClient
}

// NewToolHandlers creates new tool handlers
func NewToolHandlers(aogClient *client.AOGClient) *ToolHandlers {
	return &ToolHandlers{
		aogClient: aogClient,
	}
}

// createSuccessResponse creates success response
func createSuccessResponse(data any, message string) *mcp.CallToolResult {
	response := types.ToolResponse{
		Success: true,
		Data:    data,
		Message: message,
	}

	jsonData, _ := json.Marshal(response)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
	}
}

// createErrorResponse creates error response
func createErrorResponse(err error) *mcp.CallToolResult {
	response := types.ToolResponse{
		Success: false,
		Error:   err.Error(),
		Message: "Operation failed",
	}

	jsonData, _ := json.Marshal(response)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
		IsError: true,
	}
}

// validateModelExists validates if the specified model exists in the installed models
func (h *ToolHandlers) validateModelExists(ctx context.Context, modelName, serviceName string) error {
	if modelName == "" {
		return nil // Empty model name is allowed, will use default model
	}

	// Get installed models for the service
	req := types.GetModelsRequest{
		ServiceName: serviceName,
	}

	response, err := h.aogClient.GetModels(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get installed models: %w", err)
	}

	// Check if the specified model exists
	for _, model := range response.Data {
		if model.ModelName == modelName {
			return nil // Model found
		}
	}

	// Model not found, provide helpful error message
	var availableModels []string
	for _, model := range response.Data {
		if model.ServiceName == serviceName {
			availableModels = append(availableModels, model.ModelName)
		}
	}

	if len(availableModels) == 0 {
		return fmt.Errorf("model '%s' not found. No models are installed for service '%s'. Please install a model first using aog_install_model", modelName, serviceName)
	}

	return fmt.Errorf("model '%s' not found. Available models for service '%s': %s. Please use aog_get_models to see all installed models",
		modelName, serviceName, strings.Join(availableModels, ", "))
}

// HandleGetServices handles getting service list
func (h *ToolHandlers) HandleGetServices(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
	var req types.GetAIGCServicesRequest
	if name, ok := params["service_name"].(string); ok {
		req.ServiceName = name
	}

	response, err := h.aogClient.GetServices(ctx, req)
	if err != nil {
		return createErrorResponse(err), nil
	}

	return createSuccessResponse(response, fmt.Sprintf("Found %d services", len(response.Data))), nil
}

// HandleGetServiceProviders handles getting service providers
func (h *ToolHandlers) HandleGetServiceProviders(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
	var req types.GetServiceProvidersRequest
	if name, ok := params["service_name"].(string); ok {
		req.ServiceName = name
	}
	if name, ok := params["provider_name"].(string); ok {
		req.ProviderName = name
	}
	if source, ok := params["service_source"].(string); ok {
		req.ServiceSource = source
	}
	if flavor, ok := params["api_flavor"].(string); ok {
		req.ApiFlavor = flavor
	}

	response, err := h.aogClient.GetServiceProviders(ctx, req)
	if err != nil {
		return createErrorResponse(err), nil
	}

	return createSuccessResponse(response, fmt.Sprintf("Found %d service providers", len(response.Data))), nil
}

// HandleGetModels handles getting model list
func (h *ToolHandlers) HandleGetModels(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
	var req types.GetModelsRequest
	if name, ok := params["provider_name"].(string); ok {
		req.ProviderName = name
	}
	if name, ok := params["model_name"].(string); ok {
		req.ModelName = name
	}
	if name, ok := params["service_name"].(string); ok {
		req.ServiceName = name
	}

	response, err := h.aogClient.GetModels(ctx, req)
	if err != nil {
		return createErrorResponse(err), nil
	}

	return createSuccessResponse(response, fmt.Sprintf("Found %d installed models", len(response.Data))), nil
}

// HandleGetRecommendedModels handles getting recommended models
func (h *ToolHandlers) HandleGetRecommendedModels(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
	response, err := h.aogClient.GetRecommendedModels(ctx)
	if err != nil {
		return createErrorResponse(err), nil
	}

	// Calculate total number of recommended models
	totalCount := 0
	for _, models := range response.Data {
		totalCount += len(models)
	}

	return createSuccessResponse(response, fmt.Sprintf("Found %d recommended models", totalCount)), nil
}

// HandleGetSupportedModels handles getting supported models
func (h *ToolHandlers) HandleGetSupportedModels(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
	var req types.GetSupportModelRequest

	// Optional parameters
	if serviceSource, ok := params["service_source"].(string); ok {
		req.ServiceSource = serviceSource
	}

	if flavor, ok := params["flavor"].(string); ok {
		req.Flavor = flavor
	}

	if serviceName, ok := params["service_name"].(string); ok {
		req.ServiceName = serviceName
	}
	if searchName, ok := params["search_name"].(string); ok {
		req.SearchName = searchName
	}
	if pageSize, ok := params["page_size"].(float64); ok {
		req.PageSize = int(pageSize)
	}
	if page, ok := params["page"].(float64); ok {
		req.Page = int(page)
	}
	if mine, ok := params["mine"].(bool); ok {
		req.Mine = mine
	}

	response, err := h.aogClient.GetSupportedModels(ctx, req)
	if err != nil {
		return createErrorResponse(err), nil
	}

	return createSuccessResponse(response, fmt.Sprintf("Found %d supported models", len(response.Data))), nil
}

// HandleInstallModel handles installing model
func (h *ToolHandlers) HandleInstallModel(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
	var req types.CreateModelRequest

	modelName, ok := params["model_name"].(string)
	if !ok {
		return createErrorResponse(fmt.Errorf("missing required parameter model_name")), nil
	}
	req.ModelName = modelName

	serviceName, ok := params["service_name"].(string)
	if !ok {
		return createErrorResponse(fmt.Errorf("missing required parameter service_name")), nil
	}
	req.ServiceName = serviceName

	serviceSource, ok := params["service_source"].(string)
	if !ok {
		return createErrorResponse(fmt.Errorf("missing required parameter service_source")), nil
	}
	req.ServiceSource = serviceSource

	if providerName, ok := params["provider_name"].(string); ok {
		req.ProviderName = providerName
	}
	if size, ok := params["size"].(string); ok {
		req.Size = size
	}

	supportReq := types.GetSupportModelRequest{
		ServiceSource: serviceSource,
		ServiceName:   serviceName,
		SearchName:    modelName,
		PageSize:      1000,
	}
	supportResp, err := h.aogClient.GetSupportedModels(ctx, supportReq)
	if err != nil {
		return createErrorResponse(fmt.Errorf("failed to get supported models: %w", err)), nil
	}

	found := false
	for _, m := range supportResp.Data {
		if m.ModelName == modelName {
			found = true
			break
		}
	}
	if !found {
		return createErrorResponse(fmt.Errorf("model '%s' is not in the supported model list for service '%s', source '%s'", modelName, serviceName, serviceSource)), nil
	}

	response, err := h.aogClient.InstallModel(ctx, req)
	if err != nil {
		return createErrorResponse(err), nil
	}

	return createSuccessResponse(response, fmt.Sprintf("Model %s installation started, please use aog_get_models to check installation status later", req.ModelName)), nil
}

// HandleChat handles chat requests
func (h *ToolHandlers) HandleChat(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
	var req types.ChatRequest

	// Parse messages
	messagesRaw, ok := params["messages"]
	if !ok {
		return createErrorResponse(fmt.Errorf("missing required parameter messages")), nil
	}

	messagesData, err := json.Marshal(messagesRaw)
	if err != nil {
		return createErrorResponse(fmt.Errorf("failed to parse messages: %w", err)), nil
	}

	if err := json.Unmarshal(messagesData, &req.Messages); err != nil {
		return createErrorResponse(fmt.Errorf("failed to parse messages: %w", err)), nil
	}

	// Parse optional parameters
	if model, ok := params["model"].(string); ok && model != "" {
		req.Model = model
		// Validate model exists before making the request
		if err := h.validateModelExists(ctx, model, "chat"); err != nil {
			return createErrorResponse(err), nil
		}
	}
	if temp, ok := params["temperature"].(float64); ok {
		req.Temperature = &temp
	}
	if maxTokens, ok := params["max_tokens"].(float64); ok {
		tokens := int(maxTokens)
		req.MaxTokens = &tokens
	}
	if stream, ok := params["stream"].(bool); ok {
		req.Stream = &stream
	}

	response, err := h.aogClient.Chat(ctx, req)
	if err != nil {
		return createErrorResponse(err), nil
	}

	return createSuccessResponse(response, "Chat request successful"), nil
}

// HandleTextToImage handles text-to-image requests
func (h *ToolHandlers) HandleTextToImage(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
	var req types.TextToImageRequest

	prompt, ok := params["prompt"].(string)
	if !ok {
		return createErrorResponse(fmt.Errorf("missing required parameter prompt")), nil
	}
	req.Prompt = prompt

	// Parse optional parameters
	if model, ok := params["model"].(string); ok && model != "" {
		req.Model = model
		// Validate model exists before making the request
		if err := h.validateModelExists(ctx, model, "text-to-image"); err != nil {
			return createErrorResponse(err), nil
		}
	}
	if n, ok := params["n"].(float64); ok {
		num := int(n)
		req.N = &num
	}
	if size, ok := params["size"].(string); ok && size != "" {
		req.Size = size
	}
	if seed, ok := params["seed"].(float64); ok {
		seedInt := int(seed)
		req.Seed = &seedInt
	}

	// Due to MCP Go SDK's 10-second hardcoded timeout limit, we need to complete within 8 seconds or return processing status
	resultChan := make(chan *types.TextToImageResponse, 1)
	errorChan := make(chan error, 1)

	// Start background goroutine to handle actual request
	go func() {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		response, err := h.aogClient.TextToImage(timeoutCtx, req)
		if err != nil {
			errorChan <- err
		} else {
			resultChan <- response
		}
	}()

	// Wait for 5 minutes, return result if completed, otherwise return timeout status
	select {
	case response := <-resultChan:
		// Check business response code (if exists)
		if response.BusinessCode != 0 && response.BusinessCode != 200 {
			return createErrorResponse(fmt.Errorf("text-to-image service returned error: %s", response.Message)), nil
		}

		// Return image data, maintaining compatibility with original format
		imageData := map[string]interface{}{
			"data": response.Data,
		}
		if response.ID != "" {
			imageData["id"] = response.ID
		}

		return createSuccessResponse(imageData, "Image generation successful"), nil
	case err := <-errorChan:
		return createErrorResponse(fmt.Errorf("text-to-image service call failed: %w", err)), nil
	case <-time.After(5 * time.Minute):
		// Return timeout status after 5 minutes
		return createErrorResponse(fmt.Errorf("text-to-image service timeout: request processing time exceeded 5 minutes")), nil
	}
}

// HandleSpeechToText handles speech-to-text requests
func (h *ToolHandlers) HandleSpeechToText(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
	var req types.SpeechToTextRequest

	audio, ok := params["audio"].(string)
	if !ok {
		return createErrorResponse(fmt.Errorf("missing required parameter audio")), nil
	}
	req.Audio = audio

	// Parse optional parameters
	if model, ok := params["model"].(string); ok && model != "" {
		req.Model = model
		// Validate model exists before making the request
		if err := h.validateModelExists(ctx, model, "speech-to-text"); err != nil {
			return createErrorResponse(err), nil
		}
	}
	if language, ok := params["language"].(string); ok && language != "" {
		req.Language = language
	}

	// Create independent context for speech-to-text service to avoid MCP client timeout limits
	// Use 10-minute timeout, sufficient for processing long audio files
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	response, err := h.aogClient.SpeechToText(timeoutCtx, req)
	if err != nil {
		return createErrorResponse(err), nil
	}

	// Check business response code (if exists)
	if response.BusinessCode != 0 && response.BusinessCode != 200 {
		return createErrorResponse(fmt.Errorf("speech-to-text service returned error: %s", response.Message)), nil
	}

	// Return speech recognition data, maintaining compatibility with original format
	speechData := map[string]interface{}{
		"segments": response.Data.Segments,
	}

	// For backward compatibility, also provide merged text
	var fullText string
	for _, segment := range response.Data.Segments {
		if fullText != "" {
			fullText += " "
		}
		fullText += segment.Text
	}
	speechData["text"] = fullText

	return createSuccessResponse(speechData, "Speech recognition successful"), nil
}

// HandleEmbed handles text embedding requests
func (h *ToolHandlers) HandleEmbed(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
	var req types.EmbedRequest

	input, ok := params["input"].(string)
	if !ok {
		return createErrorResponse(fmt.Errorf("missing required parameter input")), nil
	}
	req.Input = input

	// Parse optional parameters
	if model, ok := params["model"].(string); ok && model != "" {
		req.Model = model
		// Validate model exists before making the request
		if err := h.validateModelExists(ctx, model, "embed"); err != nil {
			return createErrorResponse(err), nil
		}
	}

	response, err := h.aogClient.Embed(ctx, req)
	if err != nil {
		return createErrorResponse(err), nil
	}

	// Check business response code
	if response.BusinessCode != 200 {
		return createErrorResponse(fmt.Errorf("text embedding service returned error: %s", response.Message)), nil
	}

	// Return embedding data, maintaining compatibility with original format
	embedData := map[string]interface{}{
		"data":  response.Data.Data,
		"model": response.Data.Model,
	}

	return createSuccessResponse(embedData, "Text embedding successful"), nil
}

// HandleHealthCheck handles health check
func (h *ToolHandlers) HandleHealthCheck(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
	health, err := h.aogClient.HealthCheck(ctx)
	if err != nil {
		return createErrorResponse(err), nil
	}

	return createSuccessResponse(health, "AOG service health check completed"), nil
}

// HandleGetVersion handles getting version
func (h *ToolHandlers) HandleGetVersion(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
	version, err := h.aogClient.GetVersion(ctx)
	if err != nil {
		return createErrorResponse(err), nil
	}

	return createSuccessResponse(version, "AOG version information retrieved successfully"), nil
}
