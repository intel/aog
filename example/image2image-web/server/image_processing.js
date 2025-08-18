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
// const serveoUrl = process.env.SERVEO_URL || 'http://localhost:506';
const serveoUrl = 'http://localhost:506'; // 本地测试时使用


const aog = new AogLib();

const axios = require('axios');
const fs = require('fs');
const path = require('path');

async function generateImage(prompt, location) {
    console.log(`[${location}] 请求生成图片，Prompt: ${prompt}`);
    const data = {
        model: "OpenVINO/stable-diffusion-v1-5-fp16-ov",
        prompt: prompt,
        size: "512*512",
        n: 4
    };

    const result = await aog.textToImage(data);
    if (result.code !== 200 || !result.data) {
        throw new Error('生成图片失败，API 返回无效数据');
    }

    const downloadedPaths = result.data.url; // 获取下载路径数组
    const localImageUrls = [];

    // 确保目标文件夹存在
    const imagesDir = path.join(__dirname, 'images');
    if (!fs.existsSync(imagesDir)) {
        fs.mkdirSync(imagesDir);
    }

    // 将图片移动到本地 images 文件夹
    for (const downloadedPath of downloadedPaths) {
        const fileName = path.basename(downloadedPath); // 获取文件名
        const targetPath = path.join(imagesDir, fileName); // 目标路径

        try {
            fs.renameSync(downloadedPath, targetPath); // 移动文件
            localImageUrls.push(`${serveoUrl}/images/${fileName}`); // 构造本地链接
        } catch (error) {
            console.error(`移动文件失败: ${downloadedPath} -> ${targetPath}`, error);
            throw new Error('移动文件失败');
        }
    }
    console.log("生成完成");
    
    return localImageUrls; // 返回本地链接数组

}

async function upscaleImage(imageUrl, location, description) {
    console.log(`[${location}] 请求高清化图片，URL: ${imageUrl}`);

    // 确保输入的 imageUrl 是本地文件路径
    const localImagePath = path.join(__dirname, 'images', path.basename(imageUrl));
    if (!fs.existsSync(localImagePath)) {
        throw new Error(`本地图片文件不存在: ${localImagePath}`);
    }

    const data = {
        model: "wanx2.0-t2i-turbo",
        prompt: description,
        image: localImagePath, // 使用本地图片的 URL
        image_type: "path",
        size: "1024*1024",
        n: 1
    };
    console.log(data);

    const result = await aog.textToImage(data);
    if (result.code !== 200 || !result.data) {
        throw new Error('高清化图片失败，API 返回无效数据');
    }

    const downloadedUrl = result.data.url[0]; // 获取返回的高清图片远程 URL
    const imagesDir = path.join(__dirname, 'images'); // 本地存储目录

    // 确保目标文件夹存在
    if (!fs.existsSync(imagesDir)) {
        fs.mkdirSync(imagesDir);
    }

    const fileName = path.basename(downloadedUrl.split('?')[0]); // 去掉 URL 参数，获取文件名
    const targetPath = path.join(imagesDir, fileName); // 目标路径

    try {
        // 下载远程文件到本地
        const response = await axios({
            method: 'get',
            url: downloadedUrl,
            responseType: 'stream'
        });

        const writer = fs.createWriteStream(targetPath);
        response.data.pipe(writer);

        // 等待文件写入完成
        await new Promise((resolve, reject) => {
            writer.on('finish', resolve);
            writer.on('error', reject);
        });

        console.log(`高清化图片已保存到本地: ${targetPath}`);
    } catch (error) {
        console.error(`下载或保存文件失败: ${downloadedUrl} -> ${targetPath}`, error);
        throw new Error('下载高清化图片文件失败');
    }

    // 返回本地链接
    return `${serveoUrl}/images/${fileName}`;
}

module.exports = { generateImage, upscaleImage };