#!/usr/bin/env node

/**
 * AOG智能内容创作助手 - 服务测试脚本
 * Copyright 2024-2025 Intel Corporation
 */

const http = require('http');

// 测试配置
const AOG_SERVER = 'http://localhost:16688';

// 颜色输出
const colors = {
    reset: '\x1b[0m',
    red: '\x1b[31m',
    green: '\x1b[32m',
    yellow: '\x1b[33m',
    blue: '\x1b[34m',
    magenta: '\x1b[35m',
    cyan: '\x1b[36m'
};

function colorLog(color, message) {
    console.log(`${colors[color]}${message}${colors.reset}`);
}

// HTTP请求工具
function makeRequest(url, options = {}) {
    return new Promise((resolve, reject) => {
        const req = http.request(url, options, (res) => {
            let data = '';
            res.on('data', chunk => data += chunk);
            res.on('end', () => {
                resolve({
                    statusCode: res.statusCode,
                    headers: res.headers,
                    data: data
                });
            });
        });
        
        req.on('error', reject);
        
        if (options.body) {
            req.write(options.body);
        }
        
        req.end();
    });
}

// 测试AOG服务连接
async function testAOGConnection() {
    colorLog('blue', '\n🔍 测试AOG服务连接...');
    
    try {
        const response = await makeRequest(`${AOG_SERVER}/health`);
        
        if (response.statusCode === 200) {
            colorLog('green', '✅ AOG服务连接正常');
            return true;
        } else {
            colorLog('red', `❌ AOG服务响应异常: HTTP ${response.statusCode}`);
            return false;
        }
    } catch (error) {
        colorLog('red', `❌ AOG服务连接失败: ${error.message}`);
        colorLog('yellow', '   请确保AOG服务已启动: aog server start');
        return false;
    }
}



// 检查Node.js版本
function checkNodeVersion() {
    colorLog('blue', '\n� 检查Node.js版本...');

    const nodeVersion = process.version;
    const majorVersion = parseInt(nodeVersion.slice(1).split('.')[0]);

    if (majorVersion >= 18) {
        colorLog('green', `✅ Node.js版本符合要求: ${nodeVersion}`);
        return true;
    } else {
        colorLog('red', `❌ Node.js版本过低: ${nodeVersion}`);
        colorLog('yellow', '   需要Node.js 18.x或更高版本');
        return false;
    }
}



// 测试依赖文件
async function testDependencies() {
    colorLog('blue', '\n📦 测试依赖文件...');
    
    const fs = require('fs');
    const path = require('path');
    
    const dependencies = [
        './aog-lib-1.3.0.tgz',
        './aog-checker-1.2.0.tgz'
    ];
    
    let allExists = true;
    
    dependencies.forEach(dep => {
        const fullPath = path.resolve(__dirname, dep);
        if (fs.existsSync(fullPath)) {
            colorLog('green', `✅ ${dep}`);
        } else {
            colorLog('red', `❌ ${dep} (文件不存在)`);
            allExists = false;
        }
    });
    
    if (allExists) {
        colorLog('green', '✅ 所有依赖文件检查通过');
    } else {
        colorLog('red', '❌ 部分依赖文件缺失');
        colorLog('yellow', '   请确保image2image-web示例存在并包含所需的依赖文件');
    }
    
    return allExists;
}

// 主测试函数
async function runTests() {
    colorLog('magenta', '🤖 AOG智能内容创作助手 - 环境检查');
    colorLog('magenta', '==========================================');

    const results = {
        nodeVersion: checkNodeVersion(),
        dependencies: await testDependencies(),
        aogConnection: await testAOGConnection()
    };

    // 测试结果汇总
    colorLog('magenta', '\n📊 检查结果汇总');
    colorLog('magenta', '==================');

    const testItems = [
        { name: 'Node.js版本', result: results.nodeVersion },
        { name: '依赖文件', result: results.dependencies },
        { name: 'AOG服务连接', result: results.aogConnection }
    ];

    let passCount = 0;
    testItems.forEach(item => {
        const icon = item.result ? '✅' : '❌';
        const color = item.result ? 'green' : 'red';
        colorLog(color, `${icon} ${item.name}`);
        if (item.result) passCount++;
    });

    const totalTests = testItems.length;
    const passRate = Math.round((passCount / totalTests) * 100);

    colorLog('magenta', `\n通过率: ${passCount}/${totalTests} (${passRate}%)`);

    if (passCount === totalTests) {
        colorLog('green', '\n🎉 环境检查通过！可以启动Web Demo。');
        colorLog('cyan', '   启动后访问: http://localhost:3000');
    } else {
        colorLog('red', '\n⚠️ 环境检查失败，请解决上述问题');
        if (!results.nodeVersion) {
            colorLog('yellow', '   请升级Node.js到18.x或更高版本');
        }
        if (!results.dependencies) {
            colorLog('yellow', '   请确保image2image-web示例存在并包含依赖文件');
        }
        if (!results.aogConnection) {
            colorLog('yellow', '   请启动AOG服务: aog server start');
        }
    }

    process.exit(passCount === totalTests ? 0 : 1);
}

// 运行测试
if (require.main === module) {
    runTests().catch(error => {
        colorLog('red', `\n💥 测试过程中发生错误: ${error.message}`);
        process.exit(1);
    });
}

module.exports = { runTests };
