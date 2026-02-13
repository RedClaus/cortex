// Package tasks provides LLM tools for task management operations.
// This implements the 5 essential tools from CR-007.
package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/tools"
)

// Tool types for task management
const (
	ToolTaskCreate     tools.ToolType = "task_create"
	ToolTaskList       tools.ToolType = "task_list"
	ToolTaskUpdate     tools.ToolType = "task_update"
	ToolTaskDependency tools.ToolType = "task_dependency"
	ToolTaskNext       tools.ToolType = "task_next"
)

// ═══════════════════════════════════════════════════════════════════════════════
// TASK CREATE TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// TaskCreateTool creates new tasks with validation.
type TaskCreateTool struct {
	manager *Manager
}

// NewTaskCreateTool creates a new task creation tool.
func NewTaskCreateTool(m *Manager) *TaskCreateTool {
	return &TaskCreateTool{manager: m}
}

// Name returns the tool identifier.
func (t *TaskCreateTool) Name() tools.ToolType {
	return ToolTaskCreate
}

// Execute creates a new task from the request parameters.
func (t *TaskCreateTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()

	// Parse parameters
	task := &Task{}

	if title, ok := req.Params["title"].(string); ok && title != "" {
		task.Title = title
	} else if req.Input != "" {
		task.Title = req.Input
	} else {
		return &tools.ToolResult{
			Tool:      ToolTaskCreate,
			Success:   false,
			Error:     "title is required",
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
		}, nil
	}

	if desc, ok := req.Params["description"].(string); ok {
		task.Description = desc
	}

	if projectID, ok := req.Params["project_id"].(string); ok {
		task.ProjectID = projectID
	}

	if priority, ok := req.Params["priority"].(string); ok {
		switch strings.ToLower(priority) {
		case "low":
			task.Priority = PriorityLow
		case "medium":
			task.Priority = PriorityMedium
		case "high":
			task.Priority = PriorityHigh
		case "critical":
			task.Priority = PriorityCritical
		default:
			task.Priority = PriorityMedium
		}
	}

	if estimate, ok := req.Params["estimate"].(float64); ok {
		task.Estimate = int(estimate)
	}

	if tags, ok := req.Params["tags"].([]interface{}); ok {
		for _, tag := range tags {
			if s, ok := tag.(string); ok {
				task.Tags = append(task.Tags, s)
			}
		}
	}

	// Handle dependencies (will be validated during creation)
	if deps, ok := req.Params["dependencies"].([]interface{}); ok {
		for _, dep := range deps {
			if s, ok := dep.(string); ok {
				task.Dependencies = append(task.Dependencies, s)
			}
		}
	}

	// Create the task
	if err := t.manager.Create(ctx, task); err != nil {
		return &tools.ToolResult{
			Tool:      ToolTaskCreate,
			Success:   false,
			Error:     err.Error(),
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
		}, nil
	}

	// Format output
	output := fmt.Sprintf("Created task: %s\nID: %s\nPriority: %s\nStatus: %s",
		task.Title, task.ID, task.Priority, task.Status)

	if len(task.Dependencies) > 0 {
		output += fmt.Sprintf("\nDependencies: %d", len(task.Dependencies))
	}

	return &tools.ToolResult{
		Tool:      ToolTaskCreate,
		Success:   true,
		Output:    output,
		Duration:  time.Since(start),
		RiskLevel: tools.RiskNone,
		Metadata: map[string]interface{}{
			"task_id":   task.ID,
			"title":     task.Title,
			"priority":  task.Priority,
			"status":    task.Status,
			"project_id": task.ProjectID,
		},
	}, nil
}

// Validate checks if the request is valid.
func (t *TaskCreateTool) Validate(req *tools.ToolRequest) error {
	title, hasTitle := req.Params["title"].(string)
	if !hasTitle && req.Input == "" {
		return fmt.Errorf("title is required")
	}
	if hasTitle && strings.TrimSpace(title) == "" {
		return fmt.Errorf("title cannot be empty")
	}
	return nil
}

// AssessRisk returns the risk level (always none for task creation).
func (t *TaskCreateTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskNone
}

