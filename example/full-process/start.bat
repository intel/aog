@echo off
REM *****************************************************************************
REM Copyright 2024-2025 Intel Corporation
REM
REM Licensed under the Apache License, Version 2.0 (the "License");
REM you may not use this file except in compliance with the License.
REM You may obtain a copy of the License at
REM
REM     http://www.apache.org/licenses/LICENSE-2.0
REM
REM Unless required by applicable law or agreed to in writing, software
REM distributed under the License is distributed on an "AS IS" BASIS,
REM WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
REM See the License for the specific language governing permissions and
REM limitations under the License.
REM *****************************************************************************

REM AOG智能内容创作助手启动脚本 (Windows)

echo 🤖 AOG智能内容创作助手启动脚本
echo ==================================

REM 检查Node.js
echo 📋 检查系统环境...
where node >nul 2>nul
if %errorlevel% neq 0 (
    echo ❌ 错误: 未找到Node.js，请先安装Node.js 18.x或更高版本
    pause
    exit /b 1
)

for /f "tokens=1 delims=v" %%i in ('node -v') do set NODE_VERSION=%%i
echo ✅ Node.js版本: %NODE_VERSION%

REM 检查npm
where npm >nul 2>nul
if %errorlevel% neq 0 (
    echo ❌ 错误: 未找到npm
    pause
    exit /b 1
)

for /f %%i in ('npm -v') do set NPM_VERSION=%%i
echo ✅ npm版本: %NPM_VERSION%

REM 检查AOG服务
echo.
echo 🔍 检查AOG服务状态...
curl -s http://localhost:16688/health >nul 2>nul
if %errorlevel% equ 0 (
    echo ✅ AOG服务正在运行 (localhost:16688)
) else (
    echo ⚠️  警告: AOG服务未运行或不可访问
    echo    请确保AOG服务已启动: aog server start
    echo    或者检查服务是否运行在localhost:16688
    echo.
    set /p continue="是否继续启动Web Demo? (y/N): "
    if /i not "%continue%"=="y" (
        echo ❌ 启动已取消
        pause
        exit /b 1
    )
)

REM 检查依赖文件
echo.
echo 📦 检查依赖文件...

set AOG_LIB_PATH=.\aog-lib-1.3.0.tgz
set AOG_CHECKER_PATH=.\aog-checker-1.2.0.tgz

if not exist "%AOG_LIB_PATH%" (
    echo ❌ 错误: 未找到aog-lib依赖文件: %AOG_LIB_PATH%
    echo    请确保image2image-web示例存在并包含所需的依赖文件
    pause
    exit /b 1
)

if not exist "%AOG_CHECKER_PATH%" (
    echo ❌ 错误: 未找到aog-checker依赖文件: %AOG_CHECKER_PATH%
    echo    请确保image2image-web示例存在并包含所需的依赖文件
    pause
    exit /b 1
)

echo ✅ 依赖文件检查完成

REM 安装依赖
echo.
echo 📥 安装项目依赖...
call npm install
if %errorlevel% neq 0 (
    echo ❌ 错误: 依赖安装失败
    pause
    exit /b 1
)
echo ✅ 依赖安装完成

REM 创建上传目录
echo.
echo 📁 创建上传目录...
if not exist "server\uploads\images" mkdir server\uploads\images
if not exist "server\uploads\audio" mkdir server\uploads\audio
if not exist "server\uploads\camera" mkdir server\uploads\camera
echo ✅ 上传目录创建完成

REM 启动服务
echo.
echo 🚀 启动Web Demo服务...
echo    服务地址: http://localhost:3000
echo    按 Ctrl+C 停止服务
echo.

REM 等待一下让用户看到信息
timeout /t 2 /nobreak >nul

REM 启动服务器
call npm start

pause
