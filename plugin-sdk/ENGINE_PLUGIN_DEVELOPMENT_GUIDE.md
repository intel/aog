# AOG Engine 插件开发指南

> **基于实际代码的完整插件开发指南**  
> 参考示例：`ollama-plugin`（本地插件）和 `aliyun-plugin`（远程插件）

---

## 📖 目录

- [1. 概述](#1-概述)
- [2. 核心概念](#2-核心概念)
- [3. 快速开始](#3-快速开始)
- [4. 插件架构](#4-插件架构)
- [5. 核心接口实现](#5-核心接口实现)
- [6. 服务开发](#6-服务开发)
- [7. 配置管理](#7-配置管理)
- [8. 跨平台构建](#8-跨平台构建)
- [9. 部署和测试](#9-部署和测试)
- [10. 最佳实践](#10-最佳实践)
- [11. 常见问题](#11-常见问题)
- [附录](#附录)

---

## 1. 概述

### 1.1 什么是 AOG Engine 插件？

AOG Engine 插件是一种**可插拔的AI引擎扩展机制**，允许开发者将新的AI引擎（如Ollama、Aliyun、Deepseek等）集成到AOG生态系统中，而无需修改AOG核心代码。

### 1.2 插件系统特点

- ✅ **零依赖AOG内部包**：插件完全独立，不依赖 `github.com/intel/aog/internal/*`
- ✅ **独立编译分发**：无需AOG主程序代码即可开发和分发
- ✅ **跨平台支持**：支持Linux、macOS、Windows，支持amd64和arm64架构
- ✅ **标准化通信**：基于gRPC + hashicorp/go-plugin，接口清晰
- ✅ **适配器模式**：提供三层Adapter简化开发
- ✅ **自动发现**：AOG启动时自动发现并加载插件，无需手动安装

### 1.3 插件类型

| 插件类型 | 说明 | 典型场景 | 需要实现的接口 |
|---------|------|---------|---------------|
| **Local Plugin** | 管理本地AI引擎 | Ollama, OpenVINO, LlamaCpp | `LocalPluginProvider` |
| **Remote Plugin** | 对接云端AI服务 | Aliyun, Baidu, Deepseek | `RemotePluginProvider` |

本指南涵盖两种类型的插件开发，以 `ollama-plugin`（本地）和 `aliyun-plugin`（远程）为示例。

---

## 2. 核心概念

### 2.1 插件SDK架构

```
plugin-sdk/
├── protocol/           # gRPC协议定义
│   ├── provider.proto # Protocol Buffers定义
│   ├── provider.pb.go # 生成的protobuf代码
│   ├── provider_grpc.pb.go # 生成的gRPC代码
│   └── errors.go      # 错误定义
├── types/             # SDK类型定义
│   ├── metadata.go    # Plugin元数据
│   ├── service.go     # 服务相关类型
│   └── common.go      # 通用类型
├── adapter/           # 适配器实现
│   ├── base.go        # BasePluginProvider
│   ├── local.go       # LocalPluginAdapter
│   └── remote.go      # RemotePluginAdapter
├── server/            # gRPC Server实现
│   ├── provider.go    # ProviderPlugin
│   └── grpc_server.go # gRPC服务器
├── client/            # 客户端接口
│   ├── grpc_client.go # gRPC客户端
│   └── interfaces.go  # 接口定义
└── provider/          # Provider接口定义
    └── interface.go   # 统一的Provider接口
```

### 2.2 三层适配器模式

```
BasePluginProvider
├── 元数据管理
├── 状态管理
├── 日志记录
└── 错误处理

LocalPluginAdapter (继承 Base)
├── 引擎生命周期 (StartEngine, StopEngine)
├── 引擎安装管理 (InstallEngine, CheckEngine)
└── 模型管理 (PullModel, DeleteModel, ListModels)

YourProvider (继承 Local)
├── 覆盖需要的方法
├── 实现服务调用逻辑 (InvokeService)
└── 实现流式服务调用 (InvokeServiceStream)
```

### 2.3 核心接口

**重要**：以下接口定义来自 `plugin-sdk/client/interfaces.go`，这是实际使用的接口。

```go
// PluginProvider - 所有插件必须实现的基础接口
type PluginProvider interface {
    GetManifest() *types.PluginManifest
    GetOperateStatus() int
    SetOperateStatus(status int)
    HealthCheck(ctx context.Context) error
    // 注意：authInfo 参数用于远程插件的认证信息
    InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error)
}

// LocalPluginProvider - 本地引擎插件接口
type LocalPluginProvider interface {
    PluginProvider
    
    // 引擎生命周期
    StartEngine(mode string) error
    StopEngine() error
    GetConfig(ctx context.Context) (*types.EngineRecommendConfig, error)
    
    // 引擎安装
    CheckEngine() (bool, error)
    InstallEngine(ctx context.Context) error
    InitEnv() error
    UpgradeEngine(ctx context.Context) error
    
    // 模型管理
    PullModel(ctx context.Context, req *types.PullModelRequest, fn types.PullProgressFunc) (*types.ProgressResponse, error)
    PullModelStream(ctx context.Context, req *types.PullModelRequest) (chan []byte, chan error)
    DeleteModel(ctx context.Context, req *types.DeleteRequest) error
    ListModels(ctx context.Context) (*types.ListResponse, error)
    LoadModel(ctx context.Context, req *types.LoadRequest) error
    UnloadModel(ctx context.Context, req *types.UnloadModelRequest) error
    GetRunningModels(ctx context.Context) (*types.ListResponse, error)
    GetVersion(ctx context.Context, resp *types.EngineVersionResponse) (*types.EngineVersionResponse, error)
}

// RemotePluginProvider - 远程API插件接口
type RemotePluginProvider interface {
    PluginProvider
    
    SetAuth(req *http.Request, authInfo string, credentials map[string]string) error
    ValidateAuth(ctx context.Context) error
    RefreshAuth(ctx context.Context) error
}

// StreamablePlugin - 支持服务端流式的插件（可选）
type StreamablePlugin interface {
    PluginProvider
    InvokeServiceStream(ctx context.Context, serviceName string, authInfo string, request []byte) (<-chan StreamChunk, error)
}

// BidirectionalPlugin - 支持双向流式的插件（可选，如WebSocket）
type BidirectionalPlugin interface {
    PluginProvider
    InvokeServiceBidirectional(ctx context.Context, serviceName string, wsConnID string, authInfo string, inStream <-chan BidiMessage, outStream chan<- BidiMessage) error
}
```

---

## 3. 快速开始

### 3.1 创建插件项目

```bash
# 创建插件目录
mkdir my-engine-plugin
cd my-engine-plugin

# 初始化Go模块
go mod init github.com/yourusername/my-engine-plugin

# 添加SDK依赖
go get github.com/intel/aog/plugin-sdk
go get github.com/hashicorp/go-plugin
go get google.golang.org/grpc
go get gopkg.in/yaml.v3
```

### 3.2 项目结构

推荐的目录结构（参考 `ollama-plugin`）：

```
my-engine-plugin/
├── main.go              # 插件入口
├── plugin.yaml          # 插件元数据配置
├── go.mod               # Go模块定义
├── go.sum               # 依赖锁定
├── internal/            # 插件实现
│   ├── provider.go      # Provider实现
│   ├── engine.go        # 引擎生命周期管理
│   ├── installer.go     # 引擎安装管理
│   ├── models.go        # 模型管理
│   ├── client.go        # HTTP/gRPC客户端
│   ├── config.go        # 配置管理
│   └── services/        # 服务实现
│       ├── common.go    # 通用接口
│       ├── chat.go      # Chat服务
│       └── embed.go     # Embedding服务
├── bin/                 # 编译产物（跨平台）
│   ├── linux-amd64/
│   ├── darwin-arm64/
│   └── windows-amd64/
├── Makefile             # 构建脚本
└── README.md            # 文档
```

### 3.3 最小化实现

#### Step 1: 创建 `plugin.yaml`

```yaml
version: "1.0"

provider:
  name: my-engine-plugin
  display_name: My Engine Plugin
  version: 1.0.0
  type: local
  author: Your Name
  description: My custom AI engine plugin
  homepage: https://github.com/yourusername/my-engine-plugin
  engine_host: "http://127.0.0.1:8080"

services:
  - service_name: chat
    task_type: text-generation
    protocol: HTTP
    expose_protocol: HTTP
    endpoint: /api/chat
    auth_type: none
    default_model: my-model
    support_models:
      - my-model
    capabilities:
      support_streaming: true
      support_bidirectional: false

platforms:
  linux_amd64:
    executable: bin/linux-amd64/my-engine-plugin
  darwin_arm64:
    executable: bin/darwin-arm64/my-engine-plugin
  windows_amd64:
    executable: bin/windows-amd64/my-engine-plugin.exe
```

#### Step 2: 创建 `main.go`

**参考实现**（来自 `ollama-plugin/main.go` 和 `aliyun-plugin/main.go`）：

```go
package main

import (
    "fmt"
    "os"

    "github.com/hashicorp/go-plugin"
    "github.com/intel/aog/plugin-sdk/server"
    "github.com/yourusername/my-engine-plugin/internal"
)

func main() {
    // 加载配置
    config, err := internal.LoadConfig()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
        os.Exit(1)
    }

    // 创建provider
    provider, err := internal.NewMyEngineProvider(config)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to create provider: %v\n", err)
        os.Exit(1)
    }

    // 启动插件服务（使用SDK的server包）
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: server.PluginHandshake,
        Plugins: map[string]plugin.Plugin{
            server.PluginTypeProvider: server.NewProviderPlugin(provider),
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

**关键点**：

- 使用 `server.PluginHandshake` 作为握手配置
- 使用 `server.PluginTypeProvider` 作为插件类型
- 使用 `server.NewProviderPlugin(provider)` 包装你的provider
- 使用 `plugin.DefaultGRPCServer` 作为gRPC服务器

#### Step 3: 创建 `internal/provider.go`

**本地插件示例**（参考 `ollama-plugin/internal/provider.go`）：

```go
package internal

import (
    "context"
    "fmt"
    "log"
    "os"
    "path/filepath"

    "github.com/intel/aog/plugin-sdk/adapter"
    "github.com/intel/aog/plugin-sdk/client"
    "github.com/intel/aog/plugin-sdk/types"
    "gopkg.in/yaml.v3"
)

// 编译时接口检查
var (
    _ client.PluginProvider      = (*MyEngineProvider)(nil)
    _ client.LocalPluginProvider = (*MyEngineProvider)(nil)
    _ client.StreamablePlugin    = (*MyEngineProvider)(nil)  // 如果支持流式
)

type MyEngineProvider struct {
    *adapter.LocalPluginAdapter
    config *Config
    client *MyEngineClient

    // 服务处理器
    chatService services.ServiceHandler
}

func NewMyEngineProvider(config *Config) (*MyEngineProvider, error) {
    // 加载插件元数据（从plugin.yaml）
    manifest, err := loadManifest()
    if err != nil {
        return nil, fmt.Errorf("failed to load manifest: %w", err)
    }

    // 创建适配器
    localAdapter := adapter.NewLocalPluginAdapter(manifest)
    localAdapter.EngineHost = fmt.Sprintf("%s://%s", config.Scheme, config.Host)

    // 创建客户端
    client, err := NewMyEngineClient(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create client: %w", err)
    }

    // 创建服务处理器
    chatService := services.NewChatService(client)

    provider := &MyEngineProvider{
        LocalPluginAdapter: localAdapter,
        config:             config,
        client:             client,
        chatService:        chatService,
    }

    // 设置初始状态为运行
    provider.SetOperateStatus(1)
    return provider, nil
}

// loadManifest 从plugin.yaml加载元数据
func loadManifest() (*types.PluginManifest, error) {
    // 获取插件根目录
    pluginDir, err := getPluginDir()
    if err != nil {
        return nil, fmt.Errorf("failed to get plugin dir: %w", err)
    }

    manifestPath := filepath.Join(pluginDir, "plugin.yaml")
    data, err := os.ReadFile(manifestPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read plugin.yaml: %w", err)
    }

    var manifest types.PluginManifest
    if err := yaml.Unmarshal(data, &manifest); err != nil {
        return nil, fmt.Errorf("failed to parse plugin.yaml: %w", err)
    }

    return &manifest, nil
}

// ===== 实现核心接口 =====

// InvokeService 实现服务调用（必须实现）
// 注意：authInfo参数用于远程插件的认证，本地插件通常不使用，为空即可
func (p *MyEngineProvider) InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error) {
    log.Printf("[my-plugin] [INFO] Invoking service: %s (unary)", serviceName)
    switch serviceName {
    case "chat":
        return p.chatService.HandleUnary(ctx, authInfo, request)
    default:
        return nil, fmt.Errorf("unsupported service: %s", serviceName)
    }
}

// StartEngine 启动引擎（必须实现）
func (p *MyEngineProvider) StartEngine(mode string) error {
    log.Printf("[my-plugin] [INFO] Starting engine with mode: %s", mode)
    // 实现引擎启动逻辑
    return nil
}

// StopEngine 停止引擎（必须实现）
func (p *MyEngineProvider) StopEngine() error {
    log.Printf("[my-plugin] [INFO] Stopping engine")
    // 实现引擎停止逻辑
    return nil
}

// InvokeServiceStream 实现流式服务调用（可选，如果支持流式）
func (p *MyEngineProvider) InvokeServiceStream(
    ctx context.Context,
    serviceName string,
    authInfo string,
    request []byte,
) (<-chan client.StreamChunk, error) {
    log.Printf("[my-plugin] [INFO] Invoking service: %s (streaming)", serviceName)
    ch := make(chan client.StreamChunk, 10)

    go func() {
        defer close(ch)
        switch serviceName {
        case "chat":
            if streamingHandler, ok := p.chatService.(services.StreamingHandler); ok {
                streamingHandler.HandleStreaming(ctx, authInfo, request, ch)
            } else {
                ch <- client.StreamChunk{
                    Error: fmt.Errorf("chat service does not support streaming"),
                }
            }
        default:
            ch <- client.StreamChunk{
                Error: fmt.Errorf("service %s does not support streaming", serviceName),
            }
        }
    }()

    return ch, nil
}

// 其他方法（CheckEngine, InstallEngine等）根据需要覆盖adapter的默认实现
```

**远程插件示例**（参考 `aliyun-plugin/internal/provider.go`）：

```go
package internal

import (
    "context"
    "fmt"
    "log"

    "github.com/intel/aog/plugin-sdk/adapter"
    "github.com/intel/aog/plugin-sdk/client"
    "github.com/intel/aog/plugin-sdk/types"
)

// 编译时接口检查
var (
    _ client.PluginProvider       = (*MyRemoteProvider)(nil)
    _ client.RemotePluginProvider = (*MyRemoteProvider)(nil)
    _ client.StreamablePlugin     = (*MyRemoteProvider)(nil)
)

type MyRemoteProvider struct {
    *adapter.RemotePluginAdapter
    config *Config
    client *MyAPIClient

    // 服务处理器
    chatService services.ServiceHandler
}

func NewMyRemoteProvider(config *Config) (*MyRemoteProvider, error) {
    // 创建远程适配器
    remoteAdapter := adapter.NewRemotePluginAdapter(&types.PluginManifest{})

    // 创建API客户端
    client, err := NewMyAPIClient(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create client: %w", err)
    }

    // 创建服务处理器
    chatService := services.NewChatService(client)

    provider := &MyRemoteProvider{
        RemotePluginAdapter: remoteAdapter,
        config:              config,
        client:              client,
        chatService:         chatService,
    }

    return provider, nil
}

// InvokeService 实现服务调用
func (p *MyRemoteProvider) InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error) {
    log.Printf("[my-plugin] [INFO] Invoking service: %s", serviceName)
    switch serviceName {
    case "chat":
        return p.chatService.HandleUnary(ctx, authInfo, request)
    default:
        return nil, fmt.Errorf("unsupported service: %s", serviceName)
    }
}
```

---

## 4. 插件架构

### 4.1 整体架构图

```
┌─────────────────────────────────────────┐
│           AOG Core                      │
│  ┌──────────────────────────────────┐  │
│  │   Plugin Manager & Registry      │  │
│  └──────────────────────────────────┘  │
│              ↓ gRPC                     │
└─────────────────────────────────────────┘
                ↓
┌─────────────────────────────────────────┐
│        Plugin Process                   │
│  ┌──────────────────────────────────┐  │
│  │   plugin-sdk/server              │  │
│  │   (gRPC Server)                  │  │
│  └──────────────────────────────────┘  │
│              ↓                          │
│  ┌──────────────────────────────────┐  │
│  │   YourProvider                   │  │
│  │   (继承 LocalPluginAdapter)      │  │
│  └──────────────────────────────────┘  │
│              ↓                          │
│  ┌──────────────────────────────────┐  │
│  │   Services (chat, embed, ...)   │  │
│  └──────────────────────────────────┘  │
│              ↓                          │
│  ┌──────────────────────────────────┐  │
│  │   Engine Client (HTTP/gRPC)     │  │
│  └──────────────────────────────────┘  │
└─────────────────────────────────────────┘
                ↓
┌─────────────────────────────────────────┐
│       Local AI Engine                   │
│   (Ollama/OpenVINO/LlamaCpp/...)       │
└─────────────────────────────────────────┘
```

### 4.2 数据流

#### 非流式调用

```
AOG Core
  → gRPC: InvokeService(serviceName, request)
    → YourProvider.InvokeService()
      → ServiceHandler.HandleUnary()
        → EngineClient.Do()
          → Engine HTTP API
        ← Response
      ← Service Response
    ← gRPC Response
  ← JSON Response to User
```

#### 流式调用

```
AOG Core
  → gRPC: InvokeServiceStream(serviceName, request)
    → YourProvider.InvokeServiceStream()
      → ServiceHandler.HandleStreaming()
        → EngineClient.StreamResponse()
          → Engine HTTP API (SSE/Stream)
        ← Stream Chunks
      ← types.StreamChunk (via channel)
    ← gRPC Stream Response
  ← SSE Stream to User
```

---

## 5. 核心接口实现

本节介绍如何实现插件的各种核心接口。所有示例均来自实际代码。

---

### 5.1 引擎生命周期管理（本地插件）

#### StartEngine - 启动引擎

**实际实现**（来自 `ollama-plugin/internal/engine.go`）：

```go
func (p *OllamaProvider) StartEngine(mode string) error {
    log.Printf("[ollama-plugin] [INFO] Starting engine with mode: %s", mode)
    config, err := p.getConfig()
    if err != nil {
        log.Printf("[ollama-plugin] [ERROR] Failed to get config: %v", err)
        return fmt.Errorf("failed to get config: %w", err)
    }

    // 检查引擎是否已安装
    installed, err := p.CheckEngine()
    if err != nil {
        log.Printf("[ollama-plugin] [ERROR] Engine check failed: %v", err)
        return fmt.Errorf("failed to check engine: %w", err)
    }
    if !installed {
        log.Printf("[ollama-plugin] [ERROR] Engine not installed at: %s", config.ExecPath)
        return fmt.Errorf("ollama not installed, please run InstallEngine first")
    }

    // 初始化进程管理器
    if processManager == nil {
        processManager = utils.NewProcessManager(
            config.ExecPath,
            config.Host,
            config.ModelsDir,
        )
    }

    // 启动进程
    if err := processManager.Start(mode); err != nil {
        log.Printf("[ollama-plugin] [ERROR] Failed to start ollama: %v", err)
        return fmt.Errorf("failed to start ollama: %w", err)
    }

    log.Printf("[ollama-plugin] [INFO] ✅ Engine started successfully")
    return nil
}
```

**关键点**：
- 检查引擎是否已安装
- 使用进程管理器启动引擎进程
- 设置必要的环境变量（如OLLAMA_MODELS）
- 添加详细的日志记录

#### StopEngine - 停止引擎

```go
func (p *OllamaProvider) StopEngine() error {
    log.Printf("[my-plugin] [INFO] Stopping engine...")

    // 1. 卸载运行中的模型（可选但推荐）
    if err := p.unloadRunningModels(); err != nil {
        log.Printf("[my-plugin] [WARN] Failed to unload models: %v", err)
    }

    // 2. 停止进程
    if processManager != nil {
        if err := processManager.Stop(); err != nil {
            return fmt.Errorf("failed to stop engine: %w", err)
        }
    }

    log.Printf("[my-plugin] [INFO] ✅ Engine stopped successfully")
    return nil
}
```

**关键点**：
- 先卸载模型释放资源
- 优雅关闭引擎进程
- 清理临时文件（可选）

#### HealthCheck - 健康检查

```go
func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
    log.Printf("[my-plugin] [DEBUG] Performing health check...")
    
    // 使用进程管理器检查
    if processManager != nil {
        return processManager.HealthCheck()
    }

    // 直接HTTP检查
    config, err := p.getConfig()
    if err != nil {
        return err
    }

    url := fmt.Sprintf("%s://%s/health", config.Scheme, config.Host)
    req, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        return err
    }

    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    req = req.WithContext(ctx)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unhealthy status: %d", resp.StatusCode)
    }

    log.Printf("[my-plugin] [DEBUG] Health check passed")
    return nil
}
```

### 6.2 引擎安装管理

#### CheckEngine - 检查引擎是否已安装

```go
func (p *OllamaProvider) CheckEngine() (bool, error) {
    config, err := p.getConfig()
    if err != nil {
        return false, err
    }

    // 检查可执行文件是否存在
    if _, err := os.Stat(config.ExecPath); os.IsNotExist(err) {
        return false, nil
    }

    return true, nil
}
```

#### InstallEngine - 安装引擎

```go
func (p *OllamaProvider) InstallEngine(ctx context.Context) error {
    log.Printf("[my-plugin] [INFO] Installing engine...")
    
    config, err := p.getConfig()
    if err != nil {
        return err
    }

    // 1. 检测平台
    goos := runtime.GOOS
    goarch := runtime.GOARCH
    
    // 2. 确定下载URL
    downloadURL := p.getDownloadURL(goos, goarch)
    
    // 3. 下载引擎
    tmpFile, err := p.downloadEngine(ctx, downloadURL)
    if err != nil {
        return fmt.Errorf("failed to download engine: %w", err)
    }
    defer os.Remove(tmpFile)
    
    // 4. 解压/安装
    if err := p.extractEngine(tmpFile, config.EngineDir); err != nil {
        return fmt.Errorf("failed to extract engine: %w", err)
    }
    
    // 5. 设置权限
    if err := os.Chmod(config.ExecPath, 0755); err != nil {
        return fmt.Errorf("failed to set permissions: %w", err)
    }

    log.Printf("[my-plugin] [INFO] ✅ Engine installed successfully")
    return nil
}
```

### 6.3 模型管理

#### PullModel - 拉取模型

```go
func (p *OllamaProvider) PullModel(ctx context.Context, req *types.PullModelRequest, fn types.PullProgressFunc) (*types.ProgressResponse, error) {
    log.Printf("[my-plugin] [INFO] Pulling model: %s", req.Name)
    
    // 构建请求
    pullReq := map[string]interface{}{
        "name": req.Name,
    }
    
    // 调用引擎API
    var resp map[string]interface{}
    if err := p.client.Do(ctx, http.MethodPost, "/api/pull", pullReq, &resp); err != nil {
        return nil, fmt.Errorf("failed to pull model: %w", err)
    }
    
    // 构建响应
    return &types.ProgressResponse{
        Status:    "success",
        Total:     resp["total"].(float64),
        Completed: resp["completed"].(float64),
    }, nil
}
```

#### ListModels - 列出模型

```go
func (p *OllamaProvider) ListModels(ctx context.Context) (*types.ListResponse, error) {
    log.Printf("[my-plugin] [DEBUG] Listing models...")
    
    // 调用引擎API
    var resp map[string]interface{}
    if err := p.client.Do(ctx, http.MethodGet, "/api/tags", nil, &resp); err != nil {
        return nil, fmt.Errorf("failed to list models: %w", err)
    }
    
    // 解析模型列表
    models := []types.ModelInfo{}
    if modelList, ok := resp["models"].([]interface{}); ok {
        for _, m := range modelList {
            if modelData, ok := m.(map[string]interface{}); ok {
                models = append(models, types.ModelInfo{
                    Name:       modelData["name"].(string),
                    Size:       int64(modelData["size"].(float64)),
                    ModifiedAt: modelData["modified_at"].(string),
                })
            }
        }
    }
    
    return &types.ListResponse{Models: models}, nil
}
```

### 6.4 服务调用

#### InvokeService - 非流式服务调用

```go
func (p *OllamaProvider) InvokeService(ctx context.Context, serviceName string, request []byte) ([]byte, error) {
    log.Printf("[my-plugin] [INFO] Invoking service: %s (unary)", serviceName)
    
    switch serviceName {
    case "chat":
        return p.chatService.HandleUnary(ctx, request)
    case "embed":
        return p.embedService.HandleUnary(ctx, request)
    default:
        return nil, fmt.Errorf("unsupported service: %s", serviceName)
    }
}
```

#### InvokeServiceStream - 流式服务调用

```go
func (p *OllamaProvider) InvokeServiceStream(
    ctx context.Context,
    serviceName string,
    request []byte,
) (<-chan types.StreamChunk, error) {
    log.Printf("[my-plugin] [INFO] Invoking service: %s (streaming)", serviceName)
    
    ch := make(chan types.StreamChunk, 10)

    go func() {
        defer close(ch)

        switch serviceName {
        case "chat":
            if streamingHandler, ok := p.chatService.(services.StreamingHandler); ok {
                streamingHandler.HandleStreaming(ctx, request, ch)
            } else {
                ch <- types.StreamChunk{
                    Error: fmt.Errorf("chat service does not support streaming"),
                }
            }
        default:
            ch <- types.StreamChunk{
                Error: fmt.Errorf("service %s does not support streaming", serviceName),
            }
        }
    }()

    return ch, nil
}
```

---

## 7. 服务开发

### 7.1 服务接口设计

**参考实现**（来自 `ollama-plugin/internal/services/common.go`）：

```go
// ServiceHandler 服务处理器接口（所有服务必须实现）
type ServiceHandler interface {
    // HandleUnary 处理非流式请求
    HandleUnary(ctx context.Context, request []byte) ([]byte, error)
}

// StreamingHandler 流式服务处理器接口（可选）
type StreamingHandler interface {
    // HandleStreaming 处理流式请求
    HandleStreaming(ctx context.Context, request []byte, ch chan<- types.StreamChunk)
}

// ClientInterface 引擎客户端接口
type ClientInterface interface {
    Do(ctx context.Context, method, path string, body interface{}, result interface{}) error
    StreamResponse(ctx context.Context, method, path string, body interface{}) (chan []byte, chan error)
}
```

### 7.2 实现Chat服务

**参考实现**（来自 `ollama-plugin/internal/services/chat.go`）：

```go
type ChatService struct {
    client ClientInterface
}

func NewChatService(client ClientInterface) *ChatService {
    return &ChatService{client: client}
}

// HandleUnary 处理非流式聊天请求
func (s *ChatService) HandleUnary(ctx context.Context, request []byte) ([]byte, error) {
    // 1. 解析请求
    var req ServiceRequest
    if err := json.Unmarshal(request, &req); err != nil {
        return nil, fmt.Errorf("failed to unmarshal request: %w", err)
    }

    // 2. 构建引擎请求
    engineReq := map[string]interface{}{
        "stream":   false,
        "model":    req.Data["model"],
        "messages": req.Data["messages"],
    }

    // 3. 调用引擎API
    var engineResp map[string]interface{}
    if err := s.client.Do(ctx, http.MethodPost, "/api/chat", engineReq, &engineResp); err != nil {
        return nil, fmt.Errorf("engine chat failed: %w", err)
    }

    // 4. 构建响应
    respData := map[string]interface{}{
        "message": engineResp["message"],
        "model":   engineResp["model"],
    }

    resp := ServiceResponse{Data: respData}
    return json.Marshal(resp)
}

// HandleStreaming 处理流式聊天请求
func (s *ChatService) HandleStreaming(ctx context.Context, request []byte, ch chan<- types.StreamChunk) {
    // 1. 解析请求
    var req ServiceRequest
    if err := json.Unmarshal(request, &req); err != nil {
        ch <- types.StreamChunk{Error: err}
        return
    }

    // 2. 构建引擎请求（流式）
    engineReq := map[string]interface{}{
        "stream":   true,
        "model":    req.Data["model"],
        "messages": req.Data["messages"],
    }

    // 3. 调用引擎流式API
    dataChan, errChan := s.client.StreamResponse(ctx, http.MethodPost, "/api/chat", engineReq)

    // 4. 转发流式数据
    for {
        select {
        case data, ok := <-dataChan:
            if !ok {
                // 通道关闭，发送最后一个chunk
                ch <- types.StreamChunk{IsFinal: true}
                return
            }

            // 转换为SSE格式
            sseData := fmt.Sprintf("data: %s\n\n", string(data))
            ch <- types.StreamChunk{
                Data: []byte(sseData),
                Metadata: map[string]string{
                    "content-type": "text/event-stream",
                },
            }

        case err := <-errChan:
            if err != nil {
                ch <- types.StreamChunk{Error: err}
            }
            return

        case <-ctx.Done():
            ch <- types.StreamChunk{Error: ctx.Err()}
            return
        }
    }
}
```

### 7.3 实现Embedding服务

```go
type EmbedService struct {
    client ClientInterface
}

func NewEmbedService(client ClientInterface) *EmbedService {
    return &EmbedService{client: client}
}

func (s *EmbedService) HandleUnary(ctx context.Context, request []byte) ([]byte, error) {
    var req ServiceRequest
    if err := json.Unmarshal(request, &req); err != nil {
        return nil, fmt.Errorf("failed to unmarshal request: %w", err)
    }

    // 构建引擎请求
    engineReq := map[string]interface{}{
        "model":  req.Data["model"],
        "prompt": req.Data["input"],
    }

    // 调用引擎API
    var engineResp map[string]interface{}
    if err := s.client.Do(ctx, http.MethodPost, "/api/embeddings", engineReq, &engineResp); err != nil {
        return nil, fmt.Errorf("engine embedding failed: %w", err)
    }

    // 构建响应
    respData := map[string]interface{}{
        "embeddings": engineResp["embedding"],
    }

    resp := ServiceResponse{Data: respData}
    return json.Marshal(resp)
}
```

### 7.4 HTTP客户端实现

**参考实现**（来自 `ollama-plugin/internal/client.go`）：

```go
type OllamaClient struct {
    baseURL    string
    httpClient *http.Client
    timeout    time.Duration
}

func NewOllamaClient(config *Config) (*OllamaClient, error) {
    baseURL := fmt.Sprintf("%s://%s", config.Scheme, config.Host)
    return &OllamaClient{
        baseURL: baseURL,
        httpClient: &http.Client{
            Timeout: config.Timeout,
        },
        timeout: config.Timeout,
    }, nil
}

// Do 执行HTTP请求
func (c *OllamaClient) Do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
    url := c.baseURL + path

    // 构建请求体
    var reqBody io.Reader
    if body != nil {
        jsonData, err := json.Marshal(body)
        if err != nil {
            return fmt.Errorf("failed to marshal request: %w", err)
        }
        reqBody = bytes.NewBuffer(jsonData)
    }

    // 创建请求
    req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    // 发送请求
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    // 检查状态码
    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
    }

    // 解析响应
    if result != nil {
        if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
            return fmt.Errorf("failed to decode response: %w", err)
        }
    }

    return nil
}

// StreamResponse 执行流式HTTP请求
func (c *OllamaClient) StreamResponse(ctx context.Context, method, path string, body interface{}) (chan []byte, chan error) {
    dataChan := make(chan []byte, 10)
    errChan := make(chan error, 1)

    go func() {
        defer close(dataChan)
        defer close(errChan)

        url := c.baseURL + path

        // 构建请求体
        jsonData, err := json.Marshal(body)
        if err != nil {
            errChan <- fmt.Errorf("failed to marshal request: %w", err)
            return
        }

        // 创建请求
        req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(jsonData))
        if err != nil {
            errChan <- fmt.Errorf("failed to create request: %w", err)
            return
        }
        req.Header.Set("Content-Type", "application/json")

        // 发送请求
        resp, err := c.httpClient.Do(req)
        if err != nil {
            errChan <- fmt.Errorf("request failed: %w", err)
            return
        }
        defer resp.Body.Close()

        // 逐行读取响应
        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            select {
            case dataChan <- scanner.Bytes():
            case <-ctx.Done():
                errChan <- ctx.Err()
                return
            }
        }

        if err := scanner.Err(); err != nil {
            errChan <- fmt.Errorf("stream read error: %w", err)
        }
    }()

    return dataChan, errChan
}
```

---

## 8. 配置管理

### 8.1 配置文件结构

**参考实现**（来自 `ollama-plugin/internal/config.go`）：

```go
type Config struct {
    // 引擎配置
    Host         string        `json:"host"`
    Scheme       string        `json:"scheme"`
    DefaultModel string        `json:"default_model"`
    Timeout      time.Duration `json:"timeout"`
    
    // 路径配置
    EngineDir   string `json:"engine_dir"`
    ExecPath    string `json:"exec_path"`
    ModelsDir   string `json:"models_dir"`
    DownloadDir string `json:"download_dir"`
    
    // 设备配置
    DeviceType string `json:"device_type"`
}

func LoadConfig() (*Config, error) {
    config := &Config{
        Host:         getEnvOrDefault("ENGINE_HOST", "127.0.0.1:8080"),
        Scheme:       getEnvOrDefault("ENGINE_SCHEME", "http"),
        DefaultModel: getEnvOrDefault("ENGINE_DEFAULT_MODEL", "default-model"),
        Timeout:      30 * time.Second,
    }
    
    // 设置路径
    if err := config.initPaths(); err != nil {
        return nil, err
    }
    
    return config, nil
}

func (c *Config) initPaths() error {
    // 获取AOG数据目录
    dataDir := getAOGDataDir()
    
    c.EngineDir = filepath.Join(dataDir, "engine", "my-engine")
    c.ExecPath = filepath.Join(c.EngineDir, "bin", "engine")
    c.ModelsDir = filepath.Join(c.EngineDir, "models")
    c.DownloadDir = filepath.Join(os.Getenv("HOME"), "Downloads")
    
    // 创建目录
    dirs := []string{c.EngineDir, c.ModelsDir, c.DownloadDir}
    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("failed to create directory %s: %w", dir, err)
        }
    }
    
    return nil
}

