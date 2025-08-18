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

class FileManager {
    constructor() {
        this.uploadBasePath = path.join(__dirname, '../uploads');
        this.maxFileAge = 24 * 60 * 60 * 1000; // 24小时
    }

    /**
     * 确保目录存在
     * @param {string} dirPath - 目录路径
     */
    ensureDirectoryExists(dirPath) {
        if (!fs.existsSync(dirPath)) {
            fs.mkdirSync(dirPath, { recursive: true });
            console.log(`[FileManager] Created directory: ${dirPath}`);
        }
    }

    /**
     * 初始化上传目录
     */
    initializeUploadDirectories() {
        const directories = [
            path.join(this.uploadBasePath, 'images'),
            path.join(this.uploadBasePath, 'audio'),
            path.join(this.uploadBasePath, 'camera')
        ];

        directories.forEach(dir => {
            this.ensureDirectoryExists(dir);
        });

        console.log('[FileManager] Upload directories initialized');
    }

    /**
     * 生成唯一文件名
     * @param {string} originalName - 原始文件名
     * @param {string} prefix - 文件名前缀
     * @returns {string} 唯一文件名
     */
    generateUniqueFilename(originalName, prefix = '') {
        const timestamp = Date.now();
        const random = Math.round(Math.random() * 1E9);
        const extension = path.extname(originalName);
        const baseName = path.basename(originalName, extension);
        
        return `${prefix}${prefix ? '-' : ''}${baseName}-${timestamp}-${random}${extension}`;
    }

    /**
     * 移动文件
     * @param {string} sourcePath - 源文件路径
     * @param {string} targetPath - 目标文件路径
     * @returns {boolean} 移动是否成功
     */
    moveFile(sourcePath, targetPath) {
        try {
            // 确保目标目录存在
            const targetDir = path.dirname(targetPath);
            this.ensureDirectoryExists(targetDir);

            // 移动文件
            fs.renameSync(sourcePath, targetPath);
            console.log(`[FileManager] Moved file: ${sourcePath} -> ${targetPath}`);
            return true;
        } catch (error) {
            console.error(`[FileManager] Error moving file: ${sourcePath} -> ${targetPath}`, error);
            return false;
        }
    }

    /**
     * 复制文件
     * @param {string} sourcePath - 源文件路径
     * @param {string} targetPath - 目标文件路径
     * @returns {boolean} 复制是否成功
     */
    copyFile(sourcePath, targetPath) {
        try {
            // 确保目标目录存在
            const targetDir = path.dirname(targetPath);
            this.ensureDirectoryExists(targetDir);

            // 复制文件
            fs.copyFileSync(sourcePath, targetPath);
            console.log(`[FileManager] Copied file: ${sourcePath} -> ${targetPath}`);
            return true;
        } catch (error) {
            console.error(`[FileManager] Error copying file: ${sourcePath} -> ${targetPath}`, error);
            return false;
        }
    }

    /**
     * 删除文件
     * @param {string} filePath - 文件路径
     * @returns {boolean} 删除是否成功
     */
    deleteFile(filePath) {
        try {
            if (fs.existsSync(filePath)) {
                fs.unlinkSync(filePath);
                console.log(`[FileManager] Deleted file: ${filePath}`);
                return true;
            } else {
                console.warn(`[FileManager] File not found: ${filePath}`);
                return false;
            }
        } catch (error) {
            console.error(`[FileManager] Error deleting file: ${filePath}`, error);
            return false;
        }
    }

    /**
     * 获取文件信息
     * @param {string} filePath - 文件路径
     * @returns {Object|null} 文件信息
     */
    getFileInfo(filePath) {
        try {
            if (!fs.existsSync(filePath)) {
                return null;
            }

            const stats = fs.statSync(filePath);
            return {
                path: filePath,
                name: path.basename(filePath),
                size: stats.size,
                created: stats.birthtime,
                modified: stats.mtime,
                isFile: stats.isFile(),
                isDirectory: stats.isDirectory()
            };
        } catch (error) {
            console.error(`[FileManager] Error getting file info: ${filePath}`, error);
            return null;
        }
    }

