# AOG MCP Server (Go Version)

A Go language implementation that provides MCP (Model Context Protocol) services for AOG (AIPC Open Gateway), enabling AI models to use all AOG AI service capabilities through standardized tool calls.

## 🎯 Features

### Supported AI Services
- **Chat Service**: Multi-turn conversations, streaming output
- **Text-to-Image Service**: Generate images from text descriptions
- **Speech-to-Text Service**: Audio to text conversion
- **Text Embedding Service**: Generate text vector representations

### Model Management
- Get installed model list
- Get recommended models
- Get supported models
- Install new models

### Service Discovery
- Get available service list
- Get service provider information
- Health check and version information

### Technical Features
- **Type Safety**: Complete Go type definitions and compile-time checking
- **High Performance**: Go native performance, low memory usage
- **Concurrency Safe**: Support for multi-client concurrent connections
- **Easy Deployment**: Single binary file, no external dependencies

## 🚀 Quick Start

### Prerequisites

1. **Go 1.23+**
2. **AOG Service Running** (default port 16688)

### Installation and Build

```bash
# Clone project
git clone <repository-url>
cd aog-mcp-server-go

# Download dependencies
go mod tidy

# Build
go build -o aog-mcp-server ./cmd/aog-mcp-server/main.go

# Or run directly
go run ./cmd/aog-mcp-server/main.go
```

### Usage

#### Basic Usage

```bash
# Start with default configuration
./aog-mcp-server

# Start with custom configuration
./aog-mcp-server --base-url http://localhost:16688 --timeout 120000
```

#### Health Check

```bash
# Check AOG service status
./aog-mcp-server health
```

#### View Version

```bash
# Show version information
./aog-mcp-server version
```

#### Command Line Options

```bash
--base-url string   Base URL of AOG service (default: http://localhost:16688)
--timeout int       Request timeout in milliseconds (default: 120000, 2 minutes, suitable for time-consuming services like text-to-image)
--help             Show help information
```

**Note**: API version is now fixed to v0.2 spec version and no longer configurable.

#### 🧪 Test Functions

The project includes complete test scripts to verify basic functionality:

```bash
# Manual testing of various functions
./mcp version          # Version information
./mcp health           # Health check (requires AOG service running)
./mcp --help           # Help information
```

#### Environment Variable Support

```bash
export AOG_BASE_URL=http://localhost:16688
export AOG_TIMEOUT=120000  # 2-minute timeout, suitable for time-consuming services like text-to-image
./aog-mcp-server
```

**Note**: `AOG_VERSION` environment variable is no longer supported. The API version is fixed to v0.2.

## 🛠️ MCP Tool List

### 1. Service Discovery and Management

#### `aog_get_services`
Get all available AI service lists in AOG

```json
{
  "service_name": "chat" // Optional: Specify service name
}
```

#### `aog_get_service_providers`
Get service provider information for specified services

```json
{
  "service_name": "chat",           // Optional: Service name
  "provider_name": "local_ollama",  // Optional: Provider name
  "service_source": "local"         // Optional: local/remote
}
```

### 2. Model Management

#### `aog_get_models`
Get list of installed models

```json
{
  "provider_name": "local_ollama_chat", // Optional: Provider name
  "service_name": "chat"                // Optional: Service type
}
```

#### `aog_get_recommended_models`
Get AOG recommended model list

#### `aog_get_supported_models`
Get list of models supported by specified service provider

```json
{
  "service_source": "local",  // Required: local/remote
  "flavor": "ollama"          // Required: API flavor
}
```

#### `aog_install_model`
Install specified AI model

```json
{
  "model_name": "deepseek-r1:7b",    // Required: Model name
  "service_name": "chat",            // Required: Service name
  "service_source": "local",         // Required: local/remote
  "provider_name": "local_ollama"    // Optional: Provider name
}
```

### 3. AI Service Calls

> **⚠️ Important Note**: The `model` parameter in all model services must exactly match the `model_name` in the model list (obtainable via `aog_get_models`), otherwise a model not found error will occur. If not specified or empty, the model parameter will not be passed and the service default model will be used.

#### `aog_chat`
Use AOG's chat service for conversations

