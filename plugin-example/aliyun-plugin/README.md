# Aliyun Plugin for AOG

一个外置的 Aliyun 插件，用于在 AOG 中集成阿里云的通义千问大模型服务，支持对话、文本嵌入、文生图、图生图、语音识别和语音合成等功能。

## 📖 概述

这个插件提供了与阿里云通义千问大模型服务的完整集成，支持以下功能：

- **对话服务**：支持流式对话，兼容 OpenAI 格式的 API 调用
- **文本嵌入**：支持多种嵌入模型，用于文本向量化
- **文生图**：支持通过文本生成高质量图片
- **图生图**：支持基于参考图片生成新图片
- **语音识别**：支持实时语音识别（支持 WebSocket 协议）
- **语音合成**：支持将文本转换为自然语音

## 🔑 先决条件

1. 阿里云账号及 API 密钥
2. 已开通相关服务（如 DashScope、智能语音交互等）
3. 安装 Go 1.18+ 环境

## 🧪 测试说明

### 测试环境要求

1. **AOG 服务依赖**：插件测试必须依赖 AOG 服务，请先启动 AOG 服务：
   ```bash
   aog server start
   ```

2. **认证信息**：
   - 插件本身不存储任何认证信息
   - 所有认证信息（如 API Key）需要在 AOG 服务中配置
   - 插件仅负责接收并使用 AOG 服务传递的认证信息进行认证操作

## 🚀 快速开始

### 1. 获取插件

```bash
git clone <repository-url>
cd aliyun-plugin
```

### 2. 安装依赖

```bash
go mod tidy
```

### 3. 配置插件

创建配置文件 `plugin.yaml`：

```yaml
# 阿里云 API 认证
api_key: "your-dashscope-api-key"

# 语音服务认证 (如使用语音功能)
access_key_id: "your-access-key-id"
access_key_secret: "your-access-key-secret"
app_key: "your-app-key"
```

### 4. 编译插件

```bash
# 编译当前平台
go build -o bin/aliyun-plugin .

# 交叉编译所有平台
./build-all.sh
```

### 5. 部署插件

```bash
# 复制到插件目录
mkdir -p ~/.config/aog/plugins/aliyun-plugin
cp -r . ~/.config/aog/plugins/aliyun-plugin

# 或创建符号链接
ln -s $(pwd) ~/.config/aog/plugins/aliyun-plugin
```

## 🔌 API 使用示例

### 对话服务

```bash
curl -X POST http://localhost:16688/v0.2/services/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-aog-token" \
  -d '{
    "model": "qwen-max",
    "messages": [
      {"role": "system", "content": "你是一个有用的助手"},
      {"role": "user", "content": "你好，请介绍一下你自己"}
    ],
    "stream": true
  }'
```

### 文本嵌入

```bash
curl -X POST http://localhost:16688/v0.2/services/embed \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-aog-token" \
  -d '{
    "model": "text-embedding-v1",
    "input": "这是一个测试文本"
  }'
```

### 文生图

```bash
curl -X POST http://localhost:16688/v0.2/services/text-to-image \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-aog-token" \
  -d '{
    "model": "wanx2.1-t2i-turbo",
    "prompt": "一只可爱的小猫在花园里玩耍",
    "n": 1,
    "size": "1024x1024"
  }'
```

## 🛠️ 支持的服务

### 1. 对话服务 (Chat)
- **服务名**: `chat`
- **任务类型**: `text-generation`
- **支持的模型**:
  - `qwen-max`: 通义千问 Max 版本
  - `qwen-plus`: 通义千问 Plus 版本
- **功能特性**:
  - 支持流式响应
  - 支持多轮对话
  - 支持系统提示词

### 2. 文本嵌入 (Embedding)
- **服务名**: `embed`
- **任务类型**: `embedding`
- **支持的模型**:
  - `text-embedding-v1`: 基础版文本嵌入
  - `text-embedding-v2`: 增强版文本嵌入
  - `text-embedding-v3`: 最新版文本嵌入

### 3. 文生图 (Text-to-Image)
- **服务名**: `text-to-image`
- **任务类型**: `text-to-image`
- **支持的模型**:
  - `wanx2.1-t2i-turbo`: 通义万相文生图模型

### 4. 图生图 (Image-to-Image)
- **服务名**: `image-to-image`
- **任务类型**: `image-to-image`
- **支持的模型**:
  - `wanx2.1-imageedit`: 通义万相图像编辑模型

### 5. 语音识别 (Speech-to-Text)
- **服务名**: `speech-to-text` (HTTP) / `speech-to-text-ws` (WebSocket)
- **任务类型**: `speech-to-text`
- **支持的模型**:
  - `paraformer-realtime-v2`: 实时语音识别 V2 版本
  - `paraformer-realtime-v1`: 实时语音识别 V1 版本
  - `paraformer-realtime-8k-v2`: 8K 采样率实时语音识别

### 6. 语音合成 (Text-to-Speech)
- **服务名**: `text-to-speech`
- **任务类型**: `text-to-speech`
- **支持的模型**:
  - `qwen-tts`: 通义千问语音合成

## 📁 文件结构

```
aliyun-plugin/
├── plugin.yaml       # 插件元数据
├── main.go          # 主程序入口
├── provider.go      # Provider 接口实现
├── config.go        # 配置管理
├── client.go        # Aliyun HTTP 客户端
├── README.md        # 本文档
└── go.mod           # Go 模块定义
```

## 🔧 高级功能

