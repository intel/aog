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

const AogLib = require('aog-lib');
const axios = require('axios');
const WebSocket = require('ws');
const fs = require('fs');
const path = require('path');
const constants = require('../../constants')

class SpeechService {
    constructor() {
        this.aog = new AogLib();
        this.activeConnections = new Map(); // 存储活跃的WebSocket连接
        this.serverUrl = 'http://localhost:3000';
        this.taskId = null;
        this.modelName = constants.defaultSpeechToTextModel
        // this.modelName = "NamoLi/whisper-large-v3-ov";
        // this.modelName = 'paraformer-realtime-v2';
        console.log('[SpeechService] Initialized');
    }

    /**
     * 处理WebSocket消息
     * @param {WebSocket} ws - WebSocket连接
     * @param {Buffer|string} message - 消息内容
     */
    async handleWebSocketMessage(ws, message) {
        try {
            if (typeof message === 'string' || (message instanceof Buffer && message[0] === 0x7B)) {
                const textMessage = message.toString();
                const data = JSON.parse(textMessage);
                
                await this.handleControlMessage(ws, data);
            } else {
                // 二进制音频数据
                await this.handleAudioData(ws, message);
            }
        } catch (error) {
            console.error('[SpeechService] WebSocket message error:', error);
            this.sendError(ws, 'Failed to process message', error.message);
        }
    }

    /**
     * 处理控制消息
     * @param {WebSocket} ws - WebSocket连接
     * @param {Object} data - 控制消息数据
     */
    async handleControlMessage(ws, data) {
        // 提取消息字段，与client.go保持一致
        const { task, action, model, parameters, task_id } = data;

        console.log(`[SpeechService] Received control message: ${action}`, data);

        // 根据action处理不同的控制命令
        switch (action) {
            case 'run-task':
                console.log('[SpeechService] Processing run-task command');
                await this.startSpeechRecognition(ws, { model, parameters });
                break;
                
            case 'finish-task':
                console.log('[SpeechService] Processing finish-task command');
                // 如果客户端提供了task_id，使用它更新当前的taskId
                if (task_id) {
                    this.taskId = task_id;
                    console.log(`[SpeechService] Task ID updated from client: ${task_id}`);
                }
                await this.finishSpeechRecognition(ws);
                break;
                
            default:
                console.warn(`[SpeechService] Unsupported action: ${action}`);
                this.sendError(ws, 'Unknown action', `Action ${action} is not supported`);
        }
    }

    /**
     * 开始语音识别任务
     * @param {WebSocket} ws - WebSocket连接
     * @param {Object} config - 配置参数
     */
    async startSpeechRecognition(ws, config) {
        const {
            model = this.modelName,
            parameters = {}
        } = config;

        try {
            // 创建AOG WebSocket连接
            const aogWsUrl = 'ws://localhost:16688/aog/v0.2/services/speech-to-text-ws';
            const aogWs = new WebSocket(aogWsUrl);

            // 存储连接信息
            this.activeConnections.set(ws, {
                aogWs: aogWs,
                model: model,
                parameters: parameters,
                startTime: Date.now()
            });

            aogWs.on('open', () => {
                // 发送初始化消息到AOG - 严格按照client.go的格式
                const initMessage = {
                    task: 'speech-to-text-ws',
                    action: 'run-task',
                    model: model,
                    parameters: {
                        format: 'pcm',
                        sample_rate: 16000,
                        language: 'zh',
                        use_vad: true,
                        return_format: 'text'
                    }
                };

                // 合并用户提供的参数
                if (parameters && typeof parameters === 'object') {
                    Object.assign(initMessage.parameters, parameters);
                }

                console.log('[SpeechService] Sending run-task command to AOG:', JSON.stringify(initMessage));
                aogWs.send(JSON.stringify(initMessage));
            });

            aogWs.on('message', (data) => {
                try {
                    const result = JSON.parse(data.toString());
                    console.log('[SpeechService] Received message from AOG:', JSON.stringify(result));
                    
                    if (result.header?.event === 'task-started') {
                        console.log('[SpeechService] Received AOG task-started:', result);
                        
                        // 保存task_id
                        this.taskId = result.header.task_id;
                        console.log('[SpeechService] Task ID set to:', this.taskId);
                        
                        // 转发task_id给前端 - 使用前端期望的格式
                        ws.send(JSON.stringify({
                            type: 'task-started',
                            task_id: result.header.task_id,
                            model: model,
                            parameters: parameters,
                            timestamp: new Date().toISOString()
                        }));
                    } else {
                        this.handleAogResponse(ws, result);
                    }
                } catch (error) {
                    console.error('[SpeechService] AOG response parse error:', error);
                }
            });

            aogWs.on('error', (error) => {
                console.error('[SpeechService] AOG WebSocket error:', error);
                this.sendError(ws, 'AOG connection error', error.message);
            });

            aogWs.on('close', () => {
                console.log('[SpeechService] AOG WebSocket closed');
                this.activeConnections.delete(ws);
            });

            // 发送任务开始确认给前端 - 保持与原代码一致
            ws.send(JSON.stringify({
                type: 'task-started',
                model: model,
                parameters: parameters,
                timestamp: new Date().toISOString()
            }));

        } catch (error) {
            console.error('[SpeechService] Start recognition error:', error);
            this.sendError(ws, 'Failed to start speech recognition', error.message);
        }
    }

