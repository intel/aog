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
const http = require('http');
const fs = require('fs');
const path = require('path');
const os = require('os');
const axios = require('axios');
const { promises: fsPromises } = require("fs");
const { spawn } = require('child_process');
const { execSync } = require('child_process');
const AdmZip = require('adm-zip');

const webServerPort = 5000;
let userResponseResolve;
let userResponsePromise;

function waitForUserResponse() {
  return new Promise((resolve) => {
    userResponseResolve = resolve;
  });
}

function startWebServer() {
  const app = express();
  app.use(express.json());

  app.get('/install-prompt', (req, res) => {
    res.type('html').send(installPromptHTML);
  });

  app.get('/user-response', (req, res) => {
    const choice = req.query.choice === 'true';
    if (userResponseResolve) {
      userResponseResolve(choice);
      userResponseResolve = null;
    }
    res.sendStatus(200);
  });

  app.listen(webServerPort, () => {
    console.log(`Web server running on http://localhost:${webServerPort}`);
  });
}

function openBrowser(url) {
  let cmd, args;
  const platform = os.platform();
  if (platform === 'win32') {
    cmd = 'cmd';
    args = ['/c', 'start', url];
  } else if (platform === 'darwin') {
    cmd = 'open';
    args = [url];
  } else {
    cmd = 'xdg-open';
    args = [url];
  }
  try {
    spawn(cmd, args, { detached: true, stdio: 'ignore' });
    return true;
  } catch (err) {
    return false;
  }
}

function AddToUserPath(destDir) {
  const isMacOS = process.platform === 'darwin';

  if (isMacOS) {
    try {
      // 优先检查 .zprofile 文件
      const zprofilePath = path.join(os.homedir(), '.zprofile');
      const bashProfilePath = path.join(os.homedir(), '.bash_profile');
      let shellConfigPath = '';

      if (fs.existsSync(zprofilePath)) {
        shellConfigPath = zprofilePath;
      } else if (fs.existsSync(bashProfilePath)) {
        shellConfigPath = bashProfilePath;
      } else {
        // 如果两个文件都不存在，默认创建 .zprofile
        shellConfigPath = zprofilePath;
        fs.writeFileSync(shellConfigPath, '');
      }

      const exportLine = `export PATH="$PATH:${destDir}"`;

      // 检查是否已存在路径
      const content = fs.readFileSync(shellConfigPath, 'utf8');
      const pathRegex = new RegExp(`(^|\\n)export PATH=.*${destDir}.*`, 'm');
      if (pathRegex.test(content)) {
        console.log('✅ 环境变量已存在:', destDir);
        return true;
      }

      // 追加路径到配置文件
      fs.appendFileSync(shellConfigPath, `\n${exportLine}\n`);
      console.log(`✅ 已添加到 ${path.basename(shellConfigPath)}，请执行以下命令生效：\nsource ${shellConfigPath}`);
      return true;
    } catch (err) {
      console.error('❌ 添加环境变量失败:', err.message);
      return false;
    }
  } else {
    // Windows 环境变量处理
    try {
      const regKey = 'HKCU\\Environment';
      let currentPath = '';

      try {
        const output = execSync(`REG QUERY "${regKey}" /v Path`, {
          encoding: 'utf-8',
          stdio: ['pipe', 'pipe', 'ignore']
        });
        const match = output.match(/Path\s+REG_(SZ|EXPAND_SZ)\s+(.*)/);
        currentPath = match ? match[2].trim() : '';
      } catch { }

      // 检查路径是否已存在
      const paths = currentPath.split(';').filter(p => p);
      if (paths.includes(destDir)) {
        console.log('✅ 环境变量已存在');
        return true;
      }

      // 更新 Path 值
      const newPath = currentPath ? `${currentPath};${destDir}` : destDir;
      execSync(`REG ADD "${regKey}" /v Path /t REG_EXPAND_SZ /d "${newPath}" /f`, {
        stdio: 'inherit'
      });

      console.log('✅ 已添加到环境变量，请重新启动应用程序使更改生效');
      return true;
    } catch (error) {
      console.error('❌ 添加环境变量失败:', error.message);
      return false;
    }
  }
}

// 检查 aog 是否启动
function isAOGAvailable() {
  return new Promise((resolve) => {
    const options = {
      hostname: 'localhost',
      port: 16688,
      path: '/',
      method: 'GET',
      timeout: 3000,
    };
    const req = http.request(options, (res) => {
      resolve(res.statusCode === 200);
    });
    req.on('error', () => resolve(false));
    req.on('timeout', () => {
      req.destroy();
      resolve(false);
    });
    req.end();
  });
}

// 获取模型提供商
async function getServiceProvider() {
  try {
    const response = await axios.get('http://127.0.0.1:16688/aog/v0.4/service_provider');
    const providers = response.data.data;
    if (Array.isArray(providers) && providers.length === 0) {
      return false;
    } else {
      return true;
    }
  } catch (error) {
    throw new Error('❌ 获取模型提供商失败:', error.message);
  }
}