// ═══════════════════════════════════════════════════════════════════════════════
// TASK LIST TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// TaskListTool lists tasks with filtering.
type TaskListTool struct {
	manager *Manager
}

// NewTaskListTool creates a new task listing tool.
func NewTaskListTool(m *Manager) *TaskListTool {
	return &TaskListTool{manager: m}
}

// Name returns the tool identifier.
func (t *TaskListTool) Name() tools.ToolType {
	return ToolTaskList
}

// Execute lists tasks based on filter parameters.
func (t *TaskListTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()

	opts := ListOptions{Limit: 20} // Default limit

	if projectID, ok := req.Params["project_id"].(string); ok {
		opts.ProjectID = projectID
	}

	if status, ok := req.Params["status"].(string); ok {
		switch strings.ToLower(status) {
		case "todo":
			opts.Status = StatusTodo
		case "doing":
			opts.Status = StatusDoing
		case "blocked":
			opts.Status = StatusBlocked
		case "done":
			opts.Status = StatusDone
		}
	}

	if priority, ok := req.Params["priority"].(string); ok {
		switch strings.ToLower(priority) {
		case "low":
			opts.Priority = PriorityLow
		case "medium":
			opts.Priority = PriorityMedium
		case "high":
			opts.Priority = PriorityHigh
		case "critical":
			opts.Priority = PriorityCritical
		}
	}

	if limit, ok := req.Params["limit"].(float64); ok && limit > 0 {
		opts.Limit = int(limit)
	}

	tasks, err := t.manager.List(ctx, opts)
	if err != nil {
		return &tools.ToolResult{
			Tool:      ToolTaskList,
			Success:   false,
			Error:     err.Error(),
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
		}, nil
	}

	// Format output
	var output strings.Builder
	if len(tasks) == 0 {
		output.WriteString("No tasks found")
	} else {
		output.WriteString(fmt.Sprintf("Found %d task(s):\n\n", len(tasks)))
		for i, task := range tasks {
			statusIcon := "○"
			switch task.Status {
			case StatusDoing:
				statusIcon = "◐"
			case StatusBlocked:
				statusIcon = "⊘"
			case StatusDone:
				statusIcon = "●"
			}

			output.WriteString(fmt.Sprintf("%d. %s [%s] %s\n", i+1, statusIcon, task.Priority, task.Title))
			output.WriteString(fmt.Sprintf("   ID: %s | Status: %s\n", task.ID, task.Status))

			if len(task.Dependencies) > 0 {
				output.WriteString(fmt.Sprintf("   Dependencies: %d\n", len(task.Dependencies)))
			}

			if i < len(tasks)-1 {
				output.WriteString("\n")
			}
		}
	}

	// Build task list for metadata
	taskList := make([]map[string]interface{}, len(tasks))
	for i, task := range tasks {
		taskList[i] = map[string]interface{}{
			"id":           task.ID,
			"title":        task.Title,
			"status":       task.Status,
			"priority":     task.Priority,
			"dependencies": task.Dependencies,
		}
	}

	return &tools.ToolResult{
		Tool:      ToolTaskList,
		Success:   true,
		Output:    output.String(),
		Duration:  time.Since(start),
		RiskLevel: tools.RiskNone,
		Metadata: map[string]interface{}{
			"count": len(tasks),
			"tasks": taskList,
		},
	}, nil
}

// Validate checks if the request is valid.
func (t *TaskListTool) Validate(req *tools.ToolRequest) error {
	return nil // No required parameters for listing
}

// AssessRisk returns the risk level (always none for task listing).
func (t *TaskListTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskNone
}

// ═══════════════════════════════════════════════════════════════════════════════
// TASK UPDATE TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// TaskUpdateTool updates existing tasks.
type TaskUpdateTool struct {
	manager *Manager
}

// NewTaskUpdateTool creates a new task update tool.
func NewTaskUpdateTool(m *Manager) *TaskUpdateTool {
	return &TaskUpdateTool{manager: m}
}

// Name returns the tool identifier.
func (t *TaskUpdateTool) Name() tools.ToolType {
	return ToolTaskUpdate
}

