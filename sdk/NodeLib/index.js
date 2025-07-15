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

// const express = require('express');
const http = require('http');
const https = require('https');
const fs = require('fs');
const path = require('path');
const os = require('os');
const axios = require('axios');
const EventEmitter = require('events');
const { execFile, spawn } = require('child_process');
const { promises: fsPromises } = require("fs");

const schemas = require('./schema.js');
const tools = require('./tools.js');
const { logAndConsole, downloadFile, getAOGExecutablePath, runInstallerByPlatform, isHealthy } = require('./tools.js');
const { instance, createAxiosInstance, requestWithSchema } = require('./axiosInstance.js')
const { PLATFORM_CONFIG, AOG_HEALTH, AOG_ENGINE_PATH, AOG_VERSION, WS_URL, } = require('./constants.js');

class AOG {
  constructor(version) {
    this.version = version || "aog/v0.4";
    this.client = instance
    logAndConsole('info', `AOG类初始化，版本: ${this.version}`);
  }

  async _requestWithSchema({ method, url, data, schema }) {
    logAndConsole('info', `请求API: ${method.toUpperCase()} ${url}`);
    return await requestWithSchema({ method, url, data, schema });
  }

  // 检查 AOG 服务是否启动
  async isAOGAvailable(retries = 5, interval = 1000) {
    logAndConsole('info', '检测AOG服务可用性...');
    const fibArr = tools.fibonacci(retries, interval);
    for (let attempt = 0; attempt < retries; attempt++) {
      try {
        const [healthRes, engineHealthRes] = await Promise.all([
          axios.get(AOG_HEALTH),
          axios.get(AOG_ENGINE_PATH)
        ]);
        const healthOk = isHealthy(healthRes.status);
        const engineOk = isHealthy(engineHealthRes.status);
        logAndConsole('info', `/health: ${healthOk ? '正常' : '异常'}, /engine/health: ${engineOk ? '正常' : '异常'}`);
        if (healthOk && engineOk) return true;
      } catch (err) {
        logAndConsole('warn', `健康检查失败: ${err.message}`);
      }
      if (attempt < retries - 1) {
        await new Promise(r => setTimeout(r, fibArr[attempt]));
      }
    }
    logAndConsole('warn', 'AOG服务不可用');
    return false;
  }

  // 检查用户目录是否存在 aog.exe
  isAOGExisted() {
    const dest = getAOGExecutablePath();
    const existed = fs.existsSync(dest);
    logAndConsole('info', `检测AOG可执行文件是否存在: ${dest}，结果: ${existed}`);
    return existed;
  }

  // 私有方法：仅下载
  async _downloadFile(url, dest, options, retries = 3) {
    logAndConsole('info', `准备下载文件: ${url} 到 ${dest}`);
    return await downloadFile(url, dest, options, retries);
  }

  // 运行安装包
  async _runAOGInstaller(installerPath) {
    const platform = tools.getPlatform();
    logAndConsole('info', `运行安装包: ${installerPath}，平台: ${platform}`);
    try {
      await runInstallerByPlatform(installerPath);
      logAndConsole('info', '安装包运行成功');
      return true;
    } catch (err) {
      logAndConsole('error', '安装包运行失败：' + err.message);
      return false;
    }
  }

  async downloadAOG(retries = 3) {
    try {
      const platform = tools.getPlatform();
      if (platform === 'unsupported' || !PLATFORM_CONFIG[platform]) {
        logAndConsole('error', '不支持的平台');
        return false;
      }
      const { downloadUrl, installerFileName, userAgent } = PLATFORM_CONFIG[platform];
      const userDir = os.homedir();
      const destDir = path.join(userDir, 'AOGInstaller');
      const dest = path.join(destDir, installerFileName);
      const options = {
        headers: {
          'User-Agent': userAgent,
        },
      };
      const downloadOk = await this._downloadFile(downloadUrl, dest, options, retries);
      if (downloadOk) {
        const installResult = await this._runAOGInstaller(dest);
        return installResult;
      } else {
        logAndConsole('error', '三次下载均失败，放弃安装。');
        return false;
      }
    } catch (err) {
      logAndConsole('error', '下载或安装 AOG 失败: ' + err.message);
      return false;
    }
  }