    /**
     * 处理音频数据
     * @param {WebSocket} ws - WebSocket连接
     * @param {Buffer} audioData - 音频数据
     */
    async handleAudioData(ws, audioData) {
        console.log('[SpeechService] Received audio data, size:', audioData.length);
        const connection = this.activeConnections.get(ws);
        
        if (!connection || !connection.aogWs) {
            this.sendError(ws, 'No active recognition session', 'Please start a task first');
            return;
        }

        try {
            // 检查WebSocket连接是否打开
            if (connection.aogWs.readyState === WebSocket.OPEN) {
                // 直接转发音频数据到AOG，不做任何处理
                // 与client.go保持一致，音频数据应该是PCM格式
                console.log(`[SpeechService] Forwarding audio data to AOG, size: ${audioData.length} bytes`);
                connection.aogWs.send(audioData);
            } else {
                console.warn('[SpeechService] AOG WebSocket not ready (state:', connection.aogWs.readyState, '), dropping audio data');
            }
        } catch (error) {
            console.error('[SpeechService] Audio data forward error:', error);
            this.sendError(ws, 'Failed to process audio data', error.message);
        }
    }

    /**
     * 处理AOG响应
     * @param {WebSocket} ws - 客户端WebSocket连接
     * @param {Object} result - AOG返回的结果
     */
    handleAogResponse(ws, result) {
        try {
            const eventType = result.header?.event;
            
            // 根据事件类型处理不同的响应
            switch (eventType) {
                case 'result-generated':
                    if (result.payload?.output?.sentence) {
                        console.log('[SpeechService] Processing recognition result');
                        
                        // 保存task_id如果尚未保存
                        if (!this.taskId && result.header?.task_id) {
                            this.taskId = result.header.task_id;
                            console.log("[SpeechService] Task ID set from result:", this.taskId);
                        }
                        
                        // 处理识别结果文本
                        let text = result.payload.output.sentence.text || '';
                        // 去除字幕编号和时间戳，只保留文字
                        text = text.replace(/^\d+\s*\n[\d:,.\-–> ]+\n/, '').replace(/\n/g, '');
                        
                        // 发送给前端
                        ws.send(JSON.stringify({
                            type: 'speech-result',
                            text: text,
                            is_final: false,
                            begin_time: result.payload.output.sentence.begin_time,
                            end_time: result.payload.output.sentence.end_time,
                            timestamp: new Date().toISOString()
                        }));
                    }
                    break;
                    
                case 'task-finished':
                    console.log('[SpeechService] Task finished event received:', result.header.task_id);
                    ws.send(JSON.stringify({
                        type: 'task-finished',
                        task_id: result.header.task_id,
                        timestamp: new Date().toISOString()
                    }));
                    break;
                    
                case 'task-failed':
                    console.error('[SpeechService] Task failed:', result.header.error_message);
                    ws.send(JSON.stringify({
                        type: 'error',
                        message: 'Task failed',
                        details: result.header.error_message || 'Unknown error',
                        timestamp: new Date().toISOString()
                    }));
                    break;
                    
                default:
                    // 其他类型的消息直接转发
                    console.log(`[SpeechService] Forwarding ${eventType || 'unknown'} event to client`);
                    ws.send(JSON.stringify({
                        type: eventType || 'unknown',
                        data: result,
                        timestamp: new Date().toISOString()
                    }));
            }
        } catch (error) {
            console.error('[SpeechService] Response processing error:', error);
        }
    }

