# AOG智能内容创作助手 Web Demo

这是一个展示AOG多模态AI服务协同能力的Web演示应用，通过语音输入到语音输出的完整流程，展示AOG"一套API，多种AI能力"的核心价值。

## 功能特性

- 🎤 **实时语音输入**: WebSocket实时语音识别
- 💬 **智能文案生成**: 基于语音输入生成创意文案
- 📷 **摄像头拍摄**: 支持拍摄参考图片
- 🖼️ **本地+云端协同文生图**: 本地快速垫图 + 云端精细化生成
- 🎙️ **语音优化指令**: 通过语音描述图片优化方向
- 🔊 **语音播报**: 将文案转换为语音输出
- 📊 **服务状态监控**: 实时显示AOG服务调用状态

## 系统要求

- Windows 10 64位及以上版本 / macOS 14及以上版本
- Node.js 18.x 及以上版本
- 现代浏览器 (支持WebRTC、WebSocket)
- 摄像头和麦克风权限

## 快速启动

### 自动启动 (推荐)

**Linux/macOS:**
```bash
cd example/web
./start.sh
```

**Windows:**
```cmd
cd example\web
start.bat
```

启动脚本会自动检查系统环境、AOG服务状态、安装依赖并启动Web服务。

### 手动启动

1. **确保AOG服务已启动**
```bash
aog server start
```

2. **检查AOG服务状态**
```bash
curl http://localhost:16688/health
```

3. **安装依赖**
aog 组件
```bash
cd ./sdk/node-lib
npm pack

cd ./checker/node-lib
npm pack
```

NodeJS依赖
```bash
cd example/full-process
npm install
```

4. **启动Web服务**
```bash
npm start
```

5. **打开浏览器访问** http://localhost:3000

## 前置条件

### 系统要求
- **操作系统**: Windows 10 64位+ / macOS 14+ / Linux
- **Node.js**: 18.x 或更高版本
- **浏览器**: Chrome 90+, Firefox 88+, Safari 14+, Edge 90+
- **硬件**: 摄像头和麦克风 (用于完整功能体验)

### AOG服务配置
确保以下AOG服务已正确配置并运行：

1. **Chat服务** (本地ollama)
   - 模型: qwen2.5:0.5b, deepseek-r1:7b 等

2. **Text-to-Image服务**
   - 本地: OpenVINO/LCM_Dreamshaper_v7-fp16-ov
   - 远程: 阿里云 wanx2.0-t2i-turbo

3. **Image-to-Image服务** (远程阿里云)
   - 模型: wanx2.1-imageedit

4. **Speech-to-Text服务** (本地OpenVINO)
   - 模型: NamoLi/whisper-large-v3-ov

5. **Text-to-Speech服务**
   - 远程：阿里云 qwen-tts
   - 本地: NamoLi/speecht5-tts

6. **Generate服务** (本地ollama)
   - 模型: gemma3:4b

### 权限设置
首次访问时，浏览器会请求以下权限：
- **麦克风权限**: 用于语音输入和优化指令
- **摄像头权限**: 用于拍摄参考图片 (可选)

## 技术架构

- **后端**: Node.js + Express + AOG-lib
- **前端**: HTML5 + CSS3 + JavaScript ES6+
- **媒体API**: WebRTC + MediaDevices API + Web Audio API
- **实时通信**: WebSocket
- **AOG集成**: aog-lib + aog-checker

## 详细使用流程

### 步骤1: 语音输入 🎤
1. 点击页面左侧的**蓝色麦克风按钮**
2. 允许浏览器访问麦克风权限
3. 清晰地说出你的创作需求，例如：
   - "我想要一个关于春天花园的营销文案"
   - "帮我写一段介绍智能手机的产品描述"
   - "创作一个温馨咖啡店的宣传内容"
4. 说完后再次点击按钮停止录音
5. 系统会实时显示语音识别结果

### 步骤2: AI文案生成 💬
1. 语音识别完成后，AI会自动开始生成文案
2. 可以在右侧聊天区域看到流式生成的文案内容
3. 可以通过下拉菜单切换不同的AI模型
4. 生成完成后会自动进入下一步

