---
project: Cortex
component: Unknown
phase: Build
date_created: 2026-01-16T21:14:50
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:06.777837
---

# Backend Documentation

Cortex Evaluator's backend is built with FastAPI, providing a high-performance, asynchronous API for AI routing and codebase analysis.

## 🛠️ Tech Stack

- **Framework**: FastAPI (Python 3.11+)
- **ORM**: SQLModel (SQLAlchemy + Pydantic)
- **Database**: PostgreSQL (Production) / SQLite (Development)
- **Vector Store**: ChromaDB
- **Caching**: Redis
- **Migrations**: Alembic
- **AI Clients**: `openai`, `anthropic`, `google-genai`, `httpx`

## 📂 Directory Structure

```
backend/
├── app/
│   ├── api/            # Route handlers (evaluations, brainstorm, etc.)
│   ├── core/           # Configuration and security settings
│   ├── models/         # Database models and Pydantic schemas
│   ├── services/       # Business logic (AI Router, Vector DB, arXiv)
│   └── main.py         # Application entry point
├── alembic/            # Database migration scripts
├── data/               # Local data storage (ChromaDB, SQLite)
├── tests/              # Pytest suite
└── requirements.txt    # Python dependencies
```

## 🤖 AI Router & Fallback Logic

The `CortexRouter` (in `services/ai_router.py`) is the brain of the backend. It manages multiple AI providers and ensures reliable responses.

### Two-Lane Routing
- **Fast Lane**: Uses local models (Ollama) or fast cloud models (Groq, GPT-4o-mini) for simple tasks.
- **Smart Lane**: Escalates to high-reasoning models (Claude 3.5 Sonnet, GPT-4o) for complex architectural analysis.

### Circuit Breaker
If a provider fails or hits rate limits, the circuit breaker opens, and the router automatically falls back to the next available provider in the pool.

## 🗄️ Database & Vector Search

### Relational Data
We use SQLModel for a unified approach to database models and schemas. All sessions, projects, and evaluations are persisted here.

### Semantic Search (ChromaDB)
The `VectorStore` (in `services/vector_db.py`) indexes codebase files and past evaluations. This enables:
- **Semantic Search**: Finding relevant code snippets based on meaning rather than just keywords.
- **Evaluation Similarity**: Discovering past reviews that are technically similar to current work.

## 🛠️ Adding New Components

### Adding a New API Endpoint
1. Create a new file in `app/api/`.
2. Define the router: `router = APIRouter(prefix="/api/new-feature", tags=["new-feature"])`.
3. Include the router in `app/main.py`.

### Adding a New AI Provider
1. Define a new provider class in `app/services/ai_router.py`.
2. Implement the `analyze_code` and `brainstorm` methods.
3. Add the provider to the `CortexRouter` pools.

## 🧪 Testing

We use `pytest` for unit and integration testing.

```bash
cd backend
pytest
```

Critical paths to test:
- AI Router fallback logic.
- Vector DB indexing and querying.
- Database CRUD operations for sessions.

## 📡 Real-time Updates

The backend uses WebSockets (via FastAPI) to stream indexing progress for large repositories, ensuring the frontend stays updated without constant polling.
