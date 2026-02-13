#!/bin/bash
# Pinky Service Uninstallation Script
#
# This script removes Pinky service from Linux (systemd) or macOS (launchd).
#
# Usage:
#   ./uninstall-service.sh [--system] [--purge]
#
# Options:
#   --system    Uninstall system service (requires sudo)
#   --purge     Also remove configuration files and logs

set -e

SYSTEM_UNINSTALL=false
PURGE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --system)
            SYSTEM_UNINSTALL=true
            shift
            ;;
        --purge)
            PURGE=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--system] [--purge]"
            echo ""
            echo "Uninstall Pinky service."
            echo ""
            echo "Options:"
            echo "  --system    Uninstall system service (requires sudo)"
            echo "  --purge     Also remove configuration files and logs"
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
NC='\033[0m'

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Uninstall on Linux (systemd)
uninstall_linux() {
    info "Uninstalling on Linux..."

    if $SYSTEM_UNINSTALL; then
        info "Removing system service..."

        # Stop and disable service
        sudo systemctl stop pinky 2>/dev/null || true
        sudo systemctl disable pinky 2>/dev/null || true

        # Remove service file
        sudo rm -f /etc/systemd/system/pinky.service
        sudo systemctl daemon-reload

        if $PURGE; then
            info "Purging configuration and data..."
            sudo rm -rf /etc/pinky /var/lib/pinky /var/log/pinky
            sudo userdel pinky 2>/dev/null || true
        fi

        info "System service removed."
    else
        info "Removing user service..."

        # Stop and disable service
        systemctl --user stop pinky 2>/dev/null || true
        systemctl --user disable pinky 2>/dev/null || true

        # Remove service file
        rm -f "$HOME/.config/systemd/user/pinky.service"
        systemctl --user daemon-reload

        if $PURGE; then
            info "Purging configuration and data..."
            rm -rf "$HOME/.config/pinky" "$HOME/.pinky"
        fi

        info "User service removed."
    fi
}

# Uninstall on macOS (launchd)
uninstall_macos() {
    info "Uninstalling on macOS..."

    if $SYSTEM_UNINSTALL; then
        info "Removing system daemon..."

        # Unload daemon
        sudo launchctl unload /Library/LaunchDaemons/com.pinky.daemon.plist 2>/dev/null || true

        # Remove plist file
        sudo rm -f /Library/LaunchDaemons/com.pinky.daemon.plist

        if $PURGE; then
            info "Purging configuration and data..."
            sudo rm -rf /etc/pinky /var/lib/pinky /var/log/pinky
        fi

        info "System daemon removed."
    else
        info "Removing user agent..."

        # Unload agent
        launchctl unload "$HOME/Library/LaunchAgents/com.pinky.agent.plist" 2>/dev/null || true

        # Remove plist file
        rm -f "$HOME/Library/LaunchAgents/com.pinky.agent.plist"

        if $PURGE; then
            info "Purging configuration and data..."
            rm -rf "$HOME/.pinky"
        fi

        info "User agent removed."
    fi
}

# Main
main() {
    echo "========================================"
    echo "  Pinky Service Uninstaller"
    echo "========================================"
    echo ""

    if $PURGE; then
        warn "Purge mode enabled - configuration and data will be removed!"
        read -p "Are you sure? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            info "Cancelled."
            exit 0
        fi
    fi

    case $OS in
        linux)
            uninstall_linux
            ;;
        macos)
            uninstall_macos
            ;;
        *)
            error "Unsupported operating system: $(uname -s)"
            ;;
    esac

    echo ""
    info "Uninstallation complete!"
}

main "$@"
