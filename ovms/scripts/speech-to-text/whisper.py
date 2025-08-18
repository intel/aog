import io
import librosa
import numpy as np
from pyovms import Tensor
from pathlib import Path
import openvino_genai
from datetime import timedelta
import json
import sys
import os
from typing import List, Dict, Any, Optional, Union
import webrtcvad

# Configuration
OV_CONFIG = {'PERFORMANCE_HINT': 'LATENCY', 'NUM_STREAMS': '1'}
TARGET_SAMPLE_RATE = 16000  # Target sample rate 16kHz
FRAME_DURATION_MS = 30  # Frame duration in milliseconds
VAD_FRAME_SIZE = int(TARGET_SAMPLE_RATE * FRAME_DURATION_MS / 1000)  # 30ms = 480 samples
HALLUCINATE_THRESHOLD = 100  # Volume threshold
TARGET_BUFFER_LENGTH = 3.0  # Target buffer length in seconds (3 seconds)


def format_timedelta(td):
    """Convert timedelta to SRT time format HH:MM:SS,mmm"""
    hours = td.seconds // 3600
    minutes = (td.seconds % 3600) // 60
    seconds = td.seconds % 60
    milliseconds = td.microseconds // 1000
    return f"{hours:02d}:{minutes:02d}:{seconds:02d},{milliseconds:03d}"


def create_srt_content(chunks, time_offset=0.0, deduplicate=False):
    """
    Convert timestamps and text to SRT format

    Args:
        chunks: Chunks containing timestamps and text, can be WhisperDecodedResultChunk objects or dictionaries
        time_offset: Time offset in seconds for adjusting timestamps
        deduplicate: Whether to remove duplicate chunks based on timestamp overlap (不再使用)
    """
    srt_content = []
    for i, chunk in enumerate(chunks, 1):
        # Apply time offset, compatible with both dictionary and object formats
        if isinstance(chunk, dict):
            start_time = timedelta(seconds=float(chunk["start_ts"]) + time_offset)
            end_time = timedelta(seconds=float(chunk["end_ts"]) + time_offset)
            text = chunk["text"].strip()
        else:
            start_time = timedelta(seconds=float(chunk.start_ts) + time_offset)
            end_time = timedelta(seconds=float(chunk.end_ts) + time_offset)
            text = chunk.text.strip()

        srt_entry = (
            f"{i}\n"
            f"{format_timedelta(start_time)} --> {format_timedelta(end_time)}\n"
            f"{text}\n"
        )
        srt_content.append(srt_entry)

    return "\n".join(srt_content)


# ==================== PCM Data Processing Functions ====================

def process_pcm_data(pcm_data, sample_rate=TARGET_SAMPLE_RATE, is_int16=True):
    """
    Process PCM data to ensure it's mono 16kHz float32 format

    Args:
        pcm_data: PCM audio data, can be int16 or float32 format
        sample_rate: Input audio sample rate
        is_int16: Whether it's int16 format (if so, needs conversion to float32)

    Returns:
        Processed float32 format PCM data
    """
    # If it's bytes, convert to numpy array
    if isinstance(pcm_data, bytes):
        if is_int16:
            pcm_data = np.frombuffer(pcm_data, dtype=np.int16)
        else:
            pcm_data = np.frombuffer(pcm_data, dtype=np.float32)

    # If it's int16 format, convert to float32
    if is_int16:
        pcm_data = pcm_data.astype(np.float32) / 32768.0

    # If multiple channels, convert to mono
    if len(pcm_data.shape) > 1 and pcm_data.shape[1] > 1:
        pcm_data = np.mean(pcm_data, axis=1)

    # If sample rate is not 16kHz, resample
    if sample_rate != TARGET_SAMPLE_RATE:
        pcm_data = librosa.resample(
            pcm_data,
            orig_sr=sample_rate,
            target_sr=TARGET_SAMPLE_RATE
        )

    return pcm_data


def process_with_vad(pcm_data, sample_rate=TARGET_SAMPLE_RATE, is_int16=True,
                     use_vad=True, hallucinate_threshold=HALLUCINATE_THRESHOLD, vad_mode=3):
    """
    Process PCM data and optionally apply enhanced VAD

    Args:
        pcm_data: PCM audio data
        sample_rate: Sample rate
        is_int16: Whether it's int16 format
        use_vad: Whether to apply VAD
        hallucinate_threshold: Volume threshold
        vad_mode: VAD mode (0-3)

    Returns:
        List of processed float32 format PCM data, each element is a (audio_data, start_time, end_time) tuple
    """
    # If not using VAD, directly process PCM data
    if not use_vad:
        processed_audio = process_pcm_data(pcm_data, sample_rate, is_int16)
        return [(processed_audio, 0.0, calculate_pcm_duration(processed_audio, TARGET_SAMPLE_RATE, False))]

    # Ensure data is int16 format, VAD processing requires it
    if not is_int16 and isinstance(pcm_data, np.ndarray):
        pcm_int16 = (pcm_data * 32768.0).astype(np.int16)
    elif isinstance(pcm_data, bytes) and not is_int16:
        pcm_np = np.frombuffer(pcm_data, dtype=np.float32)
        pcm_int16 = (pcm_np * 32768.0).astype(np.int16)
    elif isinstance(pcm_data, bytes) and is_int16:
        pcm_int16 = np.frombuffer(pcm_data, dtype=np.int16)
    else:
        pcm_int16 = pcm_data

    # Use enhanced VAD processor
    vad_processor = EnhancedVadProcessor(
        sample_rate=sample_rate,
        vad_frame_size=VAD_FRAME_SIZE,
        vad_mode=vad_mode,
        hallucinate_threshold=hallucinate_threshold
    )

    # Audio segment processing
    def process_segment(segment, start_time, end_time):
        # Check if resampling is needed
        if sample_rate != TARGET_SAMPLE_RATE:
            segment = librosa.resample(
                segment,
                orig_sr=sample_rate,
                target_sr=TARGET_SAMPLE_RATE
            )
        return segment, start_time, end_time

    # If no speech detected or volume too low, return original data
    if not vad_processor._is_audio_loud_enough(pcm_int16):
        processed_audio = process_pcm_data(pcm_data, sample_rate, is_int16)
        return [(processed_audio, 0.0, calculate_pcm_duration(processed_audio, TARGET_SAMPLE_RATE, False))]

    # Process and return segmented results
    return vad_processor.process_audio(pcm_int16, process_segment)


