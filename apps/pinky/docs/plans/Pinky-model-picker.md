---
project: Cortex
component: Agents
phase: Ideation
date_created: 2026-02-08T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-08T12:13:22.394962
---

# Pinky Model Picker & Auto-Routing Settings

**Created:** 2026-02-08
**Status:** Planned
**Priority:** P1 - High

---

## Overview

Add model picker and auto-routing settings to Pinky, accessible from both the setup wizard (first-run configuration) and the TUI (runtime settings).

### Goals

1. **Auto-routing toggle**: Turn on/off automatic lane selection based on task complexity
2. **Model picker for cloud providers**: When Anthropic/OpenAI/Groq is chosen, show available models
3. **Model picker for local models**: When Ollama is chosen, show available local models
4. **Two access points**: Setup wizard AND TUI settings panel

---

## Current State Analysis

### Config Structure (`internal/config/config.go`)

```go
type InferenceConfig struct {
    DefaultLane string          `yaml:"default_lane"`
    AutoLLM     bool            `yaml:"autollm"`      // Already exists!
    Lanes       map[string]Lane `yaml:"lanes"`
}

type Lane struct {
    Engine string `yaml:"engine"`  // ollama, openai, anthropic, groq
    Model  string `yaml:"model"`
    URL    string `yaml:"url,omitempty"`
    APIKey string `yaml:"api_key,omitempty"`
}
```

### EmbeddedBrain APIs (`internal/brain/embedded.go`)

Already implemented:
- `SetAutoLLM(enabled bool)` / `GetAutoLLM() bool`
- `SetLane(name string) error` / `GetLane() string`
- `GetLanes() []LaneInfo`

Missing:
- `SetModel(lane, model string) error` - Update model for a specific lane
- `GetAvailableModels(engine string) ([]string, error)` - Fetch models from provider

### Wizard (`internal/wizard/wizard.go`)

Current steps:
1. `StepBrain` - Choose brain mode (embedded/remote)
2. `StepAPIKeys` - Enter API keys for providers
3. `StepChannels` - Configure Telegram/Discord/Slack
4. `StepPermissions` - Set permission tier
5. `StepPersona` - Choose persona
6. `StepConfirm` - Review and save
7. `StepDone` - Complete

Missing: Model picker step between StepAPIKeys and StepChannels

### TUI (`internal/tui/model.go`)

Current focus states:
- `FocusChat` - Main chat interface
- `FocusApproval` - Permission approval dialog
- `FocusHelp` - Help overlay

Missing: `FocusSettings` for runtime configuration

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Model Provider Service                        │
│                   internal/models/provider.go                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────┐ │
│  │   Ollama     │  │  Anthropic   │  │   OpenAI     │  │   Groq   │ │
│  │  (Dynamic)   │  │   (Static)   │  │   (Static)   │  │ (Static) │ │
│  │ GET /api/tags│  │  Model List  │  │  Model List  │  │Model List│ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────┘ │
│                                                                      │
└───────────────────────────────┬─────────────────────────────────────┘
                                │
                ┌───────────────┴───────────────┐
                │                               │
                ▼                               ▼
┌───────────────────────────┐   ┌───────────────────────────────────┐
│   Setup Wizard            │   │   TUI Settings Panel              │
│   internal/wizard/        │   │   internal/tui/settings.go        │
├───────────────────────────┤   ├───────────────────────────────────┤
│ - StepModelPicker (new)   │   │ - FocusSettings (new)             │
│ - Auto-routing toggle     │   │ - /settings command               │
│ - Model selection per lane│   │ - Ctrl+, hotkey                   │
└───────────────────────────┘   └───────────────────────────────────┘
                │                               │
                └───────────────┬───────────────┘
                                │
                                ▼
                ┌───────────────────────────────┐
                │   EmbeddedBrain               │
                │   internal/brain/embedded.go  │
                ├───────────────────────────────┤
                │ - SetModel(lane, model)       │
                │ - SetAutoLLM(enabled)         │
                │ - Persist to config.yaml      │
                └───────────────────────────────┘
