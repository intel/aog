# Deepseek Plugin (External)

一个外置的 Deepseek 远程插件，用于在 AOG 中使用 Deepseek API 进行 LLM 推理。

## 📖 概述

这个插件演示了如何创建一个调用远程 Deepseek 服务的 AOG 插件。它提供了：

- 完整的 Deepseek API 集成
- Chat 服务支持

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

1. **获取 Deepseek API Key**

请先在 Deepseek 平台获取 API Key：

- 访问 [Deepseek API](https://api.deepseek.com/)
- 创建应用并获取 API Key


### 1. 编译插件

```bash
go mod tidy
go build -o deepseek-plugin .
```

### 2. 部署插件

```bash
# 复制到插件目录
mkdir -p ~/.config/aog/plugins/deepseek-plugin
cp -r . ~/.config/aog/plugins/deepseek-plugin

# 或创建符号链接
ln -s $(pwd) ~/.config/aog/plugins/deepseek-plugin
```

### 4. 启用插件

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
    "model": "deepseek-chat",
    "messages": [
      {"role": "system", "content": "你是一个有用的助手"},
      {"role": "user", "content": "你好，请介绍一下你自己"}
    ],
    "stream": true
  }'
```

## 支持的服务

#### Chat 服务
- **服务名**: `chat`
- **任务类型**: `text-generation`
- **支持的模型**: deepseek-chat, deepseek-reasoner 等


## 📁 文件结构

```
deepseek-plugin/
├── plugin.yaml       # 插件元数据
├── main.go          # 主程序入口
├── provider.go      # Provider 接口实现
├── config.go        # 配置管理
├── client.go        # Deepseek HTTP 客户端
├── env.example      # 配置示例
├── README.md        # 本文档
└── go.mod           # Go 模块定义
```

## 🔧 核心功能

### 已实现
- ✅ **服务**
  - Chat 对话

### 与内置 Deepseek 的区别

| 特性 | 内置 Deepseek | 外置插件 |
|------|------------|---------|
| 自动安装 | ✅ | ❌（需手动安装） |
| 自动启动 | ✅ | ❌（需手动启动） |
| 版本管理 | ✅ | ❌ |
| 模型管理 | ✅ | ✅ |
| 服务调用 | ✅ | ✅ |
| 配置灵活性 | 🟡 | ✅ |

## 💡 使用场景

### 适合使用外置插件的情况

1. ✅ 已经有 Deepseek API Key
2. ✅ 需要自定义 Deepseek 配置
3. ✅ 需要使用远程 Deepseek 服务
4. ✅ 需要独立管理 Deepseek 版本

### 推荐使用内置引擎的情况

1. ✅ 首次使用 Deepseek
2. ✅ 需要自动安装和管理
3. ✅ 希望 AOG 完全托管 Deepseek

## 🧪 测试

### 单元测试

```bash
go test ./...
```

### 功能测试

1. **测试健康检查**

```bash
# 测试插件
aog plugin test .
```

2. **测试 Chat**

```bash
# 测试对话
curl -X POST http://localhost:16688/v1/services/chat \
  -H "Content-Type: application/json" \
  -d '{"messages": "Hello", "model": "deepseek-chat"}'
```

## 📝 API 使用示例

### Chat

```bash
curl -X POST http://localhost:16688/aog/v0.2/services/chat \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

## 🔍 故障排查

### 问题：API Key 无效

**症状**:
```
Deepseek API request failed: 401 Unauthorized
```

**解决方案**:
1. 确认 API Key 在 Deepseek 控制台中有效
2。 检查网络连接

### 问题：插件无法启动

**解决方案**:
1. 检查 Deepseek API 连接：`curl https://api.deepseek.com/v1/chat/completions`
2. 查看 AOG 日志

### 问题：服务调用失败

**解决方案**:
1. 检查模型名称是否正确
2. 确认 API Key 有相应服务的权限
3. 查看 AOG 日志

## 🆚 对比其他插件

| 插件 | 类型 | 依赖 | 使用场景 |
|------|------|------|---------|
| **Deepseek Plugin** | 远程 | Deepseek API Key | Deepseek 服务 |
| HTTP API Plugin | 远程 | 无 | 通用 HTTP API |
| Provider Plugin | 示例 | 无 | 学习参考 |

## 📚 参考资料

- [Deepseek 官方文档](https://api.deepseek.com/)
- [Deepseek API 文档](https://api.deepseek.com/docs)
- [AOG 插件开发指南](../../docs/zh-cn/source/aog插件开发指南.rst)

## 🤝 贡献

欢迎提交问题和改进建议！

## 📄 许可证

Apache License 2.0

