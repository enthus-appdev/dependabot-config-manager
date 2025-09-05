#!/usr/bin/env bash
set -e

# Build the binary
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
echo "Building dependabot-sync version $VERSION..."

go build -ldflags="-w -s -X main.Version=$VERSION" \
    -o bin/dependabot-sync ./cmd/dependabot-sync

echo "✅ Build complete: bin/dependabot-sync"