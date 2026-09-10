#!/bin/bash

# Configuration
PI_USER="pi"
PI_HOST="192.168.178.128" # Replace with your Pi's actual IP if changed
REMOTE_DIR="/home/$PI_USER/beehive-sim"

# Path to the C++ rpi-rgb-led-matrix library source on your host machine
MATRIX_PATH="$HOME/rpi-rgb-led-matrix"

echo "==> Cross-compiling ARM64 binary with 'matrix' build tag..."

export CGO_ENABLED=1
export GOOS=linux
export GOARCH=arm64
export CC=aarch64-linux-gnu-gcc

# CGO flags pointing to your matrix header and compiled lib
export CGO_CFLAGS="-I$MATRIX_PATH/include"
export CGO_LDFLAGS="-L$MATRIX_PATH/lib -lrgbmatrix -lstdc++"

# Compile passing the 'matrix' build tag
go build -tags matrix -o beehive-sim .

if [ $? -ne 0 ]; then
    echo "==> Build failed!"
    exit 1
fi

echo "==> Uploading binary to Pi ($PI_HOST)..."
ssh "$PI_USER@$PI_HOST" "mkdir -p $REMOTE_DIR"
# Upload to a temp file first to bypass Linux file lock
scp ./beehive-sim "$PI_USER@$PI_HOST:$REMOTE_DIR/beehive-sim.tmp"

echo "==> Replacing binary and restarting service..."
ssh "$PI_USER@$PI_HOST" "sudo mv $REMOTE_DIR/beehive-sim.tmp $REMOTE_DIR/beehive-sim && sudo systemctl restart beehive-sim"