  // 启动 AOG 服务
  async startAOG() {
    const alreadyRunning = await this.isAOGAvailable(2, 1000);
    if (alreadyRunning) {
      logAndConsole('info', '[startAOG] AOG 在运行中');
      return true;
    }
    return new Promise((resolve, reject) => {
      const platform = tools.getPlatform();
      const userDir = os.homedir();
      const aogDir = path.join(userDir, 'AOG');
      logAndConsole('info', `aogDir: ${aogDir}`);
      if (platform === "unsurported") return reject(new Error(`不支持的平台`));
      if (platform === 'win32') {
        if (!process.env.PATH.includes(aogDir)) {
          process.env.PATH = `${process.env.PATH}${path.delimiter}${aogDir}`;
          logAndConsole('info', '添加到临时环境变量');
        }
        const command = 'cmd.exe';
        const args = ['/c', 'start-aog.bat'];
        logAndConsole('info', `正在运行命令: ${command} ${args.join(' ')}`);
        execFile(command, args, { windowsHide: true }, async (error, stdout, stderr) => {
          if (error) logAndConsole('error', 'aog server start:error ' + error);
          if (stdout) logAndConsole('info', 'aog server start:stdout: ' + stdout.toString());
          if (stderr) logAndConsole('error', 'aog server start:stderr: ' + stderr.toString());
          const output = (stdout + stderr).toString().toLowerCase();
          if (error || output.includes('error')) {
            return resolve(false);
          }
          const available = await this.isAOGAvailable(5, 1500);
          return resolve(available);
        });
      } else if (platform === 'darwin') {
        try {
          if (!process.env.PATH.split(':').includes('/usr/local/bin')) {
            process.env.PATH = `/usr/local/bin:${process.env.PATH}`;
            logAndConsole('info', '已将 /usr/local/bin 添加到 PATH');
          }
          let child;
          let stderrContent = '';
          child = spawn('/usr/local/bin/aog', ['server', 'start', '-d'], {
            stdio: ['ignore', 'pipe', 'pipe'],
            windowsHide: true,
          });
          child.stdout.on('data', (data) => {
            if (data.toString().includes('server start successfully')) {
              //TODO：获取退出状态码
              logAndConsole('info', 'AOG 服务启动成功');
              resolve(true);
            }
            logAndConsole('info', `stdout: ${data}`);
          });
          child.stderr.on('data', (data) => {
            const errorMessage = data.toString().trim();
            stderrContent += errorMessage + '\n';
            logAndConsole('error', `stderr: ${errorMessage}`);
          });
          child.on('error', (err) => {
            logAndConsole('error', `❌ 启动失败: ${err.message}`);
            if (err.code === 'ENOENT') {
              logAndConsole('error', '未找到aog可执行文件，请检查下载是否成功或环境变量未生效');
            }
            resolve(false);
          });
          child.on('close', (code) => {
            if (stderrContent.includes('Install model engine failed')){
              logAndConsole('error', '❌ 启动失败: 模型引擎安装失败。');
              resolve(false);
            } else if (code === 0) {
              logAndConsole('info', '进程退出，正在检查服务状态...');
            } else {
              logAndConsole('error', `❌ 启动失败，退出码: ${code}`);
              resolve(false);
            }
          });
          child.unref();
        } catch (error) {
          logAndConsole('error', '启动 AOG 服务异常: ' + error.message);
          resolve(false);
        }
      }
    });
  }
  
  // 查看当前服务
  async getServices() {
    return this._requestWithSchema({
      method: 'get',
      url: '/service',
      schema: { response: schemas.getServicesSchema }
    });
  }

  // 创建新服务
  async installService(data) {
    return this._requestWithSchema({
      method: 'post',
      url: '/service',
      data,
      schema: { request: schemas.installServiceRequestSchema, response: schemas.ResponseSchema }
    });
  }

  // 更新服务
  async updateService(data) {
    return this._requestWithSchema({
      method: 'put',
      url: '/service',
      data,
      schema: { request: schemas.updateServiceRequestSchema, response: schemas.ResponseSchema }
    });
  }

