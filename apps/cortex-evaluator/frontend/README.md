---
project: Cortex
component: UI
phase: Design
date_created: 2026-01-17T23:11:57
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:06.842619
---

# Frontend (DEPRECATED)

> **DEPRECATED**: This React/Vite frontend is deprecated in favor of the embedded Go webui.

## Use the New WebUI Instead

The new webui is embedded directly in the Go binary at `cmd/webui/` and provides:

- Single binary deployment (no Node.js required)
- File browser for project selection
- Session management with delete functionality
- New Chat button (start fresh conversations within indexed sessions)
- Multi-provider LLM support (OpenAI, Anthropic, Gemini, Groq, Ollama)
- GitHub repository evaluation
- Light/Dark theme toggle
- Cortex First Principles evaluation framework

### Running the New WebUI

```bash
# Build and run
go build -o /tmp/webui ./cmd/webui
PORT=3000 /tmp/webui

# Or use default port 8080
/tmp/webui
```

Then visit http://localhost:3000 (or http://localhost:8080)

---

## Legacy Frontend (Do Not Use)

This directory contains the old React/Vite frontend which is no longer maintained.

If you need to run it for reference:

```bash
cd frontend
npm install
npm run dev  # Runs on port 5173
```

**Note**: The legacy frontend expects a backend on port 8000, which differs from the new webui architecture.