// Execute updates a task with the provided parameters.
func (t *TaskUpdateTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()

	// Get task ID
	taskID, ok := req.Params["task_id"].(string)
	if !ok || taskID == "" {
		taskID = req.Input
	}
	if taskID == "" {
		return &tools.ToolResult{
			Tool:      ToolTaskUpdate,
			Success:   false,
			Error:     "task_id is required",
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
		}, nil
	}

	// Build updates map
	updates := make(map[string]interface{})

	if title, ok := req.Params["title"].(string); ok && title != "" {
		updates["title"] = title
	}

	if desc, ok := req.Params["description"].(string); ok {
		updates["description"] = desc
	}

	if status, ok := req.Params["status"].(string); ok {
		switch strings.ToLower(status) {
		case "todo":
			updates["status"] = StatusTodo
		case "doing":
			updates["status"] = StatusDoing
		case "blocked":
			updates["status"] = StatusBlocked
		case "done":
			updates["status"] = StatusDone
			updates["completedAt"] = time.Now()
		}
	}

	if priority, ok := req.Params["priority"].(string); ok {
		switch strings.ToLower(priority) {
		case "low":
			updates["priority"] = PriorityLow
		case "medium":
			updates["priority"] = PriorityMedium
		case "high":
			updates["priority"] = PriorityHigh
		case "critical":
			updates["priority"] = PriorityCritical
		}
	}

	if estimate, ok := req.Params["estimate"].(float64); ok {
		updates["estimate"] = int(estimate)
	}

	if actualTime, ok := req.Params["actual_time"].(float64); ok {
		updates["actual_time"] = int(actualTime)
	}

	if len(updates) == 0 {
		return &tools.ToolResult{
			Tool:      ToolTaskUpdate,
			Success:   false,
			Error:     "no update fields provided",
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
		}, nil
	}

	// Apply updates
	if err := t.manager.Update(ctx, taskID, updates); err != nil {
		return &tools.ToolResult{
			Tool:      ToolTaskUpdate,
			Success:   false,
			Error:     err.Error(),
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
		}, nil
	}

	// Get updated task for output
	task, err := t.manager.Get(ctx, taskID)
	if err != nil {
		return &tools.ToolResult{
			Tool:      ToolTaskUpdate,
			Success:   true,
			Output:    fmt.Sprintf("Task %s updated successfully", taskID),
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
		}, nil
	}

	output := fmt.Sprintf("Updated task: %s\nID: %s\nStatus: %s\nPriority: %s",
		task.Title, task.ID, task.Status, task.Priority)

	return &tools.ToolResult{
		Tool:      ToolTaskUpdate,
		Success:   true,
		Output:    output,
		Duration:  time.Since(start),
		RiskLevel: tools.RiskNone,
		Metadata: map[string]interface{}{
			"task_id":  task.ID,
			"title":    task.Title,
			"status":   task.Status,
			"priority": task.Priority,
			"updated":  updates,
		},
	}, nil
}

// Validate checks if the request is valid.
func (t *TaskUpdateTool) Validate(req *tools.ToolRequest) error {
	taskID, ok := req.Params["task_id"].(string)
	if (!ok || taskID == "") && req.Input == "" {
		return fmt.Errorf("task_id is required")
	}
	return nil
}

// AssessRisk returns the risk level (low for task updates).
func (t *TaskUpdateTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskLow
}

// ═══════════════════════════════════════════════════════════════════════════════
// TASK DEPENDENCY TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// TaskDependencyTool manages task dependencies.
type TaskDependencyTool struct {
	manager *Manager
}

// NewTaskDependencyTool creates a new task dependency tool.
func NewTaskDependencyTool(m *Manager) *TaskDependencyTool {
	return &TaskDependencyTool{manager: m}
}

// Name returns the tool identifier.
func (t *TaskDependencyTool) Name() tools.ToolType {
	return ToolTaskDependency
}

