import io
import json
import numpy as np
import openvino as ov
import openvino_genai
from pyovms import Tensor
from pathlib import Path

# Configuration
OV_CONFIG = {'PERFORMANCE_HINT': 'LATENCY', 'NUM_STREAMS': '1'}
DEFAULT_SAMPLE_RATE = 16000  # Default sample rate for TTS output

# Removed _load_default_embeddings function - we only use custom embeddings

# Note: We only use custom embeddings, no default embeddings needed

def _load_custom_embeddings():
    """
    Load custom speaker embeddings extracted from audio files

    Returns:
        dict: Dictionary of custom voice embeddings
    """
    custom_embeddings = {}
    base_path = Path(__file__).parent / "embeddings"

    # Define custom voices with their file names and descriptions
    custom_voices = {
        "male": "Custom male voice extracted from audio",
        "female": "Custom female voice extracted from audio",
        "girl": "Young girl voice extracted from audio",
        "baby": "Baby voice extracted from audio"
    }

    for voice_key, description in custom_voices.items():
        voice_file = base_path / f"{voice_key}.npy"
        if voice_file.exists():
            try:
                embedding = np.load(voice_file)
                custom_embeddings[voice_key] = embedding
                print(f"Loaded custom voice: {voice_key} ({description})", flush=True)
            except Exception as e:
                print(f"Failed to load custom voice {voice_key}: {e}", flush=True)
        else:
            print(f"Custom voice file not found: {voice_file}", flush=True)

    return custom_embeddings

# Load custom embeddings
CUSTOM_EMBEDDINGS = _load_custom_embeddings()

# Voice type mapping - only support the four custom voices
VOICE_TYPES = {}

# Add custom voices if they were loaded successfully
if "male" in CUSTOM_EMBEDDINGS:
    VOICE_TYPES.update({
        "male": CUSTOM_EMBEDDINGS["male"],
        "man": CUSTOM_EMBEDDINGS["male"],
        "男": CUSTOM_EMBEDDINGS["male"],
        "男声": CUSTOM_EMBEDDINGS["male"]
    })

if "female" in CUSTOM_EMBEDDINGS:
    VOICE_TYPES.update({
        "female": CUSTOM_EMBEDDINGS["female"],
        "woman": CUSTOM_EMBEDDINGS["female"],
        "女": CUSTOM_EMBEDDINGS["female"],
        "女声": CUSTOM_EMBEDDINGS["female"]
    })

if "girl" in CUSTOM_EMBEDDINGS:
    VOICE_TYPES.update({
        "girl": CUSTOM_EMBEDDINGS["girl"],
        "child": CUSTOM_EMBEDDINGS["girl"],
        "kid": CUSTOM_EMBEDDINGS["girl"],
        "女孩": CUSTOM_EMBEDDINGS["girl"],
        "小女孩": CUSTOM_EMBEDDINGS["girl"]
    })

if "baby" in CUSTOM_EMBEDDINGS:
    VOICE_TYPES.update({
        "baby": CUSTOM_EMBEDDINGS["baby"],
        "infant": CUSTOM_EMBEDDINGS["baby"],
        "婴儿": CUSTOM_EMBEDDINGS["baby"],
        "宝宝": CUSTOM_EMBEDDINGS["baby"]
    })

# Log available voices
print(f"Available voice types: {list(VOICE_TYPES.keys())}", flush=True)


