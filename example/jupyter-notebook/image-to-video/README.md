# Image-to-Video (Image to Video)

## Scenario Description

The image-to-video service can convert static images into dynamic videos. By providing an image and optional descriptive prompts, the AI model can generate short videos with dynamic effects. This is very useful for content creation, animation production, social media, and other scenarios.

## Learning Objectives

Through this example, you will learn:

- How to use AOG's Image-to-Video service to generate videos
- How to prepare and send image data (supports path, URL, Base64)
- How to use prompts to control video generation effects
- How to process and download generated videos

## API Endpoint

```
POST http://localhost:16688/aog/v0.2/services/image-to-video
```

## Main Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | Model name, e.g., `wan2.2-i2v-plus` |
| `image` | string | Yes | Input image (path/URL/Base64) |
| `image_type` | string | Yes | Image type: `path`, `url`, or `base64` |
| `prompt` | string | Optional | Describe desired video dynamic effects |

## Prerequisites

Before running this example, ensure:

- [ ] **AOG service is installed and running**
  ```bash
  # Check AOG service status
  curl http://localhost:16688/aog/v0.2/health
  ```

- [ ] **Image-to-video service provider is configured**
  - This service typically uses cloud APIs (e.g., Alibaba Cloud Tongyi Wanxiang)
  - Need to add the appropriate service provider in AOG configuration
  - Ensure correct API keys and endpoints are set

- [ ] **Python dependencies are installed**
  ```bash
  pip install -r ../requirements.txt
  ```

- [ ] **Test images are prepared**
  - Prepare one or more images for video generation
  - Supports common image formats (PNG, JPG, JPEG, etc.)
  - Recommend using clear images with distinct subjects

## Quick Start

1. Start Jupyter Notebook:
   ```bash
   jupyter notebook
   ```

2. Open `image-to-video.ipynb`

3. Run cells sequentially

4. View generated video results

## Usage Tips

### Prompt Suggestions

- **Animal Actions**:
  - "Make the cat run on the grass"
  - "Make the bird spread its wings and fly"
  - "Make the dog wag its tail"

- **Natural Scenes**:
  - "Make the clouds move slowly"
  - "Make the leaves sway in the wind"
  - "Make ripples on the water surface"

- **Human Actions**:
  - "Make the person smile and wave"
  - "Make the person turn to look at the camera"
  - "Make the person walk forward"

- **Object Motion**:
  - "Make the car drive slowly"
  - "Make the flower bloom"
  - "Make the lights flicker"

### Image Selection Tips

1. **Clarity**: Use high-resolution, clear images
2. **Clear subject**: The subject in the image should be clearly identifiable
3. **Reasonable composition**: Avoid overly complex scenes
4. **Good lighting**: Ensure the image has sufficient lighting

### Image Input Methods

1. **Local path**: For local files
   ```python
   image_type = "path"
   image = "/path/to/image.jpg"
   ```

2. **URL**: For web images
   ```python
   image_type = "url"
   image = "https://example.com/image.jpg"
   ```

3. **Base64**: For encoding scenarios
   ```python
   image_type = "base64"
   image = base64_encoded_string
   ```

## FAQ

### Q: How long are generated videos?
A: Typically generates 3-5 second short videos; specific length depends on model configuration.

### Q: What video formats are supported?
A: Typically generates MP4 format videos, which have good compatibility and are easy to play and share.

### Q: How long does video generation take?
A: Video generation typically takes longer, possibly from tens of seconds to several minutes, depending on the model and server load.

### Q: How to download generated videos?
A: The service returns a video URL, which you can download using the `requests` library or other tools:
```python
import requests
response = requests.get(video_url)
with open("output.mp4", "wb") as f:
    f.write(response.content)
```

### Q: Can I omit the prompt?
A: Yes, the prompt is optional. Without a prompt, the model will automatically generate appropriate dynamic effects based on the image content.

### Q: How to improve video quality?
A: 
- Use high-quality input images
- Provide clear, specific prompts
- Choose images with clear subjects and simple composition
- Ensure images have sufficient lighting

### Q: How to configure cloud services?
A: Refer to AOG documentation on configuring Alibaba Cloud Tongyi Wanxiang service, which requires:
- Register Alibaba Cloud account
- Enable Tongyi Wanxiang service
- Obtain API keys
- Configure service provider in AOG

## Related Resources

- [AOG Documentation](../../../docs/zh-cn/)
- [Image Service Specification](../../../docs/zh-cn/source/service_specs/image.rst)
- [Alibaba Cloud Tongyi Wanxiang](https://help.aliyun.com/zh/dashscope/)

## Next Steps

- Try [Image-to-Image](../image-to-image/) service for image style transfer
- Explore [Text-to-Image](../text-to-image/) service to generate images from text
- Check out other [AOG service examples](../)
