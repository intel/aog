/**
 * AOG智能内容创作助手 - 音频录制模块
 * Copyright 2024-2025 Intel Corporation
 */

class AudioRecorder {
    constructor(model) {
        this.mediaRecorder = null;
        this.audioChunks = [];
        this.websocket = null;
        this.stream = null;
        this.isRecording = false;
        this.isConnected = false;
        
        // 音频配置
        this.audioConfig = {
            sampleRate: 16000,
            channelCount: 1,
            echoCancellation: true,
            noiseSuppression: true,
            autoGainControl: true
        };

        // 添加音频上下文用于格式转换
        this.audioContext = null;
        this.audioProcessor = null;
        
        // 音频采集缓冲
        this.audioBuffer = [];
        this.recordingInterval = 6000; // 每3秒生成一个WAV文件
        this.recordingTimer = null;

        // this.modelName = 'NamoLi/whisper-large-v3-ov';
        // this.modelName = 'paraformer-realtime-v2'
        this.modelName = model ||'paraformer-realtime-v2';
        
        // WebSocket配置
        this.wsConfig = {
            url: 'ws://localhost:3000',
            reconnectInterval: 3000,
            maxReconnectAttempts: 5
        };
        
        this.reconnectAttempts = 0;
        this.callbacks = {
            onSpeechResult: null,
            onError: null,
            onStatusChange: null
        };
        
        this.initializeElements();

        this.recognizedText = '';
        this.taskStarted = false;
        this.taskId = null;
        this.cleanup();
    }

    /**
     * 初始化DOM元素
     */
    initializeElements() {
        this.recordBtn = document.getElementById('record-btn');
        this.recordingIndicator = document.getElementById('recording-indicator');
        this.speechText = document.getElementById('speech-text');
        this.speechStatus = document.getElementById('speech-status');
        
        if (this.recordBtn) {
            this.recordBtn.addEventListener('click', () => this.toggleRecording());
        }
    }

    /**
     * 设置回调函数
     * @param {Object} callbacks - 回调函数对象
     */
    setCallbacks(callbacks) {
        this.callbacks = { ...this.callbacks, ...callbacks };
    }

    /**
     * 初始化麦克风
     */
    async initMicrophone() {
        try {
            console.log('[AudioRecorder] Initializing microphone...');
            
            this.stream = await navigator.mediaDevices.getUserMedia({
                audio: this.audioConfig
            });
            
            // 初始化AudioContext
            this.audioContext = new (window.AudioContext || window.webkitAudioContext)({
                sampleRate: 16000
            });
            
            console.log('[AudioRecorder] Microphone initialized successfully');
            this.updateStatus('麦克风已就绪');
            return true;
        } catch (error) {
            console.error('[AudioRecorder] Microphone initialization failed:', error);
            this.updateStatus('麦克风初始化失败', 'error');
            this.handleError('无法访问麦克风，请检查权限设置');
            return false;
        }
    }

    /**
     * 连接WebSocket
     */
    async connectWebSocket() {
        try {
            console.log('[AudioRecorder] Connecting to WebSocket...');
            
            this.websocket = new WebSocket(this.wsConfig.url);
            
            this.websocket.onopen = () => {
                console.log('[AudioRecorder] WebSocket connected');
                this.isConnected = true;
                this.reconnectAttempts = 0;
                this.updateStatus('语音识别服务已连接');
            };
            
            this.websocket.onmessage = (event) => {
                this.handleWebSocketMessage(event);
            };
            
            this.websocket.onclose = () => {
                console.log('[AudioRecorder] WebSocket disconnected');
                this.isConnected = false;
                this.updateStatus('语音识别服务已断开', 'error');
                this.attemptReconnect();
            };
            
            this.websocket.onerror = (error) => {
                console.error('[AudioRecorder] WebSocket error:', error);
                this.handleError('语音识别服务连接失败');
            };
            
        } catch (error) {
            console.error('[AudioRecorder] WebSocket connection failed:', error);
            this.handleError('无法连接到语音识别服务');
        }
    }

