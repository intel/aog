===========================================
AOG 插件开发指南
===========================================

概述
====

AOG (AIPC Open Gateway) 插件是模块化扩展，允许开发者将新的 AI 引擎或服务集成到 AOG 生态系统中。本指南提供了创建、构建和部署 AOG 插件的全面指导。

插件类型
========

AOG 支持两种类型的插件：

1. **本地插件**: 管理本地 AI 引擎（例如 Ollama、OpenVINO）
2. **远程插件**: 连接到云 AI 服务（例如 OpenAI、DeepSeek、Aliyun、Tencent、Baidu）

前置要求
========

在开发 AOG 插件之前，请确保您已安装：

- Go 1.23 或更高版本
- AOG 插件 SDK
- 对 gRPC 和 Protocol Buffers 的基本了解

快速开始
========

1. 项目设置
-----------

创建一个新的插件目录并初始化 Go 模块：

.. code-block:: bash

    mkdir my-engine-plugin
    cd my-engine-plugin
    go mod init github.com/yourusername/my-engine-plugin

添加 AOG 插件 SDK 依赖：

.. code-block:: bash

    go get github.com/intel/aog/plugin-sdk@latest

2. 插件结构
-----------

典型的 AOG 插件遵循以下结构：

::

    my-engine-plugin/
    ├── plugin.yaml          # 插件元数据
    ├── main.go             # 插件入口点
    ├── go.mod              # Go 模块定义
    ├── go.sum              # 依赖项
    ├── internal/           # 插件实现
    │   ├── provider.go     # Provider 实现
    │   ├── config.go       # 配置管理
    │   ├── client.go       # 引擎客户端
    │   └── services/       # 服务处理器
    │       ├── chat.go
    │       └── embed.go
    ├── bin/                # 构建产物
    └── README.md           # 文档

3. 插件元数据 (plugin.yaml)
---------------------------

创建一个 [plugin.yaml] 文件来定义插件的元数据：

.. code-block:: yaml

    version: "1.0"

    provider:
      name: my-engine-plugin
      display_name: My Engine Plugin
      version: 1.0.0
      type: local  # 或 remote
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

    resources:
      data_dir: "${AOG_DATA_DIR}/engine/my-engine"
      my_engine:
        executable: "${DATA_DIR}/bin/engine"
        models_dir: "${DATA_DIR}/models"
        download_dir: "${HOME}/Downloads"

插件元数据字段说明
------------------

**config_ref 字段 (重要)**

``config_ref`` 用于复用 AOG 内置的 API 转换规则，避免重复实现 API 格式转换逻辑。

格式：``"flavor:service"``

示例：

.. code-block:: yaml

    services:
      - service_name: chat
        # ... 其他配置 ...
        config_ref: ollama:chat  # 复用内置 ollama 的 chat 转换规则

使用 ``config_ref`` 后，AOG 会自动处理 API 格式转换，插件只需处理原生的请求和响应。

可用的内置转换规则：

- ``ollama:chat`` - Ollama Chat API
- ``ollama:embed`` - Ollama Embeddings API  
- ``ollama:generate`` - Ollama Generate API
- ``aliyun:chat`` - 阿里云百炼 Chat API
- ``aliyun:embed`` - 阿里云百炼 Embeddings API
- ``aliyun:text-to-image`` - 阿里云文生图 API
- 其他内置模板请参考 AOG 源码

**环境变量支持**

``resources`` 部分支持以下环境变量，用于动态路径配置：

.. list-table::
   :header-rows: 1
   :widths: 20 40 40

   * - 变量名
     - 说明
     - 默认值
   * - ``${AOG_DATA_DIR}``
     - AOG 统一数据目录
     - macOS: ``~/Library/Application Support/AOG``
       
       Linux: ``/var/lib/aog``
       
       Windows: ``%LOCALAPPDATA%/AOG``
   * - ``${DATA_DIR}``
     - 插件数据根目录
     - 由 ``data_dir`` 字段展开后的值
   * - ``${PLUGIN_DIR}``
     - 插件可执行文件所在目录
     - 插件二进制文件的父目录
   * - ``${HOME}``
     - 用户主目录
     - 系统 HOME 环境变量