    /**
     * 清理过期文件
     * @param {string} directoryPath - 目录路径
     * @param {number} maxAge - 最大文件年龄(毫秒)
     * @returns {number} 清理的文件数量
     */
    cleanupOldFiles(directoryPath, maxAge = this.maxFileAge) {
        if (!fs.existsSync(directoryPath)) {
            return 0;
        }

        try {
            const files = fs.readdirSync(directoryPath);
            const now = Date.now();
            let cleanedCount = 0;

            files.forEach(file => {
                const filePath = path.join(directoryPath, file);
                const stats = fs.statSync(filePath);
                
                if (stats.isFile() && (now - stats.mtime.getTime()) > maxAge) {
                    if (this.deleteFile(filePath)) {
                        cleanedCount++;
                    }
                }
            });

            if (cleanedCount > 0) {
                console.log(`[FileManager] Cleaned up ${cleanedCount} old files from ${directoryPath}`);
            }

            return cleanedCount;
        } catch (error) {
            console.error(`[FileManager] Error cleaning up directory: ${directoryPath}`, error);
            return 0;
        }
    }

    /**
     * 清理所有上传目录的过期文件
     * @returns {number} 总清理文件数量
     */
    cleanupAllOldFiles() {
        const directories = [
            path.join(this.uploadBasePath, 'images'),
            path.join(this.uploadBasePath, 'audio'),
            path.join(this.uploadBasePath, 'camera')
        ];

        let totalCleaned = 0;
        directories.forEach(dir => {
            totalCleaned += this.cleanupOldFiles(dir);
        });

        return totalCleaned;
    }

    /**
     * 获取目录大小
     * @param {string} directoryPath - 目录路径
     * @returns {number} 目录大小(字节)
     */
    getDirectorySize(directoryPath) {
        if (!fs.existsSync(directoryPath)) {
            return 0;
        }

        try {
            let totalSize = 0;
            const files = fs.readdirSync(directoryPath);

            files.forEach(file => {
                const filePath = path.join(directoryPath, file);
                const stats = fs.statSync(filePath);
                
                if (stats.isFile()) {
                    totalSize += stats.size;
                } else if (stats.isDirectory()) {
                    totalSize += this.getDirectorySize(filePath);
                }
            });

            return totalSize;
        } catch (error) {
            console.error(`[FileManager] Error calculating directory size: ${directoryPath}`, error);
            return 0;
        }
    }

    /**
     * 格式化文件大小
     * @param {number} bytes - 字节数
     * @returns {string} 格式化后的大小
     */
    formatFileSize(bytes) {
        if (bytes === 0) return '0 Bytes';

        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));

        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    /**
     * 获取上传目录统计信息
     * @returns {Object} 统计信息
     */
    getUploadStats() {
        const directories = {
            images: path.join(this.uploadBasePath, 'images'),
            audio: path.join(this.uploadBasePath, 'audio'),
            camera: path.join(this.uploadBasePath, 'camera')
        };

        const stats = {};

        Object.keys(directories).forEach(type => {
            const dirPath = directories[type];
            const size = this.getDirectorySize(dirPath);
            const fileCount = fs.existsSync(dirPath) ? fs.readdirSync(dirPath).length : 0;

            stats[type] = {
                file_count: fileCount,
                total_size: size,
                formatted_size: this.formatFileSize(size)
            };
        });

        return stats;
    }

    /**
     * 启动定期清理任务
     * @param {number} intervalHours - 清理间隔(小时)
     */
    startPeriodicCleanup(intervalHours = 6) {
        const intervalMs = intervalHours * 60 * 60 * 1000;
        
        setInterval(() => {
            console.log('[FileManager] Starting periodic cleanup...');
            const cleanedCount = this.cleanupAllOldFiles();
            console.log(`[FileManager] Periodic cleanup completed. Cleaned ${cleanedCount} files.`);
        }, intervalMs);

        console.log(`[FileManager] Periodic cleanup started. Interval: ${intervalHours} hours`);
    }
}

module.exports = new FileManager();