```

---

## Implementation Plan

### Phase 1: Model Provider Service

**Files to create:**
- `internal/models/provider.go` - Main provider interface and registry
- `internal/models/ollama.go` - Ollama model fetcher
- `internal/models/static.go` - Static model lists for cloud providers

**Tasks:**

#### 1.1 Create Provider Interface

```go
// internal/models/provider.go

package models

type ModelInfo struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    ContextSize int    `json:"context_size,omitempty"`
}

type Provider interface {
    Engine() string
    ListModels(ctx context.Context) ([]ModelInfo, error)
    ValidateModel(model string) bool
}

type Registry struct {
    providers map[string]Provider
}

func NewRegistry() *Registry
func (r *Registry) Register(p Provider)
func (r *Registry) Get(engine string) (Provider, bool)
func (r *Registry) ListModels(ctx context.Context, engine string) ([]ModelInfo, error)
```

#### 1.2 Implement Ollama Provider (Dynamic)

```go
// internal/models/ollama.go

type OllamaProvider struct {
    baseURL string
}

func (p *OllamaProvider) Engine() string { return "ollama" }

func (p *OllamaProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
    // GET http://localhost:11434/api/tags
    // Parse response and return ModelInfo slice
}
```

**Ollama API Response:**
```json
{
  "models": [
    {
      "name": "llama3.2:3b",
      "modified_at": "2024-01-15T...",
      "size": 2000000000
    }
  ]
}
```

#### 1.3 Implement Static Providers

```go
// internal/models/static.go