// 从服务器下载 aog
function downloadAOG() {
  return new Promise((resolve) => {
    const isMacOS = process.platform === 'darwin';
    const url = isMacOS
      ? 'https://smartvision-aipc-open.oss-cn-hangzhou.aliyuncs.com/aog/windows/aog.zip'
      : 'https://smartvision-aipc-open.oss-cn-hangzhou.aliyuncs.com/aog/windows/aog.exe';

    const userDir = os.homedir();
    const destDir = path.join(userDir, 'AOG');
    const destFileName = isMacOS ? 'aog.zip' : 'aog.exe';
    const dest = path.join(destDir, destFileName);

    fs.mkdir(destDir, { recursive: true }, async (err) => {
      if (err) {
        console.error('❌ 创建目录失败:', err.message);
        return resolve(false);
      }

      console.log('🔍 正在下载文件:', url);
      const file = fs.createWriteStream(dest);

      const request = http.get(url, (res) => {
        if (res.statusCode !== 200) {
          console.error(`❌ 下载失败，HTTP 状态码: ${res.statusCode}`);
          file.close();
          fs.unlink(dest, () => { });
          return resolve(false);
        }

        res.pipe(file);
        file.on('finish', async () => {
          file.close();
          console.log('✅ 下载完成:', dest);

          // macOS解压处理
          if (isMacOS) {
            try {
              const zip = new AdmZip(dest);
              zip.extractAllTo(destDir, true);
              console.log('✅ 解压完成');

              // 删除原始ZIP文件
              fs.unlinkSync(dest);

              // 设置可执行权限（根据需要）
              const execPath = path.join(destDir, 'aog');
              if (fs.existsSync(execPath)) {
                fs.chmodSync(execPath, 0o755);
              }
            } catch (e) {
              console.error('❌ 解压失败:', e.message);
              return resolve(false);
            }
          }

          // 添加环境变量
          const done = await AddToUserPath(destDir);
          resolve(done);
        });
      });

      request.on('error', (err) => {
        console.error('❌ 下载失败:', err.message);
        file.close();
        fs.unlink(dest, () => { });
        resolve(false);
      });
    });
  });
}

// 启动 aog 服务
function installAOG() {
  return new Promise((resolve) => {
    const isMacOS = process.platform === 'darwin';
    const userDir = os.homedir();
    const aogDir = path.join(userDir, 'AOG');

    // 确保PATH包含AOG目录（兼容跨平台）
    if (!process.env.PATH.includes(aogDir)) {
      process.env.PATH = `${process.env.PATH}${path.delimiter}${aogDir}`;
    }

    const child = spawn('aog', ['server', 'start', '-d'], {
      stdio: 'ignore',
      windowsHide: true
    });
    child.unref();

    child.on('error', (err) => {
      console.error(`❌ 启动失败: ${err.message}`);
      if (err.code === 'ENOENT') {
        console.log([
          '💡 可能原因:',
          `1. 未找到aog可执行文件，请检查下载是否成功`,
          `2. 环境变量未生效，请尝试重启终端`
        ].filter(Boolean).join('\n'));
      }
      resolve(false);
    });

    child.stdout.on('data', (data) => {
      console.log(`stdout: ${data}`);
      if (data.toString().includes('AOG server start successfully')) {
        resolve(true);
      }
    });

    child.stderr.on('data', (data) => {
      const errorMessage = data.toString().trim();
      if (errorMessage.includes('Install model engine failed')) {
        console.error('❌ 启动失败: 模型引擎安装失败。');
        resolve(false);
      }
      console.error(`stderr: ${errorMessage}`);
    });

    child.unref();
  });
}

// 导入配置文件
async function importConfig(filePath) {
  try {
    // 读取文件内容
    const data = await fsPromises.readFile(filePath, 'utf8');
    console.log('🔍 正在导入配置文件:', data);

    // 发送 POST 请求
    const res = await axios.post('http://127.0.0.1:16688/aog/v0.4/service/import', data, {
      headers: {
        'Content-Type': 'application/json',
      },
      validateStatus: () => true
    });
    console.log(res);

    // 验证响应
    if (res.status === 200) {
      console.log('✅ 配置文件导入成功');
      return true;
    } else {
      console.error(`❌ 配置文件导入失败，状态码: ${res.status}`);
      return false;
    }
  } catch (error) {
    console.error(`❌ 导入配置文件失败: ${error.message}`);
    return false;
  }
}

const installPromptHTML = `
<html>
<body style="padding:20px;font-family:Arial">
    <h2>安装确认</h2>
    <p>需要安装AOG组件才能继续，是否允许？</p>
    <button onclick="respond(true)">同意安装</button>
    <button onclick="respond(false)">取消</button>
    <script>
        function respond(choice) {
            fetch('/user-response?choice=' + choice)
                .then(() => window.close());
        }
    </script>
</body>
</html>
`;

async function AOGInit(aogFilePath = path.join(process.cwd(), '.aog')) {
  try {
    // 检查 AOG 是否可用
    const available = await isAOGAvailable();
    if (!available) {
      console.log('AOG 不可用，启动 Web 服务器等待用户确认...');
      startWebServer();
      openBrowser(`http://localhost:${webServerPort}/install-prompt`);

      // 等待用户响应或超时
      const choice = await Promise.race([
        waitForUserResponse(),
        new Promise((_, reject) => setTimeout(() => reject(new Error('用户响应超时')), 5 * 60 * 1000))
      ]);

      if (!choice) {
        console.log('用户取消了安装 AOG。');
        return;
      }

      // 下载并安装 AOG
      const downloaded = await downloadAOG();
      if (!downloaded) {
        console.error('下载 AOG 失败。');
        return;
      }

      const installed = await installAOG();
      if (!installed) {
        console.error('安装 AOG 失败。');
        return;
      }
    }

    console.log('✅ AOG 已启动，检查服务提供商...');

    // 检查服务提供商
    const hasServiceProvider = await getServiceProvider();
    if (!hasServiceProvider) {
      console.log('服务提供商不存在，尝试导入配置文件...');
      const imported = await importConfig(aogFilePath);
      if (imported) {
        console.log(`✅ 成功导入配置文件: ${aogFilePath}`);
      } else {
        console.error(`❌ 导入配置文件失败: ${aogFilePath}`);
      }
    } else {
      console.log('✅ 服务提供商已存在，无需导入配置文件。');
    }
  } catch (err) {
    console.error(`❌ AOG 初始化失败: ${err.message}`);
  }
}

module.exports = { AOGInit };
