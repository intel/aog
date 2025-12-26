# AOG Jupyter Notebook 示例

欢迎使用 AOG (AI On GPU) Jupyter Notebook 示例集！本目录包含交互式 Python notebook，演示如何使用 AOG 的各种 AI 服务。

## ⚠️ 先决条件

在运行这些示例之前，请确保：

### 1. AOG 服务已安装并运行

```bash
# 在项目根目录启动 AOG 服务
./aog
```

服务默认运行在 `http://localhost:16688`。

### 2. 已安装并配置所需的模型服务

每个场景需要对应的服务和模型：

#### 文本生成场景
- **所需服务**: Chat 服务
- **所需模型**: 例如 `qwen2.5:0.5b`、`deepseek-r1:7b`
- **安装方法**: 参考 [AOG 文档](../../docs/) 安装 Chat 服务和下载对应模型

#### Generate 服务场景
- **所需服务**: Generate 服务
- **所需模型**: 例如 `gemma3:4b`
- **支持**: 多模态输入（文本+图像）
- **安装方法**: 参考 [AOG 文档](../../docs/) 安装 Generate 服务和下载对应模型

#### Embed 服务场景
- **所需服务**: Embed 服务
- **所需模型**: 例如 `text-embedding-v3`
- **安装方法**: 参考 [AOG 文档](../../docs/) 安装 Embed 服务和下载对应模型

#### 文本转图像场景
- **所需服务**: Text-to-Image 服务
- **所需模型**: 例如 `OpenVINO/LCM_Dreamshaper_v7-fp16-ov`、`wanx2.1-t2i-turbo`（云端）
- **云端支持**: 阿里云通义万相等
- **安装方法**: 参考 [AOG 文档](../../docs/) 安装 Text-to-Image 服务和下载对应模型

#### 图生图场景
- **所需服务**: Image-to-Image 服务
- **所需模型**: 例如 `wanx2.1-i2i-turbo`（云端）
- **云端支持**: 阿里云通义万相等
- **安装方法**: 参考 [AOG 文档](../../docs/) 配置云端服务

#### 图生视频场景
- **所需服务**: Image-to-Video 服务
- **所需模型**: 例如 `wan2.2-i2v-plus`（云端）
- **云端支持**: 阿里云通义万相等
- **安装方法**: 参考 [AOG 文档](../../docs/) 配置云端服务

#### 语音转文本场景
- **所需服务**: Speech-to-Text 服务
- **所需模型**: 例如 `NamoLi/whisper-large-v3-ov`
- **安装方法**: 参考 [AOG 文档](../../docs/) 安装 Speech-to-Text 服务和下载对应模型

#### 实时语音转文本场景
- **所需服务**: Speech-to-Text-WS 服务（WebSocket）
- **所需模型**: 例如 `NamoLi/whisper-large-v3-ov`
- **安装方法**: 参考 [AOG 文档](../../docs/) 安装 Speech-to-Text-WS 服务和下载对应模型

#### 文本转语音场景
- **所需服务**: Text-to-Speech 服务
- **所需模型**: 例如 `NamoLi/speecht5-tts`
- **限制**: 当前仅支持英文
- **安装方法**: 参考 [AOG 文档](../../docs/) 安装 Text-to-Speech 服务和下载对应模型

> 💡 **提示**: 如果服务或模型未安装，运行 notebook 时会出现连接错误或模型不存在的错误。

## 📚 场景索引

