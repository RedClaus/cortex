---
project: Cortex
component: Docs
phase: Design
date_created: 2026-02-10T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-11T01:40:42.685882
---

# CortexBrain Emotional Intelligence Enhancement PRD

## Executive Summary

Enhance CortexBrain to understand emotion, intent, and context like Tavus's PALs (Personal Affective Links). This PRD outlines how to leverage existing lobes and add new capabilities for truly contextual conversations.

---

## Tavus Architecture (Reference)

Tavus uses three specialized models:

| Model | Purpose | CortexBrain Equivalent |
|-------|---------|----------------------|
| **Raven** (Perception) | Emotion, intent, body language, context | EmotionLobe + TheoryOfMindLobe |
| **Sparrow** (Conversation) | Turn-taking, timing, rhythm, flow | *New: ConversationFlowLobe* |
| **Phoenix** (Rendering) | Visual expression, lip-sync | N/A (text-based) |

Key Tavus features:
- Sub-600ms response latency
- 32,000-token context window
- Persistent memory across sessions
- Ambient awareness queries ("Is user confused?")

---

## Current CortexBrain Capabilities

### ✅ Already Implemented

1. **EmotionLobe** (`lobes/emotion.go`)
   - Text sentiment analysis (keyword-based)
   - LLM-based emotion detection
   - Voice emotion fusion (multimodal)
   - Response tone suggestions (supportive, calming, enthusiastic)

2. **TheoryOfMindLobe** (`lobes/theory_of_mind.go`)
   - User expertise level detection
   - Communication preference inference
   - Goal extraction
   - Intent classification (info-seeking, action-requesting)

3. **MemoryLobe** (`lobes/memory.go`)
   - Conversation history search
   - Context retrieval

4. **Blackboard System**
   - Shared state between lobes
   - UserState (mood, preferred tone, expertise)
   - Memory context

### ❌ Current Problem

**The Pinky compatibility handler (`pinky_compat.go`) bypasses these lobes entirely!**

- Uses direct tool execution instead of Brain.Process
- No emotion detection
- No intent understanding
- No conversation context
- Each query is independent

---

## Enhancement Design

### Phase 1: Context-Aware Pinky Handler (1 week)

Add conversation context and basic emotion awareness to `pinky_compat.go`:

```go
type PinkyCompatHandler struct {
    brain           *brain.Executive
    executor        *agent.Executor
    log             *logging.Logger

    // NEW: Context management
    conversations   map[string]*ConversationContext  // userID -> context
    emotionLobe     *lobes.EmotionLobe
    intentLobe      *lobes.TheoryOfMindLobe
}

type ConversationContext struct {
    Messages       []Message
    LastEmotion    *lobes.EmotionResult
    LastIntent     *lobes.UserModel
    LastActivity   time.Time
    UserPrefs      map[string]string
}
```

**Key Changes:**
1. Maintain conversation history per user
2. Run EmotionLobe on each input
3. Run TheoryOfMindLobe to detect intent
4. Adjust response based on detected emotion/intent

### Phase 2: Ambient Awareness (1 week)

Add Tavus-style "ambient queries" - proactive checks on user state:

```go
type AmbientQuery struct {
    Question  string  // "Is user confused?"
    Threshold float64 // Trigger if confidence > threshold
    Action    string  // "clarify", "slow_down", "offer_help"
}

var defaultAmbientQueries = []AmbientQuery{
    {"Is user frustrated?", 0.7, "acknowledge_and_help"},
    {"Is user confused?", 0.6, "clarify_and_explain"},
    {"Is user in a hurry?", 0.8, "be_concise"},
    {"Does user need emotional support?", 0.7, "be_supportive"},
}
```

### Phase 3: Conversational Flow (2 weeks)

Create a new `ConversationFlowLobe` inspired by Sparrow:

```go
// ConversationFlowLobe manages conversation rhythm and pacing
type ConversationFlowLobe struct {
    llm LLMProvider
}

type FlowAnalysis struct {
    TurnType        string   // "question", "statement", "follow_up", "clarification"
    ExpectedNext    string   // What type of response is expected
    TopicContinuity float64  // 0-1, is this on same topic?
    ShouldAsk       bool     // Should we ask a clarifying question?
    SuggestedLength string   // "brief", "moderate", "detailed"
}
```

### Phase 4: Persistent Memory (2 weeks)

Enhance memory system for cross-session context:

```go
type UserProfile struct {
    UserID              string
    Name                string
    Preferences         map[string]string
    CommunicationStyle  string
    ExpertiseAreas      []string
    PastTopics          []string
    EmotionalPatterns   map[string]float64  // emotion -> frequency
    LastInteraction     time.Time
}
```

---

## Implementation Priority

### Immediate (This Week)

1. **Add conversation history to `pinky_compat.go`**
   - Store last 10 messages per user
   - Include in tool detection prompts
   - Enable follow-up query understanding

2. **Integrate EmotionLobe**
   - Run quick sentiment analysis on each input
   - Adjust response tone based on detected emotion
   - Track emotional trends across conversation

### Short-term (2 weeks)

3. **Integrate TheoryOfMindLobe**
   - Detect user intent from query
   - Infer expertise level
   - Adjust response complexity

4. **Add ambient awareness**
   - Check for frustration/confusion signals
   - Proactively offer help when detected

### Medium-term (1 month)

5. **Create ConversationFlowLobe**
   - Manage multi-turn conversations
   - Handle topic continuity
   - Know when to ask vs answer

6. **Persistent user profiles**
   - Remember user preferences across sessions
   - Track communication patterns
   - Personalize responses

---

## Technical Requirements

### Latency Targets (like Tavus)
- Emotion detection: < 50ms (use quick analysis)
- Intent inference: < 100ms
- Full response: < 600ms

### Context Window
- Maintain 10 recent messages minimum
- Support up to 32K tokens for complex queries

### Memory
- Redis for conversation state
- SQLite/PostgreSQL for persistent profiles

---

## Metrics & Success Criteria

| Metric | Current | Target |
|--------|---------|--------|
| Follow-up query success | 0% | > 80% |
| Emotion detection accuracy | N/A | > 70% |
| Intent classification accuracy | N/A | > 75% |
| Response appropriateness | N/A | > 85% |
| User satisfaction (subjective) | N/A | Positive |

---

## References

- [Tavus Emotional AI API](https://www.tavus.io/post/emotional-ai)
- [Tavus Research](https://www.tavus.io/research)
- [CortexBrain EmotionLobe](../CortexBrain/pkg/brain/lobes/emotion.go)
- [CortexBrain TheoryOfMindLobe](../CortexBrain/pkg/brain/lobes/theory_of_mind.go)

---

## Next Steps

1. [ ] Implement conversation context in `pinky_compat.go`
2. [ ] Add quick emotion analysis to handler
3. [ ] Test with Pinky WebUI
4. [ ] Create ConversationFlowLobe
5. [ ] Add persistent user profiles
