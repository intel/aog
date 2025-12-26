# OVMS Plugin (External OpenVINO Model Server Plugin)

External plugin implementation for OpenVINO Model Server (OVMS) integration with AOG.

## 🎯 Project Status

### ✅ Completed
- [x] Project structure initialized
- [x] `plugin.yaml` - Complete service definitions (8 services)
- [x] `main.go` - Plugin entry point
- [x] `internal/config.go` - Configuration loading
- [x] `internal/provider.go` - Provider skeleton with all interface methods
- [x] `internal/utils/` - File operations and platform detection utilities
- [x] `internal/constants.go` - OVMS constants and download URLs
- [x] `Makefile` - Cross-platform build system

### 🔄 In Progress
- [ ] `internal/engine.go` - Engine lifecycle management (GetConfig, StartEngine, StopEngine, HealthCheck, GetVersion)
- [ ] `internal/installer.go` - Installation logic (InstallEngine, CheckEngine, platform detection)
- [ ] `internal/models.go` - Model management (ModelScope download, hash verification, graph.pbtxt generation)
- [ ] `internal/graphpb.go` - Graph templates and OVMS config.json management
- [ ] `internal/services/` - Service handlers (8 services: chat, generate, embed, rerank, text-to-image, STT, TTS, STT-WS)

### 📋 Pending
- [ ] Build and test the plugin locally
- [ ] Verify plugin discovery by AOG
- [ ] Integration testing with OVMS

## 📦 Services Supported

The plugin provides the following services, aligning with built-in OpenVINO capabilities:

1. **chat** - Chat completions (streaming supported)
2. **generate** - Text generation (streaming supported)
3. **embed** - Text embeddings
4. **rerank** - Document reranking
5. **text-to-image** - Image generation (Stable Diffusion)
6. **speech-to-text** - Speech recognition (Whisper)
7. **speech-to-text-ws** - Speech recognition with WebSocket (bidirectional)
8. **text-to-speech** - Text-to-speech synthesis

All services use `config_ref: openvino:xxx` to reuse the built-in OpenVINO protocol conversion rules from `internal/provider/template/openvino.yaml`.

### ⏱️ Timeout Configuration

Services are configured with appropriate timeout settings in `plugin.yaml`:

- **Fast inference services** (chat, generate, embed, rerank): Use default 60s timeout
- **Time-consuming services** (text-to-image): `timeout: -1` (no timeout limit)
- **Model download** (PullModel): No timeout limit (handled by context)

**Why no timeout for certain services?**
- Image generation can take several minutes depending on model and parameters
- Model downloads can take tens of minutes for large models
- Setting `timeout: -1` in `plugin.yaml` tells AOG to use `context.WithCancel` instead of `context.WithTimeout`

**Example configuration:**
```yaml
services:
  - service_name: text-to-image
    timeout: -1  # No timeout for image generation
```

See the [AOG Plugin Development Guide](../../docs/zh-cn/source/aog插件开发指南.rst) for detailed timeout configuration documentation.

## 🏗️ Architecture

```
ovms-plugin/
├── main.go                    # Plugin entry point
├── plugin.yaml               # Plugin manifest
├── go.mod / go.sum           # Go modules
├── Makefile                  # Build system
├── internal/
│   ├── config.go             # Config loading & parsing
│   ├── provider.go           # OvmsProvider (LocalPluginProvider impl)
│   ├── constants.go          # Constants & download URLs
│   ├── engine.go             # Engine lifecycle [TODO]
│   ├── installer.go          # Installation logic [TODO]
│   ├── models.go             # ModelScope download & management [TODO]
│   ├── graphpb.go            # Graph.pbtxt templates [TODO]
│   ├── services/             # Service handlers [TODO]
│   └── utils/
│       ├── file.go           # File operations
│       └── platform.go       # Platform detection
└── bin/                      # Cross-platform binaries (generated)
    ├── linux-amd64/
    ├── linux-arm64/
    ├── darwin-amd64/
    ├── darwin-arm64/
    └── windows-amd64/
```

## 🔧 Build Instructions

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Clean build artifacts
make clean

# Verify builds
make verify
```

## 📝 Implementation Plan

### Phase 1: Engine Lifecycle (Current)
- Migrate `GetConfig` logic from built-in OpenVINO
- Implement platform-specific configuration (Windows/Linux/macOS)
- Implement `StartEngine/StopEngine` with process management
- Implement `HealthCheck` (HTTP/GRPC)
- Implement `GetVersion` (parse OVMS version output)

### Phase 2: Installation & Setup
- Migrate `InstallEngine` logic
- Platform-specific download URL selection
- OVMS binary + scripts download and extraction
- Installation script execution (Windows .bat / Linux .sh)
- Linux distribution detection (Ubuntu 22.04/24.04/Deepin)

### Phase 3: Model Management
- ModelScope API integration
- Model file download with resume support
- SHA256 hash verification
- graph.pbtxt generation for different model types
- OVMS config.json management
- Implement all ModelManager interface methods

### Phase 4: Service Implementation
- Create service handlers for all 8 services
- Implement unary and streaming call patterns
- HTTP/GRPC client for OVMS communication
- Request/response marshaling

### Phase 5: Testing & Integration
- Local plugin testing
- AOG plugin discovery verification
- End-to-end service calls
- Performance testing

## 🔗 Relationship with Built-in OpenVINO

- **Built-in**: `internal/provider/engine/openvino.go` + `internal/provider/template/openvino.yaml`
- **Plugin**: `plugin-example/ovms-plugin` (this project)

Both implementations provide the same capabilities but:
- Built-in is called directly by AOG core
- Plugin is called via gRPC through plugin-sdk
- They are **independent** and can coexist
- Users can choose which one to use

## 📚 References

- Plugin SDK: `../../plugin-sdk/`
- Development Guide: `../../plugin-sdk/ENGINE_PLUGIN_DEVELOPMENT_GUIDE.md`
- Example Plugin: `../ollama-plugin/`
- Built-in OpenVINO: `../../internal/provider/engine/openvino.go`

## 🎯 Next Steps

To continue implementation:

1. Complete `internal/engine.go` with engine lifecycle methods
2. Complete `internal/installer.go` with installation logic
3. Port ModelScope download logic to `internal/models.go`
4. Create graph.pbtxt templates in `internal/graphpb.go`
5. Implement service handlers in `internal/services/`
6. Build and test the plugin

---

**Note**: This plugin replicates all functionalities of the built-in OpenVINO engine to ensure feature parity and provide users with a choice between built-in and plugin-based deployment.
