# AOG Jupyter Notebook Examples

Welcome to the AOG (AI On GPU) Jupyter Notebook examples! This directory contains interactive Python notebooks demonstrating how to use various AOG AI services.

## ⚠️ Prerequisites

Before running these examples, ensure:

### 1. AOG Service is Installed and Running

```bash
# Start AOG service from project root
./aog
```

The service runs on `http://localhost:16688` by default.

### 2. Required Model Services are Installed and Configured

Each scenario requires corresponding services and models:

#### Text Generation
- **Required Service**: Chat service
- **Required Models**: e.g., `qwen2.5:0.5b`, `deepseek-r1:7b`
- **Installation**: Refer to [AOG Documentation](../../docs/) for installing Chat service and downloading models

#### Generate Service
- **Required Service**: Generate service
- **Required Models**: e.g., `gemma3:4b`
- **Support**: Multimodal input (text + images)
- **Installation**: Refer to [AOG Documentation](../../docs/) for installing Generate service and downloading models

#### Embed Service
- **Required Service**: Embed service
- **Required Models**: e.g., `text-embedding-v3`
- **Installation**: Refer to [AOG Documentation](../../docs/) for installing Embed service and downloading models

#### Text-to-Image
- **Required Service**: Text-to-Image service
- **Required Models**: e.g., `OpenVINO/LCM_Dreamshaper_v7-fp16-ov`, `wanx2.1-t2i-turbo` (cloud)
- **Cloud Support**: Alibaba Cloud Tongyi Wanxiang, etc.
- **Installation**: Refer to [AOG Documentation](../../docs/) for installing Text-to-Image service and downloading models

#### Image-to-Image
- **Required Service**: Image-to-Image service
- **Required Models**: e.g., `wanx2.1-i2i-turbo` (cloud)
- **Cloud Support**: Alibaba Cloud Tongyi Wanxiang, etc.
- **Installation**: Refer to [AOG Documentation](../../docs/) for configuring cloud services

#### Image-to-Video
- **Required Service**: Image-to-Video service
- **Required Models**: e.g., `wan2.2-i2v-plus` (cloud)
- **Cloud Support**: Alibaba Cloud Tongyi Wanxiang, etc.
- **Installation**: Refer to [AOG Documentation](../../docs/) for configuring cloud services

#### Speech-to-Text
- **Required Service**: Speech-to-Text service
- **Required Models**: e.g., `NamoLi/whisper-large-v3-ov`
- **Installation**: Refer to [AOG Documentation](../../docs/) for installing Speech-to-Text service and downloading models

#### Real-time Speech-to-Text
- **Required Service**: Speech-to-Text-WS service (WebSocket)
- **Required Models**: e.g., `NamoLi/whisper-large-v3-ov`
- **Installation**: Refer to [AOG Documentation](../../docs/) for installing Speech-to-Text-WS service and downloading models

#### Text-to-Speech
- **Required Service**: Text-to-Speech service
- **Required Models**: e.g., `NamoLi/speecht5-tts`
- **Limitation**: Currently supports English only
- **Installation**: Refer to [AOG Documentation](../../docs/) for installing Text-to-Speech service and downloading models

> 💡 **Tip**: If services or models are not installed, you'll encounter connection errors or model not found errors when running notebooks.

## 📚 Scenario Index

