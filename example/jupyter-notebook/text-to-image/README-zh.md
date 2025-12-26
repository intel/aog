# 文本转图像场景示例

本示例演示如何使用 AOG Text-to-Image API 根据文本描述生成图像。

## 📝 场景描述

文本转图像服务可以：
- 根据文本描述生成图像
- 创建艺术作品和插图
- 快速原型设计和概念可视化
- 生成创意内容

## 🎯 学习目标

通过本示例，你将学会：
1. 如何调用 AOG Text-to-Image API
2. 如何编写有效的提示词（prompt）
3. 如何处理 base64 编码的图像数据
4. 如何在 Jupyter Notebook 中显示图像
5. 如何保存生成的图像
6. 如何进行错误处理

## 🔌 API 端点

```
POST http://localhost:16688/aog/v0.2/services/text-to-image
```

## 📋 请求参数

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `model` | string | 是 | 模型名称，如 `OpenVINO/LCM_Dreamshaper_v7-fp16-ov` |
| `prompt` | string | 是 | 文本描述，用于生成图像 |

### 请求示例

```json
{
  "model": "OpenVINO/LCM_Dreamshaper_v7-fp16-ov",
  "prompt": "A beautiful sunset over mountains, digital art"
}
```

## 📊 响应格式

响应包含 base64 编码的图像数据：

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

## 🚀 快速开始

### 先决条件

1. ✅ AOG 服务已安装并运行
2. ✅ Text-to-Image 服务已安装
3. ✅ 已下载所需的图像生成模型（如 `OpenVINO/LCM_Dreamshaper_v7-fp16-ov`）

### 运行步骤

1. 确保 AOG 服务正在运行
2. 打开 [text-to-image.ipynb](./text-to-image.ipynb)
3. 按顺序执行 notebook 中的代码单元格

## 💡 提示词编写技巧

### 1. 基本结构

一个好的提示词通常包含：
- **主体**: 你想要生成什么（人物、物体、场景）
- **细节**: 颜色、风格、氛围
- **质量词**: 高质量、详细、专业等

示例：
```
A majestic lion in the savanna, golden hour lighting, photorealistic, highly detailed
```

### 2. 风格关键词

添加风格关键词可以控制图像的艺术风格：
- `digital art` - 数字艺术
- `oil painting` - 油画
- `watercolor` - 水彩画
- `photorealistic` - 照片级真实
- `anime style` - 动漫风格
- `3D render` - 3D 渲染

### 3. 质量提升词

这些词可以提升生成质量：
- `highly detailed` - 高度细节
- `8k resolution` - 8K 分辨率
- `professional` - 专业的
- `masterpiece` - 杰作
- `best quality` - 最佳质量

### 4. 氛围和光照

描述氛围和光照效果：
- `golden hour` - 黄金时刻
- `dramatic lighting` - 戏剧性光照
- `soft light` - 柔和光线
- `moody atmosphere` - 情绪化氛围
- `cinematic` - 电影感

## 📝 提示词示例

### 自然风景
```
A serene mountain lake at sunrise, mist over water, pine trees, 
reflection, peaceful atmosphere, landscape photography
```

### 人物肖像
```
Portrait of a young woman, natural lighting, soft focus, 
professional photography, warm tones
```

### 科幻场景
```
Futuristic city skyline at night, neon lights, flying cars, 
cyberpunk style, highly detailed, digital art
```

### 抽象艺术
```
Abstract geometric patterns, vibrant colors, modern art, 
minimalist design, high contrast
```

## 🔍 常见问题

**Q: 如何提高生成图像的质量？**  
A: 使用详细的描述，添加质量关键词（如 "highly detailed", "professional"），明确指定风格。

**Q: 生成的图像不符合预期怎么办？**  
A: 尝试调整提示词，添加更多细节描述，或者使用否定提示词（如果 API 支持）来排除不想要的元素。

**Q: 可以生成特定尺寸的图像吗？**  
A: 这取决于模型的支持。某些模型可能支持尺寸参数，请查看 AOG API 文档。

**Q: base64 编码的图像如何保存？**  
A: 使用 Python 的 base64 库解码，然后用 PIL 保存为常见格式（PNG、JPEG 等）。示例代码在 notebook 中。

## 🎨 创意建议

1. **组合多个概念**: 将不同的元素组合在一起创造独特的图像
2. **实验不同风格**: 尝试各种艺术风格，找到最适合的
3. **迭代优化**: 根据生成结果逐步调整提示词
4. **参考优秀作品**: 学习其他人的提示词写法

## 📚 相关资源

- [AOG API 文档](../../docs/)
- [返回主页](../README.md)
- [文本生成示例](../text-generation/)
- [提示词工程指南](https://platform.openai.com/docs/guides/images/prompting)
