#!/bin/bash
# Quick fix script for Harold's bridge service

echo "🔧 Harold Bridge Service Fix Script"
echo "===================================="
echo ""

HAROLD_IP="192.168.1.128"
HAROLD_USER="norman"

echo "Checking Harold's bridge service status..."
echo ""

# Check if we can reach Harold
if ! ping -c 1 -W 1 $HAROLD_IP > /dev/null 2>&1; then
    echo "❌ ERROR: Cannot reach Harold at $HAROLD_IP"
    echo "   Check if Harold is powered on and connected to network"
    exit 1
fi

echo "✅ Harold is reachable at $HAROLD_IP"
echo ""

echo "Available fix options:"
echo ""
echo "1. Check Harold's bridge service status (READ-ONLY)"
echo "2. Attempt SSH connection to Harold for manual fix"
echo "3. Show manual fix instructions"
echo "4. Exit"
echo ""
read -p "Choose option (1-4): " choice

case $choice in
    1)
        echo ""
        echo "Checking if port 18802 is open..."
        if nc -zv -w 3 $HAROLD_IP 18802 2>&1 | grep -q "succeeded\|open"; then
            echo "✅ Port 18802 is OPEN - bridge service appears to be running!"
        else
            echo "❌ Port 18802 is CLOSED - bridge service is NOT running"
        fi

        echo ""
        echo "Checking if port 18789 (gateway) is open..."
        if nc -zv -w 3 $HAROLD_IP 18789 2>&1 | grep -q "succeeded\|open"; then
            echo "✅ Port 18789 is OPEN - gateway service is running"
        else
            echo "❌ Port 18789 is CLOSED - gateway service may be down"
        fi

        echo ""
        echo "Checking cortex-gateway health API..."
        curl -s --max-time 3 http://localhost:8080/api/v1/swarm/agents | \
            python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    harold = next((a for a in data if a['name'] == 'harold'), None)
    if harold:
        print(f'✅ Harold discoverable: {harold[\"ip\"]}')
        print(f'   Status: {harold[\"status\"]}')
        print(f'   Services: {harold.get(\"services\", \"None\")}')
    else:
        print('❌ Harold not found in agent list')
except:
    print('❌ Error checking agent list')
" 2>/dev/null || echo "❌ Cannot reach cortex-gateway API"
        ;;

    2)
        echo ""
        echo "Attempting SSH connection to Harold..."
        echo "You will be prompted for Harold's SSH password."
        echo ""
        ssh ${HAROLD_USER}@${HAROLD_IP}
        ;;

    3)
        echo ""
        echo "📋 MANUAL FIX INSTRUCTIONS FOR HAROLD"
        echo "======================================"
        echo ""
        echo "Step 1: SSH to Harold"
        echo "  ssh ${HAROLD_USER}@${HAROLD_IP}"
        echo ""
        echo "Step 2: Check if bridge service is running"
        echo "  ps aux | grep -i bridge | grep -v grep"
        echo ""
        echo "Step 3: Check systemd service status (if using systemd)"
        echo "  sudo systemctl status cortex-bridge"
        echo "  OR"
        echo "  sudo systemctl status cortex-gateway-bridge"
        echo ""
        echo "Step 4: Start the service"
        echo "  Option A (systemd):"
        echo "    sudo systemctl start cortex-bridge"
        echo ""
        echo "  Option B (manual):"
        echo "    cd /home/norman/cortex-gateway"
        echo "    ./cortex-bridge &"
        echo ""
        echo "Step 5: Verify port is listening"
        echo "  sudo lsof -i :18802"
        echo "  OR"
        echo "  sudo netstat -tlnp | grep 18802"
        echo ""
        echo "Step 6: Check health from local machine"
        echo "  curl http://192.168.1.128:18802/health"
        echo ""
        echo "Step 7: Verify in cortex-gateway"
        echo "  curl http://localhost:8080/api/v1/healthring/status | grep -A 5 harold"
        echo ""
        ;;

    4)
        echo "Exiting..."
        exit 0
        ;;

    *)
        echo "Invalid option. Exiting..."
        exit 1
        ;;
esac

echo ""
echo "Done! Check the swarm status report for details:"
echo "  cat /Users/normanking/ServerProjectsMac/cortex-gateway-test/SWARM_STATUS_REPORT_2026-02-07.md"
