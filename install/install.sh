#!/usr/bin/env bash
# sushiro-overdose one-click installer
# Usage: curl -sSL https://raw.githubusercontent.com/Ryujoxys/sushiro-overdose/master/install.sh | bash

set -euo pipefail

REPO="Ryujoxys/sushiro-overdose"
BINARY="sushiro-overdose"

echo "=== sushiro-overdose installer ==="

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
    darwin) OS="darwin" ;;
    linux)  OS="linux" ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "Detected: $OS/$ARCH"

# Find the latest release
LATEST=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$LATEST" ]; then
    echo "Could not determine latest version"
    exit 1
fi

echo "Latest version: $LATEST"

# Build download URL
if [ "$OS" = "windows" ]; then
    SUFFIX="zip"
else
    SUFFIX="tar.gz"
fi

FILENAME="${BINARY}_${LATEST#v}_${OS}_${ARCH}.${SUFFIX}"
URL="https://github.com/$REPO/releases/download/$LATEST/$FILENAME"

echo "Downloading $URL..."
curl -sSL -o "/tmp/$FILENAME" "$URL"

# Extract
INSTALL_DIR="/usr/local/bin"
echo "Extracting..."
cd /tmp
if [ "$SUFFIX" = "zip" ]; then
    unzip -o "$FILENAME"
else
    tar xzf "$FILENAME"
fi

# Install
if [ "$OS" = "windows" ]; then
    BINARY_FILE="${BINARY}.exe"
else
    BINARY_FILE="$BINARY"
fi

if [ -w "$INSTALL_DIR" ]; then
    cp "$BINARY_FILE" "$INSTALL_DIR/$BINARY"
else
    echo "需要 sudo 权限安装到 $INSTALL_DIR"
    sudo cp "$BINARY_FILE" "$INSTALL_DIR/$BINARY"
fi

chmod +x "$INSTALL_DIR/$BINARY"

# Cleanup
rm -f "/tmp/$FILENAME" "/tmp/$BINARY_FILE"

echo ""
echo "✓ sushiro-overdose $LATEST installed to $INSTALL_DIR/$BINARY"
echo "  运行 sushiro-overdose 开始使用"
