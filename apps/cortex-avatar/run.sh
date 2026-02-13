#!/bin/bash
# CortexAvatar Launcher Script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

print_banner() {
    echo -e "${BLUE}"
    echo "   ____           _            _             _            "
    echo "  / ___|___  _ __| |_ _____  _|  \\          | |           "
    echo " | |   / _ \\| '__| __/ _ \\ \\/ / _ \\ \\ /\\ / / __|/ _\\\` | '__|"
    echo " | |__| (_) | |  | ||  __/>  <  __/\\ V  V /| (_| (_| | |   "
    echo "  \\____\\___/|_|   \\__\\___/_/\\_\\___|  \\_/\\_/  \\__\\__,_|_|   "
    echo "                                                           "
    echo -e "${NC}"
    echo -e "${GREEN}The Face, Eyes, and Ears of CortexBrain${NC}"
    echo ""
}

# Check if CortexBrain A2A server is running
check_cortexbrain() {
    if curl -s http://localhost:8080/.well-known/agent-card.json > /dev/null 2>&1; then
        echo -e "${GREEN}✓ CortexBrain A2A server detected at localhost:8080${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠ CortexBrain A2A server not detected at localhost:8080${NC}"
        echo -e "  Run: cd /path/to/cortex-brain && cortex-server --port 8080"
        echo ""
        return 1
    fi
}

# Build the application
build_app() {
    echo -e "${BLUE}Building CortexAvatar...${NC}"

    # Use Wails CLI for proper build with macOS app bundle
    WAILS_CMD="${HOME}/go/bin/wails"
    if [ ! -f "$WAILS_CMD" ]; then
        echo -e "${YELLOW}Installing Wails CLI...${NC}"
        go install github.com/wailsapp/wails/v2/cmd/wails@latest
    fi

    echo "  Building with Wails CLI..."
    GOTOOLCHAIN=local "$WAILS_CMD" build

    echo -e "${GREEN}✓ Build complete${NC}"
    echo -e "  App: ${SCRIPT_DIR}/build/bin/CortexAvatar.app"
    echo ""
}

# Run the application
run_app() {
    print_banner
    check_cortexbrain || true

    if [ ! -d "build/bin/CortexAvatar.app" ]; then
        build_app
    fi

    echo -e "${BLUE}Starting CortexAvatar...${NC}"
    echo ""

    # Open the macOS app bundle
    open "build/bin/CortexAvatar.app"
}

# Rebuild and run
rebuild_run() {
    print_banner
    build_app force
    check_cortexbrain || true

    echo -e "${BLUE}Starting CortexAvatar...${NC}"
    echo ""

    open "build/bin/CortexAvatar.app"
}

# Just build
just_build() {
    print_banner
    build_app force
}

# Parse arguments
case "${1:-run}" in
    run|start)
        run_app
        ;;
    build)
        just_build
        ;;
    rebuild)
        rebuild_run
        ;;
    check)
        print_banner
        check_cortexbrain
        ;;
    help|--help|-h)
        print_banner
        echo "Usage: $0 [run|build|rebuild|check]"
        echo ""
        echo "Commands:"
        echo "  run     - Build (if needed) and run the application (default)"
        echo "  build   - Build the application"
        echo "  rebuild - Force rebuild and run"
        echo "  check   - Check if CortexBrain is running"
        echo ""
        echo "Requirements:"
        echo "  - Go 1.22+ with CGO support"
        echo "  - Node.js 18+"
        echo "  - Wails CLI (installed automatically if missing)"
        echo "  - CortexBrain A2A server running on localhost:8080"
        ;;
    *)
        echo "Unknown command: $1"
        echo "Run '$0 help' for usage"
        exit 1
        ;;
esac
