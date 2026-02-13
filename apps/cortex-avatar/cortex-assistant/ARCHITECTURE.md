---
project: Cortex
component: Unknown
phase: Design
date_created: 2026-01-16T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:10.909760
---

# Cortex Assistant - System Architecture

## 1. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CORTEX ASSISTANT                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         PRESENTATION LAYER                          │   │
│  │  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌────────┐│   │
│  │  │ Dashboard │ │ Meeting   │ │  History  │ │   Tasks   │ │ Memory ││   │
│  │  │   View    │ │   View    │ │   View    │ │   View    │ │  View  ││   │
│  │  └───────────┘ └───────────┘ └───────────┘ └───────────┘ └────────┘│   │
│  │                                                                     │   │
│  │  ┌─────────────────┐  ┌──────────────┐  ┌─────────────────────────┐│   │
│  │  │ Command Palette │  │ Theme System │  │   Keyboard Shortcuts    ││   │
│  │  └─────────────────┘  └──────────────┘  └─────────────────────────┘│   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│  ┌─────────────────────────────────▼───────────────────────────────────┐   │
│  │                          STATE LAYER (Zustand)                      │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────┐ │   │
│  │  │ Meeting  │ │  Tasks   │ │ Settings │ │  Cortex  │ │Transcribe │ │   │
│  │  │  Store   │ │  Store   │ │  Store   │ │  Store   │ │  Store    │ │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └───────────┘ │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│  ┌─────────────────────────────────▼───────────────────────────────────┐   │
│  │                         SERVICE LAYER                               │   │
│  │  ┌────────────────┐  ┌────────────────┐  ┌────────────────────────┐│   │
│  │  │  cortex.ts     │  │  meeting.ts    │  │    transcription.ts   ││   │
│  │  │ (API Client)   │  │ (IndexedDB)    │  │  (Web Speech/Cortex)  ││   │
│  │  └───────┬────────┘  └───────┬────────┘  └────────────────────────┘│   │
│  │          │                   │                                      │   │
│  │  ┌───────▼────────┐  ┌───────▼────────┐  ┌────────────────────────┐│   │
│  │  │  bridge.ts     │  │   export.ts    │  │    redaction.ts       ││   │
│  │  │ (Android JSB)  │  │  (MD/TXT/JSON) │  │  (Privacy Utils)      ││   │
│  │  └────────────────┘  └────────────────┘  └────────────────────────┘│   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└──────────────────────────────────┬──────────────────────────────────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │        CORTEX-02            │
                    │     (localhost:8080)        │
                    ├─────────────────────────────┤
                    │  • LLM Analysis             │
                    │  • Strategic Memory         │
                    │  • Knowledge Base           │
                    │  • STT (Whisper)            │
                    └─────────────────────────────┘
```

## 2. State Management: Zustand

**Why Zustand over Redux Toolkit:**
- **Simpler API**: No boilerplate (actions, reducers, selectors)
- **TypeScript-first**: Better type inference out of the box
- **Smaller bundle**: ~1KB vs ~10KB for RTK
- **Persistence built-in**: Via middleware, no extra packages
- **React 18 ready**: Works seamlessly with concurrent features

### Store Structure

| Store | Responsibility |
|-------|---------------|
| `meetingStore` | Current meeting state, recording, segments |
| `tasksStore` | Action items, filtering, persistence |
| `settingsStore` | App preferences, persisted to localStorage |
| `cortexStore` | Connection status, API operation states |
| `transcriptionStore` | Speech recognition state |
| `uiStore` | Modals, sidebar, command palette |

## 3. Data Flow

### Recording → Transcription → Save → Analyze → Commit → Ingest

```
┌─────────────┐   ┌──────────────┐   ┌─────────────┐   ┌─────────────┐
│ User clicks │──▶│ Web Speech   │──▶│ Add Segment │──▶│  Auto-save  │
│ "Start"     │   │ or Cortex STT│   │ to Meeting  │   │ to IndexedDB│
└─────────────┘   └──────────────┘   └─────────────┘   └─────────────┘
                                           │
                         ┌─────────────────▼─────────────────┐
                         │ User clicks "Analyze"             │
                         └─────────────────┬─────────────────┘
                                           ▼
                   ┌──────────────────────────────────────────┐
                   │ cortex.analyzeMeeting(session)           │
                   │ Returns: summary, actions, decisions...  │
                   └─────────────────┬────────────────────────┘
                                     ▼
              ┌──────────────────────────────────────────────────┐
              │ User can:                                        │
              │  • Import action items to Tasks                  │
              │  • Commit summary/decisions to Cortex Memory     │
              │  • Ingest full transcript to Knowledge Base      │
              └──────────────────────────────────────────────────┘