    /**
     * 结束语音识别任务
     * @param {WebSocket} ws - WebSocket连接
     */
    async finishSpeechRecognition(ws) {
        try {
            // 确保有taskId
            if (!this.taskId) {
                console.warn('[SpeechService] No task ID available for finish-task command');
            }
            
            // 严格按照client.go中的格式构建finish-task消息
            const finishTaskMsg = {
                task: "speech-to-text-ws",
                action: "finish-task",
                task_id: this.taskId || 'unknown-task',
            };
            
            console.log("[SpeechService] Sending finish-task command:", JSON.stringify(finishTaskMsg));
            
            // 获取AOG WebSocket连接
            const connection = this.activeConnections.get(ws);
            if (connection && connection.aogWs && connection.aogWs.readyState === WebSocket.OPEN) {
                // 发送finish-task命令到AOG
                connection.aogWs.send(JSON.stringify(finishTaskMsg));
                
                // 向前端发送与原代码一致的格式
                ws.send(JSON.stringify({
                    task: "speech-to-text-ws",
                    action: 'finish-task',
                    model: this.modelName,
                    task_id: this.taskId || 'unknown-task',
                }));
            } else {
                console.error('[SpeechService] Cannot send finish-task, AOG WebSocket not available');
                this.sendError(ws, 'Cannot finish task', 'AOG connection not available');
            }
        } catch (error) {
            console.error('[SpeechService] Finish recognition error:', error);
            this.sendError(ws, 'Failed to finish speech recognition', error.message);
        }
    }

    /**
     * 文本转语音
     * @param {Object} requestData - 请求数据
     * @returns {Promise<Object>} 转换结果
     */
    async textToSpeech(requestData) {
        const {
            text,
            model = 'qwen-tts',
            voice = 'Cherry',
        } = requestData;

        console.log(`[SpeechService] TTS request - Text length: ${text.length}, Model: ${model}, Voice: ${voice}`);

        const ttsData = {
            model: model,
            text: text,
            voice: voice,
        };

        try {
            const startTime = Date.now();
            const result = await this.aog.textToSpeech(ttsData);
            const responseTime = Date.now() - startTime;

            console.log(`[SpeechService] TTS completed in ${responseTime}ms`);
            console.log('[SpeechService] TTS raw result:', JSON.stringify(result, null, 2));

            if (result.code !== 200 || !result.data || !result.data.data) {
                throw new Error('Invalid response from text-to-speech service');
            }

            const audioUrl = await this.processAudioFile(result.data.data);
            console.log('[SpeechService] TTS audio URL:', audioUrl);

            return {
                success: true,
                audio_url: audioUrl,
                metadata: {
                    model: model,
                    voice: voice,
                    response_time: responseTime
                }
            };
        } catch (error) {
            console.error('[SpeechService] TTS error:', error);
            throw new Error(`Text-to-speech failed: ${error.message}`);
        }
    }

    /**
     * 处理音频文件（支持远程下载）
     * @param {string} audioPath - 音频文件路径或URL
     * @returns {Promise<string>} 处理后的音频URL
     */
    async processAudioFile(audioPath) {
        const audioDir = path.join(__dirname, '../uploads/audio');
        if (!fs.existsSync(audioDir)) {
            fs.mkdirSync(audioDir, { recursive: true });
        }

        let targetPath;
        if (audioPath.startsWith('http')) {
            // 下载远程文件
            const fileName = `tts-${Date.now()}${path.extname(audioPath.split('?')[0])}`;
            targetPath = path.join(audioDir, fileName);
            const response = await axios.get(audioPath, { responseType: 'stream' });
            const writer = fs.createWriteStream(targetPath);
            await new Promise((resolve, reject) => {
                response.data.pipe(writer);
                writer.on('finish', resolve);
                writer.on('error', reject);
            });
        } else {
            // 本地文件
            if (!fs.existsSync(audioPath)) {
                throw new Error(`Audio file not found: ${audioPath}`);
            }
            const fileName = `tts-${Date.now()}${path.extname(audioPath)}`;
            targetPath = path.join(audioDir, fileName);
            fs.renameSync(audioPath, targetPath);
        }

        const audioUrl = `${this.serverUrl}/uploads/audio/${path.basename(targetPath)}`;
        console.log(`[SpeechService] Audio file processed: ${audioUrl}`);
        return audioUrl;
    }

    /**
     * 发送错误消息
     * @param {WebSocket} ws - WebSocket连接
     * @param {string} message - 错误消息
     * @param {string} details - 错误详情
     */
    sendError(ws, message, details) {
        try {
            ws.send(JSON.stringify({
                type: 'error',
                message: message,
                details: details,
                timestamp: new Date().toISOString()
            }));
        } catch (error) {
            console.error('[SpeechService] Error sending error message:', error);
        }
    }

    /**
     * 清理WebSocket连接
     * @param {WebSocket} ws - WebSocket连接
     */
    cleanup(ws) {
        console.log('[SpeechService] cleanup called');
        const connection = this.activeConnections.get(ws);
        
        if (connection && connection.aogWs) {
            try {
                connection.aogWs.close();
            } catch (error) {
                console.error('[SpeechService] Cleanup error:', error);
            }
        }
        
        this.activeConnections.delete(ws);
        console.log('[SpeechService] Cleaned up WebSocket connection');
    }
}

module.exports = new SpeechService();