  // 查看当前模型
  async getModels() {
    return this._requestWithSchema({
      method: 'get',
      url: '/model',
      schema: { response: schemas.getModelsSchema }
    });
  }

  // 安装新模型
  async installModel(data) {
    return this._requestWithSchema({
      method: 'post',
      url: '/model',
      data,
      schema: { request: schemas.installModelRequestSchema, response: schemas.ResponseSchema }
    });
  }

  async updateModel(data) {
    return this._requestWithSchema({
      method: 'put',
      url: '/model',
      data,
      schema: { request: schemas.updateModelRequestSchema, response: schemas.ResponseSchema }
    });
  }

  async deleteModel(data) {
    return this._requestWithSchema({
      method: 'post',
      url: '/model',
      data,
      schema: { request: schemas.deleteModelRequestSchema, response: schemas.ResponseSchema }
    });
  }

  // 查看服务提供商
  async getServiceProviders() {
    return this._requestWithSchema({
      method: 'get',
      url: '/service_provider',
      schema: { response: schemas.getServiceProvidersSchema }
    });
  }

  // 新增服务提供商
  async installServiceProvider(data) {
    return this._requestWithSchema({
      method: 'post',
      url: '/service_provider',
      data,
      schema: { request: schemas.installServiceProviderRequestSchema, response: schemas.ResponseSchema }
    });
  }

  // 更新服务提供商
  async updateServiceProvider(data) {
    return this._requestWithSchema({
      method: 'put',
      url: '/service_provider',
      data,
      schema: { request: schemas.updateServiceProviderRequestSchema, response: schemas.ResponseSchema }
    });
  }

  // 删除服务提供商
  async deleteServiceProvider(data) {
    return this._requestWithSchema({
      method: 'post',
      url: '/service_provider',
      data,
      schema: { request: schemas.deleteServiceProviderRequestSchema, response: schemas.ResponseSchema }
    });
  }

  // 导入配置文件
  async importConfig(filePath) {
    try {
      const data = await fsPromises.readFile(filePath, 'utf8');
      return this._requestWithSchema({
        method: 'post',
        url: '/service/import',
        data,
        schema: { response: schemas.ResponseSchema }
      });
    } catch (error) {
      return { code: 400, msg: error.message, data: null };
    }
  }

  // 导出配置文件
  async exportConfig(data = {}) {
    // 只做文件写入，http部分用统一schema校验
    const result = await this._requestWithSchema({
      method: 'post',
      url: '/service/export',
      data,
      schema: { request: schemas.exportRequestSchema, response: schemas.ResponseSchema }
    });
    if (result.code === 200) {
      try {
        const userDir = os.homedir();
        const destDir = path.join(userDir, 'AOG');
        const dest = path.join(destDir, '.aog');
        tools.ensureDirWritable(destDir);
        const fileContent = JSON.stringify(result.data, null, 2);
        fs.writeFileSync(dest, fileContent);
        console.log(`已将生成文件写入到 ${dest}`);
      } catch (error) {
        return { code: 400, msg: error.message, data: null };
      }
    }
    return result;
  }

  // 获取推荐模型列表
  async getModelsRecommended() {
    return this._requestWithSchema({
      method: 'get',
      url: '/model/recommend',
      schema: { response: schemas.recommendModelsResponse }
    });
  }

  // getModelsSupported
  async getModelsSupported(data) {
    return this._requestWithSchema({
      method: 'get',
      url: '/model/support',
      data: { params: data },
      schema: { request: schemas.getModelsSupported, response: schemas.recommendModelsResponse }
    });
  }

  // getSmartvisionModelsSupported
  async getSmartvisionModelsSupported(data) {
    return this._requestWithSchema({
      method: 'get',
      url: '/model/support/smartvision',
      data: { params: data },
      schema: { request: schemas.SmartvisionModelSupportRequest }
    });
  }

