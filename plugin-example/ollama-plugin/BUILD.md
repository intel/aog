# Ollama Plugin - Cross-Platform Build Guide

## 📁 Directory Structure

```
ollama-plugin/
├── main.go                    # Plugin entry point
├── plugin.yaml               # Plugin manifest with platform declarations
├── internal/                 # Modular implementation
│   ├── provider.go           # Core service router
│   ├── engine.go             # Engine lifecycle management
│   ├── installer.go          # Engine installation management
│   ├── models.go             # Model management
│   ├── client.go             # HTTP client
│   ├── config.go             # Configuration management
│   └── services/             # Service implementations
│       ├── common.go         # Common interfaces and types
│       ├── chat.go           # Chat service (streaming + non-streaming)
│       ├── embed.go          # Embedding service (non-streaming)
│       └── generate.go       # Generate service (streaming + non-streaming)
├── bin/                      # Cross-platform binaries
│   ├── linux-amd64/
│   ├── linux-arm64/
│   ├── darwin-amd64/
│   ├── darwin-arm64/
│   └── windows-amd64/
├── Makefile                  # Advanced build system
├── build-all.sh              # Simple cross-platform build script
└── verify-structure.sh       # Structure verification tool
```

## 🚀 Building

### Quick Start
```bash
# Build for current platform
make build

# Build for all platforms
./build-all.sh

# Or using Makefile
make build-all
```

### Supported Platforms
- **Linux**: amd64, arm64
- **macOS**: amd64 (Intel), arm64 (Apple Silicon)
- **Windows**: amd64

### Build Commands

#### Using Shell Script (Recommended)
```bash
# Build all platforms
./build-all.sh

# With custom version
VERSION=1.2.0 ./build-all.sh
```

#### Using Makefile
```bash
# Build all platforms
make build-all

# Build specific platform
make linux-amd64
make darwin-arm64
make windows-amd64

# Clean build artifacts
make clean

# Verify build artifacts
make verify

# Create distribution package
make package
```

### Development Workflow
```bash
# 1. Develop and test locally
make build

# 2. Verify code structure
./verify-structure.sh

# 3. Build for all platforms
make build-all

# 4. Verify all builds
make verify

# 5. Package for distribution
make package
```

## 📦 Distribution

### For Plugin Developers
1. Run `make build-all` to create binaries for all platforms
2. Verify with `make verify`
3. Create package with `make package`
4. Distribute the complete plugin directory with `bin/` folder

### For End Users
1. Download and extract the plugin package
2. Place in AOG's `plugins/` directory
3. AOG will automatically select the correct platform binary based on `plugin.yaml`

## 🔧 Platform Selection

The plugin manifest (`plugin.yaml`) declares platform-specific executables:

```yaml
platforms:
  linux_amd64:
    executable: bin/linux-amd64/ollama-plugin
  darwin_arm64:
    executable: bin/darwin-arm64/ollama-plugin
  windows_amd64:
    executable: bin/windows-amd64/ollama-plugin.exe
  # ... other platforms
```

AOG's `PluginRegistry` automatically selects the appropriate binary based on the runtime platform.

## 🛠️ Services

The plugin provides three services:

### 1. Chat Service
- **Endpoint**: `/api/chat`
- **Streaming**: ✅ Supported
- **Protocol**: HTTP

### 2. Embedding Service
- **Endpoint**: `/api/embeddings`
- **Streaming**: ❌ Not supported
- **Protocol**: HTTP

### 3. Generate Service (New)
- **Endpoint**: `/api/generate`
- **Streaming**: ✅ Supported
- **Protocol**: HTTP
- **Difference**: Uses `prompt` instead of `messages`

## 📋 Verification

Run the verification script to check the plugin structure:

```bash
./verify-structure.sh
```

This will verify:
- ✅ Directory structure
- ✅ Plugin configuration
- ✅ Build tools
- ✅ Code structure
- ✅ Service implementations

## 🔍 Troubleshooting

### Build Issues
- **CGO Linking Errors**: These are environment-specific dependency issues, not code problems
- **Platform Missing**: Ensure the target platform is listed in `plugin.yaml`
- **Permission Denied**: Run `chmod +x build-all.sh` to make the script executable

### Runtime Issues
- **Plugin Not Found**: Verify the `bin/` directory contains the correct platform binary
- **Wrong Platform**: Check that `plugin.yaml` platform configuration matches your binary names

## 🎯 Best Practices

1. **Always build all platforms** before distribution
2. **Verify structure** with `./verify-structure.sh`
3. **Test locally** with current platform build first
4. **Version binaries** using the VERSION environment variable
5. **Package complete** directory including `bin/` folder for distribution
