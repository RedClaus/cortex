#!/bin/bash
# Cortex Evaluator - Local Development Setup Script
# This script helps you set up and run Cortex Evaluator locally without Docker

set -e

echo "🚀 Cortex Evaluator - Local Development Setup"
echo "================================"
echo ""

# Check dependencies
MISSING_DEPS=false

if ! command -v python3 &> /dev/null; then
    echo "❌ Python 3.10+ required"
    MISSING_DEPS=true
fi

if ! command -v node &> /dev/null; then
    echo "❌ Node.js 18+ required"
    MISSING_DEPS=true
fi

if [ "$MISSING_DEPS" = true ]; then
    echo ""
    echo "Please install the required dependencies:"
    echo "  - Python 3.10+: https://www.python.org/downloads/"
    echo "  - Node.js 18+: https://nodejs.org/"
    exit 1
fi

echo "✅ Python: $(python3 --version)"
echo "✅ Node.js: $(node --version)"
echo ""

# Setup Python backend
echo "📦 Setting up Python backend..."
cd backend

# Create virtual environment if it doesn't exist
if [ ! -d "venv" ]; then
    echo "Creating Python virtual environment..."
    python3 -m venv venv
fi

echo "Activating virtual environment..."
source venv/bin/activate

echo "Installing Python dependencies..."
pip install --quiet --upgrade pip
pip install --quiet -r requirements.txt

echo ""
echo "✅ Backend dependencies installed!"
echo "   - FastAPI"
echo "   - SQLModel"
echo "   - ChromaDB"
echo "   - httpx"
echo "   - AI providers (Gemini, OpenAI, Anthropic, etc.)"
echo ""

# Setup data directories
echo "📁 Creating data directories..."
mkdir -p data/cortex
mkdir -p data/chroma
echo "✅ Data directories created"
echo ""

# Create environment file if it doesn't exist
if [ ! -f ".env" ]; then
    echo "📝 Creating .env file..."
    cat > .env << 'EOF'
# Database (SQLite default)
DATABASE_URL=sqlite:///./data/cortex/cortex.db

# Vector Database (ChromaDB local)
CHROMA_HOST=localhost
CHROMA_PORT=8001
CHROMA_PATH=./data/chroma

# AI Provider API Keys (at least one required)
GEMINI_API_KEY=
OPENAI_API_KEY=
ANTHROPIC_API_KEY=
GROQ_API_KEY=
OLLAMA_BASE_URL=http://localhost:11434

# CORS
CORS_ORIGINS=http://localhost:3000,http://localhost:5173

# Frontend API URL (for local dev)
VITE_API_URL=http://localhost:8000
EOF
    echo "✅ .env file created"
    echo ""
    echo "⚠️  IMPORTANT: Edit .env and add your API keys!"
    echo "   Required: GEMINI_API_KEY (free tier works)"
    echo "   Optional: OPENAI_API_KEY, ANTHROPIC_API_KEY, GROQ_API_KEY"
    echo ""
else
    echo "✅ .env file exists"
fi

# Check frontend setup
echo "📦 Checking frontend setup..."
cd ../frontend

# Initialize npm if node_modules doesn't exist
if [ ! -d "node_modules" ]; then
    echo "Installing frontend dependencies..."
    npm install
else
    echo "✅ Frontend dependencies already installed"
fi

echo ""
echo "================================"
echo "✅ Setup Complete!"
echo "================================"
echo ""
echo "Next steps:"
echo ""
echo "1. Edit .env file with your API keys:"
echo "   cd /Users/normanking/ServerProjectsMac/Development/cortex-evaluator"
echo "   nano .env"
echo "   # Add: GEMINI_API_KEY=your_key_here"
echo ""
echo "2. Start backend (Terminal 1):"
echo "   cd /Users/normanking/ServerProjectsMac/Development/cortex-evaluator/backend"
echo "   source venv/bin/activate"
echo "   uvicorn app.main:app --reload --port 8000"
echo ""
echo "3. Start frontend (Terminal 2 - NEW terminal):"
echo "   cd /Users/normanking/ServerProjectsMac/Development/cortex-evaluator/frontend"
echo "   npm run dev"
echo ""
echo "4. Access the application:"
echo "   Frontend: http://localhost:3000"
echo "   Backend: http://localhost:8000"
echo "   API Docs: http://localhost:8000/docs"
echo ""
echo "⚠️  Notes:"
echo "   - Keep BOTH terminals open"
echo "   - Backend uses SQLite by default (no PostgreSQL needed)"
echo "   - ChromaDB will create local vector store"
echo "   - No Docker required!"
echo ""
