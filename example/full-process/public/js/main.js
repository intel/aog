/**
 * AOG智能内容创作助手 - 主应用逻辑 (步骤化版本)
 * Copyright 2024-2025 Intel Corporation
 */
const defaultSpeechToTextModel = "NamoLi/whisper-large-v3-ov";

const chatPrompt = "Please generate a vivid and creative description of the picture in English for the user's requirements, and make sure to answer in English.";

const defaultGenerateModel = "gemma3:4b";
const generatePrompt = "根据图片内容，生成一首短诗，格式不限，注意分行，只需要返回诗的内容，不要返回其他语句。";

const defaultChatModel = "qwen2.5:0.5b";

const defaultImageModel = "OpenVINO/LCM_Dreamshaper_v7-fp16-ov";
const defaultImageSize = "1024*1024";
const defaultImageCount = 2;

const defaultCloudImageModel = "qwen2.5:0.5b";
const defaultCloudImageSize = "1024*1024";
const defaultCloudImageType = "path";

const defaultTextToSpeechModel = "qwen-tts";
const defaultTextToSpeechVoice = "Cherry";

class AOGContentCreator {
    constructor() {
        // 初始化各个模块
        this.audioRecorder = new AudioRecorder(defaultSpeechToTextModel);
        this.cameraCapture = new CameraCapture();
        this.streamHandler = new StreamHandler();
        this.statusMonitor = new StatusMonitor();
        this.stepManager = new StepManager();

        this.maxStep = 7;

        // 应用状态
        this.workflowStartTime = Date.now();
        this.generatedContent = {
            speechText: '',
            chatResponse: '',
            localImages: [],
            optimizationText: '',
            cloudImages: [],
            audioUrl: '',
            cameraImage: null
        };
        this.isOptimizing = false

        this.initializeApp();
    }

    /**
     * 初始化应用
     */
    async initializeApp() {
        console.log('[AOGContentCreator] Initializing step-based application...');

        try {
            // 设置回调函数
            this.setupCallbacks();

            // 监听步骤事件
            this.setupStepEventListeners();

            // 启动状态监控
            this.statusMonitor.startMonitoring();
            this.stepPages = {};
            
            for (let i = 1; i <= 7; i++) {
                this.stepPages[i] = document.getElementById(`step-${i}`);
            }
            this.stepPages.complete = document.getElementById('step-complete');
            this.stepIndicator = document.querySelector('.step-indicator');
            this.chatMessages = document.getElementById('chat-messages');
            this.localImages = document.getElementById('local-images');
            this.cloudImages = document.getElementById('cloud-comparison');
            this.audioPlayer = document.getElementById('audio-player');
            this.optimizeBtn = document.getElementById('confirm-optimization-btn');
            this.generateAudioBtn = document.getElementById('generate-audio-btn');
            this.generatedAudio = document.getElementById('generated-audio');
            this.optimizationText = document.getElementById('optimization-text');

            console.log('[AOGContentCreator] Application initialized successfully');

        } catch (error) {
            console.error('[AOGContentCreator] Initialization failed:', error);
            this.showError('应用初始化失败，请刷新页面重试');
        }
    }

    /**
     * 设置各模块的回调函数
     */
    setupCallbacks() {
        // 音频录制回调
        this.audioRecorder.setCallbacks({
            onSpeechResult: (text, isFinal) => this.handleSpeechResult(text, isFinal),
            onError: (error) => this.showError(`语音识别错误: ${error}`)
        });
        
        // 摄像头拍摄回调
        this.cameraCapture.setCallbacks({
            onImageCaptured: (imageUrl) => this.handleImageCaptured(imageUrl),
            onError: (error) => this.showError(`摄像头错误: ${error}`)
        });

        // 流式处理回调
        this.streamHandler.setCallbacks({
            onStreamData: (streamId, data) => this.handleStreamData(streamId, data),
            onStreamEnd: (streamId) => {
                const confirmBtn = document.getElementById('confirm-content-btn');
                if (confirmBtn && this.generatedContent.chatResponse && this.generatedContent.chatResponse.trim()) {
                    confirmBtn.disabled = false;
                }
                const regenBtn = document.getElementById('regenerate-content-btn');
                if (this.generatedContent.chatResponse && this.generatedContent.chatResponse.trim()) {
                    if (confirmBtn) confirmBtn.disabled = false;
                    if (regenBtn) regenBtn.style.display = 'inline-flex';
                }
            },
            onStreamError: (streamId, error) => this.showError(`流处理错误: ${error.message}`)
        });

        // 状态监控回调
        this.statusMonitor.setCallbacks({
            onStatusChange: (services) => this.handleServiceStatusChange(services),
            onMetricsUpdate: (metrics) => this.handleMetricsUpdate(metrics)
        });
    }

    /**
     * 设置步骤事件监听器
     */
    setupStepEventListeners() {
        document.addEventListener('step-event', (event) => {
            const { type, step } = event.detail;

            switch (type) {
                case 'enter':
                    this.handleStepEnter(step);
                    break;
                case 'complete':
                    this.handleStepComplete(step);
                    break;
                case 'retry':
                    this.handleStepRetry(step);
                    break;
                case 'skip':
                    this.handleStepSkip(step);
                    break;
                case 'workflow-complete':
                    this.stepManager.handleWorkflowComplete();
                    break;
                case 'new-workflow':
                    this.stepManager.handleNewWorkflow();
                    break;
            }
        });
    }

