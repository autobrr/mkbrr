#!/bin/bash

set -e

# Check if we're on Linux and x86_64
if [ "$(uname)" != "Linux" ] || [ "$(uname -m)" != "x86_64" ]; then
    echo "ISA-L crypto installation is only supported on Linux x86_64"
    exit 0
fi

# Check if ISA-L is already installed
if [ -f "/usr/local/lib/libisal_crypto.so" ]; then
    echo "ISA-L crypto is already installed"
    exit 0
fi

# Install build dependencies
if command -v apt-get &> /dev/null; then
    sudo apt-get update
    sudo apt-get install -y build-essential automake autoconf libtool nasm
elif command -v yum &> /dev/null; then
    sudo yum groupinstall -y "Development Tools"
    sudo yum install -y automake autoconf libtool nasm
else
    echo "Unsupported package manager. Please install build dependencies manually."
    exit 1
fi

# Clone and build ISA-L crypto
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

git clone --depth 1 https://github.com/intel/isa-l_crypto.git
cd isa-l_crypto

./autogen.sh
./configure
make
sudo make install

# Update library cache
sudo ldconfig

# Cleanup
cd ..
rm -rf "$TEMP_DIR"

echo "ISA-L crypto installed successfully" 