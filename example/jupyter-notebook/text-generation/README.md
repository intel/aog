# Text Generation Example

This example demonstrates how to use the AOG Chat API for conversation and text generation.

## 📝 Scenario Description

The text generation service is based on large language models and can:
- Conduct multi-turn conversations
- Answer questions
- Generate creative content
- Provide suggestions and assistance

## 🎯 Learning Objectives

Through this example, you will learn:
1. How to call the AOG Chat API
2. How to construct different types of messages (system, user, assistant)
3. How to configure model parameters
4. How to handle streaming and non-streaming responses
5. How to implement error handling

## 🔌 API Endpoint

```
POST http://localhost:16688/aog/v0.2/services/chat
```

## 📋 Request Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | Model name, e.g., `qwen2.5:0.5b` |
| `messages` | array | Yes | Message list, each message contains `role` and `content` |
| `stream` | boolean | No | Whether to use streaming response, default `false` |

### Message Types

- **system**: System message, used to set the assistant's behavior and role
- **user**: User message, represents user input
- **assistant**: Assistant message, represents AI's response

### Request Example

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

## 📊 Response Format

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

## 🚀 Quick Start

### Prerequisites

1. ✅ AOG service is installed and running
2. ✅ Chat service is installed
3. ✅ Required chat models are downloaded (e.g., `qwen2.5:0.5b`)

### Steps

1. Ensure AOG service is running
2. Open [text-generation.ipynb](./text-generation.ipynb)
3. Execute the code cells in the notebook sequentially

## 💡 Usage Tips

### 1. Set System Prompt

Use system messages to define the AI assistant's role and behavior:

```python
messages = [
    {"role": "system", "content": "You are a professional Python programming assistant"},
    {"role": "user", "content": "How do I read a JSON file?"}
]
```

### 2. Multi-turn Conversation

Maintain conversation history for contextually coherent multi-turn dialogues:

```python
messages = [
    {"role": "user", "content": "What is machine learning?"},
    {"role": "assistant", "content": "Machine learning is..."},
    {"role": "user", "content": "Can you give an example?"}
]
```

### 3. Control Output Style

Control output style and format through system messages:

```python
{"role": "system", "content": "Please answer concisely, each answer should not exceed 50 words"}
```

## 🔍 Common Questions

**Q: How do I choose the right model?**  
A: Different models have different characteristics. `qwen2.5:0.5b` is a lightweight model suitable for quick responses. You can query the list of available models through the AOG service.

**Q: What is the stream parameter for?**  
A: `stream=true` returns results character by character, suitable for scenarios requiring real-time display; `stream=false` waits for the complete result before returning it all at once.

**Q: How do I limit response length?**  
A: You can explicitly request it in the system message, or use the model's `max_tokens` parameter (if supported).

## 📚 Related Resources

- [AOG API Documentation](../../docs/)
- [Back to Home](../README.md)
- [Text-to-Image Example](../text-to-image/)
