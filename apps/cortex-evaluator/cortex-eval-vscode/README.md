---
project: Cortex
component: Docs
phase: Build
date_created: 2026-01-16T21:16:58
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:11.630823
---

# Cortex Evaluator VS Code Extension

AI-powered codebase evaluation and Change Request (CR) generation directly within VS Code.

## Features

- **Analyze Files** - Analyze entire files against your codebase
- **Analyze Selection** - Analyze selected code snippets
- **Search Similar Evaluations** - Find past evaluations with semantic search
- **Push to GitHub/Jira/Linear** - Export CRs as issues
- **Side Panel UI** - View results in integrated webview panel
- **Status Bar Integration** - Connection status indicator

## Installation

1. Build the extension:
```bash
npm install
npm run compile
```

2. Package the extension:
```bash
npm install -g vsce
vsce package
```

3. Install the `.vsix` file:
```bash
code --install-extension cortex-eval-vscode-1.0.0.vsix
```

## Usage

### Commands

- `Cortex Evaluator: Analyze Current File` - Analyze entire file
- `Cortex Evaluator: Analyze Selection` - Analyze selected text
- `Cortex Evaluator: Search Similar Evaluations` - Semantic search
- `Cortex Evaluator: Push to GitHub Issue` - Export CR
- `Cortex Evaluator: Open Web Workspace` - Open web UI

### Keyboard Shortcuts

- `Ctrl+Shift+A` (Mac: `Cmd+Shift+A`) - Analyze Current File
- `Ctrl+Shift+E` (Mac: `Cmd+Shift+E`) - Analyze Selection

### Configuration

Configure the extension in your `settings.json`:

```json
{
  "cortexEval.apiUrl": "http://localhost:8000",
  "cortexEval.codebaseId": "your-codebase-id",
  "cortexEval.projectId": "your-project-id"
}
```

## Development

```bash
npm install
npm run compile
npm run watch
```

Press `F5` in VS Code to launch the Extension Development Host.

## API Integration

The extension connects to the Cortex Evaluator backend at `/api/evaluations/analyze`.

## License

MIT
