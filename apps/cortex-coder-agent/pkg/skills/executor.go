// Package skills provides skill management for the Cortex Coder Agent
package skills

import (
	"context"
	"fmt"
	"strings"

	"github.com/RedClaus/cortex-coder-agent/pkg/cortexbrain"
)

// Executor handles skill execution
type Executor struct {
	registry *Registry
	client   *cortexbrain.Client
}

// NewExecutor creates a new skill executor
func NewExecutor(registry *Registry, client *cortexbrain.Client) *Executor {
	return &Executor{
		registry: registry,
		client:   client,
	}
}

// ExecutionResult represents the result of skill execution
type ExecutionResult struct {
	SkillName   string
	Prompt      string
	Response    string
	Context     *Context
	Success     bool
	Error       error
	ToolCalls   []ToolCall
	Metadata    map[string]interface{}
}

// ToolCall represents a tool call made during execution
type ToolCall struct {
	Tool    string
	Input   map[string]interface{}
	Output  string
	Error   error
}

// ExecutionOptions configures skill execution
type ExecutionOptions struct {
	SkillName string
	Trigger   string
	Selection string
	FilePath  string
	LineNumber int
	Command   string
	UserQuery string
	Args      []string
}

// Execute runs a skill and returns the result
func (e *Executor) Execute(ctx context.Context, opts ExecutionOptions) (*ExecutionResult, error) {
	result := &ExecutionResult{
		SkillName: opts.SkillName,
		Metadata:  make(map[string]interface{}),
	}

	// Find the skill
	var skill *Skill
	var err error

	switch {
	case opts.SkillName != "":
		skill, err = e.findSkillByName(opts.SkillName)
	case opts.Command != "":
		skill, err = e.findSkillByCommand(opts.Command)
	case opts.Selection != "":
		skill, err = e.findSkillForSelection()
	case opts.FilePath != "":
		skill, err = e.findSkillForFile(opts.FilePath)
	default:
		return nil, fmt.Errorf("no skill specified and no trigger matched")
	}

	if err != nil {
		result.Success = false
		result.Error = err
		return result, err
	}

	if skill == nil {
		return nil, fmt.Errorf("skill not found")
	}

	result.SkillName = skill.Name

	// Build context
	executionCtx := e.buildContext(opts)

	// Run pre-execution hooks
	if err := e.runPreHooks(skill, executionCtx); err != nil {
		result.Success = false
		result.Error = err
		return result, err
	}

	// Parse template
	prompt, err := e.registry.GetTemplateEngine().Execute(skill.Template, executionCtx)
	if err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to execute template: %w", err)
		return result, result.Error
	}

	result.Prompt = prompt
	result.Context = executionCtx

	// If client is available, send to CortexBrain
	if e.client != nil {
		response, err := e.client.SendPrompt(ctx, cortexbrain.PromptRequest{
			SessionID: "",
			Prompt:    prompt,
			Context:   e.buildCortexContext(executionCtx),
			Skill:     skill.Name,
		})

		if err != nil {
			result.Success = false
			result.Error = err
			return result, err
		}

		result.Response = response.Content
		result.Metadata["response_type"] = response.Type
		result.Metadata["response_id"] = response.ID
	}

	result.Success = true

	// Run post-execution hooks
	if err := e.runPostHooks(skill, executionCtx, result); err != nil {
		result.Metadata["post_hook_error"] = err.Error()
	}

	return result, nil
}

// ExecuteByName executes a skill by name
func (e *Executor) ExecuteByName(ctx context.Context, name string, selection string) (*ExecutionResult, error) {
	return e.Execute(ctx, ExecutionOptions{
		SkillName: name,
		Selection: selection,
	})
}

// ExecuteByCommand executes a skill by command
func (e *Executor) ExecuteByCommand(ctx context.Context, cmd string, args []string) (*ExecutionResult, error) {
	return e.Execute(ctx, ExecutionOptions{
		Command: cmd,
		Args:    args,
	})
}

// ExecuteSelection executes a skill for the current selection
func (e *Executor) ExecuteSelection(ctx context.Context, selection string, filePath string, lineNumber int) (*ExecutionResult, error) {
	return e.Execute(ctx, ExecutionOptions{
		Selection:  selection,
		FilePath:   filePath,
		LineNumber: lineNumber,
	})
}

