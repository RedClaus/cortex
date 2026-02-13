---
project: Cortex
component: Unknown
phase: Ideation
date_created: 2026-01-16T19:56:43
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:06.830341
---

# Cortex Evaluator - Implementation Plan

**Project Codebase:** `frontier-code-review-&-feature-architect`
**Status:** Initial Frontend Complete - Ready for Full-Stack Enhancement
**Last Updated:** January 2026
**Estimated Timeline:** 12-17 weeks (based on roadmap)

---

## Executive Summary

This document provides a detailed, phase-by-phase implementation plan to transform the existing **Frontier Code Review & Feature Architect** from a pure frontend prototype into a comprehensive **development workflow tool** with brainstorming, evaluation, and CR generation capabilities.

**Current State:** React 19 frontend with Gemini-only AI integration, no persistence
**Target State:** Full-stack application with multi-provider AI routing, persistent storage, brainstorming canvas, and extensive input sources

---

## Current Implementation Assessment

### ✅ Existing Capabilities

| Feature | Status | Code Location |
|----------|----------|----------------|
| **Local Folder Import** | ✅ Implemented | `services/contextStore.ts` |
| **GitHub Repo Import** | ✅ Implemented | `services/githubService.ts` |
| **PDF Upload** | ✅ Implemented | `App.tsx:handlePdfUpload` |
| **Code Snippet Input** | ✅ Implemented | `App.tsx:addSnippet` |
| **System Documentation Gen** | ✅ Implemented | `services/documentationService.ts` |
| **AI Analysis (Gemini)** | ✅ Implemented | `services/geminiService.ts` |
| **CR Generation** | ✅ Implemented | `geminiService.ts` |
| **UI/UX** | ✅ Implemented | React + Tailwind, dark/light mode |

### ❌ Critical Gaps (from roadmap)

1. **No Persistence** - Everything lost on page refresh
2. **Single Provider** - Only Gemini implemented (UI shows others as "Coming Soon")
3. **No Brainstorming Tools** - Just analysis, no ideation workspace
4. **No Version Control Integration** - Can't push CRs to repos
5. **No History/Session Management** - No tracking of past evaluations
6. **No Collaborative Features** - Single user only
7. **No Export Options** - Can't export to different formats
8. **Limited Input Types** - Missing arXiv, URLs, markdown files

---

## Proposed Architecture

Based on research and best practices, the target architecture:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Cortex Evaluator Full Stack                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────┐    ┌──────────┐    ┌──────────────────────┐  │
│  │  React   │───▶│  FastAPI │───▶│  PostgreSQL/SQLite   │  │
│  │ Frontend │    │  Backend │    │  (Projects/Sessions) │  │
│  └──────────┘    └──────────┘    └──────────────────────┘  │
│       │              │                                      │
│       │              ├────▶ ChromaDB/Qdrant (Vector Store) │
│       │              ├────▶ Multi-Provider AI Router        │
│       │              ├────▶ GitHub API                      │
│       │              └────▶ arXiv/Papers API               │
│       │                                                      │
│       └─────────────────────▶ WebSocket (Real-time updates)       │
│                                                               │
└───────────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Core Infrastructure (Weeks 1-3)

**Priority:** P0 (Critical Path)
**Effort:** 2-3 weeks
**Key Deliverables:** Backend API, persistence, multi-provider routing

### 1.1 Backend API & Persistence

**Technology Stack:**
- **Backend:** FastAPI (Python) or Hono (TypeScript)
  - *Decision point: Python aligns with CortexBrain ecosystem, TS aligns with existing frontend*
- **Database:** SQLite (single-user) → PostgreSQL (multi-user future)
- **Vector DB:** ChromaDB (dev) → Qdrant (production)
- **Caching:** Redis for expensive API calls
- **Real-time:** WebSocket for indexing status

**Project Structure:**

```
cortex-evaluator/
├── frontend/                    # Existing React app (move from root)
│   ├── src/
│   │   ├── components/
│   │   ├── services/
│   │   ├── stores/              # NEW: Zustand stores
│   │   ├── hooks/
│   │   └── App.tsx
│   ├── package.json
│   └── vite.config.ts
│
├── backend/                     # NEW: FastAPI backend
│   ├── app/
│   │   ├── main.py
│   │   ├── api/
│   │   │   ├── evaluations.py
│   │   │   ├── codebases.py
│   │   │   ├── sessions.py
│   │   │   └── brainstorm.py
│   │   ├── models/
│   │   │   ├── database.py      # SQLAlchemy/SQLModel models
│   │   │   └── schemas.py       # Pydantic schemas
│   │   ├── services/
│   │   │   ├── ai_router.py     # Multi-provider routing
│   │   │   ├── vector_db.py     # ChromaDB/Qdrant
│   │   │   ├── github_service.py
│   │   │   └── arxiv_service.py
│   │   └── core/
│   │       ├── config.py
│   │       └── security.py
│   ├── requirements.txt
│   └── alembic/               # Database migrations
│
├── shared/                      # NEW: Shared types/interfaces
│   └── types.ts               # TypeScript types (shared with backend via codegen)
│
├── docker-compose.yml            # Dev environment
├── .env.example
└── README.md
```

**Database Schema (SQLite/PostgreSQL):**

```sql
-- Projects
CREATE TABLE projects (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Codebases (linked to projects)
CREATE TABLE codebases (
    id UUID PRIMARY KEY,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL, -- 'local', 'github', 'gitlab'
    source_url TEXT,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Files within codebases
CREATE TABLE codebase_files (
    id UUID PRIMARY KEY,
    codebase_id UUID REFERENCES codebases(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    path TEXT NOT NULL,
    content TEXT, -- Full content or reference to vector DB
    file_type VARCHAR(50),
    indexed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Evaluations/Reviews
CREATE TABLE evaluations (
    id UUID PRIMARY KEY,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    input_type VARCHAR(50) NOT NULL, -- 'pdf', 'repo', 'snippet', 'arxiv', 'url'
    input_name VARCHAR(255) NOT NULL,
    input_content TEXT,
    file_data BYTEA, -- For PDFs
    provider_id VARCHAR(50) NOT NULL, -- 'gemini', 'openai', 'anthropic', etc.
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Evaluation Results
CREATE TABLE evaluation_results (
    id UUID PRIMARY KEY,
    evaluation_id UUID REFERENCES evaluations(id) ON DELETE CASCADE,
    value_score INTEGER CHECK (value_score >= 0 AND value_score <= 100),
    executive_summary TEXT,
    technical_feasibility TEXT,
    gap_analysis TEXT,
    suggested_cr TEXT,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Brainstorming Sessions
CREATE TABLE brainstorm_sessions (
    id UUID PRIMARY KEY,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    nodes JSONB NOT NULL, -- React Flow nodes
    edges JSONB NOT NULL, -- React Flow edges
    viewport JSONB, -- Camera position {x, y, zoom}
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Change Requests
CREATE TABLE change_requests (
    id UUID PRIMARY KEY,
    evaluation_id UUID REFERENCES evaluations(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL, -- 'feature', 'refactor', 'bugfix', 'research'
    summary TEXT,
    tasks JSONB, -- Array of task objects
    estimation JSONB, -- {optimistic, expected, pessimistic, complexity}
    dependencies JSONB,
    risk_factors JSONB,
    template_id VARCHAR(50), -- 'claude-code', 'jira', 'github', etc.
    status VARCHAR(50) DEFAULT 'pending', -- 'pending', 'in-progress', 'completed', 'rejected'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Vector DB Schema (ChromaDB/Qdrant):**

**ChromaDB Collections:**
```python
# Code snippets for semantic search
"code_snippets" -> {
    ids: [file_id],
    documents: [code_content],
    metadatas: [{
        file_path: string,
        language: string,
        function_name: string,
        line_number: integer
    }]
}

# Evaluation history for similarity search
"evaluations" -> {
    ids: [evaluation_id],
    documents: [evaluation_summary + cr],
    metadatas: [{
        project_id: string,
        input_type: string,
        provider: string,
        value_score: integer,
        created_at: timestamp
    }]
}

# ArXiv papers for research context
"papers" -> {
    ids: [arxiv_id],
    documents: [paper_text],
    metadatas: [{
        title: string,
        authors: array,
        categories: array,
        published: timestamp
    }]
}
```

### 1.2 Multi-Provider AI Routing (Cortex Integration)

**Implementation Pattern:** OpenAI-Compatible Proxy Layer (from research)

**Provider Registry:**

```python
# backend/app/services/ai_router.py

from typing import Protocol
from pydantic import BaseModel
import openai
import anthropic
from openai import OpenAI

class AIProvider(Protocol):
    async def analyze_code(
        self,
        codebase: list,
        input_data: dict,
        system_doc: dict
    ) -> dict:
        ...

    async def brainstorm(
        self,
        topic: str,
        constraints: list[str]
    ) -> list[dict]:
        ...

