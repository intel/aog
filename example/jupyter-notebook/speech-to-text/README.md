# Speech-to-Text Example

This example demonstrates how to use the AOG Speech-to-Text API to convert audio into text.

## 📝 Scenario Description

The Speech-to-Text service can:
- Convert audio files into text
- Support multiple languages (Chinese, English, etc.)
- Provide timeline segment information
- Recognize long audio files

## 🎯 Learning Objectives

Through this example, you will learn:
1. How to call the AOG Speech-to-Text API
2. How to process audio files (base64 encoding)
3. How to parse recognition results
4. How to handle timeline information

## 🔌 API Endpoint

```
POST http://localhost:16688/aog/v0.2/services/speech-to-text
```

## 📋 Request Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | Model name, e.g., `NamoLi/whisper-large-v3-ov` |
| `audio` | string | Yes | Base64 encoded audio file |
| `language` | string | No | Language code, e.g., `zh` (Chinese), `en` (English) |

### Request Example

```json
{
  "model": "NamoLi/whisper-large-v3-ov",
  "audio": "base64_encoded_audio_data",
  "language": "zh"
}
```

## 📊 Response Format

```json
{
  "segments": [
    {
      "id": 0,
      "start": "00:00:00.000",
      "end": "00:00:03.500",
      "text": "Recognized text content"
    }
  ]
}
```

## 🚀 Quick Start

### Prerequisites

1. ✅ AOG service is installed and running
2. ✅ Speech-to-Text service is installed
3. ✅ Required ASR model is downloaded (e.g., `NamoLi/whisper-large-v3-ov`)
4. ✅ Test audio files are prepared (WAV, MP3, etc.)

### Steps

1. Ensure AOG service is running
2. Prepare test audio files
3. Open [speech-to-text.ipynb](./speech-to-text.ipynb)
4. Execute the code cells in the notebook sequentially

## 💡 Usage Tips

### 1. Supported Audio Formats

- WAV
- MP3
- M4A
- FLAC
- Other common formats

### 2. Language Recognition

```python
# Chinese recognition
response = call_aog_stt(audio_base64, language="zh")

# English recognition
response = call_aog_stt(audio_base64, language="en")

# Auto-detect language (omit language parameter)
response = call_aog_stt(audio_base64)
```

### 3. Handle Long Audio

For long audio files, the API automatically segments the results, with each segment containing timeline information.

## 🔍 FAQ

**Q: What languages are supported?**  
A: Supports Chinese, English, and many other languages, depending on the model used.

**Q: Is there a file size limit?**  
A: Recommend files under 25MB; larger files should be processed in segments.

**Q: How to improve recognition accuracy?**  
A: 
- Use clear audio
- Reduce background noise
- Choose appropriate language parameter
- Use larger models

**Q: How to handle timeline information?**  
A: The `segments` array in the response contains start and end times for each text segment, useful for subtitle generation and other scenarios.

## 📚 Related Resources

- [AOG API Documentation](../../docs/)
- [Back to Main](../README.md)
- [Text-to-Speech Example](../text-to-speech/)
- [Real-time Speech Recognition Example](../speech-to-text-ws/)
