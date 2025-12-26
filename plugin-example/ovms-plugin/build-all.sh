#!/bin/bash
#*****************************************************************************
# Copyright 2024-2025 Intel Corporation
# 
# OVMS Plugin - Cross-platform Build Script
#*****************************************************************************

set -e

# 配置
VERSION=${VERSION:-"1.0.0"}
BINARY_NAME="ovms-plugin"

echo "🚀 Building ovms-plugin v${VERSION} for all platforms..."
echo ""

# 确保bin目录存在
mkdir -p bin/{linux-amd64,linux-arm64,darwin-amd64,darwin-arm64,windows-amd64}

# 构建参数
LDFLAGS="-s -w -X main.version=${VERSION}"

# 构建各平台
echo "📦 Building for linux/amd64..."
GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o bin/linux-amd64/${BINARY_NAME} .
chmod +x bin/linux-amd64/${BINARY_NAME}

echo "📦 Building for linux/arm64..."
GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o bin/linux-arm64/${BINARY_NAME} .
chmod +x bin/linux-arm64/${BINARY_NAME}

echo "📦 Building for darwin/amd64..."
GOOS=darwin GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o bin/darwin-amd64/${BINARY_NAME} .
chmod +x bin/darwin-amd64/${BINARY_NAME}

echo "📦 Building for darwin/arm64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o bin/darwin-arm64/${BINARY_NAME} .
chmod +x bin/darwin-arm64/${BINARY_NAME}

echo "📦 Building for windows/amd64..."
GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o bin/windows-amd64/${BINARY_NAME}.exe .
chmod +x bin/windows-amd64/${BINARY_NAME}.exe

echo ""
echo "✅ All builds completed successfully!"
echo ""
echo "📊 Binary sizes:"
find bin/ -name "${BINARY_NAME}*" -exec ls -lh {} \;

echo ""
echo "🔍 Verification:"

# 验证所有文件存在和权限
PLATFORMS=("linux-amd64" "linux-arm64" "darwin-amd64" "darwin-arm64" "windows-amd64")
ALL_GOOD=true

for platform in "${PLATFORMS[@]}"; do
    if [ "$platform" = "windows-amd64" ]; then
        EXPECTED_FILE="bin/${platform}/${BINARY_NAME}.exe"
    else
        EXPECTED_FILE="bin/${platform}/${BINARY_NAME}"
    fi
    
    if [ -f "$EXPECTED_FILE" ]; then
        # 检查文件权限
        PERMS=$(ls -l "$EXPECTED_FILE" | awk '{print $1}')
        echo "✅ ${platform}: $EXPECTED_FILE ($PERMS)"
    else
        echo "❌ ${platform}: $EXPECTED_FILE (MISSING)"
        ALL_GOOD=false
    fi
done

if [ "$ALL_GOOD" = true ]; then
    echo ""
    echo "🎉 All platform builds verified successfully!"
    echo "📦 Ready for distribution with bin/ directory structure"
else
    echo ""
    echo "❌ Some builds failed verification"
    exit 1
fi