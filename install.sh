#!/bin/bash
#
# MUXI Server Installation Script
# https://muxi.org/install (or https://get.muxi.org)
#
# Usage:
#   System install (Linux):  curl -sSL https://muxi.org/install | sudo bash
#   User install:            curl -sSL https://muxi.org/install | bash
#

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Constants
REPO="muxi-ai/server"
INSTALL_VERSION="${MUXI_VERSION:-latest}"  # Allow override with MUXI_VERSION env var

# Detect platform
OS="$(uname -s)"
ARCH="$(uname -m)"

# Map architecture names
case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}✗ Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

# Map OS names
case "$OS" in
    Linux)
        OS="linux"
        ;;
    Darwin)
        OS="darwin"
        ;;
    *)
        echo -e "${RED}✗ Unsupported operating system: $OS${NC}"
        echo "  MUXI Server supports Linux and macOS"
        exit 1
        ;;
esac

# Helper functions
print_header() {
    echo ""
    echo "███╗   ███╗██╗   ██╗██╗  ██╗██╗"
    echo "████╗ ████║██║   ██║╚██╗██╔╝██║"
    echo "██╔████╔██║██║   ██║ ╚███╔╝ ██║"
    echo "██║╚██╔╝██║██║   ██║ ██╔██╗ ██║"
    echo "██║ ╚═╝ ██║╚██████╔╝██╔╝ ██╗██║"
    echo "╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝"
    echo ""
    echo "MUXI Server Installation"
    echo ""
}