```json
{
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "model": "qwen2.5:7b",              // Optional: Model name, must exactly match model_name
  "temperature": 0.7,                  // Optional: Temperature parameter
  "max_tokens": 1000,                  // Optional: Maximum token count
  "stream": false                      // Optional: Streaming output
}
```

#### `aog_text_to_image`
Use AOG's text-to-image service to generate images

```json
{
  "prompt": "A cute cat",                                    // Required: Image description
  "model": "OpenVINO/stable-diffusion-v1-5-fp16-ov",        // Optional: Model name, must exactly match model_name
  "n": 1,                                                   // Optional: Generation count
  "size": "1024*1024",                                      // Optional: Image size
  "seed": 12345                                             // Optional: Random seed
}
```

#### `aog_speech_to_text`
Use AOG's speech-to-text service

```json
{
  "audio_file": "/path/to/audio.wav",           // Required: Audio file path or base64
  "model": "NamoLi/whisper-large-v3-ov",       // Optional: Model name, must exactly match model_name
  "language": "en"                             // Optional: Language
}
```

#### `aog_embed`
Use AOG's embedding service to generate text vector representations, returns complete embedding vector array

```json
{
  "input": "Text to be embedded",                      // Required: Text content
  "model": "bge-m3:latest"                     // Optional: Model name, must exactly match model_name
}
```

**Return Format**:
```json
{
  "success": true,
  "data": {
    "data": [
      {
        "embedding": [-0.012523372, 0.03742505, ...], // Complete vector array
        "index": 0
      }
    ],
    "model": "bge-m3:latest"
  },
  "message": "Text embedding successful"
}
```

> **💡 Application Scenarios**: The returned vector data can be used for semantic search, text similarity calculation, recommendation systems, text clustering and other AI tasks.

### 4. System Status

#### `aog_health_check`
Check AOG service health status

#### `aog_get_version`
Get AOG version information

## 📋 Claude Desktop Integration

### Configuration File

Create or edit Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "aog": {
      "command": "/path/to/aog-mcp-server",
      "args": [],
      "env": {
        "AOG_BASE_URL": "http://localhost:16688"
      }
    }
  }
}
```

### Usage Example

After starting Claude Desktop, you can use it like this:

```
User: Please check AOG service status and tell me what AI services are available

Claude will automatically call:
1. aog_health_check() - Check service status
2. aog_get_services() - Get service list
```

## 📦 Building Desktop Extension (DXT)

AOG MCP Server supports the [Desktop Extension (DXT)](https://github.com/anthropics/dxt) format for easy distribution and one-click installation in desktop applications like Claude for macOS and Windows.

### What is DXT?

Desktop Extensions (`.dxt`) are zip archives containing a local MCP server and a `manifest.json` that describes the server and its capabilities. They enable end users to install local MCP servers with a single click, similar to Chrome extensions (`.crx`) or VS Code extensions (`.vsix`).

### Prerequisites for DXT Build

1. **DXT CLI Tool**: Install the official DXT CLI tool
   ```bash
   npm install -g @anthropic-ai/dxt
   ```

2. **Cross-platform Binaries**: Build binaries for different platforms
   ```bash
   # Build for different platforms
   GOOS=darwin GOARCH=amd64 go build -o aog-mcp-server-darwin-amd64 ./cmd/aog-mcp-server
   GOOS=darwin GOARCH=arm64 go build -o aog-mcp-server-darwin-arm64 ./cmd/aog-mcp-server  
   GOOS=windows GOARCH=amd64 go build -o aog-mcp-server-windows-amd64.exe ./cmd/aog-mcp-server
   ```

### Building DXT Package

#### Method 1: Using DXT CLI (Recommended)

```bash
# Initialize DXT configuration (if manifest.json needs updates)
dxt init

# Pack the extension
dxt pack
```

#### Method 2: Manual Build Process

1. **Prepare Directory Structure**:
   ```
   aog-mcp-server.dxt/
   ├── manifest.json              # Extension metadata
   ├── aog-mcp-server             # macOS binary
   ├── aog-mcp-server.exe         # Windows binary
   ├── README.md                  # Documentation
   └── LICENSE                    # License file
   ```

2. **Create the DXT Archive**:
   ```bash
   # Create a temporary directory
   mkdir -p build/dxt-package
   
   # Copy required files
   cp manifest.json build/dxt-package/
   cp aog-mcp-server build/dxt-package/
   cp aog-mcp-server.exe build/dxt-package/ 2>/dev/null || true
   cp README.md build/dxt-package/
   cp LICENSE build/dxt-package/
   
   # Create DXT archive
   cd build/dxt-package
   zip -r ../aog-mcp-server.dxt .
   ```

### Configuration Variables

The DXT package supports environment variable configuration:

- **AOG_BASE_URL**: Base URL of AOG service (default: `http://localhost:16688`)
- **AOG_TIMEOUT**: Request timeout in milliseconds (default: `120000`)