class GeminiProvider:
    def __init__(self, api_key: str):
        from google import genai
        self.client = genai.Client(api_key=api_key)

    async def analyze_code(self, codebase, input_data, system_doc) -> dict:
        # Existing implementation from geminiService.ts
        pass

class ClaudeProvider:
    def __init__(self, api_key: str):
        self.client = anthropic.Anthropic(api_key=api_key)

    async def analyze_code(self, codebase, input_data, system_doc) -> dict:
        message = self.client.messages.create(
            model="claude-3-5-sonnet-20241022",
            max_tokens=4096,
            system=self._build_system_prompt(system_doc),
            messages=[{
                "role": "user",
                "content": self._build_analysis_prompt(codebase, input_data)
            }]
        )
        return self._parse_response(message)

class OpenAIProvider:
    def __init__(self, api_key: str):
        self.client = OpenAI(api_key=api_key)

    async def analyze_code(self, codebase, input_data, system_doc) -> dict:
        response = self.client.chat.completions.create(
            model="gpt-4o",
            messages=[
                {"role": "system", "content": self._build_system_prompt(system_doc)},
                {"role": "user", "content": self._build_analysis_prompt(codebase, input_data)}
            ],
            response_format={"type": "json_object"}
        )
        return json.loads(response.choices[0].message.content)

class OllamaProvider:
    def __init__(self, base_url: str = "http://localhost:11434"):
        self.client = openai.OpenAI(
            base_url=base_url,
            api_key="ollama"  # Ollama doesn't require API key
        )

    async def analyze_code(self, codebase, input_data, system_doc) -> dict:
        response = self.client.chat.completions.create(
            model="llama3:8b",
            messages=[
                {"role": "system", "content": self._build_system_prompt(system_doc)},
                {"role": "user", "content": self._build_analysis_prompt(codebase, input_data)}
            ]
        )
        return json.loads(response.choices[0].message.content)

class GroqProvider:
    def __init__(self, api_key: str):
        self.client = OpenAI(
            base_url="https://api.groq.com/openai/v1",
            api_key=api_key
        )

    async def analyze_code(self, codebase, input_data, system_doc) -> dict:
        response = self.client.chat.completions.create(
            model="llama-3.1-70b-versatile",
            messages=[...],
            response_format={"type": "json_object"}
        )
        return json.loads(response.choices[0].message.content)
```

**Two-Lane Smart Routing Logic:**

```python
# backend/app/services/ai_router.py (continued)

from enum import Enum

class RoutingLane(str, Enum):
    FAST = "fast"      # Local/Cheap
    SMART = "smart"    # High-quality/Escalation

class CortexRouter:
    def __init__(self, config: dict):
        # Fast Lane providers
        self.fast_lane = [
            OllamaProvider(config.get("ollama_url")),
            GroqProvider(config.get("groq_api_key")),
        ]

        # Smart Lane providers
        self.smart_lane = [
            ClaudeProvider(config.get("anthropic_api_key")),
            OpenAIProvider(config.get("openai_api_key")),
            GeminiProvider(config.get("gemini_api_key")),
        ]

    async def route_analysis(
        self,
        codebase: list,
        input_data: dict,
        system_doc: dict,
        user_intent: str = None
    ) -> tuple[AIProvider, RoutingLane]:
        """
        Three-Phase Routing:
        1. Hard Constraints
        2. User Intent
        3. Default Fast Lane
        """

        # Phase 1: Hard Constraints
        if input_data.get("type") == "pdf" and "vision" in input_data:
            # Vision requirements → Claude/GPT-4
            return self.smart_lane[0], RoutingLane.SMART

        if len(str(codebase)) > 128000:  # Context overflow
            return self.smart_lane[0], RoutingLane.SMART

        # Phase 2: User Intent
        if user_intent == "strong":
            return self.smart_lane[0], RoutingLane.SMART
        elif user_intent == "local":
            return self.fast_lane[0], RoutingLane.FAST
        elif user_intent == "cheap":
            return self.fast_lane[1], RoutingLane.FAST

        # Phase 3: Default Fast Lane
        return self.fast_lane[0], RoutingLane.FAST

    async def analyze_with_fallback(
        self,
        codebase: list,
        input_data: dict,
        system_doc: dict,
        user_intent: str = None
    ) -> dict:
        """Sequential fallback with circuit breaker"""
        provider, lane = await self.route_analysis(codebase, input_data, system_doc, user_intent)

        # Select provider pool based on lane
        provider_pool = self.smart_lane if lane == RoutingLane.SMART else self.fast_lane

        # Try providers sequentially
        for attempt, provider in enumerate(provider_pool):
            try:
                result = await provider.analyze_code(codebase, input_data, system_doc)
                # Validate structured output
                self._validate_result(result)
                return result
            except Exception as e:
                print(f"Provider {provider.__class__.__name__} failed (attempt {attempt + 1}): {e}")
                if attempt < len(provider_pool) - 1:
                    continue  # Try next provider
                else:
                    raise Exception("All providers failed")

    def _validate_result(self, result: dict):
        """Validate required fields in structured output"""
        required_fields = ["valueScore", "executiveSummary", "technicalFeasibility", "gapAnalysis", "suggestedCR"]
        for field in required_fields:
            if field not in result:
                raise ValueError(f"Missing required field: {field}")
```

**Circuit Breaker Pattern:**

```python
# backend/app/services/circuit_breaker.py

import time
from enum import Enum
from dataclasses import dataclass

class CircuitState(Enum):
    CLOSED = "closed"     # Normal operation
    OPEN = "open"         # Failure threshold exceeded
    HALF_OPEN = "half_open"  # Testing recovery

@dataclass
class CircuitBreakerConfig:
    failure_threshold: int = 5        # Open after 5 failures
    success_threshold: int = 2        # Close after 2 successes (half-open)
    timeout: float = 60.0             # Retry after 60 seconds
    window_size: int = 10             # Track last 10 requests

class CircuitBreaker:
    def __init__(self, provider_name: str, config: CircuitBreakerConfig = None):
        self.provider_name = provider_name
        self.config = config or CircuitBreakerConfig()
        self.state = CircuitState.CLOSED
        self.failure_count = 0
        self.success_count = 0
        self.last_failure_time = 0
        self.request_history = []  # Sliding window

    async def call(self, func, *args, **kwargs):
        """Execute function with circuit breaker protection"""
        if self.state == CircuitState.OPEN:
            if time.time() - self.last_failure_time > self.config.timeout:
                self.state = CircuitState.HALF_OPEN
                print(f"Circuit breaker HALF_OPEN for {self.provider_name}")
            else:
                raise Exception(f"Circuit breaker OPEN for {self.provider_name}")

        try:
            result = await func(*args, **kwargs)
            self._on_success()
            return result
        except Exception as e:
            self._on_failure()
            raise e

    def _on_success(self):
        self.success_count += 1
        self.request_history.append(True)

        if self.state == CircuitState.HALF_OPEN:
            if self.success_count >= self.config.success_threshold:
                self.state = CircuitState.CLOSED
                self.failure_count = 0
                self.success_count = 0
                print(f"Circuit breaker CLOSED for {self.provider_name}")

        self._trim_history()

    def _on_failure(self):
        self.failure_count += 1
        self.request_history.append(False)
        self.last_failure_time = time.time()

        if self.failure_count >= self.config.failure_threshold:
            self.state = CircuitState.OPEN
            print(f"Circuit breaker OPEN for {self.provider_name}")

        self._trim_history()

    def _trim_history(self):
        if len(self.request_history) > self.config.window_size:
            self.request_history = self.request_history[-self.config.window_size:]

# Usage in router
breaker = CircuitBreaker("gemini")
result = await breaker.call(provider.analyze_code, codebase, input_data, system_doc)
```

### 1.3 Vector DB Integration

**ChromaDB Setup (Development):**

```python
# backend/app/services/vector_db.py

import chromadb
from chromadb.utils import embedding_functions
import os

