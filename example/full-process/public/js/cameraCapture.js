/**
 * AOG智能内容创作助手 - 摄像头拍摄模块
 * Copyright 2024-2025 Intel Corporation
 */

class CameraCapture {
    constructor() {
        this.stream = null;
        this.video = null;
        this.canvas = null;
        this.context = null;
        this.isActive = false;
        this.capturedImageUrl = null;
        
        // 摄像头配置
        this.cameraConfig = {
            video: {
                width: { ideal: 1280 },
                height: { ideal: 720 },
                facingMode: 'user' // 前置摄像头
            },
            audio: false
        };
        
        this.callbacks = {
            onImageCaptured: null,
            onError: null,
            onStatusChange: null
        };
        
        this.initializeElements();
    }

    /**
     * 初始化DOM元素
     */
    initializeElements() {
        this.video = document.getElementById('camera-video');
        this.canvas = document.getElementById('camera-canvas');
        this.startBtn = document.getElementById('start-camera-btn');
        this.captureBtn = document.getElementById('capture-btn');
        this.stopBtn = document.getElementById('stop-camera-btn');
        this.retakeBtn = document.getElementById('retake-btn');
        this.useImageBtn = document.getElementById('use-image-btn');
        this.capturedImage = document.getElementById('captured-image');
        this.capturedImg = document.getElementById('captured-img');
        this.cameraStatus = document.getElementById('camera-status');
        this.cameraPlaceholder = document.getElementById('camera-placeholder');
        
        if (this.canvas) {
            this.context = this.canvas.getContext('2d');
        }
        
        this.bindEvents();
    }