    /**
     * 处理WebSocket消息
     * @param {MessageEvent} event - WebSocket消息事件
     */
    handleWebSocketMessage(event) {
        try {
            const data = JSON.parse(event.data);
            console.log('[AudioRecorder] Received message:', data);
            
            // 处理新协议格式
            if (data.header && data.header.event) {
                switch (data.header.event) {
                    case 'task-started':
                        console.log('[AudioRecorder] Received task-started:', data);
                        this.taskStarted = true;
                        this.taskId = data.header.task_id;
                        // 收到task-started后，发送所有暂存的音频块
                        if (this.pendingAudioChunks.length > 0) {
                            this.pendingAudioChunks.forEach(wavFile => {
                                this.sendWavFileInChunks(wavFile);
                            });
                            this.pendingAudioChunks = [];
                        }
                        break;
                    case 'result-generated':
                        console.log('[AudioRecorder] Received result-generated:', data);
                        if (data.payload && data.payload.output && data.payload.output.sentence) {
                            this.handleSpeechResult({
                                text: data.payload.output.sentence.text || '',
                                is_final: data.payload.output.sentence.endTime !== null
                            });
                        }
                        break;
                }
                return;
            }
            
            // 处理旧协议格式
            switch (data.type) {
                case 'task-started':
                    console.log('[AudioRecorder] Received task-started:', data);
                    this.taskStarted = true;
                    if (data.task_id) {
                        this.taskId = data.task_id;
                    }
                    break;
                    
                case 'speech-result':
                    console.log('[AudioRecorder] Received speech-result:', data);
                    this.handleSpeechResult(data);
                    break;
                    
                case 'task-finished':
                    console.log('[AudioRecorder] Received task-finished:', data);
                    break;
                    
                case 'error':
                    console.error('[AudioRecorder] Speech recognition error:', data);
                    this.handleError(data.message || '语音识别出错');
                    break;
                    
                default:
                    console.log('[AudioRecorder] Unknown message type:', data.type);
            }
        } catch (error) {
            console.error('[AudioRecorder] Error parsing WebSocket message:', error, event.data);
        }
    }

    /**
     * 处理语音识别结果
     * @param {Object} data - 识别结果数据
     */
    handleSpeechResult(data) {
        const text = data.text || '';
        const isFinal = data.is_final || false;

        // 拼接识别片段
        if (text) {
            this.recognizedText = text;
        }

        console.log(`[AudioRecorder] Speech result: "${text}" (final: ${isFinal})`);

        // 更新UI显示
        if (this.speechText) {
            this.speechText.textContent = this.recognizedText || '正在识别...';
            if (isFinal) {
                this.speechText.classList.remove('message-streaming');
            } else {
                this.speechText.classList.add('message-streaming');
            }
        }

        // 回调
        if (this.callbacks.onSpeechResult) {
            this.callbacks.onSpeechResult(this.recognizedText, isFinal);
        }
    }

    /**
     * 切换录音状态
     */
    async toggleRecording() {
        if (this.isRecording) {
            this.stopRecording();
        } else {
            await this.startRecording();
        }
    }

    /**
     * 开始录音
     */
    async startRecording() {
        try {
            // 初始化麦克风（如果还没有）
            if (!this.stream) {
                const success = await this.initMicrophone();
                if (!success) return;
            }
            
            // 连接WebSocket（如果还没有）
            if (!this.isConnected) {
                await this.connectWebSocket();
                // 等待连接建立
                await this.waitForConnection();
            }
            
            console.log('[AudioRecorder] Starting recording...');

            this.taskStarted = false;
            this.taskId = null;
            this.audioBuffer = [];
            this.pendingAudioChunks = []; // 新增：用于暂存音频块
            
            // 发送开始任务消息
            this.websocket.send(JSON.stringify({
                task: 'speech-to-text-ws',
                action: 'run-task',
                model: this.modelName,
                parameters: {
                    format: 'wav',
                    sample_rate: 16000,
                    language: 'zh',
                    use_vad: true,
                    return_format: 'text'
                }
            }));
            
            // 重置音频缓冲区
            this.audioBuffer = [];
            
            // 创建音频处理节点
            const source = this.audioContext.createMediaStreamSource(this.stream);
            this.audioProcessor = this.audioContext.createScriptProcessor(4096, 1, 1);
            
            // 收集音频数据
            this.audioProcessor.onaudioprocess = (e) => {
                if (!this.isRecording) return;
                const inputBuffer = e.inputBuffer;
                const audioData = inputBuffer.getChannelData(0);
                this.audioBuffer.push(new Float32Array(audioData));
            };
            
            // 连接节点
            source.connect(this.audioProcessor);
            this.audioProcessor.connect(this.audioContext.destination);
            
            // 开始录音
            this.isRecording = true;
            
            // 定时发送WAV文件
            this.recordingTimer = setInterval(() => {
                if (this.audioBuffer.length > 0) {
                    this.sendAudioChunk();
                }
            }, this.recordingInterval);
            
            this.updateUI(true);
            this.updateStatus('正在录音...', 'active');
            
        } catch (error) {
            console.error('[AudioRecorder] Start recording failed:', error);
            this.handleError('开始录音失败');
        }
    }

