---
project: Cortex
component: Infra
phase: Design
date_created: 2026-01-16T21:14:25
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:06.785597
---

# Deployment Guide

This document outlines the steps and considerations for deploying Cortex Evaluator in various environments.

## 🐳 Docker Deployment (Recommended)

Cortex Evaluator is designed to run in a containerized environment using Docker Compose.

### Development Environment

The default `docker-compose.yml` is optimized for local development with hot-reloading for both frontend and backend.

```bash
docker-compose up --build
```

### Production Environment

For production, use the `docker-compose.prod.yml` which includes an Nginx reverse proxy, optimized builds, and persistent volumes.

```bash
# Start in production mode
./docker-prod-start.sh
```

**Production Stack:**
- **Frontend**: Nginx serving static React files.
- **Backend**: FastAPI with Uvicorn (multi-worker configuration).
- **Database**: PostgreSQL 15 (Alpine).
- **Vector DB**: ChromaDB with persistent storage.
- **Cache**: Redis 7 (Alpine).

## ⚙️ Environment Variables

Create a `.env` file in the root directory. Below are the key variables required:

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | SQLAlchemy connection string | `postgresql+psycopg2://cortex:cortex@postgres:5432/cortex_evaluator` |
| `REDIS_URL` | Redis connection URL | `redis://:cortex@redis:6379` |
| `CHROMA_HOST` | Hostname for ChromaDB | `chromadb` |
| `CHROMA_PORT` | Port for ChromaDB | `8000` |
| `GEMINI_API_KEY` | Google Gemini API Key | Required |
| `OPENAI_API_KEY` | OpenAI API Key | Required |
| `ANTHROPIC_API_KEY` | Anthropic (Claude) API Key | Required |
| `GROQ_API_KEY` | Groq API Key | Required |
| `OLLAMA_BASE_URL` | URL for local Ollama instance | `http://host.docker.internal:11434` |
| `GITHUB_API_TOKEN` | GitHub Personal Access Token | Required for private repos |
| `CORS_ORIGINS` | Allowed origins for CORS | `http://localhost:3000` |

## 🗄️ Database Migrations

Cortex Evaluator uses Alembic for database migrations.

### Running Migrations
When deploying or updating, ensure the database schema is up-to-date:

```bash
cd backend
alembic upgrade head
```

### Creating New Migrations
```bash
cd backend
alembic revision --autogenerate -m "description of changes"
```

## 🚀 Managed Platform Deployment

### Railway.app
1. Create a new project on Railway.
2. Connect your GitHub repository.
3. Add PostgreSQL, Redis, and ChromaDB plugins.
4. Set the environment variables in the Railway dashboard.
5. Railway will automatically detect the Dockerfiles and deploy.

### Fly.io
1. Install `flyctl`.
2. Run `fly launch` in the root directory.
3. Fly will detect the `docker-compose.yml` or Dockerfiles.
4. Scale as needed: `fly scale count 2`.

## 📊 Monitoring & Health Checks

The backend provides a comprehensive health check endpoint:

**Endpoint**: `GET /health`
**Response**:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "services": {
    "database": "connected",
    "chromadb": "connected",
    "redis": "connected"
  }
}
```

Use this endpoint for:
- Load balancer health checks.
- Uptime monitoring (e.g., Uptime Robot, BetterStack).
- Kubernetes Liveness/Readiness probes.

## 🔒 Security Checklist

- [ ] **API Keys**: Ensure all AI provider keys are stored as secrets, never committed to VCS.
- [ ] **HTTPS**: Use a reverse proxy (Nginx/Caddy) with SSL (Let's Encrypt).
- [ ] **Rate Limiting**: Implement rate limiting at the Nginx level or via FastAPI middleware.
- [ ] **Database Access**: Ensure PostgreSQL is not accessible from the public internet.
- [ ] **Secrets Rotation**: Periodically rotate GitHub tokens and AI provider keys.

## 💾 Backup & Restore

### PostgreSQL Backup
```bash
docker exec cortex-postgres pg_dump -U cortex cortex_evaluator > backup.sql
```

### ChromaDB Backup
ChromaDB data is stored in the `chroma_data` volume. To backup:
```bash
tar -cvf chroma_backup.tar ./data/chroma
```
