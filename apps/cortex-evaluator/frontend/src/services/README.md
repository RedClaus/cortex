---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-01-16T21:02:32
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:11.611666
---

# Cortex Evaluator API Integration

This directory contains the complete frontend API integration layer for Cortex Evaluator.

## Overview

The frontend now communicates with the backend through a centralized API client instead of making direct calls to external services (like Google Gemini SDK or GitHub API). All requests are routed through the FastAPI backend which handles:

- AI provider routing (Gemini, OpenAI, Anthropic, Groq, Ollama)
- Authentication and authorization
- Request/response validation
- Vector database operations
- Database persistence

## Directory Structure

```
frontend/src/
├── services/
│   └── api.ts                    # Centralized API client
├── hooks/
│   ├── useCodebase.ts             # Codebase management hook
│   ├── useAnalysis.ts             # Evaluation analysis hook
│   ├── useArxivSearch.ts          # arXiv paper search hook
│   ├── useEvaluationHistory.ts      # History & analytics hook
│   ├── useBrainstorm.ts           # Brainstorm session hook
│   └── index.ts                  # Hooks exports
└── types/
    └── api.ts                      # TypeScript type definitions
```

## Usage

### Configure API Client

```typescript
import { apiClient } from './services/api'

// Optional: Configure custom base URL or auth headers
apiClient.configure('https://api.example.com', {
  'Authorization': 'Bearer token123'
})
```

### Using React Hooks

```typescript
import { useCodebase } from './hooks'

function MyComponent() {
  const {
    codebase,
    systemDocumentation,
    loading,
    error,
    initializeCodebase,
    generateDocs
  } = useCodebase()

  const handleInitialize = async () => {
    const codebaseId = await initializeCodebase({
      type: 'github',
      githubUrl: 'https://github.com/owner/repo'
    })
    await generateDocs(codebaseId)
  }

  if (loading) return <div>Loading...</div>
  if (error) return <div>Error: {error}</div>

  return <div>{codebase?.name}</div>
}
```

### Using API Client Directly

```typescript
import { apiClient } from './services/api'

// Initialize a codebase
const { codebaseId } = await apiClient.initializeCodebase({
  type: 'github',
  githubUrl: 'https://github.com/owner/repo'
})

// Generate documentation
const docs = await apiClient.generateSystemDocumentation(codebaseId)

// Run analysis
const result = await apiClient.analyzeEvaluation({
  codebaseId,
  inputType: 'snippet',
  inputContent: 'code to analyze',
  providerPreference: 'gemini'
})
```

## Features

### 1. Codebase Management
- Initialize local directories or GitHub repositories
- Real-time indexing progress via WebSocket
- System documentation generation
- Reindex and delete operations

### 2. Evaluation Analysis
- AI-powered codebase analysis
- Multiple provider support (Gemini, OpenAI, etc.)
- Change Request (CR) generation
- Similar evaluation discovery

### 3. arXiv Integration
- Paper search by query
- Fetch full paper content
- Semantic similarity search
- Category filtering

### 4. Brainstorming
- Session CRUD operations
- Idea generation with AI
- Idea expansion and evaluation
- Idea connection analysis

### 5. History & Analytics
- Paginated evaluation history
- Semantic search
- Statistics and trends
- Data export (JSON/CSV)

## Error Handling

All API errors are instances of `APIError`:

```typescript
try {
  await apiClient.someMethod()
} catch (error) {
  if (error instanceof APIError) {
    console.error(error.message)    // Error message
    console.error(error.status)      // HTTP status code
    console.error(error.code)        // Error code
  }
}
```

## WebSocket Support

Real-time updates for codebase indexing:

```typescript
import { useCodebase } from './hooks'

function MyComponent() {
  const { codebase, indexingProgress } = useCodebase(codebaseId)
  
  // indexingProgress contains:
  // - isIndexing: boolean
  // - totalFiles: number
  // - processedFiles: number
  // - currentFile: string
  // - phase: 'scanning' | 'documenting' | 'vectorizing' | 'idle'
  
  return <div>Progress: {indexingProgress.processedFiles}/{indexingProgress.totalFiles}</div>
}
```

## Configuration

Environment variables (`.env`):

```bash
# Backend API URL
VITE_API_URL=http://localhost:8000

# WebSocket URL (optional, defaults to API URL with ws://)
VITE_WS_URL=ws://localhost:8000
```

## Type Safety

All API methods have full TypeScript typing:

```typescript
// Full autocomplete and type checking
const docs: SystemDocumentation = await apiClient.generateSystemDocumentation(codebaseId)
const papers: ArxivPaper[] = (await apiClient.searchArxiv('machine learning')).papers
const session: BrainstormSession = await apiClient.createBrainstormSession(projectId, title)
```

## Migration Guide

### From Direct Service Calls

**Before:**
```typescript
import { analyzeWithGemini } from './services/geminiService'
const result = await analyzeWithGemini(codebase, input, systemDoc)
```

**After:**
```typescript
import { useAnalysis } from './hooks'
const { analyze, loading, result } = useAnalysis()
await analyze({ codebaseId, inputType: 'snippet', inputContent })
```

### From Direct GitHub API

**Before:**
```typescript
import { fetchGitHubRepo } from './services/githubService'
const files = await fetchGitHubRepo(url, onProgress)
```

**After:**
```typescript
import { apiClient } from './services/api'
const files = await apiClient.fetchGitHubRepo(url)
// Progress is now handled via WebSocket
```

## Advanced Patterns

### Request Cancellation

```typescript
import { useAnalysis } from './hooks'

function MyComponent() {
  const { analyze, loading, cancel } = useAnalysis()
  
  const handleAnalyze = () => {
    analyze({ codebaseId, inputType: 'snippet', inputContent })
  }
  
  return (
    <>
      <button onClick={handleAnalyze} disabled={loading}>
        Analyze
      </button>
      <button onClick={cancel} disabled={!loading}>
        Cancel
      </button>
    </>
  )
}
```

### Debounced Search

```typescript
import { useEvaluationSearch } from './hooks'
import { useDebounce } from './hooks/useDebounce'

function MyComponent() {
  const [query, setQuery] = useState('')
  const debouncedQuery = useDebounce(query, 300)
  
  const { results, loading, search, loadMore, hasMore } = useEvaluationSearch()
  
  useEffect(() => {
    search(debouncedQuery)
  }, [debouncedQuery])
  
  return (
    <>
      <input value={query} onChange={e => setQuery(e.target.value)} />
      {results.map(r => <div key={r.id}>{r.id}</div>)}
      {hasMore && <button onClick={loadMore}>Load More</button>}
    </>
  )
}
```

## API Endpoints

See `API_INTEGRATION_SUMMARY.md` for complete endpoint documentation.

## Testing

```bash
# Start backend
cd backend
python -m uvicorn app.main:app --reload

# Start frontend
cd frontend
npm run dev
```

## Contributing

When adding new API endpoints:

1. Add TypeScript types to `types/api.ts`
2. Add method to `services/api.ts` APIClient class
3. Create React hook in `hooks/` if needed
4. Export from `hooks/index.ts`
5. Update documentation
