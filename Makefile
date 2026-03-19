.PHONY: build-linux build-windows build-macos build-all clean

APP_NAME = overshare
BUILD_DIR = builds

build-all: build-linux build-windows build-macos
	@echo "✅ All builds complete!"
	@echo "📂 Files in $(BUILD_DIR)/:"

build-linux:
	@echo "📦 Building Linux amd64..."
	@GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 main.go
	@cd $(BUILD_DIR) && zip -q $(APP_NAME)-linux-amd64.zip $(APP_NAME)-linux-amd64
	@echo "  ✅ Linux: executable + ZIP (kept both)"

build-windows:
	@echo "📦 Building Windows amd64..."
	@GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe main.go
	@cd $(BUILD_DIR) && zip -q $(APP_NAME)-windows-amd64.zip $(APP_NAME)-windows-amd64.exe
	@echo "  ✅ Windows: .exe + ZIP (kept both)"

build-macos:
	@echo "📦 Building macOS (Intel)..."
	@GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-macos-amd64 main.go
	@cd $(BUILD_DIR) && zip -q $(APP_NAME)-macos-amd64.zip $(APP_NAME)-macos-amd64
	@echo "📦 Building macOS (Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(APP_NAME)-macos-arm64 main.go
	@cd $(BUILD_DIR) && zip -q $(APP_NAME)-macos-arm64.zip $(APP_NAME)-macos-arm64
	@echo "  ✅ macOS: executables + ZIP (kept both)"

clean:
	@rm -rf $(BUILD_DIR)
	@mkdir -p $(BUILD_DIR)
	@echo "🧹 Cleaned build directory"

help:
	@echo "Available commands:"
	@echo "  make build-all    - Build for all platforms (keeps executables + ZIPs)"
	@echo "  make build-linux  - Build Linux binary + ZIP"
	@echo "  make build-windows - Build Windows binary + ZIP"
	@echo "  make build-macos  - Build macOS binaries + ZIP"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make help         - Show this help"
