# 语音转文本场景示例

本示例演示如何使用 AOG Speech-to-Text API 将语音转换为文本。

## 📝 场景描述

Speech-to-Text 服务可以：
- 将音频文件转换为文本
- 支持多种语言（中文、英文等）
- 提供时间轴分段信息
- 识别长音频文件

## 🎯 学习目标

通过本示例，你将学会：
1. 如何调用 AOG Speech-to-Text API
2. 如何处理音频文件（base64 编码）
3. 如何解析识别结果
4. 如何处理时间轴信息

## 🔌 API 端点

```
POST http://localhost:16688/aog/v0.2/services/speech-to-text
```

## 📋 请求参数

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `model` | string | 是 | 模型名称，如 `NamoLi/whisper-large-v3-ov` |
| `audio` | string | 是 | 音频文件的 base64 编码 |
| `language` | string | 否 | 语言代码，如 `zh`（中文）、`en`（英文） |

### 请求示例

```json
{
  "model": "NamoLi/whisper-large-v3-ov",
  "audio": "base64编码的音频数据",
  "language": "zh"
}
```

## 📊 响应格式

```json
{
  "segments": [
    {
      "id": 0,
      "start": "00:00:00.000",
      "end": "00:00:03.500",
      "text": "识别的文本内容"
    }
  ]
}
```

## 🚀 快速开始

### 先决条件

1. ✅ AOG 服务已安装并运行
2. ✅ Speech-to-Text 服务已安装
3. ✅ 已下载所需的 ASR 模型（如 `NamoLi/whisper-large-v3-ov`）
4. ✅ 准备测试音频文件（WAV、MP3 等格式）

### 运行步骤

1. 确保 AOG 服务正在运行
2. 准备测试音频文件
3. 打开 [speech-to-text.ipynb](./speech-to-text.ipynb)
4. 按顺序执行 notebook 中的代码单元格

## 💡 使用技巧

### 1. 支持的音频格式

- WAV
- MP3
- M4A
- FLAC
- 其他常见格式

### 2. 语言识别

```python
# 中文识别
response = call_aog_stt(audio_base64, language="zh")

# 英文识别
response = call_aog_stt(audio_base64, language="en")

# 自动检测语言（不指定 language 参数）
response = call_aog_stt(audio_base64)
```

### 3. 处理长音频

对于长音频文件，API 会自动分段返回结果，每段包含时间轴信息。

## 🔍 常见问题

**Q: 支持哪些语言？**  
A: 支持中文、英文等多种语言，具体取决于使用的模型。

**Q: 音频文件大小有限制吗？**  
A: 建议单个文件不超过 25MB，过大的文件建议分段处理。

**Q: 识别准确率如何提高？**  
A: 
- 使用清晰的音频
- 减少背景噪音
- 选择合适的语言参数
- 使用更大的模型

**Q: 如何处理时间轴信息？**  
A: 响应中的 `segments` 数组包含每段文本的开始和结束时间，可用于字幕生成等场景。

## 📚 相关资源

- [AOG API 文档](../../docs/)
- [返回主页](../README.md)
- [文本转语音示例](../text-to-speech/)
- [实时语音识别示例](../speech-to-text-ws/)
