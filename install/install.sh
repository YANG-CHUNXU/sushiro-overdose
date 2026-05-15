#!/usr/bin/env bash
# sushiro-overdose one-click installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Ryujoxys/sushiro-overdose/master/install/install.sh | bash

set -euo pipefail

REPO="Ryujoxys/sushiro-overdose"
BINARY="sushiro-overdose"
INSTALL_DIR="/usr/local/bin"

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
LATEST_JSON=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest")
LATEST=$(printf '%s\n' "$LATEST_JSON" | sed -nE 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' | head -1)
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

ASSET_ARCH="$ARCH"
if [ "$OS" = "darwin" ]; then
    ASSET_ARCH="all"
fi

FILENAME="${BINARY}_${LATEST#v}_${OS}_${ASSET_ARCH}.${SUFFIX}"
URL="https://github.com/$REPO/releases/download/$LATEST/$FILENAME"

TMP_DIR=$(mktemp -d)
cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

ARCHIVE_PATH="$TMP_DIR/$FILENAME"
EXTRACT_DIR="$TMP_DIR/extract"
mkdir -p "$EXTRACT_DIR"

echo "Downloading $URL..."
curl -fsSL -o "$ARCHIVE_PATH" "$URL"

# Extract
echo "Extracting..."
if [ "$SUFFIX" = "zip" ]; then
    unzip -q "$ARCHIVE_PATH" -d "$EXTRACT_DIR"
else
    tar xzf "$ARCHIVE_PATH" -C "$EXTRACT_DIR"
fi

# Install
if [ "$OS" = "windows" ]; then
    BINARY_FILE="${BINARY}.exe"
else
    BINARY_FILE="$BINARY"
fi

BINARY_PATH=$(find "$EXTRACT_DIR" -type f -name "$BINARY_FILE" -print -quit)
if [ -z "$BINARY_PATH" ] || [ ! -f "$BINARY_PATH" ]; then
    echo "Archive did not contain expected binary: $BINARY_FILE"
    exit 1
fi

if [ -d "$INSTALL_DIR" ] && [ -w "$INSTALL_DIR" ]; then
    install -m 0755 "$BINARY_PATH" "$INSTALL_DIR/$BINARY"
else
    echo "需要 sudo 权限安装到 $INSTALL_DIR"
    sudo install -d -m 0755 "$INSTALL_DIR"
    sudo install -m 0755 "$BINARY_PATH" "$INSTALL_DIR/$BINARY"
fi

echo ""
echo "✓ sushiro-overdose $LATEST installed to $INSTALL_DIR/$BINARY"
echo "  运行 sushiro-overdose 开始使用"
