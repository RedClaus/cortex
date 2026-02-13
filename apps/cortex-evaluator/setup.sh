#!/bin/bash
# Cortex Evaluator - Quick Setup Script
# This script helps you get started with the new implementation

set -e

echo "🚀 Cortex Evaluator Setup Script"
echo "================================"
echo ""

# Check Python
if ! command -v python3 &> /dev/null; then
    echo "❌ Python 3.10+ required. Please install Python 3.10 or later."
    exit 1
fi
PYTHON_VERSION=$(python3 --version)
echo "✅ Python found: $PYTHON_VERSION"

# Check Node.js
if ! command -v node &> /dev/null; then
    echo "❌ Node.js 18+ required. Please install Node.js 18 or later."
    exit 1
fi
NODE_VERSION=$(node --version)
echo "✅ Node.js found: $NODE_VERSION"

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "⚠️  Docker not found. Docker is recommended but not required."
    DOCKER_AVAILABLE=false
else
    DOCKER_AVAILABLE=true
    echo "✅ Docker found: $(docker --version)"
fi
echo ""

# Prompt for setup mode
echo "Choose setup mode:"
echo "1) Local development (Python venv + npm)"
echo "2) Docker development (docker-compose up)"
echo "3) Install dependencies only"
echo ""
read -p "Enter choice (1-3): " SETUP_MODE

case $SETUP_MODE in
    1)
        echo ""
        echo "📦 Setting up local development environment..."
        echo ""

        # Setup Python backend
        cd backend
        if [ ! -d "venv" ]; then
            echo "Creating Python virtual environment..."
            python3 -m venv venv
        fi

        echo "Activating virtual environment..."
        source venv/bin/activate

        echo "Installing Python dependencies..."
        pip install -q --upgrade pip
        pip install -q -r requirements.txt

        echo ""
        # Setup Node.js frontend
        cd ../frontend
        if [ ! -d "node_modules" ]; then
            echo "Installing frontend dependencies..."
            npm install --silent
        fi

        # Setup CLI tool
        cd ../cortex-eval-cli
        if [ ! -d "node_modules" ]; then
            echo "Installing CLI dependencies..."
            npm install --silent
        fi

        # Setup VSCode extension
        cd ../cortex-eval-vscode
        if [ ! -d "node_modules" ]; then
            echo "Installing extension dependencies..."
            npm install --silent
        fi

        echo ""
        echo "✅ Local development environment ready!"
        echo ""
        echo "Next steps:"
        echo "1. Copy .env.example to .env"
        echo "2. Edit .env with your API keys"
        echo "3. Start backend (Terminal 1): cd backend && source venv/bin/activate && uvicorn app.main:app --reload"
        echo "4. Start frontend (Terminal 2): cd frontend && npm run dev"
        echo ""
        ;;

    2)
        echo ""
        echo "🐳 Setting up Docker development environment..."
        echo ""

        if [ "$DOCKER_AVAILABLE" = false ]; then
            echo "❌ Docker is required for Docker mode. Please install Docker first."
            exit 1
        fi

        # Check for .env file
        if [ ! -f ".env" ]; then
            echo "⚠️  No .env file found. Copying from .env.example..."
            cp .env.example .env
        fi

        echo "Starting Docker containers..."
        docker-compose up -d

        echo ""
        echo "✅ Docker development environment starting..."
        echo ""
        echo "Services:"
        docker-compose ps
        echo ""
        echo "Access URLs:"
        echo "  Frontend:  http://localhost:3000"
        echo "  Backend:  http://localhost:8000"
        echo "  API Docs: http://localhost:8000/docs"
        echo "  Vector DB: http://localhost:8001 (if ChromaDB server)"
        echo ""
        echo "Logs:"
        echo "  View logs: docker-compose logs -f"
        echo "  Stop containers: docker-compose down"
        echo ""
        ;;

    3)
        echo ""
        echo "📦 Installing dependencies only..."
        echo ""

        cd backend
        if [ ! -d "venv" ]; then
            python3 -m venv venv
        fi
        source venv/bin/activate
        pip install -q --upgrade pip
        pip install -q -r requirements.txt

        cd ../frontend
        if [ ! -d "node_modules" ]; then
            npm install --silent
        fi

        cd ../cortex-eval-cli
        if [ ! -d "node_modules" ]; then
            npm install --silent
        fi

        cd ../cortex-eval-vscode
        if [ ! -d "node_modules" ]; then
            npm install --silent
        fi

        echo ""
        echo "✅ All dependencies installed!"
        echo ""
        echo "Next steps:"
        echo "1. Copy .env.example to .env"
        echo "2. Edit .env with your API keys"
        echo "3. Run local development or Docker setup"
        echo ""
        ;;

    *)
        echo "❌ Invalid choice. Please enter 1, 2, or 3."
        exit 1
        ;;
esac

echo ""
echo "================================"
echo "Setup complete! 🎉"
echo ""
echo "Documentation:"
echo "  - Main README: README.md"
echo "  - Deployment Guide: DEPLOYMENT.md"
echo "  - API Reference: API.md"
echo "  - Implementation Summary: IMPLEMENTATION_SUMMARY.md"
echo ""
