---
project: Cortex
component: Unknown
phase: Ideation
date_created: 2026-01-16T21:14:36
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:06.734283
---

# API Documentation

The Cortex Evaluator API is a RESTful interface built with FastAPI. It provides endpoints for codebase analysis, research paper integration, brainstorming session management, and evaluation history.

## 🔑 Authentication & Base URL

- **Base URL**: `http://localhost:8000` (Development)
- **Authentication**: Currently uses API Keys for AI providers. User authentication is managed via session persistence.
- **Content Type**: `application/json`

---

## 📂 Evaluations API

Handles AI-powered codebase analysis and Change Request (CR) generation.

### Analyze Evaluation
`POST /api/evaluations/analyze`

Runs an evaluation against a codebase using a selected input (arXiv, PDF, Snippet, etc.).

**Request Body**:
```json
{
  "codebase_id": "uuid-string",
  "input_type": "arxiv",
  "input_content": "Semantic search in vector databases...",
  "file_data": {
    "title": "Attention is All You Need",
    "paper_id": "1706.03762"
  },
  "provider_preference": "openai",
  "user_intent": "smart"
}
```

**Response**:
```json
{
  "id": "uuid-string",
  "value_score": 85,
  "executive_summary": "Implementing self-attention mechanism...",
  "technical_feasibility": "High, compatible with current stack",
  "gap_analysis": "Need to add transformer blocks...",
  "suggested_cr": "# CR: Add Transformer Support\n...",
  "provider_used": "gpt-4o",
  "similar_evaluations": []
}
```

---

## 💻 Codebases API

Manages repository indexing and context extraction.

### Initialize Codebase
`POST /api/codebases/initialize`

Imports a codebase from GitHub or local files.

**Request Body**:
```json
{
  "type": "github",
  "githubUrl": "https://github.com/openai/openai-python"
}
```

### Generate System Documentation
`POST /api/codebases/{codebase_id}/generate-docs`

Generates high-level technical documentation for the indexed codebase.

---

## 🧠 Brainstorming API

Manages interactive ideation sessions.

### Create Session
`POST /api/sessions/`

**Request Body**:
```json
{
  "projectId": "uuid-string",
  "title": "Q3 Roadmap Planning"
}
```

### Expand Idea
`POST /api/brainstorm/expand`

Generates AI-powered expansions for a specific idea node.

**Request Body**:
```json
{
  "idea": "Microservices architecture",
  "context": "Scalability requirements for 1M users"
}
```

---

## 📄 arXiv API

Integration with academic research papers.

### Search Papers
`POST /api/arxiv/search`

**Request Body**:
```json
{
  "query": "Large Language Models",
  "maxResults": 10,
  "categories": ["cs.CL", "cs.AI"]
}
```

### Get Paper Content
`POST /api/arxiv/paper`

**Request Body**:
```json
{
  "paperId": "2301.00774"
}
```

---

## 📊 History & Analytics API

Retrieves past evaluations and platform statistics.

### Get Evaluation Stats
`GET /api/history/stats`

**Response**:
```json
{
  "totalEvaluations": 142,
  "avgValueScore": 78.5,
  "providerUsage": {
    "openai": 45,
    "anthropic": 32,
    "gemini": 65
  }
}
```

---

## ⚠️ Error Codes

Cortex Evaluator uses standard HTTP status codes:

| Code | Description |
|------|-------------|
| `200` | Success |
| `400` | Bad Request (Invalid parameters) |
| `401` | Unauthorized (Invalid API Key) |
| `404` | Not Found (Resource does not exist) |
| `500` | Internal Server Error (Provider failure, database error) |
| `503` | Service Unavailable (Circuit breaker open) |

**Error Response Body**:
```json
{
  "message": "Detailed error message",
  "status": 404,
  "code": "RESOURCE_NOT_FOUND"
}
```