### 步骤3: 拍摄参考图片 📷 (可选)
1. 点击**"启动摄像头"**按钮
2. 允许浏览器访问摄像头权限
3. 调整拍摄角度，点击**"拍摄照片"**
4. 预览拍摄结果，可以选择**"重新拍摄"**或**"使用此图片"**
5. 拍摄的图片可以作为后续图片生成的参考

### 步骤4: 本地垫图生成 🖼️
1. 系统会自动从文案中提取关键词
2. 使用本地OpenVINO服务快速生成4张512x512的预览图片
3. 生成过程通常在几秒内完成
4. 可以点击图片进行选择和预览
5. 右侧会显示生成时间和服务提供商信息

### 步骤5: 语音优化指令 🎙️
1. 本地垫图完成后，**"语音描述优化方向"**按钮会激活
2. 点击按钮开始录音，描述你希望的图片优化方向，例如：
   - "让图片更加明亮温暖"
   - "添加更多的花朵元素"
   - "变成卡通风格"
   - "增加科技感"
3. 系统会实时显示优化指令识别结果

### 步骤6: 云端精细化生成 🎨
1. 接收到优化指令后，系统会自动开始云端图片生成
2. 使用阿里云服务生成1张1024*1024的高质量图片
3. 会将原始关键词和优化指令合并作为新的prompt
4. 生成时间可能需要10-30秒，请耐心等待
5. 生成完成后可以对比本地和云端图片的效果差异

### 步骤7: 语音播报生成 🔊
1. 图片生成完成后，**"生成语音播报"**按钮会激活
2. 点击按钮开始将文案转换为语音
3. 使用本地OpenVINO TTS服务进行语音合成
4. 生成完成后会显示音频播放器
5. 可以播放、暂停和调节音量

## 功能特色

### 🔄 本地+云端协同
- **本地服务**: 快速响应，低延迟，适合预览和实时交互
- **云端服务**: 高质量输出，专业效果，适合最终成品
- **智能调度**: 系统自动选择最优的服务提供商

### ⚡ 实时交互体验
- **流式文案生成**: 实时显示AI生成过程
- **实时语音识别**: WebSocket连接，低延迟语音转文字
- **即时状态反馈**: 每个步骤都有清晰的状态指示

### 📊 性能监控
- **服务状态监控**: 实时显示各AI服务的健康状态
- **响应时间统计**: 记录每个服务的响应时间
- **本地/云端比例**: 显示服务调用的分布情况

### 🎯 用户友好设计
- **步骤指示器**: 清晰显示当前进度和完成状态
- **错误处理**: 友好的错误提示和重试机制
- **响应式设计**: 支持桌面和移动设备访问

## AOG服务依赖

- Chat服务 (本地ollama)
- Text-to-Image服务 (本地OpenVINO + 远程阿里云)
- Speech-to-Text服务 (本地OpenVINO)
- Text-to-Speech服务 (本地OpenVINO)

## 故障排除

### 常见问题

**1. 麦克风无法使用**
- 检查浏览器是否已授予麦克风权限
- 确保麦克风设备正常工作
- 尝试刷新页面重新授权
- 检查系统音频设置

**2. 摄像头无法启动**
- 检查浏览器是否已授予摄像头权限
- 确保摄像头设备未被其他应用占用
- 尝试关闭其他使用摄像头的应用
- 检查系统隐私设置

**3. AOG服务连接失败**
- 确保AOG服务正在运行: `aog server start`
- 检查服务端口是否为16688: `curl http://localhost:16688/health`
- 查看AOG服务日志排查问题
- 确保防火墙未阻止端口访问

**4. 图片生成失败**
- 检查本地OpenVINO服务是否正常
- 确保阿里云API密钥配置正确
- 检查网络连接是否稳定
- 查看浏览器控制台错误信息

**5. 语音识别不准确**
- 确保环境安静，减少背景噪音
- 说话清晰，语速适中
- 检查麦克风音量设置
- 尝试更换麦克风设备

**6. 页面加载缓慢**
- 检查网络连接速度
- 清除浏览器缓存
- 确保服务器资源充足
- 检查是否有大文件下载

