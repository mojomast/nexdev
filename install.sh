#!/bin/bash

# Nexdev Install Script
# Installs nexdev binary and Pi extension files to system paths

set -e

echo "╔════════════════════════════════════════════════════════════╗"
echo "║         Nexdev Installation Script                                 ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Build Nexdev and Pi extension distribution
echo "🔨 Building Nexdev..."
go build -o nexdev ./cmd/nexdev
make pi-ext-build
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
            echo "   sudo cp nexdev /usr/local/bin/"
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
echo "📦 Installing nexdev to $INSTALL_DIR..."
if [ -w "$INSTALL_DIR" ]; then
    # Directory is writable without sudo
    cp nexdev "$INSTALL_DIR/nexdev"
    chmod +x "$INSTALL_DIR/nexdev"
else
    # Need sudo
    echo "   ⚠️  Requires sudo privileges"
    sudo cp nexdev "$INSTALL_DIR/nexdev"
    sudo chmod +x "$INSTALL_DIR/nexdev"
fi

SHARE_DIR="/usr/local/share/nexdev/pi-extension"
echo "📦 Installing Pi extension files to $SHARE_DIR..."
if [ -w "$(dirname "$SHARE_DIR")" ]; then
    mkdir -p "$SHARE_DIR"
    cp -R bin/pi-extension/. "$SHARE_DIR/"
else
    sudo mkdir -p "$SHARE_DIR"
    sudo cp -R bin/pi-extension/. "$SHARE_DIR/"
fi
echo "✅ Installation complete!"
echo ""

# Verify installation
echo "🔍 Verifying installation..."
if [ -x "$INSTALL_DIR/nexdev" ]; then
    INSTALLED_VERSION=$("$INSTALL_DIR/nexdev" version)
    echo "✅ Nexdev installed successfully!"
    echo "   Location: $INSTALL_DIR/nexdev"
    echo "   Version: $INSTALLED_VERSION"
else
    echo "❌ Installation failed!"
    echo "   Binary not found at: $INSTALL_DIR/nexdev"
    exit 1
fi

# Check if in PATH
echo ""
echo "🔍 Checking PATH..."
if command -v nexdev &> /dev/null; then
    WHICH_NEXDEV=$(which nexdev)
    echo "✅ Nexdev is in your PATH!"
    echo "   Location: $WHICH_NEXDEV"
else
    echo "⚠️  Nexdev is not in your PATH yet!"
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
echo "🎉 You can now use 'nexdev' from any directory!"
echo ""
echo "Quick test:"
echo "  cd /tmp"
echo "  mkdir test-project && cd test-project"
echo "  nexdev init"
echo ""
