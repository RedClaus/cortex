package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/normanking/cortex/internal/cognitive"
)

// initializePipeline initializes the cognitive pipeline if enabled.
// Call this in the New() function after creating the Prism instance.
func (p *Prism) initializePipeline() error {
	// Check if pipeline is enabled in config
	if !p.appConfig.Cognitive.Enabled {
		p.log.Info("[Prism] Cognitive pipeline disabled in config")
		p.usePipeline = false
		return nil
	}

	// Get Claude API key
	p.providerKeysMu.RLock()
	claudeAPIKey := p.providerKeys["anthropic"]
	p.providerKeysMu.RUnlock()

	// Check environment variable as fallback
	if claudeAPIKey == "" {
		claudeAPIKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	if claudeAPIKey == "" {
		p.log.Warn("[Prism] Claude API key not configured, pipeline will use fallback mode")
	}

	// Create pipeline configuration
	pipelineConfig := cognitive.DefaultConfig()
	pipelineConfig.FastModel = "llama3.2:1b"
	pipelineConfig.SmartModel = p.appConfig.Cognitive.FrontierModel
	if pipelineConfig.SmartModel == "" {
		pipelineConfig.SmartModel = "claude-sonnet-4-20250514"
	}

	// Create factory
	factory := cognitive.NewPipelineFactory(pipelineConfig)

	// Create pipeline
	ollamaURL := p.appConfig.Cognitive.OllamaURL
	if ollamaURL == "" {
		ollamaURL = "http://127.0.0.1:11434"
	}

	pipeline, err := factory.CreatePipelineWithFallback(ollamaURL, claudeAPIKey, p.modeTracker)
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}

	p.pipeline = pipeline
	p.pipelineMetrics = cognitive.NewPipelineMetrics()
	p.usePipeline = true

	p.log.Info("[Prism] Cognitive pipeline initialized (fast: %s, smart: %s)",
		pipelineConfig.FastModel, pipelineConfig.SmartModel)

	return nil
}

// processChatWithPipeline processes a chat request through the cognitive pipeline.
// This is called from handleChat when usePipeline is true.
func (p *Prism) processChatWithPipeline(
	ctx context.Context,
	req *ChatRequest,
	conversation *Conversation,
	systemPrompt string,
) (*ChatResponse, error) {
	// Convert conversation messages to pipeline history
	history := make([]cognitive.Message, 0, len(conversation.Messages))
	for _, msg := range conversation.Messages {
		history = append(history, cognitive.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Build pipeline request
	pipelineReq := &cognitive.PipelineRequest{
		SystemPrompt:   systemPrompt,
		Message:        req.Message,
		History:        history,
		ConversationID: conversation.ID,
	}

	// Optional: force lane based on request
	if req.Lane != "" && req.Lane != "auto" {
		lane := cognitive.Lane(req.Lane)
		pipelineReq.ForceLane = &lane
	}

	// Process through pipeline
	pipelineResp, err := p.pipeline.Process(ctx, pipelineReq)
	if err != nil {
		// Pipeline error - record metric and return error
		p.pipelineMetrics.RecordError()
		return nil, fmt.Errorf("pipeline processing failed: %w", err)
	}

	// Record metrics
	latency := time.Duration(pipelineResp.LatencyMs) * time.Millisecond
	p.pipelineMetrics.RecordRequest(pipelineResp.Lane, latency, pipelineResp.ThinkingUsed)

	// Build chat message
	now := time.Now()
	assistantMsgID := fmt.Sprintf("msg-%s-assistant", uuid.New().String())

	assistantMsg := ChatMessage{
		ID:        assistantMsgID,
		Role:      "assistant",
		Content:   pipelineResp.Content,
		Timestamp: now,
		Model:     pipelineResp.Model,
		PersonaID: req.PersonaID,
		Routing: &ChatRouting{
			Lane:      string(pipelineResp.Lane),
			Model:     pipelineResp.Model,
			LatencyMs: pipelineResp.LatencyMs,
			Reason:    buildRoutingReason(pipelineResp),
		},
	}

	// Add to conversation
	p.conversationsMu.Lock()
	conversation.Messages = append(conversation.Messages, assistantMsg)
	conversation.UpdatedAt = now
	p.conversationsMu.Unlock()

	// Build response
	response := &ChatResponse{
		Message:        assistantMsg,
		ConversationID: conversation.ID,
		Routing: ChatRouting{
			Lane:      string(pipelineResp.Lane),
			Model:     pipelineResp.Model,
			LatencyMs: pipelineResp.LatencyMs,
			Reason:    buildRoutingReason(pipelineResp),
		},
	}

	return response, nil
}

// buildRoutingReason creates a human-readable routing explanation.
func buildRoutingReason(resp *cognitive.PipelineResponse) string {
	var reason strings.Builder

	if resp.Lane == cognitive.FastLane {
		reason.WriteString("Fast lane - using local model for quick response")
	} else {
		reason.WriteString("Smart lane - using frontier model for complex reasoning")
	}

	if resp.ThinkingUsed {
		reason.WriteString(" with extended thinking")
	}

	return reason.String()
}

// handlePipelineMetrics handles GET /api/v1/metrics/pipeline.
func (p *Prism) handlePipelineMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if !p.usePipeline {
		p.writeJSON(w, http.StatusOK, map[string]string{
			"status": "disabled",
			"reason": "Cognitive pipeline is not enabled",
		})
		return
	}

	stats := p.pipelineMetrics.GetStats()
	p.writeJSON(w, http.StatusOK, stats)
}

// updateSystemMetrics updates the system metrics to include pipeline stats.
// This should be called from getSystemMetrics().
func (p *Prism) updateSystemMetrics(metrics *SystemMetrics) {
	if !p.usePipeline {
		return
	}

	stats := p.pipelineMetrics.GetStats()
	metrics.LocalRate = stats.LocalRate
}
