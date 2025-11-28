# Tencent Plugin (External)

一个外置的 Tencent 远程插件，用于在 AOG 中使用 Tencent API 进行 LLM 推理。

## 📖 概述

这个插件演示了如何创建一个调用远程 Tencent 服务的 AOG 插件。它提供了：

- 完整的 Tencent API 集成
- Chat、Embedding、Text-to-Image 和 Text-to-Speech 服务支持
- 远程推理能力

## 🧪 测试说明

### 测试环境要求

1. **AOG 服务依赖**：插件测试必须依赖 AOG 服务，请先启动 AOG 服务：
   ```bash
   aog server start
   ```

2. **认证信息**：
   - 插件本身不存储任何认证信息
   - 所有认证信息（如 API Key）需要在 AOG 服务中配置
   - 插件仅负责接收并使用 AOG 服务传递的认证信息进行认证操作

## 🚀 快速开始

### 前提条件

1. **获取 Tencent API Credentials**

请先在腾讯云平台获取 API 凭据：

- 访问 [腾讯云控制台](https://console.cloud.tencent.com/)
- 创建密钥并获取 SecretId 和 SecretKey

### 1. 编译插件

```bash
go mod tidy
go build -o tencent-plugin .
```

### 2. 部署插件

```bash
# 复制到插件目录
mkdir -p ~/.config/aog/plugins/tencent-plugin
cp -r . ~/.config/aog/plugins/tencent-plugin

# 或创建符号链接
ln -s $(pwd) ~/.config/aog/plugins/tencent-plugin
```

### 3. 启用插件

```bash
aog server restart
```

## 🔌 API 使用示例

### 对话服务

```bash
curl -X POST http://localhost:16688/v0.2/services/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-aog-token" \
  -d '{
    "model": "hunyuan-turbo",
    "messages": [
      {"role": "system", "content": "你是一个有用的助手"},
      {"role": "user", "content": "你好，请介绍一下你自己"}
    ],
    "stream": true
  }'
```

### 文本嵌入

```bash
curl -X POST http://localhost:16688/v0.2/services/embed \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-aog-token" \
  -d '{
    "model": "hunyuan-embedding",
    "input": "这是一个测试文本"
  }'
```

### 文生图

```bash
curl -X POST http://localhost:16688/v0.2/services/text-to-image \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-aog-token" \
  -d '{
    "model": "hunyuan-DiT",
    "prompt": "一只可爱的小猫在花园里玩耍",
    "n": 1,
    "size": "1024x1024"
  }'
```

## ⚙️ 配置说明

### 环境变量

| 变量 | 必需 | 默认值 | 说明 |
|------|------|--------|------|
| `TENCENT_SECRET_ID` | ✅ | 无 | Tencent Cloud SecretId |
| `TENCENT_SECRET_KEY` | ✅ | 无 | Tencent Cloud SecretKey |

### 支持的服务

#### Chat 服务
- **服务名**: `chat`
- **任务类型**: `text-generation`
- **支持的模型**: hunyuan-turbo, hunyuan-t1-latest, hunyuan-large 等

#### Embedding 服务
- **服务名**: `embed`
- **任务类型**: `embedding`
- **支持的模型**: hunyuan-embedding 等

#### Text-to-Image 服务
- **服务名**: `text-to-image`
- **任务类型**: `text-to-image`
- **支持的模型**: hunyuan-DiT 等

#### Text-to-Speech 服务
- **服务名**: `text-to-speech`
- **任务类型**: `text-to-speech`
- **支持的模型**: qwen-tts 等

## 📁 文件结构

```
tencent-plugin/
├── plugin.yaml       # 插件元数据
├── main.go          # 主程序入口
├── provider.go      # Provider 接口实现
├── config.go        # 配置管理
├── client.go        # Tencent HTTP 客户端
├── env.example      # 配置示例
├── README.md        # 本文档
└── go.mod           # Go 模块定义
```

## 🔧 核心功能

### 已实现

- ✅ **服务**
  - Chat 对话
  - Embedding 向量化
  - Text-to-Image 生成
  - Text-to-Speech 合成

### 与内置 Tencent 的区别

| 特性 | 内置 Tencent | 外置插件 |
|------|------------|---------|
| 自动安装 | ✅ | ❌（需手动安装） |
| 自动启动 | ✅ | ❌（需手动启动） |
| 版本管理 | ✅ | ❌ |
| 模型管理 | ✅ | ✅ |
| 服务调用 | ✅ | ✅ |
| 配置灵活性 | 🟡 | ✅ |

## 💡 使用场景

### 适合使用外置插件的情况

1. ✅ 已经有 Tencent Cloud 凭据
2. ✅ 需要自定义 Tencent 配置
3. ✅ 需要使用远程 Tencent 服务
4. ✅ 需要独立管理 Tencent 版本

### 推荐使用内置引擎的情况

1. ✅ 首次使用 Tencent
2. ✅ 需要自动安装和管理
3. ✅ 希望 AOG 完全托管 Tencent

## 🧪 测试

### 功能测试

1. **测试 Chat**

```bash
# 测试对话
curl -X POST http://localhost:16688/v1/services/chat \
  -H "Content-Type: application/json" \
  -d '{"messages": "Hello", "model": "hunyuan-turbo"}'
```

## 📝 API 使用示例

### Chat

```bash
curl -X POST http://localhost:16688/aog/v0.2/services/chat \
  -H "Content-Type: application/json" \
  -d '{
    "model": "hunyuan-turbo",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### Embedding

```bash
curl -X POST http://localhost:16688/aog/v0.2/services/embed \
  -H "Content-Type: application/json" \
  -d '{
    "model": "hunyuan-embedding",
    "input": "Hello world"
  }'
```

## 🔍 故障排查

### 问题：API 凭据无效

**症状**:
```
Tencent API request failed: 401 Unauthorized
```

**解决方案**:
1. 确认凭据在腾讯云控制台中有效
2. 检查网络连接

### 问题：插件无法启动

**解决方案**:
1. 检查 Tencent API 连接：`curl https://hunyuan.tencentcloudapi.com/`
2. 查看 AOG 日志

### 问题：服务调用失败

**解决方案**:
1. 检查模型名称是否正确
2. 确认凭据有相应服务的权限
3. 查看 AOG 日志

## 🆚 对比其他插件

| 插件 | 类型 | 依赖 | 使用场景 |
|------|------|------|---------|
| **Tencent Plugin** | 远程 | Tencent API 凭据 | Tencent 服务 |
| HTTP API Plugin | 远程 | 无 | 通用 HTTP API |
| Provider Plugin | 示例 | 无 | 学习参考 |

## 📚 参考资料

- [腾讯云官方文档](https://cloud.tencent.com/document/product/1721)
- [腾讯云 API 文档](https://cloud.tencent.com/document/api/1721/101023)
- [AOG 插件开发指南](../../docs/zh-cn/source/aog插件开发指南.rst)

## 🤝 贡献

欢迎提交问题和改进建议！

## 📄 许可证

Apache License 2.0

