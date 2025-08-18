//*****************************************************************************
// Copyright 2024-2025 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//*****************************************************************************

const express = require('express');
const bodyParser = require('body-parser');
const cors = require('cors');
const http = require('http');
const WebSocket = require('ws');
const multer = require('multer');
const path = require('path');
const fs = require('fs');

// 导入服务模块
const chatService = require('./services/chatService');
const imageService = require('./services/imageService');
const speechService = require('./services/speechService');
const cameraService = require('./services/cameraService');

const app = express();
const server = http.createServer(app);
const wss = new WebSocket.Server({ server });

const port = 3000;

// 中间件配置
app.use(cors());
app.use(bodyParser.json({ limit: '50mb' }));
app.use(bodyParser.urlencoded({ extended: true, limit: '50mb' }));
app.use(express.static('public'));

// 静态文件服务
app.use('/uploads', express.static(path.join(__dirname, 'uploads')));

// 确保上传目录存在
const uploadDirs = ['server/uploads/images', 'server/uploads/audio'];
uploadDirs.forEach(dir => {
    if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
    }
});

// 初始化AOG
const aogchecker = require('aog-checker');
aogchecker.AOGInit();

// 文件上传配置
const storage = multer.diskStorage({
    destination: function (req, file, cb) {
        const uploadPath = 'server/uploads/images';
        cb(null, uploadPath);
    },
    filename: function (req, file, cb) {
        const uniqueSuffix = Date.now() + '-' + Math.round(Math.random() * 1E9);
        cb(null, file.fieldname + '-' + uniqueSuffix + path.extname(file.originalname));
    }
});

const upload = multer({ 
    storage: storage,
    limits: {
        fileSize: 10 * 1024 * 1024 // 10MB限制
    }
});

// API路由

// Chat服务
app.post('/api/chat', async (req, res) => {
    try {
        console.log('[Chat API] Received request:', req.body);
        const result = await chatService.chat(req.body);
        if (req.body.stream) {
            // 流式响应
            res.writeHead(200, {
                'Content-Type': 'text/plain; charset=utf-8',
                'Transfer-Encoding': 'chunked',
                'Cache-Control': 'no-cache',
                'Connection': 'keep-alive'
            });
            
            result.on('data', (chunk) => {
                res.write(chunk);
            });
            
            result.on('end', () => {
                res.end();
            });
            
            result.on('error', (error) => {
                console.error('Chat stream error:', error);
                res.end();
            });
        } else {
            res.json(result);
        }
    } catch (error) {
        console.error('Chat API error:', error);
        res.status(500).json({ error: 'Chat service failed', details: error.message });
    }
});

app.post('/api/chat/not_stream', async (req, res) => {
    try {
        console.log('[Chat API] Received request:', req.body);
        const result = await chatService.chat(req.body);
        console.log('[Chat API] Non-stream response:', result);
        res.json(result);
    } catch (error) {
        console.error('Chat API error:', error);
        res.status(500).json({ error: 'Chat service failed', details: error.message });
    }
});

app.post('/api/generate', async (req, res) => {
    try {
        console.log('[Generate API] Recieved request', req.body)
        const result = await chatService.generate(req.body)
        console.log('[Generate API] Non-stream response:', result);
        res.json(result);
    } catch (error) {
        console.error('Chat API error:', error);
        res.status(500).json({ error: 'Chat service failed', details: error.message });
    }
})

// 文生图服务 - 本地
app.post('/api/text-to-image/local', async (req, res) => {
    try {
        const result = await imageService.generateImageLocal(req.body);
        res.json(result);
    } catch (error) {
        console.error('Local text-to-image error:', error);
        res.status(500).json({ error: 'Local image generation failed', details: error.message });
    }
});

// 图生图服务 - 云端
app.post('/api/text-to-image/cloud', async (req, res) => {
    try {
        const imageName = req.body.image;
        const imagePath = path.join(__dirname, 'uploads', 'images', imageName);
        req.body.image = imagePath; // 替换为本地路径
        const result = await imageService.generateImageCloud(req.body);
        res.json(result);
    } catch (error) {
        console.error('Cloud text-to-image error:', error);
        res.status(500).json({ error: 'Cloud image generation failed', details: error.message });
    }
});