def calculate_pcm_duration(pcm_data, sample_rate, is_int16=True):
    """
    Calculate PCM data duration in seconds accurately

    Args:
        pcm_data: PCM audio data
        sample_rate: Sample rate
        is_int16: Whether it's int16 format

    Returns:
        Audio duration in seconds
    """
    # If it's bytes, convert to numpy array
    if isinstance(pcm_data, bytes):
        if is_int16:
            # For int16 format, each sample is 2 bytes
            num_samples = len(pcm_data) // 2
        else:
            # For float32 format, each sample is 4 bytes
            num_samples = len(pcm_data) // 4
    else:
        # Already numpy array
        num_samples = len(pcm_data)
    
    # Calculate duration
    return num_samples / sample_rate


def check_buffer_status(buffer_data, sample_rate, is_int16, target_length):
    """
    Check buffer status and return current length and if it reached target length
    
    Args:
        buffer_data: Audio buffer data
        sample_rate: Sample rate
        is_int16: Whether it's int16 format
        target_length: Target buffer length in seconds
        
    Returns:
        Tuple of (current_duration, is_ready)
    """
    # Calculate buffer duration in seconds
    buffer_duration = calculate_pcm_duration(buffer_data, sample_rate, is_int16)
    
    print(f"DEBUG: Buffer data length: {len(buffer_data)} bytes, calculated duration: {buffer_duration:.4f}s", flush=True)
    if is_int16:
        print(f"DEBUG: Sample calculation: {len(buffer_data)} bytes ÷ 2 bytes per sample ÷ {sample_rate} Hz = {buffer_duration:.4f}s", flush=True)
    else:
        print(f"DEBUG: Sample calculation: {len(buffer_data)} bytes ÷ 4 bytes per sample ÷ {sample_rate} Hz = {buffer_duration:.4f}s", flush=True)
    
    # Check if buffer reached target length
    is_ready = buffer_duration >= target_length
    
    return buffer_duration, is_ready


# ==================== OVMS Model Class ====================