### 1. 流式响应

对话服务支持流式响应，可以通过设置 `stream: true` 启用：

```javascript 
const response = await fetch('http://localhost:16688/v0.2/services/chat', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    model: 'qwen-max',
    messages: [{role: 'user', content: '你好，请介绍一下你自己'}],
    stream: true,
  }),
});

const reader = response.body.getReader();
while (true) {
  const {value, done} = await reader.read();
  if (done) break;
  console.log(new TextDecoder().decode(value));
}
```

### 2. 语音识别 WebSocket 接口

```javascript
const WebSocket = require('ws');
const fs = require('fs');

const ws = new WebSocket('ws://localhost:16688/v0.2/services/speech-to-text-ws');

ws.on('open', function open() {
  // 发送配置
  ws.send(JSON.stringify({
    model: 'paraformer-realtime-v2',
    format: 'pcm',
    sample_rate: 16000,
    enable_punctuation_prediction: true,
    enable_inverse_text_normalization: true,
  }));
  
  // 发送音频数据
  const audioData = fs.readFileSync('test.pcm');
  ws.send(audioData);
  
  // 发送结束标记
  ws.send(new Uint8Array([0x00, 0x00, 0x00, 0x00]));
});

ws.on('message', function incoming(data) {
  console.log('Received:', data.toString());
});
```

## 📊 性能调优

### 1. 批处理请求

对于批量处理任务，可以使用批处理功能提高效率：

```bash
# 批量生成文本嵌入
curl -X POST http://localhost:16688/aog/v0.2/service/embed \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-aog-token" \
  -d '{
    "model": "text-embedding-v1",
    "input": ["文本1", "文本2", "文本3"]
  }'
```

## 💡 使用场景

### 适合使用外置插件的情况

1. ✅ Aliyun 已经安装在系统中
2. ✅ 需要自定义 Aliyun 配置
3. ✅ 需要使用远程 Aliyun 服务
4. ✅ 需要独立管理 Aliyun 版本

### 推荐使用内置引擎的情况

1. ✅ 首次使用 Aliyun
2. ✅ 需要自动安装和管理
3. ✅ 希望 AOG 完全托管 Aliyun


## 🚨 常见问题

### 1. 插件启动失败

**问题描述**：插件启动时报错或立即退出

**解决方案**：
- 检查配置文件格式是否正确
- 确认 API 密钥和访问凭证有效
- 查看日志文件获取详细错误信息

### 2. 请求超时

**问题描述**：请求长时间无响应或超时

**解决方案**：
- 检查网络连接
- 增加超时设置
- 检查阿里云服务状态

### 3. 认证失败

**问题描述**：返回 401 未授权错误

**解决方案**：
- 确认 API 密钥和访问凭证正确
- 检查 IAM 权限设置
- 确认服务已开通并激活

### 4. 模型不可用

**问题描述**：返回模型不存在或未授权错误

**解决方案**：
- 确认模型名称拼写正确
- 检查阿里云账号是否有权限使用该模型
- 确认模型所在区域与 API 访问区域一致

### 5. 语音识别准确率低

**问题描述**：语音识别结果不准确

**解决方案**：
- 确保音频质量良好，无背景噪音
- 检查采样率设置是否与音频文件匹配
- 尝试使用不同的语音识别模型

## 🆚 与其他插件的比较

| 功能特性 | Aliyun Plugin | HTTP API Plugin | OpenAI Plugin |
|---------|---------------|----------------|---------------|
| **支持的模型** | 通义千问系列 | 任意 HTTP API | OpenAI 系列 |
| **功能范围** | 多模态（文本、语音、图像） | 仅限文本 | 文本、图像 |
| **部署方式** | 自托管 | 自托管 | 云服务 |
| **计费方式** | 按阿里云计费 | 按后端服务计费 | 按 Token 计费 |
| **延迟** | 低（国内） | 依赖后端 | 中高（国际） |
| **数据隐私** | 数据不离开阿里云 | 依赖后端 | 数据发送至 OpenAI |
| **定制能力** | 高 | 中 | 低 |

## 📚 参考资料

### 官方文档
- [阿里云 DashScope 文档](https://help.aliyun.com/zh/dashscope/)
- [通义千问 API 文档](https://help.aliyun.com/zh/dashscope/developer-reference/tongyi-thousand-questions-api-documentation)
- [通义万相 API 文档](https://help.aliyun.com/zh/dashscope/developer-reference/tongyi-wanxiang-api-documentation)
- [智能语音交互文档](https://help.aliyun.com/zh/nls/developer-reference)

### 开发资源
- [AOG 插件开发指南](../../docs/zh-cn/source/aog插件开发指南.rst)

### 社区支持
- [阿里云开发者社区](https://developer.aliyun.com/)
- [GitHub Issues](https://github.com/your-org/aliyun-plugin/issues)
- [Discord 社区](https://discord.gg/your-community)

## 🤝 贡献指南

我们欢迎各种形式的贡献，包括但不限于：

- 提交 Bug 报告和功能请求
- 提交 Pull Request
- 改进文档
- 分享使用经验

### 开发流程

1. Fork 仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

### 代码规范

- 遵循 Go 代码规范
- 提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/)
- 确保所有测试通过
- 更新相关文档

## 📄 许可证

本项目采用 [Apache License 2.0](LICENSE) 开源协议。

## 🙏 致谢

- 感谢 [阿里云](https://www.aliyun.com/) 提供的 AI 能力
- 感谢所有贡献者的辛勤付出

