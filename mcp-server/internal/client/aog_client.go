// Package client provides AOG HTTP client
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/aog/mcp-server/internal/types"
)

// AOGClient AOG HTTP client
type AOGClient struct {
	baseURL    string
	version    string
	httpClient *http.Client
}

// NewAOGClient creates a new AOG client
func NewAOGClient(config types.AOGConfig) *AOGClient {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:16688"
	}
	if config.Version == "" {
		config.Version = "v0.2"
	}
	if config.Timeout == 0 {
		config.Timeout = 120000 // Default 2 minutes, suitable for time-consuming services like text-to-image
	}

	return &AOGClient{
		baseURL: config.BaseURL,
		version: config.Version,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Millisecond,
		},
	}
}

// getAPIPath gets API path
func (c *AOGClient) getAPIPath(endpoint string) string {
	return fmt.Sprintf("/aog/%s%s", c.version, endpoint)
}

// doRequest executes HTTP request
func (c *AOGClient) doRequest(ctx context.Context, method, path string, body any, result any) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AOG-MCP-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// HealthCheck performs health check
func (c *AOGClient) HealthCheck(ctx context.Context) (*types.HealthResponse, error) {
	var result types.HealthResponse
	err := c.doRequest(ctx, "GET", "/health", nil, &result)
	return &result, err
}

// GetVersion gets version information
func (c *AOGClient) GetVersion(ctx context.Context) (*types.VersionResponse, error) {
	var result types.VersionResponse
	err := c.doRequest(ctx, "GET", "/version", nil, &result)
	return &result, err
}

// GetServices gets service list - uses strict dto format
func (c *AOGClient) GetServices(ctx context.Context, req types.GetAIGCServicesRequest) (*types.GetAIGCServicesResponse, error) {
	path := c.getAPIPath("/service")
	if req.ServiceName != "" {
		params := url.Values{}
		params.Add("service_name", req.ServiceName)
		path += "?" + params.Encode()
	}

	var response types.GetAIGCServicesResponse
	err := c.doRequest(ctx, "GET", path, nil, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// GetServiceProviders gets service providers - uses strict dto format
func (c *AOGClient) GetServiceProviders(ctx context.Context, req types.GetServiceProvidersRequest) (*types.GetServiceProvidersResponse, error) {
	path := c.getAPIPath("/service_provider")
	params := url.Values{}
	if req.ServiceName != "" {
		params.Add("service_name", req.ServiceName)
	}
	if req.ProviderName != "" {
		params.Add("provider_name", req.ProviderName)
	}
	if req.ServiceSource != "" {
		params.Add("service_source", req.ServiceSource)
	}
	if req.ApiFlavor != "" {
		params.Add("api_flavor", req.ApiFlavor)
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var response types.GetServiceProvidersResponse
	err := c.doRequest(ctx, "GET", path, nil, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// GetModels gets model list - uses strict dto format
func (c *AOGClient) GetModels(ctx context.Context, req types.GetModelsRequest) (*types.GetModelsResponse, error) {
	path := c.getAPIPath("/model")
	params := url.Values{}
	if req.ProviderName != "" {
		params.Add("provider_name", req.ProviderName)
	}
	if req.ModelName != "" {
		params.Add("model_name", req.ModelName)
	}
	if req.ServiceName != "" {
		params.Add("service_name", req.ServiceName)
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var response types.GetModelsResponse
	err := c.doRequest(ctx, "GET", path, nil, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// GetRecommendedModels gets recommended models - uses strict dto format
func (c *AOGClient) GetRecommendedModels(ctx context.Context) (*types.RecommendModelResponse, error) {
	path := c.getAPIPath("/model/recommend")

	var response types.RecommendModelResponse
	err := c.doRequest(ctx, "GET", path, nil, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// GetSupportedModels gets supported models - uses strict dto format
func (c *AOGClient) GetSupportedModels(ctx context.Context, req types.GetSupportModelRequest) (*types.SupportModelList, error) {
	path := c.getAPIPath("/model/support")
	params := url.Values{}
	params.Add("service_source", req.ServiceSource)
	params.Add("flavor", req.Flavor)
	if req.ServiceName != "" {
		params.Add("service_name", req.ServiceName)
	}
	if req.SearchName != "" {
		params.Add("search_name", req.SearchName)
	}
	if req.PageSize > 0 {
		params.Add("page_size", fmt.Sprintf("%d", req.PageSize))
	}
	if req.Page > 0 {
		params.Add("page", fmt.Sprintf("%d", req.Page))
	}
	params.Add("mine", fmt.Sprintf("%t", req.Mine))
	path += "?" + params.Encode()

	var response types.GetSupportModelResponse
	err := c.doRequest(ctx, "GET", path, nil, &response)
	if err != nil {
		return nil, err
	}

	supportModelList := types.SupportModelList{}
	for _, v := range response.Data.Data {
		supportModelList.Data = append(supportModelList.Data, types.ModelBaseInfo{
			ModelName:     v.Name,
			Avatar:        v.Avatar,
			ProviderName:  v.ServiceProvider,
			Status:        v.Status,
			ServiceName:   v.Service,
			ServiceSource: v.Source,
			IsDefault:     v.IsDefault,
		})
	}

	return &supportModelList, nil
}

// InstallModel installs model - uses strict dto format
func (c *AOGClient) InstallModel(ctx context.Context, req types.CreateModelRequest) (*types.CreateModelResponse, error) {
	path := c.getAPIPath("/model")

	var response types.CreateModelResponse
	err := c.doRequest(ctx, "POST", path, req, &response)
	return &response, err
}

// Chat chat service
func (c *AOGClient) Chat(ctx context.Context, req types.ChatRequest) (*map[string]interface{}, error) {
	path := c.getAPIPath("/services/chat")

	var result map[string]interface{}
	err := c.doRequest(ctx, "POST", path, req, &result)
	return &result, err
}

// TextToImage text-to-image service
func (c *AOGClient) TextToImage(ctx context.Context, req types.TextToImageRequest) (*types.TextToImageResponse, error) {
	path := c.getAPIPath("/services/text-to-image")

	var result types.TextToImageResponse
	err := c.doRequest(ctx, "POST", path, req, &result)
	return &result, err
}

// SpeechToText speech-to-text service
func (c *AOGClient) SpeechToText(ctx context.Context, req types.SpeechToTextRequest) (*types.SpeechToTextResponse, error) {
	path := c.getAPIPath("/services/speech-to-text")

	var result types.SpeechToTextResponse
	err := c.doRequest(ctx, "POST", path, req, &result)
	return &result, err
}

// Embed text embedding service
func (c *AOGClient) Embed(ctx context.Context, req types.EmbedRequest) (*types.EmbedResponse, error) {
	path := c.getAPIPath("/services/embed")

	var result types.EmbedResponse
	err := c.doRequest(ctx, "POST", path, req, &result)
	return &result, err
}
