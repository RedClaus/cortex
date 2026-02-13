---
project: Cortex
component: Unknown
phase: Ideation
date_created: 2026-01-16T20:44:35
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:11.582959
---

# Zustand Store Usage Examples

This document provides practical examples for using each store in the Cortex Evaluator application.

## App Store Examples

### Theme Management
```typescript
import { useAppStore } from '@/stores';

function ThemeToggle() {
  const { isDarkMode, toggleTheme } = useAppStore();
  
  return (
    <button onClick={toggleTheme}>
      {isDarkMode ? '☀️ Light' : '🌙 Dark'}
    </button>
  );
}
```

### Provider Selection
```typescript
function ProviderSelector() {
  const { selectedProvider, setSelectedProvider } = useAppStore();
  
  return (
    <select
      value={selectedProvider}
      onChange={(e) => setSelectedProvider(e.target.value)}
    >
      <option value="openai">OpenAI</option>
      <option value="anthropic">Anthropic</option>
      <option value="gemini">Gemini</option>
    </select>
  );
}
```

## Codebase Store Examples

### Displaying Files
```typescript
import { useCodebaseStore } from '@/stores';

function FileList() {
  const { files, removeFile } = useCodebaseStore();
  
  return (
    <ul>
      {files.map((file) => (
        <li key={file.id}>
          <span>{file.name}</span>
          <button onClick={() => removeFile(file.id)}>Remove</button>
        </li>
      ))}
    </ul>
  );
}
```

### Directory Scanning
```typescript
async function handleDirectoryPick() {
  const { scanDirectory } = useCodebaseStore();
  
  const handle = await window.showDirectoryPicker();
  const files = await scanDirectory(handle);
  console.log(`Scanned ${files.length} files`);
}
```

### GitHub Repository
```typescript
async function fetchFromGitHub() {
  const { fetchGitHubRepo } = useCodebaseStore();
  
  try {
    const files = await fetchGitHubRepo('https://github.com/user/repo');
    console.log(`Loaded ${files.length} files`);
  } catch (error) {
    console.error('Failed to fetch:', error);
  }
}
```

## Session Store Examples

### Workspace List
```typescript
import { useSessionStore } from '@/stores';

function WorkspaceList() {
  const { workspaces, currentWorkspaceId, loadWorkspace, deleteWorkspace } = useSessionStore();
  
  return (
    <ul>
      {workspaces.map((workspace) => (
        <li key={workspace.id}>
          <button onClick={() => loadWorkspace(workspace.id)}>
            {workspace.name}
            {workspace.id === currentWorkspaceId && ' (active)'}
          </button>
          <button onClick={() => deleteWorkspace(workspace.id)}>Delete</button>
        </li>
      ))}
    </ul>
  );
}
```

### Create New Workspace
```typescript
function CreateWorkspaceForm() {
  const { createWorkspace } = useSessionStore();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  
  const handleSubmit = (e) => {
    e.preventDefault();
    createWorkspace(name, description);
    setName('');
    setDescription('');
  };
  
  return (
    <form onSubmit={handleSubmit}>
      <input
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="Workspace name"
      />
      <textarea
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        placeholder="Description"
      />
      <button type="submit">Create</button>
    </form>
  );
}
```

## Brainstorm Store Examples

### Canvas with Nodes and Edges
```typescript
import {
  useBrainstormNodes,
  useBrainstormEdges,
  useBrainstormActions
} from '@/stores';

function BrainstormCanvas() {
  const nodes = useBrainstormNodes();
  const edges = useBrainstormEdges();
  const { addNode, addEdge, deleteNode } = useBrainstormActions();
  
  const handleAddNode = () => {
    addNode({
      id: crypto.randomUUID(),
      type: 'idea',
      content: 'New idea',
      position: { x: 100, y: 100 },
      connections: [],
      metadata: {},
    });
  };
  
  return (
    <div>
      <button onClick={handleAddNode}>Add Node</button>
      {nodes.map((node) => (
        <div
          key={node.id}
          style={{
            position: 'absolute',
            left: node.position.x,
            top: node.position.y,
            border: '1px solid #ccc',
            padding: '8px',
          }}
        >
          {node.content}
          <button onClick={() => deleteNode(node.id)}>×</button>
        </div>
      ))}
    </div>
  );
}
```

