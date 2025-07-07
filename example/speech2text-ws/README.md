# AOG Speech-to-Text WebSocket Client Demo

This demo demonstrates how to use the AOG (AIPC Open Gateway) Speech-to-Text WebSocket service to perform real-time speech recognition.

## Overview

The AOG Speech-to-Text WebSocket service provides real-time speech recognition capabilities. This client demo shows how to:

1. Connect to the AOG WebSocket endpoint
2. Send audio data for recognition
3. Receive real-time recognition results
4. Handle different event types (task-started, result-generated, task-finished, task-failed)

## Prerequisites

- Go 1.19 or higher
- AOG server running locally on port 16688
- An audio file for testing in one of the supported formats

## Audio Format Requirements

**IMPORTANT**: The AOG Speech-to-Text WebSocket service **ONLY** supports PCM format input:

- **PCM** (.pcm) - Raw PCM audio data (16kHz sample rate, 16-bit, mono channel)

### Audio Format Processing

**Critical Note**: While the client can accept MP3 and WAV files as input, these formats **MUST** be converted to PCM format before being sent to the AOG server. The server itself only processes PCM audio data.

The client automatically handles format conversion:
- **MP3 files**: Automatically decoded to PCM using go-mp3 library before transmission
- **WAV files**: PCM data extracted after header validation before transmission
- **PCM files**: Used directly without conversion

**All audio data is converted to and sent as PCM chunks** - the server does not accept any other format.

### Recommended PCM Specifications

1. **Sample rate**: 16kHz (required for optimal recognition)
2. **Bit depth**: 16-bit
3. **Channels**: Mono (single channel)
4. **Encoding**: Linear PCM, little-endian

### Format Validation and Conversion

The client performs comprehensive audio format validation and automatic PCM conversion:

#### Supported Input Formats (Auto-converted to PCM)
- **WAV files**: Validates RIFF/WAVE headers and extracts PCM data
- **MP3 files**: Detects ID3 tags and MP3 frame sync patterns, then decodes to PCM
- **PCM files**: Identified by extension and used directly

#### Validation and Conversion Process
1. **File existence check**: Ensures the audio file exists
2. **Header analysis**: Reads file headers to detect actual format
3. **Format verification**: Validates against supported input formats
4. **PCM conversion**: Converts MP3/WAV to PCM format automatically
5. **Specification extraction**: For WAV files, extracts sample rate, channels, bit depth
6. **Optimization warnings**: Alerts about non-optimal PCM settings

#### Example Validation and Conversion Output

**WAV file processing:**
```
Audio format validated: wav (detected from file content)
WAV file specifications: 1 channels, 16000 Hz, 16 bits per sample
Extracted PCM from WAV: 1234567 bytes
Loaded and converted audio file: 1234567 bytes PCM data (original format: wav)
```

**MP3 file processing:**
```
Audio format validated: mp3 (detected from file content)
MP3 file has ID3v2.3.0 tag
Converted MP3 to PCM: 1234567 bytes
Loaded and converted audio file: 1234567 bytes PCM data (original format: mp3)
```

**Format error examples:**
```
unsupported or unrecognized audio format. File extension: .m4a. Supported input formats for PCM conversion: PCM (.pcm), WAV (RIFF/WAVE), MP3 (with ID3 or frame sync)
```

## Dependencies

This demo uses the following Go packages:
- `github.com/gorilla/websocket` - WebSocket client implementation

## Configuration

The demo uses the following default configuration:

```go
const (
    // WebSocket server URL for AOG Speech-to-Text service
    wsURL = "ws://localhost:16688/aog/v0.4/services/speech-to-text-ws"
    // Audio file path - replace with your audio file
    audioFile = "output.mp3"
    // Model to use for speech recognition
    defaultModel = "NamoLi/whisper-large-v3-ov"
)
```

## Usage

1. **Prepare your audio file**:
   - Place your audio file in one of the supported input formats (PCM, WAV, or MP3) in the same directory as the client
   - Name it `output.mp3` (or modify the `audioFile` constant in `client.go` to point to your file)
   - **Note**: All input formats will be automatically converted to PCM before being sent to the AOG server
   - Ensure the audio contains speech in the configured language (default is Chinese)

2. **Start AOG server**: Make sure the AOG server is running:
   ```bash
   aog server start -v
   ```

3. **Run the client**:
   ```bash
   go run client.go
   ```

The client will automatically:
- Validate your audio file format
- Convert the audio to PCM format (if not already PCM)
- Connect to the AOG WebSocket service
- Send the PCM audio data for recognition
- Display real-time recognition results

## How it works

The client follows this workflow:

1. **Audio Validation and Conversion**:
   - Checks file existence
   - Analyzes file headers to detect actual format (not just extension)
   - Validates format compatibility for PCM conversion
   - Converts MP3/WAV files to PCM format automatically
   - Extracts audio specifications (for WAV files)
   - Provides optimization recommendations for PCM output

2. **Connection**: Establishes a WebSocket connection to the AOG service

3. **Task Initialization**: Sends a `run-task` command with PCM format parameters

4. **Task Start**: Waits for the `task-started` event and receives a task ID

5. **PCM Audio Streaming**: Streams converted PCM audio data in chunks to simulate real-time input

