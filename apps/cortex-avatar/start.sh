#!/bin/bash
#
# CortexAvatar Startup Script
# Checks dependencies, installs if needed, and launches the app
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORTEX_DIR="$HOME/.cortex"
PIPER_VOICES_DIR="$CORTEX_DIR/piper-voices"
ENV_FILE="$CORTEX_DIR/.env"
APP_BINARY="$SCRIPT_DIR/build/bin/CortexAvatar.app/Contents/MacOS/CortexAvatar"

# Print header
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   CortexAvatar Startup Script${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to print status
print_status() {
    if [ "$2" = "ok" ]; then
        echo -e "  ${GREEN}✓${NC} $1"
    elif [ "$2" = "warn" ]; then
        echo -e "  ${YELLOW}⚠${NC} $1"
    elif [ "$2" = "error" ]; then
        echo -e "  ${RED}✗${NC} $1"
    else
        echo -e "  ${BLUE}→${NC} $1"
    fi
}

# Function to install Homebrew packages
install_brew_package() {
    if ! brew list "$1" &>/dev/null; then
        print_status "Installing $1..." "info"
        brew install "$1"
    fi
}

#
# Step 1: Check System Requirements
#
echo -e "${YELLOW}Step 1: Checking System Requirements${NC}"

# Check macOS
if [[ "$(uname)" != "Darwin" ]]; then
    print_status "This script requires macOS" "error"
    exit 1
fi
print_status "macOS detected" "ok"

# Check Homebrew
if ! command_exists brew; then
    print_status "Homebrew not found. Installing..." "warn"
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi
print_status "Homebrew installed" "ok"

#
# Step 2: Check Go and Wails
#
echo ""
echo -e "${YELLOW}Step 2: Checking Go & Wails${NC}"

# Check Go
if ! command_exists go; then
    print_status "Go not found. Installing via Homebrew..." "warn"
    brew install go
fi
GO_VERSION=$(go version | awk '{print $3}')
print_status "Go installed ($GO_VERSION)" "ok"

# Check Wails
WAILS_PATH="$HOME/go/bin/wails"
if [ ! -f "$WAILS_PATH" ]; then
    print_status "Wails not found. Installing..." "warn"
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
fi
if [ -f "$WAILS_PATH" ]; then
    WAILS_VERSION=$("$WAILS_PATH" version 2>/dev/null | head -1 || echo "unknown")
    print_status "Wails installed ($WAILS_VERSION)" "ok"
else
    print_status "Wails installation failed" "error"
    exit 1
fi

#
# Step 3: Check Python and Piper TTS
#
echo ""
echo -e "${YELLOW}Step 3: Checking Python & Piper TTS${NC}"

# Check Python 3
if ! command_exists python3; then
    print_status "Python3 not found. Installing via Homebrew..." "warn"
    brew install python@3.11
fi
PYTHON_VERSION=$(python3 --version 2>&1)
print_status "$PYTHON_VERSION" "ok"

# Check pip
if ! command_exists pip3; then
    print_status "pip3 not found. Installing..." "warn"
    python3 -m ensurepip --upgrade
fi
print_status "pip3 available" "ok"

# Check Piper TTS
PIPER_PATH="$HOME/Library/Python/3.11/bin/piper"
if [ ! -f "$PIPER_PATH" ]; then
    # Try alternate location
    PIPER_PATH=$(python3 -c "import sys; print(sys.prefix + '/bin/piper')" 2>/dev/null || echo "")
fi

if [ ! -f "$PIPER_PATH" ] || [ -z "$PIPER_PATH" ]; then
    print_status "Piper TTS not found. Installing..." "warn"
    pip3 install --user piper-tts
    PIPER_PATH="$HOME/Library/Python/3.11/bin/piper"
fi

if [ -f "$PIPER_PATH" ]; then
    print_status "Piper TTS installed ($PIPER_PATH)" "ok"
else
    print_status "Piper TTS installation failed (will use macOS TTS fallback)" "warn"
fi

#
# Step 4: Check/Download Piper Voice Models
#
echo ""
echo -e "${YELLOW}Step 4: Checking Piper Voice Models${NC}"

mkdir -p "$PIPER_VOICES_DIR"

# Amy voice (female)
AMY_MODEL="$PIPER_VOICES_DIR/en_US-amy-medium.onnx"
if [ ! -f "$AMY_MODEL" ]; then
    print_status "Downloading Amy voice model (60MB)..." "info"
    curl -L -o "$AMY_MODEL" \
        "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/amy/medium/en_US-amy-medium.onnx"
    curl -L -o "$AMY_MODEL.json" \
        "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/amy/medium/en_US-amy-medium.onnx.json"
fi
if [ -f "$AMY_MODEL" ]; then
    print_status "Amy voice model ready" "ok"
fi

# Lessac voice (male)
LESSAC_MODEL="$PIPER_VOICES_DIR/en_US-lessac-medium.onnx"
if [ ! -f "$LESSAC_MODEL" ]; then
    print_status "Downloading Lessac voice model (60MB)..." "info"
    curl -L -o "$LESSAC_MODEL" \
        "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx"
    curl -L -o "$LESSAC_MODEL.json" \
        "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json"
fi
if [ -f "$LESSAC_MODEL" ]; then
    print_status "Lessac voice model ready" "ok"
fi

#
# Step 5: Check Configuration
#
echo ""
echo -e "${YELLOW}Step 5: Checking Configuration${NC}"

mkdir -p "$CORTEX_DIR"

# Check .env file
if [ ! -f "$ENV_FILE" ]; then
    print_status "Creating default .env file..." "info"
    cat > "$ENV_FILE" << 'EOF'
# CortexAvatar Environment Variables
# Get your API keys from the respective providers

# Groq API Key (FREE - for STT/Whisper)
# Get yours at: https://console.groq.com
GROQ_API_KEY=

# OpenAI API Key (for TTS, optional - will fallback to Piper/macOS)
OPENAI_API_KEY=

# Anthropic API Key (for Claude models)
ANTHROPIC_API_KEY=

# Google Gemini API Key
GEMINI_API_KEY=

# Tavily API Key (for web search)
TAVILY_API_KEY=
EOF
    print_status "Created $ENV_FILE - please add your API keys" "warn"
else
    print_status ".env file exists" "ok"
fi

# Check for required API keys
if [ -f "$ENV_FILE" ]; then
    source "$ENV_FILE" 2>/dev/null || true

    if [ -z "$GROQ_API_KEY" ]; then
        print_status "GROQ_API_KEY not set - voice input won't work!" "warn"
        print_status "Get a FREE key at: https://console.groq.com" "info"
    else
        print_status "GROQ_API_KEY configured (STT ready)" "ok"
    fi

    if [ -z "$OPENAI_API_KEY" ]; then
        print_status "OPENAI_API_KEY not set - will use Piper/macOS TTS" "warn"
    else
        print_status "OPENAI_API_KEY configured" "ok"
    fi
fi

#
# Step 6: Check CortexBrain Server
#
echo ""
echo -e "${YELLOW}Step 6: Checking CortexBrain Server${NC}"

CORTEXBRAIN_URL="http://localhost:8080"
if curl -s --connect-timeout 2 "$CORTEXBRAIN_URL/.well-known/agent-card.json" > /dev/null 2>&1; then
    print_status "CortexBrain server running at $CORTEXBRAIN_URL" "ok"
else
    print_status "CortexBrain server not detected at $CORTEXBRAIN_URL" "warn"
    print_status "Start CortexBrain first: cd ../cortex-brain && ./cortex" "info"

    # Ask user if they want to continue anyway
    echo ""
    read -p "Continue without CortexBrain? (y/n) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

#
# Step 7: Build Application (if needed)
#
echo ""
echo -e "${YELLOW}Step 7: Building Application${NC}"

cd "$SCRIPT_DIR"

# Check if rebuild is needed
NEEDS_BUILD=false
if [ ! -f "$APP_BINARY" ]; then
    print_status "Application not built yet" "info"
    NEEDS_BUILD=true
elif [ -n "$(find . -name '*.go' -newer "$APP_BINARY" 2>/dev/null)" ]; then
    print_status "Go source files changed, rebuilding..." "info"
    NEEDS_BUILD=true
elif [ -n "$(find frontend/src -name '*.ts' -o -name '*.svelte' -newer "$APP_BINARY" 2>/dev/null)" ]; then
    print_status "Frontend files changed, rebuilding..." "info"
    NEEDS_BUILD=true
fi

if [ "$NEEDS_BUILD" = true ] || [ "$1" = "--rebuild" ] || [ "$1" = "-r" ]; then
    print_status "Building CortexAvatar..." "info"

    # Kill any running instance
    pkill -f "CortexAvatar" 2>/dev/null || true
    sleep 1

    # Build with Wails
    CGO_ENABLED=1 GOTOOLCHAIN=local "$WAILS_PATH" build -clean 2>&1 | while read line; do
        if [[ "$line" == *"Built"* ]]; then
            echo -e "  ${GREEN}✓${NC} $line"
        elif [[ "$line" == *"Error"* ]] || [[ "$line" == *"error"* ]]; then
            echo -e "  ${RED}✗${NC} $line"
        fi
    done

    if [ -f "$APP_BINARY" ]; then
        print_status "Build successful" "ok"
    else
        print_status "Build failed!" "error"
        exit 1
    fi
else
    print_status "Application is up to date" "ok"
fi

#
# Step 8: Launch Application
#
echo ""
echo -e "${YELLOW}Step 8: Launching CortexAvatar${NC}"

# Kill any existing instance
pkill -f "CortexAvatar" 2>/dev/null || true
sleep 1

print_status "Starting CortexAvatar..." "info"
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}   CortexAvatar is starting!${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "Tips:"
echo "  - Click the microphone button to start voice input"
echo "  - Check browser console (Right-click → Inspect) for logs"
echo "  - Press Ctrl+C to stop"
echo ""

# Run the application
exec "$APP_BINARY"
