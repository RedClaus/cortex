// Package prompts provides prompt management and chaining for Cortex.
// chain.go implements multi-step prompt chains with variable interpolation.
package prompts

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

// ChainExecutor provides LLM inference for chain execution.
// This interface allows the chain to work with any LLM provider.
type ChainExecutor interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// PromptChain represents a multi-step prompt execution flow.
// Brain Alignment: This mirrors phased execution in the cognitive pipeline.
type PromptChain struct {
	// Metadata
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description" json:"description"`

	// Chain steps
	Steps []ChainStep `yaml:"steps" json:"steps"`

	// Global context available to all steps
	Context map[string]any `yaml:"-" json:"-"`

	// Execution options
	Options ChainOptions `yaml:"options" json:"options"`
}

// ChainStep represents a single step in a prompt chain.
type ChainStep struct {
	// Step identification
	Name string `yaml:"name" json:"name"`

	// Prompt template (Go text/template syntax)
	Template string `yaml:"template" json:"template"`

	// Variable to store output in (accessible by later steps)
	OutputVar string `yaml:"output" json:"output"`

	// Optional condition to skip this step
	Condition string `yaml:"condition,omitempty" json:"condition,omitempty"`

	// System prompt for this step (optional, overrides chain default)
	SystemPrompt string `yaml:"system_prompt,omitempty" json:"system_prompt,omitempty"`

	// Step-specific options
	MaxTokens   int     `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`
	Temperature float64 `yaml:"temperature,omitempty" json:"temperature,omitempty"`

	// Retry configuration
	MaxRetries int `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`

	// Transform output before storing (optional)
	Transform string `yaml:"transform,omitempty" json:"transform,omitempty"`
}

// ChainOptions configures chain execution behavior.
type ChainOptions struct {
	// Default system prompt for all steps
	SystemPrompt string `yaml:"system_prompt,omitempty" json:"system_prompt,omitempty"`

	// Stop on first error vs continue
	StopOnError bool `yaml:"stop_on_error" json:"stop_on_error"`

	// Overall timeout for chain execution
	TimeoutSeconds int `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`

	// Max parallel steps (0 = sequential)
	MaxParallel int `yaml:"max_parallel,omitempty" json:"max_parallel,omitempty"`

	// Enable step-by-step logging
	Verbose bool `yaml:"verbose" json:"verbose"`
}

// ChainResult contains the result of chain execution.
type ChainResult struct {
	// Final output (from last step or specified output var)
	Output string `json:"output"`

	// All step outputs
	StepOutputs map[string]string `json:"step_outputs"`

	// Execution metadata
	TotalSteps    int           `json:"total_steps"`
	ExecutedSteps int           `json:"executed_steps"`
	SkippedSteps  int           `json:"skipped_steps"`
	TotalDuration time.Duration `json:"total_duration"`
	StepDurations map[string]time.Duration `json:"step_durations"`

	// Error information (if any)
	Error     error  `json:"error,omitempty"`
	FailedStep string `json:"failed_step,omitempty"`
}

// NewChain creates a new empty prompt chain.
func NewChain(name string) *PromptChain {
	return &PromptChain{
		Name:    name,
		Version: "1.0",
		Steps:   make([]ChainStep, 0),
		Context: make(map[string]any),
		Options: ChainOptions{
			StopOnError: true,
		},
	}
}

// LoadChain loads a chain from YAML content.
func LoadChain(yamlContent []byte) (*PromptChain, error) {
	var chain PromptChain
	if err := yaml.Unmarshal(yamlContent, &chain); err != nil {
		return nil, fmt.Errorf("parse chain YAML: %w", err)
	}

	if chain.Context == nil {
		chain.Context = make(map[string]any)
	}

	return &chain, nil
}

// AddStep adds a step to the chain.
func (c *PromptChain) AddStep(name, templateStr, outputVar string) *PromptChain {
	c.Steps = append(c.Steps, ChainStep{
		Name:      name,
		Template:  templateStr,
		OutputVar: outputVar,
	})
	return c
}

// WithContext sets a context variable.
func (c *PromptChain) WithContext(key string, value any) *PromptChain {
	c.Context[key] = value
	return c
}

// SetInput sets the primary input for the chain.
func (c *PromptChain) SetInput(input string) *PromptChain {
	c.Context["Input"] = input
	c.Context["input"] = input
	return c
}

