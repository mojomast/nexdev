#!/bin/bash

# Geoffrey Install Script
# Installs geoffrussy binary to system PATH

set -e

echo "╔════════════════════════════════════════════════════════════╗"
echo "║         Geoffrey Installation Script                               ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Build Geoffrey
echo "🔨 Building Geoffrey..."
go build -o geoffrussy ./cmd/geoffrussy
echo "✅ Build complete!"
echo ""

# Detect OS and set install path
OS_TYPE=$(uname -s)
case $OS_TYPE in
    Linux)
        INSTALL_DIR="/usr/local/bin"
        echo "📦 Detected: Linux"
        echo "📁 Install directory: $INSTALL_DIR"
        ;;
    Darwin)
        INSTALL_DIR="/usr/local/bin"
        echo "📦 Detected: macOS"
        echo "📁 Install directory: $INSTALL_DIR"
        ;;
    CYGWIN*|MINGW*|MSYS*)
        INSTALL_DIR="$HOME/bin"
        echo "📦 Detected: Windows (Git Bash)"
        echo "📁 Install directory: $INSTALL_DIR"
        ;;
    *)
        echo "❌ Unknown OS: $OS_TYPE"
        echo "   Please install manually:"
        echo "   sudo cp geoffrussy /usr/local/bin/"
        exit 1
        ;;
esac

# Check if install directory exists
if [ ! -d "$INSTALL_DIR" ]; then
    echo "⚠️  Install directory does not exist: $INSTALL_DIR"
    read -p "Create it? (y/N): " create_dir
    if [ "$create_dir" = "y" ] || [ "$create_dir" = "Y" ]; then
        sudo mkdir -p "$INSTALL_DIR"
    else
        echo "❌ Installation cancelled"
        exit 1
    fi
fi

# Install binary
echo ""
echo "📦 Installing geoffrussy to $INSTALL_DIR..."
if [ -w "$INSTALL_DIR" ]; then
    # Directory is writable without sudo
    cp geoffrussy "$INSTALL_DIR/geoffrussy"
    chmod +x "$INSTALL_DIR/geoffrussy"
else
    # Need sudo
    echo "   ⚠️  Requires sudo privileges"
    sudo cp geoffrussy "$INSTALL_DIR/geoffrussy"
    sudo chmod +x "$INSTALL_DIR/geoffrussy"
fi
echo "✅ Installation complete!"
echo ""

# Verify installation
echo "🔍 Verifying installation..."
if [ -x "$INSTALL_DIR/geoffrussy" ]; then
    INSTALLED_VERSION=$("$INSTALL_DIR/geoffrussy" version)
    echo "✅ Geoffrey installed successfully!"
    echo "   Location: $INSTALL_DIR/geoffrussy"
    echo "   Version: $INSTALLED_VERSION"
else
    echo "❌ Installation failed!"
    echo "   Binary not found at: $INSTALL_DIR/geoffrussy"
    exit 1
fi

# Check if in PATH
echo ""
echo "🔍 Checking PATH..."
if command -v geoffrussy &> /dev/null; then
    WHICH_GEOFFRUSSY=$(which geoffrussy)
    echo "✅ Geoffrey is in your PATH!"
    echo "   Location: $WHICH_GEOFFRUSSY"
else
    echo "⚠️  Geoffrey is not in your PATH yet!"
    echo ""
    echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
    case $OS_TYPE in
        Linux|Darwin)
            echo ""
            echo "  if [ -d \"/usr/local/bin\" ]; then"
            echo "    export PATH=\"/usr/local/bin:\$PATH\""
            echo "  fi"
            ;;
        *)
            echo ""
            echo "  export PATH=\"$HOME/bin:\$PATH\""
            ;;
    esac
    echo ""
    echo "Then reload your shell:"
    echo "  source ~/.bashrc   # or ~/.zshrc"
    echo ""
fi

echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                    Installation Complete!                     ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "🎉 You can now use 'geoffrussy' from any directory!"
echo ""
echo "Quick test:"
echo "  cd /tmp"
echo "  mkdir test-project && cd test-project"
echo "  geoffrussy init"
echo ""
