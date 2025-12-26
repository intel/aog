# Speech-to-Text-WS (Real-time Speech Recognition)

## Scenario Description

The real-time speech recognition service is based on the WebSocket protocol, allowing you to stream audio data and receive recognition results in real-time. Unlike traditional batch speech recognition, the WebSocket approach supports continuous recognition as you speak, making it ideal for real-time subtitles, voice assistants, meeting transcription, and other scenarios requiring immediate feedback.

## Learning Objectives

Through this example, you will learn:

- How to connect to AOG's real-time speech recognition service using WebSocket
- How to convert audio files to PCM format and stream them
- How to process real-time recognition results
- How to use Voice Activity Detection (VAD)
- The "command-event" communication pattern in WebSocket

## WebSocket Endpoint

```
ws://localhost:16688/aog/v0.2/services/speech-to-text-ws
```

## Communication Flow

1. Client connects to WebSocket server
2. Client sends `run-task` command to start task
3. Server returns `task-started` event (includes task_id)
4. Client sends PCM audio data (binary)
5. Server returns `result-generated` event (real-time recognition results)
6. Client sends `finish-task` command to end task
7. Server returns `task-finished` event

## Main Parameters

### run-task Command Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `task` | string | Yes | Fixed as `speech-to-text-ws` |
| `action` | string | Yes | Fixed as `run-task` |
| `model` | string | Yes | Model name, e.g., `NamoLi/whisper-large-v3-ov` |
| `parameters.format` | string | Optional | Audio format, **only supports `pcm`** |
| `parameters.sample_rate` | integer | Optional | Sample rate, only supports `16000` |
| `parameters.language` | string | Optional | Language code, e.g., `zh`, `en` |
| `parameters.use_vad` | boolean | Optional | Whether to use Voice Activity Detection, default `true` |
| `parameters.return_format` | string | Optional | Return format, default `text` |

### finish-task Command Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `task` | string | Yes | Fixed as `speech-to-text-ws` |
| `action` | string | Yes | Fixed as `finish-task` |
| `task_id` | string | Yes | Task ID returned by server |
| `model` | string | Yes | Model name used |

## Prerequisites

Before running this example, ensure:

- [ ] **AOG service is installed and running**
  ```bash
  # Check AOG service status
  curl http://localhost:16688/aog/v0.2/health
  ```

- [ ] **Speech recognition model is installed**
  - Recommended model: `NamoLi/whisper-large-v3-ov`
  - Use AOG's model management to download and install the model

- [ ] **Python dependencies are installed**
  ```bash
  pip install -r ../requirements.txt
  ```
  
  Main dependencies include:
  - `websocket-client`: WebSocket client
  - `pydub`: Audio processing
  - `soundfile`: Audio file I/O

- [ ] **Test audio is prepared**
  - Prepare one or more audio files (supports WAV, MP3, etc.)
  - Audio will be automatically converted to PCM format

## Quick Start

1. Start Jupyter Notebook:
   ```bash
   jupyter notebook
   ```

2. Open `speech-to-text-ws.ipynb`

3. Run cells sequentially

4. View real-time recognition results

## Usage Tips

### Audio Format Requirements

- **Server requirement**: Only accepts PCM format audio data
- **Sample rate**: 16000 Hz
- **Channels**: Mono
- **Bit depth**: 16-bit

### Audio Conversion

The notebook provides automatic conversion, supporting:
- WAV files automatically converted to PCM
- MP3 files automatically converted to PCM
- Other formats converted via `pydub`

### Voice Activity Detection (VAD)

Enabling VAD can:
- Automatically detect speech segments
- Filter silence
- Improve recognition accuracy
- Reduce unnecessary processing

### Real-time Optimization

1. **Chunked sending**: Split audio into small chunks (e.g., 1024 bytes) for streaming
2. **Buffer control**: Set appropriate sending intervals to avoid too fast or too slow
3. **Network optimization**: Ensure stable network connection

## Event Types

### task-started Event
Task started successfully, returns task_id for subsequent communication.

### result-generated Event
Real-time recognition result, includes:
- `begin_time`: Start time (milliseconds)
- `end_time`: End time (milliseconds, may be null)
- `text`: Recognized text content

### task-finished Event
Task completed normally.

### task-failed Event
Task failed, includes error code and message:
- `CLIENT_ERROR`: Client error
- `SERVER_ERROR`: Server error
- `MODEL_ERROR`: Model processing error

## FAQ

### Q: Why must PCM format be used?
A: AOG's real-time speech recognition service only accepts PCM format audio data. This ensures processing efficiency and consistency. Clients can input WAV or MP3 files, but must convert them to PCM format before sending.

### Q: How to handle long audio?
A: Split audio into small chunks (e.g., 1024 bytes) and stream them; the server will return recognition results in real-time.

### Q: Are recognition results final?
A: Real-time recognition returns both intermediate and final results. Intermediate results may have null `end_time`, while final results include complete timing information.

### Q: How to improve recognition accuracy?
A: 
- Use high-quality audio (clear, no noise)
- Enable VAD functionality
- Specify correct language code
- Ensure audio format meets requirements (PCM, 16000Hz, Mono, 16-bit)

### Q: What if WebSocket connection drops?
A: 
- Implement reconnection mechanism
- Save already recognized results
- Continue sending audio from breakpoint

### Q: What languages are supported?
A: Depends on the model used; Whisper models typically support multiple languages including Chinese and English.

### Q: How to debug WebSocket communication?
A: 
- Print sent and received messages
- Check event types and task_id
- Review error codes and messages
- Use WebSocket debugging tools

## Related Resources

- [AOG Documentation](../../../docs/zh-cn/)
- [Speech Service Specification](../../../docs/zh-cn/source/service_specs/speech.rst)
- [Speech-to-Text Service](../speech-to-text/) (batch recognition)

## Next Steps

- Try [Speech-to-Text](../speech-to-text/) service for batch speech recognition
- Explore [Text-to-Speech](../text-to-speech/) service to convert text to speech
- Check out other [AOG service examples](../)
