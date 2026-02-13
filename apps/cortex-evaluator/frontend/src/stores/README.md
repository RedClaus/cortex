---
project: Cortex
component: Docs
phase: Build
date_created: 2026-01-16T20:43:22
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:11.591912
---

# Zustand State Management

This directory contains the Zustand state management system for Cortex Evaluator following the slice pattern.

## Store Structure

### Core Files

- `types.ts` - Shared TypeScript interfaces for all stores
- `useAppStore.ts` - Main application state (auth, theme, provider)
- `useCodebaseStore.ts` - Codebase and documentation management
- `useSessionStore.ts` - Project workspaces and sessions
- `useBrainstormStore.ts` - Brainstorming canvas state
- `useEvaluationStore.ts` - Evaluation and results management
- `index.ts` - Centralized exports

## Store Details

### useAppStore
**Slices:**
- `authSlice` - User authentication state
- `themeSlice` - Dark/light theme management
- `providerSlice` - AI provider selection

**Persistence:** LocalStorage (theme, provider preferences)

```typescript
import { useAppStore } from '@/stores';

const { isDarkMode, toggleTheme, selectedProvider, setSelectedProvider } = useAppStore();
```

### useCodebaseStore
**Slices:**
- `codebaseSlice` - File management and directory scanning
- `documentationSlice` - System documentation

**Actions:**
- `scanDirectory()` - Scan local file system
- `fetchGitHubRepo()` - Fetch from GitHub repository

```typescript
import { useCodebaseStore } from '@/stores';

const { files, addFile, scanDirectory } = useCodebaseStore();
```

### useSessionStore
**Slices:**
- `sessionSlice` - Workspace sessions
- `workspaceSlice` - Workspace CRUD operations

**Persistence:** LocalStorage (workspaces, current workspace)

```typescript
import { useSessionStore } from '@/stores';

const { workspaces, createWorkspace, loadWorkspace, deleteWorkspace } = useSessionStore();
```

### useBrainstormStore
**Slices:**
- `nodeSlice` - Canvas node management
- `edgeSlice` - Edge connections
- `canvasSlice` - Viewport and sessions

**Persistence:** LocalStorage (nodes, edges, viewport)

**Selective Selectors (shallow):**
```typescript
import {
  useBrainstormNodes,
  useBrainstormEdges,
  useBrainstormViewport,
  useBrainstormActions
} from '@/stores';

const nodes = useBrainstormNodes();
const { addNode, deleteNode, updateNode } = useBrainstormActions();
```

### useEvaluationStore
**Slices:**
- `evaluationSlice` - Evaluation lifecycle
- `resultSlice` - Evaluation results

**Selective Selectors:**
```typescript
import {
  useEvaluations,
  useCurrentEvaluation,
  useEvaluationResults,
  useEvaluationActions
} from '@/stores';

const evaluations = useEvaluations();
const { addEvaluation, runEvaluation, fetchEvaluations } = useEvaluationActions();
```

## Slice Pattern

Each store is composed of multiple slices:

```typescript
interface SliceType {
  state: StateType;
  action1: () => void;
  action2: () => void;
}

const createSlice: (
  set: (partial) => void,
  get: () => StoreType
) => SliceType = (set, get) => ({
  // state and actions
});

export const useStore = create<StoreType>((set, get) => ({
  ...createSlice(set, get),
}));
```

## Persistence

Use the `persist` middleware for local storage:

```typescript
import { persist } from 'zustand/middleware';

export const useStore = create<StoreType>()(
  persist(
    (set, get) => ({ /* state */ }),
    {
      name: 'storage-key',
      partialize: (state) => ({
        // select what to persist
        importantState: state.importantState,
      }),
    }
  )
);
```

## Shallow Selectors

Prevent unnecessary re-renders with shallow comparison:

```typescript
import { shallow } from 'zustand/shallow';

const nodes = useStore((state) => state.nodes, shallow);
```

## Type Safety

All stores are fully typed using TypeScript interfaces from `types.ts`.

## Best Practices

1. Use slice pattern for modular state organization
2. Persist only essential data to localStorage
3. Use shallow selectors for array/object properties
4. Keep actions focused and single-responsibility
5. Use `crypto.randomUUID()` for generating IDs
6. Handle async operations with try-catch
7. Update timestamps when data changes