class VectorStore:
    def __init__(self):
        # Use OpenAI embeddings (or local SentenceTransformer)
        self.openai_ef = embedding_functions.OpenAIEmbeddingFunction(
            api_key=os.getenv("OPENAI_API_KEY"),
            model_name="text-embedding-3-small"
        )

        self.client = chromadb.PersistentClient(path="./data/chroma")

        # Collections
        self.code_snippets = self.client.get_or_create_collection(
            name="code_snippets",
            embedding_function=self.openai_ef,
            metadata={"hnsw:space": "cosine", "hnsw:M": 16}
        )

        self.evaluations = self.client.get_or_create_collection(
            name="evaluations",
            embedding_function=self.openai_ef,
            metadata={"hnsw:space": "cosine"}
        )

    async def index_codebase(
        self,
        codebase_id: str,
        files: list[dict],
        on_progress: callable = None
    ):
        """Index codebase files for semantic search"""
        batch_size = 100

        for i in range(0, len(files), batch_size):
            batch = files[i:i+batch_size]

            ids = [f"{codebase_id}:{f['path']}" for f in batch]
            documents = [f['content'][:10000] for f in batch]  # Limit size
            metadatas = [{
                "file_path": f['path'],
                "language": f['type'],
                "codebase_id": codebase_id
            } for f in batch]

            self.code_snippets.add(
                ids=ids,
                documents=documents,
                metadatas=metadatas
            )

            if on_progress:
                on_progress(len(files))

    async def search_similar_evaluations(
        self,
        query: str,
        n_results: int = 5,
        filters: dict = None
    ) -> list:
        """Find semantically similar evaluations"""
        where_clause = {}
        if filters:
            for key, value in filters.items():
                where_clause[key] = value

        results = self.evaluations.query(
            query_texts=[query],
            n_results=n_results,
            where=where_clause if where_clause else None,
            include=["documents", "metadatas", "distances"]
        )

        return [
            {
                "id": results["ids"][0][i],
                "similarity": 1 - results["distances"][0][i],
                "metadata": results["metadatas"][0][i]
            }
            for i in range(len(results["ids"][0]))
        ]

    async def store_evaluation(self, evaluation_id: str, summary: str, cr: str, metadata: dict):
        """Store evaluation result for future similarity search"""
        text = f"{summary}\n\n{cr}"
        self.evaluations.add(
            ids=[evaluation_id],
            documents=[text],
            metadatas=[metadata]
        )
```

**API Endpoints:**

```python
# backend/app/api/evaluations.py

from fastapi import APIRouter, HTTPException, BackgroundTasks
from pydantic import BaseModel
from ..services.ai_router import CortexRouter
from ..services.vector_db import VectorStore

router = APIRouter(prefix="/api/evaluations", tags=["evaluations"])

router_instance = CortexRouter({})  # Load from config
vector_store = VectorStore()

class EvaluationRequest(BaseModel):
    codebase_id: str
    input_type: str  # 'pdf', 'repo', 'snippet'
    input_content: str
    file_data: dict = None  # For PDFs
    provider_preference: str = None  # Optional override
    user_intent: str = None  # 'strong', 'local', 'cheap'

class EvaluationResponse(BaseModel):
    id: str
    value_score: int
    executive_summary: str
    technical_feasibility: str
    gap_analysis: str
    suggested_cr: str
    provider_used: str
    similar_evaluations: list = None

@router.post("/analyze", response_model=EvaluationResponse)
async def analyze_evaluation(
    request: EvaluationRequest,
    background_tasks: BackgroundTasks
):
    """Run evaluation against codebase with AI analysis"""

    # 1. Fetch codebase from database
    # (Implement codebase fetching logic)

    # 2. Fetch system documentation
    # (Use existing documentationService pattern)

    # 3. Run AI analysis with router
    try:
        result = await router_instance.analyze_with_fallback(
            codebase=codebase_files,
            input_data={
                "type": request.input_type,
                "content": request.input_content,
                "fileData": request.file_data
            },
            system_doc=system_doc,
            user_intent=request.user_intent
        )

        # 4. Save evaluation to database
        evaluation_id = str(uuid.uuid4())
        # (Database save logic)

        # 5. Store in vector DB (background task)
        background_tasks.add_task(
            vector_store.store_evaluation,
            evaluation_id,
            result["executiveSummary"],
            result["suggestedCR"],
            {
                "provider": result.get("provider"),
                "value_score": result["valueScore"],
                "input_type": request.input_type
            }
        )

        # 6. Find similar evaluations
        similar = await vector_store.search_similar_evaluations(
            result["executiveSummary"],
            n_results=5
        )

        return EvaluationResponse(
            id=evaluation_id,
            value_score=result["valueScore"],
            executive_summary=result["executiveSummary"],
            technical_feasibility=result["technicalFeasibility"],
            gap_analysis=result["gapAnalysis"],
            suggested_cr=result["suggestedCR"],
            provider_used=result.get("provider"),
            similar_evaluations=similar
        )

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@router.get("/history/{evaluation_id}")
async def get_evaluation_history(evaluation_id: str):
    """Get specific evaluation with full history"""
    # (Database fetch logic)
    pass

@router.get("/similar/{evaluation_id}")
async def get_similar_evaluations(evaluation_id: str, limit: int = 10):
    """Get evaluations similar to given ID"""
    # (Vector search logic)
    pass
```

### 1.4 WebSocket Support for Real-Time Updates

```python
# backend/app/api/websocket.py

from fastapi import WebSocket, WebSocketDisconnect

@router.websocket("/ws/indexing")
async def websocket_indexing(websocket: WebSocket):
    await websocket.accept()
    try:
        while True:
            data = await websocket.receive_json()

            if data["type"] == "indexing_progress":
                await websocket.send_json({
                    "type": "progress",
                    "file": data["current_file"],
                    "count": data["processed_count"],
                    "total": data["total_count"]
                })

            elif data["type"] == "indexing_complete":
                await websocket.send_json({
                    "type": "complete",
                    "codebase_id": data["codebase_id"]
                })

    except WebSocketDisconnect:
        print("WebSocket disconnected")
```

**Frontend WebSocket Integration:**

```typescript
// frontend/src/hooks/useWebsocket.ts

import { useEffect, useState } from 'react';

export function useIndexingProgress(codebaseId: string) {
  const [progress, setProgress] = useState({
    file: '',
    count: 0,
    total: 0,
    percentage: 0
  });

  useEffect(() => {
    const ws = new WebSocket(`ws://localhost:8000/ws/indexing`);

    ws.onopen = () => {
      ws.sendJSON({ type: 'start_indexing', codebase_id: codebaseId });
    };

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);

      if (data.type === 'progress') {
        setProgress({
          file: data.file,
          count: data.count,
          total: data.total,
          percentage: Math.round((data.count / data.total) * 100)
        });
      } else if (data.type === 'complete') {
        setProgress(prev => ({ ...prev, percentage: 100 }));
      }
    };

    return () => ws.close();
  }, [codebaseId]);

  return progress;
}
```

---

## Phase 2: Enhanced Input Sources (Weeks 4-5)

**Priority:** P1 (High)
**Effort:** 1-2 weeks
**Key Deliverables:** arXiv, web URLs, markdown files

### 2.1 arXiv Integration

**Service Implementation:**

```python
# backend/app/services/arxiv_service.py

import httpx
from typing import List, Dict

class ArxivService:
    BASE_URL = "https://export.arxiv.org/api/query"

    async def search_papers(
        self,
        query: str,
        max_results: int = 10
    ) -> List[Dict]:
        """Search arXiv by topic/author"""
        params = {
            "search_query": query,
            "start": 0,
            "max_results": max_results,
            "sortBy": "submittedDate",
            "sortOrder": "descending"
        }

        async with httpx.AsyncClient() as client:
            response = await client.get(self.BASE_URL, params=params)
            response.raise_for_status()
            return self._parse_xml_response(response.text)

    async def get_paper(
        self,
        paper_id: str
    ) -> Dict:
        """Get paper metadata and PDF"""
        params = {
            "id_list": paper_id,
            "max_results": 1
        }

        async with httpx.AsyncClient() as client:
            response = await client.get(self.BASE_URL, params=params)
            response.raise_for_status()
            papers = self._parse_xml_response(response.text)

            if papers:
                paper = papers[0]
                # Extract text from PDF
                pdf_url = paper["pdf_url"]
                pdf_text = await self._extract_pdf_text(pdf_url)
                paper["content"] = pdf_text

                return paper
            else:
                raise Exception(f"Paper not found: {paper_id}")

    def _parse_xml_response(self, xml: str) -> List[Dict]:
        """Parse arXiv XML response"""
        import xml.etree.ElementTree as ET
        root = ET.fromstring(xml)

        papers = []
        for entry in root.findall("{http://www.w3.org/2005/Atom}entry"):
            paper = {
                "id": entry.findtext("{http://www.w3.org/2005/Atom}id").split("/")[-1],
                "title": entry.findtext("{http://www.w3.org/2005/Atom}title"),
                "authors": [
                    author.findtext("{http://www.w3.org/2005/Atom}name")
                    for author in entry.findall("{http://www.w3.org/2005/Atom}author")
                ],
                "summary": entry.findtext("{http://www.w3.org/2005/Atom}summary"),
                "published": entry.findtext("{http://www.w3.org/2005/Atom}published"),
                "categories": [
                    cat.attrib["term"]
                    for cat in entry.findall("{http://www.w3.org/2005/Atom}category")
                ],
                "pdf_url": next(
                    link.get("href")
                    for link in entry.findall("{http://www.w3.org/2005/Atom}link")
                    if link.attrib.get("type") == "application/pdf"
                )
            }
            papers.append(paper)

        return papers

    async def _extract_pdf_text(self, pdf_url: str) -> str:
        """Extract text from arXiv PDF"""
        import PyPDF2

        async with httpx.AsyncClient() as client:
            response = await client.get(pdf_url)
            response.raise_for_status()
            pdf_file = BytesIO(response.content)

            reader = PyPDF2.PdfReader(pdf_file)
            text = ""
            for page in reader.pages:
                text += page.extract_text() + "\n"

            return text
