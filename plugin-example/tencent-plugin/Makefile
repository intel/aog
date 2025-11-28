# Tencent Plugin - Cross-platform Build Makefile
# Copyright 2024-2025 Intel Corporation

# 版本信息
VERSION ?= 1.0.0
BINARY_NAME = tencent-plugin

# 支持的平台
PLATFORMS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64

# Go构建参数
GOFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"

# 默认目标：构建当前平台
.PHONY: build
build:
	@echo "Building for current platform..."
	@go build $(GOFLAGS) -o $(BINARY_NAME) .

# 构建所有平台
.PHONY: build-all
build-all: $(PLATFORMS)
	@echo "All platforms built successfully!"
	@echo "Binary sizes:"
	@find bin/ -name "$(BINARY_NAME)*" -exec ls -lh {} \;

# 各平台构建规则
.PHONY: linux-amd64
linux-amd64:
	@echo "Building for linux/amd64..."
	@GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o bin/linux-amd64/$(BINARY_NAME) .

.PHONY: linux-arm64
linux-arm64:
	@echo "Building for linux/arm64..."
	@GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -o bin/linux-arm64/$(BINARY_NAME) .

.PHONY: darwin-amd64
darwin-amd64:
	@echo "Building for darwin/amd64..."
	@GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -o bin/darwin-amd64/$(BINARY_NAME) .

.PHONY: darwin-arm64
darwin-arm64:
	@echo "Building for darwin/arm64..."
	@GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -o bin/darwin-arm64/$(BINARY_NAME) .

.PHONY: windows-amd64
windows-amd64:
	@echo "Building for windows/amd64..."
	@GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o bin/windows-amd64/$(BINARY_NAME).exe .

# 清理构建产物
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f $(BINARY_NAME) $(BINARY_NAME).exe

# 验证构建产物
.PHONY: verify
verify:
	@echo "Verifying build artifacts..."
	@for platform in $(PLATFORMS); do \
		if [ "$$platform" = "windows-amd64" ]; then \
			if [ ! -f "bin/$$platform/$(BINARY_NAME).exe" ]; then \
				echo "❌ Missing: bin/$$platform/$(BINARY_NAME).exe"; \
				exit 1; \
			else \
				echo "✅ Found: bin/$$platform/$(BINARY_NAME).exe"; \
			fi; \
		else \
			if [ ! -f "bin/$$platform/$(BINARY_NAME)" ]; then \
				echo "❌ Missing: bin/$$platform/$(BINARY_NAME)"; \
				exit 1; \
			else \
				echo "✅ Found: bin/$$platform/$(BINARY_NAME)"; \
			fi; \
		fi; \
	done
	@echo "✅ All platforms verified successfully!"

# 打包分发
.PHONY: package
package: build-all verify
	@echo "Packaging plugin for distribution..."
	@tar --exclude='internal' --exclude='*.go' --exclude='go.mod' --exclude='go.sum' --exclude='Makefile' \
		-czf tencent-plugin-$(VERSION).tar.gz \
		plugin.yaml README.md env.example bin/
	@echo "✅ Package created: tencent-plugin-$(VERSION).tar.gz"
	@ls -lh tencent-plugin-$(VERSION).tar.gz

# 开发模式：构建当前平台并运行测试
.PHONY: dev
dev: build
	@echo "Development build completed for current platform"

# 显示帮助信息
.PHONY: help
help:
	@echo "Tencent Plugin Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build       - Build for current platform"
	@echo "  build-all   - Build for all supported platforms"
	@echo "  clean       - Remove all build artifacts"
	@echo "  verify      - Verify all platform builds exist"
	@echo "  package     - Create distribution package"
	@echo "  dev         - Development build (current platform)"
	@echo "  help        - Show this help message"
	@echo ""
	@echo "Supported Platforms:"
	@echo "  $(PLATFORMS)"
	@echo ""
	@echo "Environment Variables:"
	@echo "  VERSION     - Version to embed in binaries (default: $(VERSION))"