func getAOGDataDir() string {
    switch runtime.GOOS {
    case "darwin":
        return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "AOG")
    case "linux":
        return "/var/lib/aog"
    case "windows":
        return filepath.Join(os.Getenv("LOCALAPPDATA"), "AOG")
    default:
        return filepath.Join(os.Getenv("HOME"), ".aog")
    }
}

func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

### 8.2 环境变量支持

在 `plugin.yaml` 中使用环境变量展开：

```yaml
resources:
  data_dir: "${AOG_DATA_DIR}/engine/my-engine"
  
  my_engine:
    executable: "${DATA_DIR}/bin/engine"
    models_dir: "${DATA_DIR}/models"
    download_dir: "${HOME}/Downloads"
```

支持的变量：
- `${AOG_DATA_DIR}`: AOG统一数据目录
- `${PLUGIN_DIR}`: 插件可执行文件所在目录
- `${HOME}`: 用户主目录
- `${DATA_DIR}`: `resources.data_dir` 的值

---

## 9. 跨平台构建

### 9.1 Makefile

**参考实现**（来自 `ollama-plugin/Makefile`）：

```makefile
# 插件信息
PLUGIN_NAME := my-engine-plugin
VERSION := 1.0.0

# 构建目录
BIN_DIR := bin
BUILD_FLAGS := -trimpath -ldflags="-s -w"

# 支持的平台
PLATFORMS := \
	linux-amd64 \
	linux-arm64 \
	darwin-amd64 \
	darwin-arm64 \
	windows-amd64

.PHONY: all build build-all clean verify package

all: build

# 构建当前平台
build:
	@echo "Building $(PLUGIN_NAME) for current platform..."
	@mkdir -p $(BIN_DIR)
	go build $(BUILD_FLAGS) -o $(BIN_DIR)/$(PLUGIN_NAME) .

# 构建所有平台
build-all:
	@echo "Building $(PLUGIN_NAME) for all platforms..."
	@$(MAKE) $(PLATFORMS)

# 平台特定构建规则
linux-amd64:
	@echo "Building for linux-amd64..."
	@mkdir -p $(BIN_DIR)/linux-amd64
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BIN_DIR)/linux-amd64/$(PLUGIN_NAME) .

linux-arm64:
	@echo "Building for linux-arm64..."
	@mkdir -p $(BIN_DIR)/linux-arm64
	GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BIN_DIR)/linux-arm64/$(PLUGIN_NAME) .

darwin-amd64:
	@echo "Building for darwin-amd64..."
	@mkdir -p $(BIN_DIR)/darwin-amd64
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BIN_DIR)/darwin-amd64/$(PLUGIN_NAME) .

darwin-arm64:
	@echo "Building for darwin-arm64..."
	@mkdir -p $(BIN_DIR)/darwin-arm64
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BIN_DIR)/darwin-arm64/$(PLUGIN_NAME) .

windows-amd64:
	@echo "Building for windows-amd64..."
	@mkdir -p $(BIN_DIR)/windows-amd64
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BIN_DIR)/windows-amd64/$(PLUGIN_NAME).exe .

# 清理构建产物
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_DIR)

# 验证构建产物
verify:
	@echo "Verifying build artifacts..."
	@for platform in $(PLATFORMS); do \
		if [ "$$platform" = "windows-amd64" ]; then \
			file=$(BIN_DIR)/$$platform/$(PLUGIN_NAME).exe; \
		else \
			file=$(BIN_DIR)/$$platform/$(PLUGIN_NAME); \
		fi; \
		if [ -f "$$file" ]; then \
			echo "✓ $$file"; \
		else \
			echo "✗ $$file not found"; \
			exit 1; \
		fi; \
	done

# 打包分发
package: build-all verify
	@echo "Creating distribution package..."
	@tar -czf $(PLUGIN_NAME)-$(VERSION).tar.gz \
		--exclude='*.go' \
		--exclude='go.mod' \
		--exclude='go.sum' \
		--exclude='.git*' \
		.
	@echo "✓ Package created: $(PLUGIN_NAME)-$(VERSION).tar.gz"
```

