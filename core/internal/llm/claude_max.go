package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ClaudeMaxProvider implements the Provider interface using claude.ai Max subscription
type ClaudeMaxProvider struct {
	baseProvider
	cookie       string
	conversationID string
}

// NewClaudeMaxProvider creates a new Claude Max provider using web cookies
func NewClaudeMaxProvider(cfg *ProviderConfig) *ClaudeMaxProvider {
	// Get cookie from environment variable
	cookie := os.Getenv("CLAUDE_MAX_COOKIE")
	if cookie == "" {
		// Try to get from config
		if cfg.APIKey != "" {
			cookie = cfg.APIKey
		}
	}
	
	return &ClaudeMaxProvider{
		baseProvider: newBaseProvider(cfg, "claude-max"),
		cookie:       cookie,
	}
}

// Chat sends a chat request to Claude Max via the unofficial API
func (p *ClaudeMaxProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.cookie == "" {
		return nil, fmt.Errorf("Claude Max cookie not configured. Set CLAUDE_MAX_COOKIE environment variable")
	}

	start := time.Now()

	// Build the prompt from messages
	var prompt string
	if req.SystemPrompt != "" {
		prompt = req.SystemPrompt + "\n\n"
	}
	for _, msg := range req.Messages {
		prompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}

	// Create Python script to use the unofficial API
	pythonScript := fmt.Sprintf(`
import sys
sys.path.insert(0, '/tmp/Claude-API')
from claude_api import Client
import json

cookie = '''%s'''
prompt = '''%s'''
conversation_id = '%s'

try:
    claude_api = Client(cookie)
    
    # Create new conversation if needed
    if not conversation_id:
        new_chat = claude_api.create_new_chat()
        conversation_id = new_chat['uuid']
    
    # Send message
    response = claude_api.send_message(prompt, conversation_id)
    
    # Output as JSON
    result = {
        'content': response,
        'conversation_id': conversation_id,
        'success': True
    }
    print(json.dumps(result))
except Exception as e:
    result = {
        'error': str(e),
        'success': False
    }
    print(json.dumps(result))
`, p.cookie, strings.ReplaceAll(prompt, "'", "\\'"), p.conversationID)

	// Execute Python script
	cmd := exec.CommandContext(ctx, "python3", "-c", pythonScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("execute claude-api script: %w, output: %s", err, string(output))
	}

	// Parse the JSON response
	var result struct {
		Content        string `json:"content"`
		ConversationID string `json:"conversation_id"`
		Success        bool   `json:"success"`
		Error          string `json:"error"`
	}
	
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w, output: %s", err, string(output))
	}

	if !result.Success {
		return nil, fmt.Errorf("claude-api error: %s", result.Error)
	}

	// Store conversation ID for future use
	p.conversationID = result.ConversationID

	// Estimate tokens (rough approximation)
	promptTokens := len(prompt) / 4
	completionTokens := len(result.Content) / 4

	return &ChatResponse{
		Content:          result.Content,
		Model:            "claude-max",
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TokensUsed:       promptTokens + completionTokens,
		Duration:         time.Since(start),
		FinishReason:     "stop",
	}, nil
}

// ValidateModel checks if the model is supported
func (p *ClaudeMaxProvider) ValidateModel(model string) error {
	// Claude Max uses the latest models automatically
	return nil
}
