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
const fs = require('fs');
const path = require('path');
const https = require('https');
const http = require('http');
const { url } = require('inspector');
const constants = require('../../constants')

class ImageService {
    constructor() {
        this.aog = new AogLib();
        this.serverUrl = 'http://localhost:3000';
        this.localModel = constants.defaultTextToImageModel;
        this.remoteModel = constants.defaultImageToImageModel;
        this.localSize = constants.defaultLocalSize;
        this.remoteSize = constants.defaultRemoteSize;
    }

    generateImageLocalMock(requestData) {
        const {
            prompt,
            size = '1024*1024',
            n = 1,
            model = 'wanx2.0-t2i-turbo',
            image_type = "path"
        } = requestData;

        console.log(`[ImageService] Mock Local generation - Prompt: ${prompt}, Size: ${size}, Count: ${n}`);

        const res = {
            code: 200,
            images: [
                "/uploads/images/local-1754301650045-1.png",
                "/uploads/images/local-1754301650878-2.png"
            ]
        }

        return res;
    }

    generateImageCloudMock(requestData) {
        const {
            prompt,
            size = '1024*1024',
            n = 1,
            model = 'wanx2.0-t2i-turbo',
            image,
            image_type = "path"
        } = requestData;

        console.log(`[ImageService] Mock Cloud generation - Prompt: ${prompt}, Size: ${size}, image: ${image}`);
        const res = {
            code: 200,
            images: [
                "/uploads/images/local-1754467752056-1.png"
            ]
        };

        return res;
    }

    /**
     * 本地图片生成 (OpenVINO)
     * @param {Object} requestData - 请求数据
     * @returns {Promise<Object>} 生成结果
     */
    async generateImageLocal(requestData) {
        const {
            prompt,
            size = this.localSize,
            n = 2,
            model = this.localModel,
        } = requestData;

        console.log(`[ImageService] Local generation - Prompt: ${prompt}, Size: ${size}, Count: ${n}`);

        const imageData = {
            model: model,
            prompt: prompt,
            size: size,
            n: n,
        };

        try {
            const startTime = Date.now();
            const result = await this.aog.textToImage(imageData);
            console.log(`[ImageService] Local generation result: ${JSON.stringify(result)}`);
            const responseTime = Date.now() - startTime;

            console.log(`[ImageService] Local generation completed in ${responseTime}ms`);

            if (result.code !== 200 || !result.data) {
                throw new Error('Invalid response from local image generation service');
            }

            const localPaths = result.data.url;
            const imageUrls = await this.processLocalImages(localPaths, 'local');

            return {
                success: true,
                images: imageUrls,
                metadata: {
                    model: model,
                    prompt: prompt,
                    size: size,
                    count: imageUrls.length,
                    provider: 'local',
                    response_time: responseTime
                }
            };
        } catch (error) {
            console.error('[ImageService] Local generation error:', error);
            throw new Error(`Local image generation failed: ${error.message}`);
        }
    }

    /**
     * 云端图片生成 (阿里云)
     * @param {Object} requestData - 请求数据
     * @returns {Promise<Object>} 生成结果
     */
    async generateImageCloud(requestData) {
        const {
            prompt,
            model = this.remoteModel,
            image_type = "path",
            image,
        } = requestData;

        console.log(`[ImageService] Cloud generation - Prompt: ${prompt}, Model: ${model}, Image: ${image}`);

        const imageData = {
            model: model,
            prompt: prompt,
            image: image || null,
            image_type: image_type
        };

        try {
            const startTime = Date.now();
            const result = await this.aog.imageToImage(imageData);
            const responseTime = Date.now() - startTime;

            console.log(`[ImageService] Cloud generation completed`, result);

            if (result.code !== 200 || !result.data) {
                throw new Error('Invalid response from cloud image generation service');
            }

            const cloudUrls = result.data.url;
            const imageUrls = await this.processLocalImages(cloudUrls, 'local');

            return {
                success: true,
                images: imageUrls,
                metadata: {
                    model: model,
                    prompt: prompt,
                    count: imageUrls.length,
                    provider: 'cloud',
                    response_time: responseTime
                }
            };
        } catch (error) {
            console.error('[ImageService] Cloud generation error:', error);
            throw new Error(`Cloud image generation failed: ${error.message}`);
        }
    }