```

**API Endpoints:**

```python
# backend/app/api/arxiv.py

from fastapi import APIRouter
from pydantic import BaseModel

router = APIRouter(prefix="/api/arxiv", tags=["arxiv"])

class ArxivSearchRequest(BaseModel):
    query: str
    max_results: int = 10

class ArxivPaperRequest(BaseModel):
    paper_id: str  # e.g., "2301.00774"

@router.post("/search")
async def search_arxiv(request: ArxivSearchRequest):
    """Search arXiv papers by query"""
    service = ArxivService()
    papers = await service.search_papers(
        query=request.query,
        max_results=request.max_results
    )
    return {"papers": papers}

@router.post("/paper")
async def get_arxiv_paper(request: ArxivPaperRequest):
    """Get specific arXiv paper with PDF content"""
    service = ArxivService()
    paper = await service.get_paper(request.paper_id)
    return paper
```

**Frontend Integration:**

```typescript
// frontend/src/services/arxivService.ts

export interface ArxivPaper {
  id: string;
  title: string;
  authors: string[];
  summary: string;
  published: string;
  categories: string[];
  pdf_url: string;
  content?: string;
}

export async function searchArxiv(query: string, maxResults: number = 10): Promise<ArxivPaper[]> {
  const response = await fetch('/api/arxiv/search', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query, max_results: maxResults })
  });
  return response.json();
}

export async function getArxivPaper(paperId: string): Promise<ArxivPaper> {
  const response = await fetch('/api/arxiv/paper', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ paper_id: paperId })
  });
  return response.json();
}
```

### 2.2 Web URL Content Extraction

**Service Implementation:**

```python
# backend/app/services/url_service.py

import httpx
from bs4 import BeautifulSoup
from readability import Document
from typing import Dict

class URLService:
    async def extract_content(self, url: str) -> Dict:
        """Extract clean text from URL"""
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.get(url, follow_redirects=True)
            response.raise_for_status()

            # Try readability first (for articles)
            try:
                doc = Document(response.text)
                title = doc.title()
                content = doc.summary()
            except:
                # Fallback to BeautifulSoup
                soup = BeautifulSoup(response.text, 'html.parser')
                title = soup.find('title').get_text()
                # Remove scripts and styles
                for script in soup(['script', 'style']):
                    script.decompose()
                content = soup.get_text(separator='\n', strip=True)

            return {
                "url": url,
                "title": title,
                "content": content[:50000],  # Limit size
                "type": "url"
            }

    async def is_valid_url(self, url: str) -> bool:
        """Validate URL format"""
        try:
            result = urlparse(url)
            return all([result.scheme, result.netloc])
        except:
            return False
```

### 2.3 Markdown File Support

**Implementation:**

```typescript
// frontend/src/components/MarkdownInput.tsx

import React, { useRef } from 'react';

export function MarkdownInput({ onFileLoad }: { onFileLoad: (content: string) => void }) {
  const inputRef = useRef<HTMLInputElement>(null);

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.name.endsWith('.md')) {
      alert('Only markdown files are supported.');
      return;
    }

    const content = await file.text();
    onFileLoad(content);
  };

  return (
    <div className="border-2 border-dashed rounded-xl p-6">
      <input
        ref={inputRef}
        type="file"
        accept=".md"
        onChange={handleFileChange}
        className="hidden"
      />
      <button
        onClick={() => inputRef.current?.click()}
        className="w-full py-3 bg-blue-600 text-white rounded-xl font-bold"
      >
        📄 Upload Markdown File
      </button>
    </div>
  );
}
```

---

## Phase 3: Brainstorming & Ideation Tools (Weeks 6-8)

**Priority:** P2 (Medium)
**Effort:** 2-3 weeks
**Key Deliverables:** React Flow canvas, AI-assisted ideation

### 3.1 React Flow Canvas Implementation

**Based on research patterns:**
- **nodeOrigin: [0.5, 0.5]** - Centered expansion
- **Floating Edges** - Organic connections
- **Zustand Store** - Centralized state management

**Data Models:**

```typescript
// frontend/src/types/brainstorm.ts

export type NodeType = 'problem' | 'solution' | 'question' | 'reference' | 'constraint';

export interface IdeaNode {
  id: string;
  type: NodeType;
  position: { x: number; y: number };
  data: {
    label: string;
    content: string;
    aiGenerated: boolean;
    source?: string;
    confidence?: number;
    createdAt: Date;
  };
}

export interface IdeaEdge {
  id: string;
  source: string;
  target: string;
  type: 'default' | 'supports' | 'contradicts' | 'related';
  label?: string;
}

export interface BrainstormSession {
  id: string;
  title: string;
  nodes: IdeaNode[];
  edges: IdeaEdge[];
  viewport?: { x: number; y: number; zoom: number };
  createdAt: Date;
  updatedAt: Date;
}
```

**Zustand Store (Slice Pattern):**

```typescript
// frontend/src/stores/brainstormStore.ts

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { IdeaNode, IdeaEdge, NodeType, BrainstormSession } from '../types/brainstorm';
import { StateCreator } from 'zustand';

// Auth Slice (for session management)
interface AuthSlice {
  userId: string | null;
  setUserId: (id: string) => void;
}

const createAuthSlice: StateCreator<BrainstormStore, [], [], AuthSlice> = (set) => ({
  userId: null,
  setUserId: (id) => set({ userId: id })
});

// Session Slice
interface SessionSlice {
  sessions: BrainstormSession[];
  currentSessionId: string | null;
  createSession: (title: string) => void;
  loadSession: (id: string) => void;
  deleteSession: (id: string) => void;
}

const createSessionSlice: StateCreator<BrainstormStore, [], [], SessionSlice> = (set, get) => ({
  sessions: [],
  currentSessionId: null,

  createSession: (title) => {
    const newSession: BrainstormSession = {
      id: crypto.randomUUID(),
      title,
      nodes: [],
      edges: [],
      createdAt: new Date(),
      updatedAt: new Date()
    };
    set({ sessions: [...get().sessions, newSession], currentSessionId: newSession.id });
  },

  loadSession: (id) => {
    const session = get().sessions.find(s => s.id === id);
    if (session) {
      set({ currentSessionId: id });
    }
  },

  deleteSession: (id) => {
    set({
      sessions: get().sessions.filter(s => s.id !== id),
      currentSessionId: get().currentSessionId === id ? null : get().currentSessionId
    });
  }
});

// Node Slice
interface NodeSlice {
  nodes: IdeaNode[];
  edges: IdeaEdge[];
  addNode: (node: IdeaNode) => void;
  updateNode: (id: string, data: Partial<IdeaNode['data']>) => void;
  deleteNode: (id: string) => void;
  addEdge: (edge: IdeaEdge) => void;
  deleteEdge: (id: string) => void;
  setNodes: (nodes: IdeaNode[]) => void;
  setEdges: (edges: IdeaEdge[]) => void;
}

const createNodeSlice: StateCreator<BrainstormStore, [], [], NodeSlice> = (set, get) => ({
  nodes: [],
  edges: [],

  addNode: (node) => {
    set({ nodes: [...get().nodes, node] });
  },

  updateNode: (id, data) => {
    set({
      nodes: get().nodes.map(node =>
        node.id === id ? { ...node, data: { ...node.data, ...data } } : node
      )
    });
  },

  deleteNode: (id) => {
    const connectedEdgeIds = get().edges
      .filter(e => e.source === id || e.target === id)
      .map(e => e.id);

    set({
      nodes: get().nodes.filter(n => n.id !== id),
      edges: get().edges.filter(e => !connectedEdgeIds.includes(e.id))
    });
  },

  addEdge: (edge) => {
    set({ edges: [...get().edges, edge] });
  },

  deleteEdge: (id) => {
    set({ edges: get().edges.filter(e => e.id !== id) });
  },

  setNodes: (nodes) => set({ nodes }),
  setEdges: (edges) => set({ edges })
});

// Combined Store
export type BrainstormStore = AuthSlice & SessionSlice & NodeSlice;

export const useBrainstormStore = create<BrainstormStore>()(
  persist(
    (...args) => ({
      ...createAuthSlice(...args),
      ...createSessionSlice(...args),
      ...createNodeSlice(...args)
    }),
    {
      name: 'brainstorm-storage',
      partialize: (state) => ({
        sessions: state.sessions,
        currentSessionId: state.currentSessionId
      })
    }
  )
);

// Selectors with shallow
import { shallow } from 'zustand/shallow';

export const useBrainstormNodes = () =>
  useBrainstormStore(
    shallow(state => ({ nodes: state.nodes, edges: state.edges }))
  );
```

**Custom Node Components:**

```typescript
// frontend/src/components/brainstorm/ProblemNode.tsx

import React from 'react';
import { Handle, Position, NodeProps } from 'reactflow';
import { useBrainstormStore } from '../../stores/brainstormStore';

