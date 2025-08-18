#!/bin/bash

# AOG MCP Server DXT Build Script
set -e

echo "🚀 Building AOG MCP Server DXT Package..."

# Check if we're in the right directory
if [[ ! -f "manifest.json" ]]; then
    echo "❌ Error: manifest.json not found. Please run this script from the mcp-server directory."
    exit 1
fi

# Clean previous builds
echo "🧹 Cleaning previous builds..."
rm -rf build/
mkdir -p build/dxt-package

# Build binaries for different platforms
echo "🔨 Building cross-platform binaries..."

# macOS AMD64
echo "  Building for macOS (amd64)..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o build/aog-mcp-server-darwin-amd64 ./cmd/aog-mcp-server/

# macOS ARM64 
echo "  Building for macOS (arm64)..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o build/aog-mcp-server-darwin-arm64 ./cmd/aog-mcp-server/

# Windows AMD64
echo "  Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o build/aog-mcp-server-windows-amd64.exe ./cmd/aog-mcp-server/

# Copy main binary (use the current platform's binary as default)
echo "📋 Preparing DXT package structure..."

# Determine current platform and copy appropriate binary
if [[ "$OSTYPE" == "darwin"* ]]; then
    if [[ $(uname -m) == "arm64" ]]; then
        cp build/aog-mcp-server-darwin-arm64 build/dxt-package/aog-mcp-server
    else
        cp build/aog-mcp-server-darwin-amd64 build/dxt-package/aog-mcp-server
    fi
elif [[ "$OSTYPE" == "msys" ]]; then
    cp build/aog-mcp-server-windows-amd64.exe build/dxt-package/aog-mcp-server.exe
fi

# Copy required files
cp manifest.json build/dxt-package/
cp README.md build/dxt-package/

# Create LICENSE file if it doesn't exist
if [[ ! -f "LICENSE" ]]; then
    echo "📄 Creating LICENSE file..."
    cat > build/dxt-package/LICENSE << EOF
Apache License
Version 2.0, January 2004
http://www.apache.org/licenses/

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
EOF
else
    cp LICENSE build/dxt-package/
fi

# Validate manifest.json
echo "🔍 Validating manifest.json..."
if command -v jq &> /dev/null; then
    cat build/dxt-package/manifest.json | jq . > /dev/null
    echo "✅ manifest.json is valid JSON"
else
    echo "⚠️  jq not found, skipping JSON validation"
fi

# Set executable permissions
chmod +x build/dxt-package/aog-mcp-server 2>/dev/null || true

# Create DXT archive
echo "📦 Creating DXT archive..."
cd build/dxt-package
zip -r ../aog-mcp-server.dxt . -x "*.DS_Store"
cd ../..

# Verify the archive
echo "🔍 Verifying DXT archive..."
unzip -l build/aog-mcp-server.dxt

# Calculate file size
DXT_SIZE=$(du -h build/aog-mcp-server.dxt | cut -f1)
echo "✅ DXT package created successfully!"
echo "📁 Location: build/aog-mcp-server.dxt"
echo "📏 Size: $DXT_SIZE"

# Test archive integrity
echo "🧪 Testing archive integrity..."
if unzip -t build/aog-mcp-server.dxt > /dev/null; then
    echo "✅ Archive integrity verified"
else
    echo "❌ Archive integrity check failed"
    exit 1
fi

echo ""
echo "🎉 DXT build completed successfully!"
echo ""
echo "Next steps:"
echo "1. Test the DXT package in a compatible application"
echo "2. Distribute via GitHub releases or extension directories"
echo "3. Users can install by opening the .dxt file in Claude for Desktop"
echo ""