// findSkillByName finds a skill by exact name
func (e *Executor) findSkillByName(name string) (*Skill, error) {
	skill, ok := e.registry.Get(name)
	if !ok {
		// Try to find by partial match
		names := e.registry.GetSkillNames(name)
		if len(names) == 0 {
			return nil, fmt.Errorf("skill not found: %s", name)
		}
		if len(names) > 1 {
			return nil, fmt.Errorf("multiple skills match: %s", strings.Join(names, ", "))
		}
		skill, _ = e.registry.Get(names[0])
	}
	return skill, nil
}

// findSkillByCommand finds a skill by command name
func (e *Executor) findSkillByCommand(cmd string) (*Skill, error) {
	skill, ok := e.registry.GetByCommand(cmd)
	if !ok {
		return nil, fmt.Errorf("no skill found for command: /%s", cmd)
	}
	return skill, nil
}

// findSkillForSelection finds a skill for text selection
func (e *Executor) findSkillForSelection() (*Skill, error) {
	skills := e.registry.FindForSelection()
	if len(skills) == 0 {
		return nil, fmt.Errorf("no skills found for selection trigger")
	}
	return skills[0], nil
}

// findSkillForFile finds a skill for a file
func (e *Executor) findSkillForFile(path string) (*Skill, error) {
	skills := e.registry.FindForFile(path)
	if len(skills) == 0 {
		return nil, fmt.Errorf("no skills found for file: %s", path)
	}
	return skills[0], nil
}

// buildContext builds the execution context from options
func (e *Executor) buildContext(opts ExecutionOptions) *Context {
	builder := NewContextBuilder()

	if opts.Selection != "" {
		builder.FromSelection(opts.Selection, opts.LineNumber)
	}

	if opts.FilePath != "" {
		builder.FromFile(opts.FilePath)
	}

	// Handle command execution context
	if opts.Command != "" {
		// UserQuery should be just the args, not the command name
		userQuery := strings.Join(opts.Args, " ")
		builder.FromCommand(userQuery, opts.Args)
	} else if opts.UserQuery != "" {
		builder.FromCommand(opts.UserQuery, opts.Args)
	}

	return builder.Build()
}

// buildCortexContext converts skill context to CortexBrain format
func (e *Executor) buildCortexContext(ctx *Context) map[string]interface{} {
	return map[string]interface{}{
		"code":         ctx.Code,
		"file_path":    ctx.FilePath,
		"package_name": ctx.PackageName,
		"project_type": ctx.ProjectType,
		"git_branch":   ctx.GitBranch,
		"selection":    ctx.Selection,
		"line_number":  ctx.LineNumber,
		"language":     ctx.Language,
		"function_name": ctx.FunctionName,
		"class_name":    ctx.ClassName,
		"project_path":  ctx.ProjectPath,
	}
}

// runPreHooks runs pre-execution hooks
func (e *Executor) runPreHooks(skill *Skill, ctx *Context) error {
	for _, hook := range skill.PreHooks {
		if err := e.executeHook(hook, ctx, nil); err != nil {
			return fmt.Errorf("pre-hook failed: %w", err)
		}
	}
	return nil
}

// runPostHooks runs post-execution hooks
func (e *Executor) runPostHooks(skill *Skill, ctx *Context, result *ExecutionResult) error {
	for _, hook := range skill.PostHooks {
		if err := e.executeHook(hook, ctx, result); err != nil {
			return fmt.Errorf("post-hook failed: %w", err)
		}
	}
	return nil
}

// executeHook executes a single hook
func (e *Executor) executeHook(hook Hook, ctx *Context, result *ExecutionResult) error {
	switch hook.Type {
	case "validation":
		return e.runValidationHook(hook, ctx, result)
	case "transformation":
		return e.runTransformationHook(hook, ctx, result)
	default:
		return nil
	}
}

// runValidationHook runs a validation hook
func (e *Executor) runValidationHook(hook Hook, ctx *Context, result *ExecutionResult) error {
	// Simple validation based on action
	switch hook.Action {
	case "require_selection":
		if ctx.Selection == "" {
			return fmt.Errorf("skill requires a code selection")
		}
	case "require_file":
		if ctx.FilePath == "" {
			return fmt.Errorf("skill requires a file context")
		}
	case "require_language":
		if ctx.Language == "" {
			return fmt.Errorf("skill requires language detection")
		}
	}
	return nil
}

// runTransformationHook runs a transformation hook
func (e *Executor) runTransformationHook(hook Hook, ctx *Context, result *ExecutionResult) error {
	// Transformation hooks could modify context or result
	// For now, just pass through
	return nil
}

// SetClient sets the CortexBrain client
func (e *Executor) SetClient(client *cortexbrain.Client) {
	e.client = client
}

// GetRegistry returns the executor's registry
func (e *Executor) GetRegistry() *Registry {
	return e.registry
}