  // chat服务（支持流式和非流式）
  async chat(data) {
    const stream = data.stream;
    if (!stream) {
      // 非流式
      return this._requestWithSchema({ method: 'post', url: 'services/chat', data });
    }
    // 流式
    try {
      const config = { responseType: 'stream' };
      const res = await this.client.post('services/chat', data, config);
      const eventEmitter = new EventEmitter();
      res.data.on('data', (chunk) => {
        try {
          let rawData = _.isString(chunk) ? _.trim(chunk) : _.trim(chunk.toString());
          let jsonString = _.startsWith(rawData, 'data:') ? rawData.slice(5) : rawData;
          jsonString = _.trim(jsonString);
          if (_.isEmpty(jsonString)) {
            throw new Error('收到空的流数据');
          }
          const response = JSON.parse(jsonString);
          eventEmitter.emit('data', response);
          if (response.status === 'success' || response.status === 'canceled' || response.status === 'error') {
            eventEmitter.emit('end', response);
          }
        } catch (err) {
          eventEmitter.emit('error', `解析流数据失败: ${err.message}`);
        }
      });
      res.data.on('error', (err) => {
        eventEmitter.emit('error', `流式响应错误: ${err.message}`);
      });
      return eventEmitter;
    } catch (error) {
      return { code: 400, msg: error.response?.data?.message || error.message, data: null };
    }
  }

  // 生文服务（支持流式和非流式）
  async generate(data) {
    const stream = data.stream;
    if (!stream) {
      return this._requestWithSchema({ method: 'post', url: 'services/generate', data });
    }
    try {
      const config = { responseType: 'stream' };
      const res = await this.client.post('services/generate', data, config);
      const eventEmitter = new EventEmitter();
      res.data.on('data', (chunk) => {
        try {
          let rawData = _.isString(chunk) ? _.trim(chunk) : _.trim(chunk.toString());
          let jsonString = _.startsWith(rawData, 'data:') ? rawData.slice(5) : rawData;
          jsonString = _.trim(jsonString);
          if (_.isEmpty(jsonString)) {
            throw new Error('收到空的流数据');
          }
          const response = JSON.parse(jsonString);
          eventEmitter.emit('data', response);
          if (response.status === 'success' || response.status === 'canceled' || response.status === 'error') {
            eventEmitter.emit('end', response);
          }
        } catch (err) {
          eventEmitter.emit('error', `解析流数据失败: ${err.message}`);
        }
      });
      res.data.on('error', (err) => {
        eventEmitter.emit('error', `流式响应错误: ${err.message}`);
      });
      return eventEmitter;
    } catch (error) {
      return { code: 400, msg: error.response?.data?.message || error.message, data: null };
    }
  }
  
  // 生图服务
  async textToImage(data) {
    return this._requestWithSchema({
      method: 'post',
      url: '/services/text-to-image',
      data,
      schema: { request: schemas.textToImageRequest, response: schemas.textToImageResponse }
    });
  }

  // embed服务
  async embed(data) {
    return this._requestWithSchema({
      method: 'post',
      url: '/services/embed',
      data,
      schema: { request: schemas.embeddingRequest, response: schemas.embeddingResponse }
    });
  }

  async speechToText(data) {
    return this._requestWithSchema({
      method: 'post',
      url: '/services/speech-to-text',
      data,
      schema: { request: schemas.speechToTextRequest, response: schemas.speechToTextResponse}
    })
  }

