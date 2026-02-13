---
project: Cortex
component: Docs
phase: Ideation
date_created: 2024-01-16T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:14.457549
---

# @cortex-evaluator/cli

CLI tool for Cortex Evaluator - Analyze papers, generate CRs, and evaluate technical proposals.

## Installation

```bash
cd cortex-eval-cli
npm install
npm run build
npm link
```

Or install globally:

```bash
npm install -g @cortex-evaluator/cli
```

## Usage

### Initialize Project

```bash
cortex-eval init
```

Creates a `.cortex-eval.json` config file in your project directory.

### Analyze Codebase

```bash
# Analyze against arXiv paper
cortex-eval analyze 2301.00774

# Analyze against text input
cortex-eval analyze "Implement RAG with vector database"

# Analyze against local file
cortex-eval analyze ./proposal.md

# Analyze against URL
cortex-eval analyze https://arxiv.org/abs/2301.00774

# With options
cortex-eval analyze 2301.00774 --provider claude --template github --output cr-rag.md
```

### Analyze arXiv Paper

```bash
# Fetch and analyze paper
cortex-eval paper 2301.00774

# Fetch only, no analysis
cortex-eval paper 2301.00774 --no-analysis

# Download PDF
cortex-eval paper 2301.00774 --download-pdf

# With custom provider
cortex-eval paper 2301.00774 --provider openai
```

### Compare Approaches

```bash
# Compare multiple papers
cortex-eval compare 2301.00774 2301.08241 2301.09321

# Compare text inputs
cortex-eval compare "Use RAG" "Use fine-tuning" "Use hybrid"

# With custom criteria
cortex-eval compare "Approach A" "Approach B" --criteria feasibility,cost,performance

# Save comparison matrix
cortex-eval compare "A" "B" "C" --output comparison.md
```

### Push CR to Platform

```bash
# Push CR file to GitHub
cortex-eval push --file cr-latest.md --platform github --repo owner/repo

# Dry run (preview without creating)
cortex-eval push --dry-run

# Use default file (cr-latest.md)
cortex-eval push --platform github

# Interactive mode
cortex-eval push
```

## Configuration

The CLI uses a `.cortex-eval.json` config file:

```json
{
  "projectId": "proj_1234567890_abc123",
  "projectName": "My Project",
  "codebaseId": "cb_abc123",
  "apiBaseUrl": "http://localhost:8000",
  "createdAt": "2024-01-16T12:00:00.000Z",
  "updatedAt": "2024-01-16T12:00:00.000Z"
}
```

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize Cortex Evaluator in current project |
| `analyze` | Analyze codebase against input (arxiv URL, paper ID, text, or file) |
| `paper` | Analyze an arXiv paper against codebase |
| `compare` | Compare multiple approaches/papers side-by-side |
| `push` | Push CR to external platform (GitHub, Jira, Linear) |

## Development

```bash
# Install dependencies
npm install

# Build TypeScript
npm run build

# Watch mode for development
npm run watch

# Run tests
npm test

# Lint
npm run lint

# Format
npm run format
```

## Options

### Global Options

- `-h, --help` - Display help
- `-V, --version` - Display version

### init Options

- `--name <name>` - Project name
- `--dir <directory>` - Project directory (default: current)

### analyze Options

- `--provider <provider>` - AI provider (openai, anthropic, gemini, etc.)
- `--template <template>` - CR template to use
- `-o, --output <file>` - Output file for CR
- `--codebase <id>` - Codebase ID to analyze against

### paper Options

- `--no-analysis` - Only fetch paper without analysis
- `--download-pdf` - Download PDF paper
- `--provider <provider>` - AI provider preference
- `--codebase <id>` - Codebase ID to analyze against

### compare Options

- `--criteria <criteria>` - Comma-separated evaluation criteria
- `--output <file>` - Output file for comparison matrix
- `--codebase <id>` - Codebase ID to analyze against

### push Options

- `-f, --file <file>` - CR file to push (default: cr-latest.md)
- `--platform <platform>` - Platform (github, jira, linear)
- `--repo <repo>` - Repository for GitHub (owner/repo)
- `--dry-run` - Preview without creating issue

## Examples

```bash
# Initialize project
cortex-eval init --name "RAG Pipeline"

# Analyze RAG paper
cortex-eval analyze 2301.00774 --provider claude

# Compare embedding approaches
cortex-eval compare "OpenAI embeddings" "Sentence transformers" "Cohere embeddings" --criteria cost,performance,accuracy

# Push CR to GitHub
cortex-eval push --file cr-rag.md --platform github --repo myorg/myrepo
```

## Requirements

- Node.js 18+
- Backend API running on configured port (default: http://localhost:8000)
- For GitHub integration: GitHub CLI (gh)

## License

MIT