export function ProblemNode({ data, selected }: NodeProps) {
  const updateNode = useBrainstormStore(state => state.updateNode);

  return (
    <div className={`p-4 rounded-xl border-2 ${selected ? 'border-red-500' : 'border-red-300'} bg-red-50 dark:bg-red-900/20`}>
      <Handle type="target" position={Position.Top} />
      <div className="flex items-center gap-2 mb-2">
        <span className="text-lg">⚠️</span>
        <input
          value={data.label}
          onChange={(e) => updateNode(data.id, { label: e.target.value })}
          className="font-bold bg-transparent border-none outline-none w-full text-sm"
        />
      </div>
      <textarea
        value={data.content}
        onChange={(e) => updateNode(data.id, { content: e.target.value })}
        className="w-full h-20 bg-transparent border-none outline-none text-xs resize-none"
        placeholder="Describe the problem..."
      />
      {data.aiGenerated && (
        <div className="text-[10px] text-purple-500 mt-1 flex items-center gap-1">
          ✨ AI Generated • Confidence: {data.confidence}%
        </div>
      )}
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}

// frontend/src/components/brainstorm/SolutionNode.tsx

export function SolutionNode({ data, selected }: NodeProps) {
  // Similar structure with green theme
}

// frontend/src/components/brainstorm/QuestionNode.tsx

export function QuestionNode({ data, selected }: NodeProps) {
  // Similar structure with blue theme
}

// frontend/src/components/brainstorm/ReferenceNode.tsx

export function ReferenceNode({ data, selected }: NodeProps) {
  // Similar structure with purple theme, smaller size
}
```

**Canvas Component:**

```typescript
// frontend/src/components/brainstorm/BrainstormCanvas.tsx

import React, { useCallback, useMemo } from 'react';
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  useReactFlow,
  NodeTypes,
  EdgeTypes,
  Connection,
  Edge,
  addEdge,
  ConnectionLineComponent
} from 'reactflow';
import 'reactflow/dist/style.css';
import { useBrainstormStore } from '../../stores/brainstormStore';
import { ProblemNode, SolutionNode, QuestionNode, ReferenceNode } from './nodes';

const nodeTypes: NodeTypes = {
  problem: ProblemNode,
  solution: SolutionNode,
  question: QuestionNode,
  reference: ReferenceNode
};

export function BrainstormCanvas() {
  const { nodes, edges, addNode, addEdge, setNodes, setEdges } = useBrainstormStore();
  const { toObject } = useReactFlow();

  const onConnect = useCallback(
    (params: Edge | Connection) => {
      const newEdge = {
        ...params,
        id: crypto.randomUUID(),
        type: 'default'
      } as IdeaEdge;
      addEdge(newEdge);
    },
    [addEdge]
  );

  const onNodeChange = useCallback(
    (changes: any) => {
      setNodes(applyNodeChanges(changes, nodes));
    },
    [nodes, setNodes]
  );

  const onEdgeChange = useCallback(
    (changes: any) => {
      setEdges(applyEdgeChanges(changes, edges));
    },
    [edges, setEdges]
  );

  const onDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault();

      const nodeType = event.dataTransfer.getData('application/reactflow');
      const position = reactFlowInstance.project({
        x: event.clientX,
        y: event.clientY
      });

      const newNode: IdeaNode = {
        id: crypto.randomUUID(),
        type: nodeType,
        position,
        data: {
          label: `New ${nodeType}`,
          content: '',
          aiGenerated: false,
          createdAt: new Date()
        }
      };

      addNode(newNode);
    },
    [addNode]
  );

  const onDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
  }, []);

  return (
    <div className="h-screen w-full">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onConnect={onConnect}
        onNodesChange={onNodeChange}
        onEdgesChange={onEdgeChange}
        onDrop={onDrop}
        onDragOver={onDragOver}
        nodeOrigin={[0.5, 0.5]}
        fitView
      >
        <Background />
        <Controls />
        <MiniMap />
      </ReactFlow>

      {/* Sidebar with node palette */}
      <div className="absolute top-20 left-4 bg-white dark:bg-slate-900 p-4 rounded-xl shadow-lg border">
        <h3 className="font-bold mb-2">Add Idea</h3>
        <div className="space-y-2">
          {(['problem', 'solution', 'question', 'reference'] as NodeType[]).map(type => (
            <div
              key={type}
              draggable
              onDragStart={(e) => e.dataTransfer.setData('application/reactflow', type)}
              className={`p-2 rounded-lg cursor-move ${
                type === 'problem' ? 'bg-red-100' :
                type === 'solution' ? 'bg-green-100' :
                type === 'question' ? 'bg-blue-100' : 'bg-purple-100'
              }`}
            >
              {type}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
```

### 3.2 AI-Assisted Ideation

**Service Integration:**

```python
# backend/app/services/ideation_service.py

class IdeationService:
    def __init__(self, ai_router: CortexRouter):
        self.router = ai_router

    async def expand_idea(
        self,
        topic: str,
        constraints: list[str],
        idea_type: str = "solution"
    ) -> list[dict]:
        """Generate related ideas using AI"""
        prompt = f"""
        Given the following topic and constraints, generate 3-5 related {idea_type} ideas:

        Topic: {topic}
        Constraints: {', '.join(constraints)}

        For each idea, provide:
        - title (short, catchy)
        - description (1-2 sentences)
        - confidence_score (0-100)
        - potential_challenges (list)

        Return as JSON array.
        """

        result = await self.router.analyze_with_fallback(
            codebase=[],  # No codebase needed for pure ideation
            input_data={"type": "text", "content": prompt},
            system_doc=None,
            user_intent="fast"
        )

        return self._parse_ideas(result)

    async def connect_ideas(
        self,
        idea_a: str,
        idea_b: str
    ) -> dict:
        """Analyze relationship between two ideas"""
        prompt = f"""
        Analyze the relationship between these two ideas:

        Idea A: {idea_a}
        Idea B: {idea_b}

        Determine the relationship type:
        - supports: Idea A supports Idea B
        - contradicts: Ideas are in conflict
        - related: Ideas are connected but not dependent

        Provide reasoning (1-2 sentences).
        """

        result = await self.router.analyze_with_fallback(
            codebase=[],
            input_data={"type": "text", "content": prompt},
            system_doc=None,
            user_intent="fast"
        )

        return {
            "relationship": result["relationship_type"],
            "reasoning": result["reasoning"],
            "confidence": result["confidence"]
        }
```

**Frontend AI Ideation Hook:**

```typescript
// frontend/src/hooks/useAIideation.ts

import { useState } from 'react';

export function useAIideation() {
  const [isGenerating, setIsGenerating] = useState(false);

  const generateIdeas = async (
    topic: string,
    constraints: string[],
    type: 'problem' | 'solution' | 'question' | 'reference'
  ) => {
    setIsGenerating(true);
    try {
      const response = await fetch('/api/brainstorm/expand', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ topic, constraints, idea_type: type })
      });

      const ideas = await response.json();
      return ideas;
    } finally {
      setIsGenerating(false);
    }
  };

  const analyzeRelationship = async (ideaA: string, ideaB: string) => {
    setIsGenerating(true);
    try {
      const response = await fetch('/api/brainstorm/connect', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ idea_a: ideaA, idea_b: ideaB })
      });

      const relationship = await response.json();
      return relationship;
    } finally {
      setIsGenerating(false);
    }
  };

  return { isGenerating, generateIdeas, analyzeRelationship };
}
```

---

## Phase 4: Advanced CR Generation (Weeks 9-10)

**Priority:** P2 (Medium)
**Effort:** 1-2 weeks
**Key Deliverables:** CR templates, task breakdown, estimation

### 4.1 CR Template System

**Template Definitions:**

```typescript
// frontend/src/types/cr.ts

export interface CRTemplate {
  id: string;
  name: string;
  format: 'markdown' | 'jira' | 'linear' | 'github' | 'custom';
  description: string;
  sections: CRSection[];
}

export interface CRSection {
  id: string;
  title: string;
  required: boolean;
  placeholder?: string;
}

export interface DetailedCR {
  summary: string;
  type: 'feature' | 'refactor' | 'bugfix' | 'research';

  tasks: CRTask[];
  estimation: CREstimation;
  dependencies: CRDependency[];
  riskFactors: CRRisk[];
  testingRequirements: string[];
  documentationNeeds: string[];

  // Template formatting
  template_id: string;
  formatted_output: string;
}

export interface CRTask {
  id: string;
  title: string;
  description: string;
  acceptance_criteria: string[];
  estimate_hours?: number;
  priority: 'critical' | 'high' | 'medium' | 'low';
  status: 'pending' | 'in-progress' | 'completed';
}

export interface CREstimation {
  optimistic: { value: number; unit: string };
  expected: { value: number; unit: string };
  pessimistic: { value: number; unit: string };
  complexity: 1 | 2 | 3 | 5 | 8 | 13; // Fibonacci
}
```

**Built-in Templates:**

```typescript
// frontend/src/data/crTemplates.ts