### 9.2 构建脚本

**简化版构建脚本** (`build-all.sh`):

```bash
#!/bin/bash

PLUGIN_NAME="my-engine-plugin"
VERSION=${VERSION:-1.0.0}
BIN_DIR="bin"

PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

echo "Building $PLUGIN_NAME v$VERSION for all platforms..."

for platform in "${PLATFORMS[@]}"; do
    GOOS=${platform%/*}
    GOARCH=${platform#*/}
    OUTPUT_DIR="$BIN_DIR/$GOOS-$GOARCH"
    OUTPUT_NAME="$PLUGIN_NAME"
    
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="$OUTPUT_NAME.exe"
    fi
    
    echo "Building for $GOOS-$GOARCH..."
    mkdir -p "$OUTPUT_DIR"
    
    GOOS=$GOOS GOARCH=$GOARCH go build \
        -trimpath \
        -ldflags="-s -w -X main.version=$VERSION" \
        -o "$OUTPUT_DIR/$OUTPUT_NAME" \
        .
    
    if [ $? -eq 0 ]; then
        echo "✓ Built: $OUTPUT_DIR/$OUTPUT_NAME"
    else
        echo "✗ Failed to build for $GOOS-$GOARCH"
        exit 1
    fi
done

echo ""
echo "✅ All platforms built successfully!"
```

