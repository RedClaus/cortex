#!/bin/bash
# CortexAvatar System Diagnostics
# Usage: ./scripts/diagnose.sh [component]
# Components: all, stt, tts, a2a, memory, performance

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

pass() { echo -e "${GREEN}✓${NC} $1"; }
fail() { echo -e "${RED}✗${NC} $1"; }
warn() { echo -e "${YELLOW}⚠${NC} $1"; }
info() { echo -e "${BLUE}ℹ${NC} $1"; }

COMPONENT="${1:-all}"

header() {
    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo " $1"
    echo "═══════════════════════════════════════════════════════════════"
}

# ============================================================================
# CORE SERVICES CHECK
# ============================================================================
check_core_services() {
    header "CORE SERVICES"

    # CortexBrain Server
    if pgrep -f "cortex-server" > /dev/null 2>&1; then
        pass "CortexBrain server running"

        # Check A2A endpoint
        if curl -s http://localhost:8080/.well-known/agent-card.json > /dev/null 2>&1; then
            pass "A2A endpoint responding"
            AGENT_NAME=$(curl -s http://localhost:8080/.well-known/agent-card.json 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('name','unknown'))" 2>/dev/null || echo "unknown")
            info "Agent: $AGENT_NAME"
        else
            fail "A2A endpoint not responding"
        fi
    else
        fail "CortexBrain server not running"
        warn "Start with: /tmp/cortex-server"
    fi

    # CortexAvatar
    if pgrep -f "CortexAvatar" > /dev/null 2>&1; then
        pass "CortexAvatar running"
    else
        fail "CortexAvatar not running"
        warn "Start with: open build/bin/CortexAvatar.app"
    fi

    # dnet (optional)
    if curl -s http://localhost:9080/health > /dev/null 2>&1; then
        MODEL_LOADED=$(curl -s http://localhost:9080/health 2>/dev/null | python3 -c "import sys,json; print('yes' if json.load(sys.stdin).get('model_loaded') else 'no')" 2>/dev/null || echo "unknown")
        if [ "$MODEL_LOADED" = "yes" ]; then
            pass "dnet cluster running (model loaded)"
        else
            warn "dnet cluster running (no model loaded)"
        fi
    else
        info "dnet cluster not running (optional)"
    fi

    # Ollama (optional)
    if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
        pass "Ollama running"
        MODELS=$(curl -s http://localhost:11434/api/tags 2>/dev/null | python3 -c "import sys,json; print(', '.join([m['name'] for m in json.load(sys.stdin).get('models',[])]))" 2>/dev/null || echo "unknown")
        info "Models: $MODELS"
    else
        info "Ollama not running (optional)"
    fi
}

# ============================================================================
# STT (Speech-to-Text) CHECK
# ============================================================================
check_stt() {
    header "VOICE INPUT (STT)"

    # Check microphone
    if [[ "$OSTYPE" == "darwin"* ]]; then
        MIC_COUNT=$(system_profiler SPAudioDataType 2>/dev/null | grep -c "Input Source" || echo "0")
        if [ "$MIC_COUNT" -gt 0 ]; then
            pass "Audio input devices found: $MIC_COUNT"
        else
            fail "No audio input devices found"
        fi
    fi

    # Check GROQ_API_KEY for Whisper
    if [ -n "$GROQ_API_KEY" ]; then
        pass "GROQ_API_KEY is set (Whisper STT available)"
    else
        # Check in .cortex/.env
        if grep -q "GROQ_API_KEY" ~/.cortex/.env 2>/dev/null; then
            pass "GROQ_API_KEY found in ~/.cortex/.env"
        else
            warn "GROQ_API_KEY not set - voice input may not work"
            info "Get a free key at: https://console.groq.com"
        fi
    fi

    # Check for recent STT logs
    if [ -f /tmp/cortex-server.log ]; then
        STT_ERRORS=$(grep -i "stt\|transcri\|whisper" /tmp/cortex-server.log 2>/dev/null | grep -i "error\|fail" | tail -3)
        if [ -n "$STT_ERRORS" ]; then
            warn "Recent STT errors found:"
            echo "$STT_ERRORS" | head -3
        fi
    fi
}

# ============================================================================
# TTS (Text-to-Speech) CHECK
# ============================================================================
check_tts() {
    header "VOICE OUTPUT (TTS)"

    # Check audio output
    if [[ "$OSTYPE" == "darwin"* ]]; then
        OUTPUT_VOL=$(osascript -e "output volume of (get volume settings)" 2>/dev/null || echo "0")
        if [ "$OUTPUT_VOL" -gt 0 ]; then
            pass "Audio output volume: $OUTPUT_VOL%"
        else
            fail "Audio output muted or volume at 0"
        fi
    fi

    # Check OpenAI TTS API key
    if [ -n "$OPENAI_API_KEY" ]; then
        pass "OPENAI_API_KEY is set (cloud TTS available)"
    else
        if grep -q "OPENAI_API_KEY" ~/.cortex/.env 2>/dev/null; then
            pass "OPENAI_API_KEY found in ~/.cortex/.env"
        else
            info "OPENAI_API_KEY not set - using local/browser TTS"
        fi
    fi

    # Check Piper TTS (local)
    if command -v piper &> /dev/null; then
        pass "Piper TTS installed"
        if [ -d ~/.cortex/piper-voices ]; then
            VOICE_COUNT=$(ls ~/.cortex/piper-voices/*.onnx 2>/dev/null | wc -l | tr -d ' ')
            info "Piper voices installed: $VOICE_COUNT"
        else
            warn "No Piper voices found in ~/.cortex/piper-voices"
        fi
    else
        info "Piper TTS not installed (optional local TTS)"
    fi

    # Check for TTS errors in logs
    if [ -f /tmp/cortex-server.log ]; then
        TTS_ERRORS=$(grep -i "tts\|speak\|synthesize" /tmp/cortex-server.log 2>/dev/null | grep -i "error\|fail" | tail -3)
        if [ -n "$TTS_ERRORS" ]; then
            warn "Recent TTS errors found:"
            echo "$TTS_ERRORS" | head -3
        fi
    fi
}

# ============================================================================
# A2A PROTOCOL CHECK
# ============================================================================
check_a2a() {
    header "A2A PROTOCOL"

    # Check endpoint
    if curl -s http://localhost:8080/.well-known/agent-card.json > /dev/null 2>&1; then
        pass "Agent card endpoint accessible"

        # Get capabilities
        CAPS=$(curl -s http://localhost:8080/.well-known/agent-card.json 2>/dev/null | python3 -c "
import sys,json
data = json.load(sys.stdin)
caps = data.get('capabilities', {})
print(f\"Streaming: {caps.get('streaming', False)}\")
print(f\"Stateful: {caps.get('stateTransitionHistory', False)}\")
" 2>/dev/null || echo "Could not parse capabilities")
        info "$CAPS"

        # Test message send
        RESPONSE=$(curl -s -X POST http://localhost:8080/ \
            -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","id":"test","method":"message/send","params":{"message":{"role":"user","parts":[{"kind":"text","text":"ping"}]}}}' 2>/dev/null)

        if echo "$RESPONSE" | grep -q "result"; then
            pass "A2A message/send working"
        else
            fail "A2A message/send failed"
            echo "$RESPONSE" | head -1
        fi
    else
        fail "A2A endpoint not accessible"
    fi
}

# ============================================================================
# MEMORY CHECK
# ============================================================================
check_memory() {
    header "MEMORY SYSTEMS"

    # Check knowledge database
    if [ -f ~/.cortex/knowledge.db ]; then
        DB_SIZE=$(ls -lh ~/.cortex/knowledge.db | awk '{print $5}')
        pass "Knowledge DB exists ($DB_SIZE)"

        # Check for recent entries
        ENTRY_COUNT=$(sqlite3 ~/.cortex/knowledge.db "SELECT COUNT(*) FROM memories;" 2>/dev/null || echo "0")
        info "Memory entries: $ENTRY_COUNT"
    else
        warn "Knowledge DB not found"
    fi

    # Check config
    if [ -f ~/.cortex/config.yaml ]; then
        pass "Config file exists"
    else
        warn "Config file not found at ~/.cortex/config.yaml"
    fi

    # Check API keys file
    if [ -f ~/.cortex/.env ]; then
        KEY_COUNT=$(grep -c "API_KEY" ~/.cortex/.env 2>/dev/null || echo "0")
        pass "API keys file exists ($KEY_COUNT keys)"
    elif [ -f ~/.cortex/api-keys.yaml ]; then
        pass "API keys file exists (YAML format)"
    else
        warn "No API keys file found"
    fi
}

# ============================================================================
# PERFORMANCE CHECK
# ============================================================================
check_performance() {
    header "PERFORMANCE"

    # Check response times from recent logs
    if [ -f /tmp/cortex-server.log ]; then
        RECENT_TIMES=$(grep "totalTime=" /tmp/cortex-server.log 2>/dev/null | tail -5 | sed 's/.*totalTime=\([0-9.]*\)s.*/\1/' | tr '\n' ' ')
        if [ -n "$RECENT_TIMES" ]; then
            info "Recent response times: ${RECENT_TIMES}seconds"
        fi
    fi

    # Check system resources
    if [[ "$OSTYPE" == "darwin"* ]]; then
        MEM_PRESSURE=$(memory_pressure 2>/dev/null | grep "System-wide memory free percentage" | awk '{print $NF}')
        if [ -n "$MEM_PRESSURE" ]; then
            info "Memory free: $MEM_PRESSURE"
        fi
    fi

    # Check for dnet performance
    if curl -s http://localhost:9080/health > /dev/null 2>&1; then
        info "dnet cluster available for local inference"
    fi
}

# ============================================================================
# MAIN
# ============================================================================

echo ""
echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║            CORTEXAVATAR DIAGNOSTIC REPORT                     ║"
echo "║                   $(date '+%Y-%m-%d %H:%M:%S')                        ║"
echo "╚═══════════════════════════════════════════════════════════════╝"

case "$COMPONENT" in
    all)
        check_core_services
        check_stt
        check_tts
        check_a2a
        check_memory
        check_performance
        ;;
    stt)
        check_stt
        ;;
    tts)
        check_tts
        ;;
    a2a)
        check_a2a
        ;;
    memory)
        check_memory
        ;;
    performance)
        check_performance
        ;;
    *)
        echo "Usage: $0 [all|stt|tts|a2a|memory|performance]"
        exit 1
        ;;
esac

header "SUMMARY"
echo "For detailed troubleshooting, see: docs/TROUBLESHOOTING.md"
echo ""
