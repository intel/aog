# Image-to-Video（图像转视频）

## 场景描述

图像转视频服务可以将静态图像转换为动态视频。通过提供一张图像和可选的描述性提示词，AI 模型可以生成具有动态效果的短视频。这在内容创作、动画制作、社交媒体等场景中非常有用。

## 学习目标

通过本示例，您将学习：

- 如何使用 AOG 的 Image-to-Video 服务生成视频
- 如何准备和发送图像数据（支持路径、URL、Base64）
- 如何使用提示词控制视频生成效果
- 如何处理和下载生成的视频

## API 端点

```
POST http://localhost:16688/aog/v0.2/services/image-to-video
```

## 主要参数

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `model` | string | 是 | 使用的模型名称，如 `wan2.2-i2v-plus` |
| `image` | string | 是 | 输入图像（路径/URL/Base64） |
| `image_type` | string | 是 | 图像类型：`path`、`url` 或 `base64` |
| `prompt` | string | 可选 | 描述期望的视频动态效果 |

## 先决条件

在运行本示例之前，请确保：

- [ ] **AOG 服务已安装并运行**
  ```bash
  # 检查 AOG 服务状态
  curl http://localhost:16688/aog/v0.2/health
  ```

- [ ] **已配置图像转视频服务提供商**
  - 本服务通常使用云端 API（如阿里云通义万相）
  - 需要在 AOG 配置中添加相应的服务提供商
  - 确保已设置正确的 API 密钥和端点

- [ ] **已安装 Python 依赖**
  ```bash
  pip install -r ../requirements.txt
  ```

- [ ] **准备测试图像**
  - 准备一张或多张用于生成视频的图像
  - 支持常见图像格式（PNG、JPG、JPEG 等）
  - 建议使用清晰、主体明确的图像

## 快速开始

1. 启动 Jupyter Notebook：
   ```bash
   jupyter notebook
   ```

2. 打开 `image-to-video.ipynb`

3. 按顺序运行单元格

4. 查看生成的视频结果

## 使用技巧

### 提示词建议

- **动物动作**：
  - "让猫在草地上奔跑"
  - "让鸟儿展翅飞翔"
  - "让狗摇尾巴"

- **自然场景**：
  - "让云朵缓缓移动"
  - "让树叶随风摇摆"
  - "让水面泛起涟漪"

- **人物动作**：
  - "让人物微笑并挥手"
  - "让人物转头看向镜头"
  - "让人物走向前方"

- **物体运动**：
  - "让汽车缓缓驶过"
  - "让花朵绽放"
  - "让灯光闪烁"

### 图像选择建议

1. **清晰度**：使用高分辨率、清晰的图像
2. **主体明确**：图像中的主体应该清晰可辨
3. **构图合理**：避免过于复杂的场景
4. **光线良好**：确保图像光线充足

### 图像输入方式

1. **本地路径**：适合本地文件
   ```python
   image_type = "path"
   image = "/path/to/image.jpg"
   ```

2. **URL**：适合网络图像
   ```python
   image_type = "url"
   image = "https://example.com/image.jpg"
   ```

3. **Base64**：适合需要编码的场景
   ```python
   image_type = "base64"
   image = base64_encoded_string
   ```

## 常见问题

### Q: 生成的视频有多长？
A: 通常生成 3-5 秒的短视频，具体长度取决于模型配置。

### Q: 支持哪些视频格式？
A: 通常生成 MP4 格式的视频，兼容性好，易于播放和分享。

### Q: 生成视频需要多长时间？
A: 视频生成通常需要较长时间，可能需要几十秒到几分钟，取决于模型和服务器负载。

### Q: 如何下载生成的视频？
A: 服务返回视频的 URL，您可以使用 `requests` 库或其他工具下载：
```python
import requests
response = requests.get(video_url)
with open("output.mp4", "wb") as f:
    f.write(response.content)
```

### Q: 可以不提供提示词吗？
A: 可以，提示词是可选的。不提供提示词时，模型会根据图像内容自动生成合适的动态效果。

### Q: 如何提高视频质量？
A: 
- 使用高质量的输入图像
- 提供清晰、具体的提示词
- 选择主体明确、构图简单的图像
- 确保图像光线充足

### Q: 如何配置云端服务？
A: 请参考 AOG 文档中关于配置阿里云通义万相服务的说明，需要：
- 注册阿里云账号
- 开通通义万相服务
- 获取 API 密钥
- 在 AOG 中配置服务提供商

## 相关资源

- [AOG 文档](../../../docs/zh-cn/)
- [Image 服务规范](../../../docs/zh-cn/source/service_specs/image.rst)
- [阿里云通义万相](https://help.aliyun.com/zh/dashscope/)

## 下一步

- 尝试 [Image-to-Image](../image-to-image/) 服务，进行图像风格转换
- 探索 [Text-to-Image](../text-to-image/) 服务，从文本生成图像
- 查看其他 [AOG 服务示例](../)