| 场景 | 描述 | Notebook |
|------|------|----------|
| [文本生成](./text-generation/) | 使用 AOG Chat API 进行对话和文本生成 | [text-generation.ipynb](./text-generation/text-generation.ipynb) |
| [Generate 服务](./generate/) | 使用 Generate API 进行多模态内容生成 | [generate.ipynb](./generate/generate.ipynb) |
| [Embed 服务](./embed/) | 使用 Embed API 生成文本嵌入向量 | [embed.ipynb](./embed/embed.ipynb) |
| [文本转图像](./text-to-image/) | 使用 AOG Text-to-Image API 生成图像 | [text-to-image.ipynb](./text-to-image/text-to-image.ipynb) |
| [图生图](./image-to-image/) | 使用 Image-to-Image API 进行图像风格转换 | [image-to-image.ipynb](./image-to-image/image-to-image.ipynb) |
| [图生视频](./image-to-video/) | 使用 Image-to-Video API 将图像转换为视频 | [image-to-video.ipynb](./image-to-video/image-to-video.ipynb) |
| [语音转文本](./speech-to-text/) | 使用 Speech-to-Text API 进行语音识别 | [speech-to-text.ipynb](./speech-to-text/speech-to-text.ipynb) |
| [实时语音转文本](./speech-to-text-ws/) | 使用 WebSocket 进行实时语音识别 | [speech-to-text-ws.ipynb](./speech-to-text-ws/speech-to-text-ws.ipynb) |
| [文本转语音](./text-to-speech/) | 使用 Text-to-Speech API 进行语音合成 | [text-to-speech.ipynb](./text-to-speech/text-to-speech.ipynb) |

## 🚀 快速开始

### 1. 安装依赖

```bash
# 进入 jupyter-notebook 目录
cd example/jupyter-notebook

# 安装 Python 依赖
pip install -r requirements.txt
```

### 2. 确保 AOG 服务和模型已准备

确保 AOG 服务正在运行，并且已安装所需的服务和模型。参考上面的[先决条件](#⚠️-先决条件)部分。

### 3. 启动 Jupyter Notebook

```bash
# 启动 Jupyter Notebook
jupyter notebook
```

浏览器会自动打开，然后你可以选择想要运行的场景目录和 notebook 文件。

## 📖 使用说明

每个场景目录包含：
- **README.md** - 场景说明、API 端点信息和参数说明
- **{service-name}.ipynb** - 可执行的 Jupyter Notebook 示例

所有 notebook 都是自包含的，包含所有必要的代码和注释，无需额外的配置文件或工具模块。

## 🔧 配置

如果你的 AOG 服务运行在不同的地址或端口，可以在 notebook 开头修改配置：

```python
# 修改 AOG 服务地址
AOG_BASE_URL = "http://your-host:your-port"
```

## ❓ 故障排除

### 无法连接到 AOG 服务

**问题**: 运行 notebook 时出现连接错误

**解决方案**:
1. 确认 AOG 服务已启动：`./aog`
2. 检查服务地址是否正确（默认 `http://localhost:16688`）
3. 确认防火墙没有阻止连接

### 模型不存在或服务未安装

**问题**: 提示模型不存在或服务不可用

**解决方案**:
1. 确认已安装对应的服务（Chat 或 Text-to-Image）
2. 确认已下载所需的模型文件
3. 查看 [AOG 文档](../../docs/) 了解如何安装服务和下载模型
4. 检查模型名称是否正确

### 依赖安装失败

**问题**: `pip install -r requirements.txt` 失败

**解决方案**:
1. 确保使用 Python 3.8 或更高版本：`python --version`
2. 升级 pip：`pip install --upgrade pip`
3. 如果使用虚拟环境，确保已激活

### Jupyter Notebook 无法启动

**问题**: `jupyter notebook` 命令不存在

**解决方案**:
1. 确认已安装 jupyter：`pip install jupyter`
2. 检查 PATH 环境变量是否包含 Python 的 bin 目录
3. 尝试使用完整路径：`python -m jupyter notebook`

### 图像无法显示

**问题**: text-to-image notebook 中图像不显示

**解决方案**:
1. 确认已安装 Pillow：`pip install pillow`
2. 检查 AOG 服务返回的响应格式
3. 确认 base64 解码没有错误

## 📝 更多信息

- [AOG 项目主页](../../README.md)
- [AOG API 文档](../../docs/)
- 如有问题，请提交 [Issue](https://github.com/your-repo/issues)

## 🤝 贡献

欢迎贡献新的示例场景！请参考现有的 notebook 结构，确保：
- 代码自包含，无外部依赖
- 包含详细的中文注释
- 提供清晰的使用说明
- 遵循简洁的目录结构

---

**祝你使用愉快！** 🎉
