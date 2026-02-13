.PHONY: help build up down restart logs ps clean prune backup restore test shell

help:
	@echo "Cortex Evaluator - Docker Commands"
	@echo "==================================="
	@echo ""
	@echo "Development:"
	@echo "  make build        - Build all Docker images"
	@echo "  make up           - Start all services"
	@echo "  make down         - Stop all services"
	@echo "  make restart      - Restart all services"
	@echo "  make logs         - View all logs"
	@echo "  make logs-f       - Follow logs (tail -f)"
	@echo "  make ps           - Show running containers"
	@echo ""
	@echo "Production:"
	@echo "  make prod-build   - Build production images"
	@echo "  make prod-up      - Start production services"
	@echo "  make prod-down    - Stop production services"
	@echo "  make prod-scale   - Scale backend (default: 3)"
	@echo ""
	@echo "Maintenance:"
	@echo "  make clean        - Stop and remove containers"
	@echo "  make prune        - Remove unused Docker resources"
	@echo "  make backup       - Backup all volumes"
	@echo "  make restore      - Restore from backup"
	@echo "  make test         - Run tests"
	@echo ""
	@echo "Utilities:"
	@echo "  make shell        - Access backend shell"
	@echo "  make shell-db     - Access PostgreSQL shell"
	@echo "  make shell-redis  - Access Redis shell"
	@echo "  make health       - Check service health"

build:
	docker-compose build

up:
	docker-compose up -d

down:
	docker-compose down

restart:
	docker-compose restart

logs:
	docker-compose logs

logs-f:
	docker-compose logs -f

ps:
	docker-compose ps

prod-build:
	docker-compose -f docker-compose.prod.yml build

prod-up:
	docker-compose -f docker-compose.prod.yml --env-file .env.production up -d

prod-down:
	docker-compose -f docker-compose.prod.yml down

prod-scale:
	@read -p "Number of backend replicas: " replicas; \
	docker-compose -f docker-compose.prod.yml up -d --scale backend=$$replicas

clean:
	docker-compose down -v

prune:
	docker system prune -a --volumes

backup:
	@mkdir -p backups
	docker-compose exec postgres pg_dump -U cortex cortex_evaluator > backups/postgres_$$(date +%Y%m%d_%H%M%S).sql
	docker run --rm -v cortex-evaluator_postgres_data:/data -v $$(pwd)/backups:/backup \
		alpine tar czf /backup/postgres_data_$$(date +%Y%m%d_%H%M%S).tar.gz -C /data .
	docker run --rm -v cortex-evaluator_redis_data:/data -v $$(pwd)/backups:/backup \
		alpine tar czf /backup/redis_data_$$(date +%Y%m%d_%H%M%S).tar.gz -C /data .
	docker run --rm -v cortex-evaluator_chroma_data:/data -v $$(pwd)/backups:/backup \
		alpine tar czf /backup/chroma_data_$$(date +%Y%m%d_%H%M%S).tar.gz -C /data .
	@echo "Backup completed in backups/"

restore:
	@read -p "Backup file to restore (e.g., postgres_20240116_120000.sql): " backup; \
	docker-compose exec -T postgres psql -U cortex cortex_evaluator < backups/$$backup

test:
	docker-compose exec backend pytest -v

shell:
	docker-compose exec backend bash

shell-db:
	docker-compose exec postgres psql -U cortex -d cortex_evaluator

shell-redis:
	docker-compose exec redis redis-cli -a cortex

health:
	@echo "=== Service Health ==="
	@echo ""
	@echo "Backend:"
	@curl -s http://localhost:8000/health || echo "Backend not responding"
	@echo ""
	@echo "PostgreSQL:"
	@docker-compose exec postgres pg_isready -U cortex || echo "PostgreSQL not ready"
	@echo ""
	@echo "Redis:"
	@docker-compose exec redis redis-cli ping || echo "Redis not responding"
	@echo ""
	@echo "ChromaDB:"
	@curl -s http://localhost:8001/api/v1/heartbeat || echo "ChromaDB not responding"
	@echo ""
	@echo "Containers:"
	@docker-compose ps

install:
	cp .env.example .env
	@echo "Created .env file. Please configure your API keys before starting."

setup: install build up
	@echo "Setup complete! Access the application at:"
	@echo "  Frontend: http://localhost:3000"
	@echo "  Backend:  http://localhost:8000"
	@echo "  Docs:     http://localhost:8000/docs"

dev:
	docker-compose up --build

dev-logs:
	docker-compose up --build

prod:
	docker-compose -f docker-compose.prod.yml --env-file .env.production up -d --build

dev-clean: clean build up logs-f