Users can configure these variables in their desktop application's MCP server settings.

### Distribution and Installation

#### For End Users

1. **Download** the `.dxt` file from releases
2. **Double-click** the `.dxt` file or open with Claude for macOS/Windows
3. **Follow** the installation dialog to configure AOG connection settings
4. **Start using** AOG tools in your AI desktop application

#### For Developers

1. **Build** the DXT package using the methods above
2. **Test** the package by installing it in a compatible application
3. **Distribute** via GitHub releases, websites, or extension directories

### Extension Capabilities

The DXT package includes the following MCP tools:

**Service Discovery & Management:**
- `aog_get_services` - Get available AI services
- `aog_get_service_providers` - Get service provider information
- `aog_health_check` - Check service health status
- `aog_get_version` - Get version information

**Model Management:**
- `aog_get_models` - Get installed models
- `aog_get_recommended_models` - Get recommended models  
- `aog_get_supported_models` - Get supported models
- `aog_install_model` - Install new models

**AI Services:**
- `aog_chat` - Multi-turn conversations
- `aog_text_to_image` - Generate images from text
- `aog_speech_to_text` - Convert audio to text
- `aog_embed` - Generate text embeddings

### Troubleshooting DXT

**Common Issues:**

1. **Installation Failed**
   ```bash
   # Verify DXT file integrity
   unzip -t aog-mcp-server.dxt
   
   # Check manifest.json syntax
   cat manifest.json | jq .
   ```

2. **Binary Not Found**
   ```bash
   # Ensure binaries have correct permissions
   chmod +x aog-mcp-server
   
   # Check platform compatibility
   file aog-mcp-server
   ```

3. **AOG Connection Issues**
   - Verify AOG service is running on configured URL
   - Check firewall settings for port 16688
   - Test connection with: `curl http://localhost:16688/health`

## 🔧 Development

### Project Structure

```
aog-mcp-server-go/
├── cmd/
│   └── aog-mcp-server/
│       └── main.go           # Main program entry
├── internal/
│   ├── client/
│   │   └── aog_client.go     # AOG HTTP client
│   ├── server/
│   │   └── server.go         # MCP server implementation
│   ├── tools/
│   │   ├── schemas.go        # Tool definitions and schemas
│   │   └── handlers.go       # Tool handler implementations
│   └── types/
│       └── types.go          # Type definitions
├── go.mod
├── go.sum
└── README.md
```

### Development Commands

```bash
# Run development version
go run ./cmd/aog-mcp-server

# Run tests
go test ./...

# Code formatting
go fmt ./...

# Static analysis
go vet ./...

# Build release version
go build -ldflags="-s -w" -o aog-mcp-server ./cmd/aog-mcp-server
```

### Adding New Tools

1. Define tool schema in `internal/tools/schemas.go`
2. Implement handler in `internal/tools/handlers.go`
3. Register tool in `internal/server/server.go`

## 🐛 Troubleshooting

### Common Issues

1. **Unable to connect to AOG service**
   ```bash
   # Check AOG service status
   ./aog-mcp-server health

   # Confirm AOG service is running
   aog server start

   # Check if port is correct
   netstat -an | grep 16688
   ```

2. **Tool call failed**
   ```bash
   # Check parameter format
   # Confirm required parameters are provided
   # Check detailed description in error message
   ```

3. **Model installation failed**
   ```bash
   # Check network connection
   # Confirm sufficient disk space
   # Check AOG logs: aog server start -v
   ```

## 📄 License

Apache License 2.0

## 🤝 Contributing

Welcome to submit Issues and Pull Requests!

## 📞 Support

If you have questions, please:
1. Check the troubleshooting section of this documentation
2. Check AOG official documentation
3. Submit GitHub Issue
