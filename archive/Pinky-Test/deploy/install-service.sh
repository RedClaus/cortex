#!/bin/bash
# Pinky Service Installation Script
#
# This script installs Pinky as a background service on Linux (systemd) or macOS (launchd).
#
# Usage:
#   ./install-service.sh [--system]
#
# Options:
#   --system    Install as system service (requires sudo)
#               Without this flag, installs as user service

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PINKY_BIN="${PINKY_BIN:-$(which pinky 2>/dev/null || echo "")}"
SYSTEM_INSTALL=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --system)
            SYSTEM_INSTALL=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--system]"
            echo ""
            echo "Install Pinky as a background service."
            echo ""
            echo "Options:"
            echo "  --system    Install as system service (requires sudo)"
            echo "              Default: Install as user service"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "macos" ;;
        *)       echo "unknown" ;;
    esac
}

OS=$(detect_os)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Check if pinky binary exists
check_binary() {
    if [[ -z "$PINKY_BIN" ]]; then
        # Try common locations
        for path in /usr/local/bin/pinky "$HOME/.local/bin/pinky" ./pinky ../pinky; do
            if [[ -x "$path" ]]; then
                PINKY_BIN="$path"
                break
            fi
        done
    fi

    if [[ -z "$PINKY_BIN" ]] || [[ ! -x "$PINKY_BIN" ]]; then
        error "Pinky binary not found. Please build it first or set PINKY_BIN environment variable."
    fi

    info "Found Pinky binary: $PINKY_BIN"
}

# Install on Linux (systemd)
install_linux() {
    info "Installing on Linux with systemd..."

    if $SYSTEM_INSTALL; then
        # System service installation
        info "Installing system service (requires sudo)..."

        SERVICE_FILE="$SCRIPT_DIR/systemd/pinky-system.service"
        DEST="/etc/systemd/system/pinky.service"

        if [[ ! -f "$SERVICE_FILE" ]]; then
            error "Service file not found: $SERVICE_FILE"
        fi

        # Create pinky user if needed
        if ! id pinky &>/dev/null; then
            info "Creating pinky user..."
            sudo useradd -r -s /bin/false -d /var/lib/pinky pinky || true
        fi

        # Create directories
        sudo mkdir -p /var/lib/pinky /etc/pinky /var/log/pinky
        sudo chown pinky:pinky /var/lib/pinky /var/log/pinky

        # Copy binary
        info "Copying binary to /usr/local/bin/..."
        sudo cp "$PINKY_BIN" /usr/local/bin/pinky
        sudo chmod 755 /usr/local/bin/pinky

        # Copy service file
        info "Installing service file..."
        sudo cp "$SERVICE_FILE" "$DEST"
        sudo chmod 644 "$DEST"

        # Create env file if it doesn't exist
        if [[ ! -f /etc/pinky/env ]]; then
            info "Creating environment file template..."
            sudo tee /etc/pinky/env > /dev/null << 'EOF'
# Pinky Environment Variables
# Add your API tokens here:
# OPENAI_API_KEY=your-key
# TELEGRAM_BOT_TOKEN=your-token
# DISCORD_BOT_TOKEN=your-token
EOF
            sudo chmod 600 /etc/pinky/env
        fi

        # Reload and enable
        info "Enabling service..."
        sudo systemctl daemon-reload
        sudo systemctl enable pinky

        info "Service installed. Start with: sudo systemctl start pinky"
    else
        # User service installation
        info "Installing user service..."

        SERVICE_FILE="$SCRIPT_DIR/systemd/pinky.service"
        DEST_DIR="$HOME/.config/systemd/user"
        DEST="$DEST_DIR/pinky.service"

        if [[ ! -f "$SERVICE_FILE" ]]; then
            error "Service file not found: $SERVICE_FILE"
        fi

        # Create directories
        mkdir -p "$DEST_DIR" "$HOME/.config/pinky" "$HOME/.pinky/logs" "$HOME/.local/bin"

        # Copy binary if not already in PATH
        if [[ "$PINKY_BIN" != "$HOME/.local/bin/pinky" ]]; then
            info "Copying binary to ~/.local/bin/..."
            cp "$PINKY_BIN" "$HOME/.local/bin/pinky"
            chmod 755 "$HOME/.local/bin/pinky"
        fi

        # Copy service file
        info "Installing service file..."
        cp "$SERVICE_FILE" "$DEST"

        # Create env file if it doesn't exist
        if [[ ! -f "$HOME/.config/pinky/env" ]]; then
            info "Creating environment file template..."
            cat > "$HOME/.config/pinky/env" << 'EOF'
# Pinky Environment Variables
# Add your API tokens here:
# OPENAI_API_KEY=your-key
# TELEGRAM_BOT_TOKEN=your-token
# DISCORD_BOT_TOKEN=your-token
EOF
            chmod 600 "$HOME/.config/pinky/env"
        fi

        # Reload and enable
        info "Enabling service..."
        systemctl --user daemon-reload
        systemctl --user enable pinky

        info "Service installed. Start with: systemctl --user start pinky"
        info "To start on boot (without login): loginctl enable-linger $USER"
    fi
}