---

## 9. 部署和测试

### 9.1 插件部署方式

**重要**：AOG目前**没有**插件管理CLI命令。插件部署是通过直接放置到插件目录实现的。

AOG启动时会自动扫描并加载 `plugins/` 目录下的插件。

#### 方法1：开发模式部署（推荐）

```bash
# 1. 在AOG项目根目录创建plugins目录
cd /path/to/aog
mkdir -p plugins

# 2. 创建符号链接到你的插件项目
ln -s /path/to/my-engine-plugin plugins/my-engine-plugin

# 3. 启动AOG
./aog server start
```

**日志输出示例**：

```log
[INFO] Initializing plugin system... pluginDir=/path/to/aog/plugins
[INFO] Plugin discovery succeeded total=1 directory=/path/to/aog/plugins
[INFO] Discovered plugin name=my-engine-plugin version=1.0.0 type=local services=2
```

#### 方法2：生产环境部署

```bash
# 1. 构建插件的所有平台版本
cd /path/to/my-engine-plugin
make build-all

# 2. 打包插件
make package
# 生成：my-engine-plugin-1.0.0.tar.gz

# 3. 在目标服务器上解压到plugins目录
ssh user@server "mkdir -p /opt/aog/plugins/my-engine-plugin"
scp my-engine-plugin-1.0.0.tar.gz user@server:/tmp/
ssh user@server "cd /opt/aog/plugins/my-engine-plugin && tar -xzf /tmp/my-engine-plugin-1.0.0.tar.gz"

# 4. 重启AOG
ssh user@server "systemctl restart aog"
```