  SpeechToTextStream(options = {}) {
    if (!options.model) {
      throw new Error('model is required');
    }

    // 默认配置
    const config = {
      language: 'zh',
      sampleRate: 16000,
      channels: 1,
      useVad: true,
      returnFormat: 'text',
      ...options
    };
    const emitter = new EventEmitter();
    const wsUrl = WS_URL;
    const ws = new WebSocket(wsUrl);

    let taskId = null;
    let isConnected = false;
    let isTaskStarted = false;
    
    const queue = [];
    
    ws.on('open', () => {
      isConnected = true;
      console.log('✅ WebSocket 连接已建立');
      
      // 发送 run-task 指令
      ws.send(JSON.stringify({
        task: "speech-to-text-ws",
        action: "run-task",
        model: config.model,
        parameters: {
          format: "pcm",
          sample_rate: config.sampleRate,
          language: config.language,
          use_vad: config.useVad,
          return_format: config.returnFormat,
          channels: config.channels
        }
      }));
      
      emitter.emit('open');
    });
    
    // 接收消息
    ws.on('message', (data) => {
      try {
        const message = JSON.parse(data);
        const event = message.header?.event
        
        switch (event) {
          case 'task-started':
            console.log(`task started`, message);
            taskId = message.header.task_id;
            isTaskStarted = true;

            while (queue.length > 0) {
              const chunk = queue.shift();
              ws.send(chunk);
            }
            
            emitter.emit('taskStarted', { taskId });
            break;
            
          case 'task-finished':
            console.log("task finished:", message)
            emitter.emit('finished', { 
              text: message.text,
              taskId: message.task_id
            });
            ws.close();
            break;
            
          case 'error':
            console.log("error", message)
            emitter.emit('error', new Error(message.message || '服务器返回错误'));
            break;
            
          default:
            console.warn('message:', message);
            emitter.emit('message', message);
        }
      } catch (err) {
        emitter.emit('error', new Error(`消息解析失败: ${err.message}`));
      }
    });
    
    // 错误处理
    ws.on('error', (err) => {
      console.error('WebSocket 错误:', err);
      emitter.emit('error', new Error(`WebSocket 错误: ${err.message}`));
    });
    
    // 连接关闭
    ws.on('close', () => {
      isConnected = false;
      isTaskStarted = false;
      console.log('🔌 WebSocket 连接已关闭');
      emitter.emit('close');
    });
    
    emitter.write = (chunk) => {
      if (isConnected && isTaskStarted) {
        ws.send(chunk);
      } else if (isConnected) {
        // 连接已建立但任务尚未启动，加入队列
        queue.push(chunk);
        console.log('📦 音频数据已加入队列，等待任务启动后发送');
      } else {
        // 连接未就绪，加入队列并等待连接建立
        queue.push(chunk);
        console.warn('⚠️ 音频数据已加入队列，等待连接建立后发送');
      }
    };
    
    emitter.end = () => {
      if (isConnected && isTaskStarted) {
        console.log(`⏹️ 发送结束任务指令`);
        ws.send(JSON.stringify({
          task: "speech-to-text-ws",
          action: "finish-task",
          task_id: taskId,
          model: config.model
        }));
      } else if (isConnected && !isTaskStarted) {
        const error = new Error('无法结束任务: 任务尚未启动');
        console.error(error.message);
        emitter.emit('error', error);
      } else if (!isConnected) {
        const error = new Error('无法结束任务: 连接未建立');
        console.error(error.message);
        emitter.emit('error', error);
      } else if (!taskId) {
        const error = new Error('无法结束任务: 任务ID未分配');
        console.error(error.message);
        emitter.emit('error', error);
      }
    };
    
    return emitter;
  }

  // 用于一键安装 AOG 和 导入配置
  // TODO：记录日志
  async AOGInit(path){
    const isAOGAvailable = await this.isAOGAvailable();
    if (isAOGAvailable) {
      logAndConsole('info','✅ AOG 服务已启动，跳过安装。');
      return true;
    }
    
    const isAOGExisted = this.isAOGExisted();
    if (!isAOGExisted) {
      const downloadSuccess = await this.downloadAOG();
      if (!downloadSuccess) {
        logAndConsole('error','❌ 下载 AOG 失败，请检查网络连接或手动下载。');
        return false;
      }
    } else {
      logAndConsole('info','✅ AOG 已存在，跳过下载。');
    }

    const installSuccess = await this.startAOG();
    if (!installSuccess) {
      logAndConsole('error','❌ 启动 AOG 服务失败，请检查配置或手动启动。');
      return false;
    } else {
      logAndConsole('info','✅ AOG 服务已启动。');
    }

    const importSuccess = await this.importConfig(path);
    if (!importSuccess) {
      logAndConsole('error','❌ 导入配置文件失败。');
      return false;
    } else {
      logAndConsole('info','✅ 配置文件导入成功。');
    }
    return true;
  }
}

module.exports = AOG;