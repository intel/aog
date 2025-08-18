import io

from PIL import Image
from pyovms import Tensor
from pathlib import Path
import openvino_genai

OV_CONFIG = {'PERFORMANCE_HINT': 'LATENCY', 'NUM_STREAMS': '1'}


class OvmsPythonModel:
    def initialize(self, kwargs: dict):
        print("-------------- Running initialize", flush=True)
        print(kwargs)
        path = Path(kwargs.get("base_path"))
        model_path = path.parent.parent / "models" / kwargs.get("node_name")
        self.pipe = openvino_genai.py_openvino_genai.Text2ImagePipeline(model_path, device="AUTO")
        print("-------------- Model loaded", flush=True)

    def execute(self, inputs: list):
        """
        Execute text-to-image generation with robust error handling and input validation.

        Args:
            inputs: List of input tensors containing generation parameters

        Expected inputs:
            - batch: Number of images to generate (1-4, default: 1)
            - height: Image height in pixels (64-2048, default: 512)
            - width: Image width in pixels (64-2048, default: 512)
            - prompt: Text prompt for image generation (required)

        Returns:
            List containing image tensor with binary data format:
            [batch_count(4bytes)] + [image1_size(4bytes) + image1_data] + [image2_size(4bytes) + image2_data] + ...
        """
        try:
            # Default parameters with validation ranges
            batch = 1
            height = 512
            width = 512
            prompt = ""

            # Input validation constants
            MIN_BATCH, MAX_BATCH = 1, 4
            MIN_DIMENSION, MAX_DIMENSION = 64, 2048
            MAX_PROMPT_LENGTH = 1000

            print(f"Processing {len(inputs)} input parameters", flush=True)

            # Parse and validate input parameters
            for i, input_tensor in enumerate(inputs):
                try:
                    if not hasattr(input_tensor, 'name') or not input_tensor.name:
                        print(f"Warning: Input tensor {i} has no name, skipping", flush=True)
                        continue

                    # Safely decode input data
                    try:
                        raw_data = bytes(input_tensor)
                        decoded_str = raw_data.decode('utf-8').strip()
                    except (UnicodeDecodeError, AttributeError) as e:
                        print(f"Warning: Failed to decode input '{input_tensor.name}': {e}", flush=True)
                        continue

                    # Process each parameter type
                    match input_tensor.name.lower():
                        case "batch":
                            try:
                                batch_value = int(decoded_str)
                                if MIN_BATCH <= batch_value <= MAX_BATCH:
                                    batch = batch_value
                                    print(f"Set batch size: {batch}", flush=True)
                                else:
                                    batch = max(MIN_BATCH, min(MAX_BATCH, batch_value))
                                    print(f"Warning: Batch size {batch_value} out of range [{MIN_BATCH}-{MAX_BATCH}], clamped to {batch}", flush=True)
                            except ValueError as e:
                                print(f"Warning: Invalid batch value '{decoded_str}': {e}, using default {batch}", flush=True)

                        case "height":
                            try:
                                height_value = int(decoded_str)
                                if height_value > 0:  # Allow 0 to keep default
                                    if MIN_DIMENSION <= height_value <= MAX_DIMENSION:
                                        height = height_value
                                        print(f"Set height: {height}", flush=True)
                                    else:
                                        height = max(MIN_DIMENSION, min(MAX_DIMENSION, height_value))
                                        print(f"Warning: Height {height_value} out of range [{MIN_DIMENSION}-{MAX_DIMENSION}], clamped to {height}", flush=True)
                            except ValueError as e:
                                print(f"Warning: Invalid height value '{decoded_str}': {e}, using default {height}", flush=True)

                        case "width":
                            try:
                                width_value = int(decoded_str)
                                if width_value > 0:  # Allow 0 to keep default
                                    if MIN_DIMENSION <= width_value <= MAX_DIMENSION:
                                        width = width_value
                                        print(f"Set width: {width}", flush=True)
                                    else:
                                        width = max(MIN_DIMENSION, min(MAX_DIMENSION, width_value))
                                        print(f"Warning: Width {width_value} out of range [{MIN_DIMENSION}-{MAX_DIMENSION}], clamped to {width}", flush=True)
                            except ValueError as e:
                                print(f"Warning: Invalid width value '{decoded_str}': {e}, using default {width}", flush=True)

                        case "prompt":
                            if len(decoded_str) > MAX_PROMPT_LENGTH:
                                prompt = decoded_str[:MAX_PROMPT_LENGTH]
                                print(f"Warning: Prompt truncated to {MAX_PROMPT_LENGTH} characters", flush=True)
                            else:
                                prompt = decoded_str
                            print(f"Set prompt: {prompt[:50]}{'...' if len(prompt) > 50 else ''}", flush=True)

                        case _:
                            print(f"Warning: Unknown parameter '{input_tensor.name}', ignoring", flush=True)

                except Exception as e:
                    print(f"Error processing input parameter {i} ('{getattr(input_tensor, 'name', 'unknown')}'): {e}", flush=True)
                    continue

            # Validate final parameters
            if not prompt.strip():
                raise ValueError("Prompt cannot be empty. Please provide a valid text prompt for image generation.")

            # Ensure dimensions are multiples of 8 for better model compatibility
            height = (height // 8) * 8
            width = (width // 8) * 8

            print(f"Final parameters - Batch: {batch}, Size: {width}x{height}, Prompt length: {len(prompt)}", flush=True)

            # Generate images with error handling
            try:
                print("Starting image generation...", flush=True)
                image_tensors = self.pipe.generate(
                    prompt,
                    width=width,
                    height=height,
                    num_images_per_prompt=batch
                )
                print("Image generation completed successfully", flush=True)

            except Exception as e:
                print(f"Image generation failed: {e}", flush=True)
                raise RuntimeError(f"Failed to generate images: {str(e)}")

            # Validate generation results
            if not hasattr(image_tensors, 'data'):
                raise RuntimeError("Image generation returned invalid results: missing 'data' attribute")

            # Check if data exists and has content (avoid numpy array boolean ambiguity)
            try:
                data_length = len(image_tensors.data)
                if data_length == 0:
                    raise RuntimeError("Image generation returned empty results")
            except (TypeError, AttributeError):
                raise RuntimeError("Image generation returned invalid data structure")

            if data_length != batch:
                print(f"Warning: Expected {batch} images, got {data_length}", flush=True)
                batch = data_length  # Update batch to actual count

            # Process and encode images
            try:
                raw_output = batch.to_bytes(4, 'little')
                total_size = 0

                for i in range(batch):
                    try:
                        # Convert tensor to PIL Image
                        image_array = image_tensors.data[i]

                        # Check if image data is valid (avoid numpy array boolean issues)
                        if image_array is None:
                            raise ValueError(f"Image {i} data is None")

                        # Additional validation for array-like objects
                        try:
                            if hasattr(image_array, 'size') and image_array.size == 0:
                                raise ValueError(f"Image {i} data is empty array")
                        except (AttributeError, ValueError):
                            pass  # Continue if size check fails, let PIL handle it

                        image = Image.fromarray(image_array)

                        # Encode as PNG
                        img_byte_arr = io.BytesIO()
                        image.save(img_byte_arr, format='PNG', optimize=True)
                        img_bytes = img_byte_arr.getvalue()

                        if len(img_bytes) == 0:
                            raise ValueError(f"Image {i} encoding resulted in empty data")

                        # Add to output
                        raw_output += len(img_bytes).to_bytes(4, 'little') + img_bytes
                        total_size += len(img_bytes)

                        print(f"Processed image {i+1}/{batch}, size: {len(img_bytes)} bytes", flush=True)

                    except Exception as e:
                        print(f"Error processing image {i}: {e}", flush=True)
                        raise RuntimeError(f"Failed to process image {i}: {str(e)}")

                print(f"Successfully generated {batch} images, total size: {total_size} bytes", flush=True)
                return [Tensor("image", raw_output)]

            except Exception as e:
                print(f"Error encoding images: {e}", flush=True)
                raise RuntimeError(f"Failed to encode generated images: {str(e)}")

        except Exception as e:
            # Comprehensive error handling with detailed logging
            error_msg = f"Text-to-image execution failed: {str(e)}"
            print(f"ERROR: {error_msg}", flush=True)

            # Return error information as a special tensor
            error_data = {
                "status": "error",
                "message": str(e),
                "error_type": type(e).__name__
            }

            try:
                import json
                error_bytes = json.dumps(error_data, ensure_ascii=False).encode('utf-8')
                return [Tensor("error", error_bytes)]
            except Exception as json_error:
                # Fallback to simple error message if JSON encoding fails
                print(f"Failed to encode error as JSON: {json_error}", flush=True)
                fallback_error = f"EXECUTION_ERROR: {str(e)}".encode('utf-8')
                return [Tensor("error", fallback_error)]
