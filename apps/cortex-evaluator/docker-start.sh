#!/bin/bash

set -e

echo "======================================"
echo "Cortex Evaluator - Docker Quick Start"
echo "======================================"
echo ""

if [ ! -f .env ]; then
    echo "Creating .env file from template..."
    cp .env.example .env
    echo ""
    echo "⚠️  IMPORTANT: Edit .env and add your API keys!"
    echo "   Required: GEMINI_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY"
    echo ""
    read -p "Press Enter to continue, or Ctrl+C to edit .env first..."
fi

if [ ! -f chroma_auth.txt ]; then
    echo "Creating ChromaDB authentication token..."
    openssl rand -hex 32 > chroma_auth.txt
    echo "Created chroma_auth.txt"
fi

echo ""
echo "Checking Docker installation..."
if ! command -v docker &> /dev/null; then
    echo "❌ Docker not found. Please install Docker first."
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose not found. Please install Docker Compose first."
    exit 1
fi

echo "✅ Docker and Docker Compose found"
echo ""

echo "Building Docker images..."
docker-compose build

echo ""
echo "Starting services..."
docker-compose up -d

echo ""
echo "Waiting for services to be ready..."
sleep 10

echo ""
echo "Checking service health..."
for i in {1..6}; do
    if curl -s http://localhost:8000/health > /dev/null 2>&1; then
        echo "✅ Backend is ready!"
        break
    fi
    echo "Waiting for backend... ($i/6)"
    sleep 5
done

echo ""
echo "======================================"
echo "✅ Setup Complete!"
echo "======================================"
echo ""
echo "Access the application:"
echo "  Frontend: http://localhost:3000"
echo "  Backend:  http://localhost:8000"
echo "  API Docs: http://localhost:8000/docs"
echo ""
echo "Useful commands:"
echo "  make logs      - View all logs"
echo "  make health    - Check service health"
echo "  make shell     - Access backend shell"
echo "  make down      - Stop all services"
echo ""
echo "For full documentation, see README.docker.md"
echo ""