var AnthropicModels = []ModelInfo{
    {ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", ContextSize: 200000},
    {ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", ContextSize: 200000},
    {ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", ContextSize: 200000},
    {ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", ContextSize: 200000},
}

var OpenAIModels = []ModelInfo{
    {ID: "gpt-4o", Name: "GPT-4o", ContextSize: 128000},
    {ID: "gpt-4o-mini", Name: "GPT-4o Mini", ContextSize: 128000},
    {ID: "gpt-4-turbo", Name: "GPT-4 Turbo", ContextSize: 128000},
    {ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", ContextSize: 16385},
}

var GroqModels = []ModelInfo{
    {ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", ContextSize: 128000},
    {ID: "llama-3.1-8b-instant", Name: "Llama 3.1 8B Instant", ContextSize: 128000},
    {ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", ContextSize: 32768},
    {ID: "gemma2-9b-it", Name: "Gemma 2 9B", ContextSize: 8192},
}
```

#### 1.4 Unit Tests

```go
// internal/models/provider_test.go

func TestOllamaProvider_ListModels(t *testing.T)
func TestStaticProvider_ListModels(t *testing.T)
func TestRegistry_Get(t *testing.T)
```

---

### Phase 2: EmbeddedBrain Enhancements

**Files to modify:**
- `internal/brain/embedded.go` - Add SetModel method
- `internal/config/config.go` - Add Save method if not present

**Tasks:**

#### 2.1 Add SetModel Method

```go
// internal/brain/embedded.go

func (b *EmbeddedBrain) SetModel(laneName, model string) error {
    b.mu.Lock()
    defer b.mu.Unlock()

    lane, ok := b.lanes[laneName]
    if !ok {
        return fmt.Errorf("lane %s not found", laneName)
    }

    // Update the lane's model
    lane.Model = model
    b.lanes[laneName] = lane

    // Persist to config
    return b.persistConfig()
}

func (b *EmbeddedBrain) persistConfig() error {
    // Update config.yaml with new model settings
}
```

#### 2.2 Add GetModelsForLane Method

```go
func (b *EmbeddedBrain) GetModelsForLane(ctx context.Context, laneName string) ([]models.ModelInfo, error) {
    lane, ok := b.lanes[laneName]
    if !ok {
        return nil, fmt.Errorf("lane %s not found", laneName)
    }

    return b.modelRegistry.ListModels(ctx, lane.Engine)
}
```

---

### Phase 3: Wizard Model Picker Step

**Files to modify:**
- `internal/wizard/wizard.go` - Add StepModelPicker
- `internal/wizard/steps.go` - Model picker UI (create if needed)

**Tasks:**

#### 3.1 Add StepModelPicker Constant

```go
const (
    StepBrain Step = iota
    StepAPIKeys
    StepModelPicker  // NEW
    StepChannels
    StepPermissions
    StepPersona
    StepConfirm
    StepDone
)
```

#### 3.2 Implement Model Picker View

```go
func (m *Model) viewModelPicker() string {
    var sb strings.Builder

    sb.WriteString(titleStyle.Render("Model Selection"))
    sb.WriteString("\n\n")

    // Auto-routing toggle
    autoStatus := "OFF"
    if m.config.Inference.AutoLLM {
        autoStatus = "ON"
    }
    sb.WriteString(fmt.Sprintf("Auto-routing: [%s]\n", autoStatus))
    sb.WriteString("  (Automatically selects lane based on task complexity)\n\n")

    // Model selection for each lane
    for laneName, lane := range m.config.Inference.Lanes {
        sb.WriteString(fmt.Sprintf("── %s Lane (%s) ──\n", strings.Title(laneName), lane.Engine))

        models, _ := m.modelRegistry.ListModels(context.Background(), lane.Engine)
        for i, model := range models {
            cursor := "  "
            if model.ID == lane.Model {
                cursor = "> "
            }
            sb.WriteString(fmt.Sprintf("%s%s\n", cursor, model.Name))
        }
        sb.WriteString("\n")
    }

    return sb.String()
}
```

#### 3.3 Handle Model Selection Input

```go
func (m *Model) handleModelPickerInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "up", "k":
        m.modelCursor--
    case "down", "j":
        m.modelCursor++
    case "enter", " ":
        // Select the current model for the current lane
        m.selectModel()
    case "tab":
        // Toggle auto-routing
        m.config.Inference.AutoLLM = !m.config.Inference.AutoLLM
    case "n", "enter":
        // Move to next step
        m.step = StepChannels
    }
    return m, nil
}
```

---

### Phase 4: TUI Settings Panel

**Files to create:**
- `internal/tui/settings.go` - Settings panel component

**Files to modify:**
- `internal/tui/model.go` - Add FocusSettings state
- `internal/tui/update.go` - Handle settings input
- `internal/tui/view.go` - Render settings panel

**Tasks:**

#### 4.1 Add FocusSettings State

```go
// internal/tui/model.go

type Focus int

const (
    FocusChat Focus = iota
    FocusApproval
    FocusHelp
    FocusSettings  // NEW
)
```

#### 4.2 Create Settings Component

```go
// internal/tui/settings.go

package tui

type SettingsPanel struct {
    brain         brain.Brain
    modelRegistry *models.Registry

    autoLLM       bool
    lanes         []brain.LaneInfo

    focusedLane   int
    focusedModel  int

    width, height int
}

func NewSettingsPanel(brain brain.Brain, registry *models.Registry) *SettingsPanel

func (s *SettingsPanel) Init() tea.Cmd
func (s *SettingsPanel) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (s *SettingsPanel) View() string

func (s *SettingsPanel) SetSize(width, height int)
```

#### 4.3 Settings Panel UI

```
┌─────────────────────────────────────────────────┐
│  ⚙️  Inference Settings              [Esc] Close │
├─────────────────────────────────────────────────┤
│                                                 │
│  Auto-routing: [ ON ] [OFF]    ← Tab to toggle  │
│  ─────────────────────────────────────────────  │
│                                                 │
│  ▶ Local Lane (ollama)          ← Enter to edit │
│    Model: llama3.2:3b                           │
│                                                 │
│    Fast Lane (groq)                             │
│    Model: llama-3.1-8b-instant                  │
│                                                 │
│    Smart Lane (anthropic)                       │
│    Model: claude-3-5-sonnet-20241022            │
│                                                 │
│  ─────────────────────────────────────────────  │
│  ↑↓ Navigate  Enter Select  Tab Toggle  Esc Back│
└─────────────────────────────────────────────────┘
```

#### 4.4 Wire Up /settings Command

```go
// internal/tui/update.go or main.go

func handleSlashCommand(msg string, brn brain.Brain, t *tui.TUI) bool {
    // ... existing commands ...

    case cmd == "/settings":
        t.ShowSettings()
        return true
}
```

#### 4.5 Add Ctrl+, Hotkey

```go
// internal/tui/update.go

case tea.KeyCtrlComma:
    m.focus = FocusSettings
    return m, nil
```

---

### Phase 5: Integration & Testing

**Tasks:**

#### 5.1 Integration Tests

```go
// internal/tui/settings_test.go

func TestSettingsPanel_ToggleAutoLLM(t *testing.T)
func TestSettingsPanel_ChangeModel(t *testing.T)
func TestSettingsPanel_PersistsToConfig(t *testing.T)
```

#### 5.2 End-to-End Testing

1. Run wizard, verify model picker step works
2. Change models in wizard, verify saved to config.yaml
3. Run TUI, open settings with `/settings`
4. Toggle auto-routing, verify brain state changes
5. Change model, verify persisted

#### 5.3 Documentation

- Update README with new commands
- Add `/settings` to help text
- Document auto-routing behavior

---

## File Summary

### New Files

| File | Purpose |
|------|---------|
| `internal/models/provider.go` | Provider interface and registry |
| `internal/models/ollama.go` | Ollama dynamic model fetcher |
| `internal/models/static.go` | Static model lists for cloud providers |
| `internal/models/provider_test.go` | Unit tests |
| `internal/tui/settings.go` | Settings panel component |
| `internal/tui/settings_test.go` | Settings panel tests |

### Modified Files

| File | Changes |
|------|---------|
| `internal/brain/embedded.go` | Add SetModel, GetModelsForLane, persistConfig |
| `internal/config/config.go` | Ensure Save method exists |
| `internal/wizard/wizard.go` | Add StepModelPicker, model picker view/input |
| `internal/tui/model.go` | Add FocusSettings state |
| `internal/tui/update.go` | Handle settings input, /settings command |
| `internal/tui/view.go` | Render settings panel |
| `cmd/pinky/main.go` | Wire up /settings command handler |

---

## Timeline Estimate

| Phase | Scope |
|-------|-------|
| Phase 1 | Model Provider Service |
| Phase 2 | EmbeddedBrain Enhancements |
| Phase 3 | Wizard Model Picker Step |
| Phase 4 | TUI Settings Panel |
| Phase 5 | Integration & Testing |

---

## Success Criteria

- [ ] Wizard shows model picker step after API keys
- [ ] User can toggle auto-routing in wizard
- [ ] User can select model for each lane in wizard
- [ ] TUI has `/settings` command that opens settings panel
- [ ] User can toggle auto-routing in TUI
- [ ] User can change model for each lane in TUI
- [ ] All changes persist to `~/.pinky/config.yaml`
- [ ] Ollama models are fetched dynamically
- [ ] Cloud provider models show from static lists
- [ ] Unit tests pass
- [ ] Integration tests pass

---

## Open Questions

1. **Ollama connection failure**: What to show if Ollama isn't running?
   - Recommendation: Show "Ollama not available" with retry option

2. **Model validation**: Should we validate model exists before saving?
   - Recommendation: Yes, at least for Ollama; for cloud providers trust the static list

3. **Hot reload**: Should model changes apply immediately or require restart?
   - Recommendation: Apply immediately for better UX

4. **Default selection**: What if user doesn't have any models installed?
   - Recommendation: Show guidance to install models via `ollama pull`