class OvmsPythonModel:
    # Class constants
    MIN_AUDIO_LENGTH = 1.0  # Minimum audio processing length in seconds

    def initialize(self, kwargs: dict):
        print("-------------- Running initialize", flush=True)
        print(kwargs)
        path = Path(kwargs.get("base_path"))
        model_path = path.parent.parent / "models" / kwargs.get("node_name")
        self.pipe = openvino_genai.WhisperPipeline(model_path, device="AUTO")

        # Add audio cache dictionary for storing audio data from different sessions
        self.audio_cache = {}
        # Add connection state tracking
        self.client_connections = {}

        # Set default configuration
        self.config = self.pipe.get_generation_config()
        self.config.language = "<|zh|>"
        self.config.task = "transcribe"
        self.config.return_timestamps = True
        
        # Set minimum audio length (in seconds) for processing
        self.MIN_AUDIO_LENGTH = 1.0  # Keep this for backward compatibility
        
        # Print buffer configuration
        print(f"-------------- Initialized with buffer settings:", flush=True)
        print(f"Target buffer length: {TARGET_BUFFER_LENGTH} seconds", flush=True)
        print(f"VAD mode: 3 (default)", flush=True)
        
        # Print audio format information
        print(f"-------------- Audio format information:", flush=True)
        print(f"Target sample rate: {TARGET_SAMPLE_RATE} Hz", flush=True)
        print(f"Int16 format: 2 bytes per sample", flush=True)
        print(f"Float32 format: 4 bytes per sample", flush=True)
        print(f"Expected bytes per second (int16): {TARGET_SAMPLE_RATE * 2}", flush=True)
        print(f"Expected bytes per second (float32): {TARGET_SAMPLE_RATE * 4}", flush=True)
        print(f"5 seconds of audio (int16): {TARGET_SAMPLE_RATE * 2 * 5} bytes", flush=True)
        print(f"5 seconds of audio (float32): {TARGET_SAMPLE_RATE * 4 * 5} bytes", flush=True)

        print("-------------- Model loaded", flush=True)

    def finalize(self):
        """Called when service shuts down, ensure all remaining data is processed"""
        print("-------------- Running finalize", flush=True)
        # Process all unprocessed session caches
        for session_id, cache in self.audio_cache.items():
            if cache["audio_buffer"] and len(cache["audio_buffer"]) > 0:
                print(f"Processing remaining data for session {session_id}", flush=True)
                # Can process remaining data here, but since finalize can't return results, just log

    def execute(self, inputs: list):
        """
        Unified execute method that handles both file and streaming speech recognition
        Based on the 'service' parameter in params
        """
        try:
            # Extract input parameters
            audio_data = None
            params = {
                "service": "speech-to-text",  # Default to speech-to-text service
                "language": "zh",  # 默认语言改为不带格式符号的简单代码
                "sample_rate": TARGET_SAMPLE_RATE,
                "is_int16": True,
                "return_format": "text",  # speech-to-text: "text", speech-to-text-ws: "srt"
                "use_vad": False,  # speech-to-text: False, speech-to-text-ws: True
                "hallucinate_threshold": HALLUCINATE_THRESHOLD,
                "vad_mode": 3,
                "time_offset": 0.0,
                "task_id": "",  # Only used for speech-to-text-ws service
                "clear_cache": False,  # Only used for speech-to-text-ws service
                "connection_close": False,  # Only used for speech-to-text-ws service
                "target_buffer_length": TARGET_BUFFER_LENGTH  # New parameter for buffer control
            }

            # Parse input tensors
            for input_tensor in inputs:
                if input_tensor.name == "audio":
                    audio_data = bytes(input_tensor)
                elif input_tensor.name == "language":
                    # 修改语言参数处理，只保存简单代码，不添加格式符号
                    params["language"] = bytes(input_tensor).decode('utf-8')
                elif input_tensor.name == "params":
                    # For speech-to-text-ws service and unified service
                    try:
                        params_str = bytes(input_tensor).decode('utf-8')
                        user_params = json.loads(params_str)
                        # Update default parameters
                        for key, value in user_params.items():
                            if key in params:
                                # Type conversion
                                if key == "sample_rate":
                                    params[key] = int(value)
                                elif key in ["is_int16", "use_vad", "clear_cache", "connection_close"]:
                                    params[key] = bool(value)
                                elif key in ["hallucinate_threshold", "time_offset", "target_buffer_length"]:
                                    params[key] = float(value)
                                elif key == "vad_mode":
                                    params[key] = int(value)
                                else:
                                    params[key] = value
                            # 特殊处理嵌套在params中的language参数
                            elif key == "params" and isinstance(value, dict) and "language" in value:
                                params["language"] = value["language"]
                    except Exception as e:
                        print(f"Error parsing params: {e}", flush=True)

            # Print received parameters for debugging
            print(f"-------------- Received parameters:", flush=True)
            print(f"Service: {params['service']}", flush=True)
            print(f"Language: {params['language']}", flush=True)
            print(f"Sample rate: {params['sample_rate']}", flush=True)
            print(f"Is int16: {params['is_int16']}", flush=True)
            print(f"Target buffer length: {params['target_buffer_length']}", flush=True)
            print(f"Audio data size: {len(audio_data) if audio_data else 'None'} bytes", flush=True)
            
            # Calculate expected audio duration if audio data is provided
            if audio_data:
                expected_bytes_per_sample = 2 if params["is_int16"] else 4
                expected_bytes_per_second = params["sample_rate"] * expected_bytes_per_sample
                expected_duration = len(audio_data) / expected_bytes_per_second
                print(f"Expected audio duration: {expected_duration:.4f} seconds", flush=True)

            # Determine service type and set defaults
            service_type = params.get("service", "speech-to-text")

            if service_type == "speech-to-text":
                # speech-to-text service mode - set appropriate defaults
                params["return_format"] = "text"
                params["use_vad"] = False
                return self._execute_file_service(audio_data, params)
            elif service_type == "speech-to-text-ws":
                # speech-to-text-ws service mode - set appropriate defaults
                params["return_format"] = "srt"
                params["use_vad"] = True
                return self._execute_streaming_service(audio_data, params)
            else:
                raise ValueError(
                    f"Unknown service type: {service_type}. Must be 'speech-to-text' or 'speech-to-text-ws'")

        except Exception as e:
            print(f"Error during execution: {str(e)}", flush=True)
            import traceback
            traceback.print_exc()
            error_message = {
                "status": "error",
                "message": str(e),
                "is_final": False
            }
            return [Tensor("result", json.dumps(error_message, ensure_ascii=False).encode('utf-8'))]

    def _execute_file_service(self, audio_data, params):
        """Execute speech-to-text service (original speech-to-text logic)"""
        if audio_data is None:
            raise ValueError("No audio data provided")

        # Set language with proper formatting
        language_code = params["language"]
        if language_code.startswith("<|") and language_code.endswith("|>"):
            self.config.language = language_code
        else:
            self.config.language = f"<|{language_code}|>"

        # Load audio (fixed sample rate at 16kHz)
        try:
            audio_array, _ = librosa.load(io.BytesIO(audio_data), sr=16000)
            print(f"Audio duration: {len(audio_array) / 16000:.2f}s", flush=True)
        except Exception as e:
            print(f"Audio loading error: {str(e)}", flush=True)
            raise

        # Generate transcription result
        try:
            result = self.pipe.generate(audio_array.tolist(), self.config)
            print(f"Generated {len(result.chunks)} chunks", flush=True)
        except Exception as e:
            print(f"Transcription error: {str(e)}", flush=True)
            raise

        # Timestamp continuity correction (core logic)
        output_text = ""
        time_offset = 0.0
        last_chunk_end = 0.0

        for chunk in result.chunks:
            # Detect segment jumps (current start time < previous end time)
            if chunk.start_ts < last_chunk_end - 0.5:  # 0.5 second tolerance
                time_offset += last_chunk_end
                print(f"Time offset adjusted to {time_offset:.2f}s", flush=True)

            # Calculate absolute timestamps
            start = chunk.start_ts + time_offset
            end = chunk.end_ts + time_offset
            output_text += f"[{start:.2f}, {end:.2f}] {chunk.text}\n"
            last_chunk_end = chunk.end_ts

        print("Generated subtitles:\n" + output_text, flush=True)

        # Return result based on format
        return [Tensor("result", output_text.encode('utf-8'))]

    def _execute_streaming_service(self, audio_data, params):
        """Execute speech-to-text-ws service with improved buffering"""
        # Initialize configuration
        config = self.pipe.get_generation_config()
        config.task = "transcribe"
        config.return_timestamps = True
        
        # 修复语言参数处理，确保正确格式化
        language_code = params["language"]
        if language_code.startswith("<|") and language_code.endswith("|>"):
            config.language = language_code
        else:
            config.language = f"<|{language_code}|>"

        # Check session ID
        session_id = params["task_id"]
        if not session_id:
            raise ValueError("Must provide task_id parameter to identify session")

        # Handle cache clear request
        if params["clear_cache"] and session_id in self.audio_cache:
            del self.audio_cache[session_id]
            return [Tensor("result", json.dumps({
                "status": "cache_cleared",
                "message": "Cache cleared",
                "is_final": False
            }, ensure_ascii=False).encode('utf-8'))]

        # 检查action参数，处理finish-task动作
        action = params.get("action", "")
        if action == "finish-task" and session_id in self.audio_cache:
            print(f"Received finish-task action for session {session_id}, finalizing processing", flush=True)
            final_result = self._finalize_processing(session_id, is_final_task=True)
            if final_result:
                return [Tensor("result", final_result)]
            else:
                return [Tensor("result", json.dumps({
                    "status": "completed",
                    "message": "Session processing completed, no valid transcription results",
                    "is_final": True,
                    "content": ""
                }, ensure_ascii=False).encode('utf-8'))]

        # Handle empty audio data as session end signal
        if (audio_data is None or len(audio_data) == 0) and session_id in self.audio_cache:
            print(f"Received empty audio data, processing remaining data for session {session_id}", flush=True)
            final_result = self._finalize_processing(session_id, is_final_task=False)
            if final_result:
                return [Tensor("result", final_result)]
            else:
                return [Tensor("result", json.dumps({
                    "status": "completed",
                    "message": "Session processing completed",
                    "is_final": True,
                    "content": ""
                }, ensure_ascii=False).encode('utf-8'))]

        # Initialize session cache on first call
        if audio_data is None:
            return [Tensor("result", json.dumps({
                "status": "error",
                "message": "First call must provide audio data",
                "is_final": False
            }, ensure_ascii=False).encode('utf-8'))]

        try:
            # Verify audio data format
            if len(audio_data) > 0:
                # Check if the audio data size makes sense
                expected_bytes_per_second = params["sample_rate"] * (2 if params["is_int16"] else 4)
                audio_seconds_estimate = len(audio_data) / expected_bytes_per_second
                print(f"Received audio chunk of {len(audio_data)} bytes, estimated duration: {audio_seconds_estimate:.4f}s", flush=True)
                print(f"Expected bytes per second: {expected_bytes_per_second} (sample_rate={params['sample_rate']}, is_int16={params['is_int16']})", flush=True)
            
            # Add new audio data to cache
            if session_id not in self.audio_cache:
                self.audio_cache[session_id] = {
                    "audio_buffer": b"",
                    "sample_rate": params["sample_rate"],
                    "is_int16": params["is_int16"],
                    "processed_length": 0.0,  # Record processed audio length in seconds
                    "accumulated_chunks": [],  # Accumulate all text chunks
                    "overlap_buffer": b"",  # 用于存储未完成的语音段
                    "last_segment_start_time": 0.0  # 存储未完成语音段的开始时间
                }

            # Append new audio data to cache
            if audio_data:
                self.audio_cache[session_id]["audio_buffer"] += audio_data
                print(f"Added {len(audio_data)} bytes to buffer for session {session_id}", flush=True)

            # Get cached information
            session_cache = self.audio_cache[session_id]
            audio_buffer = session_cache["audio_buffer"]
            sample_rate = session_cache["sample_rate"]
            is_int16 = session_cache["is_int16"]
            processed_length = session_cache["processed_length"]
            overlap_buffer = session_cache.get("overlap_buffer", b"")  # 获取上次处理的尾部数据

            # Use the buffer status check function
            buffer_duration, buffer_ready = check_buffer_status(
                audio_buffer, 
                sample_rate, 
                is_int16, 
                params["target_buffer_length"]
            )
            
            # Log buffer status
            print(f"Buffer status: {buffer_duration:.2f}s / {params['target_buffer_length']:.2f}s (ready: {buffer_ready})", flush=True)
            print(f"Total buffer size: {len(audio_buffer)} bytes", flush=True)

            # If buffer too small and not finish-task action, continue waiting
            if not buffer_ready and action != "finish-task":
                return [Tensor("result", json.dumps({
                    "status": "buffering",
                    "buffered_seconds": buffer_duration,
                    "message": f"Audio buffer accumulating ({buffer_duration:.2f}s / {params['target_buffer_length']}s)",
                    "is_final": False
                }, ensure_ascii=False).encode('utf-8'))]

            # Log when buffer is ready for processing
            print(f"Buffer ready for processing: {buffer_duration:.2f}s of audio data", flush=True)

            # Prepare audio data for processing
            process_data = audio_buffer
            
            # 如果有上次的未完成语音段，添加到当前处理数据前面
            if overlap_buffer and len(overlap_buffer) > 0:
                overlap_bytes = len(overlap_buffer)
                overlap_duration = calculate_pcm_duration(overlap_buffer, sample_rate, is_int16)
                print(f"Prepending {overlap_bytes} bytes ({overlap_duration:.2f}s) of previous incomplete speech segment", flush=True)
                # 将上一次的未完成语音段添加到当前处理数据前面
                process_data = overlap_buffer + process_data
            
            # 清空主缓冲区
            self.audio_cache[session_id]["audio_buffer"] = b""
            
            # Apply enhanced VAD processing to get multiple speech segments
            print("-------------- Starting VAD processing", flush=True)
            audio_segments = process_with_vad(
                process_data,
                sample_rate,
                is_int16,
                params["use_vad"],
                params["hallucinate_threshold"],
                params["vad_mode"]
            )
            print(f"-------------- VAD processing completed, got {len(audio_segments)} speech segments", flush=True)
            
            # 检查是否有未完成的语音段（最后一段语音可能未结束）
            is_last_segment_incomplete = False
            
            # 检查是否是finish-task动作，如果是则处理所有语音段
            if action == "finish-task":
                print("Finish-task action received, processing all speech segments without reservation", flush=True)
            elif audio_segments and len(audio_segments) > 0:
                # 获取最后一段语音
                last_segment, last_start, last_end = audio_segments[-1]
                last_segment_duration = last_end - last_start
                
                # 更保守的处理：总是将最后一段语音留到下一次处理
                is_last_segment_incomplete = True
                print(f"Preserving last speech segment for next processing, duration: {last_segment_duration:.2f}s", flush=True)
                
                # 将最后一段语音保存为下一次的输入
                # 计算最后一段语音在原始音频中的位置
                last_segment_start_sample = int(last_start * sample_rate) * (2 if is_int16 else 4)
                
                # 从原始音频中提取最后一段语音
                if isinstance(process_data, bytes):
                    last_segment_bytes = process_data[last_segment_start_sample:]
                    # 更新重叠缓冲区
                    self.audio_cache[session_id]["overlap_buffer"] = last_segment_bytes
                else:
                    sample_size = 2 if is_int16 else 4
                    last_segment_array = process_data[last_segment_start_sample // sample_size:]
                    last_segment_bytes = last_segment_array.tobytes() if is_int16 else last_segment_array
                    self.audio_cache[session_id]["overlap_buffer"] = last_segment_bytes
                
                # 记录最后一段语音的开始时间（相对于当前缓冲区）
                self.audio_cache[session_id]["last_segment_start_time"] = last_start
                
                # 从处理结果中移除最后一段未完成的语音
                audio_segments = audio_segments[:-1]
                print(f"Removed last segment from current processing, saved {len(self.audio_cache[session_id]['overlap_buffer'])} bytes for next processing", flush=True)
            
            # Process each speech segment and merge results
            current_chunks = []

            for seg_audio, seg_start, seg_end in audio_segments:
                # Calculate absolute time (add processed audio length and time offset)
                abs_start = processed_length + params["time_offset"] + seg_start
                abs_end = processed_length + params["time_offset"] + seg_end  # 修复：使用seg_end而不是seg_start
                
                # Print segment info
                seg_duration = calculate_pcm_duration(seg_audio, sample_rate, False)  # seg_audio is already float32
                print(f"Processing segment: {seg_duration:.2f}s, time: {abs_start:.2f}s - {abs_end:.2f}s", flush=True)

                # Call model to transcribe current segment
                result = self.pipe.generate(seg_audio, config)
                print(f"Generated {len(result.chunks)} text chunks for segment", flush=True)

                # Create timestamp-adjusted new objects
                adjusted_chunks = []
                for chunk in result.chunks:
                    # Create dictionary with adjusted timestamps
                    adjusted_chunk = {
                        "text": chunk.text,
                        "start_ts": chunk.start_ts + abs_start,
                        "end_ts": chunk.end_ts + abs_start
                    }
                    adjusted_chunks.append(adjusted_chunk)
                    print(f"Text chunk: '{chunk.text}', time: {adjusted_chunk['start_ts']:.2f}s - {adjusted_chunk['end_ts']:.2f}s", flush=True)

                # Collect transcription results for current audio segment
                current_chunks.extend(adjusted_chunks)

            # Update processed audio length
            # 计算有效处理长度（不包含最后一段未完成的语音）
            effective_duration = buffer_duration
            
            # 如果检测到未完成的语音段，减去其时长
            if is_last_segment_incomplete:
                # 获取最后一段语音的开始时间
                last_segment_start_time = self.audio_cache[session_id].get("last_segment_start_time", 0.0)
                if last_segment_start_time > 0:
                    # 减去未完成语音段的长度
                    effective_duration = last_segment_start_time
                    print(f"Adjusted effective duration: {effective_duration:.2f}s (original: {buffer_duration:.2f}s, incomplete segment starts at: {last_segment_start_time:.2f}s)", flush=True)
            
            # 更新处理长度
            self.audio_cache[session_id]["processed_length"] += effective_duration
            print(f"Updated processed_length to {self.audio_cache[session_id]['processed_length']:.2f}s", flush=True)
            
            # 将当前识别的块添加到累积结果中
            if "accumulated_chunks" not in self.audio_cache[session_id]:
                self.audio_cache[session_id]["accumulated_chunks"] = []
            
            # 保存当前块到累积结果
            if current_chunks:
                # 保存语言信息到缓存，以便在会话结束时使用
                self.audio_cache[session_id]["language"] = params["language"]
                
                self.audio_cache[session_id]["accumulated_chunks"].extend(current_chunks)
                print(f"Added {len(current_chunks)} chunks to accumulated results (total: {len(self.audio_cache[session_id]['accumulated_chunks'])})", flush=True)

            # If there are transcription results, return current part in SRT format
            if current_chunks:
                # 获取所有累积的结果（包括之前的和当前的）
                all_accumulated_chunks = self.audio_cache[session_id].get("accumulated_chunks", [])
                
                # Generate SRT format content for all accumulated results
                output_content = create_srt_content(all_accumulated_chunks, 0)
                
                # Add processing status marker
                result = {
                    "status": "processing",
                    "content": output_content,
                    "is_final": False,
                    "current_chunks": len(current_chunks),
                    "total_chunks": len(all_accumulated_chunks)
                }
                print(f"Returning result with {len(all_accumulated_chunks)} total chunks, content length: {len(output_content)}", flush=True)
                
                return [Tensor("result", json.dumps(result, ensure_ascii=False).encode('utf-8'))]
            else:
                # No speech detected, return status information
                print("No speech detected in the processed audio", flush=True)
                return [Tensor("result", json.dumps({
                    "status": "no_speech_detected",
                    "processed_seconds": buffer_duration,
                    "total_processed": self.audio_cache[session_id]["processed_length"],
                    "is_final": False
                }, ensure_ascii=False).encode('utf-8'))]
        except Exception as e:
            print(f"Error during streaming execution: {str(e)}", flush=True)
            import traceback
            traceback.print_exc()
            error_message = {
                "status": "error",
                "message": str(e),
                "is_final": False
            }
            return [Tensor("result", json.dumps(error_message, ensure_ascii=False).encode('utf-8'))]

    def _finalize_processing(self, session_id, is_final_task=False):
        """Handle session finalization and process any remaining audio data"""
        print(f"-------------- Finalizing processing for session {session_id} (is_final_task: {is_final_task})", flush=True)
        if session_id not in self.audio_cache:
            return None

        cache = self.audio_cache[session_id]
        audio_buffer = cache.get("audio_buffer", b"")
        sample_rate = cache["sample_rate"]
        is_int16 = cache["is_int16"]
        accumulated_chunks = cache.get("accumulated_chunks", [])
        
        # 检查是否有未处理的重叠缓冲区数据
        overlap_buffer = cache.get("overlap_buffer", b"")
        if overlap_buffer and len(overlap_buffer) > 0:
            print(f"Found {len(overlap_buffer)} bytes of unprocessed speech segment, adding to final processing", flush=True)
            if audio_buffer:
                audio_buffer = overlap_buffer + audio_buffer
            else:
                audio_buffer = overlap_buffer
        
        # 获取语言代码并正确格式化
        language_code = "<|zh|>"  # 默认值
        if "language" in cache:
            lang = cache["language"]
            if lang.startswith("<|") and lang.endswith("|>"):
                language_code = lang
            else:
                language_code = f"<|{lang}|>"

        # Check if there's remaining audio data to process
        has_remaining_audio = audio_buffer and len(audio_buffer) > 0

        if has_remaining_audio:
            # Use pipeline_finalizer to process remaining audio
            result = self._pipeline_finalizer(
                self.pipe,
                audio_buffer,
                sample_rate,
                is_int16,
                language_code,
                use_vad=True
            )

            # If processing successful, add new text chunks to accumulated chunks
            if result["status"] == "success" and "chunks" in result and result["chunks"] > 0:
                # 打印识别结果
                print(f"Final processing successful, got {result['chunks']} chunks", flush=True)
                print(f"Final recognition result: {result['result']}", flush=True)
                
                # Note: This assumes result contains processed text chunks, may need adjustment based on actual pipeline_finalizer return
                new_srt = result["result"]
                # Clear cache and return result
                # del self.audio_cache[session_id]  # 注释掉删除缓存的代码，保持连接
                print(f"Keeping session cache for {session_id} to maintain connection open", flush=True)
                final_result = {
                    "status": "completed",
                    "content": new_srt,
                    "is_final": True,
                    "message": "Processing completed"
                }
                return json.dumps(final_result, ensure_ascii=False).encode('utf-8')

        # If no remaining audio or processing result is empty, but has previously accumulated results
        if accumulated_chunks:
            final_srt = create_srt_content(accumulated_chunks, 0)
            print(f"Using accumulated results: {len(accumulated_chunks)} chunks", flush=True)
            print(f"Final recognition result: {final_srt}", flush=True)
            # del self.audio_cache[session_id]  # 注释掉删除缓存的代码，保持连接
            print(f"Keeping session cache for {session_id} to maintain connection open", flush=True)
            final_result = {
                "status": "completed",
                "content": final_srt,
                "is_final": True,
                "message": "Processing completed"
            }
            return json.dumps(final_result, ensure_ascii=False).encode('utf-8')

        # If neither remaining audio nor accumulated results, return session completion status
        # del self.audio_cache[session_id]  # 注释掉删除缓存的代码，保持连接
        print(f"Keeping session cache for {session_id} to maintain connection open", flush=True)
        return json.dumps({
            "status": "completed",
            "message": "Session processing completed, no valid transcription results",
            "is_final": True,
            "content": ""
        }, ensure_ascii=False).encode('utf-8')

    def _pipeline_finalizer(self, pipe, audio_buffer, sample_rate=TARGET_SAMPLE_RATE, is_int16=True, language="<|zh|>",
                            use_vad=True):
        """
        Process remaining audio data at the end of streaming processing

        Args:
            pipe: WhisperPipeline object
            audio_buffer: Remaining audio data
            sample_rate: Sample rate
            is_int16: Whether it's int16 format
            language: Language code
            use_vad: Whether to use VAD segmentation processing

        Returns:
            Processing result containing transcription text and timestamps
        """
        print("-------------- Starting to process remaining audio", flush=True)
        # Check data validity
        if not audio_buffer or (isinstance(audio_buffer, bytes) and len(audio_buffer) == 0):
            return {"status": "empty_buffer", "chunks": 0}

        try:
            # Set model configuration
            config = pipe.get_generation_config()
            config.task = "transcribe"
            config.return_timestamps = True
            
            # 确保语言代码格式正确
            if language.startswith("<|") and language.endswith("|>"):
                config.language = language
            else:
                config.language = f"<|{language}|>"

            # Use VAD to process audio
            print("-------------- Starting VAD processing", flush=True)
            audio_segments = process_with_vad(
                audio_buffer,
                sample_rate,
                is_int16,
                use_vad=use_vad,
                hallucinate_threshold=HALLUCINATE_THRESHOLD,
                vad_mode=3
            )
            print(f"-------------- VAD processing completed, got {len(audio_segments)} speech segments", flush=True)

            # Process each speech segment
            all_chunks = []
            for seg_audio, seg_start, seg_end in audio_segments:
                # Call model for transcription
                result = pipe.generate(seg_audio, config)
                print(f"Final segment generated {len(result.chunks)} text chunks", flush=True)
                
                # Create timestamp-adjusted new objects
                for chunk in result.chunks:
                    # Create dictionary with adjusted timestamps
                    adjusted_chunk = {
                        "text": chunk.text,
                        "start_ts": chunk.start_ts + seg_start,
                        "end_ts": chunk.end_ts + seg_end
                    }
                    all_chunks.append(adjusted_chunk)
                    print(f"Final text chunk: '{chunk.text}', time: {adjusted_chunk['start_ts']:.2f}s - {adjusted_chunk['end_ts']:.2f}s", flush=True)

            # Generate SRT format content
            if all_chunks:
                output = create_srt_content(all_chunks, 0)
                # Cannot directly return text chunk objects as they cannot be JSON serialized
                # But can return SRT format and chunk count
                return {
                    "status": "success",
                    "result": output,
                    "chunks": len(all_chunks),
                    "has_text": True
                }
            else:
                return {"status": "no_speech_detected", "chunks": 0, "has_text": False}
        except Exception as e:
            print(f"Processing error: {e}", flush=True)
            return {"status": "error", "message": str(e), "chunks": 0, "has_text": False}


# ==================== Enhanced VAD Processing Functions ====================

class EnhancedVadProcessor:
    """Enhanced VAD processor that integrates WebRTC VAD and audio segmentation functionality"""

    def __init__(
            self,
            sample_rate=TARGET_SAMPLE_RATE,
            vad_frame_size=VAD_FRAME_SIZE,
            vad_mode=3,
            min_speech_duration_ms=250,
            min_silence_duration_ms=500,
            speech_pad_ms=400,
            hallucinate_threshold=HALLUCINATE_THRESHOLD
    ):
        """
        Initialize enhanced VAD processor

        Args:
            sample_rate: Sample rate
            vad_frame_size: VAD frame size
            vad_mode: VAD sensitivity, 0-3, higher is more "aggressive"
            min_speech_duration_ms: Minimum speech duration (ms)
            min_silence_duration_ms: Minimum silence duration (ms) for segmentation
            speech_pad_ms: Silence padding duration before and after speech segments (ms)
            hallucinate_threshold: Volume threshold, below this is considered noise
        """
        self.sample_rate = sample_rate
        self.vad_frame_size = vad_frame_size
        self.vad_mode = vad_mode
        self.vad = webrtcvad.Vad(vad_mode)

        # Convert time parameters to frame counts
        self.min_speech_frames = int(min_speech_duration_ms / FRAME_DURATION_MS)
        self.min_silence_frames = int(min_silence_duration_ms / FRAME_DURATION_MS)
        self.speech_pad_frames = int(speech_pad_ms / FRAME_DURATION_MS)
        self.hallucinate_threshold = hallucinate_threshold

        # State variables
        self.reset_state()

    def reset_state(self):
        """Reset processing state"""
        self.is_speech = False
        self.speech_frames = []
        self.silence_frames = 0
        self.speech_frame_count = 0
        self.speech_start_frame = 0

    def _is_audio_loud_enough(self, audio_data):
        """
        Check if audio is loud enough

        Args:
            audio_data: Audio data, can be bytes or numpy array

        Returns:
            Boolean indicating whether audio is loud enough
        """
        if isinstance(audio_data, bytes):
            audio_np = np.frombuffer(audio_data, dtype=np.int16)
        else:
            audio_np = audio_data

        amplitude = np.mean(np.abs(audio_np))
        return amplitude > self.hallucinate_threshold

    def _transfer_audiodata_to_float(self, frame_data: np.ndarray) -> np.ndarray:
        """
        Convert audio data from int16 format to float32 format, range [-1.0, 1.0]

        Args:
            frame_data: int16 format audio data

        Returns:
            float32 format audio data
        """
        return frame_data.astype(np.float32) / 32768.0

    def process_frame(self, frame_data):
        """
        Process single frame of audio data, determine if it's speech

        Args:
            frame_data: One frame of audio data

        Returns:
            Boolean indicating whether current frame is determined to be speech
        """
        # Ensure frame data is in appropriate format
        if isinstance(frame_data, bytes):
            frame_data_np = np.frombuffer(frame_data, dtype=np.int16)
        else:
            frame_data_np = frame_data

        # Prepare data for VAD
        if frame_data_np.dtype != np.int16:
            frame_data_bytes = (frame_data_np * 32768.0).astype(np.int16).tobytes()
        else:
            frame_data_bytes = frame_data_np.tobytes()

        # Use WebRTC VAD to determine if it's speech
        try:
            return self.vad.is_speech(frame_data_bytes, self.sample_rate)
        except Exception:
            return False

    def segment_audio(self, audio_data, return_timestamps=False):
        """
        Segment long audio stream into multiple speech segments

        Args:
            audio_data: Audio data in PCM int16 format
            return_timestamps: Whether to return timestamps

        Returns:
            List of segmented audio, each segment in float32 format, range [-1.0, 1.0]
            If return_timestamps is True, also returns timestamp list for each segment
        """
        # Ensure audio data is numpy array format
        if isinstance(audio_data, bytes):
            audio_data = np.frombuffer(audio_data, dtype=np.int16)

        # Reset state
        self.reset_state()

        # Lists to save results
        segments = []
        timestamps = []

        # Split audio into frames for processing
        frame_count = len(audio_data) // self.vad_frame_size

        # Optimization: Use numpy vectorized operations to improve performance
        # 1. Pre-split audio data into frames, avoid repeated index calculations in loop
        frames = [audio_data[i * self.vad_frame_size:(i + 1) * self.vad_frame_size] for i in range(frame_count)]

        # 2. Pre-process VAD results for each frame
        is_speech_frames = []
        for i in range(frame_count):
            is_speech = self.process_frame(frames[i])
            is_speech_frames.append(is_speech)

        # 3. Segment based on VAD results
        self.speech_start_frame = 0
        speech_segments_indices = []  # Store start and end indices for each segment

        i = 0
        while i < frame_count:
            # Look for speech start
            if not self.is_speech and is_speech_frames[i]:
                self.is_speech = True
                self.speech_start_frame = max(0, i - self.speech_pad_frames)
                self.speech_frame_count = 1
                self.silence_frames = 0
            # Update speech state
            elif self.is_speech:
                if is_speech_frames[i]:
                    self.speech_frame_count += 1
                    self.silence_frames = 0
                else:
                    self.silence_frames += 1

                # Check if speech segment should end
                if self.silence_frames >= self.min_silence_frames:
                    # If speech segment is long enough, save this segment
                    if self.speech_frame_count >= self.min_speech_frames:
                        speech_end_frame = i - self.silence_frames + 1
                        speech_segments_indices.append((self.speech_start_frame, speech_end_frame))

                        # Calculate timestamps
                        start_time = self.speech_start_frame * FRAME_DURATION_MS / 1000
                        end_time = speech_end_frame * FRAME_DURATION_MS / 1000
                        timestamps.append((start_time, end_time))

                    # Reset state
                    self.is_speech = False
                    self.speech_frame_count = 0
                    self.silence_frames = 0

            i += 1

        # Handle last possible speech segment
        if self.is_speech and self.speech_frame_count >= self.min_speech_frames:
            speech_end_frame = frame_count
            speech_segments_indices.append((self.speech_start_frame, speech_end_frame))

            # Calculate timestamps
            start_time = self.speech_start_frame * FRAME_DURATION_MS / 1000
            end_time = speech_end_frame * FRAME_DURATION_MS / 1000
            timestamps.append((start_time, end_time))

        # 4. Generate audio segments based on segment indices
        for start_frame, end_frame in speech_segments_indices:
            start_sample = start_frame * self.vad_frame_size
            end_sample = min(end_frame * self.vad_frame_size, len(audio_data))
            segment_audio = audio_data[start_sample:end_sample]
            segments.append(self._transfer_audiodata_to_float(segment_audio))

        # 5. If no speech segments detected, return entire audio
        if not segments:
            segments.append(self._transfer_audiodata_to_float(audio_data))
            if return_timestamps:
                timestamps.append((0, len(audio_data) / self.sample_rate))

        if return_timestamps:
            return segments, timestamps
        else:
            return segments

    def process_audio(self, audio_data, process_callback=None):
        """
        Process audio and segment, optionally apply callback function to each segment

        Args:
            audio_data: Audio data
            process_callback: Callback function for processing segments, accepts (audio_segment, start_time, end_time) parameters

        Returns:
            List of processed audio segments
        """
        segments, timestamps = self.segment_audio(audio_data, return_timestamps=True)

        results = []
        for segment, (start_time, end_time) in zip(segments, timestamps):
            if process_callback:
                result = process_callback(segment, start_time, end_time)
                results.append(result)
            else:
                results.append((segment, start_time, end_time))

        return results