    /**
     * 绑定事件
     */
    bindEvents() {
        if (this.startBtn) {
            this.startBtn.addEventListener('click', () => this.startCamera());
        }
        
        if (this.cameraPlaceholder) {
            this.cameraPlaceholder.addEventListener('click', () => this.startCamera());
        }
        if (this.captureBtn) {
            this.captureBtn.addEventListener('click', () => this.capturePhoto());
        }
        
        if (this.stopBtn) {
            this.stopBtn.addEventListener('click', () => this.stopCamera());
        }
        
        if (this.retakeBtn) {
            this.retakeBtn.addEventListener('click', () => this.retakePhoto());
        }
        
        if (this.useImageBtn) {
            this.useImageBtn.addEventListener('click', () => this.useImage());
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
     * 启动摄像头
     */
    async startCamera() {
        try {
            console.log('[CameraCapture] Starting camera...');
            this.updateStatus('正在启动摄像头...', 'active');
            
            this.stream = await navigator.mediaDevices.getUserMedia(this.cameraConfig);
            
            if (this.video) {
                this.video.srcObject = this.stream;
                await this.video.play();
            }
            
            this.isActive = true;
            this.updateUI(true, false);
            this.updateStatus('摄像头已启动', 'active');
            
            console.log('[CameraCapture] Camera started successfully');
            
        } catch (error) {
            console.error('[CameraCapture] Failed to start camera:', error);
            this.handleError('无法启动摄像头，请检查权限设置');
        }
    }

    /**
     * 停止摄像头
     */
    stopCamera() {
        try {
            console.log('[CameraCapture] Stopping camera...');
            
            if (this.stream) {
                this.stream.getTracks().forEach(track => track.stop());
                this.stream = null;
            }
            
            if (this.video) {
                this.video.srcObject = null;
            }
            
            this.isActive = false;
            this.updateUI(false, false);
            this.updateStatus('摄像头已关闭');
            
            console.log('[CameraCapture] Camera stopped');
            
        } catch (error) {
            console.error('[CameraCapture] Failed to stop camera:', error);
            this.handleError('关闭摄像头失败');
        }
    }

    /**
     * 拍摄照片
     */
    async capturePhoto() {
        try {
            if (!this.isActive || !this.video || !this.canvas || !this.context) {
                throw new Error('Camera not ready');
            }
            
            console.log('[CameraCapture] Capturing photo...');
            this.updateStatus('正在拍摄...', 'active');
            
            // 设置canvas尺寸
            const videoWidth = this.video.videoWidth;
            const videoHeight = this.video.videoHeight;
            
            this.canvas.width = videoWidth;
            this.canvas.height = videoHeight;
            
            // 绘制当前视频帧到canvas
            this.context.drawImage(this.video, 0, 0, videoWidth, videoHeight);
            
            // 转换为Blob
            const imageBlob = await this.canvasToBlob();
            
            // 上传图片
            const result = await this.uploadImage(imageBlob);
            
            if (result.success) {
                this.capturedImageUrl = result.image_url;
                this.showCapturedImage(result.image_url);
                this.updateStatus('照片拍摄成功', 'completed');
                
                console.log('[CameraCapture] Photo captured successfully:', result.image_url);
            } else {
                throw new Error('Upload failed');
            }
            
        } catch (error) {
            console.error('[CameraCapture] Failed to capture photo:', error);
            this.handleError('拍摄照片失败');
        }
    }

    /**
     * 将canvas转换为Blob
     * @returns {Promise<Blob>} 图片Blob
     */
    canvasToBlob() {
        return new Promise((resolve, reject) => {
            this.canvas.toBlob((blob) => {
                if (blob) {
                    resolve(blob);
                } else {
                    reject(new Error('Failed to convert canvas to blob'));
                }
            }, 'image/jpeg', 0.8);
        });
    }

    /**
     * 上传图片到服务器
     * @param {Blob} imageBlob - 图片Blob
     * @returns {Promise<Object>} 上传结果
     */
    async uploadImage(imageBlob) {
        const formData = new FormData();
        formData.append('image', imageBlob, 'camera-capture.jpg');
        formData.append('type', 'camera');
        
        const response = await fetch('/api/upload/camera', {
            method: 'POST',
            body: formData
        });
        
        if (!response.ok) {
            throw new Error(`Upload failed: ${response.status}`);
        }
        
        return await response.json();
    }

    /**
     * 显示拍摄的照片
     * @param {string} imageUrl - 图片URL
     */
    showCapturedImage(imageUrl) {
        // 找到虚线框和按钮区域
        this.cameraPlaceholder = document.getElementById('camera-placeholder');
        this.capturedImage = document.getElementById('captured-image');
        if (this.cameraPlaceholder) {
            // 清空虚线框内容，只插入图片
            this.cameraPlaceholder.innerHTML = '';
            const img = document.createElement('img');
            img.src = imageUrl;
            img.alt = '拍摄的照片';
            img.style.maxWidth = '100%';
            img.style.maxHeight = '100%';
            img.style.borderRadius = '8px';
            img.style.objectFit = 'contain';
            this.cameraPlaceholder.appendChild(img);
        }
        // 显示按钮在虚线框下方
        if (this.capturedImage) {
            this.capturedImage.style.display = 'block';
            const retakeBtn = document.getElementById('retake-btn');
            const useBtn = document.getElementById('use-image-btn');
            if (retakeBtn) retakeBtn.onclick = () => this.retakePhoto();
            if (useBtn) useBtn.onclick = () => this.useImage(imageUrl);
        }
        this.updateUI(true, true);
    }

    /**
     * 重新拍摄
     */
    retakePhoto() {
        console.log('[CameraCapture] Retaking photo...');
        
        if (this.capturedImage) {
            this.capturedImage.style.display = 'none';
        }
        
        this.capturedImageUrl = null;
        this.updateUI(true, false);
        this.updateStatus('请重新拍摄', 'active');
    }

    /**
     * 使用当前图片
     */
    useImage() {
        if (!this.capturedImageUrl) {
            this.handleError('没有可用的图片');
            return;
        }
        
        console.log('[CameraCapture] Using captured image:', this.capturedImageUrl);
        
        // 调用回调函数
        if (this.callbacks.onImageCaptured) {
            this.callbacks.onImageCaptured(this.capturedImageUrl);
        }
        
        this.updateStatus('图片已使用', 'completed');
        
        // 可选：关闭摄像头以节省资源
        // this.stopCamera();
    }

    /**
     * 更新UI状态
     * @param {boolean} cameraActive - 摄像头是否激活
     * @param {boolean} imageCaptured - 是否已拍摄图片
     */
    updateUI(cameraActive, imageCaptured) {
        // 更新按钮状态
        if (this.startBtn) {
            this.startBtn.disabled = cameraActive;
        }
        
        if (this.captureBtn) {
            this.captureBtn.disabled = !cameraActive || imageCaptured;
            this.captureBtn.style.display = cameraActive && !imageCaptured ? 'inline-block' : 'none';
        }
        
        if (this.stopBtn) {
            this.stopBtn.disabled = !cameraActive;
        }
        
        if (this.retakeBtn) {
            this.retakeBtn.disabled = !imageCaptured;
        }
        
        if (this.useImageBtn) {
            this.useImageBtn.disabled = !imageCaptured;
        }
    }

    /**
     * 更新状态显示
     * @param {string} message - 状态消息
     * @param {string} type - 状态类型
     */
    updateStatus(message, type = '') {
        if (this.cameraStatus) {
            this.cameraStatus.textContent = message;
            this.cameraStatus.className = 'section-status';
            if (type) {
                this.cameraStatus.classList.add(type);
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
        console.error('[CameraCapture] Error:', message);
        this.updateStatus(message, 'error');
        
        if (this.callbacks.onError) {
            this.callbacks.onError(message);
        }
    }

    /**
     * 获取拍摄的图片URL
     * @returns {string|null} 图片URL
     */
    getCapturedImageUrl() {
        return this.capturedImageUrl;
    }

    /**
     * 检查摄像头是否可用
     * @returns {Promise<boolean>} 是否可用
     */
    async checkCameraAvailability() {
        try {
            const devices = await navigator.mediaDevices.enumerateDevices();
            const videoDevices = devices.filter(device => device.kind === 'videoinput');
            return videoDevices.length > 0;
        } catch (error) {
            console.error('[CameraCapture] Failed to check camera availability:', error);
            return false;
        }
    }

    /**
     * 切换摄像头（前置/后置）
     */
    async switchCamera() {
        if (!this.isActive) {
            return;
        }
        
        try {
            // 切换facingMode
            const currentFacingMode = this.cameraConfig.video.facingMode;
            this.cameraConfig.video.facingMode = currentFacingMode === 'user' ? 'environment' : 'user';
            
            // 重新启动摄像头
            this.stopCamera();
            await new Promise(resolve => setTimeout(resolve, 500)); // 等待一下
            await this.startCamera();
            
        } catch (error) {
            console.error('[CameraCapture] Failed to switch camera:', error);
            this.handleError('切换摄像头失败');
        }
    }

    /**
     * 清理资源
     */
    cleanup() {
        this.stopCamera();
        this.capturedImageUrl = null;
        
        if (this.capturedImage) {
            this.capturedImage.style.display = 'none';
        }
        
        this.updateUI(false, false);
        this.updateStatus('摄像头已清理');
    }
}

// 导出类
window.CameraCapture = CameraCapture;