// Execute runs the chain with the given executor.
func (c *PromptChain) Execute(ctx context.Context, executor ChainExecutor) (*ChainResult, error) {
	startTime := time.Now()

	result := &ChainResult{
		StepOutputs:   make(map[string]string),
		StepDurations: make(map[string]time.Duration),
		TotalSteps:    len(c.Steps),
	}

	// Apply timeout if configured
	if c.Options.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(c.Options.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	// Execute each step
	for i, step := range c.Steps {
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			result.FailedStep = step.Name
			result.TotalDuration = time.Since(startTime)
			return result, ctx.Err()
		default:
		}

		// Check condition
		if step.Condition != "" {
			shouldRun, err := c.evaluateCondition(step.Condition)
			if err != nil {
				if c.Options.StopOnError {
					result.Error = fmt.Errorf("condition eval for step %s: %w", step.Name, err)
					result.FailedStep = step.Name
					result.TotalDuration = time.Since(startTime)
					return result, result.Error
				}
				result.SkippedSteps++
				continue
			}
			if !shouldRun {
				result.SkippedSteps++
				continue
			}
		}

		// Execute step
		stepStart := time.Now()
		output, err := c.executeStep(ctx, executor, step)
		stepDuration := time.Since(stepStart)
		result.StepDurations[step.Name] = stepDuration

		if err != nil {
			if c.Options.StopOnError {
				result.Error = err
				result.FailedStep = step.Name
				result.TotalDuration = time.Since(startTime)
				return result, err
			}
			// Continue but record the error
			result.StepOutputs[step.OutputVar] = fmt.Sprintf("ERROR: %v", err)
			continue
		}

		// Store output
		result.StepOutputs[step.OutputVar] = output
		c.Context[step.OutputVar] = output
		result.ExecutedSteps++

		// Last step output becomes the final output
		if i == len(c.Steps)-1 {
			result.Output = output
		}
	}

	result.TotalDuration = time.Since(startTime)

	// If a specific output var is specified, use that
	if finalVar, ok := c.Context["_output_var"].(string); ok {
		if val, exists := result.StepOutputs[finalVar]; exists {
			result.Output = val
		}
	}

	return result, nil
}

// executeStep runs a single chain step.
func (c *PromptChain) executeStep(ctx context.Context, executor ChainExecutor, step ChainStep) (string, error) {
	// Interpolate template
	prompt, err := c.interpolate(step.Template)
	if err != nil {
		return "", fmt.Errorf("interpolate template: %w", err)
	}

	// Execute with retries
	maxRetries := step.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		output, err := executor.Complete(ctx, prompt)
		if err == nil {
			// Apply transform if specified
			if step.Transform != "" {
				output, err = c.applyTransform(output, step.Transform)
				if err != nil {
					return "", fmt.Errorf("transform output: %w", err)
				}
			}
			return output, nil
		}
		lastErr = err

		// Wait before retry (exponential backoff)
		if attempt < maxRetries-1 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}
	}

	return "", lastErr
}

// interpolate fills in template variables from context.
func (c *PromptChain) interpolate(templateStr string) (string, error) {
	// Handle simple {{.VarName}} syntax
	tmpl, err := template.New("prompt").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, c.Context); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// evaluateCondition evaluates a simple condition string.
// Supports: "varName", "!varName", "varName == 'value'", "varName != 'value'"
func (c *PromptChain) evaluateCondition(condition string) (bool, error) {
	condition = strings.TrimSpace(condition)

	// Negation
	if strings.HasPrefix(condition, "!") {
		result, err := c.evaluateCondition(condition[1:])
		return !result, err
	}

	// Equality check
	if strings.Contains(condition, "==") {
		parts := strings.SplitN(condition, "==", 2)
		if len(parts) == 2 {
			varName := strings.TrimSpace(parts[0])
			expected := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
			if val, ok := c.Context[varName]; ok {
				return fmt.Sprintf("%v", val) == expected, nil
			}
			return false, nil
		}
	}

	// Inequality check
	if strings.Contains(condition, "!=") {
		parts := strings.SplitN(condition, "!=", 2)
		if len(parts) == 2 {
			varName := strings.TrimSpace(parts[0])
			expected := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
			if val, ok := c.Context[varName]; ok {
				return fmt.Sprintf("%v", val) != expected, nil
			}
			return true, nil
		}
	}

	// Simple variable existence/truthiness
	if val, ok := c.Context[condition]; ok {
		switch v := val.(type) {
		case bool:
			return v, nil
		case string:
			return v != "", nil
		case int, int64, float64:
			return v != 0, nil
		default:
			return true, nil
		}
	}

	return false, nil
}

