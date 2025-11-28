# Ollama Plugin (External)

一个外置的 Ollama 本地插件，用于在 AOG 中使用 Ollama 进行本地 LLM 推理。

## 📖 概述

这个插件演示了如何创建一个调用本地 Ollama 服务的 AOG 插件。它提供了：

- 完整的 Ollama API 集成
- 模型管理（下载、删除、列表）
- Chat 和 Embedding 服务支持
- 本地推理能力

## 🚀 快速开始

### 前提条件

1. **安装 Ollama**

请先安装 Ollama：

```bash
# macOS
brew install ollama

# Linux
curl https://ollama.ai/install.sh | sh

# Windows
# 从 https://ollama.ai 下载安装程序
```

2. **启动 Ollama 服务**

```bash
ollama serve
```

### 1. 配置环境变量

复制配置示例并修改：

```bash
cp env.example .env
# 编辑 .env 文件（如果需要自定义配置）
```

或者直接设置环境变量：

```bash
export OLLAMA_HOST="127.0.0.1:11434"
export OLLAMA_SCHEME="http"
export OLLAMA_DEFAULT_MODEL="llama2"
```

### 2. 编译插件

```bash
go mod tidy
go build -o ollama-plugin .
```

### 3. 验证插件

```bash
# 验证配置
aog plugin lint .

# 测试插件
aog plugin test .
```

### 4. 部署插件

```bash
# 复制到插件目录
mkdir -p ~/.config/aog/plugins/ollama-plugin
cp -r . ~/.config/aog/plugins/ollama-plugin

# 或创建符号链接
ln -s $(pwd) ~/.config/aog/plugins/ollama-plugin
```

### 5. 启用插件

```bash
aog plugin enable ollama-plugin
export AOG_ENABLED_ENGINES="ollama-plugin"
aog server restart
```

## ⚙️ 配置说明

### 环境变量

| 变量 | 必需 | 默认值 | 说明 |
|------|------|--------|------|
| `OLLAMA_HOST` | ❌ | `127.0.0.1:11434` | Ollama 服务地址 |
| `OLLAMA_SCHEME` | ❌ | `http` | 协议（http/https） |
| `OLLAMA_DEFAULT_MODEL` | ❌ | `llama2` | 默认模型 |

### 支持的服务

#### Chat 服务
- **服务名**: `chat`
- **任务类型**: `text-generation`
- **支持的模型**: llama2, llama3, mistral, phi3, qwen 等

#### Embedding 服务
- **服务名**: `embed`
- **任务类型**: `embedding`
- **支持的模型**: nomic-embed-text, mxbai-embed-large 等

## 📁 文件结构

```
ollama-plugin/
├── plugin.yaml       # 插件元数据
├── main.go          # 主程序入口
├── provider.go      # Provider 接口实现
├── config.go        # 配置管理
├── client.go        # Ollama HTTP 客户端
├── env.example      # 配置示例
├── README.md        # 本文档
└── go.mod           # Go 模块定义
```

## 🔧 核心功能

### 已实现

- ✅ **引擎管理**
  - 启动/停止引擎
  - 健康检查
  - 版本查询

- ✅ **模型管理**
  - 下载模型（Pull）
  - 删除模型
  - 列出模型
  - 查看运行中的模型
  - 加载/卸载模型

- ✅ **服务**
  - Chat 对话
  - Embedding 向量化

### 与内置 Ollama 的区别

| 特性 | 内置 Ollama | 外置插件 |
|------|------------|---------|
| 自动安装 | ✅ | ❌（需手动安装） |
| 自动启动 | ✅ | ❌（需手动启动） |
| 版本管理 | ✅ | ❌ |
| 模型管理 | ✅ | ✅ |
| 服务调用 | ✅ | ✅ |
| 配置灵活性 | 🟡 | ✅ |

## 💡 使用场景

### 适合使用外置插件的情况

1. ✅ Ollama 已经安装在系统中
2. ✅ 需要自定义 Ollama 配置
3. ✅ 需要使用远程 Ollama 服务
4. ✅ 需要独立管理 Ollama 版本

### 推荐使用内置引擎的情况

1. ✅ 首次使用 Ollama
2. ✅ 需要自动安装和管理
3. ✅ 希望 AOG 完全托管 Ollama

## 🧪 测试

### 单元测试

```bash
go test ./...
```

### 功能测试

1. **测试健康检查**

```bash
# 确保 Ollama 正在运行
ollama serve &

# 测试插件
aog plugin test .
```

2. **测试模型下载**

```bash
# 通过 Ollama 插件下载模型
curl -X POST http://localhost:16688/v1/models/pull \
  -H "Content-Type: application/json" \
  -d '{"name": "llama2"}'
```

3. **测试 Chat**

```bash
# 测试对话
curl -X POST http://localhost:16688/v1/services/chat \
  -H "Content-Type: application/json" \
  -d '{"messages": "Hello", "model": "llama2"}'
```

## 📝 API 使用示例

### 下载模型

```bash
curl -X POST http://localhost:11434/api/pull \
  -d '{"name": "llama2"}'
```

### 列出模型

```bash
curl http://localhost:11434/api/tags
```

### Chat

```bash
curl -X POST http://localhost:11434/api/chat \
  -d '{
    "model": "llama2",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### Embedding

```bash
curl -X POST http://localhost:11434/api/embeddings \
  -d '{
    "model": "nomic-embed-text",
    "prompt": "Hello world"
  }'
```

## 🔍 故障排查

### 问题：无法连接到 Ollama

**症状**:
```
Ollama health check failed: request failed: connect: connection refused
```

**解决方案**:
1. 确认 Ollama 正在运行：`ps aux | grep ollama`
2. 检查 Ollama 端口：`lsof -i :11434`
3. 手动启动：`ollama serve`
4. 检查 `OLLAMA_HOST` 环境变量

### 问题：模型下载失败

**解决方案**:
1. 检查网络连接
2. 确认模型名称正确
3. 检查磁盘空间
4. 查看 Ollama 日志

### 问题：插件无法启动

**解决方案**:
1. 验证插件配置：`aog plugin lint .`
2. 检查 Ollama 是否安装：`ollama --version`
3. 查看 AOG 日志

## 🆚 对比其他插件

| 插件 | 类型 | 依赖 | 使用场景 |
|------|------|------|---------|
| **Ollama Plugin** | 本地 | Ollama | 本地 LLM 推理 |
| HTTP API Plugin | 远程 | 无 | 通用 HTTP API |
| Provider Plugin | 示例 | 无 | 学习参考 |

## 📚 参考资料

- [Ollama 官方文档](https://github.com/ollama/ollama)
- [Ollama API 文档](https://github.com/ollama/ollama/blob/main/docs/api.md)
- [AOG 插件开发指南](../../docs/zh-cn/source/aog插件开发指南.rst)
- [Plugin SDK 文档](../../plugin-sdk/README.md)
- [Plugin SDK 详细指南](../../plugin-sdk/ENGINE_PLUGIN_DEVELOPMENT_GUIDE.md)

## 🤝 贡献

欢迎提交问题和改进建议！

## 📄 许可证

Apache License 2.0