// Execute adds or removes a task dependency.
func (t *TaskDependencyTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()

	// Get parameters
	taskID, ok := req.Params["task_id"].(string)
	if !ok || taskID == "" {
		return &tools.ToolResult{
			Tool:      ToolTaskDependency,
			Success:   false,
			Error:     "task_id is required",
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
		}, nil
	}

	dependsOnID, ok := req.Params["depends_on_id"].(string)
	if !ok || dependsOnID == "" {
		return &tools.ToolResult{
			Tool:      ToolTaskDependency,
			Success:   false,
			Error:     "depends_on_id is required",
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
		}, nil
	}

	// Determine action (default: add)
	action := "add"
	if a, ok := req.Params["action"].(string); ok {
		action = strings.ToLower(a)
	}

	var err error
	var output string

	switch action {
	case "add":
		err = t.manager.AddDependency(ctx, taskID, dependsOnID)
		if err == nil {
			output = fmt.Sprintf("Added dependency: task %s now depends on task %s", taskID, dependsOnID)
		}
	case "remove":
		err = t.manager.RemoveDependency(ctx, taskID, dependsOnID)
		if err == nil {
			output = fmt.Sprintf("Removed dependency: task %s no longer depends on task %s", taskID, dependsOnID)
		}
	default:
		return &tools.ToolResult{
			Tool:      ToolTaskDependency,
			Success:   false,
			Error:     fmt.Sprintf("invalid action: %s (use 'add' or 'remove')", action),
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
		}, nil
	}

	if err != nil {
		return &tools.ToolResult{
			Tool:      ToolTaskDependency,
			Success:   false,
			Error:     err.Error(),
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
		}, nil
	}

	return &tools.ToolResult{
		Tool:      ToolTaskDependency,
		Success:   true,
		Output:    output,
		Duration:  time.Since(start),
		RiskLevel: tools.RiskNone,
		Metadata: map[string]interface{}{
			"task_id":       taskID,
			"depends_on_id": dependsOnID,
			"action":        action,
		},
	}, nil
}

// Validate checks if the request is valid.
func (t *TaskDependencyTool) Validate(req *tools.ToolRequest) error {
	taskID, ok1 := req.Params["task_id"].(string)
	dependsOnID, ok2 := req.Params["depends_on_id"].(string)

	if !ok1 || taskID == "" {
		return fmt.Errorf("task_id is required")
	}
	if !ok2 || dependsOnID == "" {
		return fmt.Errorf("depends_on_id is required")
	}
	if taskID == dependsOnID {
		return fmt.Errorf("task cannot depend on itself")
	}
	return nil
}

// AssessRisk returns the risk level (low for dependency management).
func (t *TaskDependencyTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskLow
}

// ═══════════════════════════════════════════════════════════════════════════════
// TASK NEXT TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// TaskNextTool suggests the next task to work on.
type TaskNextTool struct {
	manager *Manager
}

// NewTaskNextTool creates a new task suggestion tool.
func NewTaskNextTool(m *Manager) *TaskNextTool {
	return &TaskNextTool{manager: m}
}

// Name returns the tool identifier.
func (t *TaskNextTool) Name() tools.ToolType {
	return ToolTaskNext
}

// Execute suggests the next task based on dependencies and priority.
func (t *TaskNextTool) Execute(ctx context.Context, req *tools.ToolRequest) (*tools.ToolResult, error) {
	start := time.Now()

	projectID, _ := req.Params["project_id"].(string)

	task, err := t.manager.SuggestNext(ctx, projectID)
	if err != nil {
		return &tools.ToolResult{
			Tool:      ToolTaskNext,
			Success:   false,
			Error:     err.Error(),
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
		}, nil
	}

	if task == nil {
		return &tools.ToolResult{
			Tool:      ToolTaskNext,
			Success:   true,
			Output:    "No tasks available. All tasks are either completed, in progress, or blocked.",
			Duration:  time.Since(start),
			RiskLevel: tools.RiskNone,
			Metadata: map[string]interface{}{
				"suggestion": nil,
			},
		}, nil
	}

	// Build detailed output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Suggested next task:\n\n"))
	output.WriteString(fmt.Sprintf("  Title: %s\n", task.Title))
	output.WriteString(fmt.Sprintf("  ID: %s\n", task.ID))
	output.WriteString(fmt.Sprintf("  Priority: %s\n", task.Priority))
	output.WriteString(fmt.Sprintf("  Status: %s\n", task.Status))

	if task.Description != "" {
		output.WriteString(fmt.Sprintf("  Description: %s\n", task.Description))
	}

	if task.Estimate > 0 {
		output.WriteString(fmt.Sprintf("  Estimated time: %d minutes\n", task.Estimate))
	}

	// Check for blockers (should be none, but include for completeness)
	blockers, _ := t.manager.GetBlockers(ctx, task.ID)
	if len(blockers) > 0 {
		output.WriteString(fmt.Sprintf("\n  Warning: Task has %d incomplete dependencies\n", len(blockers)))
	}

	return &tools.ToolResult{
		Tool:      ToolTaskNext,
		Success:   true,
		Output:    output.String(),
		Duration:  time.Since(start),
		RiskLevel: tools.RiskNone,
		Metadata: map[string]interface{}{
			"suggestion": map[string]interface{}{
				"id":          task.ID,
				"title":       task.Title,
				"priority":    task.Priority,
				"status":      task.Status,
				"estimate":    task.Estimate,
				"description": task.Description,
			},
		},
	}, nil
}