**认证字段说明**

``auth_fields`` 字段用于声明服务需要哪些认证参数，这些参数由 AOG 统一管理，插件无需实现认证逻辑。

常见的认证类型：

- ``auth_type: none`` - 无需认证
- ``auth_type: apikey`` - API Key 认证，需要 ``auth_fields: [api_key]``
- ``auth_type: token`` - Token 认证，需要指定具体的 token 字段

AOG 会自动将配置的认证信息注入到插件的服务请求中。

**其他重要字段说明**

.. list-table::
   :header-rows: 1
   :widths: 20 60 20

   * - 字段名
     - 说明
     - 是否必需
   * - ``special_url``
     - 特殊 URL，用于某些服务的特定端点（如 WebSocket 连接）
     - 否
   * - ``extra_url``
     - 额外 URL，用于异步任务查询等辅助接口
     - 否
   * - ``extra_header``
     - 额外的 HTTP 头，JSON 格式字符串，如 ``{"X-Custom": "value"}``
     - 否
   * - ``expose_protocol``
     - 服务暴露的协议类型：``HTTP`` 或 ``WEBSOCKET``
     - 是
   * - ``capabilities.support_streaming``
     - 是否支持服务器端流式响应（SSE/NDJSON）
     - 是
   * - ``capabilities.support_bidirectional``
     - 是否支持双向流式通信（WebSocket）
     - 是

4. 插件入口点 (main.go)
-----------------------

创建插件的主入口点：

.. code-block:: go

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

        // 创建 provider
        provider, err := internal.NewMyEngineProvider(config)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to create provider: %v\n", err)
            os.Exit(1)
        }

        // 启动插件服务
        plugin.Serve(&plugin.ServeConfig{
            HandshakeConfig: server.PluginHandshake,
            Plugins: map[string]plugin.Plugin{
                server.PluginTypeProvider: server.NewProviderPlugin(provider),
            },
            GRPCServer: plugin.DefaultGRPCServer,
        })
    }

核心接口
========

PluginProvider (必需)
---------------------

所有插件都必须实现此接口：

.. code-block:: go

    type PluginProvider interface {
        GetManifest() *types.PluginManifest
        GetOperateStatus() int
        SetOperateStatus(status int)
        HealthCheck(ctx context.Context) error
        InvokeService(ctx context.Context, serviceName string, request []byte) ([]byte, error)
    }

LocalPluginProvider (用于本地插件)
----------------------------------

本地插件必须实现额外的方法来管理引擎：

.. code-block:: go

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

RemotePluginProvider (用于远程插件)
----------------------------------

远程插件需要实现额外的方法来应用 AOG 提供的认证信息：

.. code-block:: go

    type RemotePluginProvider interface {
        PluginProvider
        
        // 应用认证信息（由 AOG 提供）
        SetAuth(req *http.Request, authInfo string, credentials map[string]string) error
        
        // 验证认证信息是否有效
        ValidateAuth(ctx context.Context) error
        
        // 刷新认证信息（用于 OAuth）
        RefreshAuth(ctx context.Context) error
    }

**注意：** 认证信息（API Key、Token 等）由 AOG 统一管理和配置，插件只需实现如何将这些信息应用到实际的 API 请求中。

构建插件
========

Makefile 方法
-------------

创建一个 [Makefile] 用于跨平台构建：

.. code-block:: makefile

    VERSION ?= 1.0.0
    BINARY_NAME = my-engine-plugin

    PLATFORMS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64

    .PHONY: build
    build:
        @echo "Building for current platform..."
        @go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY_NAME) .

    .PHONY: build-all
    build-all: $(PLATFORMS)

    .PHONY: linux-amd64
    linux-amd64:
        @GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/linux-amd64/$(BINARY_NAME) .

    .PHONY: linux-arm64
    linux-arm64:
        @GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/linux-arm64/$(BINARY_NAME) .

    .PHONY: darwin-amd64
    darwin-amd64:
        @GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/darwin-amd64/$(BINARY_NAME) .

    .PHONY: darwin-arm64
    darwin-arm64:
        @GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/darwin-arm64/$(BINARY_NAME) .

    .PHONY: windows-amd64
    windows-amd64:
        @GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/windows-amd64/$(BINARY_NAME).exe .

    .PHONY: clean
    clean:
        @rm -rf bin/

