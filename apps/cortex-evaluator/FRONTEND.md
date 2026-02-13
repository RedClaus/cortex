---
project: Cortex
component: UI
phase: Build
date_created: 2026-01-16T21:14:43
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:06.811230
---

# Frontend Documentation

Cortex Evaluator's frontend is a high-performance, interactive dashboard built with React 19 and Vite.

## 🛠️ Tech Stack

- **Framework**: React 19
- **Build Tool**: Vite
- **State Management**: Zustand (with Slice pattern and Persistence)
- **Styling**: Tailwind CSS
- **Interactive Canvas**: React Flow
- **API Client**: Fetch with custom class wrapper

## 📂 Directory Structure

```
frontend/src/
├── components/          # UI Components
│   ├── analytics/       # Stats and Dashboard widgets
│   ├── brainstorm/      # React Flow canvas and custom nodes
│   ├── cr-editor/       # Markdown CR editor
│   ├── evaluations/     # Analysis cards and modal details
│   ├── shared/          # Reusable UI elements (buttons, inputs)
│   └── workspace/       # Project and session selectors
├── hooks/               # Custom React hooks (useAnalysis, useBrainstorm)
├── services/            # API and formatting logic
├── stores/              # Zustand state stores
├── types/               # TypeScript interfaces and types
└── App.tsx              # Main entry point
```

## 🧠 State Management (Zustand)

We use a modular approach with Zustand slices to manage different parts of the application state.

### Core Stores
- **useAppStore**: Manages authentication, theme (dark/light), and global provider preferences.
- **useBrainstormStore**: Manages React Flow nodes, edges, and canvas viewport.
- **useEvaluationStore**: Manages current analysis results and history.
- **useCodebaseStore**: Manages indexed codebase context and documentation.

**Persistence**: All stores use the `persist` middleware to ensure data is saved to `localStorage`, allowing users to resume work after a page refresh.

## 🎨 Styling Guidelines

- **Tailwind CSS**: Use utility classes for all styling.
- **Dark Mode**: Every component must support dark mode using the `dark:` prefix.
- **Glassmorphism**: Use `backdrop-blur` and translucent backgrounds for overlays.
- **Animations**: Use Framer Motion for smooth transitions between views.

## 🧩 Key Components

### BrainstormCanvas
Located in `components/brainstorm/`. It uses React Flow to create a mind-map-like interface. 
- **Custom Nodes**: `ProblemNode`, `SolutionNode`, `QuestionNode`, and `ReferenceNode`.
- **Node Origin**: Centered at `[0.5, 0.5]` for organic expansion.

### EvaluationHistory
Located in `components/evaluations/`. Displays a searchable timeline of past analyses with performance scores.

## 🛠️ Build Commands

### Development
```bash
npm run dev
```

### Build for Production
```bash
npm run build
```

### Run Tests
```bash
npm run test
```

## 🔌 API Integration

The `apiClient` (in `services/api.ts`) provides a type-safe way to communicate with the FastAPI backend. It handles error responses, base URL configuration, and JSON parsing.

Example usage:
```typescript
import { apiClient } from './services/api';

const result = await apiClient.analyzeEvaluation({
  codebase_id: '...',
  input_type: 'snippet',
  input_content: '...'
});
```
