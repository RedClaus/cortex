#!/bin/bash
# Deploy Cortex Swarm Dashboard to Pink (192.168.1.186)

set -e

PINK_HOST="192.168.1.186"
PINK_USER="norman"
REMOTE_DIR="/home/norman/cortex-gateway"

echo "🚀 Deploying Cortex Swarm Dashboard to Pink..."

# Build dashboard for Linux
echo "📦 Building dashboard for Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -o swarm-dashboard-linux-arm64 ./cmd/swarm-dashboard/

# Copy binary to pink
echo "📤 Copying binary to pink..."
scp swarm-dashboard-linux-arm64 ${PINK_USER}@${PINK_HOST}:${REMOTE_DIR}/swarm-dashboard

# Copy service file
echo "📤 Copying service file..."
scp swarm-dashboard.service ${PINK_USER}@${PINK_HOST}:/tmp/swarm-dashboard.service

# Install and start service
echo "⚙️  Installing service..."
ssh ${PINK_USER}@${PINK_HOST} << 'EOF'
    cd /home/norman/cortex-gateway
    chmod +x swarm-dashboard

    # Install service
    sudo mv /tmp/swarm-dashboard.service /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl enable swarm-dashboard
    sudo systemctl restart swarm-dashboard

    echo "✅ Service installed and started"
    sleep 2

    # Check status
    sudo systemctl status swarm-dashboard --no-pager -l
EOF

echo ""
echo "✅ Dashboard deployed successfully!"
echo "🌐 Access at: http://192.168.1.186:18888"
echo ""
echo "Useful commands:"
echo "  ssh norman@192.168.1.186 'sudo journalctl -u swarm-dashboard -f'  # View logs"
echo "  ssh norman@192.168.1.186 'sudo systemctl restart swarm-dashboard' # Restart"
echo "  ssh norman@192.168.1.186 'sudo systemctl status swarm-dashboard'  # Status"
