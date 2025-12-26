# OVMS Plugin - Cross-platform Build Makefile

VERSION ?= 1.0.0
BINARY_NAME = ovms-plugin

# Supported platforms
PLATFORMS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64

# Go build flags
GOFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"

# Default: build for current platform
.PHONY: build
build:
	@echo "Building for current platform..."
	@go build $(GOFLAGS) -o $(BINARY_NAME) .

# Build all platforms
.PHONY: build-all
build-all: $(PLATFORMS)
	@echo "All platforms built successfully!"
	@echo "Binary sizes:"
	@find bin/ -name "$(BINARY_NAME)*" -exec ls -lh {} \;

# Platform-specific build rules
.PHONY: linux-amd64
linux-amd64:
	@echo "Building for linux/amd64..."
	@mkdir -p bin/linux-amd64
	@GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o bin/linux-amd64/$(BINARY_NAME) .
	@chmod +x bin/linux-amd64/$(BINARY_NAME)

.PHONY: linux-arm64
linux-arm64:
	@echo "Building for linux/arm64..."
	@mkdir -p bin/linux-arm64
	@GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -o bin/linux-arm64/$(BINARY_NAME) .
	@chmod +x bin/linux-arm64/$(BINARY_NAME)

.PHONY: darwin-amd64
darwin-amd64:
	@echo "Building for darwin/amd64..."
	@mkdir -p bin/darwin-amd64
	@GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -o bin/darwin-amd64/$(BINARY_NAME) .
	@chmod +x bin/darwin-amd64/$(BINARY_NAME)

.PHONY: darwin-arm64
darwin-arm64:
	@echo "Building for darwin/arm64..."
	@mkdir -p bin/darwin-arm64
	@GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -o bin/darwin-arm64/$(BINARY_NAME) .
	@chmod +x bin/darwin-arm64/$(BINARY_NAME)

.PHONY: windows-amd64
windows-amd64:
	@echo "Building for windows/amd64..."
	@mkdir -p bin/windows-amd64
	@GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o bin/windows-amd64/$(BINARY_NAME).exe .
	@chmod +x bin/windows-amd64/$(BINARY_NAME).exe

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f $(BINARY_NAME) $(BINARY_NAME).exe

# Verify build artifacts
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

# Help
.PHONY: help
help:
	@echo "OVMS Plugin Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build       - Build for current platform"
	@echo "  build-all   - Build for all supported platforms"
	@echo "  clean       - Remove all build artifacts"
	@echo "  verify      - Verify all platform builds exist"
	@echo "  help        - Show this help message"
	@echo ""
	@echo "Supported Platforms:"
	@echo "  $(PLATFORMS)"
	@echo ""
	@echo "Environment Variables:"
	@echo "  VERSION     - Version to embed in binaries (default: $(VERSION))"
