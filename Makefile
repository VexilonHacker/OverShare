.PHONY: build-linux build-windows build-macos build-all clean

APP_NAME = overshare
BUILD_DIR = builds

build-all: build-linux build-windows build-macos
	@echo "✅ All builds complete!"

build-linux:
	@echo "📦 Building Linux amd64..."
	@GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 main.go
	@cd $(BUILD_DIR) && tar -czf $(APP_NAME)-linux-amd64.tar.gz $(APP_NAME)-linux-amd64 2>/dev/null
	@echo "  ✅ Linux build complete"

build-windows:
	@echo "📦 Building Windows amd64..."
	@GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe main.go
	@cd $(BUILD_DIR) && zip -q $(APP_NAME)-windows-amd64.zip $(APP_NAME)-windows-amd64.exe
	@echo "  ✅ Windows build complete"

build-macos:
	@echo "📦 Building macOS (Intel)..."
	@GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-macos-amd64 main.go
	@echo "📦 Building macOS (Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(APP_NAME)-macos-arm64 main.go
	@cd $(BUILD_DIR) && tar -czf $(APP_NAME)-macos-amd64.tar.gz $(APP_NAME)-macos-amd64 2>/dev/null
	@cd $(BUILD_DIR) && tar -czf $(APP_NAME)-macos-arm64.tar.gz $(APP_NAME)-macos-arm64 2>/dev/null
	@echo "  ✅ macOS build complete"

clean:
	@rm -rf $(BUILD_DIR)
	@mkdir -p $(BUILD_DIR)
	@echo "🧹 Cleaned build directory"

help:
	@echo "Available commands:"
	@echo "  make build-all    - Build for all platforms"
	@echo "  make build-linux  - Build Linux binary"
	@echo "  make build-windows - Build Windows binary"
	@echo "  make build-macos  - Build macOS binaries"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make help         - Show this help"