# Install on macOS (launchd)
install_macos() {
    info "Installing on macOS with launchd..."

    if $SYSTEM_INSTALL; then
        # System daemon installation
        info "Installing system daemon (requires sudo)..."

        PLIST_FILE="$SCRIPT_DIR/launchd/com.pinky.daemon.plist"
        DEST="/Library/LaunchDaemons/com.pinky.daemon.plist"

        if [[ ! -f "$PLIST_FILE" ]]; then
            error "Plist file not found: $PLIST_FILE"
        fi

        # Create directories
        sudo mkdir -p /var/lib/pinky /etc/pinky /var/log/pinky

        # Copy binary
        info "Copying binary to /usr/local/bin/..."
        sudo cp "$PINKY_BIN" /usr/local/bin/pinky
        sudo chmod 755 /usr/local/bin/pinky

        # Copy plist file
        info "Installing daemon plist..."
        sudo cp "$PLIST_FILE" "$DEST"
        sudo chown root:wheel "$DEST"
        sudo chmod 644 "$DEST"

        # Load the daemon
        info "Loading daemon..."
        sudo launchctl load "$DEST" 2>/dev/null || true

        info "Daemon installed. Check status with: sudo launchctl list | grep pinky"
    else
        # User agent installation
        info "Installing user agent..."

        PLIST_FILE="$SCRIPT_DIR/launchd/com.pinky.agent.plist"
        DEST_DIR="$HOME/Library/LaunchAgents"
        DEST="$DEST_DIR/com.pinky.agent.plist"

        if [[ ! -f "$PLIST_FILE" ]]; then
            error "Plist file not found: $PLIST_FILE"
        fi

        # Create directories
        mkdir -p "$DEST_DIR" "$HOME/.pinky/logs"

        # Copy binary if not already in PATH
        if [[ ! -f /usr/local/bin/pinky ]]; then
            info "Copying binary to /usr/local/bin/ (requires password)..."
            sudo cp "$PINKY_BIN" /usr/local/bin/pinky
            sudo chmod 755 /usr/local/bin/pinky
        fi

        # Process plist file (replace username placeholder)
        info "Installing agent plist..."
        sed "s|REPLACE_WITH_USERNAME|$USER|g" "$PLIST_FILE" > "$DEST"

        # Load the agent
        info "Loading agent..."
        launchctl load "$DEST" 2>/dev/null || true

        info "Agent installed. Check status with: launchctl list | grep pinky"
        info "View logs at: ~/.pinky/logs/"
    fi
}

# Main
main() {
    echo "========================================"
    echo "  Pinky Service Installer"
    echo "========================================"
    echo ""

    check_binary

    case $OS in
        linux)
            install_linux
            ;;
        macos)
            install_macos
            ;;
        *)
            error "Unsupported operating system: $(uname -s)"
            ;;
    esac

    echo ""
    info "Installation complete!"
    echo ""
    echo "Next steps:"
    echo "  1. Edit the environment file to add your API tokens"
    echo "  2. Create/edit ~/.pinky/config.yaml if needed"
    echo "  3. Start the service"
    echo ""
}

main "$@"
