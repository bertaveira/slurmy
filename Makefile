.PHONY: build build-linux-amd64 build-linux-arm64 clean

# Default: build for Linux amd64
build: build-linux-amd64

# Build for Linux amd64 (most common)
build-linux-amd64:
	@echo "Building for Linux amd64..."
	GOOS=linux GOARCH=amd64 go build -o slurmy-linux-amd64 .
	@echo "✓ Built slurmy-linux-amd64"

# Build for Linux arm64 (for newer clusters)
build-linux-arm64:
	@echo "Building for Linux arm64..."
	GOOS=linux GOARCH=arm64 go build -o slurmy-linux-arm64 .
	@echo "✓ Built slurmy-linux-arm64"

# Build both architectures
build-all: build-linux-amd64 build-linux-arm64
	@echo "Build complete! Binaries ready:"
	@ls -lh slurmy-linux-*

# Clean build artifacts
clean:
	rm -f slurmy-linux-amd64 slurmy-linux-arm64 slurmy
