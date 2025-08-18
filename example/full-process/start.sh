#!/bin/bash

#*****************************************************************************
# Copyright 2024-2025 Intel Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#*****************************************************************************

# AOG智能内容创作助手启动脚本

echo "🤖 AOG智能内容创作助手启动脚本"
echo "=================================="

# 检查Node.js版本
echo "📋 检查系统环境..."
if ! command -v node &> /dev/null; then
    echo "❌ 错误: 未找到Node.js，请先安装Node.js 18.x或更高版本"
    exit 1
fi

NODE_VERSION=$(node -v | cut -d'v' -f2 | cut -d'.' -f1)
if [ "$NODE_VERSION" -lt 18 ]; then
    echo "❌ 错误: Node.js版本过低，当前版本: $(node -v)，需要18.x或更高版本"
    exit 1
fi

echo "✅ Node.js版本: $(node -v)"

# 检查npm
if ! command -v npm &> /dev/null; then
    echo "❌ 错误: 未找到npm"
    exit 1
fi

echo "✅ npm版本: $(npm -v)"

# 检查AOG服务是否运行
echo ""
echo "🔍 检查AOG服务状态..."
if curl -s http://localhost:16688/health > /dev/null 2>&1; then
    echo "✅ AOG服务正在运行 (localhost:16688)"
else
    echo "⚠️  警告: AOG服务未运行或不可访问"
    echo "   请确保AOG服务已启动: aog server start"
    echo "   或者检查服务是否运行在localhost:16688"
    echo ""
    read -p "是否继续启动Web Demo? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "❌ 启动已取消"
        exit 1
    fi
fi

# 检查依赖文件
echo ""
echo "📦 检查依赖文件..."

AOG_LIB_PATH="../image2image-web/aog-lib-1.2.23.tgz"
AOG_CHECKER_PATH="../image2image-web/aog-checker-1.2.0.tgz"

if [ ! -f "$AOG_LIB_PATH" ]; then
    echo "❌ 错误: 未找到aog-lib依赖文件: $AOG_LIB_PATH"
    echo "   请确保image2image-web示例存在并包含所需的依赖文件"
    exit 1
fi

if [ ! -f "$AOG_CHECKER_PATH" ]; then
    echo "❌ 错误: 未找到aog-checker依赖文件: $AOG_CHECKER_PATH"
    echo "   请确保image2image-web示例存在并包含所需的依赖文件"
    exit 1
fi

echo "✅ 依赖文件检查完成"

# 安装依赖
echo ""
echo "📥 安装项目依赖..."
if npm install; then
    echo "✅ 依赖安装完成"
else
    echo "❌ 错误: 依赖安装失败"
    exit 1
fi

# 创建上传目录
echo ""
echo "📁 创建上传目录..."
mkdir -p server/uploads/images
mkdir -p server/uploads/audio
mkdir -p server/uploads/camera
echo "✅ 上传目录创建完成"

# 启动服务
echo ""
echo "🚀 启动Web Demo服务..."
echo "   服务地址: http://localhost:3000"
echo "   按 Ctrl+C 停止服务"
echo ""

# 等待一下让用户看到信息
sleep 2

# 启动服务器
npm start