### Viewport Control
```typescript
import { useBrainstormViewport, useBrainstormActions } from '@/stores';

function ViewportControls() {
  const viewport = useBrainstormViewport();
  const { setViewport } = useBrainstormActions();
  
  const handleZoomIn = () => {
    setViewport({ zoom: viewport.zoom * 1.2 });
  };
  
  const handleZoomOut = () => {
    setViewport({ zoom: viewport.zoom / 1.2 });
  };
  
  return (
    <div>
      <button onClick={handleZoomOut}>-</button>
      <span>{Math.round(viewport.zoom * 100)}%</span>
      <button onClick={handleZoomIn}>+</button>
    </div>
  );
}
```

## Evaluation Store Examples

### Evaluation List
```typescript
import { useEvaluations, useEvaluationActions } from '@/stores';

function EvaluationList() {
  const evaluations = useEvaluations();
  const { deleteEvaluation, runEvaluation } = useEvaluationActions();
  
  return (
    <ul>
      {evaluations.map((evaluation) => (
        <li key={evaluation.id}>
          <span>{evaluation.name}</span>
          <span>Status: {evaluation.status}</span>
          <button
            onClick={() => runEvaluation(evaluation.id)}
            disabled={evaluation.status === 'running'}
          >
            Run
          </button>
          <button onClick={() => deleteEvaluation(evaluation.id)}>Delete</button>
        </li>
      ))}
    </ul>
  );
}
```

### Create Evaluation
```typescript
function CreateEvaluationForm() {
  const { addEvaluation } = useEvaluationActions();
  const [name, setName] = useState('');
  
  const handleSubmit = (e) => {
    e.preventDefault();
    addEvaluation({
      id: crypto.randomUUID(),
      workspaceId: 'current-workspace-id',
      name,
      criteria: [],
      status: 'pending',
      createdAt: new Date(),
    });
    setName('');
  };
  
  return (
    <form onSubmit={handleSubmit}>
      <input
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="Evaluation name"
      />
      <button type="submit">Create</button>
    </form>
  );
}
```

### Display Results
```typescript
import { useEvaluationResults } from '@/stores';

function EvaluationResults({ evaluationId }: { evaluationId: string }) {
  const results = useEvaluationResults(evaluationId);
  
  return (
    <div>
      <h3>Results ({results.length})</h3>
      {results.map((result) => (
        <div key={result.id}>
          <p>Score: {result.score}</p>
          <p>Notes: {result.notes}</p>
        </div>
      ))}
    </div>
  );
}
```

## Best Practices

### Use Selectors for Performance
```typescript
// Bad - causes re-renders when any state changes
const store = useStore();

// Good - only re-renders when nodes change
const nodes = useStore((state) => state.nodes);

// Better - use shallow for arrays/objects
const nodes = useStore((state) => state.nodes, shallow);
```

### Handle Async Operations
```typescript
async function handleAsync() {
  const { fetchEvaluations } = useEvaluationActions();
  
  try {
    await fetchEvaluations(workspaceId);
  } catch (error) {
    console.error('Operation failed:', error);
    // Show error to user
  }
}
```

### Combine Multiple Stores
```typescript
import { useAppStore } from '@/stores';
import { useCodebaseStore } from '@/stores';

function CombinedComponent() {
  const { isDarkMode } = useAppStore();
  const { files } = useCodebaseStore();
  
  return (
    <div className={isDarkMode ? 'dark' : 'light'}>
      <h2>Files ({files.length})</h2>
    </div>
  );
}
```