export const CR_TEMPLATES: CRTemplate[] = [
  {
    id: 'claude-code',
    name: 'Claude Code Optimized',
    format: 'markdown',
    description: 'Optimized for AI-assisted implementation with Claude Code',
    sections: [
      { id: 'summary', title: 'Summary', required: true, placeholder: 'Brief description of changes' },
      { id: 'type', title: 'Type', required: true, placeholder: 'feature/refactor/bugfix' },
      { id: 'rationale', title: 'Rationale', required: true, placeholder: 'Why this change is needed' },
      { id: 'implementation', title: 'Implementation Plan', required: true, placeholder: 'Step-by-step plan' },
      { id: 'acceptance', title: 'Acceptance Criteria', required: true, placeholder: 'How to verify completion' },
      { id: 'files', title: 'Affected Files', required: false, placeholder: 'List of files to modify' }
    ]
  },
  {
    id: 'jira-epic',
    name: 'Jira Epic',
    format: 'jira',
    description: 'Full Jira-compatible structure with story points',
    sections: [
      { id: 'summary', title: 'Summary', required: true },
      { id: 'priority', title: 'Priority', required: true },
      { id: 'story_points', title: 'Story Points', required: true },
      { id: 'description', title: 'Description', required: true },
      { id: 'acceptance', title: 'Acceptance Criteria', required: true }
    ]
  },
  {
    id: 'github-issue',
    name: 'GitHub Issue',
    format: 'github',
    description: 'Standard GitHub issue format with labels',
    sections: [
      { id: 'title', title: 'Title', required: true },
      { id: 'body', title: 'Body', required: true },
      { id: 'labels', title: 'Labels', required: false },
      { id: 'milestone', title: 'Milestone', required: false }
    ]
  },
  {
    id: 'linear-ticket',
    name: 'Linear Ticket',
    format: 'linear',
    description: 'Linear-specific format with projects and cycles',
    sections: [
      { id: 'title', title: 'Title', required: true },
      { id: 'description', title: 'Description', required: true },
      { id: 'priority', title: 'Priority', required: true },
      { id: 'project', title: 'Project', required: false },
      { id: 'cycle', title: 'Cycle', required: false }
    ]
  },
  {
    id: 'technical-spec',
    name: 'Technical Spec',
    format: 'markdown',
    description: 'Detailed implementation document',
    sections: [
      { id: 'background', title: 'Background', required: true },
      { id: 'requirements', title: 'Requirements', required: true },
      { id: 'architecture', title: 'Architecture', required: true },
      { id: 'implementation', title: 'Implementation Details', required: true },
      { id: 'testing', title: 'Testing Strategy', required: true },
      { id: 'rollout', title: 'Rollout Plan', required: false }
    ]
  }
];
```

### 4.2 CR Breakdown & Estimation

**AI Service:**

```python
# backend/app/services/cr_service.py

class CRService:
    def __init__(self, ai_router: CortexRouter):
        self.router = ai_router

    async def breakdown_cr(
        self,
        analysis_result: dict
    ) -> DetailedCR:
        """Break down CR into detailed tasks with estimation"""
        prompt = f"""
        Given the following analysis, create a detailed Change Request breakdown:

        Executive Summary: {analysis_result['executiveSummary']}
        Suggested CR: {analysis_result['suggestedCR']}

        Break down into:
        1. Summary (1-2 sentences)
        2. Type (feature/refactor/bugfix/research)
        3. Tasks (5-10 items, each with title, description, acceptance criteria, priority)
        4. Estimation (optimistic, expected, pessimistic in hours/days; complexity 1-13 Fibonacci)
        5. Dependencies (what needs to be done first)
        6. Risk Factors (potential blockers)
        7. Testing Requirements (how to verify)
        8. Documentation Needs (what docs to update)

        Return as structured JSON.
        """

        result = await self.router.analyze_with_fallback(
            codebase=[],
            input_data={"type": "text", "content": prompt},
            system_doc=None,
            user_intent="smart"  # Use smart lane for complex breakdown
        )

        return self._parse_detailed_cr(result)
```

**Template Formatter:**

```typescript
// frontend/src/services/crFormatter.ts

export function formatCR(cr: DetailedCR, template: CRTemplate): string {
  switch (template.format) {
    case 'jira':
      return formatJira(cr, template);
    case 'github':
      return formatGitHub(cr, template);
    case 'linear':
      return formatLinear(cr, template);
    case 'markdown':
    default:
      return formatMarkdown(cr, template);
  }
}

function formatJira(cr: DetailedCR, template: CRTemplate): string {
  return `
h1. ${cr.summary}

*Type:* ${cr.type}
*Priority:* ${cr.tasks[0]?.priority}
*Story Points:* ${cr.estimation.complexity}

h2. Description
${cr.summary}

h2. Acceptance Criteria
${cr.tasks.map(t => `- [ ] ${t.title}`).join('\n')}

h2. Tasks
${cr.tasks.map((t, i) => `
*Task ${i + 1}: ${t.title}*
${t.description}
*Acceptance:*
${t.acceptance_criteria.map(ac => `  - [ ] ${ac}`).join('\n')}
`).join('\n')}
  `.trim();
}

function formatMarkdown(cr: DetailedCR, template: CRTemplate): string {
  return `
# ${cr.summary}

**Type:** ${cr.type} | **Complexity:** ${cr.estimation.complexity} (${cr.estimation.expected.value} ${cr.estimation.expected.unit})

## Summary
${cr.summary}

## Implementation Plan

${cr.tasks.map((t, i) => `
### ${i + 1}. ${t.title}

**Priority:** ${t.priority}

${t.description}

**Acceptance Criteria:**
${t.acceptance_criteria.map(ac => `- [ ] ${ac}`).join('\n')}
`).join('\n')}

## Dependencies
${cr.dependencies.map(d => `- [ ] ${d.description}`).join('\n') || 'None'}

## Risk Factors
${cr.riskFactors.map(r => `- ⚠️ **${r.title}:** ${r.description} (${r.mitigation})`).join('\n') || 'None'}

## Testing
${cr.testingRequirements.map(t => `- ${t}`).join('\n') || 'None'}
  `.trim();
}
```

### 4.3 Direct Issue Creation

**GitHub Integration:**

```python
# backend/app/services/github_integration.py

import httpx
from typing import Dict

class GitHubIntegration:
    def __init__(self, api_token: str):
        self.api_token = api_token
        self.base_url = "https://api.github.com"

    async def create_issue(
        self,
        owner: str,
        repo: str,
        title: str,
        body: str,
        labels: list[str] = None,
        milestone: int = None
    ) -> Dict:
        """Create GitHub issue"""
        headers = {
            "Authorization": f"token {self.api_token}",
            "Accept": "application/vnd.github.v3+json"
        }

        payload = {"title": title, "body": body}
        if labels:
            payload["labels"] = labels
        if milestone:
            payload["milestone"] = milestone

        async with httpx.AsyncClient() as client:
            response = await client.post(
                f"{self.base_url}/repos/{owner}/{repo}/issues",
                json=payload,
                headers=headers
            )
            response.raise_for_status()

            return response.json()

    async def get_issues(
        self,
        owner: str,
        repo: str,
        state: str = "open"
    ) -> list:
        """Get repository issues"""
        headers = {
            "Authorization": f"token {self.api_token}",
            "Accept": "application/vnd.github.v3+json"
        }

        async with httpx.AsyncClient() as client:
            response = await client.get(
                f"{self.base_url}/repos/{owner}/{repo}/issues",
                params={"state": state},
                headers=headers
            )
            response.raise_for_status()

            return response.json()
```

**API Endpoint:**

```python
# backend/app/api/integrations.py

router = APIRouter(prefix="/api/integrations", tags=["integrations"])

class CreateGitHubIssueRequest(BaseModel):
    owner: str
    repo: str
    title: str
    body: str
    labels: list[str] = None
    milestone: int = None

@router.post("/github/issues")
async def create_github_issue(request: CreateGitHubIssueRequest):
    """Create GitHub issue from CR"""
    # Get GitHub token from user settings (database)
    token = await get_user_github_token()

    github = GitHubIntegration(token)
    issue = await github.create_issue(
        owner=request.owner,
        repo=request.repo,
        title=request.title,
        body=request.body,
        labels=request.labels,
        milestone=request.milestone
    )

    return {
        "issue_url": issue["html_url"],
        "issue_number": issue["number"],
        "status": "created"
    }
```

---

## Phase 5: Session & History Management (Week 11)

**Priority:** P2 (Medium)
**Effort:** 1 week
**Key Deliverables:** Project workspaces, evaluation history, search

### 5.1 Project Workspaces

**Frontend Workspace Component:**

```typescript
// frontend/src/components/Workspace.tsx

import { useState, useEffect } from 'react';
import { useBrainstormStore } from '../stores/brainstormStore';

