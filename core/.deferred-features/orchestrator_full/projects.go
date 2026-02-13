package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/fingerprint"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/planning/tasks"
)

// ===========================================================================
// PROJECT FINGERPRINT & PLANNING METHODS
// ===========================================================================

// DetectProject performs project-level fingerprinting for the given path.
// This includes detecting project type, language, framework, and git information.
func (o *Orchestrator) DetectProject(ctx context.Context, path string) (*fingerprint.Fingerprint, error) {
	if o.fpDetector == nil {
		return nil, fmt.Errorf("fingerprint detector not initialized")
	}
	return o.fpDetector.DetectProject(ctx, path)
}

// GetProjectContext returns the current project context from the most recent fingerprint.
// This is useful for REST endpoints that need to query the current project state.
// Returns nil if no project has been detected yet.
func (o *Orchestrator) GetProjectContext() *fingerprint.Fingerprint {
	// Note: In the current implementation, fingerprinting happens per-request
	// via the fingerprintStage. This method would need to be enhanced to
	// cache the most recent fingerprint if we want persistent project context.
	// For now, callers should use DetectProject directly.
	return nil
}

// ApplyProjectContext enhances a prompt with project-specific context.
// This adds information about the detected project type, language, framework, etc.
func (o *Orchestrator) ApplyProjectContext(prompt string, fp *fingerprint.Fingerprint) string {
	if fp == nil {
		return prompt
	}

	var sb strings.Builder
	sb.WriteString(prompt)

	// Add project context section
	if fp.ProjectType != "" && fp.ProjectType != fingerprint.ProjectUnknown {
		sb.WriteString("\n\n## Project Context\n")
		sb.WriteString(fmt.Sprintf("- Type: %s\n", fp.ProjectType))

		if fp.ProjectRoot != "" {
			sb.WriteString(fmt.Sprintf("- Root: %s\n", fp.ProjectRoot))
		}

		if fp.PackageFile != "" {
			sb.WriteString(fmt.Sprintf("- Package File: %s\n", fp.PackageFile))
		}

		// Add Git information
		if fp.GitBranch != "" {
			dirty := ""
			if fp.GitDirty {
				dirty = " (uncommitted changes)"
			}
			sb.WriteString(fmt.Sprintf("- Git Branch: %s%s\n", fp.GitBranch, dirty))
		}

		// Add runtime versions
		if len(fp.RuntimeVersions) > 0 {
			sb.WriteString("- Runtime Versions:\n")
			for runtime, version := range fp.RuntimeVersions {
				sb.WriteString(fmt.Sprintf("  - %s: %s\n", runtime, version))
			}
		}
	}

	return sb.String()
}

// DecomposeTask breaks down a complex request into subtasks using the task manager.
// This is useful for planning multi-step operations.
// NOTE: This is a placeholder - actual task decomposition would require an LLM
// to analyze the request and create appropriate subtasks.
func (o *Orchestrator) DecomposeTask(ctx context.Context, request string, projectID string) ([]*tasks.Task, error) {
	if o.taskManager == nil {
		return nil, fmt.Errorf("task manager not initialized")
	}

	// TODO: In a full implementation, this would:
	// 1. Send the request to an LLM with a task decomposition prompt
	// 2. Parse the LLM response into Task objects
	// 3. Create tasks in the database with dependencies
	// 4. Return the created tasks
	//
	// For now, this is a stub that returns an empty list.
	// The actual implementation would require cognitive/planning integration.

	log := logging.Global()
	log.Warn("[Orchestrator] DecomposeTask called but not fully implemented - requires LLM integration")

	return []*tasks.Task{}, nil
}

// ExecutePlan executes a sequence of tasks in dependency order.
// This orchestrates the execution of a pre-defined task plan.
func (o *Orchestrator) ExecutePlan(ctx context.Context, projectID string) error {
	if o.taskManager == nil {
		return fmt.Errorf("task manager not initialized")
	}

	log := logging.Global()
	log.Info("[Orchestrator] Executing task plan for project: %s", projectID)

	// Get tasks in execution order (topological sort)
	orderedTasks, err := o.taskManager.GetExecutionOrder(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get execution order: %w", err)
	}

	if len(orderedTasks) == 0 {
		log.Info("[Orchestrator] No tasks to execute")
		return nil
	}

	log.Info("[Orchestrator] Executing %d tasks in order", len(orderedTasks))

	// Execute each task in order
	for i, task := range orderedTasks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip already completed tasks
		if task.Status == tasks.StatusDone {
			log.Debug("[Orchestrator] Task %s already done, skipping", task.ID)
			continue
		}

		log.Info("[Orchestrator] Executing task %d/%d: %s", i+1, len(orderedTasks), task.Title)

		// Mark task as in progress
		if err := o.taskManager.Update(ctx, task.ID, map[string]interface{}{
			"status": tasks.StatusDoing,
		}); err != nil {
			log.Warn("[Orchestrator] Failed to update task status: %v", err)
		}

		// Execute the task via the normal orchestrator pipeline
		// The task description becomes the input to Process
		startTime := time.Now()
		resp, err := o.ProcessSimple(ctx, task.Description)
		duration := time.Since(startTime)

		if err != nil {
			log.Error("[Orchestrator] Task %s failed: %v", task.ID, err)
			// Mark as blocked and continue to next task
			if updateErr := o.taskManager.Update(ctx, task.ID, map[string]interface{}{
				"status": tasks.StatusBlocked,
			}); updateErr != nil {
				log.Warn("[Orchestrator] Failed to update task status: %v", updateErr)
			}
			continue
		}

		// Mark task as complete
		actualTime := int(duration.Minutes())
		if err := o.taskManager.Update(ctx, task.ID, map[string]interface{}{
			"status":      tasks.StatusDone,
			"actualTime":  actualTime,
			"completedAt": time.Now(),
		}); err != nil {
			log.Warn("[Orchestrator] Failed to update task completion: %v", err)
		}

		log.Info("[Orchestrator] Task %s completed (took %s)", task.ID, duration)

		// Optional: publish task completion event
		if o.eventBus != nil {
			// Could publish a TaskCompletedEvent here if we define one in the bus package
			log.Debug("[Orchestrator] Task completed, output: %s", truncateString(resp.Content, 100))
		}
	}

	log.Info("[Orchestrator] Task plan execution completed")
	return nil
}

// SuggestNextTask suggests the next task to work on based on dependencies and priority.
func (o *Orchestrator) SuggestNextTask(ctx context.Context, projectID string) (*tasks.Task, error) {
	if o.taskManager == nil {
		return nil, fmt.Errorf("task manager not initialized")
	}
	return o.taskManager.SuggestNext(ctx, projectID)
}
