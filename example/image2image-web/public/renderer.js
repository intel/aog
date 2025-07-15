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

const promptInput = document.getElementById('prompt');
const generateButton = document.getElementById('generate-button');
const imageDisplay = document.querySelector('.image-display');
const upscaleButton = document.getElementById('upscale-button');
const upscaledImageDisplay = document.querySelector('.upscaled-image-display');
const thumbnail = document.getElementById('thumbnail'); // 获取缩略图显示框的 img 元素

let generatedImageUrls = [];
let selectedImageUrl = null;

generateButton.addEventListener('click', async () => {
    const promptText = promptInput.value;
    const location = document.querySelector('input[name="generate-location"]:checked').value; // 获取选中的单选按钮值
    if (!promptText) {
        alert('请输入描述文字');
        return;
    }

    // 显示加载图样
    const spinner = document.createElement('div');
    spinner.classList.add('loading-spinner');
    generateButton.disabled = true;
    generateButton.textContent = '正在生成';
    generateButton.appendChild(spinner);

    try {
        const response = await fetch('/api/generate-image', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ prompt: promptText, location: location })
        });
        const data = await response.json();
        if (data.imageUrls && data.imageUrls.length > 0) {
            imageDisplay.innerHTML = '';
            generatedImageUrls = data.imageUrls.map((url, index) => {
                const img = document.createElement('img');
                img.src = url;
                img.classList.add('generated-image');
                img.addEventListener('click', () => {
                    // 清除其他图片的选中状态
                    document.querySelectorAll('.generated-image').forEach(el => el.classList.remove('selected'));
                    img.classList.add('selected');
                    selectedImageUrl = url;

                    // 更新缩略图显示框
                    thumbnail.src = url; // 设置缩略图的 src 为选中图片的 URL
                    thumbnail.alt = `选中的图片 ${index + 1}`; // 设置缩略图的 alt 属性

                    upscaleButton.disabled = false; // 启用高清图按钮
                });
                imageDisplay.appendChild(img);
                return url;
            });
            upscaleButton.disabled = true;
            upscaledImageDisplay.innerHTML = '';
            selectedImageUrl = null;

            // 清空缩略图显示框
            thumbnail.src = '';
            thumbnail.alt = '当前未选择图片';
        } else {
            alert('生成图片失败，请重试');
        }
    } catch (error) {
        console.error('生成图片请求失败:', error);
        alert('生成图片请求失败');
    } finally {
        // 隐藏加载图样
        generateButton.disabled = false;
        generateButton.textContent = 'AOG文生图';
    }
});

upscaleButton.addEventListener('click', async () => {
    if (!selectedImageUrl) {
        alert('请先选择一张图片');
        return;
    }

    const location = document.querySelector('input[name="location"]:checked').value; // 获取本地/云端选项
    const description = document.getElementById('upscale-description').value; // 获取描述文字

    if (!description) {
        alert('请输入图片描述');
        return;
    }

    // 显示加载图样
    const spinner = document.createElement('div');
    spinner.classList.add('loading-spinner');
    upscaleButton.disabled = true;
    upscaleButton.textContent = '正在生成';
    upscaleButton.appendChild(spinner);

    try {
        const response = await fetch('/api/upscale-image', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ 
                imageUrl: selectedImageUrl, 
                location: location, 
                description: description // 添加描述文字
            })
        });
        const data = await response.json();
        if (data.upscaledImageUrl) {
            upscaledImageDisplay.innerHTML = `<img src="${data.upscaledImageUrl}" class="upscaled-image">`;
        } else {
            alert('生成高清图失败，请重试');
        }
    } catch (error) {
        console.error('生成高清图请求失败:', error);
        alert('生成高清图请求失败');
    } finally {
        // 隐藏加载图样
        upscaleButton.disabled = false;
        upscaleButton.textContent = 'AOG图生图';
    }
});