export function WorkspaceSelector() {
  const { sessions, currentSessionId, createSession, loadSession } = useBrainstormStore();
  const [isModalOpen, setIsModalOpen] = useState(false);

  return (
    <div className="p-4 border rounded-xl">
      <h2 className="font-bold mb-4">📁 Workspaces</h2>

      <div className="space-y-2">
        {sessions.map(session => (
          <div
            key={session.id}
            onClick={() => loadSession(session.id)}
            className={`p-3 rounded-lg cursor-pointer transition-colors ${
              currentSessionId === session.id
                ? 'bg-blue-500 text-white'
                : 'bg-slate-100 dark:bg-slate-800 hover:bg-blue-100 dark:hover:bg-slate-700'
            }`}
          >
            <div className="font-semibold">{session.title}</div>
            <div className="text-xs text-slate-500">
              {new Date(session.createdAt).toLocaleDateString()} • {session.nodes.length} ideas
            </div>
          </div>
        ))}
      </div>

      <button
        onClick={() => setIsModalOpen(true)}
        className="mt-4 w-full py-2 bg-blue-600 text-white rounded-lg font-bold"
      >
        + New Workspace
      </button>

      {isModalOpen && (
        <CreateWorkspaceModal
          onClose={() => setIsModalOpen(false)}
          onCreate={createSession}
        />
      )}
    </div>
  );
}
```

### 5.2 Evaluation History

**API Endpoints:**

```python
# backend/app/api/history.py

router = APIRouter(prefix="/api/history", tags=["history"])

@router.get("/evaluations")
async def get_evaluation_history(
    project_id: str = None,
    limit: int = 50,
    offset: int = 0
):
    """Get evaluation history with pagination"""
    # Database query with filters and pagination
    pass

@router.get("/evaluations/{evaluation_id}")
async def get_evaluation_detail(evaluation_id: str):
    """Get specific evaluation with full context"""
    # Database fetch
    pass

@router.get("/stats")
async def get_evaluation_stats(project_id: str = None):
    """Get evaluation statistics for dashboard"""
    # Aggregation queries
    # - Total evaluations
    # - Average value score
    # - Provider usage distribution
    # - Evaluation types distribution
    pass
```

### 5.3 Search & Discovery

**API Endpoint:**

```python
@router.get("/search")
async def search_evaluations(
    query: str,
    semantic: bool = True,
    filters: dict = None,
    limit: int = 10
):
    """Search evaluations (full-text + semantic)"""

    if semantic:
        # Use vector DB for semantic search
        results = await vector_store.search_similar_evaluations(
            query=query,
            n_results=limit,
            filters=filters
        )
    else:
        # Use database full-text search
        # PostgreSQL: tsvector
        # SQLite: FTS5
        pass

    return {"results": results}
```

---

## Phase 6: CLI Tool & IDE Extensions (Weeks 12-13)

**Priority:** P3 (Low)
**Effort:** 2 weeks
**Key Deliverables:** CLI tool, VSCode extension

### 6.1 CLI Tool

**Project Structure:**

```
cortex-eval-cli/
├── src/
│   ├── commands/
│   │   ├── init.ts
│   │   ├── analyze.ts
│   │   ├── paper.ts
│   │   └── push.ts
│   ├── services/
│   │   ├── api.ts
│   │   └── config.ts
│   └── index.ts
├── package.json
└── README.md
```

**CLI Implementation:**

```typescript
// cortex-eval-cli/src/commands/init.ts

import { Command } from 'commander';

export const initCommand = new Command('init')
  .description('Initialize Cortex Evaluator in current project')
  .option('-n, --name <name>', 'Project name')
  .action(async (options) => {
    const projectName = options.name || process.cwd().split('/').pop();

    // Create config file
    await fs.writeFile(
      '.cortex-eval.json',
      JSON.stringify({
        projectId: crypto.randomUUID(),
        name: projectName,
        codebaseType: 'local',
        createdAt: new Date().toISOString()
      }, null, 2)
    );

    console.log(`✅ Initialized Cortex Evaluator for project: ${projectName}`);
    console.log(`📄 Configuration saved to: .cortex-eval.json`);
  });

// cortex-eval-cli/src/commands/analyze.ts

export const analyzeCommand = new Command('analyze')
  .description('Analyze codebase against input')
  .argument('<input>', 'Input to analyze (arxiv URL, paper ID, or text)')
  .option('-p, --provider <provider>', 'AI provider (default: auto)')
  .option('-t, --template <template>', 'CR template (default: claude-code)')
  .action(async (input, options) => {
    // Call backend API
    const response = await fetch('/api/evaluations/analyze', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        input_type: 'text',
        input_content: input,
        provider_preference: options.provider
      })
    });

    const result = await response.json();

    // Display results
    console.log(`\n📊 Value Score: ${result.value_score}/100`);
    console.log(`\n📝 Executive Summary:`);
    console.log(result.executive_summary);
    console.log(`\n🔧 Suggested CR:`);
    console.log(result.suggested_cr);

    // Save to file
    const filename = `cr-${Date.now()}.md`;
    await fs.writeFile(filename, result.suggested_cr);
    console.log(`\n💾 CR saved to: ${filename}`);
  });

// cortex-eval-cli/src/commands/paper.ts

export const paperCommand = new Command('paper')
  .description('Analyze arXiv paper against codebase')
  .argument('<paper_id>', 'arXiv paper ID (e.g., 2301.00774)')
  .action(async (paperId) => {
    // Fetch paper from arXiv
    const paper = await fetchArxivPaper(paperId);

    // Analyze against codebase
    const response = await fetch('/api/evaluations/analyze', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        input_type: 'arxiv',
        input_content: paper.content,
        file_data: {
          title: paper.title,
          authors: paper.authors,
          paper_id: paper.id
        }
      })
    });

    const result = await response.json();
    console.log(`\n📄 Analyzed Paper: ${paper.title}`);
    console.log(`📊 Value Score: ${result.value_score}/100`);
    console.log(result.suggested_cr);
  });
```

### 6.2 VSCode Extension

**Project Structure:**

```
cortex-eval-vscode/
├── src/
│   ├── extension.ts
│   ├── commands/
│   │   ├── analyzeFile.ts
│   │   ├── analyzeSelection.ts
│   │   └── pushCR.ts
│   └── api/
│       └── client.ts
├── package.json
├── tsconfig.json
└── README.md
```

**Extension Implementation:**

```typescript
// cortex-eval-vscode/src/extension.ts

import * as vscode from 'vscode';

export function activate(context: vscode.ExtensionContext) {
  // Analyze current file
  const analyzeFileCommand = vscode.commands.registerCommand(
    'cortex-eval.analyzeFile',
    async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor) return;

      const document = editor.document;
      const content = document.getText();

      // Show progress
      await vscode.window.withProgress(
        { location: vscode.ProgressLocation.Notification, title: 'Analyzing file...' },
        async () => {
          const result = await api.analyze({
            type: 'snippet',
            content: content,
            name: document.fileName
          });

          // Show results in new editor
          const doc = await vscode.workspace.openTextDocument({
            content: result.suggested_cr,
            language: 'markdown'
          });
          await vscode.window.showTextDocument(doc);
        }
      );
    }
  );

  context.subscriptions.push(analyzeFileCommand);

  // Analyze selected text
  const analyzeSelectionCommand = vscode.commands.registerCommand(
    'cortex-eval.analyzeSelection',
    async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor) return;

      const selection = editor.selection;
      const content = editor.document.getText(selection);

      if (!content) {
        vscode.window.showWarningMessage('Please select some text first');
        return;
      }

      const result = await api.analyze({
        type: 'snippet',
        content: content,
        name: 'Selection'
      });

      // Show as quick pick
      const action = await vscode.window.showQuickPick([
        { label: 'Insert CR below selection', description: result.executive_summary },
        { label: 'Open in new file', description: 'Create separate CR document' }
      ]);

      if (action?.label.includes('Insert')) {
        await editor.edit(editBuilder => {
          editBuilder.insert(selection.end, `\n${result.suggested_cr}`);
        });
      } else {
        const doc = await vscode.workspace.openTextDocument({
          content: result.suggested_cr,
          language: 'markdown'
        });
        await vscode.window.showTextDocument(doc);
      }
    }
  );

  context.subscriptions.push(analyzeSelectionCommand);

  // Push to GitHub
  const pushCRCommand = vscode.commands.registerCommand(
    'cortex-eval.pushCR',
    async () => {
      // Get current editor content
      const editor = vscode.window.activeTextEditor;
      if (!editor) return;

      const cr = editor.document.getText();

      // Parse repo from workspace
      const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
      if (!workspaceFolder) {
        vscode.window.showErrorMessage('No workspace folder open');
        return;
      }

      // Create issue
      const issueUrl = await api.pushToGitHub({
        repo: workspaceFolder.name,
        title: `CR: ${cr.split('\n')[0].substring(0, 50)}`,
        body: cr
      });

      // Open in browser
      await vscode.env.openExternal(vscode.Uri.parse(issueUrl));
    }
  );

  context.subscriptions.push(pushCRCommand);
}