    /**
     * 发送当前累积的音频数据块
     */
    sendAudioChunk() {
        if (!this.isRecording || this.audioBuffer.length === 0) return;
        
        try {
            // 计算总长度
            let totalLength = 0;
            for (const buffer of this.audioBuffer) {
                totalLength += buffer.length;
            }
            
            // 合并音频数据
            const mergedBuffer = new Float32Array(totalLength);
            let offset = 0;
            
            for (const buffer of this.audioBuffer) {
                mergedBuffer.set(buffer, offset);
                offset += buffer.length;
            }
            
            // 生成完整的WAV文件
            const wavFile = this.createWavFile(mergedBuffer, 16000);
            
            console.log(`[AudioRecorder] Sending audio chunk, size: ${wavFile.byteLength} bytes`);
            
            // 保存WAV文件到本地
            this.saveWavFileLocally(wavFile);
            
            // 分块发送WAV文件
            if (this.taskStarted) {
                this.sendWavFileInChunks(wavFile);
            } else {
                // 未收到task-started，暂存
                this.pendingAudioChunks.push(wavFile);
            }
            
            // 清空缓冲区
            this.audioBuffer = [];
            
        } catch (error) {
            console.error('[AudioRecorder] Error sending audio chunk:', error);
        }
    }

    /**
     * 保存WAV文件到本地
     * @param {ArrayBuffer} wavFile - WAV文件数据
     */
    saveWavFileLocally(wavFile) {
        try {
            // 创建blob对象
            const blob = new Blob([wavFile], { type: 'audio/wav' });
            
            // 创建下载链接
            const url = URL.createObjectURL(blob);
            
            // 生成时间戳作为文件名
            const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
            const filename = `audio-recording-${timestamp}.wav`;
            
            // 创建下载链接元素
            const downloadLink = document.createElement('a');
            downloadLink.href = url;
            downloadLink.download = filename;
            downloadLink.innerHTML = `<span class="download-link-text">下载录音片段 ${timestamp}</span>`;
            downloadLink.className = 'audio-download-link';
            downloadLink.style.display = 'block';
            downloadLink.style.marginTop = '5px';
            
            // 添加一个可以直接播放的音频元素
            const audio = document.createElement('audio');
            audio.controls = true;
            audio.src = url;
            audio.style.display = 'block';
            audio.style.marginTop = '5px';
            audio.style.width = '100%';
            
            // 创建容器
            const container = document.createElement('div');
            container.className = 'audio-snippet';
            container.style.margin = '10px 0';
            container.style.padding = '10px';
            container.style.border = '1px solid #ddd';
            container.style.borderRadius = '5px';
            container.style.backgroundColor = '#f9f9f9';
            
            // 添加标签
            const label = document.createElement('div');
            label.textContent = `录音片段 (${new Date().toLocaleTimeString()})`;
            label.style.fontWeight = 'bold';
            label.style.marginBottom = '5px';
            
            // 组装元素
            container.appendChild(label);
            container.appendChild(audio);
            container.appendChild(downloadLink);
            
            // 查找或创建音频片段容器
            let audioSnippetsContainer = document.getElementById('audio-snippets-container');
            if (!audioSnippetsContainer) {
                audioSnippetsContainer = document.createElement('div');
                audioSnippetsContainer.id = 'audio-snippets-container';
                audioSnippetsContainer.style.maxHeight = '300px';
                audioSnippetsContainer.style.overflowY = 'auto';
                audioSnippetsContainer.style.marginTop = '20px';
                
                // 创建标题
                const title = document.createElement('h3');
                title.textContent = '录音片段';
                title.style.borderBottom = '1px solid #ddd';
                title.style.paddingBottom = '10px';
                
                // 将标题和音频片段容器添加到页面
                const parentElement = document.querySelector('.speech-input-section') || document.body;
                parentElement.appendChild(title);
                parentElement.appendChild(audioSnippetsContainer);
            }
            
            // 将新的片段添加到容器的顶部
            if (audioSnippetsContainer.firstChild) {
                audioSnippetsContainer.insertBefore(container, audioSnippetsContainer.firstChild);
            } else {
                audioSnippetsContainer.appendChild(container);
            }
            
            console.log(`[AudioRecorder] WAV file saved locally: ${filename}`);
        } catch (error) {
            console.error('[AudioRecorder] Error saving WAV file locally:', error);
        }
    }