info() {
    echo -e "${BLUE}→${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

warn() {
    echo -e "${YELLOW}!${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

# Check if running as root
IS_ROOT=0
if [ "$EUID" = 0 ]; then
    IS_ROOT=1
fi

# Determine installation type
if [ "$IS_ROOT" = 1 ] && [ "$OS" = "linux" ]; then
    INSTALL_TYPE="system"
    INSTALL_DIR="/usr/local/bin"
    CONFIG_DIR="/etc/muxi/server"
    DATA_DIR="/var/lib/muxi"
    LOG_DIR="/var/log/muxi"
else
    INSTALL_TYPE="user"
    INSTALL_DIR="$HOME/.local/bin"
    CONFIG_DIR="$HOME/.muxi/server"
    DATA_DIR="$HOME/.muxi/server"
    LOG_DIR="$HOME/.muxi/server/logs"
fi

# Print header
print_header

# Show installation details
if [ "$INSTALL_TYPE" = "system" ]; then
    info "Installing MUXI Server (system-wide)"
    info "Platform: $OS/$ARCH"
    info "Binary: $INSTALL_DIR/muxi-server"
    info "Config: $CONFIG_DIR"
    info "Data:   $DATA_DIR"
    info "Logs:   $LOG_DIR"
else
    info "Installing MUXI Server (user-level)"
    info "Platform: $OS/$ARCH"
    info "Binary: $INSTALL_DIR/muxi-server"
    info "Paths:  ~/.muxi/server/"
    
    if [ "$OS" = "darwin" ]; then
        warn "macOS detected: Using user-level install (no system paths)"
    fi
fi

echo ""

# Check for required commands
if ! command -v curl >/dev/null 2>&1; then
    error "curl is required but not installed"
    echo "  Install with: sudo apt install curl  (Debian/Ubuntu)"
    echo "  Install with: sudo yum install curl  (RHEL/CentOS)"
    echo "  Install with: brew install curl      (macOS)"
    exit 1
fi

# Determine download URL
BINARY_NAME="muxi-server-${OS}-${ARCH}"
if [ "$INSTALL_VERSION" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}"
else
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${INSTALL_VERSION}/${BINARY_NAME}"
fi

info "Downloading MUXI Server from GitHub..."
info "URL: $DOWNLOAD_URL"

# Create temporary directory
TMP_DIR="$(mktemp -d)"
trap "rm -rf $TMP_DIR" EXIT

# Download binary
if ! curl -fsSL -o "$TMP_DIR/muxi-server" "$DOWNLOAD_URL"; then
    error "Failed to download MUXI Server"
    echo ""
    echo "Possible reasons:"
    echo "  • No release available for $OS/$ARCH"
    echo "  • Network connectivity issues"
    echo "  • GitHub API rate limiting"
    echo ""
    echo "Try downloading manually from:"
    echo "  https://github.com/${REPO}/releases"
    exit 1
fi

success "Downloaded successfully"

# Create install directory
info "Creating directories..."
mkdir -p "$INSTALL_DIR"

if [ "$INSTALL_TYPE" = "system" ]; then
    # System install: create system directories
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$LOG_DIR"
    
    # Set permissions
    chmod 755 "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
    
    success "Created system directories"
else
    # User install: just ensure ~/.muxi/server exists
    mkdir -p "$HOME/.muxi/server"
    success "Created user directories"
fi

# Install binary
info "Installing binary to $INSTALL_DIR/muxi-server..."
mv "$TMP_DIR/muxi-server" "$INSTALL_DIR/muxi-server"
chmod +x "$INSTALL_DIR/muxi-server"
success "Binary installed"

# Verify installation
if ! "$INSTALL_DIR/muxi-server" version >/dev/null 2>&1; then
    error "Binary verification failed"
    exit 1
fi

VERSION_OUTPUT=$("$INSTALL_DIR/muxi-server" version 2>/dev/null || echo "unknown")
success "Verified: $VERSION_OUTPUT"

echo ""
echo "────────────────────────────────────────────────────────────"
success "MUXI Server installed successfully!"
echo "────────────────────────────────────────────────────────────"
echo ""

# PATH management for user installs
if [ "$INSTALL_TYPE" = "user" ]; then
    # Check if already in PATH
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        warn "Adding $INSTALL_DIR to PATH"
        echo ""
        
        # Detect shell and update appropriate config file
        SHELL_CONFIG=""
        SHELL_NAME=$(basename "$SHELL")
        
        case "$SHELL_NAME" in
            bash)
                if [ "$OS" = "darwin" ]; then
                    SHELL_CONFIG="$HOME/.bash_profile"
                else
                    SHELL_CONFIG="$HOME/.bashrc"
                fi
                ;;
            zsh)
                SHELL_CONFIG="$HOME/.zshrc"
                ;;
            fish)
                SHELL_CONFIG="$HOME/.config/fish/config.fish"
                ;;
            *)
                warn "Unknown shell: $SHELL_NAME"
                ;;
        esac
        
        if [ -n "$SHELL_CONFIG" ]; then
            # Add to PATH in shell config
            echo "" >> "$SHELL_CONFIG"
            echo "# Added by MUXI Server installer" >> "$SHELL_CONFIG"
            if [ "$SHELL_NAME" = "fish" ]; then
                echo "set -gx PATH $INSTALL_DIR \$PATH" >> "$SHELL_CONFIG"
            else
                echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$SHELL_CONFIG"
            fi
            
            success "Added to $SHELL_CONFIG"
            echo ""
            info "Reload your shell:"
            echo "  source $SHELL_CONFIG"
            echo ""
            info "Or open a new terminal window"
        else
            # Fallback: show manual instructions
            warn "Please add to your PATH manually:"
            echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
            echo ""
            info "Add this to your shell config file:"
            echo "  ~/.bashrc (bash)"
            echo "  ~/.zshrc (zsh)"
            echo "  ~/.config/fish/config.fish (fish)"
        fi
        echo ""
    else
        success "$INSTALL_DIR is already in PATH"
        echo ""
    fi
fi

# Next steps
echo "Next steps:"
echo ""

if [ "$INSTALL_TYPE" = "system" ]; then
    echo "  1. Initialize the server:"
    echo "     sudo muxi-server init"
    echo ""
    echo "  2. Start the server:"
    echo "     sudo muxi-server start"
    echo ""
    echo "  3. Check server status:"
    echo "     curl http://localhost:7890/health"
else
    echo "  1. Initialize the server:"
    echo "     muxi-server init"
    echo ""
    echo "  2. Start the server:"
    echo "     muxi-server start"
    echo ""
    echo "  3. Check server status:"
    echo "     curl http://localhost:7890/health"
fi

echo ""
echo "Documentation: https://docs.muxi.org/getting-started"
echo "Repository:    https://github.com/${REPO}"
echo ""

# Optional: offer to run init
if [ -t 0 ]; then  # Check if running interactively
    echo ""
    read -p "Run 'muxi-server init' now? [y/N] " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo ""
        if [ "$INSTALL_TYPE" = "system" ]; then
            sudo "$INSTALL_DIR/muxi-server" init
        else
            "$INSTALL_DIR/muxi-server" init
        fi
    fi
fi
