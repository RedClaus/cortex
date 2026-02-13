---
project: Cortex
component: Infra
phase: Build
date_created: 2026-01-16T21:11:29
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:06.760656
---

# Docker Setup Summary

Complete Docker and Docker Compose setup for Cortex Evaluator development and production environments.

## Files Created

### Core Configuration
- **docker-compose.yml** - Development environment with all services
- **docker-compose.prod.yml** - Production environment with load balancing
- **.env.example** - Template for environment variables
- **chroma_auth.txt** - ChromaDB authentication (create with: `echo "cortex-token" > chroma_auth.txt`)

### Dockerfiles
- **backend/Dockerfile** - Multi-stage Python build (3.11-slim)
- **frontend/Dockerfile** - Multi-stage Node.js build (development + production)

### Nginx Configuration
- **frontend/nginx.conf** - Nginx config for frontend static files
- **nginx.prod.conf** - Production Nginx with load balancing and upstream

### Ignore Files
- **backend/.dockerignore** - Files to exclude from backend builds
- **frontend/.dockerignore** - Files to exclude from frontend builds

### Documentation & Utilities
- **README.docker.md** - Comprehensive Docker usage guide
- **Makefile** - Convenience commands for Docker operations
- **docker-start.sh** - Quick start script for development
- **docker-prod-start.sh** - Quick start script for production

## Services Overview

### Development Stack

| Service | Image | Ports | Purpose |
|---------|-------|-------|---------|
| backend | python:3.11-slim | 8000 | FastAPI application |
| frontend | node:18-alpine | 3000 | Vite dev server |
| postgres | postgres:15-alpine | 5432 | PostgreSQL database |
| redis | redis:7-alpine | 6379 | Redis cache |
| chromadb | chromadb/chroma:latest | 8001 | Vector database |

### Production Stack

| Service | Image | Ports | Replicas |
|---------|-------|-------|----------|
| backend | python:3.11-slim | internal | 3 (scalable) |
| frontend | nginx:alpine | 80, 443 | 1 |
| nginx | nginx:alpine | 8080, 443 | 1 |
| postgres | postgres:15-alpine | internal | 1 |
| redis | redis:7-alpine | internal | 1 |
| chromadb | chromadb/chroma:latest | internal | 1 |

## Quick Start

### Development

```bash
# 1. Setup environment
cp .env.example .env
nano .env  # Add your API keys
echo "cortex-token" > chroma_auth.txt

# 2. Start services
./docker-start.sh

# OR use Docker Compose directly
docker-compose up -d

# 3. Verify
make health
curl http://localhost:8000/health
```

### Production

```bash
# 1. Setup production environment
cp .env.example .env.production
nano .env.production  # Configure production settings
openssl rand -hex 32 > chroma_auth.txt

# 2. Start production services
./docker-prod-start.sh

# 3. Scale backend
make prod-scale
```

## Makefile Commands

```bash
make help          # Show all commands
make build         # Build images
make up            # Start services
make down          # Stop services
make logs          # View logs
make logs-f        # Follow logs
make ps            # Show containers
make health        # Check health
make shell         # Backend shell
make shell-db      # PostgreSQL shell
make shell-redis   # Redis shell
make test          # Run tests
make backup        # Backup volumes
make clean         # Clean up
```

## Access Points

### Development
- Frontend: http://localhost:3000
- Backend API: http://localhost:8000
- API Docs: http://localhost:8000/docs
- Health Check: http://localhost:8000/health
- PostgreSQL: localhost:5432
- Redis: localhost:6379
- ChromaDB: localhost:8001

### Production
- Frontend (HTTP): http://localhost:80
- Frontend (HTTPS): https://your-domain.com:443
- Nginx (HTTP): http://localhost:8080
- Backend (internal): port 8000

## Volume Management

All data is persisted in Docker volumes:
- `cortex-evaluator_postgres_data` - PostgreSQL database
- `cortex-evaluator_redis_data` - Redis cache
- `cortex-evaluator_chroma_data` - ChromaDB vector store
- `cortex-evaluator_backend_data` - Backend application data

### Backup Commands

```bash
# Backup all volumes
make backup

# Manual backup
docker-compose exec postgres pg_dump -U cortex cortex_evaluator > backup.sql
```

### Restore Commands

```bash
# Restore from backup
make restore

# Manual restore
docker-compose exec -T postgres psql -U cortex cortex_evaluator < backup.sql
```

## Health Checks

All services have health checks configured:

```bash
# Check all services
make health

# Individual checks
curl http://localhost:8000/health          # Backend
docker-compose exec postgres pg_isready    # PostgreSQL
docker-compose exec redis redis-cli ping   # Redis
curl http://localhost:8001/api/v1/heartbeat  # ChromaDB
```

## Resource Limits

### Production Resources

| Service | CPU Limit | Memory Limit | CPU Reserve | Memory Reserve |
|---------|-----------|--------------|-------------|----------------|
| postgres | 1.0 | 512MB | 0.5 | 256MB |
| redis | 0.5 | 256MB | 0.25 | 128MB |
| chromadb | 1.0 | 1GB | 0.5 | 512MB |
| backend | 1.0 | 1GB | 0.5 | 512MB |
| frontend | 0.5 | 256MB | 0.25 | 128MB |
| nginx | 0.5 | 256MB | 0.25 | 128MB |

## Security Features

- Environment-based configuration
- Password-protected PostgreSQL and Redis
- Token-based ChromaDB authentication
- CORS restrictions
- Network isolation
- SSL/TLS support (production)
- Resource limits to prevent DoS
- Health checks for monitoring
- Graceful shutdown handling

## Troubleshooting

### Common Issues

1. **Container won't start**
   ```bash
   docker-compose logs backend
   ```

2. **Port conflicts**
   ```bash
   lsof -i :8000
   ```

3. **Database connection issues**
   ```bash
   docker-compose exec postgres pg_isready -U cortex
   ```

4. **High memory usage**
   ```bash
   docker stats
   ```

5. **Build failures**
   ```bash
   docker-compose build --no-cache
   ```

See [README.docker.md](README.docker.md) for complete troubleshooting guide.

## Next Steps

1. Configure your environment variables in `.env`
2. Start the development environment: `./docker-start.sh`
3. Access the application at http://localhost:3000
4. Review the [API documentation](http://localhost:8000/docs)
5. Configure production settings when ready to deploy

## Support

For detailed information, see [README.docker.md](README.docker.md)

---

**Note:** Never commit `.env`, `chroma_auth.txt`, or `.env.production` to version control.
