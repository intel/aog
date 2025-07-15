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

const express = require('express');
const bodyParser = require('body-parser');
const imageProcessing = require('./image_processing');
const cors = require('cors');

const app = express();
const port = 506;

app.use(cors());
app.use(bodyParser.json());
app.use(express.static('public'));
app.use('/images', express.static('server/images')); // 静态文件目录

const aogchecker = require('aog-checker');
aogchecker.AOGInit();

app.post('/api/generate-image', async (req, res) => {
    const { prompt, location } = req.body;
    try {
        const imageUrls = await imageProcessing.generateImage(prompt, location);
        console.log(imageUrls);
        res.json({ imageUrls });
    } catch (error) {
        console.error('生成图片失败:', error);
        res.status(500).json({ error: '生成图片失败' });
    }
});

app.post('/api/upscale-image', async (req, res) => {
    const { imageUrl, location, description } = req.body;
    try {
        const upscaledImageUrl = await imageProcessing.upscaleImage(imageUrl, location, description);
        res.json({ upscaledImageUrl });
    } catch (error) {
        console.error('生成高清图失败:', error);
        res.status(500).json({ error: '生成高清图失败' });
    }
});

app.listen(port, () => {
    console.log(`服务器运行在 http://localhost:${port}`);
});
