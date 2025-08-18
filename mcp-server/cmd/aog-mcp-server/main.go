// Package main AOG MCP Server main program
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/aog/mcp-server/internal/server"
	"github.com/aog/mcp-server/internal/types"
)

var (
	baseURL string
	timeout int
)

// rootCmd root command
var rootCmd = &cobra.Command{
	Use:   "aog-mcp-server",
	Short: "AOG MCP Server - Provides MCP protocol support for AOG",
	Long: `AOG MCP Server

Provides MCP (Model Context Protocol) services for AOG (AIPC Open Gateway),
enabling AI models to use all AOG AI service capabilities through standardized tool calls.

Supported AI Services:
- Chat Service: Multi-turn conversations, streaming output
- Text-to-Image Service: Generate images from text descriptions
- Speech-to-Text Service: Audio to text conversion
- Text Embedding Service: Generate text vector representations

Features:
- Complete model management (discovery, installation, usage)
- Service discovery and status monitoring
- Intelligent scheduling strategy support
- Type-safe Go implementation`,
	Run: runServer,
}

// versionCmd version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("AOG MCP Server v1.0.0")
		fmt.Println("Built with Go MCP SDK v0.2.0")
		fmt.Println("Copyright (c) 2025 AOG Team")
	},
}

// healthCmd health check command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check AOG service health status",
	Run: func(cmd *cobra.Command, args []string) {
		config := types.AOGConfig{
			BaseURL: baseURL,
			Version: "v0.2", // Fixed to v0.2 spec version
			Timeout: timeout,
		}

		mcpServer, err := server.NewAOGMCPServer(config)
		if err != nil {
			log.Fatalf("❌ Failed to create server: %v", err)
		}

		ctx := context.Background()
		if err := mcpServer.CheckAOGConnection(ctx); err != nil {
			log.Fatalf("❌ AOG service health check failed: %v", err)
		}

		fmt.Println("✅ AOG service is running normally")
	},
}

func init() {
	// Add global flags
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "http://localhost:16688", "Base URL of AOG service")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 120000, "Request timeout in milliseconds, default 2 minutes, suitable for time-consuming services like text-to-image")

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(healthCmd)

	// Set environment variable support
	if envBaseURL := os.Getenv("AOG_BASE_URL"); envBaseURL != "" {
		baseURL = envBaseURL
	}
	if envTimeout := os.Getenv("AOG_TIMEOUT"); envTimeout != "" {
		if t, err := strconv.Atoi(envTimeout); err == nil {
			timeout = t
		}
	}
}

// runServer runs the server
func runServer(cmd *cobra.Command, args []string) {
	// Create configuration
	config := types.AOGConfig{
		BaseURL: baseURL,
		Version: "v0.2", // Fixed to v0.2 spec version
		Timeout: timeout,
	}

	// Create MCP server
	mcpServer, err := server.NewAOGMCPServer(config)
	if err != nil {
		log.Fatalf("❌ Failed to create server: %v", err)
	}

	// Create context and signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for system signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("📡 Received signal %v, shutting down server...", sig)
		cancel()
	}()

	// Create stdio transport
	transport := mcp.NewStdioTransport()

	// Run server
	if err := mcpServer.Run(ctx, transport); err != nil {
		if ctx.Err() != nil {
			log.Printf("🛑 Server has been shut down")
		} else {
			log.Fatalf("❌ Server failed to run: %v", err)
		}
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("❌ Command execution failed: %v", err)
	}
}
