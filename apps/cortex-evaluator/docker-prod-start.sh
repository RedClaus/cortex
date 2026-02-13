#!/bin/bash

set -e

echo "======================================"
echo "Cortex Evaluator - Production Setup"
echo "======================================"
echo ""

if [ ! -f .env.production ]; then
    echo "Creating .env.production file from template..."
    cp .env.example .env.production
    echo ""
    echo "⚠️  CRITICAL: Edit .env.production before continuing!"
    echo "   Required changes:"
    echo "   - Set strong passwords (POSTGRES_PASSWORD, REDIS_PASSWORD)"
    echo "   - Set DATABASE_URL to PostgreSQL connection"
    echo "   - Set CORS_ORIGINS to production domain"
    echo "   - Generate strong SECRET_KEY"
    echo "   - Configure all API keys"
    echo ""
    read -p "Press Enter to continue, or Ctrl+C to edit .env.production first..."
fi

if [ ! -f chroma_auth.txt ]; then
    echo "Creating ChromaDB authentication token..."
    openssl rand -hex 32 > chroma_auth.txt
    echo "Created chroma_auth.txt"
fi

echo ""
echo "Building production images..."
docker-compose -f docker-compose.prod.yml --env-file .env.production build

echo ""
echo "Starting production services..."
docker-compose -f docker-compose.prod.yml --env-file .env.production up -d

echo ""
echo "Waiting for services to be ready..."
sleep 15

echo ""
echo "Checking service health..."
for i in {1..10}; do
    if curl -s http://localhost/health > /dev/null 2>&1; then
        echo "✅ Production backend is ready!"
        break
    fi
    echo "Waiting for backend... ($i/10)"
    sleep 5
done

echo ""
echo "======================================"
echo "✅ Production Setup Complete!"
echo "======================================"
echo ""
echo "Access the application:"
echo "  HTTP:  http://localhost"
echo "  HTTPS: https://your-domain.com (if configured)"
echo ""
echo "Backend scaled to 3 replicas"
echo ""
echo "Useful commands:"
echo "  make prod-down     - Stop production services"
echo "  make prod-scale    - Scale backend replicas"
echo "  make backup        - Backup all volumes"
echo ""
echo "For full documentation, see README.docker.md"
echo ""
