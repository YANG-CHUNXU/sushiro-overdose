#!/bin/bash
set -e

cd "$(dirname "$0")"

# Build if binary doesn't exist or source changed
if [ ! -f sushiro ] || [ main.go -nt sushiro ]; then
    echo "正在编译..."
    go build -o sushiro . || { echo "编译失败"; exit 1; }
fi

# Install to /usr/local/bin if not already there or outdated
if [ ! -f /usr/local/bin/sushiro ] || [ sushiro -nt /usr/local/bin/sushiro ]; then
    echo "安装到 /usr/local/bin/sushiro ..."
    sudo cp sushiro /usr/local/bin/sushiro
fi

# Run
exec ./sushiro "$@"