```

### Memory Search Flow

```
┌───────────────┐   ┌─────────────────────────────────────────────────┐
│ User enters   │──▶│ PARALLEL SEARCH:                                │
│ search query  │   │  • searchMeetings() - Local IndexedDB           │
│               │   │  • cortex.searchMemory() - Remote Cortex        │
└───────────────┘   └────────────────────┬────────────────────────────┘
                                         ▼
                    ┌────────────────────────────────────────────────┐
                    │ Merge results by relevance score               │
                    │ Display unified result list                    │
                    └────────────────────────────────────────────────┘
```

## 4. Extension Points

### Future Features Architecture

| Feature | Extension Point | Implementation Notes |
|---------|----------------|----------------------|
| **Multi-language** | `TranscriptionService.setLanguage()` | Language selector in settings |
| **Live Collaboration** | New `collaboration.ts` service | WebSocket/WebRTC + CRDT state sync |
| **Voice Commands** | `useTranscription` hook | Pattern matching on interim text |
| **Video Recording** | New `video.ts` service | MediaRecorder API + chunk storage |
| **Calendar Integration** | New `calendar.ts` adapter | Google/Outlook OAuth adapters |
| **Slack/Teams** | New `integrations/` folder | Webhook + API adapters |
| **AI Coach** | Real-time streaming analysis | SSE from Cortex during meeting |

### Adding New Integrations

```typescript
// src/services/integrations/calendar.ts
export interface CalendarAdapter {
  connect(): Promise<void>;
  getEvents(range: DateRange): Promise<CalendarEvent[]>;
  createEvent(event: CalendarEvent): Promise<string>;
}

export class GoogleCalendarAdapter implements CalendarAdapter { ... }
export class OutlookCalendarAdapter implements CalendarAdapter { ... }
```

## 5. Project Structure

```
src/
├── app/
│   └── App.tsx              # Root component with router
├── views/
│   ├── Dashboard.tsx        # Home view with stats
│   ├── MeetingView.tsx      # Live transcription view
│   ├── HistoryView.tsx      # Meeting history list
│   ├── TasksView.tsx        # Action item management
│   └── MemoryView.tsx       # Semantic search view
├── components/
│   ├── ui/                  # Base UI components
│   │   ├── Button.tsx
│   │   ├── Input.tsx
│   │   ├── Modal.tsx
│   │   ├── Badge.tsx
│   │   ├── Select.tsx
│   │   └── Card.tsx
│   ├── layout/              # Layout components
│   │   ├── Layout.tsx
│   │   ├── Sidebar.tsx
│   │   ├── CommandPalette.tsx
│   │   ├── SettingsModal.tsx
│   │   └── ExportModal.tsx
│   └── meeting/             # Meeting-specific components
│       ├── TranscriptView.tsx
│       ├── RecordingControls.tsx
│       ├── MeetingHeader.tsx
│       └── AnalysisPanel.tsx
├── services/
│   ├── cortex.ts            # Cortex-02 API client
│   ├── meeting.ts           # IndexedDB persistence
│   ├── transcription.ts     # Speech recognition service
│   └── bridge.ts            # Android JSBridge
├── store/
│   ├── meetingStore.ts
│   ├── tasksStore.ts
│   ├── settingsStore.ts
│   ├── cortexStore.ts
│   ├── transcriptionStore.ts
│   └── uiStore.ts
├── models/
│   └── index.ts             # All TypeScript types
├── hooks/
│   ├── useTheme.ts
│   ├── useKeyboardShortcuts.ts
│   ├── useAutoSave.ts
│   ├── useCortexConnection.ts
│   └── useTranscription.ts
├── utils/
│   ├── format.ts            # Time/date formatting
│   ├── export.ts            # Markdown/text export
│   └── redaction.ts         # Privacy redaction
├── styles/
│   └── index.css            # Tailwind + custom styles
└── tests/
    ├── setup.ts
    ├── models.test.ts
    └── utils.test.ts
