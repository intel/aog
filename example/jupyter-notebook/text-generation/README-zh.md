# 文本生成场景示例

本示例演示如何使用 AOG Chat API 进行对话和文本生成。

## 📝 场景描述

文本生成服务基于大语言模型，可以：
- 进行多轮对话
- 回答问题
- 生成创意内容
- 提供建议和帮助

## 🎯 学习目标

通过本示例，你将学会：
1. 如何调用 AOG Chat API
2. 如何构造不同类型的消息（system、user、assistant）
3. 如何配置模型参数
4. 如何处理流式和非流式响应
5. 如何进行错误处理

## 🔌 API 端点

```
POST http://localhost:16688/aog/v0.2/services/chat
```

## 📋 请求参数

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `model` | string | 是 | 模型名称，如 `qwen2.5:0.5b` |
| `messages` | array | 是 | 消息列表，每条消息包含 `role` 和 `content` |
| `stream` | boolean | 否 | 是否使用流式响应，默认 `false` |

### 消息类型

- **system**: 系统消息，用于设置助手的行为和角色
- **user**: 用户消息，表示用户的输入
- **assistant**: 助手消息，表示 AI 的回复

### 请求示例

```json
{
  "model": "qwen2.5:0.5b",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant"
    },
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ],
  "stream": false
}
```

## 📊 响应格式

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "qwen2.5:0.5b",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! I'm doing well, thank you for asking..."
      },
      "finish_reason": "stop"
    }
  ]
}
```

## 🚀 快速开始

### 先决条件

1. ✅ AOG 服务已安装并运行
2. ✅ Chat 服务已安装
3. ✅ 已下载所需的聊天模型（如 `qwen2.5:0.5b`）

### 运行步骤

1. 确保 AOG 服务正在运行
2. 打开 [text-generation.ipynb](./text-generation.ipynb)
3. 按顺序执行 notebook 中的代码单元格

## 💡 使用技巧

### 1. 设置系统提示词

使用 system 消息来定义 AI 助手的角色和行为：

```python
messages = [
    {"role": "system", "content": "你是一个专业的 Python 编程助手"},
    {"role": "user", "content": "如何读取 JSON 文件？"}
]
```

### 2. 多轮对话

保持对话历史，实现上下文连贯的多轮对话：

```python
messages = [
    {"role": "user", "content": "什么是机器学习？"},
    {"role": "assistant", "content": "机器学习是..."},
    {"role": "user", "content": "能举个例子吗？"}
]
```

### 3. 控制输出风格

通过 system 消息控制输出的风格和格式：

```python
{"role": "system", "content": "请用简洁的语言回答，每个答案不超过 50 字"}
```

## 🔍 常见问题

**Q: 如何选择合适的模型？**  
A: 不同模型有不同的特点。`qwen2.5:0.5b` 是一个轻量级模型，适合快速响应。可以通过 AOG 服务查询可用的模型列表。

**Q: stream 参数有什么用？**  
A: `stream=true` 会逐字返回结果，适合需要实时显示的场景；`stream=false` 会等待完整结果后一次性返回。

**Q: 如何限制回复长度？**  
A: 可以在 system 消息中明确要求，或者使用模型的 `max_tokens` 参数（如果支持）。

## 📚 相关资源

- [AOG API 文档](../../docs/)
- [返回主页](../README.md)
- [文本转图像示例](../text-to-image/)