构建命令
--------

.. code-block:: bash

    # 为当前平台构建
    make build

    # 为所有平台构建
    make build-all

    # 清理构建产物
    make clean

部署
====

插件目录
--------

AOG 会根据不同操作系统在以下位置搜索插件：

.. list-table::
   :header-rows: 1
   :widths: 20 60 20

   * - 操作系统
     - 插件目录路径
     - 优先级
   * - **Linux**
     - ``/var/lib/aog/plugins``
     - 系统级
   * - **macOS**
     - ``~/Library/Application Support/AOG/plugins``
     - 用户级
   * - **Windows**
     - ``%APPDATA%\AOG\plugins``
       
       (通常为 ``C:\Users\<用户名>\AppData\Roaming\AOG\plugins``)
     - 用户级

**自定义插件目录**

您可以通过设置环境变量 ``AOG_PLUGIN_DIR`` 来自定义插件目录：

.. code-block:: bash

    # Linux/macOS
    export AOG_PLUGIN_DIR=/path/to/custom/plugins
    
    # Windows (PowerShell)
    $env:AOG_PLUGIN_DIR="C:\path\to\custom\plugins"
    
    # Windows (CMD)
    set AOG_PLUGIN_DIR=C:\path\to\custom\plugins

**开发模式**

在开发过程中，如果当前工作目录包含 ``plugins/`` 子目录，AOG 会优先使用该目录：

.. code-block:: bash

    # 当前目录结构
    my-project/
    ├── plugins/              # ← 开发时 AOG 会优先使用此目录
    │   ├── my-engine-plugin/
    │   └── another-plugin/
    └── ...

这使得开发和测试更加便捷，无需手动配置环境变量。

本地开发
--------

对于开发，您可以创建一个符号链接到您的插件：

.. code-block:: bash

    # Linux/macOS - 创建符号链接到用户插件目录
    ln -s $(pwd) ~/Library/Application\ Support/AOG/plugins/my-engine-plugin  # macOS
    ln -s $(pwd) /var/lib/aog/plugins/my-engine-plugin                         # Linux
    
    # Windows (需要管理员权限)
    mklink /D "%APPDATA%\AOG\plugins\my-engine-plugin" "%CD%"

安装
----

使用 AOG CLI 安装插件：

.. code-block:: bash

    # 从当前目录安装插件
    aog plugin install .

    # 启用插件
    aog plugin enable my-engine-plugin

测试
====

手动测试
--------

测试您的插件服务：

.. code-block:: bash

    # 测试聊天服务
    curl -X POST http://localhost:16688/aog/v0.2/services/chat \
      -H "Content-Type: application/json" \
      -d '{"messages": [{"role": "user", "content": "Hello!"}]}'


示例插件
========

参考 AOG 仓库中的现有插件：

- [plugin-example/ollama-plugin/]: 本地插件示例
- [plugin-example/aliyun-plugin/]: 远程插件示例

远程插件示例 (Aliyun Plugin)
----------------------------

阿里云插件是一个典型的远程插件，支持多种AI服务：

.. code-block:: yaml

    version: "1.0"

    provider:
      name: aliyun-plugin
      display_name: Aliyun Plugin (External)
      version: 1.0.0
      type: remote
      author: AOG Team
      description: External Aliyun plugin for local LLM inference
      homepage: https://github.com/intel/aog
      engine_host: https://dashscope.aliyuncs.com

    services:
      - service_name: chat
        task_type: text-generation
        protocol: HTTP
        expose_protocol: HTTP
        endpoint: /compatible-mode/v1/chat/completions
        auth_type: apikey
        auth_fields:
          - api_key
        default_model: qwen-max
        support_models:
          - qwen-max
          - qwen-plus
        config_ref: aliyun:chat
        capabilities:
          support_streaming: true
          support_bidirectional: false

      - service_name: embed
        task_type: embedding
        protocol: HTTP
        expose_protocol: HTTP
        endpoint: /compatible-mode/v1/embeddings
        auth_type: apikey
        auth_fields:
          - api_key
        default_model: text-embedding-v1
        support_models:
          - text-embedding-v1
        config_ref: aliyun:embed
        capabilities:
          support_streaming: false
          support_bidirectional: false

    platforms:
      linux_amd64:
        executable: bin/linux-amd64/aliyun-plugin
      darwin_arm64:
        executable: bin/darwin-arm64/aliyun-plugin
      windows_amd64:
        executable: bin/windows-amd64/aliyun-plugin.exe

