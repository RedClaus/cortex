---
project: Cortex
component: Brain Kernel
phase: Design
date_created: 2026-01-16T20:42:50
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:11.601365
---

# Brainstorm Canvas Component

A complete React Flow-based brainstorming canvas with custom nodes for the Cortex Evaluator frontend.

## Features

- **Custom Node Types**: Problem, Solution, Question, Reference
- **Drag-and-Drop**: Drag nodes from the palette to the canvas
- **Editable Content**: All node fields are editable
- **AI Integration**: Support for AI-generated content with confidence scores
- **Context Menu**: Right-click for AI expansion and deletion
- **Auto-Save**: Canvas state persists to localStorage
- **React Flow Features**: MiniMap, Controls, Background, Floating edges

## Installation

This component requires the following dependencies:

```bash
npm install reactflow
```

## Usage

```tsx
import { BrainstormCanvas } from '@/components/brainstorm';

function App() {
  return <BrainstormCanvas />;
}
```

## Node Types

### Problem Node (Red)
- Icon: ⚠️
- Use for identifying issues, bugs, or challenges
- AI-generated content with confidence scores

### Solution Node (Green)
- Icon: ✅
- Use for proposed solutions and fixes
- AI-generated content with confidence scores

### Question Node (Blue)
- Icon: ❓
- Use for open questions and areas to explore
- AI-generated content with confidence scores

### Reference Node (Purple)
- Icon: 🔗
- Smaller, compact layout
- Use for external references, links, and citations
- Source attribution field

## Architecture

```
brainstorm/
├── types.ts           # TypeScript interfaces and enums
├── ProblemNode.tsx    # Custom problem node component
├── SolutionNode.tsx   # Custom solution node component
├── QuestionNode.tsx   # Custom question node component
├── ReferenceNode.tsx  # Custom reference node component
├── BrainstormCanvas.tsx # Main canvas component with React Flow
├── NodePalette.tsx    # Sidebar with draggable node types
└── index.ts          # Export all components
```

## State Management

- Canvas state is saved to localStorage as `brainstorm-canvas-state`
- State includes nodes, edges, and timestamp
- Auto-saves on any node/edge change

## Future Enhancements

- AI node expansion integration with CortexBrain
- Export/import canvas as JSON
- Collaborative editing support
- Node templates
- Keyboard shortcuts
- Undo/redo functionality
