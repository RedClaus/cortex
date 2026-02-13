#!/bin/bash

# Ollama A2A Adapter Build Script
# Date: February 1, 2026
# Target: Pink (192.168.1.186)

set -e

echo "========================================="
echo "OLLAMA A2A ADAPTER BUILD"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Check if on Pink
echo -e "${YELLOW}[Step 1/5]${NC} Checking if running on Pink..."
HOSTNAME=$(hostname)
if [ "$HOSTNAME" != "Pink" ] && [ "$HOSTNAME" != "pink" ]; then
    echo -e "${RED}Error: This script must be run on Pink (192.168.1.186)${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Running on Pink${NC}"
echo ""

# Create directory
echo -e "${YELLOW}[Step 2/5]${NC} Creating ollama-adapter directory..."
cd ~/clawd
mkdir -p ollama-adapter
cd ollama-adapter
echo -e "${GREEN}✓ Directory created${NC}"
echo ""

# Check if go.mod exists
echo -e "${YELLOW}[Step 3/5]${NC} Checking Go module..."
if [ -f "go.mod" ]; then
    echo -e "${GREEN}✓ Go module already exists${NC}"
else
    echo "Creating Go module..."
    go mod init github.com/normanking/clawd/ollama-adapter
    echo -e "${GREEN}✓ Go module created${NC}"
fi
echo ""

# Check if client.go exists
echo -e "${YELLOW}[Step 4/5]${NC} Checking client.go..."
if [ -f "client.go" ]; then
    echo -e "${GREEN}✓ client.go already exists${NC}"
    echo "Checking if it needs update..."
    
    # Show first few lines
    echo "Existing client.go (first 5 lines):"
    head -5 client.go
    echo ""
    read -p "Do you want to overwrite client.go? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Please copy the code from OLLAMA-ADAPTER-CLIENT.go.md"
        echo "Location: ~/ServerProjectsMac/OLLAMA-ADAPTER-CLIENT.go.md"
        echo ""
        read -p "Press Enter after copying the code..."
    else
        echo "Keeping existing client.go"
    fi
else
    echo -e "${YELLOW}client.go not found${NC}"
    echo "Please copy the code from:"
    echo "  ~/ServerProjectsMac/OLLAMA-ADAPTER-CLIENT.go.md"
    echo ""
    echo "Create client.go file and paste the code:"
    echo "  nano client.go"
    echo ""
    read -p "Press Enter after creating client.go..."
fi
echo ""

# Build
echo -e "${YELLOW}[Step 5/5]${NC} Building ollama-client..."
go mod tidy
go build -o ollama-client client.go

if [ -f "ollama-client" ]; then
    echo -e "${GREEN}✓ Build successful${NC}"
    echo "Executable: ~/clawd/ollama-adapter/ollama-client"
else
    echo -e "${RED}Error: Build failed${NC}"
    exit 1
fi
echo ""

# Test
echo "========================================="
echo -e "${GREEN}BUILD COMPLETE${NC}"
echo "========================================="
echo ""
echo "Test the adapter:"
echo "  cd ~/clawd/ollama-adapter"
echo "  ./ollama-client"
echo ""
echo -e "${YELLOW}Expected output:${NC}"
echo "  Generated code:"
echo "  func fibonacci(n int) int {"
echo "      if n <= 1 {"
echo "          return n"
echo "      }"
echo "      return fibonacci(n-1) + fibonacci(n-2)"
echo "  }"
echo ""