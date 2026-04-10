#!/bin/bash
# Builds release binaries for Linux, Windows, and macOS.
# Output goes to ./releases/
#
# Usage:  ./build-releases.sh

set -e

OUT=releases
mkdir -p "$OUT"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS="-s -w -X main.version=$VERSION"

echo "Building Cocobase $VERSION..."

# Linux amd64 — CGO enabled (supports SQLite + PostgreSQL)
echo "  linux/amd64..."
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -ldflags="$LDFLAGS" -o "$OUT/cocobase-linux-amd64" ./cmd/cocobase/

# Windows amd64 — CGO disabled (PostgreSQL only; use DATABASE_URL=postgres://...)
echo "  windows/amd64..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -ldflags="$LDFLAGS" -o "$OUT/cocobase-windows-amd64.exe" ./cmd/cocobase/

# macOS Intel — CGO disabled (PostgreSQL only)
echo "  darwin/amd64 (Intel)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
  go build -ldflags="$LDFLAGS" -o "$OUT/cocobase-macos-amd64" ./cmd/cocobase/

# macOS Apple Silicon — CGO disabled (PostgreSQL only)
echo "  darwin/arm64 (Apple Silicon)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
  go build -ldflags="$LDFLAGS" -o "$OUT/cocobase-macos-arm64" ./cmd/cocobase/

echo ""
echo "Done. Binaries in ./$OUT/:"
ls -lh "$OUT/"
echo ""
echo "Note: Linux binary supports SQLite. Windows/macOS binaries require PostgreSQL (DATABASE_URL=postgres://...)."
