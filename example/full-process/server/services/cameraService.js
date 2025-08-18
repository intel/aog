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

const fs = require('fs');
const path = require('path');

class CameraService {
    constructor() {
        this.serverUrl = 'http://localhost:3000';
        this.maxFileSize = 10 * 1024 * 1024; // 10MB
        this.allowedFormats = ['jpg', 'jpeg', 'png', 'webp'];
    }

    /**
     * 处理摄像头拍摄的图片
     * @param {Object} file - multer文件对象
     * @returns {Promise<Object>} 处理结果
     */
    async processCameraImage(file) {
        if (!file) {
            throw new Error('No image file provided');
        }

        console.log(`[CameraService] Processing camera image: ${file.filename}`);

        try {
            // 验证文件
            this.validateImageFile(file);

            // 生成图片URL
            const imageUrl = `${this.serverUrl}/uploads/images/${file.filename}`;

            // 获取图片信息
            const imageInfo = await this.getImageInfo(file.path);

            return {
                success: true,
                image_url: imageUrl,
                filename: file.filename,
                metadata: {
                    original_name: file.originalname,
                    size: file.size,
                    format: path.extname(file.originalname).toLowerCase().slice(1),
                    dimensions: imageInfo.dimensions,
                    upload_time: new Date().toISOString()
                }
            };
        } catch (error) {
            // 如果处理失败，删除已上传的文件
            if (file.path && fs.existsSync(file.path)) {
                try {
                    fs.unlinkSync(file.path);
                    console.log(`[CameraService] Cleaned up failed upload: ${file.path}`);
                } catch (cleanupError) {
                    console.error('[CameraService] Cleanup error:', cleanupError);
                }
            }
            
            console.error('[CameraService] Image processing error:', error);
            throw new Error(`Camera image processing failed: ${error.message}`);
        }
    }

    /**
     * 验证图片文件
     * @param {Object} file - 文件对象
     */
    validateImageFile(file) {
        // 检查文件大小
        if (file.size > this.maxFileSize) {
            throw new Error(`File size too large. Maximum allowed: ${this.maxFileSize / 1024 / 1024}MB`);
        }

        // 检查文件格式
        const fileExtension = path.extname(file.originalname).toLowerCase().slice(1);
        if (!this.allowedFormats.includes(fileExtension)) {
            throw new Error(`Unsupported file format. Allowed formats: ${this.allowedFormats.join(', ')}`);
        }

        // 检查MIME类型
        if (!file.mimetype.startsWith('image/')) {
            throw new Error('File is not a valid image');
        }
    }

    /**
     * 获取图片信息
     * @param {string} imagePath - 图片路径
     * @returns {Promise<Object>} 图片信息
     */
    async getImageInfo(imagePath) {
        try {
            const stats = fs.statSync(imagePath);
            
            // 这里可以使用图片处理库获取更详细的信息
            // 为了保持简单，我们只返回基本信息
            return {
                dimensions: {
                    width: null, // 可以使用sharp或jimp库获取
                    height: null
                },
                file_size: stats.size,
                created_time: stats.birthtime,
                modified_time: stats.mtime
            };
        } catch (error) {
            console.error('[CameraService] Get image info error:', error);
            return {
                dimensions: { width: null, height: null },
                file_size: 0,
                created_time: new Date(),
                modified_time: new Date()
            };
        }
    }

    /**
     * 生成图片描述prompt
     * @param {string} imageUrl - 图片URL
     * @param {string} userDescription - 用户描述
     * @returns {string} 生成的prompt
     */
    generateImagePrompt(imageUrl, userDescription = '') {
        // 这里可以集成图像识别服务来自动生成描述
        // 目前返回基础的prompt模板
        
        const basePrompt = "Based on the captured image";
        
        if (userDescription && userDescription.trim() !== '') {
            return `${basePrompt}, ${userDescription}`;
        }
        
        return `${basePrompt}, create a detailed and artistic description`;
    }

    /**
     * 清理过期的摄像头图片
     * @param {number} maxAge - 最大保存时间(毫秒)
     */
    cleanupOldCameraImages(maxAge = 24 * 60 * 60 * 1000) { // 默认24小时
        const cameraDir = path.join(__dirname, '../uploads/images');
        
        if (!fs.existsSync(cameraDir)) {
            return;
        }

        try {
            const files = fs.readdirSync(cameraDir);
            const now = Date.now();
            let cleanedCount = 0;

            files.forEach(file => {
                const filePath = path.join(cameraDir, file);
                const stats = fs.statSync(filePath);
                
                if (now - stats.mtime.getTime() > maxAge) {
                    fs.unlinkSync(filePath);
                    cleanedCount++;
                    console.log(`[CameraService] Cleaned up old camera image: ${file}`);
                }
            });

            if (cleanedCount > 0) {
                console.log(`[CameraService] Cleaned up ${cleanedCount} old camera images`);
            }
        } catch (error) {
            console.error('[CameraService] Error cleaning up old camera images:', error);
        }
    }

    /**
     * 获取摄像头图片列表
     * @param {number} limit - 返回数量限制
     * @returns {Array} 图片列表
     */
    getCameraImageList(limit = 10) {
        const cameraDir = path.join(__dirname, '../uploads/images');
        
        if (!fs.existsSync(cameraDir)) {
            return [];
        }

        try {
            const files = fs.readdirSync(cameraDir);
            const imageList = [];

            files
                .filter(file => {
                    const ext = path.extname(file).toLowerCase().slice(1);
                    return this.allowedFormats.includes(ext);
                })
                .sort((a, b) => {
                    // 按修改时间倒序排列
                    const aPath = path.join(cameraDir, a);
                    const bPath = path.join(cameraDir, b);
                    const aStat = fs.statSync(aPath);
                    const bStat = fs.statSync(bPath);
                    return bStat.mtime.getTime() - aStat.mtime.getTime();
                })
                .slice(0, limit)
                .forEach(file => {
                    const filePath = path.join(cameraDir, file);
                    const stats = fs.statSync(filePath);
                    
                    imageList.push({
                        filename: file,
                        url: `${this.serverUrl}/uploads/images/${file}`,
                        size: stats.size,
                        created_time: stats.birthtime,
                        modified_time: stats.mtime
                    });
                });

            return imageList;
        } catch (error) {
            console.error('[CameraService] Error getting camera image list:', error);
            return [];
        }
    }

    /**
     * 删除摄像头图片
     * @param {string} filename - 文件名
     * @returns {boolean} 删除是否成功
     */
    deleteCameraImage(filename) {
        const cameraDir = path.join(__dirname, '../uploads/images');
        const filePath = path.join(cameraDir, filename);

        try {
            if (fs.existsSync(filePath)) {
                fs.unlinkSync(filePath);
                console.log(`[CameraService] Deleted camera image: ${filename}`);
                return true;
            } else {
                console.warn(`[CameraService] Camera image not found: ${filename}`);
                return false;
            }
        } catch (error) {
            console.error(`[CameraService] Error deleting camera image ${filename}:`, error);
            return false;
        }
    }
}

module.exports = new CameraService();
