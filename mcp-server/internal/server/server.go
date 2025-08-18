// Package server implements AOG MCP server
package server

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aog/mcp-server/internal/client"
	"github.com/aog/mcp-server/internal/tools"
	"github.com/aog/mcp-server/internal/types"
)

// AOGMCPServer AOG MCP server
type AOGMCPServer struct {
	server       *mcp.Server
	aogClient    *client.AOGClient
	toolHandlers *tools.ToolHandlers
}

// NewAOGMCPServer creates a new AOG MCP server
func NewAOGMCPServer(config types.AOGConfig) (*AOGMCPServer, error) {
	// Create AOG client
	aogClient := client.NewAOGClient(config)

	// Create tool handlers
	toolHandlers := tools.NewToolHandlers(aogClient)

	// Create MCP server
	impl := &mcp.Implementation{
		Name:    "aog-mcp-server",
		Version: "1.0.0",
	}

	opts := &mcp.ServerOptions{
		Instructions: "AOG MCP Server - Provides MCP protocol support for AOG (AIPC Open Gateway), enabling AI models to use all AOG AI service capabilities",
	}

	server := mcp.NewServer(impl, opts)

	mcpServer := &AOGMCPServer{
		server:       server,
		aogClient:    aogClient,
		toolHandlers: toolHandlers,
	}

	// Register all tools
	mcpServer.registerTools()

	return mcpServer, nil
}

// registerTools registers all MCP tools
func (s *AOGMCPServer) registerTools() {
	// Service discovery and management
	s.server.AddTool(tools.GetServicesSchema, s.createToolHandler(s.toolHandlers.HandleGetServices))
	s.server.AddTool(tools.GetServiceProvidersSchema, s.createToolHandler(s.toolHandlers.HandleGetServiceProviders))

	// Model management
	s.server.AddTool(tools.GetModelsSchema, s.createToolHandler(s.toolHandlers.HandleGetModels))
	s.server.AddTool(tools.GetRecommendedModelsSchema, s.createToolHandler(s.toolHandlers.HandleGetRecommendedModels))
	s.server.AddTool(tools.GetSupportedModelsSchema, s.createToolHandler(s.toolHandlers.HandleGetSupportedModels))
	s.server.AddTool(tools.InstallModelSchema, s.createToolHandler(s.toolHandlers.HandleInstallModel))

	// AI service calls
	s.server.AddTool(tools.ChatSchema, s.createToolHandler(s.toolHandlers.HandleChat))
	s.server.AddTool(tools.TextToImageSchema, s.createToolHandler(s.toolHandlers.HandleTextToImage))
	s.server.AddTool(tools.SpeechToTextSchema, s.createToolHandler(s.toolHandlers.HandleSpeechToText))
	s.server.AddTool(tools.EmbedSchema, s.createToolHandler(s.toolHandlers.HandleEmbed))

	// System status
	s.server.AddTool(tools.HealthCheckSchema, s.createToolHandler(s.toolHandlers.HandleHealthCheck))
	s.server.AddTool(tools.GetVersionSchema, s.createToolHandler(s.toolHandlers.HandleGetVersion))
}

// createToolHandler creates adapter function to convert our handler to the format expected by MCP SDK
func (s *AOGMCPServer) createToolHandler(handler func(context.Context, map[string]interface{}) (*mcp.CallToolResult, error)) mcp.ToolHandler {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[map[string]interface{}]) (*mcp.CallToolResult, error) {
		return handler(ctx, params.Arguments)
	}
}

// CheckAOGConnection checks AOG service connection
func (s *AOGMCPServer) CheckAOGConnection(ctx context.Context) error {
	_, err := s.aogClient.HealthCheck(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to AOG service: %w", err)
	}
	return nil
}

// Run runs the MCP server
func (s *AOGMCPServer) Run(ctx context.Context, transport mcp.Transport) error {
	// First check AOG service connection
	if err := s.CheckAOGConnection(ctx); err != nil {
		log.Printf("❌ %v", err)
		log.Printf("Please ensure AOG service is running: aog server start")
		return err
	}
	log.Printf("✅ AOG service connection successful")

	// Start MCP server
	log.Printf("🚀 AOG MCP Server started successfully")
	return s.server.Run(ctx, transport)
}

// Connect connects to client
func (s *AOGMCPServer) Connect(ctx context.Context, transport mcp.Transport) (*mcp.ServerSession, error) {
	// First check AOG service connection
	if err := s.CheckAOGConnection(ctx); err != nil {
		return nil, fmt.Errorf("AOG service unavailable: %w", err)
	}

	// Connect to client
	session, err := s.server.Connect(ctx, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to client: %w", err)
	}

	log.Printf("✅ Client connection successful, session ID: %s", session.ID())
	return session, nil
}