```

## 6. Cortex-02 API Contracts

### Base Configuration

```typescript
interface CortexClientConfig {
  baseUrl: string;        // Default: "http://localhost:8080"
  timeout?: number;       // Default: 30000ms
  retryAttempts?: number; // Default: 3
  retryDelay?: number;    // Default: 1000ms
}
```

### API Methods

#### analyzeMeeting

```typescript
// Request
POST /
{
  "jsonrpc": "2.0",
  "method": "message/send",
  "params": {
    "message": {
      "role": "user",
      "parts": [{ "kind": "text", "text": "Analyze this meeting..." }],
      "metadata": { "analysisRequest": true, "meetingId": "..." }
    }
  },
  "id": 1234567890
}

// Response
{
  "jsonrpc": "2.0",
  "result": {
    "status": {
      "message": {
        "role": "agent",
        "parts": [{ "kind": "text", "text": "{...JSON analysis...}" }]
      }
    }
  },
  "id": 1234567890
}
```

#### commitMemory

```typescript
// Request
POST /
{
  "jsonrpc": "2.0",
  "method": "memory/commit",
  "params": {
    "type": "meeting_summary",
    "content": {
      "meetingId": "...",
      "title": "...",
      "summary": "...",
      "decisions": [...],
      "actionItems": [...],
      "participants": [...]
    },
    "metadata": { "redacted": false, "timestamp": "..." }
  },
  "id": 1234567890
}
```

#### searchMemory

```typescript
// Request
POST /
{
  "jsonrpc": "2.0",
  "method": "memory/search",
  "params": {
    "query": "project timeline decisions",
    "limit": 20,
    "sources": ["memory", "knowledge"]
  },
  "id": 1234567890
}
```

### Error Handling Strategy

```typescript
class HttpCortexClient {
  private async request<T>(method: string, endpoint: string, body?: unknown): Promise<T> {
    // 1. Retry with exponential backoff for transient errors
    // 2. Abort via AbortController on timeout
    // 3. Update cortexStore.status on connection changes
    // 4. Surface errors in UI via cortexStore.error
  }
}
```

## 7. Android WebView JSBridge Contract

### Available Methods

| Method | Purpose | Fallback |
|--------|---------|----------|
| `requestAudioFocus()` | Request Android audio focus | Returns true (browser) |
| `releaseAudioFocus()` | Release Android audio focus | No-op |
| `saveFile(name, base64)` | Save file to device | Browser download |
| `shareContent(title, text)` | Native share sheet | Web Share API |
| `getDeviceInfo()` | Device info JSON | `navigator.platform` |
| `showToast(message)` | Native toast | No-op |
| `vibrate(pattern)` | Haptic feedback | `navigator.vibrate()` |
| `isNetworkAvailable()` | Network check | `navigator.onLine` |
| `setKeepScreenOn(bool)` | Prevent screen sleep | No-op |

### Usage Pattern

```typescript
import { bridge } from '@/services/bridge';

// Safe calls with automatic fallbacks
bridge.requestAudioFocus();
await bridge.saveFile('meeting.md', content);
bridge.setKeepScreenOn(true);
```

## 8. Security & Privacy

### Redaction System

```typescript
// Default patterns (configurable in settings)
const patterns = [
  { id: 'email', pattern: '[a-zA-Z0-9._%+-]+@...', replacement: '[EMAIL]' },
  { id: 'phone', pattern: '\\d{3}-\\d{3}-\\d{4}', replacement: '[PHONE]' },
  { id: 'ssn', pattern: '\\d{3}-\\d{2}-\\d{4}', replacement: '[SSN]' },
];

// Applied before memory commit or export
const { text, count } = applyRedactions(transcript, patterns);
```

### Data Flow Privacy

1. **Local-first**: All data stored in IndexedDB by default
2. **Explicit consent**: Memory commit requires user action
3. **Redaction option**: Available before any external send
4. **No cloud STT by default**: Web Speech API runs locally