### 9.2 验证插件加载

#### 查看日志

```bash
# AOG启动日志会显示插件发现情况
tail -f /var/log/aog/engine.log | grep -i plugin
```

**成功加载的示例日志**：

```log
[INFO] Plugin discovery succeeded total=2 directory=/opt/aog/plugins
[INFO] Discovered plugin name=ollama-plugin version=1.0.0 type=local services=3
[INFO] Discovered plugin name=aliyun-plugin version=1.0.0 type=remote services=7
[INFO] Plugin registered as APIFlavor plugin=ollama-plugin services=3
[INFO] Plugin registered as APIFlavor plugin=aliyun-plugin services=7
```

#### 通过API验证

```bash
# 查看可用的providers
curl http://localhost:16688/v1/providers

# 应该包含你的插件
# {
#   "providers": [
#     {"name": "my-engine-plugin", "type": "local", "services": [...]},
#     ...
#   ]
# }

# 测试插件服务
curl -X POST http://localhost:16688/v1/services/chat \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "my-engine-plugin",
    "service": "chat",
    "data": {
      "model": "my-model",
      "messages": [{"role": "user", "content": "hello"}]
    }
  }'
```

### 9.3 常见部署问题

### 9.4 调试技巧

#### 方法1：日志调试

```go
// 在provider中添加详细日志
func (p *MyEngineProvider) InvokeService(ctx context.Context, serviceName string, request []byte) ([]byte, error) {
    p.LogInfo(fmt.Sprintf("InvokeService called: service=%s, request_size=%d", serviceName, len(request)))
    p.LogDebug(fmt.Sprintf("Request data: %s", string(request)))
    
    result, err := p.handleService(ctx, serviceName, request)
    
    if err != nil {
        p.LogError("Service invocation failed", err)
    } else {
        p.LogDebug(fmt.Sprintf("Response data: %s", string(result)))
    }
    
    return result, err
}
```

