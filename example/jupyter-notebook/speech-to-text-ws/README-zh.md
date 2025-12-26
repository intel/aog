# Speech-to-Text-WS（实时语音识别）

## 场景描述

实时语音识别服务基于 WebSocket 协议，允许您流式提交语音数据并实时获取识别结果。与传统的批量语音识别不同，WebSocket 方式支持边说边识别，适用于实时字幕、语音助手、会议记录等需要即时反馈的场景。

## 学习目标

通过本示例，您将学习：

- 如何使用 WebSocket 连接 AOG 的实时语音识别服务
- 如何将音频文件转换为 PCM 格式并流式发送
- 如何处理实时识别结果
- 如何使用语音活动检测（VAD）功能
- WebSocket 通信的"指令-事件"模式

## WebSocket 端点

```
ws://localhost:16688/aog/v0.2/services/speech-to-text-ws
```

## 通信流程

1. 客户端连接 WebSocket 服务端
2. 客户端发送 `run-task` 指令启动任务
3. 服务端返回 `task-started` 事件（包含 task_id）
4. 客户端发送 PCM 音频数据（二进制）
5. 服务端返回 `result-generated` 事件（实时识别结果）
6. 客户端发送 `finish-task` 指令结束任务
7. 服务端返回 `task-finished` 事件

## 主要参数

### run-task 指令参数

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `task` | string | 是 | 固定为 `speech-to-text-ws` |
| `action` | string | 是 | 固定为 `run-task` |
| `model` | string | 是 | 模型名称，如 `NamoLi/whisper-large-v3-ov` |
| `parameters.format` | string | 可选 | 音频格式，**仅支持 `pcm`** |
| `parameters.sample_rate` | integer | 可选 | 采样率，仅支持 `16000` |
| `parameters.language` | string | 可选 | 语言代码，如 `zh`、`en` |
| `parameters.use_vad` | boolean | 可选 | 是否使用语音活动检测，默认 `true` |
| `parameters.return_format` | string | 可选 | 返回格式，默认 `text` |

### finish-task 指令参数

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `task` | string | 是 | 固定为 `speech-to-text-ws` |
| `action` | string | 是 | 固定为 `finish-task` |
| `task_id` | string | 是 | 服务端返回的任务 ID |
| `model` | string | 是 | 使用的模型名称 |

## 先决条件

在运行本示例之前，请确保：

- [ ] **AOG 服务已安装并运行**
  ```bash
  # 检查 AOG 服务状态
  curl http://localhost:16688/aog/v0.2/health
  ```

- [ ] **已安装语音识别模型**
  - 推荐模型：`NamoLi/whisper-large-v3-ov`
  - 使用 AOG 的模型管理功能下载和安装模型

- [ ] **已安装 Python 依赖**
  ```bash
  pip install -r ../requirements.txt
  ```
  
  主要依赖包括：
  - `websocket-client`：WebSocket 客户端
  - `pydub`：音频处理
  - `soundfile`：音频文件读写

- [ ] **准备测试音频**
  - 准备一个或多个音频文件（支持 WAV、MP3 等格式）
  - 音频将自动转换为 PCM 格式

## 快速开始

1. 启动 Jupyter Notebook：
   ```bash
   jupyter notebook
   ```

2. 打开 `speech-to-text-ws.ipynb`

3. 按顺序运行单元格

4. 查看实时识别结果

## 使用技巧

### 音频格式要求

- **服务端要求**：仅接受 PCM 格式的音频数据
- **采样率**：16000 Hz
- **声道**：单声道（Mono）
- **位深度**：16-bit

### 音频转换

Notebook 中提供了自动转换功能，支持：
- WAV 文件自动转换为 PCM
- MP3 文件自动转换为 PCM
- 其他格式通过 `pydub` 转换

### 语音活动检测（VAD）

启用 VAD 可以：
- 自动检测语音段落
- 过滤静音部分
- 提高识别准确率
- 减少不必要的处理

### 实时性优化

1. **分块发送**：将音频分成小块（如 1024 字节）流式发送
2. **缓冲控制**：合理设置发送间隔，避免过快或过慢
3. **网络优化**：确保网络连接稳定

## 事件类型说明

### task-started 事件
任务启动成功，返回 task_id 用于后续通信。

### result-generated 事件
实时识别结果，包含：
- `begin_time`：开始时间（毫秒）
- `end_time`：结束时间（毫秒，可能为 null）
- `text`：识别的文本内容

### task-finished 事件
任务正常结束。

### task-failed 事件
任务失败，包含错误代码和错误信息：
- `CLIENT_ERROR`：客户端错误
- `SERVER_ERROR`：服务器错误
- `MODEL_ERROR`：模型处理错误

## 常见问题

### Q: 为什么必须使用 PCM 格式？
A: AOG 服务器的实时语音识别服务仅接受 PCM 格式的音频数据。这是为了确保处理效率和一致性。客户端可以输入 WAV 或 MP3 文件，但必须在发送前自动转换为 PCM 格式。

### Q: 如何处理长音频？
A: 将音频分成小块（如 1024 字节）流式发送，服务端会实时返回识别结果。

### Q: 识别结果是最终结果吗？
A: 实时识别会返回中间结果和最终结果。中间结果的 `end_time` 可能为 null，最终结果会包含完整的时间信息。

### Q: 如何提高识别准确率？
A: 
- 使用高质量的音频（清晰、无噪音）
- 启用 VAD 功能
- 指定正确的语言代码
- 确保音频格式符合要求（PCM, 16000Hz, Mono, 16-bit）

### Q: WebSocket 连接断开怎么办？
A: 
- 实现重连机制
- 保存已识别的结果
- 从断点处继续发送音频

### Q: 支持哪些语言？
A: 取决于使用的模型，Whisper 模型通常支持多种语言，包括中文、英文等。

### Q: 如何调试 WebSocket 通信？
A: 
- 打印发送和接收的消息
- 检查事件类型和 task_id
- 查看错误代码和错误信息
- 使用 WebSocket 调试工具

## 相关资源

- [AOG 文档](../../../docs/zh-cn/)
- [Speech 服务规范](../../../docs/zh-cn/source/service_specs/speech.rst)
- [Speech-to-Text 服务](../speech-to-text/)（批量识别）

## 下一步

- 尝试 [Speech-to-Text](../speech-to-text/) 服务，进行批量语音识别
- 探索 [Text-to-Speech](../text-to-speech/) 服务，将文本转换为语音
- 查看其他 [AOG 服务示例](../)
