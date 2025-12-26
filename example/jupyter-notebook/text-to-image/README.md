# Text-to-Image Example

This example demonstrates how to use the AOG Text-to-Image API to generate images from text descriptions.

## 📝 Scenario Description

The text-to-image service can:
- Generate images from text descriptions
- Create artwork and illustrations
- Rapid prototyping and concept visualization
- Generate creative content

## 🎯 Learning Objectives

Through this example, you will learn:
1. How to call the AOG Text-to-Image API
2. How to write effective prompts
3. How to handle base64-encoded image data
4. How to display images in Jupyter Notebook
5. How to save generated images
6. How to implement error handling

## 🔌 API Endpoint

```
POST http://localhost:16688/aog/v0.2/services/text-to-image
```

## 📋 Request Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | Model name, e.g., `OpenVINO/LCM_Dreamshaper_v7-fp16-ov` |
| `prompt` | string | Yes | Text description for image generation |

### Request Example

```json
{
  "model": "OpenVINO/LCM_Dreamshaper_v7-fp16-ov",
  "prompt": "A beautiful sunset over mountains, digital art"
}
```

## 📊 Response Format

Response contains base64-encoded image data:

```json
{
  "created": 1234567890,
  "data": [
    {
      "b64_json": "iVBORw0KGgoAAAANSUhEUgAA..."
    }
  ]
}
```

## 🚀 Quick Start

### Prerequisites

1. ✅ AOG service is installed and running
2. ✅ Text-to-Image service is installed
3. ✅ Required image generation models are downloaded (e.g., `OpenVINO/LCM_Dreamshaper_v7-fp16-ov`)

### Steps

1. Ensure AOG service is running
2. Open [text-to-image.ipynb](./text-to-image.ipynb)
3. Execute the code cells in the notebook sequentially

## 💡 Prompt Writing Tips

### 1. Basic Structure

A good prompt typically includes:
- **Subject**: What you want to generate (person, object, scene)
- **Details**: Colors, style, atmosphere
- **Quality words**: High quality, detailed, professional, etc.

Example:
```
A majestic lion in the savanna, golden hour lighting, photorealistic, highly detailed
```

### 2. Style Keywords

Add style keywords to control the artistic style:
- `digital art` - Digital artwork
- `oil painting` - Oil painting
- `watercolor` - Watercolor
- `photorealistic` - Photorealistic
- `anime style` - Anime style
- `3D render` - 3D rendering

### 3. Quality Enhancement Words

These words can improve generation quality:
- `highly detailed` - Highly detailed
- `8k resolution` - 8K resolution
- `professional` - Professional
- `masterpiece` - Masterpiece
- `best quality` - Best quality

### 4. Atmosphere and Lighting

Describe atmosphere and lighting effects:
- `golden hour` - Golden hour
- `dramatic lighting` - Dramatic lighting
- `soft light` - Soft light
- `moody atmosphere` - Moody atmosphere
- `cinematic` - Cinematic

## 📝 Prompt Examples

### Natural Landscape
```
A serene mountain lake at sunrise, mist over water, pine trees, 
reflection, peaceful atmosphere, landscape photography
```

### Portrait
```
Portrait of a young woman, natural lighting, soft focus, 
professional photography, warm tones
```

### Sci-Fi Scene
```
Futuristic city skyline at night, neon lights, flying cars, 
cyberpunk style, highly detailed, digital art
```

### Abstract Art
```
Abstract geometric patterns, vibrant colors, modern art, 
minimalist design, high contrast
```

## 🔍 Common Questions

**Q: How can I improve the quality of generated images?**  
A: Use detailed descriptions, add quality keywords (like "highly detailed", "professional"), and specify the style clearly.

**Q: What if the generated image doesn't match expectations?**  
A: Try adjusting the prompt, add more detailed descriptions, or use negative prompts (if the API supports it) to exclude unwanted elements.

**Q: Can I generate images of specific sizes?**  
A: This depends on model support. Some models may support size parameters, please check the AOG API documentation.

**Q: How do I save base64-encoded images?**  
A: Use Python's base64 library to decode, then save with PIL to common formats (PNG, JPEG, etc.). Example code is in the notebook.

## 🎨 Creative Suggestions

1. **Combine multiple concepts**: Combine different elements to create unique images
2. **Experiment with different styles**: Try various artistic styles to find the best fit
3. **Iterative refinement**: Gradually adjust prompts based on generated results
4. **Reference excellent works**: Learn from others' prompt writing techniques

## 📚 Related Resources

- [AOG API Documentation](../../docs/)
- [Back to Home](../README.md)
- [Text Generation Example](../text-generation/)
- [Prompt Engineering Guide](https://platform.openai.com/docs/guides/images/prompting)