    /**
     * 处理本地生成的图片
     * @param {Array} paths - 图片路径或URL数组
     * @param {string} prefix - 文件名前缀
     * @returns {Promise<Array>} 处理后的图片URL数组
     */
    async processLocalImages(paths, prefix) {
        const imageUrls = [];
        const imagesDir = path.join(__dirname, '../uploads/images');

        // 确保目标目录存在
        if (!fs.existsSync(imagesDir)) {
            fs.mkdirSync(imagesDir, { recursive: true });
        }

        for (let i = 0; i < paths.length; i++) {
            const imagePath = paths[i];
            
            try {
                // 检查是否为URL
                if (imagePath.startsWith('http://') || imagePath.startsWith('https://')) {
                    // 处理远程URL
                    const fileName = `${prefix}-${Date.now()}-${i + 1}.png`;
                    const targetPath = path.join(imagesDir, fileName);
                    
                    // 下载图片到本地
                    await this.downloadFile(imagePath, targetPath);
                    
                    const imageUrl = `${this.serverUrl}/uploads/images/${fileName}`;
                    imageUrls.push(imageUrl);
                    
                    console.log(`[ImageService] Downloaded image: ${imagePath} -> ${targetPath}`);
                } else if (fs.existsSync(imagePath)) {
                    // 处理本地路径
                    const fileName = `${prefix}-${Date.now()}-${i + 1}${path.extname(imagePath)}`;
                    const targetPath = path.join(imagesDir, fileName);
                    
                    // 移动文件到uploads目录
                    fs.copyFileSync(imagePath, targetPath);
                    
                    const imageUrl = `${this.serverUrl}/uploads/images/${fileName}`;
                    imageUrls.push(imageUrl);
                    
                    console.log(`[ImageService] Moved local image: ${imagePath} -> ${targetPath}`);
                } else {
                    console.warn(`[ImageService] Image path not valid: ${imagePath}`);
                    // 如果路径无效，直接使用原始路径
                    imageUrls.push(imagePath);
                }
            } catch (error) {
                console.error(`[ImageService] Error processing image ${imagePath}:`, error);
                // 发生错误时，直接使用原始路径
                imageUrls.push(imagePath);
            }
        }

        return imageUrls;
    }

    /**
     * 下载文件，带重试机制
     * @param {string} url - 文件URL
     * @param {string} targetPath - 目标路径
     * @param {number} retries - 最大重试次数
     * @returns {Promise<void>}
     */
    async downloadFile(url, targetPath, retries = 3) {
        for (let attempt = 1; attempt <= retries; attempt++) {
            try {
                await new Promise((resolve, reject) => {
                    const protocol = url.startsWith('https:') ? https : http;
                    const file = fs.createWriteStream(targetPath);

                    const request = protocol.get(url, (response) => {
                        if (response.statusCode !== 200) {
                            reject(new Error(`HTTP ${response.statusCode}: ${response.statusMessage}`));
                            return;
                        }

                        response.pipe(file);

                        file.on('finish', () => {
                            file.close();
                            resolve();
                        });

                        file.on('error', (error) => {
                            fs.unlink(targetPath, () => {});
                            reject(error);
                        });
                    });

                    request.on('error', (error) => {
                        reject(error);
                    });

                    request.setTimeout(120000, () => {
                        request.abort();
                        reject(new Error('Download timeout'));
                    });
                });
                // 下载成功，直接返回
                return;
            } catch (error) {
                console.error(`[ImageService] Download attempt ${attempt} failed:`, error);
                if (attempt < retries) {
                    // 等待2秒后重试
                    await new Promise(res => setTimeout(res, 2000));
                } else {
                    // 达到最大重试次数，抛出错误
                    throw error;
                }
            }
        }
    }

    /**
     * 合并优化指令到原始prompt
     * @param {string} originalPrompt - 原始prompt
     * @param {string} optimizationText - 优化指令
     * @returns {string} 合并后的prompt
     */
    mergePrompts(originalPrompt, optimizationText) {
        if (!optimizationText || optimizationText.trim() === '') {
            return originalPrompt;
        }

        // 简单的prompt合并逻辑
        return `${originalPrompt}, ${optimizationText}`;
    }

    /**
     * 清理过期的图片文件
     * @param {number} maxAge - 最大保存时间(毫秒)
     */
    cleanupOldImages(maxAge = 24 * 60 * 60 * 1000) { // 默认24小时
        const imagesDir = path.join(__dirname, '../uploads/images');
        
        if (!fs.existsSync(imagesDir)) {
            return;
        }

        try {
            const files = fs.readdirSync(imagesDir);
            const now = Date.now();

            files.forEach(file => {
                const filePath = path.join(imagesDir, file);
                const stats = fs.statSync(filePath);
                
                if (now - stats.mtime.getTime() > maxAge) {
                    fs.unlinkSync(filePath);
                    console.log(`[ImageService] Cleaned up old image: ${file}`);
                }
            });
        } catch (error) {
            console.error('[ImageService] Error cleaning up old images:', error);
        }
    }
}

module.exports = new ImageService();
