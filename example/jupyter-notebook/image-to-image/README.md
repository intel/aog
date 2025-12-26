# Image-to-Image (Image Style Transfer)

## Scenario Description

The image style transfer service allows you to transform an image into different artistic styles or perform image editing. By providing an original image and descriptive prompts, the AI model can generate images with new styles or characteristics. This is very useful for artistic creation, image editing, style transfer, and other scenarios.

## Learning Objectives

Through this example, you will learn:

- How to use AOG's Image-to-Image service for image style transfer
- How to prepare and send image data (supports path, URL, Base64)
- How to use prompts to control image transformation effects
- How to process and save generated images

## API Endpoint

```
POST http://localhost:16688/aog/v0.2/services/image-to-image
```

## Main Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | Model name, e.g., `wanx2.1-i2i-turbo` |
| `image` | string | Yes | Input image (path/URL/Base64) |
| `image_type` | string | Yes | Image type: `path`, `url`, or `base64` |
| `prompt` | string | Optional | Describe desired style or changes |
| `n` | integer | Optional | Number of images to generate, default is 1 |

## Prerequisites

Before running this example, ensure:

- [ ] **AOG service is installed and running**
  ```bash
  # Check AOG service status
  curl http://localhost:16688/aog/v0.2/health
  ```

- [ ] **Image transformation service provider is configured**
  - This service typically uses cloud APIs (e.g., Alibaba Cloud Tongyi Wanxiang)
  - Need to add the appropriate service provider in AOG configuration
  - Ensure correct API keys and endpoints are set

- [ ] **Python dependencies are installed**
  ```bash
  pip install -r ../requirements.txt
  ```

- [ ] **Test images are prepared**
  - Prepare one or more images for transformation
  - Supports common image formats (PNG, JPG, JPEG, etc.)

## Quick Start

1. Start Jupyter Notebook:
   ```bash
   jupyter notebook
   ```

2. Open `image-to-image.ipynb`

3. Run cells sequentially

4. View generated image results

## Usage Tips

### Prompt Suggestions

- **Style Transfer**:
  - "Convert to oil painting style"
  - "Transform to watercolor style"
  - "Change to cartoon style"
  - "Convert to sketch style"

- **Image Editing**:
  - "Add more details"
  - "Change background to night"
  - "Add more colors"
  - "Make the image brighter"

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

### Q: What image formats are supported?
A: Supports common image formats including PNG, JPG, JPEG, WebP, etc.

### Q: Where are generated images saved?
A: Cloud services typically return image URLs that you need to download and save. Local services return local file paths.

### Q: How to improve transformation quality?
A: 
- Use high-quality input images
- Provide clear, specific prompts
- Try different model parameters
- Generate multiple images (set `n > 1`) and select the best result

### Q: How long does transformation take?
A: Depends on image size and model complexity, typically from a few seconds to tens of seconds.

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

- Try [Image-to-Video](../image-to-video/) service to convert images to videos
- Explore [Text-to-Image](../text-to-image/) service to generate images from text
- Check out other [AOG service examples](../)