    /**
     * 处理步骤进入
     */
    handleStepEnter(step) {
        console.log(`[AOGContentCreator] Entering step ${step}`);

        switch (step) {
            case 1:
                this.initializeStep1();
                break;
            case 2:
                this.initializeStep2();
                break;
            case 3:
                this.initializeStep3();
                break;
            case 4:
                this.initializeStep4();
                break;
            case 5:
                this.initializeStep5();
                break;
            case 6:
                this.initializeStep6();
                break;
            case 7:
                this.initializeStep7();
                break;
        }
    }

    /**
     * 处理步骤完成
     */
    handleStepComplete(step) {
        console.log(`[AOGContentCreator] Step ${step} completed`);

        switch (step) {
            case 1:
                this.completeStep1();
                break;
            case 2:
                this.completeStep2();
                break;
            case 3:
                this.completeStep3();
                break;
            case 4:
                this.completeStep4();
                break;
            case 5:
                this.completeStep5();
                break;
            case 6:
                this.completeStep6();
                break;
            case 7:
                this.completeStep7();
                break;
        }
    }

    /**
     * 处理步骤重试
     */
    handleStepRetry(step) {
        console.log(`[AOGContentCreator] Retrying step ${step}`);

        switch (step) {
            case 1:
                this.retryStep1();
                break;
            case 2:
                this.retryStep2();
                break;
            case 4:
                this.retryStep4();
                break;
            case 5:
                this.retryStep5();
                break;
            case 6:
                this.retryStep6();
                break;
            case 7:
                this.retryStep7();
                break;
        }
    }

    /**
     * 处理步骤跳过
     */
    handleStepSkip(step) {
        console.log(`[AOGContentCreator] Skipping step ${step}`);

        if (step === 3) {
            // 跳过摄像头拍摄
            this.generatedContent.cameraImage = null;
        }
    }

    /**
     * 跳转到指定步骤
     * @param {number} step - 目标步骤
     */
    goToStep(step) {
        if (this.stepManager) {
            this.stepManager.goToStep(step);
        }
    }

    /**
     * 处理语音识别结果
     * @param {string} text - 识别的文本
     * @param {boolean} isFinal - 是否是最终结果
     */
    async handleSpeechResult(text, isFinal) {
        console.log(`[AOGContentCreator] Speech result: "${text}" (final: ${isFinal})`);

        const speechTextElement = document.getElementById('speech-text');
        if (speechTextElement) {
            speechTextElement.value = text || '正在识别...';
        }
        this.generatedContent.speechText = text;

        if ( text.trim()) {
            

            // 启用确认按钮
            const confirmBtn = document.getElementById('confirm-speech-btn');
            if (confirmBtn) {
                confirmBtn.disabled = false;
            }

            // 显示重试按钮
            const retryBtn = document.getElementById('retry-speech-btn');
            if (retryBtn) {
                retryBtn.style.display = 'inline-flex';
            }
        }
    }

    /**
     * 确认语音输入并进入下一步
     */
    async confirmSpeechAndNext() {
        const speechTextElement = document.getElementById('speech-text');
        const text = speechTextElement ? speechTextElement.value.trim() : '';
        if (!text) {
            this.showError('请先完成语音输入');
            return;
        }
        this.generatedContent.speechText = text;
        this.goToStep(2);
        await this.generateChatResponse(text);
    }

    /**
     * 重新进行语音输入
     */
    retryStep1() {
        this.generatedContent.speechText = '';

        const speechTextElement = document.getElementById('speech-text');
        if (speechTextElement) {
            speechTextElement.value = '等待语音输入...';
        }

        const confirmBtn = document.getElementById('confirm-speech-btn');
        if (confirmBtn) {
            confirmBtn.disabled = true;
        }

        const retryBtn = document.getElementById('retry-speech-btn');
        if (retryBtn) {
            retryBtn.style.display = 'none';
        }

        // 重新开始录音
        this.audioRecorder?.stopRecording?.();
        this.audioRecorder.startRecording();
    }

    /**
     * 生成Chat响应
     * @param {string} userInput - 用户输入
     */
    async generateChatResponse(userInput, model) {
        try {
            console.log('[AOGContentCreator] Generating chat response...');

            // 显示用户输入
            const userInputDisplay = document.getElementById('user-input-display');
            if (userInputDisplay) {
                userInputDisplay.textContent = userInput;
            }

            // 清空聊天消息
            const chatMessages = document.getElementById('chat-messages');
            if (chatMessages) {
                chatMessages.innerHTML = `
                    <div class="welcome-message">
                        <div class="message-icon">🤖</div>
                        <div class="message-text">正在为您生成创意文案，请稍候...</div>
                    </div>
                `;
            }

            // 开始流式生成
            const streamId = this.streamHandler.generateStreamId('chat');
            const requestData = {
                model: model,
                messages: [
                    {
                        role: 'system',
                        content: chatPrompt
                    },
                    {
                        role: 'user',
                        content: userInput
                    }
                ]
            };

            const startTime = Date.now();

            await this.streamHandler.handleChatStream(streamId, requestData);

            const responseTime = Date.now() - startTime;
            this.statusMonitor.recordServiceCall('chat', responseTime, 'local');

        } catch (error) {
            console.error('[AOGContentCreator] Chat generation failed:', error);
            this.showError('文案生成失败，请重试');
            this.statusMonitor.recordServiceError('chat', error.message);
        }
    }