#### 方法2：文件调试

```go
// 写入调试信息到文件（避免stdout/stderr冲突）
debugFile, _ := os.OpenFile("/tmp/my-plugin-debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
if debugFile != nil {
    defer debugFile.Close()
    fmt.Fprintf(debugFile, "[DEBUG] %s: %v\n", time.Now().Format(time.RFC3339), data)
}
```

#### 方法3：查看AOG日志

```bash
# 实时查看日志
aog logs --follow

# 查看插件相关日志
aog logs --follow | grep my-engine-plugin

# 查看错误日志
aog logs --level error
```

#### 方法4：手动测试gRPC接口

```bash
# 使用grpcurl测试插件
grpcurl -plaintext \
    -d '{"service_name":"chat","request_data":"..."}' \
    unix:///tmp/my-plugin.sock \
    provider.ProviderService/InvokeService
```

#### 问题1：插件无法启动

```bash
# 检查可执行文件权限
ls -la bin/*/my-engine-plugin

# 设置执行权限
chmod +x bin/*/my-engine-plugin

# 检查依赖
ldd bin/linux-amd64/my-engine-plugin  # Linux
otool -L bin/darwin-arm64/my-engine-plugin  # macOS
```

#### 问题2：AOG无法发现插件

```bash
# 检查插件目录结构
ls -la /path/to/aog/plugins/my-engine-plugin/

# 应该包含：
# - plugin.yaml
# - bin/目录（包含各平台的可执行文件）

# 检查plugin.yaml格式
cat /path/to/aog/plugins/my-engine-plugin/plugin.yaml

# 检查AOG启动日志
tail -100 /var/log/aog/engine.log | grep -i "plugin"
```

#### 问题3：插件加载但服务调用失败

```bash
# 测试引擎是否运行（本地插件）
curl http://127.0.0.1:16677  # 示例：Ollama的端口

# 测试服务端点
curl -X POST http://127.0.0.1:16677/api/chat \
    -H "Content-Type: application/json" \
    -d '{"model":"test","messages":[{"role":"user","content":"hello"}]}'

# 查看插件日志
tail -f /var/log/aog/engine.log | grep "my-plugin"
```

---

## 10. 最佳实践

### 11.1 代码组织

1. **使用适配器模式**
   - 优先使用 `LocalPluginAdapter` 或 `RemotePluginAdapter`
   - 只覆盖需要自定义的方法
   - 利用基类的日志、错误处理功能

2. **模块化设计**
   - 将不同功能拆分到独立文件
   - `provider.go`: 核心Provider实现
   - `engine.go`: 引擎生命周期管理
   - `models.go`: 模型管理
   - `services/`: 服务实现

3. **接口抽象**
   - 定义清晰的接口（如 `ServiceHandler`）
   - 使用依赖注入，方便测试
   - 编译时接口检查：`var _ Interface = (*Implementation)(nil)`

### 11.2 错误处理

1. **使用SDK的错误类型**

```go
import "github.com/intel/aog/plugin-sdk/types"

return &types.PluginError{
    Code:    types.ErrCodeInternal,
    Message: "operation failed",
    Details: err.Error(),
}
```

2. **统一错误包装**

```go
func (p *MyEngineProvider) wrapError(operation string, err error) error {
    return p.WrapError(operation, err)  // 使用BasePluginProvider的方法
}
```

3. **分级错误处理**

```go
// 致命错误：直接返回
if err := p.CheckEngine(); err != nil {
    return fmt.Errorf("engine check failed: %w", err)
}

// 警告：记录日志但继续
if err := p.cleanupTempFiles(); err != nil {
    p.LogWarn("Failed to cleanup temp files: %v", err)
}
```

