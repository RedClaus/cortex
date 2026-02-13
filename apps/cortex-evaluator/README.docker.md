---
project: Cortex
component: Infra
phase: Design
date_created: 2026-01-16T21:09:10
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:06.751740
---

# Cortex Evaluator - Docker Setup Guide

Complete Docker and Docker Compose setup for Cortex Evaluator development and production environments.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Development Setup](#development-setup)
- [Production Setup](#production-setup)
- [Configuration](#configuration)
- [Volume Management](#volume-management)
- [Service Health](#service-health)
- [Troubleshooting](#troubleshooting)
- [Security Best Practices](#security-best-practices)

## Prerequisites

- Docker Engine 20.10+
- Docker Compose 2.0+
- 4GB+ RAM (8GB recommended)
- 10GB+ free disk space

```bash
docker --version
docker-compose --version
```

## Quick Start

```bash
# 1. Clone repository
git clone <repo-url>
cd cortex-evaluator

# 2. Copy environment file
cp .env.example .env

# 3. Configure your API keys
nano .env
# Set GEMINI_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY, etc.

# 4. Create ChromaDB auth file
echo "cortex-token" > chroma_auth.txt

# 5. Start all services
docker-compose up -d

# 6. Verify services
docker-compose ps
docker-compose logs -f
```

Access the application at:
- Frontend: http://localhost:3000
- Backend API: http://localhost:8000
- API Documentation: http://localhost:8000/docs
- Health Check: http://localhost:8000/health

## Development Setup

### Development Mode (Hot Reload)

```bash
# Start with hot reload enabled
docker-compose up -d

# View logs
docker-compose logs -f

# Rebuild specific service
docker-compose build backend
docker-compose up -d backend

# Access backend container
docker-compose exec backend bash

# Run tests in backend
docker-compose exec backend pytest

# Run database migrations
docker-compose exec backend alembic upgrade head

# Connect to PostgreSQL
docker-compose exec postgres psql -U cortex -d cortex_evaluator

# Connect to Redis
docker-compose exec redis redis-cli -a cortex

# Stop all services
docker-compose down

# Stop and remove volumes (delete all data)
docker-compose down -v
```

### Build without Cache

```bash
docker-compose build --no-cache
docker-compose up -d
```

### Running Individual Services

```bash
# Start only databases
docker-compose up -d postgres redis chromadb

# Start only backend
docker-compose up -d backend

# Start only frontend
docker-compose up -d frontend
```

## Production Setup

### Production Deployment

```bash
# 1. Create production environment file
cp .env.example .env.production

# 2. Edit production settings
nano .env.production
# Important:
# - Set strong passwords for POSTGRES_PASSWORD, REDIS_PASSWORD
# - Set DATABASE_URL to PostgreSQL connection
# - Set CORS_ORIGINS to your production domain
# - Set LOG_LEVEL to INFO or ERROR
# - Generate strong SECRET_KEY

# 3. Create ChromaDB auth file
openssl rand -hex 32 > chroma_auth.txt

# 4. Build and start production services
docker-compose -f docker-compose.prod.yml --env-file .env.production up -d --build

# 5. Verify health
curl http://localhost/health
curl http://localhost:8080/health
```

### Production Scaling

```bash
# Scale backend to 5 instances
docker-compose -f docker-compose.prod.yml up -d --scale backend=5

# Check scaling status
docker-compose -f docker-compose.prod.yml ps

# Rollback scaling
docker-compose -f docker-compose.prod.yml up -d --scale backend=3
```

### SSL/TLS Setup

```bash
# Create SSL directory
mkdir -p nginx-ssl

# Generate self-signed certificate (for testing)
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout nginx-ssl/cert.key \
  -out nginx-ssl/cert.crt

# OR use Let's Encrypt (for production)
certbot certonly --standalone -d your-domain.com
cp /etc/letsencrypt/live/your-domain.com/fullchain.pem nginx-ssl/cert.crt
cp /etc/letsencrypt/live/your-domain.com/privkey.pem nginx-ssl/cert.key

# Update nginx.prod.conf for SSL (uncomment SSL section)
docker-compose -f docker-compose.prod.yml restart nginx
```

### Production Backup

```bash
# Backup PostgreSQL
docker-compose -f docker-compose.prod.yml exec postgres \
  pg_dump -U cortex cortex_evaluator > backup_$(date +%Y%m%d).sql

# Backup volumes
docker run --rm -v cortex-prod_postgres_prod:/data -v $(pwd):/backup \
  alpine tar czf /backup/postgres_$(date +%Y%m%d).tar.gz -C /data .

# Restore PostgreSQL
docker-compose -f docker-compose.prod.yml exec -T postgres \
  psql -U cortex cortex_evaluator < backup_20240116.sql
```

## Configuration

### Environment Variables

#### Database
- `DATABASE_URL`: Database connection string
- `POSTGRES_USER`: PostgreSQL username
- `POSTGRES_PASSWORD`: PostgreSQL password
- `POSTGRES_DB`: PostgreSQL database name

#### Redis
- `REDIS_URL`: Redis connection string
- `REDIS_PASSWORD`: Redis authentication password

#### ChromaDB
- `CHROMA_HOST`: ChromaDB hostname
- `CHROMA_PORT`: ChromaDB port
- `CHROMA_AUTH_TOKEN`: ChromaDB authentication token

#### AI Providers
- `GEMINI_API_KEY`: Google Gemini API key
- `OPENAI_API_KEY`: OpenAI API key
- `ANTHROPIC_API_KEY`: Anthropic API key
- `GROQ_API_KEY`: Groq API key
- `OLLAMA_BASE_URL`: Ollama server URL

#### Application
- `CORS_ORIGINS`: Comma-separated list of allowed origins
- `LOG_LEVEL`: Logging level (DEBUG, INFO, WARNING, ERROR)
- `SECRET_KEY`: Secret key for encryption

### Custom Ports

Override ports in `.env`:

```bash
BACKEND_PORT=8080
FRONTEND_PORT=3001
POSTGRES_PORT=5433
REDIS_PORT=6380
CHROMA_PORT=8002
```

Then restart services:

```bash
docker-compose down
docker-compose up -d
```

## Volume Management

### List Volumes

```bash
docker volume ls | grep cortex
```

### Inspect Volume

```bash
docker volume inspect cortex-evaluator_postgres_data
```

### Backup Volume

```bash
docker run --rm -v cortex-evaluator_postgres_data:/data -v $(pwd):/backup \
  alpine tar czf /backup/postgres_$(date +%Y%m%d).tar.gz -C /data .
```

### Restore Volume

```bash
docker run --rm -v cortex-evaluator_postgres_data:/data -v $(pwd):/backup \
  alpine tar xzf /backup/postgres_20240116.tar.gz -C /data
```

### Delete Volume

```bash
# Stop services first
docker-compose down

# Remove volume
docker volume rm cortex-evaluator_postgres_data
```

## Service Health

### Check All Services

```bash
docker-compose ps
```

### Check Specific Service

```bash
docker-compose ps backend
```

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f backend

# Last 100 lines
docker-compose logs --tail=100 backend
```

### Health Checks

```bash
# Backend health
curl http://localhost:8000/health

# PostgreSQL health
docker-compose exec postgres pg_isready -U cortex

# Redis health
docker-compose exec redis redis-cli ping

# ChromaDB health
curl http://localhost:8001/api/v1/heartbeat
```

### Restart Services

```bash
# Restart all
docker-compose restart

# Restart specific service
docker-compose restart backend

# Graceful shutdown
docker-compose stop
docker-compose start
```

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker-compose logs backend

# Check if port is already in use
lsof -i :8000

# Kill process using port
kill -9 $(lsof -t -i:8000)
```

### Database Connection Issues

```bash
# Check PostgreSQL is ready
docker-compose exec postgres pg_isready -U cortex

# Check PostgreSQL logs
docker-compose logs postgres

# Test connection from backend
docker-compose exec backend python -c "
from sqlalchemy import create_engine
engine = create_engine('postgresql+psycopg2://cortex:cortex@postgres:5432/cortex_evaluator')
connection = engine.connect()
print('Connected!')
"
```

### High Memory Usage

```bash
# Check container resource usage
docker stats

# Restart with resource limits
docker-compose up -d --force-recreate
```

### Build Failures

```bash
# Clean build cache
docker builder prune -a

# Rebuild without cache
docker-compose build --no-cache

# Check Docker disk usage
docker system df

# Clean up unused images
docker image prune -a
```

### Volume Permission Issues

```bash
# Fix volume permissions
docker-compose exec backend chown -R www-data:www-data /app/data
```

### Frontend Build Errors

```bash
# Clear node_modules and rebuild
docker-compose exec frontend rm -rf node_modules package-lock.json
docker-compose restart frontend
```

### Network Issues

```bash
# Check network
docker network ls
docker network inspect cortex-evaluator_cortex-network

# Rebuild network
docker-compose down
docker network prune
docker-compose up -d
```

### ChromaDB Issues

```bash
# Check ChromaDB logs
docker-compose logs chromadb

# Reset ChromaDB data
docker-compose down
docker volume rm cortex-evaluator_chroma_data
docker-compose up -d chromadb
```

## Security Best Practices

### 1. Never Commit Secrets

```bash
# Add to .gitignore
echo ".env" >> .gitignore
echo "chroma_auth.txt" >> .gitignore
```

### 2. Use Strong Passwords

```bash
# Generate strong password
openssl rand -base64 32
```

### 3. Enable SSL in Production

```bash
# Use nginx.prod.conf with SSL
# Obtain SSL certificates from Let's Encrypt
```

### 4. Limit Container Resources

```bash
# Production mode already has limits
# Adjust in docker-compose.prod.yml as needed
```

### 5. Regular Updates

```bash
# Update base images
docker-compose pull
docker-compose up -d --build
```

### 6. Network Isolation

```bash
# Services use internal network
# Only expose necessary ports
```

### 7. Backup Regularly

```bash
# Automate backups with cron
0 2 * * * /path/to/backup-script.sh
```

## Performance Tuning

### PostgreSQL

```bash
# Edit docker-compose.yml
# Add to postgres environment:
POSTGRES_SHARED_BUFFERS: 256MB
POSTGRES_EFFECTIVE_CACHE_SIZE: 1GB
POSTGRES_MAX_CONNECTIONS: 100
```

### Redis

```bash
# Edit docker-compose.yml
# Add maxmemory setting:
command: redis-server --maxmemory 256mb --maxmemory-policy allkeys-lru
```

### Backend

```bash
# Increase workers in Dockerfile
CMD ["gunicorn", "app.main:app", "-w", "4", "-k", "uvicorn.workers.UvicornWorker", "--bind", "0.0.0.0:8000"]
```

## Monitoring

### Container Monitoring

```bash
# Real-time stats
docker stats

# Resource usage history
docker stats --no-stream
```

### Log Aggregation

```bash
# Export logs
docker-compose logs > logs.txt

# Follow specific log
docker-compose logs -f backend | grep ERROR
```

## Cleanup

### Remove All Containers and Volumes

```bash
docker-compose down -v
```

### Prune System

```bash
# Remove unused data
docker system prune -a --volumes
```

### Reset Everything

```bash
# Stop and remove everything
docker-compose down -v

# Remove all images
docker rmi $(docker images -q cortex-*)
```

## Support

For issues and questions:
- Check logs: `docker-compose logs`
- Verify health: `curl http://localhost:8000/health`
- Review configuration: `.env`
- Check documentation: `/docs` endpoint

## License

[Your License Here]