6. **Result Processing**: Receives and displays `result-generated` events with recognition results

7. **Task Completion**: Sends a `finish-task` command and waits for `task-finished` event

## Message Types

### Client to Server Messages

- **run-task**: Initializes a new speech recognition task
- **finish-task**: Signals the end of audio input
- **Binary data**: PCM audio chunks sent as WebSocket binary messages

### Server to Client Events

- **task-started**: Confirms task initialization and provides task ID
- **result-generated**: Contains recognition results with text and timing information
- **task-finished**: Indicates successful task completion
- **task-failed**: Reports task failure with error details

## Customization

### Audio File Configuration

Change the audio file by modifying the constant in `client.go`:

```go
const (
    audioFile = "your-audio-file.wav"  // Change to your audio file
)
```

### Recognition Parameters

You can customize the recognition parameters by modifying the `Parameters` struct in the `sendRunTaskCommand` method:

```go
Parameters: &Parameters{
    Format:     "pcm",        // Always PCM format (converted from input format)
    SampleRate: 16000,        // Sample rate in Hz (16000 recommended)
    Language:   "zh",     // Language code: "<|zh|>" for Chinese, "<|en|>" for English
    UseVAD:     true,         // Voice Activity Detection (recommended: true)
}
```

### Supported Language Codes

- `"zh"` - Chinese (default)
- `"en"` - English
- Other language codes may be supported depending on the model

## Error Handling

The demo includes comprehensive error handling for:
- WebSocket connection failures
- Message parsing errors
- Task execution failures
- Audio file reading errors

## Example Output

```
AOG Speech-to-Text WebSocket Client Demo
========================================
Connected to AOG Speech-to-Text service at ws://localhost:16688/aog/v0.4/services/speech-to-text-ws
Validating input audio file: output.mp3
Input audio format validated: mp3 (will be converted to PCM for server)
Converted MP3 to PCM: 1234567 bytes
Loaded and converted audio file: 1234567 bytes PCM data (original format: mp3)
Sending run-task command with PCM format: {"task":"speech-to-text-ws","action":"run-task","parameters":{"format":"pcm",...}}
Task started successfully, Task ID: abc123
Waiting for task ID from server...
Received task ID: abc123
Loading audio file: output.mp3
Streaming PCM audio data to server...
Recognition result: 你好，这是一个测试。
  Time: 0ms - 2000ms
Recognition result: 今天天气怎么样？
  Time: 2000ms - 4000ms
Audio streaming completed
Sending finish-task command: {"task":"speech-to-text-ws","action":"finish-task","task_id":"abc123"}
Task completed successfully, Task ID: abc123
WebSocket connection closed
Speech recognition completed successfully!
```

## Troubleshooting

### Common Issues

1. **Connection refused**:
   - Make sure AOG server is running: `aog server start -v`
   - Verify the server is listening on port 16688
   - Check if firewall is blocking the connection

2. **Audio format errors**:
   ```
   audio validation failed: unsupported or unrecognized audio format
   ```
   - The client detects format by analyzing file content, not just extension
   - Only PCM, WAV, and MP3 input formats are supported for PCM conversion
   - Convert your audio file to one of the supported input formats
   - Ensure the file is not corrupted or truncated

   **Specific format issues:**
   - **WAV files**: Must have valid RIFF/WAVE headers and PCM audio format for extraction
   - **MP3 files**: Must have valid ID3 tags or MP3 frame sync patterns for decoding
   - **PCM files**: Detected by .pcm extension, ensure proper 16kHz/16-bit/mono specifications

3. **Audio file not found**:
   ```
   audio file not found: output.mp3
   ```
   - Ensure the audio file exists in the correct location
   - Check the file path in the `audioFile` constant
   - Verify file permissions

4. **Recognition errors**:
   - Verify the model is available on the AOG server
   - Check if the audio contains clear speech
   - Ensure the language setting matches your audio content
   - Try with a different audio file to isolate the issue

5. **Task timeout**:
   - Increase the `taskStartTimeout` constant if needed
   - Check AOG server logs for errors
   - Verify the model is properly loaded

6. **Format detection and conversion issues**:
   ```
   WAV format validation failed: unsupported WAV audio format: 3
   ```
   - WAV files must use PCM format (format code 1) for PCM extraction
   - Compressed WAV formats (like MP3-in-WAV) cannot be converted to PCM
   - Convert to uncompressed PCM WAV format before using as input

   ```
   MP3 format validation failed: no valid MP3 frame sync found
   ```
   - File may be corrupted or not a valid MP3 for PCM conversion
   - Try re-encoding the MP3 file
   - Ensure the file has proper MP3 headers for decoding

7. **Audio specification warnings**:
   - **Sample rate**: 16kHz is optimal, other rates may work but not guaranteed
   - **Channels**: Mono is recommended, stereo may cause issues
   - **Bit depth**: 16-bit is recommended for WAV/PCM files

8. **Empty recognition results**:
   - Check if the audio file contains actual speech
   - Verify the audio quality and volume level
   - Try enabling/disabling VAD (Voice Activity Detection)
   - Check if the language setting is correct

## API Reference

For detailed API documentation, refer to the [AOG API Specification](https://intel.github.io/aog/index.html).
