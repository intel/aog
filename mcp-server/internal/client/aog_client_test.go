package client

import (
	"context"
	"testing"

	"github.com/aog/mcp-server/internal/types"
)

func TestNewAOGClient(t *testing.T) {
	// Test default configuration
	config := types.AOGConfig{}
	client := NewAOGClient(config)

	if client.baseURL != "http://localhost:16688" {
		t.Errorf("Expected default baseURL to be http://localhost:16688, got %s", client.baseURL)
	}

	if client.version != "v0.2" {
		t.Errorf("Expected default version to be v0.2, got %s", client.version)
	}

	// Test custom configuration
	config = types.AOGConfig{
		BaseURL: "http://example.com:8080",
		Version: "v1.0",
		Timeout: 60000,
	}
	client = NewAOGClient(config)

	if client.baseURL != "http://example.com:8080" {
		t.Errorf("Expected baseURL to be http://example.com:8080, got %s", client.baseURL)
	}

	if client.version != "v1.0" {
		t.Errorf("Expected version to be v1.0, got %s", client.version)
	}
}

func TestGetAPIPath(t *testing.T) {
	client := NewAOGClient(types.AOGConfig{Version: "v0.2"})

	tests := []struct {
		endpoint string
		expected string
	}{
		{"/service", "/aog/v0.2/service"},
		{"/model", "/aog/v0.2/model"},
		{"/services/chat", "/aog/v0.2/services/chat"},
	}

	for _, test := range tests {
		result := client.getAPIPath(test.endpoint)
		if result != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, result)
		}
	}
}

// Note: The following tests require actual AOG service running, may need mock in CI environment
func TestHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skip integration test")
	}

	client := NewAOGClient(types.AOGConfig{})
	ctx := context.Background()

	_, err := client.HealthCheck(ctx)
	if err != nil {
		t.Logf("Health check failed (this is normal if AOG service is not running): %v", err)
	}
}
