# Image-to-Image（图像风格转换）

## 场景描述

图像风格转换服务允许您将一张图像转换为不同的艺术风格或进行图像编辑。通过提供原始图像和描述性提示词，AI 模型可以生成具有新风格或特征的图像。这在艺术创作、图像编辑、风格迁移等场景中非常有用。

## 学习目标

通过本示例，您将学习：

- 如何使用 AOG 的 Image-to-Image 服务进行图像风格转换
- 如何准备和发送图像数据（支持路径、URL、Base64）
- 如何使用提示词控制图像转换效果
- 如何处理和保存生成的图像

## API 端点

```
POST http://localhost:16688/aog/v0.2/services/image-to-image
```

## 主要参数

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `model` | string | 是 | 使用的模型名称，如 `wanx2.1-i2i-turbo` |
| `image` | string | 是 | 输入图像（路径/URL/Base64） |
| `image_type` | string | 是 | 图像类型：`path`、`url` 或 `base64` |
| `prompt` | string | 可选 | 描述期望的风格或变化 |
| `n` | integer | 可选 | 生成图像数量，默认为 1 |

## 先决条件

在运行本示例之前，请确保：

- [ ] **AOG 服务已安装并运行**
  ```bash
  # 检查 AOG 服务状态
  curl http://localhost:16688/aog/v0.2/health
  ```

- [ ] **已配置图像转换服务提供商**
  - 本服务通常使用云端 API（如阿里云通义万相）
  - 需要在 AOG 配置中添加相应的服务提供商
  - 确保已设置正确的 API 密钥和端点

- [ ] **已安装 Python 依赖**
  ```bash
  pip install -r ../requirements.txt
  ```

- [ ] **准备测试图像**
  - 准备一张或多张用于转换的图像
  - 支持常见图像格式（PNG、JPG、JPEG 等）

## 快速开始

1. 启动 Jupyter Notebook：
   ```bash
   jupyter notebook
   ```

2. 打开 `image-to-image.ipynb`

3. 按顺序运行单元格

4. 查看生成的图像结果

## 使用技巧

### 提示词建议

- **风格转换**：
  - "改为油画风格"
  - "转换为水彩画风格"
  - "改为卡通风格"
  - "转换为素描风格"

- **图像编辑**：
  - "增加更多细节"
  - "改变背景为夜晚"
  - "添加更多色彩"
  - "使图像更加明亮"

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

### Q: 支持哪些图像格式？
A: 支持常见的图像格式，包括 PNG、JPG、JPEG、WebP 等。

### Q: 生成的图像保存在哪里？
A: 云端服务通常返回图像 URL，您需要下载并保存。本地服务会返回本地文件路径。

### Q: 如何提高转换质量？
A: 
- 使用高质量的输入图像
- 提供清晰、具体的提示词
- 尝试不同的模型参数
- 可以生成多张图像（设置 `n > 1`）选择最佳结果

### Q: 转换需要多长时间？
A: 取决于图像大小和模型复杂度，通常在几秒到几十秒之间。

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

- 尝试 [Image-to-Video](../image-to-video/) 服务，将图像转换为视频
- 探索 [Text-to-Image](../text-to-image/) 服务，从文本生成图像
- 查看其他 [AOG 服务示例](../)