    /**
     * 确认文案并进入下一步
     */
    confirmContentAndNext() {
        if (!this.generatedContent.chatResponse.trim()) {
            this.showError('请等待文案生成完成');
            return;
        }

        // 进入步骤3（摄像头拍摄）
        this.goToStep(3);
    }

    /**
     * 重新生成文案
     */
    async regenerateContent() {
        // 获取语音识别结果
        const speechText = this.generatedContent.speechText.trim();
        // if (!speechText) {
        //     this.showError('没有语音输入内容');
        //     return;
        // }

        // 清空当前文案
        this.generatedContent.chatResponse = '';

        // 禁用确认按钮
        const confirmBtn = document.getElementById('confirm-content-btn');
        if (confirmBtn) {
            confirmBtn.disabled = true;
        }

        // 获取当前模型
        const chatModel = document.getElementById('chat-model');
        const model = chatModel?.value || defaultChatModel;

        // 重新生成文案
        await this.generateChatResponse(speechText, model);
    }

    /**
     * 跳过摄像头拍摄并进入下一步
     */
    skipCameraAndNext() {
        console.log('[AOGContentCreator] Skipping camera capture');
        this.generatedContent.cameraImage = null;

        // 进入步骤4并开始生成本地图片
        this.goToStep(4);
        this.generateLocalImages(this.generatedContent.chatResponse);
    }

    /**
     * 确认摄像头拍摄并进入下一步
     */
    confirmCameraAndNext() {
        // 进入步骤4并开始生成本地图片
        this.goToStep(4);
        this.generateLocalImages(this.generatedContent.chatResponse);
    }

    /**
     * 处理流数据
     * @param {string} streamId - 流ID
     * @param {Object} data - 流数据
     */
    handleStreamData(streamId, data) {
        if (streamId.startsWith('chat-')) {
            this.updateChatMessage(data);
        }
    }

    /**
     * 处理流结束
     * @param {string} streamId - 流ID
     */
    async handleStreamEnd(streamId) {
        if (streamId.startsWith('chat-')) {
            const confirmBtn = document.getElementById('confirm-content-btn');
            const regenBtn = document.getElementById('regenerate-content-btn');
            if (this.generatedContent.chatResponse && this.generatedContent.chatResponse.trim()) {
                if (confirmBtn) confirmBtn.disabled = false;
                if (regenBtn) regenBtn.style.display = 'inline-flex';
            }
            // 步骤高亮同步
            if (window.stepManager && typeof window.stepManager.updateStepIndicator === 'function') {
                window.stepManager.updateStepIndicator();
            }
        }
    }

    /**
     * 生成本地垫图
     * @param {string} prompt - 用于生图的关键词
     */
    async generateLocalImages(prompt) {
        try {
            console.log('[AOGContentCreator] Generating local images...', prompt);
            this.updateSectionStatus('image-section', '正在生成本地垫图...', 'active');

            // 用关键词作为prompt
            const requestData = {
                prompt: prompt,
                n: 2
            };

            const startTime = Date.now();
            const response = await fetch('/api/text-to-image/local', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(requestData)
            });

            console.log('[AOGContentCreator] Local image generation response received:', response);

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }

