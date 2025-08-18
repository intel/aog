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

class PromptExtractor {
    constructor() {
        // 中文停用词
        this.stopWords = new Set([
            '的', '了', '在', '是', '我', '你', '他', '她', '它', '们', '这', '那', '有', '和', '与',
            '或', '但', '而', '就', '都', '被', '从', '把', '比', '让', '使', '给', '对', '向',
            '为', '以', '用', '由', '如', '若', '则', '将', '会', '能', '可', '要', '想', '说',
            '看', '听', '做', '去', '来', '到', '上', '下', '前', '后', '左', '右', '中', '内',
            '外', '东', '西', '南', '北', '大', '小', '多', '少', '高', '低', '长', '短', '新',
            '旧', '好', '坏', '美', '丑', '快', '慢', '早', '晚', '今', '明', '昨', '年', '月',
            '日', '时', '分', '秒', '个', '只', '件', '条', '张', '片', '块', '段', '点', '些'
        ]);

        // 图像相关关键词权重
        this.imageKeywords = new Map([
            // 颜色
            ['红色', 3], ['蓝色', 3], ['绿色', 3], ['黄色', 3], ['紫色', 3], ['橙色', 3],
            ['黑色', 3], ['白色', 3], ['灰色', 3], ['粉色', 3], ['棕色', 3],
            // 风格
            ['现代', 2], ['古典', 2], ['简约', 2], ['复古', 2], ['科幻', 2], ['梦幻', 2],
            ['写实', 2], ['抽象', 2], ['卡通', 2], ['动漫', 2], ['油画', 2], ['水彩', 2],
            // 场景
            ['室内', 2], ['室外', 2], ['城市', 2], ['乡村', 2], ['海边', 2], ['山区', 2],
            ['森林', 2], ['花园', 2], ['建筑', 2], ['街道', 2], ['公园', 2], ['商店', 2],
            // 物体
            ['人物', 2], ['动物', 2], ['植物', 2], ['花朵', 2], ['树木', 2], ['汽车', 2],
            ['房子', 2], ['桌子', 2], ['椅子', 2], ['窗户', 2], ['门', 2], ['灯光', 2],
            // 情感
            ['温暖', 2], ['冷酷', 2], ['明亮', 2], ['昏暗', 2], ['宁静', 2], ['热闹', 2],
            ['神秘', 2], ['浪漫', 2], ['优雅', 2], ['活泼', 2]
        ]);
    }

    /**
     * 从文本中提取图像生成的prompt
     * @param {string} text - 输入文本
     * @param {number} maxKeywords - 最大关键词数量
     * @returns {string} 提取的prompt
     */
    extractImagePrompt(text, maxKeywords = 8) {
        if (!text || typeof text !== 'string') {
            return '';
        }

        console.log(`[PromptExtractor] Extracting prompt from text: ${text.substring(0, 100)}...`);

        // 清理和分词
        const words = this.tokenize(text);
        
        // 提取关键词
        const keywords = this.extractKeywords(words, maxKeywords);
        
        // 构建prompt
        const prompt = this.buildImagePrompt(keywords);
        
        console.log(`[PromptExtractor] Extracted prompt: ${prompt}`);
        return prompt;
    }

    /**
     * 文本分词
     * @param {string} text - 输入文本
     * @returns {Array} 词汇数组
     */
    tokenize(text) {
        // 移除标点符号并分词
        const cleanText = text.replace(/[，。！？；：""''（）【】\s]+/g, ' ');
        
        // 简单的中文分词（实际应用中可以使用更专业的分词库）
        const words = [];
        
        // 按空格分割
        const segments = cleanText.split(/\s+/).filter(seg => seg.length > 0);
        
        segments.forEach(segment => {
            // 对于中文，尝试提取2-4字的词组
            for (let i = 0; i < segment.length; i++) {
                for (let len = 2; len <= Math.min(4, segment.length - i); len++) {
                    const word = segment.substring(i, i + len);
                    if (this.isValidWord(word)) {
                        words.push(word);
                    }
                }
                // 也添加单个字符（如果是有意义的）
                const char = segment[i];
                if (this.isValidWord(char)) {
                    words.push(char);
                }
            }
        });
        
        return words;
    }

    /**
     * 检查是否是有效词汇
     * @param {string} word - 词汇
     * @returns {boolean} 是否有效
     */
    isValidWord(word) {
        if (!word || word.length === 0) return false;
        if (this.stopWords.has(word)) return false;
        if (word.length === 1 && !/[\u4e00-\u9fa5a-zA-Z]/.test(word)) return false;
        return true;
    }

    /**
     * 提取关键词
     * @param {Array} words - 词汇数组
     * @param {number} maxKeywords - 最大关键词数量
     * @returns {Array} 关键词数组
     */
    extractKeywords(words, maxKeywords) {
        const wordFreq = new Map();
        const wordScores = new Map();

        // 计算词频
        words.forEach(word => {
            wordFreq.set(word, (wordFreq.get(word) || 0) + 1);
        });

        // 计算词汇得分
        wordFreq.forEach((freq, word) => {
            let score = freq;
            
            // 图像相关关键词加权
            if (this.imageKeywords.has(word)) {
                score *= this.imageKeywords.get(word);
            }
            
            // 长词汇加权
            if (word.length >= 3) {
                score *= 1.5;
            }
            
            wordScores.set(word, score);
        });

        // 按得分排序并取前N个
        const sortedWords = Array.from(wordScores.entries())
            .sort((a, b) => b[1] - a[1])
            .slice(0, maxKeywords)
            .map(entry => entry[0]);

        return sortedWords;
    }

