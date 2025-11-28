# Plugin SDK - Protocol Makefile
# 用于生成gRPC代码

.PHONY: all generate clean install-tools

all: generate

# 生成gRPC代码
generate:
	@echo "🔧 Generating gRPC code from provider.proto..."
	set PATH="$$PATH:$$HOME/go/bin" && \
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       provider.proto
	@echo "✅ gRPC code generated successfully!"

# 安装必要的工具
install-tools:
	@echo "📦 Installing protoc plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "✅ Tools installed!"

# 清理生成的文件
clean:
	@echo "🧹 Cleaning generated files..."
	rm -f provider.pb.go provider_grpc.pb.go
	@echo "✅ Clean complete!"

# 检查protoc是否安装
check:
	@echo "🔍 Checking protoc installation..."
	@which protoc > /dev/null || (echo "❌ protoc not found! Please install Protocol Buffers compiler." && exit 1)
	@which protoc-gen-go > /dev/null || (echo "❌ protoc-gen-go not found! Run 'make install-tools'" && exit 1)
	@which protoc-gen-go-grpc > /dev/null || (echo "❌ protoc-gen-go-grpc not found! Run 'make install-tools'" && exit 1)
	@echo "✅ All required tools are installed!"

