package agent

import (
	"context"
	"github.com/cortexhub/cortex-gateway/internal/channel"
	"github.com/cortexhub/cortex-gateway/internal/inference"
)

type AgentLoop struct {
	router *inference.Router
}

func NewAgentLoop(router *inference.Router) *AgentLoop {
	return &AgentLoop{router: router}
}

func (a *AgentLoop) Process(ctx context.Context, msg channel.InboundMessage, adapter channel.ChannelAdapter) {
	// Build context
	prompt := "You are a helpful assistant. User: " + msg.Content
	// Call LLM
	resp, err := a.router.Infer("", &inference.Request{Prompt: prompt})
	var content string
	if err != nil {
		content = "Error: " + err.Error()
	} else {
		content = resp.Content
	}
	// Send response
	out := &channel.Response{Content: content}
	adapter.SendMessage(msg.UserID, out)
}