### 调试模式

启用详细日志输出：
```bash
DEBUG=* npm start
```

查看浏览器控制台：
- 按F12打开开发者工具
- 查看Console标签页的日志信息
- 检查Network标签页的网络请求

### 性能优化建议

1. **本地服务优化**
   - 确保有足够的GPU内存用于模型推理
   - 关闭不必要的后台应用释放资源
   - 使用SSD存储提升文件读写速度

2. **网络优化**
   - 使用稳定的网络连接
   - 配置CDN加速静态资源加载
   - 启用gzip压缩减少传输大小

3. **浏览器优化**
   - 使用最新版本的现代浏览器
   - 关闭不必要的浏览器扩展
   - 增加浏览器内存限制

## 开发说明

### 项目结构
```
example/web/
├── README.md                    # 项目说明文档
├── package.json                 # 项目依赖配置
├── .aog                        # AOG服务配置
├── start.sh / start.bat        # 启动脚本
├── server/                     # 后端服务
│   ├── server.js              # Express服务器主文件
│   ├── services/              # AOG服务封装层
│   │   ├── chatService.js     # Chat服务
│   │   ├── imageService.js    # 图片生成服务
│   │   ├── speechService.js   # 语音服务
│   │   └── cameraService.js   # 摄像头服务
│   ├── utils/                 # 工具函数
│   │   ├── fileManager.js     # 文件管理
│   │   └── promptExtractor.js # 关键词提取
│   └── uploads/               # 文件存储目录
│       ├── images/            # 生成的图片
│       ├── audio/             # 音频文件
│       └── camera/            # 摄像头照片
└── public/                    # 前端资源
    ├── index.html             # 主页面
    ├── css/
    │   └── style.css          # 样式文件
    ├── js/                    # JavaScript模块
    │   ├── main.js            # 主应用逻辑
    │   ├── audioRecorder.js   # 音频录制模块
    │   ├── cameraCapture.js   # 摄像头拍摄模块
    │   ├── streamHandler.js   # 流式响应处理
    │   └── statusMonitor.js   # 状态监控模块
    └── assets/                # 静态资源
```

### 技术架构

**后端技术栈:**
- **Node.js + Express**: Web服务器框架
- **aog-lib**: AOG服务集成库
- **WebSocket**: 实时语音识别通信
- **multer**: 文件上传处理
- **cors**: 跨域资源共享

**前端技术栈:**
- **原生HTML5/CSS3/JavaScript**: 轻量级实现
- **WebRTC**: 音频视频设备访问
- **WebSocket**: 实时通信
- **Fetch API**: HTTP请求处理
- **Canvas API**: 图片处理

**AOG服务集成:**
- **统一API调用**: 通过aog-lib统一调用各种AI服务
- **混合策略**: 支持本地和远程服务的智能调度
- **错误处理**: 完善的错误处理和重试机制
- **性能监控**: 实时监控服务状态和响应时间

### 扩展开发

**添加新的AI服务:**
1. 在`server/services/`目录下创建新的服务模块
2. 在`server.js`中添加对应的API路由
3. 在前端添加相应的UI组件和交互逻辑
4. 更新`.aog`配置文件添加服务配置

**自定义UI组件:**
1. 在`public/js/`目录下创建新的模块文件
2. 在`public/css/style.css`中添加样式定义
3. 在`main.js`中集成新组件
4. 更新`index.html`添加必要的DOM元素

**性能优化:**
1. 实现请求缓存机制
2. 添加图片压缩和优化
3. 实现懒加载和虚拟滚动
4. 优化WebSocket连接管理

### 部署说明

**开发环境:**
```bash
npm run dev  # 使用nodemon自动重启
```

**生产环境:**
```bash
npm start    # 直接启动服务
```

**Docker部署:**
```dockerfile
FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
CMD ["npm", "start"]
```

**环境变量配置:**
- `PORT`: Web服务端口 (默认: 3000)
- `AOG_SERVER_URL`: AOG服务地址 (默认: http://localhost:16688)
- `UPLOAD_MAX_SIZE`: 文件上传大小限制 (默认: 10MB)
- `DEBUG`: 调试模式开关
