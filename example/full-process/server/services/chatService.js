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
const { EventEmitter } = require('events');
const constants = require('../../constants')

class ChatService {
    constructor() {
        this.aog = new AogLib();
        this.chatModel = constants.defaultChatModel;
        this.generateModel = constants.defaultGenerateModel;
    }

    /**
     * 处理Chat请求
     * @param {Object} requestData - 请求数据
     * @param {Array} requestData.messages - 消息数组
     * @param {boolean} requestData.stream - 是否流式响应
     * @param {string} requestData.model - 模型名称
     * @returns {Promise|EventEmitter} 响应数据或流式事件发射器
     */
    async chat(requestData) {
        const {
            messages,
            stream = true,
            model = this.chatModel,
            temperature = 0.7,
        } = requestData;

        console.log(`[Chat] Processing request with model: ${model}, stream: ${stream}`);

        const chatData = {
            model: model,
            messages: messages,
            stream: stream,
            temperature: temperature,
        };

        try {
            if (stream) {
                // 流式响应处理
                return await this.handleStreamResponse(chatData);
            } else {
                // 非流式响应
                const result = await this.aog.chat(chatData);
                console.log(`[Chat] Non-stream response received: ${JSON.stringify(result)}`);
                return this.formatResponse(result);
            }
        } catch (error) {
            console.error('[Chat] Service error:', error);
            throw new Error(`Chat service failed: ${error.message}`);
        }
    }

    async generate(request) {
        const {
            prompt,
            model = this.generateModel,
            stream = false,
            images
        } = request;
        console.log(`[Generate] Processing request with model: ${model}, stream: ${stream}`);

        const generateData = {
            images: images,
            prompt: prompt,
            stream: stream,
            model: model
        }

        try {
            const result= await this.aog.generate(generateData);
            console.log(`[Generate] Non-stream response received: ${JSON.stringify(result)}`);
            return this.formatResponse(result);
        } catch (error) {
            console.error('[Generate] Service error:', error);
            throw new Error(`Generate service failed: ${error.message}`);
        }
    }

    /**
     * 处理流式响应
     * @param {Object} chatData - Chat请求数据
     * @returns {EventEmitter} 事件发射器
     */
    async handleStreamResponse(chatData) {
        const emitter = new EventEmitter();
        
        try {
            const streamResult = await this.aog.chat(chatData);
            
            if (streamResult && streamResult.on) {
                // 如果返回的是流对象
                streamResult.on('data', (chunk) => {
                    try {
                        // 如果chunk的business_code字段是500则舍弃
                        if (chunk.business_code === 500) {
                            console.warn('[Chat] Skipping chunk with business_code 500:', chunk);
                            return;
                        }
                        emitter.emit('data', JSON.stringify(chunk) + '\n');
                        console.log(`[Chat] Stream data emitted: ${JSON.stringify(chunk)}`);
                    } catch (parseError) {
                        console.error('[Chat] Stream parse error:', parseError);
                    }
                });

                streamResult.on('end', () => {
                    console.log('[Chat] Stream ended');
                    emitter.emit('end');
                });

                streamResult.on('error', (error) => {
                    console.error('[Chat] Stream error:', error);
                    emitter.emit('error', error);
                });
            } else {
                // 如果返回的不是流对象，直接发送结果
                setTimeout(() => {
                    const formattedResult = this.formatResponse(streamResult);
                    emitter.emit('data', JSON.stringify(formattedResult) + '\n');
                    emitter.emit('end');
                }, 0);
            }
        } catch (error) {
            setTimeout(() => {
                emitter.emit('error', error);
            }, 0);
        }

        return emitter;
    }

    /**
     * 解析流式数据块
     * @param {Buffer|string} chunk - 数据块
     * @returns {Object|null} 解析后的数据
     */
    parseStreamChunk(chunk) {
        try {
            const chunkStr = chunk.toString();
            // chunkStr 是 [object Object],需要解析他
            console.log(`[Chat] Parsing chunk: ${chunkStr[0]}`);
            
            // 尝试解析JSON
            if (chunkStr.trim().startsWith('{')) {
                console.log(`[Chat] Parsing chunk as JSON: ${chunkStr}`);
                return JSON.parse(chunkStr);
            }
            
            // 处理SSE格式
            if (chunkStr.includes('data: ')) {
                console.log(`[Chat] Parsing chunk as SSE: ${chunkStr}`);
                const dataMatch = chunkStr.match(/data: (.+)/);
                if (dataMatch && dataMatch[1] !== '[DONE]') {
                    return JSON.parse(dataMatch[1]);
                }
            }
            
            return null;
        } catch (error) {
            console.error('[Chat] Chunk parse error:', error);
            return null;
        }
    }

    /**
     * 格式化响应数据
     * @param {Object} result - AOG返回的原始数据
     * @returns {Object} 格式化后的响应
     */
    formatResponse(result) {
        if (!result) {
            throw new Error('Empty response from AOG');
        }

        // 检查AOG响应格式
        if (result.code === 200 && result.data) {
            return {
                success: true,
                message: result.data.message || result.data,
                model: result.data.model,
                created_at: result.data.created_at,
                finish_reason: result.data.finish_reason,
                aog_info: {
                    served_by: result.data.aog?.served_by,
                    served_by_api_flavor: result.data.aog?.served_by_api_flavor,
                    response_time: result.data.aog?.response_time
                }
            };
        }

        // 处理直接返回的消息格式
        if (result.message) {
            return {
                success: true,
                message: result.message,
                model: result.model,
                created_at: result.created_at,
                finish_reason: result.finish_reason
            };
        }

        // 兜底处理
        return {
            success: true,
            message: result,
            timestamp: new Date().toISOString()
        };
    }

}

module.exports = new ChatService();
