---
project: Cortex
component: Unknown
phase: Design
date_created: 2026-01-16T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:11.011482
---

# Cortex Assistant - Acceptance Checklist

## Core Functionality

### Offline Operation
- [ ] App loads without network connection
- [ ] Meetings save to IndexedDB when offline
- [ ] Web Speech API works without Cortex connection
- [ ] Graceful degradation when Cortex is unavailable

### Recording & Transcription
- [ ] Start/stop recording works
- [ ] Pause/resume recording works
- [ ] Web Speech API transcription displays in real-time
- [ ] Interim (partial) results show while speaking
- [ ] Final results are added as segments
- [ ] Timer updates during recording
- [ ] Recording indicator pulses when active

### Auto-save & Crash Recovery
- [ ] Auto-save triggers during recording (30s default)
- [ ] Refreshing page shows recovery prompt
- [ ] Clicking "Restore" loads auto-saved meeting
- [ ] Clicking "Cancel" clears auto-save and starts fresh

### Meeting Persistence
- [ ] Save button saves to IndexedDB
- [ ] Meeting appears in History view
- [ ] Opening from History loads full meeting
- [ ] Meeting title can be edited
- [ ] Tags can be added/removed
- [ ] Duplicate meeting creates a copy
- [ ] Delete meeting removes from storage

### Transcript Editing
- [ ] Hover shows edit button on segments
- [ ] Clicking edit enables inline editing
- [ ] Save edit updates segment text
- [ ] Cancel edit reverts changes
- [ ] Edited segments show "(edited)" indicator
- [ ] Original text is preserved

### AI Analysis (requires Cortex)
- [ ] Analyze button disabled when no content
- [ ] Analyze button shows loading state
- [ ] Analysis populates Summary tab
- [ ] Action items appear in Actions tab
- [ ] Decisions appear in Decisions tab
- [ ] Risks appear in Risks tab
- [ ] Sentiment indicator shows

### Memory Commit
- [ ] Commit button disabled without analysis
- [ ] Commit shows loading state
- [ ] Success message appears after commit
- [ ] Button text changes to "Committed!"

### Knowledge Ingest
- [ ] Ingest button works with any meeting
- [ ] Shows loading state during ingest
- [ ] Success message appears after ingest
- [ ] Button text changes to "Ingested!"

### Task Management
- [ ] Import tasks from analysis works
- [ ] Manual task creation works
- [ ] Task completion toggle works
- [ ] Task editing works
- [ ] Task deletion works
- [ ] Priority filter works
- [ ] Status filter works
- [ ] Overdue tasks highlighted

### Memory Search
- [ ] Search box accepts input
- [ ] Enter triggers search
- [ ] Local meeting results appear
- [ ] Cortex memory results appear (when connected)
- [ ] Knowledge base results appear (when connected)
- [ ] Results sorted by relevance
- [ ] Clicking local result opens meeting

### Export
- [ ] Markdown export downloads file
- [ ] Plain text export downloads file
- [ ] JSON export downloads file
- [ ] Redaction checkbox applies patterns
- [ ] File contains meeting content

## UI/UX

### Theme
- [ ] Light theme displays correctly
- [ ] Dark theme displays correctly
- [ ] System theme follows OS preference
- [ ] Theme toggle in settings works
- [ ] Theme persists after refresh

### Keyboard Shortcuts
- [ ] Ctrl/Cmd+K opens command palette
- [ ] Ctrl/Cmd+Shift+R starts/stops recording
- [ ] Ctrl/Cmd+Shift+P pauses/resumes
- [ ] Ctrl/Cmd+J toggles auto-scroll
- [ ] ESC closes modals/palette

### Command Palette
- [ ] Opens with Ctrl/Cmd+K
- [ ] Typing filters commands
- [ ] Selecting command executes action
- [ ] Navigation commands work
- [ ] Meeting commands work
- [ ] Settings commands work

### Responsive Design
- [ ] Sidebar collapses/expands
- [ ] Meeting view adapts to width
- [ ] Analysis panel scrollable
- [ ] Mobile-friendly touch targets

### Accessibility
- [ ] Focus indicators visible
- [ ] Keyboard navigation works
- [ ] Color contrast sufficient
- [ ] Screen reader labels present

## Connection Status

### Cortex Connection
- [ ] Status indicator in sidebar
- [ ] Connected shows green dot
- [ ] Connecting shows yellow pulse
- [ ] Offline shows red dot
- [ ] Status in meeting header matches

### Network Handling
- [ ] App works when Cortex offline
- [ ] Analyze disabled when offline
- [ ] Memory search falls back to local
- [ ] Reconnection attempted automatically

## Android WebView (if applicable)

### JSBridge
- [ ] `bridge.isAndroidWebView()` returns correct value
- [ ] Audio focus requested on recording start
- [ ] Audio focus released on recording stop
- [ ] File save uses native method
- [ ] Share uses native share sheet
- [ ] Toast shows native toast

### Fallbacks
- [ ] File save falls back to browser download
- [ ] Share falls back to Web Share API
- [ ] Network check falls back to navigator.onLine

## Data Integrity

### Schema Versioning
- [ ] New meetings have current schema version
- [ ] Older schema meetings load correctly
- [ ] Schema migration warning shown if needed

### Data Validation
- [ ] Invalid JSON import rejected
- [ ] Missing required fields handled
- [ ] Malformed dates handled gracefully

## Performance

### Transcript Virtualization
- [ ] Long transcripts scroll smoothly
- [ ] Memory usage stable during long meetings
- [ ] No jank when adding segments

### IndexedDB
- [ ] Save operations complete quickly
- [ ] History view loads promptly
- [ ] Search results appear quickly

## Security

### Redaction
- [ ] Email patterns detected
- [ ] Phone patterns detected
- [ ] SSN patterns detected
- [ ] Custom patterns can be added
- [ ] Patterns can be disabled
- [ ] Redaction preview accurate

### Privacy
- [ ] No data sent without user action
- [ ] Clear consent for memory commit
- [ ] Local-first architecture respected

## Test Results

| Category | Pass | Fail | Notes |
|----------|------|------|-------|
| Offline Operation | | | |
| Recording | | | |
| Auto-save | | | |
| Persistence | | | |
| Editing | | | |
| Analysis | | | |
| Memory | | | |
| Tasks | | | |
| Search | | | |
| Export | | | |
| Theme | | | |
| Shortcuts | | | |
| Palette | | | |
| Connection | | | |
| Android | | | |
| Performance | | | |
| Security | | | |

**Tested By:** _________________  
**Date:** _________________  
**Build Version:** _________________