// Validate checks if the request is valid.
func (t *TaskNextTool) Validate(req *tools.ToolRequest) error {
	return nil // No required parameters
}

// AssessRisk returns the risk level (always none for suggestions).
func (t *TaskNextTool) AssessRisk(req *tools.ToolRequest) tools.RiskLevel {
	return tools.RiskNone
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL REGISTRATION
// ═══════════════════════════════════════════════════════════════════════════════

// ToolDefinition provides schema for LLM function calling.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// GetToolDefinitions returns the schema definitions for all task tools.
// These can be used for LLM function calling / tool use.
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        string(ToolTaskCreate),
			Description: "Create a new task with title, description, priority, and optional dependencies. Use this to break down work into actionable items.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "The task title (required)",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Detailed description of the task",
					},
					"project_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the project this task belongs to",
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"low", "medium", "high", "critical"},
						"description": "Task priority level",
					},
					"estimate": map[string]interface{}{
						"type":        "number",
						"description": "Estimated time in minutes",
					},
					"dependencies": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "IDs of tasks this task depends on",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Tags for categorization",
					},
				},
				"required": []string{"title"},
			},
		},
		{
			Name:        string(ToolTaskList),
			Description: "List tasks with optional filters by status, priority, or project. Use this to see what tasks exist and their current state.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by project ID",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"todo", "doing", "blocked", "done"},
						"description": "Filter by task status",
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"low", "medium", "high", "critical"},
						"description": "Filter by priority level",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of tasks to return",
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        string(ToolTaskUpdate),
			Description: "Update an existing task's status, priority, description, or time tracking. Use this to mark tasks as done or modify task details.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the task to update (required)",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "New title for the task",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New description for the task",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"todo", "doing", "blocked", "done"},
						"description": "New status for the task",
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"low", "medium", "high", "critical"},
						"description": "New priority level",
					},
					"estimate": map[string]interface{}{
						"type":        "number",
						"description": "Updated estimated time in minutes",
					},
					"actual_time": map[string]interface{}{
						"type":        "number",
						"description": "Actual time spent in minutes",
					},
				},
				"required": []string{"task_id"},
			},
		},
		{
			Name:        string(ToolTaskDependency),
			Description: "Add or remove a dependency between tasks. Task A depends on Task B means B must be completed before A can start.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the task that has the dependency (required)",
					},
					"depends_on_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the task that must be completed first (required)",
					},
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"add", "remove"},
						"description": "Whether to add or remove the dependency",
						"default":     "add",
					},
				},
				"required": []string{"task_id", "depends_on_id"},
			},
		},
		{
			Name:        string(ToolTaskNext),
			Description: "Suggest the next task to work on based on dependencies and priority. Returns the highest priority task that has no incomplete dependencies.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project_id": map[string]interface{}{
						"type":        "string",
						"description": "Limit suggestions to a specific project",
					},
				},
				"required": []string{},
			},
		},
	}
}

// GetToolDefinitionsJSON returns the tool definitions as JSON (for LLM APIs).
func GetToolDefinitionsJSON() (string, error) {
	defs := GetToolDefinitions()
	data, err := json.MarshalIndent(defs, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