### 11.3 日志规范

**实际做法**：所有插件示例都直接使用Go标准库的 `log.Printf`，采用统一的日志格式。

1. **标准日志格式**

```go
log.Printf("[plugin-name] [LEVEL] message")
```

**实际示例**（来自 `ollama-plugin` 和 `aliyun-plugin`）：

```go
// INFO: 重要操作
log.Printf("[ollama-plugin] [INFO] Starting engine with mode: %s", mode)
log.Printf("[aliyun-plugin] [INFO] Invoking service: %s (streaming)", serviceName)

// DEBUG: 详细调试信息
log.Printf("[ollama-plugin] [DEBUG] Checking if engine is installed...")
log.Printf("[aliyun-plugin] [DEBUG] Chat service completed successfully")

// WARN: 可恢复的错误
log.Printf("[ollama-plugin] [WARN] Failed to unload models (continuing): %v", err)

// ERROR: 严重错误
log.Printf("[ollama-plugin] [ERROR] Failed to start ollama: %v", err)
log.Printf("[aliyun-plugin] [ERROR] Chat service failed: %v", err)
```

2. **日志级别指南**
   - `[DEBUG]`: 详细的调试信息（请求/响应内容、状态检查）
   - `[INFO]`: 重要操作（启动、停止、服务调用完成）
   - `[WARN]`: 可恢复的错误
   - `[ERROR]`: 严重错误

3. **避免敏感信息**

```go
// ❌ 不要记录完整的API密钥
log.Printf("[my-plugin] [DEBUG] API Key: %s", apiKey)

// ✅ 只记录部分信息
log.Printf("[my-plugin] [DEBUG] API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
```

**注意**：SDK的 `BasePluginAdapter` 提供了 `LogInfo()`, `LogDebug()`, `LogError()` 方法，但实际插件开发中并不使用这些方法，而是直接使用 `log.Printf` 以获得更灵活的格式控制。

### 11.4 性能优化

1. **连接池复用**

```go
type OllamaClient struct {
    httpClient *http.Client
}

func NewOllamaClient(config *Config) *OllamaClient {
    return &OllamaClient{
        httpClient: &http.Client{
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
            Timeout: config.Timeout,
        },
    }
}
```

2. **流式传输优化**

```go
// 使用缓冲通道
ch := make(chan types.StreamChunk, 10)  // 缓冲10个chunk

// 避免阻塞
select {
case ch <- chunk:
case <-ctx.Done():
    return
}
```

3. **资源清理**

```go
func (p *MyEngineProvider) StopEngine() error {
    // 清理顺序：模型 → 进程 → 临时文件
    p.unloadRunningModels()
    p.stopProcess()
    p.cleanupTempFiles()
    return nil
}
```

### 11.5 测试策略

1. **单元测试**

```go
func TestChatService_HandleUnary(t *testing.T) {
    mockClient := &MockClient{
        DoFunc: func(ctx context.Context, method, path string, body, result interface{}) error {
            // 模拟引擎响应
            resp := result.(*map[string]interface{})
            (*resp)["message"] = map[string]interface{}{
                "role":    "assistant",
                "content": "Hello!",
            }
            return nil
        },
    }

    service := NewChatService(mockClient)
    request := []byte(`{"data":{"model":"test","messages":[...]}}`)
    
    response, err := service.HandleUnary(context.Background(), request)
    assert.NoError(t, err)
    assert.NotEmpty(t, response)
}
```

2. **集成测试**

```bash
# 手动测试服务
curl --location 'http://localhost:16688/aog/v0.2/services/chat' \
--header 'Content-Type: application/json' \
--data '{
    "model": "qwen3:xxxxx",
    "stream": false,
    "messages": [
        {
            "role": "user",
            "content": "天空为什么是蓝色的？"
        }
    ]
}'
```

### 11.6 安全建议

1. **输入验证**

```go
func (s *ChatService) HandleUnary(ctx context.Context, request []byte) ([]byte, error) {
    var req ServiceRequest
    if err := json.Unmarshal(request, &req); err != nil {
        return nil, fmt.Errorf("invalid request format: %w", err)
    }
    
    // 验证必需字段
    if req.Data["model"] == nil || req.Data["messages"] == nil {
        return nil, fmt.Errorf("missing required fields")
    }
    
    // 验证模型名称
    if !s.isValidModel(req.Data["model"].(string)) {
        return nil, fmt.Errorf("invalid model name")
    }
    
    // ...
}
```

2. **超时控制**

```go
func (c *OllamaClient) Do(ctx context.Context, method, path string, body, result interface{}) error {
    // 添加超时
    ctx, cancel := context.WithTimeout(ctx, c.timeout)
    defer cancel()
    
    req, _ := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
    // ...
}
```

3. **资源限制**

```go
// 限制并发请求数
var semaphore = make(chan struct{}, 10)  // 最多10个并发

func (p *MyEngineProvider) InvokeService(ctx context.Context, serviceName string, request []byte) ([]byte, error) {
    select {
    case semaphore <- struct{}{}:
        defer func() { <-semaphore }()
    case <-ctx.Done():
        return nil, ctx.Err()
    }
    
    // 处理请求
    return p.handleService(ctx, serviceName, request)
}
```

---

## 11. 常见问题

### Q1: 如何支持多个服务？

**A**: 在 `plugin.yaml` 中定义多个服务，并在 `InvokeService` 中路由：

```yaml
services:
  - service_name: chat
    task_type: text-generation
    # ...
  - service_name: embed
    task_type: embedding
    # ...
```

```go
func (p *MyEngineProvider) InvokeService(ctx context.Context, serviceName string, request []byte) ([]byte, error) {
    switch serviceName {
    case "chat":
        return p.chatService.HandleUnary(ctx, request)
    case "embed":
        return p.embedService.HandleUnary(ctx, request)
    default:
        return nil, fmt.Errorf("unsupported service: %s", serviceName)
    }
}
```

### Q2: 如何复用内置引擎的转换规则？

**A**: 使用 `config_ref` 引用内置模板：

```yaml
services:
  - service_name: chat
    # ...
    config_ref: ollama:chat  # 复用内置ollama的chat转换规则
```

AOG会自动应用转换规则，插件只需处理原生请求/响应。

### Q3: 如何支持流式响应？

**A**: 实现 `StreamablePlugin` 接口：

```go
func (p *MyEngineProvider) InvokeServiceStream(
    ctx context.Context,
    serviceName string,
    request []byte,
) (<-chan types.StreamChunk, error) {
    ch := make(chan types.StreamChunk, 10)

    go func() {
        defer close(ch)
        
        // 处理流式请求
        if handler, ok := p.getStreamingHandler(serviceName); ok {
            handler.HandleStreaming(ctx, request, ch)
        } else {
            ch <- types.StreamChunk{
                Error: fmt.Errorf("service does not support streaming"),
            }
        }
    }()

    return ch, nil
}
```

### Q4: 如何处理不同平台的差异？

**A**: 使用条件编译：

```go
// process_unix.go
// +build linux darwin

func startProcess(execPath string, args []string) (*os.Process, error) {
    // Unix-specific implementation
}

// process_windows.go
// +build windows

func startProcess(execPath string, args []string) (*os.Process, error) {
    // Windows-specific implementation
}
```

### Q5: 如何调试插件？

**A**: 主要调试方法：
1. **查看AOG日志**：`tail -f /var/log/aog/engine.log`
2. **使用文件日志**：在插件中写入 `/tmp/my-plugin-debug.log`
3. **直接测试引擎**：先确保引擎自身正常工作
4. **添加详细日志**：在每个关键步骤添加 `log.Printf`
5. **使用AOG API测试**：直接调用AOG的REST API测试服务

### Q6: 插件如何与AOG通信？

**A**: 通过gRPC + Protocol Buffers：
- AOG调用插件的gRPC方法
- 插件实现gRPC接口（由SDK自动处理）
- 数据使用JSON格式传输

