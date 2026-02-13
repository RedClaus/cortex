#!/bin/bash
#
# Cortex Key Vault - Production Startup Script
#
# This script checks for CortexBrain (cortex-03) and starts it if needed,
# then launches the Key Vault TUI.
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BRAIN_PATH="/Users/normanking/ServerProjectsMac/Production/cortex-brain"
BRAIN_PORT=8080
VAULT_BINARY="/tmp/vault"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}   Cortex Key Vault - Production${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Check if CortexBrain is running
check_brain() {
    if curl -s --connect-timeout 2 "http://localhost:${BRAIN_PORT}/health" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Try to start CortexBrain
start_brain() {
    if [ -d "$BRAIN_PATH" ] && [ -f "$BRAIN_PATH/start.sh" ]; then
        echo -e "${YELLOW}Starting CortexBrain...${NC}"
        cd "$BRAIN_PATH"
        ./start.sh &
        BRAIN_PID=$!

        # Wait for brain to be ready (max 10 seconds)
        for i in {1..10}; do
            sleep 1
            if check_brain; then
                echo -e "${GREEN}CortexBrain is ready!${NC}"
                return 0
            fi
            echo "Waiting for CortexBrain... ($i/10)"
        done

        echo -e "${YELLOW}Warning: CortexBrain may not be fully ready${NC}"
        return 0
    else
        echo -e "${YELLOW}Warning: CortexBrain not found at $BRAIN_PATH${NC}"
        echo -e "${YELLOW}Key Vault will run in standalone mode${NC}"
        return 1
    fi
}

# Main startup logic
echo "Checking for CortexBrain on port ${BRAIN_PORT}..."

if check_brain; then
    echo -e "${GREEN}CortexBrain is already running${NC}"
else
    echo -e "${YELLOW}CortexBrain not detected${NC}"
    start_brain || true
fi

echo ""
echo "Building Key Vault..."
cd "$SCRIPT_DIR"

if go build -o "$VAULT_BINARY" ./cmd/vault; then
    echo -e "${GREEN}Build successful!${NC}"
    echo ""
    echo "Launching Key Vault..."
    echo ""
    exec "$VAULT_BINARY"
else
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi
