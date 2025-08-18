/**
 * AOG智能内容创作助手 - 流式响应处理模块
 * Copyright 2024-2025 Intel Corporation
 */

class StreamHandler {
    constructor() {
        this.decoder = new TextDecoder();
        this.activeStreams = new Map();
        
        this.callbacks = {
            onStreamData: null,
            onStreamEnd: null,
            onStreamError: null
        };
    }

    /**
     * 设置回调函数
     * @param {Object} callbacks - 回调函数对象
     */
    setCallbacks(callbacks) {
        this.callbacks = { ...this.callbacks, ...callbacks };
    }

    async handleChat(request) {
        try {
            const {
                requestContent,
                model
            } = request;
            const requestData = {
                messages: [
                    // 把用户输入文案转化提炼成几个适合输入给文生图模型的英文关键词，返回数组格式，注意要用英文
                    { role: 'system', content: 'You are a helpful assistant, please convert the user input into suitable English keywords for the image generation model, make sure to return an array of English keywords.' },
                    { role: 'user', content: requestContent }
                ],
                model: model,
                temperature: 0.7
            };

            const response = await fetch('/api/chat/not_stream', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    ...requestData,
                    stream: false
                })
            });

            return await response.json();
        } catch (error) {
            console.error('[StreamHandler] Chat request error:', error);
            throw new Error(`Chat request failed: ${error.message}`);
        }
    }

    /**
     * 处理Chat流式响应
     * @param {string} streamId - 流ID
     * @param {Object} requestData - 请求数据
     * @returns {Promise<void>}
     */
    async handleChatStream(streamId, requestData) {
        try {
            console.log(`[StreamHandler] Starting chat stream: ${streamId}`);
            
            const response = await fetch('/api/chat', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    ...requestData,
                    stream: true
                })
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            // 存储流信息
            this.activeStreams.set(streamId, {
                type: 'chat',
                startTime: Date.now(),
                response: response
            });

            await this.processTextStream(streamId, response);

        } catch (error) {
            console.error(`[StreamHandler] Chat stream error (${streamId}):`, error);
            this.handleStreamError(streamId, error);
        }
    }

    /**
     * 处理文本流
     * @param {string} streamId - 流ID
     * @param {Response} response - Fetch响应对象
     */
    async processTextStream(streamId, response) {
        const reader = response.body.getReader();
        let buffer = '';
        let messageCount = 0;

        try {
            while (true) {
                const { done, value } = await reader.read();
                
                if (done) {
                    console.log(`[StreamHandler] Stream ${streamId} completed. Messages: ${messageCount}`);
                    this.handleStreamEnd(streamId);
                    break;
                }

                // 解码数据块
                const chunk = this.decoder.decode(value, { stream: true });
                buffer += chunk;

                // 处理完整的消息
                const messages = this.extractMessages(buffer);
                
                for (const message of messages.complete) {
                    messageCount++;
                    this.handleStreamData(streamId, message);
                }

                // 保留不完整的消息
                buffer = messages.incomplete;
            }
        } catch (error) {
            console.error(`[StreamHandler] Stream processing error (${streamId}):`, error);
            this.handleStreamError(streamId, error);
        } finally {
            reader.releaseLock();
        }
    }

    async handleGenerate(request) {
        try {
            console.log("[StreamHandler] Generating request:", request);

            const response = await fetch('/api/generate', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    ...request,
                    stream: false
                })
            });

            return await response.json();
        } catch (error) {
            console.error('[StreamHandler] Chat request error:', error);
            throw new Error(`Chat request failed: ${error.message}`);
        }
    }

    /**
     * 从缓冲区提取消息
     * @param {string} buffer - 缓冲区内容
     * @returns {Object} 提取结果
     */
    extractMessages(buffer) {
        const lines = buffer.split('\n');
        const complete = [];
        let incomplete = '';

        for (let i = 0; i < lines.length; i++) {
            const line = lines[i].trim();
            
            if (line === '') {
                continue;
            }

            // 检查是否是完整的JSON
            if (this.isCompleteJSON(line)) {
                try {
                    const parsed = JSON.parse(line);
                    complete.push(parsed);
                } catch (error) {
                    console.warn('[StreamHandler] Failed to parse JSON:', line);
                }
            } else {
                // 如果是最后一行且不完整，保留到下次处理
                if (i === lines.length - 1) {
                    incomplete = line;
                }
            }
        }

        return { complete, incomplete };
    }

    /**
     * 检查字符串是否是完整的JSON
     * @param {string} str - 字符串
     * @returns {boolean} 是否是完整的JSON
     */
    isCompleteJSON(str) {
        if (!str.trim()) return false;
        
        try {
            JSON.parse(str);
            return true;
        } catch (error) {
            return false;
        }
    }

    /**
     * 处理SSE格式的数据
     * @param {string} data - SSE数据
     * @returns {Object|null} 解析后的数据
     */
    parseSSEData(data) {
        const lines = data.split('\n');
        let eventData = null;

        for (const line of lines) {
            if (line.startsWith('data: ')) {
                const jsonStr = line.substring(6);
                
                if (jsonStr === '[DONE]') {
                    return { type: 'done' };
                }
                
                try {
                    eventData = JSON.parse(jsonStr);
                } catch (error) {
                    console.warn('[StreamHandler] Failed to parse SSE data:', jsonStr);
                }
            }
        }

        return eventData;
    }

    /**
     * 处理流数据
     * @param {string} streamId - 流ID
     * @param {Object} data - 数据
     */
    handleStreamData(streamId, data) {
        const streamInfo = this.activeStreams.get(streamId);
        
        if (streamInfo) {
            streamInfo.lastUpdate = Date.now();
        }

        if (this.callbacks.onStreamData) {
            this.callbacks.onStreamData(streamId, data);
        }
    }

    /**
     * 处理流结束
     * @param {string} streamId - 流ID
     */
    handleStreamEnd(streamId) {
        const streamInfo = this.activeStreams.get(streamId);
        
        if (streamInfo) {
            const duration = Date.now() - streamInfo.startTime;
            console.log(`[StreamHandler] Stream ${streamId} ended. Duration: ${duration}ms`);
        }

        this.activeStreams.delete(streamId);

        if (this.callbacks.onStreamEnd) {
            this.callbacks.onStreamEnd(streamId);
        }
    }

    /**
     * 处理流错误
     * @param {string} streamId - 流ID
     * @param {Error} error - 错误对象
     */
    handleStreamError(streamId, error) {
        console.error(`[StreamHandler] Stream ${streamId} error:`, error);
        
        this.activeStreams.delete(streamId);

        if (this.callbacks.onStreamError) {
            this.callbacks.onStreamError(streamId, error);
        }
    }

    /**
     * 取消流
     * @param {string} streamId - 流ID
     */
    cancelStream(streamId) {
        const streamInfo = this.activeStreams.get(streamId);
        
        if (streamInfo && streamInfo.response) {
            try {
                // 尝试取消请求
                if (streamInfo.response.body) {
                    const reader = streamInfo.response.body.getReader();
                    reader.cancel();
                }
            } catch (error) {
                console.warn(`[StreamHandler] Failed to cancel stream ${streamId}:`, error);
            }
        }

        this.activeStreams.delete(streamId);
        console.log(`[StreamHandler] Stream ${streamId} cancelled`);
    }

    /**
     * 获取活跃流信息
     * @returns {Array} 活跃流列表
     */
    getActiveStreams() {
        const streams = [];
        
        this.activeStreams.forEach((info, streamId) => {
            streams.push({
                id: streamId,
                type: info.type,
                startTime: info.startTime,
                lastUpdate: info.lastUpdate,
                duration: Date.now() - info.startTime
            });
        });

        return streams;
    }

    /**
     * 检查流是否活跃
     * @param {string} streamId - 流ID
     * @returns {boolean} 是否活跃
     */
    isStreamActive(streamId) {
        return this.activeStreams.has(streamId);
    }

    /**
     * 清理所有流
     */
    cleanup() {
        console.log('[StreamHandler] Cleaning up all streams...');
        
        this.activeStreams.forEach((info, streamId) => {
            this.cancelStream(streamId);
        });

        this.activeStreams.clear();
    }

    /**
     * 生成唯一的流ID
     * @param {string} prefix - 前缀
     * @returns {string} 流ID
     */
    generateStreamId(prefix = 'stream') {
        const timestamp = Date.now();
        const random = Math.random().toString(36).substring(2, 8);
        return `${prefix}-${timestamp}-${random}`;
    }

    /**
     * 格式化流数据用于显示
     * @param {Object} data - 原始数据
     * @returns {Object} 格式化后的数据
     */
    formatStreamData(data) {
        // 处理不同类型的流数据格式
        if (data.message) {
            return {
                type: 'message',
                content: data.message,
                model: data.model,
                timestamp: data.created_at || new Date().toISOString()
            };
        }

        if (data.choices && data.choices[0]) {
            const choice = data.choices[0];
            return {
                type: 'message',
                content: choice.delta?.content || choice.message?.content || '',
                finish_reason: choice.finish_reason,
                timestamp: new Date().toISOString()
            };
        }

        // 兜底处理
        return {
            type: 'unknown',
            content: JSON.stringify(data),
            timestamp: new Date().toISOString()
        };
    }

    /**
     * 计算流统计信息
     * @param {string} streamId - 流ID
     * @returns {Object|null} 统计信息
     */
    getStreamStats(streamId) {
        const streamInfo = this.activeStreams.get(streamId);
        
        if (!streamInfo) {
            return null;
        }

        const now = Date.now();
        const duration = now - streamInfo.startTime;
        const timeSinceLastUpdate = streamInfo.lastUpdate ? now - streamInfo.lastUpdate : 0;

        return {
            duration,
            timeSinceLastUpdate,
            isStale: timeSinceLastUpdate > 30000, // 30秒无更新认为是停滞
            type: streamInfo.type
        };
    }
}

// 导出类
window.StreamHandler = StreamHandler;
