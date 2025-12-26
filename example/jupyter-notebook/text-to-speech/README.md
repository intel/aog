# Text-to-Speech Example

This example demonstrates how to use the AOG Text-to-Speech API to convert text into speech.

## 📝 Scenario Description

The Text-to-Speech service can:
- Convert text into natural-sounding speech
- Support female voice
- Generate WAV format audio files

## ⚠️ Important Limitations

- **Language Support**: Local service (OpenVINO) currently only supports English text; cloud services support varies by provider
- **Audio Format**: Output is in WAV format
- **Voice Selection**: Local service (OpenVINO) currently only supports `female` voice; cloud services (e.g., Aliyun) support multiple voices

## 🎯 Learning Objectives

Through this example, you will learn:
1. How to call the AOG Text-to-Speech API
2. How to use the `female` voice parameter
3. How to save and play generated audio
4. How to handle API responses

## 🔌 API Endpoint

```
POST http://localhost:16688/aog/v0.2/services/text-to-speech
```

## 📋 Request Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | Model name, e.g., `NamoLi/speecht5-tts` |
| `text` | string | Yes | Text to convert. Local service only supports English |
| `voice` | string | No | Voice type. Local service (OpenVINO) only supports `female`; cloud services (e.g., Aliyun) support multiple voices, default is `female` |

### Request Example

```json
{
  "model": "NamoLi/speecht5-tts",
  "text": "Hello, this is a text to speech example.",
  "voice": "female"
}
```

## 📊 Response Format

```json
{
  "business_code": 200,
  "message": "success",
  "data": {
    "url": "/Users/xxxx/Downloads/202507171635597494.wav"
  }
}
```

## 🚀 Quick Start

### Prerequisites

1. ✅ AOG service is installed and running
2. ✅ Text-to-Speech service is installed
3. ✅ Required TTS model is downloaded (e.g., `NamoLi/speecht5-tts`)

### Steps

1. Ensure AOG service is running
2. Open [text-to-speech.ipynb](./text-to-speech.ipynb)
3. Execute the code cells in the notebook sequentially

## 💡 Usage Tips

### 1. Using Female Voice

```python
# Local service (OpenVINO) uses female voice (currently the only supported voice)
response = call_aog_tts("Hello world", voice="female")

# Cloud services (e.g., Aliyun) may support more voices, please refer to provider documentation
```

### 2. Handle Long Text

For longer texts, consider processing in segments:

```python
long_text = "This is a very long text..."
# Process in segments
segments = long_text.split(". ")
for segment in segments:
    response = call_aog_tts(segment)
```

## 🔍 FAQ

**Q: Does it support Chinese?**  
A: Local service (OpenVINO) currently only supports English text. Cloud services (e.g., Aliyun) may support Chinese, please refer to provider documentation.

**Q: Can I adjust the speech rate?**  
A: The current API does not support speech rate adjustment; it uses the default rate.

**Q: Where is the generated audio file?**  
A: The audio file path is in the `data.url` field of the response.

**Q: What audio formats are supported?**  
A: Currently only WAV format output is supported.

**Q: What voices are supported?**  
A: Local service (OpenVINO) only supports `female` voice. Cloud services (e.g., Aliyun) support multiple voices, please refer to provider documentation.

## 📚 Related Resources

- [AOG API Documentation](../../docs/)
- [Back to Main](../README.md)
- [Speech-to-Text Example](../speech-to-text/)