export function deactivate() {}
```

---

## Development Workflow

### Environment Setup

**1. Clone and Setup:**

```bash
# Clone repository
git clone <repo-url>
cd cortex-evaluator

# Backend setup
cd backend
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
pip install -r requirements.txt

# Frontend setup
cd ../frontend
npm install

# Environment
cp .env.example .env
# Edit .env with your API keys
```

**2. Run Development:**

```bash
# Terminal 1: Backend
cd backend
uvicorn app.main:app --reload --port 8000

# Terminal 2: Frontend
cd frontend
npm run dev
```

**3. Docker (Optional):**

```yaml
# docker-compose.yml
version: '3.8'

services:
  backend:
    build: ./backend
    ports:
      - "8000:8000"
    environment:
      - DATABASE_URL=postgresql://user:pass@db:5432/cortex
      - REDIS_URL=redis://redis:6379
    depends_on:
      - db
      - redis

  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
    depends_on:
      - backend

  db:
    image: postgres:15
    volumes:
      - postgres_data:/var/lib/postgresql/data
    environment:
      - POSTGRES_DB=cortex
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass

  redis:
    image: redis:7
    volumes:
      - redis_data:/data

  chromadb:
    image: chromadb/chroma:latest
    ports:
      - "8001:8000"
    volumes:
      - chroma_data:/chroma/chroma

volumes:
  postgres_data:
  redis_data:
  chroma_data:
```

### Testing

**Backend Tests:**

```python
# tests/test_ai_router.py

import pytest
from app.services.ai_router import CortexRouter, OllamaProvider

@pytest.fixture
def router():
    return CortexRouter({})

async def test_fast_lane_routing(router):
    provider, lane = await router.route_analysis(
        codebase=[],
        input_data={"type": "text"},
        system_doc={},
        user_intent="cheap"
    )
    assert lane == RoutingLane.FAST

async def test_vision_requirement(router):
    provider, lane = await router.route_analysis(
        codebase=[],
        input_data={"type": "pdf", "vision": True},
        system_doc={},
        user_intent=None
    )
    assert lane == RoutingLane.SMART
```

**Frontend Tests:**

```typescript
// frontend/src/components/__tests__/BrainstormCanvas.test.tsx

import { render, screen } from '@testing-library/react';
import { BrainstormCanvas } from '../BrainstormCanvas';

describe('BrainstormCanvas', () => {
  it('renders canvas with nodes', () => {
    render(<BrainstormCanvas />);
    expect(screen.getByText('Add Idea')).toBeInTheDocument();
  });

  it('creates node on drop', () => {
    render(<BrainstormCanvas />);
    // Simulate drag and drop
    // Verify node creation
  });
});
```

---

## Deployment Strategy

### Development

**Local Development:**
- Backend: FastAPI with `--reload` flag
- Frontend: Vite dev server
- Database: SQLite (easy to reset)

### Staging

**Docker Compose:**
- Full stack in containers
- PostgreSQL for production-like testing
- ChromaDB for vector search
- Nginx reverse proxy

### Production

**Recommended Stack:**
- **Backend:** FastAPI + Uvicorn (Gunicorn + Uvicorn workers)
- **Frontend:** Vite build → Nginx static files
- **Database:** PostgreSQL (managed: RDS, Cloud SQL)
- **Vector DB:** Qdrant (self-hosted or cloud)
- **Cache:** Redis (managed: ElastiCache, Redis Cloud)
- **Reverse Proxy:** Nginx/Caddy
- **SSL:** Let's Encrypt (Certbot)

**Deployment Options:**
- **Fly.io:** Simple deployment, managed PostgreSQL
- **Railway:** Quick setup, auto-scaling
- **DigitalOcean App Platform:** Full control
- **Self-hosted:** VPS + Docker Compose

---

## Migration Guide: Frontend-Only to Full-Stack

### Step 1: Restructure Project

```bash
# Move existing frontend to subdirectory
mkdir frontend
mv *.tsx *.ts *.json components services types frontend/
mkdir backend
mkdir shared
```

### Step 2: Implement Backend

1. Set up FastAPI project structure
2. Define database models (SQLModel/SQLAlchemy)
3. Implement multi-provider AI router
4. Set up ChromaDB integration
5. Create API endpoints

### Step 3: Integrate Frontend with Backend

1. Replace direct Gemini calls with API calls
2. Implement WebSocket for real-time updates
3. Add Zustand stores with persistence
4. Replace local state with API state

### Step 4: Add Persistence

1. Implement database migrations
2. Add user authentication (if multi-user)
3. Set up session management
4. Add search and filtering

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|-------|---------|-------------|
| **Provider Rate Limits** | High | Circuit breaker, multiple providers, Redis caching |
| **Vector DB Scaling** | Medium | Start with ChromaDB, migrate to Qdrant when >1M vectors |
| **Complex State Management** | Medium | Zustand slice pattern, shallow selectors |
| **API Key Security** | High | Environment variables, never commit keys, rotation strategy |
| **Database Performance** | Medium | Indexing, query optimization, connection pooling |
| **Testing Coverage** | Medium | Unit tests for critical paths, integration tests for API |

---

## Success Metrics

### Phase 1 Success Criteria
- [ ] Backend API running with 90%+ uptime
- [ ] At least 3 AI providers operational (Gemini, Claude, OpenAI)
- [ ] Circuit breaker preventing cascading failures
- [ ] Vector DB indexing < 10K files in < 5 minutes
- [ ] Evaluation storage and retrieval working

### Phase 2 Success Criteria
- [ ] arXiv paper search and retrieval working
- [ ] URL content extraction functional
- [ ] Markdown file upload supported
- [ ] All input types stored in database

### Phase 3 Success Criteria
- [ ] Brainstorming canvas with React Flow
- [ ] AI-assisted idea generation
- [ ] Workspace persistence (save/load)
- [ ] Canvas state serialization

### Phase 4 Success Criteria
- [ ] CR templates for 5+ platforms
- [ ] Task breakdown with estimation
- [ ] Direct GitHub issue creation
- [ ] CR export in multiple formats

### Phase 5 Success Criteria
- [ ] Project workspaces functional
- [ ] Evaluation history with search
- [ ] Semantic similarity search
- [ ] Evaluation statistics dashboard

### Phase 6 Success Criteria
- [ ] CLI tool with 5+ commands
- [ ] VSCode extension with 3+ commands
- [ ] Git hooks for pre-commit validation
- [ ] Documentation for CLI and extension

---

## Appendix A: Technology Decision Log

| Decision | Options Chosen | Rationale |
|-----------|-----------------|------------|
| **Backend Framework** | FastAPI (Python) | Async support, automatic docs, CortexBrain ecosystem alignment |
| **Database** | SQLite → PostgreSQL | Easy dev, scale with PostgreSQL later |
| **Vector DB** | ChromaDB → Qdrant | Rapid dev start, production scale with Qdrant |
| **State Management** | Zustand | Lightweight, excellent TypeScript support, slice pattern |
| **Brainstorming UI** | React Flow | Powerful, production-tested, organic connections |
| **AI Provider Pattern** | OpenAI-compatible proxy | Zero-code migration, provider-agnostic |

---

## Appendix B: Key Research Sources

Based on research conducted:

1. **React Flow Patterns**
   - Official tutorials: https://reactflow.dev/learn/tutorials/mind-map-app-with-react-flow
   - Source code: https://github.com/xyflow/react-flow-mindmap-app

2. **Zustand Best Practices**
   - Slice pattern: https://github.com/pmndrs/zustand/blob/main/docs/guides/slices-pattern.md
   - TypeScript: https://github.com/pmndrs/zustand/blob/main/docs/guides/advanced-typescript.md

3. **Vector DB Integration**
   - ChromaDB: https://github.com/chroma-core/chroma
   - Qdrant: https://qdrant.tech/documentation/

4. **Multi-Provider Routing**
   - Portkey Gateway: https://github.com/Portkey-AI/gateway
   - NVIDIA LLM Router: https://github.com/NVIDIA-AI-Blueprints/llm-router

---

## Next Steps

1. **Week 1:** Set up backend infrastructure, database schema, basic API
2. **Week 2:** Implement multi-provider AI router with circuit breaker
3. **Week 3:** Integrate ChromaDB, implement evaluation API endpoints
4. **Week 4:** Add arXiv, URL, and markdown input sources
5. **Week 5:** Connect frontend to backend API, replace direct Gemini calls
6. **Week 6-8:** Implement brainstorming canvas with React Flow and Zustand
7. **Week 9-10:** Build CR template system and task breakdown
8. **Week 11:** Add workspace and history management
9. **Week 12-13:** Develop CLI tool and VSCode extension
10. **Week 14-15:** Testing, bug fixes, documentation
11. **Week 16-17:** Deployment, monitoring, feedback collection

---

**Document Version:** 1.0
**Last Updated:** January 2026
**Maintained By:** Cortex Evaluator Team