本地插件示例 (Ollama Plugin)
----------------------------

Ollama插件是一个典型的本地插件，管理本地AI引擎：

.. code-block:: yaml

    version: "1.0"

    provider:
      name: ollama-plugin
      display_name: Ollama Plugin (External)
      version: 1.0.0
      type: local
      author: AOG Team
      description: External Ollama plugin for local LLM inference
      homepage: https://github.com/intel/aog
      engine_host: "http://127.0.0.1:16677"

    services:
      - service_name: chat
        task_type: text-generation
        protocol: HTTP
        expose_protocol: HTTP
        endpoint: /api/chat
        auth_type: none
        default_model: qwen3:0.6b
        support_models:
          - qwen3:0.6b
        config_ref: ollama:chat
        capabilities:
          support_streaming: true
          support_bidirectional: false

    platforms:
      linux_amd64:
        executable: bin/linux-amd64/ollama-plugin
      darwin_arm64:
        executable: bin/darwin-arm64/ollama-plugin
      windows_amd64:
        executable: bin/windows-amd64/ollama-plugin.exe

    resources:
      data_dir: "${AOG_DATA_DIR}/engine/ollama"
      ollama:
        executable: "${DATA_DIR}/bin/ollama"
        models_dir: "${DATA_DIR}/models"
        download_dir: "${HOME}/Downloads"

Demo Plugin
===========

创建一个简单的Demo插件来演示插件开发流程：

1. 创建插件目录结构：

.. code-block:: bash

    mkdir demo-plugin
    cd demo-plugin
    go mod init github.com/yourusername/demo-plugin
    go get github.com/intel/aog/plugin-sdk@latest

2. 创建 plugin.yaml：

.. code-block:: yaml

    version: "1.0"

    provider:
      name: demo-plugin
      display_name: Demo Plugin
      version: 1.0.0
      type: local
      author: AOG Developer
      description: A demo plugin for learning AOG plugin development
      homepage: https://github.com/yourusername/demo-plugin
      engine_host: "http://127.0.0.1:8080"

    services:
      - service_name: chat
        task_type: text-generation
        protocol: HTTP
        expose_protocol: HTTP
        endpoint: /api/chat
        auth_type: none
        default_model: demo-model
        support_models:
          - demo-model
        capabilities:
          support_streaming: false
          support_bidirectional: false

    platforms:
      linux_amd64:
        executable: bin/linux-amd64/demo-plugin
      darwin_arm64:
        executable: bin/darwin-arm64/demo-plugin
      windows_amd64:
        executable: bin/windows-amd64/demo-plugin.exe

3. 创建 main.go：