            const result = await response.json();
            const responseTime = Date.now() - startTime;
            if (result) {
                console.log(result)
                this.generatedContent.localImages = result.images;
                this.displayImages('local', result.images);
                this.updateSectionStatus('image-section', '本地垫图生成完成', 'completed');

                // 启用语音优化功能
                this.setCurrentStep(5);
                this.enableVoiceOptimization();

                // 记录性能指标
                this.statusMonitor.recordServiceCall('text-to-image', responseTime, 'local');
                document.getElementById('local-timing').textContent = `${responseTime}ms`;
            } else {
                throw new Error('Local image generation failed');
            }

        } catch (error) {
            console.error('[AOGContentCreator] Local image generation failed:', error);
            this.showError('本地图片生成失败，请重试');
            this.statusMonitor.recordServiceError('text-to-image', error.message);
        }
    }

    /**
     * 启用语音优化功能
     */
    enableVoiceOptimization() {
        if (this.optimizeBtn) {
            this.optimizeBtn.disabled = false;
        }
        this.updateSectionStatus('voice-optimization-section', '请语音描述优化方向', 'active');
    }

    /**
     * 开始语音优化
     */
    async startVoiceOptimization() {
        this.audioRecorder = new AudioRecorder(defaultSpeechToTextModel);
        if (this.isOptimizing) return;
        this.isOptimizing = true;
        this.updateSectionStatus('voice-optimization-section', '正在录音...', 'active');
        const indicator = document.getElementById('optimize-recording-indicator');
        if (indicator) indicator.style.display = 'block';

        const optimizationTextArea = document.getElementById('optimization-text');
        if (optimizationTextArea) {
            optimizationTextArea.value = '';
        }

        // 设置回调
        const originalCallback = this.audioRecorder.callbacks.onSpeechResult;
        this.audioRecorder.setCallbacks({
            ...this.audioRecorder.callbacks,
            onSpeechResult: (text, isFinal) => {
                const optimizationTextArea = document.getElementById('optimization-text');
                if (optimizationTextArea) {
                    optimizationTextArea.value = text || '正在识别...';
                }
                this.generatedContent.optimizationText = text;
                if (isFinal && text.trim()) {
                    this.handleOptimizationResult(text);
                    // 恢复原始回调
                    this.audioRecorder.setCallbacks({
                        ...this.audioRecorder.callbacks,
                        onSpeechResult: originalCallback
                    });
                    this.stopVoiceOptimization();
                }
            }
        });

        await this.audioRecorder.startRecording();
    }

    stopVoiceOptimization() {
        this.isOptimizing = false;
        this.audioRecorder.stopRecording?.();
        const indicator = document.getElementById('optimize-recording-indicator');
        if (indicator) indicator.style.display = 'none';
        this.updateSectionStatus('voice-optimization-section', '录音已停止', 'active');
    }

    /**
     * 处理优化指令结果
     * @param {string} optimizationText - 优化指令文本
     */
    async handleOptimizationResult(optimizationText) {
        console.log(`[AOGContentCreator] Optimization instruction: "${optimizationText}"`);
        this.generatedContent.optimizationText = optimizationText;

        const optimizationTextArea = document.getElementById('optimization-text');
        if (optimizationTextArea) {
            optimizationTextArea.value = optimizationText;
        }

        this.updateSectionStatus('voice-optimization-section', '优化指令已接收', 'completed');
        // 生成云端精细化图片
        this.setCurrentStep(6);
        // await this.generateCloudImages();
    }

    async confirmOptimizationAndNext() {
        const optimizationTextArea = document.getElementById('optimization-text');
        const text = optimizationTextArea ? optimizationTextArea.value.trim() : '';
        if (!text) {
            this.showError('请先输入优化指令');
            return;
        }
        this.generatedContent.optimizationText = text;
        this.goToStep(6);
        // await this.generateCloudImages();
    }

    /**
     * 生成云端精细化图片
     */
    async generateCloudImages(prompt) {
        try {
            console.log('[AOGContentCreator] Generating cloud images...');
            this.updateSectionStatus('image-section', '正在生成云端精图...', 'active');
            
            // 获取选中图片的本地绝对路径
            let localImagePath = null;
            let filename = '';
            
            if (this.selectedLocalImage) {
                // 从 URL 提取文件名部分
                const url = new URL(this.selectedLocalImage);
                const pathname = url.pathname; // 例如: /uploads/images/local-123456789.png
                filename = pathname.split('/').pop(); // 提取文件名，例如: local-123456789.png
                
                // 构建绝对路径
                // localImagePath = __dirname + `\\server\\uploads\\images\\${filename}`;
                
            }

            const requestData = {
                prompt: this.generatedContent.optimizationText,
                image: filename,
                image_type: "path"     // 使用path类型
            };
            
            console.log('[AOGContentCreator] Cloud image request data:', requestData);
            
            const startTime = Date.now();
            const response = await fetch('/api/text-to-image/cloud', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(requestData)
            });
            
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            const result = await response.json();
            const responseTime = Date.now() - startTime;


            if (result) {
                console.log(result);
                this.generatedContent.cloudImages = result.images;
                this.displayImages('cloud', result.images);
                this.updateSectionStatus('image-section', '云端精图生成完成', 'completed');
                
                // 启用语音播报功能
                // this.setCurrentStep(7);
                // this.enableAudioGeneration();
                
                // 记录性能指标
                this.statusMonitor.recordServiceCall('text-to-image', responseTime, 'remote');
                document.getElementById('cloud-timing').textContent = `${responseTime}ms`;
            } else {
                throw new Error('Cloud image generation failed');
            }
            const confirmBtn = document.getElementById('confirm-cloud-btn');
            if (confirmBtn) {
                confirmBtn.disabled = false;
            }
            
        } catch (error) {
            console.error('[AOGContentCreator] Cloud image generation failed:', error);
            this.showError('云端图片生成失败，请重试');
            this.statusMonitor.recordServiceError('text-to-image', error.message);
        }
    }

    /**
     * 启用音频生成功能
     */
    enableAudioGeneration() {
        if (this.generateAudioBtn) {
            this.generateAudioBtn.disabled = false;
        }
        this.updateSectionStatus('audio-section', '可以生成语音播报', 'active');
    }

    async generatePoems() {
        try {
            console.log(`[Generate] Processing request with model: ${model}, stream: ${stream}`);

            const response = await fetch('/api/geneate', {
                method: "POST",
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(requestData)
            })
        } catch (error) {
            console.error('[AOGContentCreator] Generation failed:', error);
            this.showError('文案生成失败，请重试');
        }
    }

    /**
     * 生成音频
     */
    async generateAudio(text) {
        try {
            console.log('[AOGContentCreator] Generating audio...');
            this.updateSectionStatus('audio-section', '正在生成语音...', 'active');
            
            const requestData = {
                text: text,
                voice: defaultTextToSpeechVoice
            };
            
            const startTime = Date.now();
            const response = await fetch('/api/text-to-speech', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(requestData)
            });
            
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            const result = await response.json();
            const responseTime = Date.now() - startTime;
            
            if (result.success) {
                this.generatedContent.audioUrl = result.audio_url;
                this.displayAudio(result.audio_url);
                this.updateSectionStatus('audio-section', '语音播报生成完成', 'completed');
                
                // 记录性能指标
                this.statusMonitor.recordServiceCall('text-to-speech', responseTime, 'local');
                
                // 完成所有步骤
                this.completeWorkflow();
            } else {
                throw new Error('Audio generation failed');
            }
            
        } catch (error) {
            console.error('[AOGContentCreator] Audio generation failed:', error);
            this.showError('语音生成失败，请重试');
            this.statusMonitor.recordServiceError('text-to-speech', error.message);
        }
    }

    /**
     * 完成工作流程
     */
    completeWorkflow() {
        console.log('[AOGContentCreator] Workflow completed successfully!');

        // 标记所有步骤为完成
        for (let i = 1; i <= this.maxStep; i++) {
            this.markStepCompleted(i);
        }

        this.showSuccess('内容创作流程已完成！');
    }

    /**
     * 添加聊天消息
     * @param {string} role - 角色 (user/assistant)
     * @param {string} content - 消息内容
     * @param {boolean} streaming - 是否是流式消息
     * @returns {string} 消息ID
     */
    addChatMessage(role, content, streaming = false) {
        if (!this.chatMessages) return null;

        const messageId = `msg-${Date.now()}-${Math.random().toString(36).substring(2, 8)}`;
        const messageDiv = document.createElement('div');
        messageDiv.className = `message ${role}`;
        messageDiv.id = messageId;

        if (streaming) {
            messageDiv.classList.add('message-streaming');
        }

        const icon = role === 'user' ? '👤' : '🤖';
        messageDiv.innerHTML = `
            <div class="message-icon">${icon}</div>
            <div class="message-text">${content}</div>
        `;

        this.chatMessages.appendChild(messageDiv);
        this.chatMessages.scrollTop = this.chatMessages.scrollHeight;

        return messageId;
    }

    /**
     * 更新聊天消息
     * @param {Object} data - 消息数据
     */
    updateChatMessage(data) {
        // 覆盖 welcome-message 的内容
        const welcomeMsg = this.chatMessages?.querySelector('.welcome-message .message-text');

        if (welcomeMsg && data.message) {
            if (welcomeMsg.textContent === '正在为您生成创意文案，请稍候...') {
                welcomeMsg.textContent = '';
            }
            welcomeMsg.textContent += data.message.content;
            this.generatedContent.chatResponse = welcomeMsg.textContent;
            this.chatMessages.scrollTop = this.chatMessages.scrollHeight;
        }
    }

    /**
     * 显示图片
     * @param {string} type - 图片类型 (local/cloud)
     * @param {Array} images - 图片URL数组
     */
    displayImages(type, images) {
        // type === 'local' 时，插入到 generation-status
        if (type === 'local') {
            const statusDiv = document.getElementById('local-generation-status');
            if (!statusDiv) return;

            statusDiv.innerHTML = ''; // 清空原有“准备生成本地预览图...”内容

            // 创建 image-grid 容器
            const grid = document.createElement('div');
            grid.className = 'image-grid';
            statusDiv.appendChild(grid);

            // 确保images是数组
            const imageArray = Array.isArray(images) ? images : (images ? [images] : []);
            if (imageArray.length === 0) {
                grid.innerHTML = '<div class="no-images">无图片可显示</div>';
                return;
            }

            imageArray.forEach((imageUrl, index) => {
                const img = document.createElement('img');
                img.src = imageUrl;
                img.alt = `本地生成图片${index + 1}`;
                img.style.width = '280px';
                img.style.height = '280px';
                img.style.margin = '5px';
                img.style.borderRadius = '8px';
                img.style.cursor = 'pointer';
                img.style.objectFit = 'cover';
                img.addEventListener('click', () => this.selectImage(img));
                grid.appendChild(img);
            });
            return;
        }

        // 云端图片插入 cloudImages
        const container = type === 'cloud' ? this.cloudImages : this.localImages;
        if (!container) return;
        container.innerHTML = '';

        // 只取第一张图片
        const imageUrl = Array.isArray(images) ? images[0] : images;
        if (!imageUrl) {
            container.innerHTML = '<div class="no-images">无图片可显示</div>';
            return;
        }

        const img = document.createElement('img');
        img.src = imageUrl;
        img.alt = '云端精细化图片';
        img.style.width = '100%';
        img.style.maxWidth = '300px';
        img.style.height = '300px';
        img.style.borderRadius = '10px';
        img.style.objectFit = 'cover';
        container.appendChild(img);
    }

    /**
     * 选择图片
     * @param {HTMLImageElement} imgElement - 图片元素
     */
    selectImage(imgElement) {
        // 移除其他图片的选中状态
        const allImages = document.querySelectorAll('.image-grid img');
        allImages.forEach(img => img.classList.remove('selected'));

        // 选中当前图片
        imgElement.classList.add('selected');

        // 记录选中的图片
        this.selectedLocalImage = imgElement.src;

        // 启用“确认预览并继续”按钮
        const confirmBtn = document.getElementById('confirm-local-btn');
        if (confirmBtn) {
            confirmBtn.disabled = false;
        }

        console.log('[AOGContentCreator] Image selected:', imgElement.src);
    }

    /**
     * 显示音频播放器
     * @param {string} audioUrl - 音频URL
     */
    displayAudio(audioUrl) {
        if (this.generatedAudio) {
            this.generatedAudio.src = audioUrl;
        }

        if (this.audioPlayer) {
            this.audioPlayer.style.display = 'block';
        }

        console.log('[AOGContentCreator] Audio player displayed:', audioUrl);
    }

    /**
     * 提取图片生成的prompt
     * @param {string} text - 文本内容
     * @returns {string} 提取的prompt
     */
    extractImagePrompt(text) {
        // 简单的关键词提取逻辑
        const keywords = text.split(/[，。！？；：\s]+/)
            .filter(word => word.length > 1)
            .slice(0, 5)
            .join(', ');

        return keywords || 'beautiful, detailed, high quality';
    }

    /**
     * 设置当前步骤
     * @param {number} step - 步骤号
     */
    setCurrentStep(step) {
        this.currentStep = Math.min(step, this.maxStep);
        this.updateStepIndicator();
        console.log(`[AOGContentCreator] Current step: ${this.currentStep}`);
    }

    /**
     * 标记步骤为完成
     * @param {number} step - 步骤号
     */
    markStepCompleted(step) {
        const stepElement = this.stepIndicator?.querySelector(`[data-step="${step}"]`);
        if (stepElement) {
            stepElement.classList.remove('active');
            stepElement.classList.add('completed');
        }
    }

    /**
     * 更新步骤指示器
     */
    updateStepIndicator() {
        if (!this.stepIndicator) return;

        const steps = this.stepIndicator.querySelectorAll('.step');

        steps.forEach((step, index) => {
            const stepNumber = index + 1;
            step.classList.remove('active', 'completed');

            if (stepNumber < this.currentStep) {
                step.classList.add('completed');
            } else if (stepNumber === this.currentStep) {
                step.classList.add('active');
            }
        });
    }

    /**
     * 更新区块状态
     * @param {string} sectionId - 区块ID
     * @param {string} message - 状态消息
     * @param {string} type - 状态类型
     */
    updateSectionStatus(sectionId, message, type = '') {
        const section = document.getElementById(sectionId);
        if (!section) return;

        const statusElement = section.querySelector('.section-status');
        if (statusElement) {
            statusElement.textContent = message;
            statusElement.className = 'section-status';
            if (type) {
                statusElement.classList.add(type);
            }
        }
    }

    /**
     * 处理图片拍摄
     * @param {string} imageUrl - 图片URL
     */
    handleImageCaptured(imageUrl) {
        console.log('[AOGContentCreator] Camera image captured:', imageUrl);
        // 记录选中的图片为拍摄图片
        this.generatedContent.cameraImage = imageUrl;
        this.selectedLocalImage = imageUrl;
        this.goToStep(5);
    }

    /**
     * 处理服务状态变化
     * @param {Object} services - 服务状态
     */
    handleServiceStatusChange(services) {
        // console.log('[AOGContentCreator] Service status updated:', services);
    }

    /**
     * 处理性能指标更新
     * @param {Object} metrics - 性能指标
     */
    handleMetricsUpdate(metrics) {
        console.log('[AOGContentCreator] Performance metrics updated:', metrics);
    }

    /**
     * 处理模型变化
     */
    handleModelChange() {
        const selectedModel = this.chatModel?.value;
        console.log('[AOGContentCreator] Model changed to:', selectedModel);
    }

    /**
     * 显示成功消息
     * @param {string} message - 成功消息
     */
    showSuccess(message) {
        console.log(`[AOGContentCreator] Success: ${message}`);
        // 这里可以添加UI提示
    }

    /**
     * 显示错误消息
     * @param {string} message - 错误消息
     */
    showError(message) {
        console.error(`[AOGContentCreator] Error: ${message}`);
        // 这里可以添加UI提示
        alert(message); // 临时使用alert，实际应用中可以使用更好的UI组件
    }

    toggleVoiceOptimization() {
        if (this.isOptimizing) {
            this.stopVoiceOptimization();
        } else {
            this.startVoiceOptimization();
        }
    }


    async getKeyword() {
        const chatResponse = this.generatedContent.chatResponse;
        if (!chatResponse || chatResponse.trim() === '') {
            this.showError('请先生成文案');
            return '';
        }
        const chatModel = document.getElementById('chat-model');
        const model = chatModel?.value || defaultChatModel;
        const request = {
            model: model,
            requestContent: chatResponse
        }
        const response = await this.streamHandler.handleChat(request);
        console.log('[AOGContentCreator] Keywords extraction response:', response);

        if (response && response.message && response.message.content) {
            let content = response.message.content;
            // 尝试解析为数组
            let keywordsArr = [];
            try {
                // 兼容单引号和双引号
                content = content.replace(/'/g, '"');
                keywordsArr = JSON.parse(content);
            } catch (e) {
                // 解析失败，尝试用逗号分割
                keywordsArr = content.replace(/[\[\]'" ]/g, '').split(',');
            }
            // 过滤空项并拼接
            return keywordsArr.filter(k => k && k.trim()).join(', ');
        } else {
            this.showError('关键词提取失败，请重试');
            return '';
        }
    }

    /**
    * 生成图片的base64编码（不带头部）
    * @param {string} imageUrl - 图片URL
    * @returns {Promise<string>} - Base64编码字符串（无头部）
    */
    async generateImageBase64(imageUrl) {
        try {
            const response = await fetch(imageUrl);
            const blob = await response.blob();
            const reader = new FileReader();
            reader.readAsDataURL(blob);
            return new Promise((resolve) => {
                reader.onloadend = () => {
                    // 去掉头部，只保留base64数据
                    const base64 = reader.result;
                    const pureBase64 = base64.replace(/^data:image\/\w+;base64,/, '');
                    resolve(pureBase64);
                };
            });
        } catch (error) {
            console.error('[AOGContentCreator] Error generating image base64:', error);
            throw new Error('Image base64 generation failed');
        }
    }

    /**
     * 重置应用状态
     */
    reset() {
        console.log('[AOGContentCreator] Resetting application...');

        // 重置状态
        this.currentStep = 1;
        this.generatedContent = {
            speechText: '',
            chatResponse: '',
            localImages: [],
            optimizationText: '',
            cloudImages: [],
            audioUrl: ''
        };

        // 清理UI
        if (this.chatMessages) {
            this.chatMessages.innerHTML = `
                <div class="welcome-message">
                    <div class="message-icon">🤖</div>
                    <div class="message-text">
                        你好！我是AOG智能助手。请通过语音告诉我你想要创作的内容，我会为你生成精彩的文案和配图。
                    </div>
                </div>
            `;
        }

        if (this.localImages) this.localImages.innerHTML = '<div class="placeholder">等待生成...</div>';
        if (this.cloudImages) this.cloudImages.innerHTML = '<div class="placeholder">等待优化指令...</div>';
        if (this.audioPlayer) this.audioPlayer.style.display = 'none';

        // 重置按钮状态
        if (this.optimizeBtn) this.optimizeBtn.disabled = true;
        if (this.generateAudioBtn) this.generateAudioBtn.disabled = true;

        // 更新UI
        this.updateStepIndicator();
        this.updateSectionStatus();

        // 重置性能指标
        this.statusMonitor.resetMetrics();
    }

    /**
     * 清理资源
     */
    cleanup() {
        console.log('[AOGContentCreator] Cleaning up application...');

        this.audioRecorder?.cleanup();
        this.cameraCapture?.cleanup();
        this.streamHandler?.cleanup();
        this.statusMonitor?.cleanup();
    }

    initializeStep2() {
        console.log('[AOGContentCreator] Initializing step 2...');
        const speechTextElement = document.getElementById('user-input-display');
        if (speechTextElement) {
            speechTextElement.textContent = this.generatedContent.speechText;
        }
        this.setCurrentStep(2);

        // 绑定“重新生成”按钮事件
        const regenBtn = document.getElementById('regenerate-content-btn');
        if (regenBtn) {
            regenBtn.onclick = () => this.regenerateContent();
        }

        const chatModel = document.getElementById('chat-model');
        const model = chatModel?.value || defaultChatModel;
        this.generateChatResponse(this.generatedContent.speechText, model);
    }

    initializeStep3() {}

    async initializeStep4() {
        console.log('[AOGContentCreator] Initializing step 4...');
        this.setCurrentStep(4);

        // 1. 获取关键词
        let keywords = '';
        try {
            keywords = await this.getKeyword();
        } catch (e) {
            this.showError('关键词提取失败，请重试');
            return;
        }

        // 2. 展示关键词到页面
        const keywordsSpan = document.getElementById('extracted-keywords');
        if (keywordsSpan) {
            keywordsSpan.textContent = keywords || '未提取到关键词';
        }

        // 3. 用关键词作为prompt生成本地图片
        this.generateLocalImages(keywords);
    }

    initializeStep5() {
        console.log('[AOGContentCreator] Initializing step 5...');
        this.setCurrentStep(5);

        const exampleTags = document.querySelectorAll('.example-tag');
        exampleTags.forEach(tag => {
            tag.onclick = () => {
                const text = tag.textContent.trim();
                const optimizationTextArea = document.getElementById('optimization-text');
                if (optimizationTextArea) {
                    optimizationTextArea.value = text;
                }
                this.generatedContent.optimizationText = text;
            };
        });

        // 只展示选中的图片
        const previewGrid = document.getElementById('preview-images');
        if (previewGrid) {
            previewGrid.innerHTML = '';
            const imgUrl = this.generatedContent.cameraImage || this.selectedLocalImage;
            if (imgUrl) {
                const img = document.createElement('img');
                img.src = imgUrl;
                img.alt = `预览图`;
                previewGrid.appendChild(img);
            }
        }

        // 重置优化指令显示
        const optimizationTextArea = document.getElementById('optimization-text');
        if (optimizationTextArea) {
            optimizationTextArea.value = '等待语音输入...';
        }

        // 启用/禁用按钮
        if (this.optimizeBtn) {
            this.optimizeBtn.disabled = false;
        }
        const retryBtn = document.getElementById('retry-optimization-btn');
        if (retryBtn) retryBtn.style.display = 'none';

        // 绑定录音按钮事件
        const recordBtn = document.getElementById('optimize-record-btn');
        if (recordBtn) {
            recordBtn.onclick = () => this.toggleVoiceOptimization();
        }

        // 录音动画隐藏
        const indicator = document.getElementById('optimize-recording-indicator');
        if (indicator) indicator.style.display = 'none';

        this.isOptimizing = false;
        this.updateSectionStatus('voice-optimization-section', '请用语音描述优化方向', 'active');
    }

    initializeStep6() {
        console.log('[AOGContentCreator] Initializing step 6...');
        this.setCurrentStep(6);
        // 展示优化指令
        const appliedOpt = document.getElementById('applied-optimization');
        if (appliedOpt) {
            appliedOpt.textContent = this.generatedContent.optimizationText || '-';
        }

        // 只显示选中的图片
        const localComparison = document.getElementById('local-comparison');
        if (localComparison) {
            localComparison.innerHTML = '';
            const imgUrl = this.generatedContent.cameraImage || this.selectedLocalImage;
            if (imgUrl) {
                const img = document.createElement('img');
                img.src = imgUrl;
                img.alt = '本地预览图';
                localComparison.appendChild(img);
            }
        }
        this.generateCloudImages();
    }

    async initializeStep7() {
        console.log('[AOGContentCreator] Initializing step 7...');
        this.setCurrentStep(7);

        // 展示最终图片
        const finalImage = document.getElementById('final-image-preview');
        if (finalImage) {
            finalImage.innerHTML = '';
            const imgUrl = (this.generatedContent.cloudImages && this.generatedContent.cloudImages[0]) || '';
            if (imgUrl) {
                const img = document.createElement('img');
                img.src = imgUrl;
                img.alt = '最终图片';
                finalImage.appendChild(img);
            }
        }
        const finalContent = document.getElementById('final-content-preview');
        if (finalContent) {
            finalContent.textContent = "等待生成";
        }

        // 1. 获取云端图片的 base64 编码
        const cloudImageUrl = this.generatedContent.cloudImages[0];
        const base64Image = await this.generateImageBase64(cloudImageUrl);
        // 2. 请求生成诗句
        const generateRequest = {
            images: [base64Image],
            prompt: generatePrompt,
            model: defaultGenerateModel,
            stream: false
        };
        let poemText = '';
        try {
            const response = await fetch('/api/generate', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(generateRequest)
            });
            const result = await response.json();
            console.log("[Generate] Result:", result)
            poemText = result.message.response || '-';
        } catch (e) {
            poemText = '诗句生成失败';
        }

        // 3. 展示诗句
        if (finalContent) {
            finalContent.textContent = poemText;
        }

        const completeBtn = document.getElementById('complete-workflow-btn');
        if (completeBtn) {
            completeBtn.disabled = false;
        }


        // 4. 请求语音播报
        const generateAudioBtn = document.getElementById('generate-audio-btn');
        if (generateAudioBtn) {
            generateAudioBtn.onclick = async () => {
                // 获取诗句文本
                const poemText = document.getElementById('final-content-preview')?.textContent || '';
                if (!poemText.trim()) {
                    this.showError('请先生成诗句');
                    return;
                }
                await this.generateAudio(poemText);
            };
        }


        // 绑定下载按钮
        const downloadBtn = document.getElementById('download-results-btn');
        if (downloadBtn) {
            downloadBtn.onclick = () => {
                // 获取最终云端精图的URL
                const imageUrl = (this.generatedContent.cloudImages && this.generatedContent.cloudImages[0]) || '';
                if (!imageUrl) {
                    this.showError('没有可下载的图片');
                    return;
                }
                // 创建隐藏a标签并触发下载
                const a = document.createElement('a');
                a.href = imageUrl;
                a.download = 'final-image.png'; // 可自定义文件名
                document.body.appendChild(a);
                a.click();
                document.body.removeChild(a);
            };
        }
    }

    completeStep2(){
        console.log('[AOGContentCreator] Completing step 2...');
        this.markStepCompleted(2);
        const confirmBtn = document.getElementById('confirm-content-btn');
        if (confirmBtn && this.generatedContent.chatResponse.trim()) {
            confirmBtn.disabled = false;
        }
        this.confirmContentAndNext();
    }


}

// 应用启动
document.addEventListener('DOMContentLoaded', () => {
    console.log('[AOGContentCreator] DOM loaded, starting application...');
    window.aogApp = new AOGContentCreator();
});