    /**
     * 创建完整的WAV文件
     * @param {Float32Array} samples - 音频采样数据
     * @param {number} sampleRate - 采样率
     * @returns {ArrayBuffer} 完整的WAV文件
     */
    createWavFile(samples, sampleRate) {
        // 将Float32Array转换为Int16Array
        const pcmData = new Int16Array(samples.length);
        for (let i = 0; i < samples.length; i++) {
            // 将-1.0到1.0的浮点数转换为-32768到32767的整数
            const s = Math.max(-1, Math.min(1, samples[i]));
            pcmData[i] = s < 0 ? s * 0x8000 : s * 0x7FFF;
        }
        
        // WAV文件头大小：44字节
        const wavHeaderSize = 44;
        const dataSize = pcmData.length * 2; // 16位每样本 = 2字节
        const fileSize = wavHeaderSize + dataSize;
        
        // 创建WAV文件
        const wav = new ArrayBuffer(fileSize);
        const view = new DataView(wav);
        
        // WAV文件头
        // "RIFF"标识
        view.setUint8(0, 0x52); // 'R'
        view.setUint8(1, 0x49); // 'I'
        view.setUint8(2, 0x46); // 'F'
        view.setUint8(3, 0x46); // 'F'
        
        // 文件大小（不包括RIFF/WAVE标识和大小）
        view.setUint32(4, fileSize - 8, true);
        
        // "WAVE"标识
        view.setUint8(8, 0x57);  // 'W'
        view.setUint8(9, 0x41);  // 'A'
        view.setUint8(10, 0x56); // 'V'
        view.setUint8(11, 0x45); // 'E'
        
        // "fmt "子块
        view.setUint8(12, 0x66); // 'f'
        view.setUint8(13, 0x6D); // 'm'
        view.setUint8(14, 0x74); // 't'
        view.setUint8(15, 0x20); // ' '
        
        // fmt子块大小：16
        view.setUint32(16, 16, true);
        
        // 音频格式：1表示PCM
        view.setUint16(20, 1, true);
        
        // 通道数：1
        view.setUint16(22, 1, true);
        
        // 采样率
        view.setUint32(24, sampleRate, true);
        
        // 字节率 = 采样率 * 通道数 * 每样本字节数
        view.setUint32(28, sampleRate * 1 * 2, true);
        
        // 块对齐 = 通道数 * 每样本字节数
        view.setUint16(32, 1 * 2, true);
        
        // 每样本位数：16
        view.setUint16(34, 16, true);
        
        // "data"子块
        view.setUint8(36, 0x64); // 'd'
        view.setUint8(37, 0x61); // 'a'
        view.setUint8(38, 0x74); // 't'
        view.setUint8(39, 0x61); // 'a'
        
        // 数据大小
        view.setUint32(40, dataSize, true);
        
        // 写入PCM数据
        for (let i = 0; i < pcmData.length; i++) {
            view.setInt16(wavHeaderSize + i * 2, pcmData[i], true);
        }
        
        return wav;
    }

    /**
     * 将WAV文件分块发送
     * @param {ArrayBuffer} wavFile - 完整的WAV文件
     */
    sendWavFileInChunks(wavFile) {
        const chunkSize = 32000; // 8KB每块
        const totalSize = wavFile.byteLength;
        
        for (let offset = 0; offset < totalSize; offset += chunkSize) {
            const end = Math.min(offset + chunkSize, totalSize);
            const chunk = wavFile.slice(offset, end);
            
            // 发送二进制数据块
            if (this.websocket && this.isConnected) {
                this.websocket.send(chunk);
            }
        }
    }