### Q7: 如何处理模型下载进度？

**A**: 使用 `PullProgressFunc` 回调：

```go
func (p *MyEngineProvider) PullModel(ctx context.Context, req *types.PullModelRequest, fn types.PullProgressFunc) (*types.ProgressResponse, error) {
    // 开始下载
    total := getModelSize(req.Name)
    
    for downloaded := 0; downloaded < total; downloaded += chunkSize {
        // 下载chunk
        downloadChunk(...)
        
        // 报告进度
        if fn != nil {
            fn(&types.ProgressResponse{
                Status:    "downloading",
                Total:     float64(total),
                Completed: float64(downloaded),
            })
        }
    }
    
    return &types.ProgressResponse{Status: "success"}, nil
}
```

### Q8: 如何共享模型存储？

**A**: 使用AOG统一数据目录：

```yaml
resources:
  data_dir: "${AOG_DATA_DIR}/engine/my-engine"
  
  my_engine:
    models_dir: "${DATA_DIR}/models"  # 与其他插件共享
```

### Q9: 如何实现认证管理（Remote插件）？

**A**: 实现 `RemotePluginProvider` 接口：

```go
type MyRemoteProvider struct {
    *adapter.RemotePluginAdapter
    apiKey string
}

func (p *MyRemoteProvider) SetAuth(authType string, credentials map[string]string) error {
    if authType != "apikey" {
        return fmt.Errorf("unsupported auth type: %s", authType)
    }
    
    apiKey, ok := credentials["api_key"]
    if !ok {
        return fmt.Errorf("api_key is required")
    }
    
    p.apiKey = apiKey
    return nil
}

func (p *MyRemoteProvider) ValidateAuth(ctx context.Context) error {
    // 验证API Key
    return p.client.ValidateAPIKey(ctx, p.apiKey)
}
```

### Q10: 如何分发插件？

**A**: 
1. **构建所有平台**：`make build-all`
2. **打包插件**：`make package`
3. **发布方式**：
   - 直接分发：直接使用对应平台的可执行文件+plugin.yaml
4. **用户安装**：解压到 `plugins/` 目录，重启AOG

---

---

## 附录

### A. 完整的plugin.yaml模板

```yaml
version: "1.0"

provider:
  name: my-engine-plugin
  display_name: My Engine Plugin
  version: 1.0.0
  type: local  # local 或 remote
  author: Your Name
  description: A custom AI engine plugin for AOG
  homepage: https://github.com/yourusername/my-engine-plugin
  engine_host: "http://127.0.0.1:8080"

services:
  - service_name: chat
    task_type: text-generation
    protocol: HTTP
    expose_protocol: HTTP
    endpoint: /api/chat
    auth_type: none
    default_model: my-model
    support_models:
      - my-model
      - my-model-large
    config_ref: ""  # 可选：引用内置模板
    timeout: 300  # 可选：超时时间（秒）
    capabilities:
      support_streaming: true
      support_bidirectional: false

  - service_name: embed
    task_type: embedding
    protocol: HTTP
    expose_protocol: HTTP
    endpoint: /api/embeddings
    auth_type: none
    default_model: embed-model
    support_models:
      - embed-model
    capabilities:
      support_streaming: false
      support_bidirectional: false

platforms:
  linux_amd64:
    executable: bin/linux-amd64/my-engine-plugin
    dependencies: []
  
  linux_arm64:
    executable: bin/linux-arm64/my-engine-plugin
    dependencies: []
  
  darwin_amd64:
    executable: bin/darwin-amd64/my-engine-plugin
    dependencies: []
  
  darwin_arm64:
    executable: bin/darwin-arm64/my-engine-plugin
    dependencies: []
  
  windows_amd64:
    executable: bin/windows-amd64/my-engine-plugin.exe
    dependencies: []

resources:
  data_dir: "${AOG_DATA_DIR}/engine/my-engine"
  
  my_engine:
    executable: "${DATA_DIR}/bin/engine"
    models_dir: "${DATA_DIR}/models"
    download_dir: "${HOME}/Downloads"
```

### B. SDK接口速查表

**重要**：以下接口来自 `plugin-sdk/client/interfaces.go`，这是实际定义。

| 接口 | 方法 | 说明 | 必须实现 |
|------|------|------|---------|
| `PluginProvider` | `GetManifest()` | 获取插件元数据 | ✅ (SDK实现) |
|  | `GetOperateStatus()` | 获取运行状态 | ✅ (SDK实现) |
|  | `SetOperateStatus(int)` | 设置运行状态 | ✅ (SDK实现) |
|  | `HealthCheck(ctx)` | 健康检查 | ⚠️ 建议覆盖 |
|  | `InvokeService(ctx, name, req)` | 服务调用 | ✅ |
| `LocalPluginProvider` | `StartEngine(mode)` | 启动引擎 | ✅ |
|  | `StopEngine()` | 停止引擎 | ✅ |
|  | `GetConfig(ctx)` | 获取引擎配置 | ✅ |
|  | `CheckEngine()` | 检查引擎是否安装 | ⚠️ 建议实现 |
|  | `InstallEngine(ctx)` | 安装引擎 | ⚠️ 建议实现 |
|  | `InitEnv()` | 初始化环境变量 | ⚠️ 可选 |
|  | `UpgradeEngine(ctx)` | 升级引擎 | ⚠️ 可选 |
|  | `PullModel(ctx, req, fn)` | 拉取模型 | ⚠️ 建议实现 |
|  | `PullModelStream(ctx, req)` | 流式拉取模型 | ⚠️ 可选 |
|  | `DeleteModel(ctx, req)` | 删除模型 | ⚠️ 建议实现 |
|  | `ListModels(ctx)` | 列出模型 | ⚠️ 建议实现 |
|  | `LoadModel(ctx, req)` | 加载模型 | ⚠️ 可选 |
|  | `UnloadModel(ctx, req)` | 卸载模型 | ⚠️ 可选 |
|  | `GetRunningModels(ctx)` | 获取运行中的模型 | ⚠️ 可选 |
|  | `GetVersion(ctx, resp)` | 获取引擎版本 | ⚠️ 可选 |
| `StreamablePlugin` | `InvokeServiceStream(ctx, name, req)` | 流式服务调用 | ⚠️ 支持流式时需要 |

图例：
- ✅ 必须实现
- ⚠️ 建议实现/可选
- ✅ (SDK实现) SDK已提供默认实现

### C. 参考资源

- [AOG Plugin SDK README](../README.md)
- [Ollama Plugin 示例](../../plugin-example/ollama-plugin/)
- [Aliyun Plugin 示例](../../plugin-example/aliyun-plugin/)
- [AOG 官方仓库](https://github.com/intel/aog)
- [gRPC Go快速开始](https://grpc.io/docs/languages/go/quickstart/)
- [hashicorp/go-plugin文档](https://github.com/hashicorp/go-plugin)

---

## 总结

通过本指南，您应该能够：

1. ✅ 理解AOG插件系统的架构和核心概念
2. ✅ 快速创建一个新的Engine插件项目
3. ✅ 实现Local/Remote插件的核心接口
4. ✅ 开发和测试多种AI服务（chat, embed等）
5. ✅ 构建跨平台的插件二进制
6. ✅ 部署和调试插件
7. ✅ 遵循最佳实践确保代码质量

**重要提示**：
- AOG插件通过直接放置到 `plugins/` 目录来部署
- AOG启动时会自动发现并加载插件
- 所有接口定义来自 `plugin-sdk/client/interfaces.go`
- 参考示例代码：`ollama-plugin` 和 `aliyun-plugin`

如有问题，请参考：
- [常见问题](#11-常见问题)
- [ollama-plugin示例](../../plugin-example/ollama-plugin/)
- [aliyun-plugin示例](../../plugin-example/aliyun-plugin/)
- [AOG GitHub仓库](https://github.com/intel/aog)

祝您开发愉快！🚀

