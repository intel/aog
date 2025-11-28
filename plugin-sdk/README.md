# AOG Plugin SDK

## 概述

AOG Plugin SDK 是一个**完全独立**的Go模块，用于开发AOG (AIPC Open Gateway) 插件。

### 核心设计原则

- ✅ **零依赖AOG内部包**：SDK不依赖`github.com/intel/aog/internal/*`任何包
- ✅ **独立编译**：Plugin可以独立编译和分发，无需AOG主程序代码
- ✅ **跨平台支持**：支持Linux、macOS、Windows，支持amd64和arm64架构
- ✅ **清晰的通信协议**：基于gRPC + Protocol Buffers，定义明确的插件接口

---

## 目录结构

```
plugin-sdk/
├── protocol/           # gRPC协议定义
│   ├── plugin.proto   # Protocol Buffers定义
│   ├── plugin.pb.go   # 生成的protobuf代码
│   └── plugin_grpc.pb.go  # 生成的gRPC代码
├── types/             # SDK类型定义
│   ├── metadata.go    # Plugin元数据类型
│   ├── service.go     # 服务相关类型
│   └── common.go      # 通用类型
├── adapter/           # 基础Adapter实现
│   ├── base.go        # BasePluginProvider
│   ├── local.go       # LocalPluginAdapter (本地引擎插件)
│   └── remote.go      # RemotePluginAdapter (远程API插件)
└── server/            # gRPC Server实现
    ├── provider.go    # ProviderPlugin
    └── grpc_server.go # gRPC服务器实现
```

---

## 快速开始

### 1. 导入SDK

在你的plugin的`go.mod`中添加：

```go
require (
    github.com/intel/aog/plugin-sdk v0.1.0
)
```

### 2. 实现Plugin Provider

```go
package main

import (
    "github.com/intel/aog/plugin-sdk/adapter"
    "github.com/intel/aog/plugin-sdk/server"
    "github.com/intel/aog/plugin-sdk/types"
    "github.com/hashicorp/go-plugin"
)

// MyProvider 实现插件逻辑
type MyProvider struct {
    *adapter.BasePluginProvider
}

func (p *MyProvider) InvokeService(ctx context.Context, serviceName string, request []byte) ([]byte, error) {
    // 实现服务调用逻辑
    return nil, nil
}

func main() {
    provider := &MyProvider{
        BasePluginProvider: adapter.NewBasePluginProvider("my-plugin", "1.0.0"),
    }

    // 启动插件服务
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: server.DefaultHandshake,
        Plugins: map[string]plugin.Plugin{
            "provider": server.NewProviderPlugin(provider),
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

### 3. 编译和分发

```bash
# 编译单个平台
go build -o my-plugin .

# 跨平台编译
GOOS=linux GOARCH=amd64 go build -o my-plugin-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o my-plugin-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o my-plugin-windows-amd64.exe .
```

---

## Plugin类型

### Local Plugin (本地引擎插件)

管理本地AI引擎（如Ollama, OpenVINO）：

```go
type MyLocalProvider struct {
    *adapter.LocalPluginAdapter
}

// 实现生命周期管理
func (p *MyLocalProvider) StartEngine(mode string) error { /* ... */ }
func (p *MyLocalProvider) StopEngine() error { /* ... */ }
func (p *MyLocalProvider) CheckEngine() bool { /* ... */ }
```

### Remote Plugin (远程API插件)

对接云端AI服务（如OpenAI, Anthropic）：

```go
type MyRemoteProvider struct {
    *adapter.RemotePluginAdapter
}

// 实现认证
func (p *MyRemoteProvider) ValidateCredentials(creds map[string]string) error { /* ... */ }
```

---

## 开发指南

### 类型系统

所有类型定义在`types/`包中，**不依赖**AOG内部类型：

- `types.PluginManifest`：插件元数据
- `types.ServiceRequest/Response`：服务请求响应
- `types.ModelInfo`：模型信息

### Adapter模式

SDK提供三层Adapter简化开发：

1. **BasePluginProvider**：所有插件的基础，提供日志、状态管理
2. **LocalPluginAdapter**：本地引擎插件，扩展生命周期管理
3. **RemotePluginAdapter**：远程API插件，扩展认证管理

### gRPC通信

Plugin与AOG通过gRPC通信，SDK已实现所有样板代码，你只需实现业务逻辑。

---

## 版本兼容性

| SDK版本 | AOG版本 | Go版本 |
|---------|---------|--------|
| 0.1.0   | ≥ 0.6.0 | ≥ 1.23 |

---

## 许可证

Apache License 2.0

---

## 贡献

欢迎贡献！请确保：
- SDK保持零依赖AOG内部包
- 添加充分的单元测试
- 更新文档

---

## 示例

完整示例请参考：
- `examples/ollama-plugin/`：本地引擎插件示例
- `examples/openai-plugin/`：远程API插件示例

## 文档

有关插件开发的详细指南，请参阅：
- [AOG 插件开发指南](../docs/zh-cn/source/aog插件开发指南.rst)
- [AOG Plugin Development Guide (English)](../docs/en/source/aog_plugin_development_guide.md)

