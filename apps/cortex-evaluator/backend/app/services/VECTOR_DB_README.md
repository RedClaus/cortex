---
project: Cortex
component: Docs
phase: Design
date_created: 2026-01-16T20:52:16
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:13.725917
---

# Vector Database Service

ChromaDB integration for Cortex Evaluator backend, providing semantic search for code, evaluations, and research papers.

## Features

- **Three collections**: code_snippets, evaluations, papers
- **Dual embedding support**: OpenAI (cloud) or SentenceTransformer (local)
- **Batch processing**: Efficient file indexing with progress callbacks
- **Metadata filtering**: Search by codebase, project, provider, scores, etc.
- **Error handling**: Graceful duplicate ID handling and retry logic

## Installation

```bash
cd backend

# Install dependencies with compatible versions
pip install -r requirements.txt

# If using OpenAI embeddings (recommended for production):
export OPENAI_API_KEY="your-key-here"

# ChromaDB will use local SentenceTransformer embeddings by default
# if OPENAI_API_KEY is not set
```

## Quick Start

```python
from app.services.vector_db import VectorStore

# Initialize
vector_store = VectorStore()

# Index a codebase
files = [
    {
        'file_path': 'src/main.py',
        'content': 'def hello(): return "Hello"',
        'language': 'python',
        'function_name': 'hello'
    }
]

await vector_store.index_codebase(
    codebase_id='my-project',
    files=files
)

# Search similar code
results = await vector_store.search_similar_code(
    query='greeting function',
    codebase_id='my-project',
    n_results=5
)

for result in results:
    print(f"{result['file_path']}: {result['similarity']:.2f}")
```

## API Reference

### VectorStore

#### `__init__()`
Initialize ChromaDB PersistentClient and create collections.

#### `async index_codebase(codebase_id, files, on_progress=None)`
Index a codebase by processing files and generating embeddings.

**Args:**
- `codebase_id` (str): Unique identifier for the codebase
- `files` (List[Dict]): List of file dictionaries with:
  - `file_path` (str): Path to the file
  - `content` (str): File content
  - `language` (str): Programming language
  - `function_name` (Optional[str]): Function/class name
- `on_progress` (Optional[Callable[[int, int], None]]): Progress callback

**Returns:** `int` - Number of files indexed

**Batch Size:** 100 documents per batch for optimal performance

#### `async search_similar_code(query, codebase_id=None, n_results=5)`
Search for similar code snippets.

**Args:**
- `query` (str): Search query
- `codebase_id` (Optional[str]): Filter by specific codebase
- `n_results` (int): Number of results (default: 5)

**Returns:** `List[Dict]` with keys:
- `code` (str): Code content
- `file_path` (str): File path
- `language` (str): Programming language
- `function_name` (str): Function/class name
- `similarity` (float): 0-1 similarity score
- `distance` (float): Cosine distance

#### `async store_evaluation(evaluation_id, summary, cr, metadata)`
Store an evaluation result.

**Args:**
- `evaluation_id` (str): Unique identifier
- `summary` (str): Evaluation summary
- `cr` (str): Code review content
- `metadata` (Dict): Additional metadata (project_id, provider, scores, etc.)

**Returns:** `str` - Evaluation ID

#### `async search_similar_evaluations(query, project_id=None, filters=None, n_results=5)`
Search for similar evaluations.

**Args:**
- `query` (str): Search query
- `project_id` (Optional[str]): Filter by project
- `filters` (Optional[Dict]): Metadata filters (e.g., `{"provider": "gemini"}`)
- `n_results` (int): Number of results (default: 5)

**Returns:** `List[Dict]` with keys:
- `id` (str): Evaluation ID
- `similarity` (float): 0-1 similarity score
- `distance` (float): Cosine distance
- `metadata` (Dict): Evaluation metadata
- `content` (str): Combined summary + CR

#### `async index_arxiv_paper(paper_id, title, authors, content, metadata)`
Index an arXiv research paper.

**Args:**
- `paper_id` (str): arXiv paper ID (e.g., "2301.12345")
- `title` (str): Paper title
- `authors` (List[str]): List of author names
- `content` (str): Paper content/abstract
- `metadata` (Dict): Additional metadata (categories, published date, etc.)

**Returns:** `str` - Paper ID

#### `async search_papers(query, categories=None, n_results=5)`
Search for similar research papers.

**Args:**
- `query` (str): Search query
- `categories` (Optional[List[str]]): Filter by arXiv categories
- `n_results` (int): Number of results (default: 5)

**Returns:** `List[Dict]` with keys:
- `paper_id` (str): arXiv paper ID
- `title` (str): Paper title
- `authors` (List[str]): List of authors
- `similarity` (float): 0-1 similarity score
- `distance` (float): Cosine distance
- `metadata` (Dict): Paper metadata
- `content` (str): Combined title + content

#### `async delete_codebase(codebase_id)`
Delete all documents for a codebase.

**Args:**
- `codebase_id` (str): Unique identifier

**Returns:** `int` - Number of documents deleted

#### `async get_collection_stats()`
Get statistics for all collections.

**Returns:** `Dict[str, int]` with keys:
- `code_snippets` (int): Document count
- `evaluations` (int): Document count
- `papers` (int): Document count

## Embedding Comparison

| Factor | OpenAI | SentenceTransformer |
|--------|---------|-------------------|
| **Setup** | API key only | pip install + model download |
| **Models** | text-embedding-3-small (1536 dim) | all-MiniLM-L6-v2 (384 dim) |
| **Cost** | $0.02 per 1M tokens | Free (compute cost only) |
| **Latency** | Network-dependent (~50-200ms) | Local (~10-50ms CPU, 5-15ms GPU) |
| **Privacy** | Data sent to OpenAI | Data stays local |
| **Quality** | Higher quality | Good quality |

## Configuration

Edit `backend/app/core/config.py`:

```python
CHROMA_PATH: str = "./data/chroma"  # Vector database path
OPENAI_API_KEY: str = ""  # Set for OpenAI embeddings
```

## Error Handling

The service handles these ChromaDB errors gracefully:

- `DuplicateIDException`: Updates existing records instead of failing
- `NotEnoughElementsException`: Automatically reduces `n_results`
- `NoIndexException`: Returns empty results for empty collections

## Performance Tips

1. **Batch processing**: Use batch size 100-1000 for indexing
2. **OpenAI embeddings**: Better quality, faster for small datasets
3. **SentenceTransformer**: Better for large datasets, no API costs
4. **Metadata filtering**: Reduce search space with filters before querying

## Testing

```bash
cd backend
python -m pytest tests/test_vector_store.py
```

Or run the test script directly:

```bash
python test_vector_store.py
```

## Data Persistence

ChromaDB stores all embeddings and metadata at `CHROMA_PATH` (default: `./data/chroma`).

**Backup:**
```bash
tar -czf chroma-backup.tar.gz ./data/chroma
```

**Restore:**
```bash
tar -xzf chroma-backup.tar.gz
```
