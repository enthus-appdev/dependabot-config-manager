#!/usr/bin/env bash
set -e

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
PLATFORMS="darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64"

echo "Cross-compiling for multiple platforms..."
mkdir -p dist

for platform in $PLATFORMS; do
    os=$(echo $platform | cut -d/ -f1)
    arch=$(echo $platform | cut -d/ -f2)
    output="dist/dependabot-sync-${os}-${arch}"
    
    if [ "$os" = "windows" ]; then
        output="${output}.exe"
    fi
    
    echo "Building $output..."
    GOOS=$os GOARCH=$arch go build \
        -ldflags="-w -s -X main.Version=$VERSION" \
        -o "$output" ./cmd/dependabot-sync
done

echo "✅ Cross-compilation complete. Binaries in dist/"