    /**
     * 构建图像prompt
     * @param {Array} keywords - 关键词数组
     * @returns {string} 构建的prompt
     */
    buildImagePrompt(keywords) {
        if (keywords.length === 0) {
            return 'beautiful, detailed, high quality';
        }

        // 基础prompt模板
        let prompt = keywords.join(', ');
        
        // 添加质量修饰词
        const qualityTerms = ['detailed', 'high quality', 'beautiful'];
        prompt += ', ' + qualityTerms.join(', ');

        return prompt;
    }

    /**
     * 合并两个prompt
     * @param {string} originalPrompt - 原始prompt
     * @param {string} additionalPrompt - 附加prompt
     * @returns {string} 合并后的prompt
     */
    mergePrompts(originalPrompt, additionalPrompt) {
        if (!originalPrompt) return additionalPrompt || '';
        if (!additionalPrompt) return originalPrompt;

        // 去重并合并
        const originalKeywords = originalPrompt.split(',').map(k => k.trim()).filter(k => k);
        const additionalKeywords = additionalPrompt.split(',').map(k => k.trim()).filter(k => k);
        
        const allKeywords = [...new Set([...originalKeywords, ...additionalKeywords])];
        
        return allKeywords.join(', ');
    }

    /**
     * 优化prompt用于特定风格
     * @param {string} prompt - 原始prompt
     * @param {string} style - 风格类型
     * @returns {string} 优化后的prompt
     */
    optimizeForStyle(prompt, style) {
        const styleModifiers = {
            'realistic': 'photorealistic, detailed, high resolution',
            'artistic': 'artistic, creative, expressive',
            'cartoon': 'cartoon style, colorful, cute',
            'anime': 'anime style, manga, detailed',
            'vintage': 'vintage, retro, classic',
            'modern': 'modern, contemporary, sleek',
            'fantasy': 'fantasy, magical, mystical',
            'sci-fi': 'sci-fi, futuristic, technological'
        };

        const modifier = styleModifiers[style.toLowerCase()];
        if (modifier) {
            return `${prompt}, ${modifier}`;
        }

        return prompt;
    }

    /**
     * 从语音识别结果中提取优化指令
     * @param {string} speechText - 语音识别文本
     * @returns {Object} 提取的指令
     */
    extractOptimizationInstructions(speechText) {
        const instructions = {
            style: null,
            color: null,
            mood: null,
            additional: []
        };

        if (!speechText) return instructions;

        const text = speechText.toLowerCase();

        // 提取风格指令
        const stylePatterns = [
            { pattern: /(更|变得|变成).*(写实|真实)/, value: 'realistic' },
            { pattern: /(更|变得|变成).*(艺术|绘画)/, value: 'artistic' },
            { pattern: /(更|变得|变成).*(卡通|动画)/, value: 'cartoon' },
            { pattern: /(更|变得|变成).*(动漫|二次元)/, value: 'anime' },
            { pattern: /(更|变得|变成).*(复古|怀旧)/, value: 'vintage' },
            { pattern: /(更|变得|变成).*(现代|时尚)/, value: 'modern' }
        ];

        stylePatterns.forEach(({ pattern, value }) => {
            if (pattern.test(text)) {
                instructions.style = value;
            }
        });

        // 提取颜色指令
        const colorPatterns = [
            { pattern: /(更|变得|变成).*(红|红色)/, value: 'red' },
            { pattern: /(更|变得|变成).*(蓝|蓝色)/, value: 'blue' },
            { pattern: /(更|变得|变成).*(绿|绿色)/, value: 'green' },
            { pattern: /(更|变得|变成).*(黄|黄色)/, value: 'yellow' },
            { pattern: /(更|变得|变成).*(紫|紫色)/, value: 'purple' }
        ];

        colorPatterns.forEach(({ pattern, value }) => {
            if (pattern.test(text)) {
                instructions.color = value;
            }
        });

        // 提取情绪指令
        const moodPatterns = [
            { pattern: /(更|变得|变成).*(明亮|亮)/, value: 'bright' },
            { pattern: /(更|变得|变成).*(暗|昏暗)/, value: 'dark' },
            { pattern: /(更|变得|变成).*(温暖|暖)/, value: 'warm' },
            { pattern: /(更|变得|变成).*(冷|冷酷)/, value: 'cool' },
            { pattern: /(更|变得|变成).*(梦幻|神秘)/, value: 'dreamy' }
        ];

        moodPatterns.forEach(({ pattern, value }) => {
            if (pattern.test(text)) {
                instructions.mood = value;
            }
        });

        // 提取其他指令
        const additionalKeywords = this.extractKeywords(this.tokenize(speechText), 5);
        instructions.additional = additionalKeywords;

        return instructions;
    }
}

module.exports = new PromptExtractor();