// 文本转语音服务
app.post('/api/text-to-speech', async (req, res) => {
    try {
        const result = await speechService.textToSpeech(req.body);
        res.json(result);
    } catch (error) {
        console.error('Text-to-speech error:', error);
        res.status(500).json({ error: 'Text-to-speech failed', details: error.message });
    }
});

// 摄像头图片上传
const cameraUpload = multer({
    storage: multer.diskStorage({
        destination: function (req, file, cb) {
            cb(null, path.join(__dirname, 'uploads/images'));
        },
        filename: function (req, file, cb) {
            const uniqueSuffix = Date.now() + '-' + Math.round(Math.random() * 1E9);
            cb(null, file.fieldname + '-' + uniqueSuffix + path.extname(file.originalname));
        }
    }),
    limits: { fileSize: 10 * 1024 * 1024 }
});
app.post('/api/upload/camera', cameraUpload.single('image'), async (req, res) => {
    try {
        const result = await cameraService.processCameraImage(req.file);
        res.json(result);
    } catch (error) {
        console.error('Camera upload error:', error);
        res.status(500).json({ error: 'Camera image upload failed', details: error.message });
    }
});

// 服务状态监控
app.get('/api/status', async (req, res) => {
    try {
        // 这里可以调用AOG的健康检查接口
        res.json({
            status: 'healthy',
            services: {
                chat: 'available',
                'text-to-image': 'available',
                'speech-to-text': 'available',
                'text-to-speech': 'available'
            },
            timestamp: new Date().toISOString()
        });
    } catch (error) {
        console.error('Status check error:', error);
        res.status(500).json({ error: 'Status check failed' });
    }
});

// 获取可用模型
app.get('/api/models', async (req, res) => {
    try {
        // 这里可以调用AOG的模型列表接口
        res.json({
            'chat': ['deepseek-r1:7b', 'qwen2.5:0.5b'],
            'text-to-image': {
                'local': ['OpenVINO/stable-diffusion-v1-5-fp16-ov'],
                'remote': ['wanx2.1-t2i-turbo', 'wanx2.1-t2i-plus']
            },
            'speech-to-text': ['NamoLi/whisper-large-v3-ov'],
            'text-to-speech': ['NamoLi/speecht5-tts']
        });
    } catch (error) {
        console.error('Models API error:', error);
        res.status(500).json({ error: 'Failed to get models' });
    }
});

// WebSocket处理语音识别
wss.on('connection', (ws) => {
    console.log('WebSocket client connected');
    
    ws.on('message', async (message) => {
        try {
            await speechService.handleWebSocketMessage(ws, message);
        } catch (error) {
            console.error('WebSocket message error:', error);
            ws.send(JSON.stringify({ 
                type: 'error', 
                message: 'Speech recognition failed',
                details: error.message 
            }));
        }
    });
    
    ws.on('close', () => {
        console.log('WebSocket client disconnected');
        speechService.cleanup(ws);
    });
    
    ws.on('error', (error) => {
        console.error('WebSocket error:', error);
    });
});


// 文本转语音服务
app.post('/api/text-to-speech', async (req, res) => {
    try {
        const { text, voice = 'male', model = 'NamoLi/speecht5-tts' } = req.body;
        if (!text || typeof text !== 'string' || !text.trim()) {
            return res.status(400).json({ success: false, message: 'Text is required' });
        }
        // 只支持英文
        // 可加英文校验

        // 调用 speechService
        const result = await speechService.textToSpeech({ text, voice, model });
        // result: { success, audio_url, metadata }
        if (result && result.success) {
            res.json({
                business_code: 200,
                message: 'success',
                data: {
                    data: {
                        url: result.audio_url
                    }
                }
            });
        } else {
            res.status(500).json({ business_code: 500, message: 'TTS failed', data: {} });
        }
    } catch (error) {
        console.error('Text-to-speech error:', error);
        res.status(500).json({ business_code: 500, message: error.message, data: {} });
    }
});

// 启动服务器
server.listen(port, () => {
    console.log(`AOG Content Creator Web Demo running at http://localhost:${port}`);
    console.log('Make sure AOG server is running on localhost:16688');
});

// 优雅关闭
process.on('SIGINT', () => {
    console.log('\nShutting down server...');
    server.close(() => {
        console.log('Server closed');
        process.exit(0);
    });
});
