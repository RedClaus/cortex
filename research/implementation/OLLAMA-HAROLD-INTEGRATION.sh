#!/bin/bash

# Harold Ollama Agent Integration Script
# Date: February 1, 2026
# Target: Harold (192.168.1.229)

set -e

echo "========================================="
echo "HAROLD OLLAMA AGENT INTEGRATION"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Check if on Harold
echo -e "${YELLOW}[Step 1/6]${NC} Checking if running on Harold..."
HOSTNAME=$(hostname)
if [ "$HOSTNAME" != "Harold" ] && [ "$HOSTNAME" != "harold" ]; then
    echo -e "${RED}Error: This script must be run on Harold (192.168.1.229)${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Running on Harold${NC}"
echo ""

# Navigate to agent directory
echo -e "${YELLOW}[Step 2/6]${NC} Navigating to agent directory..."
cd ~/clawd/cortex-brain/pkg/brain/lobes/agent
if [ ! -d "agent" ]; then
    echo -e "${RED}Error: agent directory not found${NC}"
    echo "Expected: ~/clawd/cortex-brain/pkg/brain/lobes/agent"
    exit 1
fi
cd agent
echo -e "${GREEN}✓ In agent directory${NC}"
echo ""

# Check if ollama_agent.go exists
echo -e "${YELLOW}[Step 3/6]${NC} Checking ollama_agent.go..."
if [ -f "ollama_agent.go" ]; then
    echo -e "${GREEN}✓ ollama_agent.go already exists${NC}"
    echo ""
    read -p "Do you want to overwrite ollama_agent.go? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Please copy the code from OLLAMA-AGENT-INTEGRATION.go.md"
        echo "Location: ~/ServerProjectsMac/OLLAMA-AGENT-INTEGRATION.go.md"
        echo ""
        read -p "Press Enter after copying the code..."
    else
        echo "Keeping existing ollama_agent.go"
    fi
else
    echo -e "${YELLOW}ollama_agent.go not found${NC}"
    echo "Please copy the code from:"
    echo "  ~/ServerProjectsMac/OLLAMA-AGENT-INTEGRATION.go.md"
    echo ""
    echo "Create ollama_agent.go file and paste the code:"
    echo "  nano ollama_agent.go"
    echo ""
    read -p "Press Enter after creating ollama_agent.go..."
fi
echo ""

# Check if registry.go exists
echo -e "${YELLOW}[Step 4/6]${NC} Checking registry.go..."
if [ ! -f "registry.go" ]; then
    echo -e "${RED}Error: registry.go not found${NC}"
    exit 1
fi

# Check if Ollama agent is registered
echo "Checking if Ollama agent is registered..."
if grep -q "ollama-pink" registry.go; then
    echo -e "${GREEN}✓ Ollama agent already registered${NC}"
    echo ""
    read -p "Do you want to re-register? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Please manually add the registration code to registry.go:"
        echo ""
        echo "  RegisterAgent(NewOllamaAgent("
        echo "      \"ollama-pink\","
        echo "      \"http://192.168.1.186:11434\","
        echo "      \"deepseek-coder-v2-lite\","
        echo "  ))"
        echo ""
        read -p "Press Enter after updating registry.go..."
    fi
else
    echo -e "${YELLOW}Ollama agent not registered${NC}"
    echo "Please add the registration code to registry.go:"
    echo ""
    echo "  RegisterAgent(NewOllamaAgent("
    echo "      \"ollama-pink\","
    echo "      \"http://192.168.1.186:11434\","
    echo "      \"deepseek-coder-v2-lite\","
    echo "  ))"
    echo ""
    read -p "Press Enter after updating registry.go..."
fi
echo ""

# Check if orchestrator.go exists
echo -e "${YELLOW}[Step 5/6]${NC} Checking orchestrator.go..."
if [ ! -f "orchestrator.go" ]; then
    echo -e "${RED}Error: orchestrator.go not found${NC}"
    exit 1
