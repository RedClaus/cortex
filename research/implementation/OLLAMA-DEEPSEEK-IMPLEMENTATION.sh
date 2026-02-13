# OLLAMA-DEEPSEEK-IMPLEMENTATION.sh

#!/bin/bash

# Ollama + DeepSeek-Coder-V2-Lite Implementation Script
# Date: February 1, 2026
# Target: Pink (192.168.1.186) - NVIDIA RTX 3090

set -e

echo "========================================="
echo "OLLAMA + DEEPSEEK-CODER INSTALLATION"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Step 1: Check if on Pink
echo -e "${YELLOW}[Step 1/6]${NC} Checking if running on Pink..."
HOSTNAME=$(hostname)
if [ "$HOSTNAME" != "Pink" ] && [ "$HOSTNAME" != "pink" ]; then
    echo -e "${RED}Error: This script must be run on Pink (192.168.1.186)${NC}"
    echo "Current hostname: $HOSTNAME"
    exit 1
fi
echo -e "${GREEN}✓ Running on Pink${NC}"
echo ""

# Step 2: Check if Ollama is installed
echo -e "${YELLOW}[Step 2/6]${NC} Checking if Ollama is installed..."
if ! command -v ollama &> /dev/null; then
    echo -e "${YELLOW}Ollama not found. Installing...${NC}"
    
    # Check if brew is available
    if command -v brew &> /dev/null; then
        echo "Installing Ollama via Homebrew..."
        brew install ollama
    else
        echo -e "${RED}Error: Homebrew not found${NC}"
        echo "Please install Homebrew first or install Ollama manually:"
        echo "  curl -fsSL https://ollama.com/install.sh | sh"
        exit 1
    fi
    
    echo -e "${GREEN}✓ Ollama installed${NC}"
else
    OLLAMA_VERSION=$(ollama --version)
    echo -e "${GREEN}✓ Ollama already installed: $OLLAMA_VERSION${NC}"
fi
echo ""

# Step 3: Start Ollama server
echo -e "${YELLOW}[Step 3/6]${NC} Starting Ollama server..."
# Check if Ollama is already running
if pgrep -f "ollama serve" > /dev/null; then
    echo -e "${GREEN}✓ Ollama server already running${NC}"
else
    echo "Starting Ollama server in background..."
    nohup ollama serve > ~/clawd/logs/ollama-server.log 2>&1 &
    
    # Wait for server to start
    echo "Waiting for Ollama server to start..."
    sleep 5
    
    # Verify server is running
    if pgrep -f "ollama serve" > /dev/null; then
        echo -e "${GREEN}✓ Ollama server started${NC}"
    else
        echo -e "${RED}Error: Ollama server failed to start${NC}"
        echo "Check logs: tail -f ~/clawd/logs/ollama-server.log"
        exit 1
    fi
fi
echo ""

# Step 4: Verify Ollama API
echo -e "${YELLOW}[Step 4/6]${NC} Verifying Ollama API..."
API_RESPONSE=$(curl -s http://localhost:11434/api/tags 2>&1)
if echo "$API_RESPONSE" | grep -q "models"; then
    echo -e "${GREEN}✓ Ollama API responding${NC}"
else
    echo -e "${RED}Error: Ollama API not responding${NC}"
    echo "Response: $API_RESPONSE"
    exit 1
fi
echo ""

# Step 5: Check if DeepSeek-Coder-V2-Lite is available
echo -e "${YELLOW}[Step 5/6]${NC} Checking DeepSeek-Coder-V2-Lite model..."
MODELS_OUTPUT=$(ollama list 2>&1)
if echo "$MODELS_OUTPUT" | grep -q "deepseek-coder-v2-lite"; then
    echo -e "${GREEN}✓ DeepSeek-Coder-V2-Lite already downloaded${NC}"
    echo "$MODELS_OUTPUT" | grep "deepseek-coder-v2-lite"
else
    echo -e "${YELLOW}DeepSeek-Coder-V2-Lite not found. Downloading...${NC}"
    echo "This will download ~9.2GB (quantized model)"
    echo "Please wait..."
    
    ollama pull deepseek-coder-v2-lite
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ DeepSeek-Coder-V2-Lite downloaded${NC}"
        
        # Show model info
        ollama list | grep "deepseek-coder-v2-lite"
    else
        echo -e "${RED}Error: Failed to download DeepSeek-Coder-V2-Lite${NC}"
        exit 1
    fi
fi
echo ""

# Step 6: Test DeepSeek-Coder
echo -e "${YELLOW}[Step 6/6]${NC} Testing DeepSeek-Coder-V2-Lite..."
echo "Running test: 'Write a Go function to calculate fibonacci'..."
echo ""

TEST_RESPONSE=$(curl -s http://localhost:11434/api/generate \
    -H "Content-Type: application/json" \
    -d '{
        "model": "deepseek-coder-v2-lite",
        "prompt": "Write a Go function to calculate fibonacci numbers. Keep it simple.",
        "stream": false
    }' 2>&1)

if echo "$TEST_RESPONSE" | grep -q "response"; then
    echo -e "${GREEN}✓ DeepSeek-Coder-V2-Lite test successful${NC}"
    echo ""
    echo "Generated code:"
    echo "$TEST_RESPONSE" | grep -o '"response":"[^"]*"' | sed 's/"response":"//g' | sed 's/"$//g' | sed 's/\\n/\n/g' | sed 's/\\t/\t/g'
else
    echo -e "${RED}Error: DeepSeek-Coder-V2-Lite test failed${NC}"
    echo "Response: $TEST_RESPONSE"
    exit 1
fi
echo ""

# Summary
echo "========================================="
echo -e "${GREEN}INSTALLATION COMPLETE${NC}"
echo "========================================="
echo ""
echo "Ollama server: Running on port 11434"
echo "Model: DeepSeek-Coder-V2-Lite (16B, quantized)"
echo "API endpoint: http://localhost:11434/api/generate"
echo ""
echo "Test the API:"
echo "  curl http://localhost:11434/api/generate \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"model\":\"deepseek-coder-v2-lite\",\"prompt\":\"Write code\",\"stream\":false}'"
echo ""
echo "View logs:"
echo "  tail -f ~/clawd/logs/ollama-server.log"
echo ""
echo -e "${GREEN}Next steps:${NC}"
echo "  1. Build A2A adapter (see OLLAMA-ADAPTER-CLIENT.go.md)"
echo "  2. Integrate with Harold (see OLLAMA-AGENT-INTEGRATION.go.md)"
echo "  3. Test A2A messaging"
echo ""