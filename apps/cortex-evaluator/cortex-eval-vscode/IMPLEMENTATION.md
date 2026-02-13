---
project: Cortex
component: Unknown
phase: Design
date_created: 2026-01-16T21:21:26
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:11.621301
---

# Cortex Evaluator VS Code Extension - Complete Implementation

## Overview

A complete VS Code extension for Cortex Evaluator that provides AI-powered codebase evaluation and Change Request (CR) generation directly within the editor.

## Project Structure

```
cortex-eval-vscode/
├── src/
│   ├── api/
│   │   └── client.ts              # Backend API client (axios-based)
│   ├── commands/
│   │   ├── analyzeFile.ts          # Analyze entire file command
│   │   ├── analyzeSelection.ts      # Analyze selected text command
│   │   ├── searchSimilar.ts        # Search similar evaluations
│   │   ├── pushCR.ts             # Push CR to GitHub/Jira/Linear
│   │   └── openWorkspace.ts       # Open web workspace in browser
│   ├── webview/
│   │   └── SidePanelProvider.ts   # Side panel webview UI
│   ├── media/
│   │   ├── icon.svg               # Extension icon
│   │   └── style.css             # Webview styling
│   └── extension.ts               # Main extension entry point
├── dist/                          # Compiled JavaScript output
├── package.json                   # Extension manifest
├── tsconfig.json                 # TypeScript configuration
├── .vscode/
│   ├── launch.json                 # Debug configuration
│   └── tasks.json                  # Build tasks
├── .gitignore                     # Git ignore rules
├── .vscodeignore                 # Packaging ignore rules
├── .eslintrc.js                  # ESLint configuration
└── README.md                      # Documentation
```

## Features Implemented

### 1. Main Extension (`extension.ts`)
- **activate()**: Registers all commands and webview provider
- **deactivate()**: Cleanup on extension deactivation
- **Status Bar Item**: Connection status indicator in status bar
- **Health Check**: Tests backend connection on startup

### 2. Commands

#### `analyzeFile` (`commands/analyzeFile.ts`)
- Gets active text editor document
- Reads full file content
- Calls backend API `/api/evaluations/analyze`
- Shows progress with `window.withProgress`
- Displays results in side panel
- Creates timestamped CR output file
- Auto-selects codebase from config or prompts

#### `analyzeSelection` (`commands/analyzeSelection.ts`)
- Gets editor selection
- Validates selection not empty
- Calls backend API with selected text
- Shows quick pick for action:
  - Insert Below Selection
  - Open in New File
  - Show in Panel Only
- Supports file or selection analysis

#### `searchSimilar` (`commands/searchSimilar.ts`)
- Gets current file path or selection
- Calls `/api/history/search?semantic=true`
- Displays results in quick pick with metadata
- Opens full evaluation details on select
- Shows score and metadata preview

#### `pushCR` (`commands/pushCR.ts`)
- Gets current editor content
- Parses CR title from markdown heading
- Shows platform picker (GitHub/Jira/Linear)
- Collects metadata:
  - GitHub: labels (comma-separated)
  - Jira: priority (Highest/Lowest)
- Calls `/api/integrations/issues`
- Opens issue URL in browser
- Shows success notification

#### `openWorkspace` (`commands/openWorkspace.ts`)
- Gets project ID from config
- Opens web UI in browser via `env.openExternal`
- Option to copy URL instead
- Uses `{apiUrl}/projects/{projectId}` format

### 3. Webview Side Panel (`webview/SidePanelProvider.ts`)

**Features:**
- Connection status indicator (Connected/Disconnected)
- Result display with:
  - Value Score (0-100) with color-coded bar
  - Executive Summary
  - Technical Feasibility
  - Gap Analysis
  - Change Request
  - Similar Evaluations list
- Action buttons:
  - Insert Below: Insert CR into current editor
  - Copy CR: Copy to clipboard
  - Open in New File: Create markdown file
- Responsive design with CSS variables
- Message passing between extension and webview

### 4. API Client (`api/client.ts`)

**Endpoints Implemented:**
- `healthCheck()` - GET `/health`
- `analyzeEvaluation()` - POST `/api/evaluations/analyze`
- `searchEvaluations()` - GET `/api/history/search`
- `getEvaluation()` - GET `/api/evaluations/{id}`
- `getSimilarEvaluations()` - GET `/api/evaluations/{id}/similar`
- `createIssue()` - POST `/api/integrations/issues`
- `getCodebase()` - GET `/api/codebases/{id}`
- `listCodebases()` - GET `/api/codebases/`

**Features:**
- Axios-based HTTP client with 60s timeout
- Error handling with custom `APIError` type
- Dynamic baseURL configuration
- Request/response interceptors
- Type-safe API interfaces

