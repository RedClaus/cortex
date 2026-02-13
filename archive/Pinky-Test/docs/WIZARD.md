---
project: Cortex
component: Docs
phase: Ideation
date_created: 2026-02-08T10:57:49
source: ServerProjectsMac
librarian_indexed: 2026-02-08T12:13:09.200656
---

# Pinky Setup Wizard

The Pinky setup wizard (`pinky --wizard`) provides an interactive, step-by-step configuration experience for first-time users.

## Wizard Steps

### Step 1: Brain Mode Selection

Choose how Pinky should operate:

- **Embedded** (default): Single binary that runs locally with built-in inference
- **Remote**: Connect to a separate CortexBrain server instance

### Step 2: API Keys Configuration

Configure AI provider credentials:

| Provider | Purpose | Required |
|----------|---------|----------|
| **Ollama** | Local inference | Optional (defaults to localhost:11434) |
| **Anthropic** | Claude models | Optional |
| **OpenAI** | GPT models | Optional |
| **Groq** | Fast inference | Optional |

> **Note**: At minimum, Ollama is recommended for local-only operation. Cloud providers require API keys.

### Step 3: Model Selection (Model Picker)

Configure inference lanes and select models for each.

#### Auto-Routing

Toggle automatic lane selection based on task complexity:
- **ON**: Pinky automatically routes simple queries to the local lane, standard tasks to the fast lane, and complex analysis to the smart lane
- **OFF**: Always uses the default lane

Toggle with `Tab` key.

#### Lane Configuration

For each configured lane (local, fast, smart), you can:
- View available models from the lane's engine
- Select a specific model for that lane
- See model descriptions

**Navigation**:
- `←/→` - Switch between lanes
- `↑/↓` - Navigate models within a lane
- `Enter` or `Space` - Select the highlighted model
- `d` - Toggle model details view
- `n` - Continue to next step

#### Model Sources

- **Ollama**: Models are fetched dynamically from your local Ollama instance (falls back to common defaults if unreachable)
- **Cloud Providers** (Anthropic, OpenAI, Groq): Static curated lists of available models

### Step 4: Channel Configuration

Enable and configure messaging channels:

- **Telegram**: Bot token from @BotFather
- **Discord**: Bot token from Discord Developer Portal
- **Slack**: Bot token from Slack API

### Step 5: Permission Level

Select the default permission tier:

| Tier | Description |
|------|-------------|
| **Unrestricted** | Execute all tools automatically |
| **Some Restrictions** | Auto-approve low-risk, prompt for high-risk |
| **Restricted** | Ask before every tool execution |

### Step 6: Persona Selection

Choose Pinky's personality:

- **Professional**: Clear, concise, formal
- **Casual**: Friendly, conversational
- **Mentor**: Patient, educational, explains concepts
- **Minimalist**: Terse, just the facts

### Step 7: Confirmation

Review all settings before saving:

- Shows summary of brain mode, channels, permissions, and persona
- Press `y` or `Enter` to save to `~/.pinky/config.yaml`
- Press `n` to restart the wizard

## Post-Wizard

After completing the wizard:

```bash
# Start TUI mode
./pinky --tui

# Start server mode (WebUI + channels)
./pinky
```

## Re-running the Wizard

To reconfigure Pinky:

```bash
./pinky --wizard
```

This will overwrite your existing `~/.pinky/config.yaml`.

## Manual Configuration

For advanced configuration, edit `~/.pinky/config.yaml` directly:

```yaml
inference:
  default_lane: fast
  autollm: true
  lanes:
    local:
      engine: ollama
      model: llama3.2:3b
      url: http://localhost:11434
    fast:
      engine: groq
      model: llama-3.1-8b-instant
      api_key: ${GROQ_API_KEY}
    smart:
      engine: anthropic
      model: claude-3-5-sonnet-20241022
      api_key: ${ANTHROPIC_API_KEY}
```

## Changing Models After Setup

Use the TUI settings panel:

```bash
./pinky --tui
# Then type: /settings
```

Or manually edit the config file and restart Pinky.
