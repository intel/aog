# 文本转语音场景示例

本示例演示如何使用 AOG Text-to-Speech API 将文本转换为语音。

## 📝 场景描述

Text-to-Speech 服务可以：
- 将文本转换为自然的语音
- 支持女声音色
- 生成 WAV 格式音频文件

## ⚠️ 重要限制

- **语言支持**: 本地服务（OpenVINO）当前仅支持英文文本；云服务支持情况请参考各服务商文档
- **音频格式**: 输出为 WAV 格式
- **音色选择**: 本地服务（OpenVINO）当前仅支持 `female`（女声）音色；云服务（如 Aliyun）支持多种音色

## 🎯 学习目标

通过本示例，你将学会：
1. 如何调用 AOG Text-to-Speech API
2. 如何使用 `female` 音色参数
3. 如何保存和播放生成的音频
4. 如何处理 API 响应

## 🔌 API 端点

```
POST http://localhost:16688/aog/v0.2/services/text-to-speech
```

## 📋 请求参数

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `model` | string | 是 | 模型名称，如 `NamoLi/speecht5-tts` |
| `text` | string | 是 | 需要转换的文本。本地服务仅支持英文 |
| `voice` | string | 否 | 音色。本地服务（OpenVINO）仅支持 `female`（女声）；云服务（如 Aliyun）支持多种音色，默认 `female` |

### 请求示例

```json
{
  "model": "NamoLi/speecht5-tts",
  "text": "Hello, this is a text to speech example.",
  "voice": "female"
}
```

## 📊 响应格式

```json
{
  "business_code": 200,
  "message": "success",
  "data": {
    "url": "/Users/xxxx/Downloads/202507171635597494.wav"
  }
}
```

## 🚀 快速开始

### 先决条件

1. ✅ AOG 服务已安装并运行
2. ✅ Text-to-Speech 服务已安装
3. ✅ 已下载所需的 TTS 模型（如 `NamoLi/speecht5-tts`）

### 运行步骤

1. 确保 AOG 服务正在运行
2. 打开 [text-to-speech.ipynb](./text-to-speech.ipynb)
3. 按顺序执行 notebook 中的代码单元格

## 💡 使用技巧

### 1. 使用女声音色

```python
# 本地服务（OpenVINO）使用女声（当前唯一支持的音色）
response = call_aog_tts("Hello world", voice="female")

# 云服务（如 Aliyun）可能支持更多音色，请参考服务商文档
```

### 2. 处理长文本

对于较长的文本，建议分段处理：

```python
long_text = "This is a very long text..."
# 分段处理
segments = long_text.split(". ")
for segment in segments:
    response = call_aog_tts(segment)
```

## 🔍 常见问题

**Q: 支持中文吗？**  
A: 本地服务（OpenVINO）当前版本仅支持英文文本。云服务（如 Aliyun）可能支持中文，请参考服务商文档。

**Q: 可以调整语速吗？**  
A: 当前 API 不支持语速调整，使用默认语速。

**Q: 生成的音频文件在哪里？**  
A: 音频文件路径在响应的 `data.url` 字段中。

**Q: 支持哪些音频格式？**  
A: 当前仅支持 WAV 格式输出。

**Q: 支持哪些音色？**  
A: 本地服务（OpenVINO）仅支持 `female`（女声）音色。云服务（如 Aliyun）支持多种音色，请参考服务商文档。

## 📚 相关资源

- [AOG API 文档](../../docs/)
- [返回主页](../README.md)
- [语音转文本示例](../speech-to-text/)