// applyTransform applies a transformation to the output.
// Supports: "trim", "lower", "upper", "first_line", "last_line", "extract:pattern"
func (c *PromptChain) applyTransform(output, transform string) (string, error) {
	switch {
	case transform == "trim":
		return strings.TrimSpace(output), nil

	case transform == "lower":
		return strings.ToLower(output), nil

	case transform == "upper":
		return strings.ToUpper(output), nil

	case transform == "first_line":
		lines := strings.Split(output, "\n")
		if len(lines) > 0 {
			return strings.TrimSpace(lines[0]), nil
		}
		return "", nil

	case transform == "last_line":
		lines := strings.Split(output, "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) != "" {
				return strings.TrimSpace(lines[i]), nil
			}
		}
		return "", nil

	case strings.HasPrefix(transform, "extract:"):
		pattern := strings.TrimPrefix(transform, "extract:")
		re, err := regexp.Compile(pattern)
		if err != nil {
			return "", fmt.Errorf("invalid regex pattern: %w", err)
		}
		matches := re.FindStringSubmatch(output)
		if len(matches) > 1 {
			return matches[1], nil // Return first capture group
		}
		if len(matches) > 0 {
			return matches[0], nil // Return full match
		}
		return "", nil

	default:
		return output, nil
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PREDEFINED CHAINS
// ─────────────────────────────────────────────────────────────────────────────

// ReasoningChain returns a multi-step reasoning chain.
func ReasoningChain() *PromptChain {
	return NewChain("reasoning").
		AddStep("understand", `Analyze the following request and identify:
1. The core question or problem
2. Required knowledge domains
3. Key constraints or requirements
4. Potential approaches

Request: {{.Input}}

Provide a structured analysis.`, "analysis").
		AddStep("reason", `Based on this analysis:
{{.analysis}}

Now work through the problem step by step:
1. Break it down into sub-problems
2. Address each sub-problem
3. Synthesize the results

Provide your reasoning.`, "reasoning").
		AddStep("conclude", `Review your reasoning:
{{.reasoning}}

Now provide a clear, concise final answer. Be direct and specific.`, "conclusion")
}

// CodingChain returns a chain for code generation tasks.
func CodingChain() *PromptChain {
	return NewChain("coding").
		AddStep("plan", `You need to write code for this task:
{{.Input}}

Before writing code, create a plan:
1. What functions/methods are needed?
2. What data structures should be used?
3. What are the edge cases to handle?
4. What tests would verify correctness?

Provide your implementation plan.`, "plan").
		AddStep("implement", `Based on this plan:
{{.plan}}

Now implement the code. Include:
- Clear comments explaining key logic
- Error handling
- Type hints or documentation

Write the complete implementation.`, "code").
		AddStep("review", `Review this code:
{{.code}}

Check for:
1. Bugs or logic errors
2. Missing edge case handling
3. Performance issues
4. Code style improvements

Provide the final, corrected code with any fixes applied.`, "final_code")
}

// SummarizationChain returns a chain for summarizing content.
func SummarizationChain() *PromptChain {
	return NewChain("summarization").
		AddStep("extract", `Read the following content and extract the key points:
{{.Input}}

List the main ideas, facts, and conclusions.`, "key_points").
		AddStep("synthesize", `Based on these key points:
{{.key_points}}

Write a concise summary that:
1. Captures the essential information
2. Maintains the original meaning
3. Is clear and well-organized

Target length: {{if .target_length}}{{.target_length}}{{else}}2-3 paragraphs{{end}}`, "summary")
}

// AnalysisChain returns a chain for analytical tasks.
func AnalysisChain() *PromptChain {
	return NewChain("analysis").
		AddStep("gather", `Examine the following and identify all relevant data points:
{{.Input}}

List every significant piece of information.`, "data_points").
		AddStep("pattern", `Looking at this data:
{{.data_points}}

Identify patterns, trends, and relationships. Note any anomalies or outliers.`, "patterns").
		AddStep("interpret", `Based on these patterns:
{{.patterns}}

Provide your interpretation:
1. What do these patterns mean?
2. What conclusions can be drawn?
3. What are the implications?
4. What uncertainties remain?`, "interpretation")
}

// CreativeChain returns a chain for creative tasks.
func CreativeChain() *PromptChain {
	return NewChain("creative").
		AddStep("brainstorm", `For this creative task:
{{.Input}}

Generate 5-7 diverse ideas. Be imaginative and explore different angles.
Don't filter - include both safe and unconventional ideas.`, "ideas").
		AddStep("develop", `From these ideas:
{{.ideas}}

Select the most promising {{if .num_to_develop}}{{.num_to_develop}}{{else}}2-3{{end}} and develop them further.
For each, add details, examples, and flesh out the concept.`, "developed").
		AddStep("refine", `Based on the developed concepts:
{{.developed}}

Create the final output. Make it polished, engaging, and complete.`, "output")
}
