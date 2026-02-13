package autollm

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ClaudeMaxAdapter provides access to Claude models through the Claude Code CLI
// using a Claude Max subscription instead of API keys
type ClaudeMaxAdapter struct {
	claudePath string
}

// NewClaudeMaxAdapter creates a new adapter for Claude Max subscription
func NewClaudeMaxAdapter() (*ClaudeMaxAdapter, error) {
	// Find claude executable
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		// Try npm global path
		claudePath, err = exec.LookPath("claude-code")
		if err != nil {
			return nil, fmt.Errorf("claude CLI not found. Install with: npm install -g @anthropic-ai/claude-code")
		}
	}

	return &ClaudeMaxAdapter{
		claudePath: claudePath,
	}, nil
}

// ChatCompletion sends a chat request to Claude using the subscription
func (c *ClaudeMaxAdapter) ChatCompletion(ctx context.Context, messages []Message, model string) (string, error) {
	// Prepare the prompt
	prompt := c.formatMessages(messages)

	// Build command with print flag for non-interactive output
	cmd := exec.CommandContext(ctx, c.claudePath, "--print", "--model", model)

	// Set environment to force Claude Max subscription usage
	cmd.Env = append(os.Environ(),
		"CLAUDE_CODE_ENTRYPOINT=max-alias",
		"CLAUDE_USE_SUBSCRIPTION=true",
		"CLAUDE_BYPASS_BALANCE_CHECK=true",
	)

	// Remove ANTHROPIC_API_KEY to force subscription usage
	for i, env := range cmd.Env {
		if strings.HasPrefix(env, "ANTHROPIC_API_KEY=") {
			cmd.Env = append(cmd.Env[:i], cmd.Env[i+1:]...)
			break
		}
	}

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start claude: %w", err)
	}

	// Send the prompt
	go func() {
		defer stdin.Close()
		fmt.Fprint(stdin, prompt)
	}()

	// Read the response
	scanner := bufio.NewScanner(stdout)
	var response strings.Builder
	for scanner.Scan() {
		response.WriteString(scanner.Text())
		response.WriteString("\n")
	}

	// Wait for command to finish
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("claude command failed: %w", err)
	}

	return strings.TrimSpace(response.String()), nil
}

// formatMessages converts messages to a prompt string
func (c *ClaudeMaxAdapter) formatMessages(messages []Message) string {
	var prompt strings.Builder

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			prompt.WriteString(fmt.Sprintf("System: %s\n\n", msg.Content))
		case "user":
			prompt.WriteString(fmt.Sprintf("Human: %s\n\n", msg.Content))
		case "assistant":
			prompt.WriteString(fmt.Sprintf("Assistant: %s\n\n", msg.Content))
		}
	}

	// Add final prompt for Claude to respond
	prompt.WriteString("Assistant: ")

	return prompt.String()
}

// StreamChatCompletion streams a chat response from Claude
func (c *ClaudeMaxAdapter) StreamChatCompletion(ctx context.Context, messages []Message, model string) (<-chan string, error) {
	// For now, we'll implement a simple non-streaming version
	// that sends the full response at once
	ch := make(chan string, 1)

	go func() {
		defer close(ch)

		response, err := c.ChatCompletion(ctx, messages, model)
		if err != nil {
			ch <- fmt.Sprintf("Error: %v", err)
			return
		}

		ch <- response
	}()

	return ch, nil
}

// IsAvailable checks if Claude CLI is installed and authenticated
func (c *ClaudeMaxAdapter) IsAvailable() bool {
	cmd := exec.Command(c.claudePath, "--version")
	cmd.Env = append(os.Environ(),
		"CLAUDE_USE_SUBSCRIPTION=true",
	)

	// Remove API key
	for i, env := range cmd.Env {
		if strings.HasPrefix(env, "ANTHROPIC_API_KEY=") {
			cmd.Env = append(cmd.Env[:i], cmd.Env[i+1:]...)
			break
		}
	}

	err := cmd.Run()
	return err == nil
}
