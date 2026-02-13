---
project: Cortex
component: Docs
phase: Build
date_created: 2026-01-07T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:39.551047
---

# Cortex-02 Avatar Protocol

## Overview

This document defines the communication protocol between the Cortex-02 cognitive system and the Cortex Avatar rendering system. The avatar acts as the visual embodiment of Cortex, translating cognitive states into facial expressions and behaviors.

## Connection

### Transport
- **Unix Socket**: `/tmp/cortex.sock` (default, preferred for local)
- **TCP**: `localhost:9090` (for remote/debugging)

### Message Format
All messages are JSON-encoded, newline-delimited.

```
{"timestamp":"2025-01-07T12:00:00Z","mode":"thinking",...}\n
```

## State Message Schema

### CortexState

```json
{
  "timestamp": "2025-01-07T12:00:00.123Z",
  
  "valence": 0.3,
  "arousal": 0.5,
  
  "attention_level": 0.8,
  "confidence": 0.7,
  "processing_load": 0.4,
  
  "mode": "thinking",
  
  "gaze_target": {
    "x": 0.0,
    "y": 0.1
  },
  
  "is_speaking": false,
  "visemes": []
}
```

### Field Descriptions

| Field | Type | Range | Description |
|-------|------|-------|-------------|
| `timestamp` | ISO8601 | - | Message timestamp |
| `valence` | float | -1 to +1 | Emotional valence (negative to positive) |
| `arousal` | float | 0 to 1 | Activation level (calm to excited) |
| `attention_level` | float | 0 to 1 | Focus intensity |
| `confidence` | float | 0 to 1 | Response confidence |
| `processing_load` | float | 0 to 1 | Computational effort |
| `mode` | string | enum | Current cognitive mode |
| `gaze_target` | object | -1 to 1 | Gaze direction (optional) |
| `is_speaking` | bool | - | Speech active |
| `visemes` | array | - | Lip-sync data (optional) |

### Cognitive Modes

| Mode | Description | Avatar Behavior |
|------|-------------|-----------------|
| `idle` | No active task | Neutral expression, subtle breathing |
| `listening` | Processing input | Attentive expression, eye contact |
| `thinking` | Reasoning/processing | Slight gaze up, brow furrow |
| `speaking` | Generating response | Confident, lip-sync active |
| `attentive` | Focused on user | Wide eyes, engaged expression |

## Viseme Schema

For lip-sync during speech:

```json
{
  "shape": "aa",
  "weight": 0.8,
  "duration": 0.1,
  "offset": 0.05
}
```

### Viseme Shapes

| Shape | Phonemes | Mouth Position |
|-------|----------|----------------|
| `sil` | silence | Closed |
| `PP` | p, b, m | Lips together |
| `FF` | f, v | Lower lip to teeth |
| `TH` | th | Tongue to teeth |
| `DD` | t, d | Tongue to alveolar |
| `kk` | k, g | Back tongue |
| `CH` | ch, j, sh | Teeth together |
| `SS` | s, z | Teeth together, narrow |
| `nn` | n, l | Tongue to palate |
| `RR` | r | Rounded |
| `aa` | a | Open wide |
| `E` | e | Slightly open |
| `I` | i | Teeth together, wide |
| `O` | o | Rounded, medium |
| `U` | u | Rounded, small |

## Expression Mapping

### Valence → Expression

```
Valence  Expression Effect
──────────────────────────────
-1.0     Full concern/frown
-0.5     Slight frown
 0.0     Neutral
+0.5     Slight smile
+1.0     Full smile
```

### Arousal → Expression

```
Arousal  Expression Effect
──────────────────────────────
0.0      Relaxed, droopy
0.3      Calm, neutral
0.5      Alert
0.7      Engaged, wide eyes
1.0      Highly activated
```

### Mode → Base Expression

| Mode | Primary Blendshapes | Intensity |
|------|---------------------|-----------|
| `idle` | None | 0% |
| `listening` | `browInnerUp`, `eyeWide*` | 15% |
| `thinking` | `browInnerUp`, `eyeLookUp*` | 25% |
| `speaking` | `mouthSmile*`, `cheekSquint*` | Confidence-scaled |
| `attentive` | `eyeWide*`, `browOuterUp*` | Attention-scaled |

## Update Rate

- **State Updates**: 10-30 Hz (Cortex sends as state changes)
- **Avatar Polling**: 60 Hz (avatar interpolates between states)
- **Interpolation Buffer**: 50ms (smooth out network jitter)

## Example Session

```
→ Avatar connects to unix:/tmp/cortex.sock
← {"timestamp":"...","mode":"idle","valence":0,"arousal":0.3,...}
← {"timestamp":"...","mode":"listening","attention_level":0.8,...}
← {"timestamp":"...","mode":"thinking","processing_load":0.6,...}
← {"timestamp":"...","mode":"speaking","is_speaking":true,"visemes":[...]}
← {"timestamp":"...","mode":"idle","valence":0.2,...}
```

## Error Handling

- **Connection Lost**: Avatar continues with last state, attempts reconnect
- **Invalid JSON**: Log error, skip message
- **Missing Fields**: Use defaults (valence=0, arousal=0.3, mode="idle")

## Future Extensions

- `emotion_labels`: Discrete emotion classification
- `head_pose`: Head rotation targets
- `gesture_id`: Trigger predefined animations
- `priority`: Message priority for queueing