### 5. Extension Manifest (`package.json`)

**Contribution Points:**
- **Commands**: 6 commands registered
- **Views**: Activity bar icon + Side panel webview
- **Menus**: Context menu for selection
- **Keybindings**:
  - `Ctrl+Shift+A` / `Cmd+Shift+A`: Analyze Current File
  - `Ctrl+Shift+E` / `Cmd+Shift+E`: Analyze Selection
- **Configuration**:
  - `cortexEval.apiUrl`: Backend API URL (default: http://localhost:8000)
  - `cortexEval.codebaseId`: Default codebase ID
  - `cortexEval.projectId`: Project ID for workspace

## Configuration

### User Settings (`.vscode/settings.json`)
```json
{
  "cortexEval.apiUrl": "http://localhost:8000",
  "cortexEval.codebaseId": "codebase-uuid-here",
  "cortexEval.projectId": "project-uuid-here"
}
```

## Usage

### Analyzing Files
1. Open a file in VS Code
2. Press `Ctrl+Shift+A` (Mac: `Cmd+Shift+A`)
3. Select codebase if prompted
4. View results in side panel

### Analyzing Selection
1. Select code snippet
2. Press `Ctrl+Shift+E` (Mac: `Cmd+Shift+E`)
3. Choose action: Insert, New File, or Panel
4. Review results

### Searching Similar Evaluations
1. Command Palette: `Cortex Evaluator: Search Similar Evaluations`
2. Enter search query or use current file
3. Browse results in quick pick
4. View full evaluation

### Pushing to GitHub
1. Open CR markdown file
2. Command Palette: `Cortex Evaluator: Push to GitHub Issue`
3. Select platform (GitHub/Jira/Linear)
4. Configure metadata if needed
5. Issue created, URL opened

## Development

### Installation
```bash
cd cortex-eval-vscode
npm install
npm run compile
```

### Running in Development
```bash
# Press F5 in VS Code to launch Extension Development Host
# Or use CLI:
npm run watch
```

### Building for Distribution
```bash
npm install -g @vscode/vsce
vsce package
# Creates cortex-eval-vscode-1.0.0.vsix
```

### Running Tests
```bash
npm run test
```

### Linting
```bash
npm run lint
```

## API Integration

The extension connects to the Cortex Evaluator backend:

- **Base URL**: Configurable via `cortexEval.apiUrl` (default: `http://localhost:8000`)
- **Authentication**: Not required for basic endpoints
- **Timeout**: 60 seconds for long-running evaluations
- **Error Handling**: User-friendly error messages with `vscode.window.showErrorMessage()`

## Security

- Content Security Policy in webview restricts script execution
- No external script loading
- Local resource roots only for assets
- User consent for browser opening
- No sensitive data storage

## Dependencies

### Runtime
- `axios@^1.6.5`: HTTP client

### Development
- `@types/vscode@^1.85.0`: VS Code API types
- `@types/node@^20.11.5`: Node.js types
- `typescript@^5.3.3`: TypeScript compiler
- `eslint@^8.56.0`: Linting
- `@typescript-eslint/*`: TypeScript linting rules

## TypeScript Configuration

- **Target**: ES2020
- **Module**: CommonJS
- **Strict**: Yes
- **Source Maps**: Enabled for debugging
- **Declaration**: Enabled for `.d.ts` generation

## Browser Compatibility

Webview uses standard APIs:
- `acquireVsCodeApi()` (VS Code specific)
- `window.postMessage()` for communication
- CSS Grid/Flexbox for layout
- No external libraries required

## Future Enhancements

Potential features to add:
1. Inline annotations for code issues
2. Diff view for CR vs original
3. Multi-file batch analysis
4. Custom CR templates
5. Integration with other VCS (GitLab, Bitbucket)
6. Evaluation history visualization
7. Export reports (PDF, HTML)
8. Real-time progress updates via websockets

## Troubleshooting

### Connection Issues
- Check `cortexEval.apiUrl` matches backend
- Verify backend is running on correct port
- Check firewall/network settings
- View status bar indicator for connection state

### Analysis Fails
- Ensure codebase ID is valid
- Check file size (should be < 10MB)
- Verify API keys in backend configuration
- Check backend logs for errors

### Compilation Errors
- Ensure all dependencies installed (`npm install`)
- Check TypeScript version matches package.json
- Run `npm run clean && npm run compile`
- Verify no syntax errors in `.ts` files

## License

MIT License - See parent project for details.

---

**Extension ID**: cortex-eval-vscode
**Version**: 1.0.0
**VS Code Engine**: ^1.85.0