.. code-block:: go

    package main

    import (
        "context"
        "fmt"
        "os"

        "github.com/hashicorp/go-plugin"
        "github.com/intel/aog/plugin-sdk/adapter"
        "github.com/intel/aog/plugin-sdk/server"
        "github.com/intel/aog/plugin-sdk/types"
    )

    type DemoProvider struct {
        *adapter.LocalPluginAdapter
    }

    func NewDemoProvider() (*DemoProvider, error) {
        manifest, err := types.LoadManifest(".")
        if err != nil {
            return nil, fmt.Errorf("failed to load manifest: %w", err)
        }

        localAdapter := adapter.NewLocalPluginAdapter(manifest)
        provider := &DemoProvider{
            LocalPluginAdapter: localAdapter,
        }

        return provider, nil
    }

    func (p *DemoProvider) InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error) {
        p.LogInfo(fmt.Sprintf("Demo plugin received request for service: %s", serviceName))
        
        switch serviceName {
        case "chat":
            // 实际项目中，应该解析请求、调用后端 API、处理响应
            // 这里展示一个简化的示例
            
            // 1. 解析请求（示例：假设请求包含 messages 字段）
            // var req ChatRequest
            // if err := json.Unmarshal(request, &req); err != nil {
            //     return nil, fmt.Errorf("failed to parse request: %w", err)
            // }
            
            // 2. 调用实际的 AI 服务（示例：HTTP 调用）
            // resp, err := http.Post("http://your-ai-service/chat", "application/json", bytes.NewReader(request))
            // if err != nil {
            //     return nil, fmt.Errorf("failed to call AI service: %w", err)
            // }
            
            // 3. 处理响应并返回
            response := `{"message": {"role": "assistant", "content": "Hello from demo plugin!"}}`
            return []byte(response), nil
        default:
            return nil, fmt.Errorf("unsupported service: %s", serviceName)
        }
    }

    func (p *DemoProvider) StartEngine(mode string) error {
        p.LogInfo("Demo plugin engine started")
        return nil
    }

    func (p *DemoProvider) StopEngine() error {
        p.LogInfo("Demo plugin engine stopped")
        return nil
    }

    func (p *DemoProvider) HealthCheck(ctx context.Context) error {
        p.LogInfo("Demo plugin health check passed")
        return nil
    }

    func main() {
        provider, err := NewDemoProvider()
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to create provider: %v\n", err)
            os.Exit(1)
        }

        plugin.Serve(&plugin.ServeConfig{
            HandshakeConfig: server.PluginHandshake,
            Plugins: map[string]plugin.Plugin{
                server.PluginTypeProvider: server.NewProviderPlugin(provider),
            },
            GRPCServer: plugin.DefaultGRPCServer,
        })
    }


4. 构建和部署插件
-----------------

### 4.1 构建插件

使用以下命令构建当前平台的插件：

.. code-block:: bash

    # 构建当前平台的插件
    go build -o bin/aliyun-plugin .

或者使用构建脚本构建所有支持平台的插件：

.. code-block:: bash

    # 构建所有平台插件
    ./build-all.sh

### 4.2 插件目录结构要求

构建完成后，请按照以下标准目录结构组织插件文件，然后将其放入 AOG 的 `plugins` 目录中：

::

    aliyun-plugin/                # 插件根目录（目录名即插件ID）
    ├── plugin.yaml              # 插件元数据（必需）
    └── bin/                     # 可执行文件目录（必需）
        ├── linux-amd64/         # Linux x86_64 平台
        │   └── aliyun-plugin    # 可执行文件（与plugin.yaml中配置一致）
        ├── linux-arm64/         # Linux ARM64 平台
        │   └── aliyun-plugin
        ├── darwin-amd64/        # macOS Intel 平台
        │   └── aliyun-plugin
        ├── darwin-arm64/        # macOS Apple Silicon 平台
        │   └── aliyun-plugin
        └── windows-amd64/       # Windows x86_64 平台
            └── aliyun-plugin.exe

### 4.3 测试插件

插件部署完成后，可以使用以下命令测试插件是否正常工作：

.. code-block:: bash

    # 测试对话服务
    curl -X POST http://localhost:16688/v0.2/services/chat \
      -H "Content-Type: application/json" \
      -d '{
        "provider": "aliyun-plugin", 
        "service": "chat", 
        "data": {
          "model": "qwen-max",
          "messages": [
            {"role": "system", "content": "你是一个AI助手"},
            {"role": "user", "content": "你好，请介绍一下你自己"}
          ]
        }
      }'

故障排除
========

常见问题
--------

1. **插件无法加载**: 检查 plugin.yaml 语法和文件路径
2. **引擎无法启动**: 验证引擎安装和权限
3. **服务调用失败**: 检查网络连接和 API 端点
4. **认证失败**: 检查远程插件的认证配置

调试
----

启用详细日志：

.. code-block:: bash

    # 以详细日志模式启动 AOG
    aog server start -v

结论
====

AOG 插件提供了一种强大的方式来扩展平台，集成新的 AI 引擎和服务。通过遵循本指南并利用插件 SDK，您可以创建健壮的跨平台插件，无缝集成到 AOG 生态系统中。