    /**
     * 停止录音
     */
    stopRecording() {
        try {
            console.log('[AudioRecorder] Stopping recording...');
            
            // 清除定时器
            if (this.recordingTimer) {
                clearInterval(this.recordingTimer);
                this.recordingTimer = null;
            }
            
            // 发送最后一个音频块
            if (this.audioBuffer.length > 0) {
                if (this.taskStarted) {
                    this.sendAudioChunk();
                } else {
                    // 未收到task-started，暂存
                    this.pendingAudioChunks.push(this.audioBuffer);
                }
            }
            
            // 断开音频处理节点
            if (this.audioProcessor) {
                this.audioProcessor.disconnect();
                this.audioProcessor = null;
            }
            
            this.isRecording = false;
            
            // 发送结束任务消息
            // if (this.websocket && this.isConnected && this.taskStarted && this.taskId) {
            if (this.taskId) {
                console.log('[AudioRecorder] Sending finish-task:', {
                    task: 'speech-to-text-ws',
                    action: 'finish-task',
                    task_id: this.taskId,
                    model: this.modelName
                });

                this.websocket.send(JSON.stringify({
                    task: 'speech-to-text-ws',
                    action: 'finish-task',
                    task_id: this.taskId,
                    model: this.modelName
                }));
            }
                
            this.taskStarted = false;
            // }
            
            this.updateUI(false);
            this.updateStatus('录音已停止');
            
        } catch (error) {
            console.error('[AudioRecorder] Stop recording failed:', error);
            this.handleError('停止录音失败');
        }
    }

    /**
     * 等待WebSocket连接建立
     */
    waitForConnection() {
        return new Promise((resolve, reject) => {
            const timeout = setTimeout(() => {
                reject(new Error('WebSocket connection timeout'));
            }, 5000);
            
            const checkConnection = () => {
                if (this.isConnected) {
                    clearTimeout(timeout);
                    resolve();
                } else {
                    setTimeout(checkConnection, 100);
                }
            };
            
            checkConnection();
        });
    }

    /**
     * 尝试重连WebSocket
     */
    attemptReconnect() {
        if (this.reconnectAttempts < this.wsConfig.maxReconnectAttempts) {
            this.reconnectAttempts++;
            console.log(`[AudioRecorder] Attempting to reconnect (${this.reconnectAttempts}/${this.wsConfig.maxReconnectAttempts})...`);
            
            setTimeout(() => {
                this.connectWebSocket();
            }, this.wsConfig.reconnectInterval);
        } else {
            console.error('[AudioRecorder] Max reconnection attempts reached');
            this.handleError('语音识别服务连接失败，请刷新页面重试');
        }
    }

    /**
     * 更新UI状态
     * @param {boolean} recording - 是否正在录音
     */
    updateUI(recording) {
        if (this.recordBtn) {
            const icon = this.recordBtn.querySelector('.record-icon');
            const text = this.recordBtn.querySelector('.record-text');
            
            if (recording) {
                this.recordBtn.classList.add('recording');
                if (icon) icon.textContent = '⏹️';
                if (text) text.textContent = '点击停止录音';
            } else {
                this.recordBtn.classList.remove('recording');
                if (icon) icon.textContent = '🎤';
                if (text) text.textContent = '点击开始录音';
            }
        }
        
        if (this.recordingIndicator) {
            if (recording) {
                this.recordingIndicator.classList.add('active');
            } else {
                this.recordingIndicator.classList.remove('active');
            }
        }
    }

    /**
     * 更新状态显示
     * @param {string} message - 状态消息
     * @param {string} type - 状态类型
     */
    updateStatus(message, type = '') {
        if (this.speechStatus) {
            this.speechStatus.textContent = message;
            this.speechStatus.className = 'section-status';
            if (type) {
                this.speechStatus.classList.add(type);
            }
        }
        
        if (this.callbacks.onStatusChange) {
            this.callbacks.onStatusChange(message, type);
        }
    }

    /**
     * 处理错误
     * @param {string} message - 错误消息
     */
    handleError(message) {
        console.error('[AudioRecorder] Error:', message);
        this.updateStatus(message, 'error');
        
        if (this.callbacks.onError) {
            this.callbacks.onError(message);
        }
    }

    /**
     * 清理资源
     */
    cleanup() {
        if (this.isRecording) {
            this.stopRecording();
        }
        
        if (this.recordingTimer) {
            clearInterval(this.recordingTimer);
            this.recordingTimer = null;
        }
        
        if (this.stream) {
            this.stream.getTracks().forEach(track => track.stop());
            this.stream = null;
        }
        
        if (this.audioProcessor) {
            this.audioProcessor.disconnect();
            this.audioProcessor = null;
        }
        
        if (this.audioContext && this.audioContext.state !== 'closed') {
            this.audioContext.close().catch(console.error);
        }
        
        if (this.websocket) {
            this.websocket.close();
            this.websocket = null;
        }
        
        this.isConnected = false;
        this.isRecording = false;
        this.audioBuffer = [];
    }
}

// 导出类
window.AudioRecorder = AudioRecorder;