fi

# Check if routing logic exists
echo "Checking if routing logic exists..."
if grep -q "isSmallCodingTask" orchestrator.go; then
    echo -e "${GREEN}✓ Routing logic already exists${NC}"
    echo ""
    read -p "Do you want to update routing logic? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Please manually add the routing logic to orchestrator.go:"
        echo ""
        echo "  func (o *Orchestrator) RouteTask(ctx context.Context, task string) (string, error) {"
        echo "      if o.isSmallCodingTask(task) {"
        echo "          return o.agents[\"ollama-pink\"].ExecuteTask(ctx, task)"
        echo "      }"
        echo "      if o.isCodingTask(task) {"
        echo "          return o.agents[\"pink\"].ExecuteTask(ctx, task)"
        echo "      }"
        echo "      return o.agents[\"default\"].ExecuteTask(ctx, task)"
        echo "  }"
        echo ""
        echo "  func (o *Orchestrator) isSmallCodingTask(task string) bool {"
        echo "      keywords := []string{"
        echo "          \"utility function\", \"helper function\", \"calculate\","
        echo "          \"algorithm\", \"fibonacci\", \"factorial\","
        echo "      }"
        echo "      taskLower := strings.ToLower(task)"
        echo "      for _, keyword := range keywords {"
        echo "          if strings.Contains(taskLower, keyword) {"
        echo "              return true"
        echo "          }"
        echo "      }"
        echo "      return false"
        echo "  }"
        echo ""
        read -p "Press Enter after updating orchestrator.go..."
    fi
else
    echo -e "${YELLOW}Routing logic not found${NC}"
    echo "Please add the routing logic to orchestrator.go:"
    echo ""
    echo "  func (o *Orchestrator) RouteTask(ctx context.Context, task string) (string, error) {"
    echo "      if o.isSmallCodingTask(task) {"
    echo "          return o.agents[\"ollama-pink\"].ExecuteTask(ctx, task)"
    echo "      }"
    echo "      if o.isCodingTask(task) {"
    echo "          return o.agents[\"pink\"].ExecuteTask(ctx, task)"
    echo "      }"
    echo "      return o.agents[\"default\"].ExecuteTask(ctx, task)"
    echo "  }"
    echo ""
    read -p "Press Enter after updating orchestrator.go..."
fi
echo ""

# Rebuild Harold
echo -e "${YELLOW}[Step 6/6]${NC} Rebuilding Harold..."
cd ~/clawd/cortex-brain
go build -o cortex-brain cmd/cortex-brain/main.go

if [ -f "cortex-brain" ]; then
    echo -e "${GREEN}✓ Harold built successfully${NC}"
else
    echo -e "${RED}Error: Harold build failed${NC}"
    exit 1
fi
echo ""

# Restart Harold
echo "Restarting Harold..."
if command -v systemctl &> /dev/null; then
    sudo systemctl restart cortex-brain
    echo -e "${GREEN}✓ Harold restarted${NC}"
else
    echo "Please restart Harold manually:"
    echo "  sudo systemctl restart cortex-brain"
    echo "  or"
    echo "  sudo ~/clawd/cortex-brain/cortex-brain &"
    echo ""
fi
echo ""

# Summary
echo "========================================="
echo -e "${GREEN}INTEGRATION COMPLETE${NC}"
echo "========================================="
echo ""
echo "Next steps:"
echo "  1. Test A2A messaging:"
echo "     echo '{\"agent\":\"harold\",\"target\":\"ollama-pink\",\"message\":\"Write a utility function\"}' | \\"
echo "       curl -X POST http://localhost:18802/messages -H 'Content-Type: application/json' -d @"
echo ""
echo "  2. Check Harold logs:"
echo "     tail -f ~/clawd/logs/cortex-brain.log"
echo ""
echo "  3. Test DeepSeek-Coder:"
echo "     cd ~/clawd/ollama-adapter"
echo "     ./ollama-client"
echo ""