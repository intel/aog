# Get GOOS and GOARCH
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
SQLITE_VEC_DIR ?= $(abspath internal/datastore/sqlite/sqlite-vec)

# CGO 配置（用于测试）
export CGO_ENABLED=1
export CGO_LDFLAGS=-lsqlite3

.PHONY: help build-all test test-metadata test-verbose test-coverage clean-test lint fmt proto

# ============================================================================
# 原有构建命令
# ============================================================================

build-all:
ifeq ($(GOOS),windows)
	$(MAKE) build-cli-win
else ifeq ($(GOOS),darwin)
ifeq ($(GOARCH),amd64)
	$(MAKE) build-cli-darwin
else
	$(MAKE) build-cli-darwin-arm
endif
else ifeq ($(GOOS),linux)
	$(MAKE) build-cli-linux
else
	@echo "Unsupported platform: $(GOOS)"
endif


build-cli-win:
	set CGO_ENABLED=1 && set CGO_CFLAGS=-I$(SQLITE_VEC_DIR) && go build -o aog.exe -ldflags="-s -w"  cmd/cli/main.go

build-cli-darwin:
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64  CGO_CFLAGS=-I$(SQLITE_VEC_DIR) go build -o aog -ldflags="-s -w"  cmd/cli/main.go

build-cli-darwin-arm:
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64  CGO_CFLAGS=-I$(SQLITE_VEC_DIR) go build -o aog -ldflags="-s -w"  cmd/cli/main.go

build-cli-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CGO_CFLAGS=-I$(SQLITE_VEC_DIR) go build -o aog -ldflags="-s -w"  cmd/cli/main.go

build-dll-win:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -o AogChecker.dll -buildmode=c-shared checker/AogChecker.go

build-dll-darwin:
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o AogChecker.dylib -buildmode=c-shared checker/AogChecker.go

build-dll-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o AogChecker.so -buildmode=c-shared checker/AogChecker.go

# ============================================================================
# 新增测试命令（Phase 0）
# ============================================================================

help: ## 显示帮助信息
	@echo "============================================"
	@echo "AOG Makefile 帮助"
	@echo "============================================"
	@echo ""
	@echo "构建命令:"
	@echo "  make build-all          - 根据当前平台构建 CLI"
	@echo "  make build-cli-win      - 构建 Windows CLI"
	@echo "  make build-cli-darwin   - 构建 macOS CLI (amd64)"
	@echo "  make build-cli-darwin-arm - 构建 macOS CLI (arm64)"
	@echo "  make build-cli-linux    - 构建 Linux CLI"
	@echo "  make build-dll-*        - 构建动态库"
	@echo ""
	@echo "测试命令:"
	@echo "  make test               - 运行所有测试"
	@echo "  make test-metadata      - 运行 metadata 包测试"
	@echo "  make test-verbose       - 运行所有测试（详细输出）"
	@echo "  make test-coverage      - 生成测试覆盖率报告"
	@echo ""
	@echo "开发命令:"
	@echo "  make lint               - 运行代码检查"
	@echo "  make fmt                - 格式化代码"
	@echo "  make clean-test         - 清理测试产物"
	@echo ""

test: ## 运行所有测试
	@echo "运行所有测试..."
	go test ./...

test-plugin-sdk: ## 运行 plugin-sdk 测试
	@echo "运行 plugin-sdk 测试..."
	go test -v ./plugin-sdk/...

test-verbose: ## 运行所有测试（详细输出）
	@echo "运行所有测试（详细模式）..."
	go test -v ./...

test-coverage: ## 运行测试并生成覆盖率报告
	@echo "生成测试覆盖率报告..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

clean-test: ## 清理测试产物
	@echo "清理测试产物..."
	rm -f coverage.out coverage.html

lint: ## 运行代码检查
	@echo "运行 golangci-lint..."
	golangci-lint run ./...

fmt: ## 格式化代码
	@echo "格式化代码..."
	go fmt ./...
	gofmt -s -w .

# ============================================================================
# Plugin 相关命令
# ============================================================================

proto: ## 生成 protobuf 代码
	@echo "生成 protobuf 代码..."
	@command -v protoc >/dev/null 2>&1 || { echo "错误: protoc 未安装。请访问 https://grpc.io/docs/protoc-installation/ 安装"; exit 1; }
	@test -f $(shell go env GOPATH)/bin/protoc-gen-go || { echo "错误: protoc-gen-go 未安装。运行: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; exit 1; }
	@test -f $(shell go env GOPATH)/bin/protoc-gen-go-grpc || { echo "错误: protoc-gen-go-grpc 未安装。运行: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"; exit 1; }
	PATH="$(shell go env GOPATH)/bin:$$PATH" protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/plugin/protocol/provider.proto
	@echo "✅ protobuf 代码生成完成"

.DEFAULT_GOAL := help