class OvmsPythonModel:
    """
    SpeechT5 Text-to-Speech service implementation
    Supports Chinese text-to-speech synthesis with optional speaker embedding
    """

    def initialize(self, kwargs: dict):
        """
        Initialize the Text2Speech pipeline

        Args:
            kwargs: Initialization parameters containing base_path and node_name
        """
        print("-------------- Running TTS initialize", flush=True)
        print(kwargs)

        # Construct model path
        path = Path(kwargs.get("base_path"))
        model_path = path.parent.parent / "models" / kwargs.get("node_name")

        # Initialize Text2Speech pipeline
        self.pipe = openvino_genai.Text2SpeechPipeline(str(model_path), device="AUTO")

        # Default speaker embedding (can be overridden by input)
        self.default_speaker_embedding = None

        print("-------------- TTS Model loaded", flush=True)

    def execute(self, inputs: list):
        """
        Execute text-to-speech synthesis

        Args:
            inputs: List of input tensors containing text and optional parameters

        Expected inputs:
            - text: Input text to synthesize (required)
            - voice: Optional voice type (male/female/girl/baby/男/女/女孩/宝宝, default: male)
            - speaker_embedding: Optional custom speaker embedding binary data
            - params: Optional JSON parameters for configuration

        Returns:
            List containing audio tensor with WAV format binary data
        """
        try:
            # Default parameters
            text = ""
            speaker_embedding = None
            voice_type = "male"  # Default to male voice (if available)
            sample_rate = DEFAULT_SAMPLE_RATE
            return_format = "wav"  # wav or raw

            # Check if any voices are available
            if not VOICE_TYPES:
                raise RuntimeError("No voice types available. Please ensure custom voice files are loaded.")

            # Parse input tensors
            for input_tensor in inputs:
                if input_tensor.name == "text":
                    text = bytes(input_tensor).decode('utf-8')
                elif input_tensor.name == "voice":
                    # Voice type selection from custom voices
                    voice_input = bytes(input_tensor).decode('utf-8').strip()
                    # Try exact match first, then lowercase match
                    if voice_input in VOICE_TYPES:
                        voice_type = voice_input
                        print(f"Using voice type: {voice_type}", flush=True)
                    elif voice_input.lower() in VOICE_TYPES:
                        voice_type = voice_input.lower()
                        print(f"Using voice type: {voice_type}", flush=True)
                    else:
                        # Find first available voice as fallback
                        if VOICE_TYPES:
                            fallback_voice = list(VOICE_TYPES.keys())[0]
                            voice_type = fallback_voice
                            print(f"Unknown voice type '{voice_input}', using fallback: {fallback_voice}", flush=True)
                            print(f"Available voice types: {list(VOICE_TYPES.keys())}", flush=True)
                        else:
                            raise RuntimeError("No voice types available")
                elif input_tensor.name == "speaker_embedding":
                    # Custom speaker embedding as binary data (512 float32 values)
                    embedding_data = bytes(input_tensor)
                    if len(embedding_data) == 512 * 4:  # 512 float32 values
                        embedding_array = np.frombuffer(embedding_data, dtype=np.float32).reshape(1, 512)
                        speaker_embedding = ov.Tensor(embedding_array)
                        print("Using custom speaker embedding", flush=True)
                    else:
                        print(f"Invalid speaker embedding size: {len(embedding_data)} bytes, "
                              f"expected {512 * 4} bytes", flush=True)
                elif input_tensor.name == "params":
                    # Optional parameters in JSON format
                    try:
                        params_str = bytes(input_tensor).decode('utf-8')
                        user_params = json.loads(params_str)

                        # Update parameters
                        if "sample_rate" in user_params:
                            sample_rate = int(user_params["sample_rate"])
                        if "return_format" in user_params:
                            return_format = user_params["return_format"]
                        # Support voice type in params as well
                        if "voice" in user_params:
                            voice_input = user_params["voice"].lower().strip()
                            if voice_input in VOICE_TYPES:
                                voice_type = voice_input

                    except Exception as e:
                        print(f"Error parsing params: {e}", flush=True)

            # Validate input
            if not text.strip():
                raise ValueError("Input text cannot be empty")

            print(f"Generating speech for text: {text[:50]}{'...' if len(text) > 50 else ''}", flush=True)

            # Prepare speaker embedding
            if speaker_embedding is None:
                # Use selected voice type embedding
                if voice_type not in VOICE_TYPES:
                    # Final fallback - use first available voice
                    if VOICE_TYPES:
                        voice_type = list(VOICE_TYPES.keys())[0]
                        print(f"Voice type not found, using fallback: {voice_type}", flush=True)
                    else:
                        raise RuntimeError("No voice embeddings available")

                voice_embedding = VOICE_TYPES[voice_type]
                speaker_embedding = ov.Tensor(voice_embedding.reshape(1, 512))
                print(f"Using {voice_type} voice embedding", flush=True)

            # Generate speech
            result = self.pipe.generate(text, speaker_embedding)

            # Process result
            if len(result.speeches) != 1:
                raise RuntimeError(f"Expected 1 speech output, got {len(result.speeches)}")

            speech = result.speeches[0]
            audio_data = speech.data[0]  # Get the audio waveform

            print(f"Generated audio: {len(audio_data)} samples at {sample_rate}Hz", flush=True)

            # Convert to output format
            if return_format == "wav":
                # Convert to WAV format binary data
                audio_bytes = self._convert_to_wav(audio_data, sample_rate)
            else:
                # Return raw float32 audio data
                audio_bytes = audio_data.astype(np.float32).tobytes()

            # Return as tensor
            return [Tensor("audio", audio_bytes)]

        except Exception as e:
            print(f"Error during TTS execution: {str(e)}", flush=True)
            # Return error information
            error_message = {
                "status": "error",
                "message": str(e)
            }
            return [Tensor("error", json.dumps(error_message, ensure_ascii=False).encode('utf-8'))]

    def _convert_to_wav(self, audio_data: np.ndarray, sample_rate: int) -> bytes:
        """
        Convert audio data to WAV format binary data

        Args:
            audio_data: Audio waveform as numpy array
            sample_rate: Sample rate

        Returns:
            WAV format binary data
        """
        try:
            import wave

            # Ensure audio data is in the correct format
            if audio_data.dtype != np.float32:
                audio_data = audio_data.astype(np.float32)

            # Normalize to 16-bit range
            audio_int16 = (audio_data * 32767).astype(np.int16)

            # Create WAV file in memory
            wav_buffer = io.BytesIO()

            with wave.open(wav_buffer, 'wb') as wav_file:
                wav_file.setnchannels(1)  # Mono
                wav_file.setsampwidth(2)  # 16-bit
                wav_file.setframerate(sample_rate)
                wav_file.writeframes(audio_int16.tobytes())

            wav_buffer.seek(0)
            return wav_buffer.getvalue()

        except ImportError:
            # Fallback: return raw audio data if wave module not available
            print("Warning: wave module not available, returning raw audio data", flush=True)
            return audio_data.astype(np.float32).tobytes()
        except Exception as e:
            print(f"Error converting to WAV: {e}, returning raw audio data", flush=True)
            return audio_data.astype(np.float32).tobytes()