| Scenario | Description | Notebook |
|----------|-------------|----------|
| [Text Generation](./text-generation/) | Conversation and text generation using AOG Chat API | [text-generation.ipynb](./text-generation/text-generation.ipynb) |
| [Generate Service](./generate/) | Multimodal content generation using Generate API | [generate.ipynb](./generate/generate.ipynb) |
| [Embed Service](./embed/) | Generate text embeddings using Embed API | [embed.ipynb](./embed/embed.ipynb) |
| [Text-to-Image](./text-to-image/) | Image generation using AOG Text-to-Image API | [text-to-image.ipynb](./text-to-image/text-to-image.ipynb) |
| [Image-to-Image](./image-to-image/) | Image style transfer using Image-to-Image API | [image-to-image.ipynb](./image-to-image/image-to-image.ipynb) |
| [Image-to-Video](./image-to-video/) | Convert images to videos using Image-to-Video API | [image-to-video.ipynb](./image-to-video/image-to-video.ipynb) |
| [Speech-to-Text](./speech-to-text/) | Speech recognition using Speech-to-Text API | [speech-to-text.ipynb](./speech-to-text/speech-to-text.ipynb) |
| [Real-time Speech-to-Text](./speech-to-text-ws/) | Real-time speech recognition using WebSocket | [speech-to-text-ws.ipynb](./speech-to-text-ws/speech-to-text-ws.ipynb) |
| [Text-to-Speech](./text-to-speech/) | Speech synthesis using Text-to-Speech API | [text-to-speech.ipynb](./text-to-speech/text-to-speech.ipynb) |

## 🚀 Quick Start

### 1. Install Dependencies

```bash
# Navigate to jupyter-notebook directory
cd example/jupyter-notebook

# Install Python dependencies
pip install -r requirements.txt
```

### 2. Ensure AOG Service and Models are Ready

Make sure the AOG service is running and required services and models are installed. Refer to the [Prerequisites](#⚠️-prerequisites) section above.

### 3. Launch Jupyter Notebook

```bash
# Start Jupyter Notebook
jupyter notebook
```

Your browser will open automatically, then you can select the scenario directory and notebook file you want to run.

## 📖 Usage

Each scenario directory contains:
- **README.md** - Scenario description, API endpoint information, and parameter descriptions
- **{service-name}.ipynb** - Executable Jupyter Notebook example

All notebooks are self-contained with all necessary code and comments, requiring no additional configuration files or utility modules.

## 🔧 Configuration

If your AOG service runs on a different address or port, modify the configuration at the beginning of the notebook:

```python
# Modify AOG service address
AOG_BASE_URL = "http://your-host:your-port"
```

## ❓ Troubleshooting

### Cannot Connect to AOG Service

**Issue**: Connection error when running notebook

**Solution**:
1. Confirm AOG service is running: `./aog`
2. Check if service address is correct (default `http://localhost:16688`)
3. Ensure firewall is not blocking the connection

### Model Not Found or Service Not Installed

**Issue**: Model not found or service unavailable error

**Solution**:
1. Confirm the corresponding service (Chat or Text-to-Image) is installed
2. Confirm required model files are downloaded
3. See [AOG Documentation](../../docs/) for how to install services and download models
4. Check if model name is correct

### Dependency Installation Failed

**Issue**: `pip install -r requirements.txt` fails

**Solution**:
1. Ensure Python 3.8 or higher: `python --version`
2. Upgrade pip: `pip install --upgrade pip`
3. If using virtual environment, ensure it's activated

### Jupyter Notebook Won't Start

**Issue**: `jupyter notebook` command not found

**Solution**:
1. Confirm jupyter is installed: `pip install jupyter`
2. Check if PATH environment variable includes Python's bin directory
3. Try using full path: `python -m jupyter notebook`

### Images Not Displaying

**Issue**: Images not showing in text-to-image notebook

**Solution**:
1. Confirm Pillow is installed: `pip install pillow`
2. Check AOG service response format
3. Verify base64 decoding has no errors

## 📝 More Information

- [AOG Project Home](../../README.md)
- [AOG API Documentation](../../docs/)
- For issues, please submit an [Issue](https://github.com/your-repo/issues)

## 🤝 Contributing

Contributions of new example scenarios are welcome! Please refer to existing notebook structure and ensure:
- Code is self-contained with no external dependencies
- Includes detailed comments
- Provides clear usage instructions
- Follows simple directory structure

---

**Enjoy!** 🎉
