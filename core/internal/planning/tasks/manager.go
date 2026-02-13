package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/normanking/cortex/internal/planning/graph"
)

// Status represents the current state of a task
type Status string

const (
	StatusTodo    Status = "todo"
	StatusDoing   Status = "doing"
	StatusBlocked Status = "blocked"
	StatusDone    Status = "done"
)

// Priority represents the importance level of a task
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Task represents a work item in the system
type Task struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Status       Status    `json:"status"`
	Priority     Priority  `json:"priority"`
	Dependencies []string  `json:"dependencies"` // IDs of tasks this depends on
	Estimate     int       `json:"estimate"`     // Estimated time in minutes
	ActualTime   int       `json:"actual_time"`  // Actual time spent in minutes
	Tags         []string  `json:"tags"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// ListOptions provides filtering options for listing tasks
type ListOptions struct {
	ProjectID string
	Status    Status
	Priority  Priority
	Tags      []string
	Limit     int
	Offset    int
}

// Manager handles task operations
type Manager struct {
	db *sql.DB
}

// NewManager creates a new task manager
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// Create creates a new task
func (m *Manager) Create(ctx context.Context, task *Task) error {
	if task.ID == "" {
		task.ID = uuid.New().String()
	}

	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now

	if task.Status == "" {
		task.Status = StatusTodo
	}

	if task.Priority == "" {
		task.Priority = PriorityMedium
	}

	query := `
		INSERT INTO tasks (
			id, project_id, title, description, status, priority,
			estimate, actual_time, tags, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	tagsStr := strings.Join(task.Tags, ",")

	_, err := m.db.ExecContext(ctx, query,
		task.ID,
		task.ProjectID,
		task.Title,
		task.Description,
		task.Status,
		task.Priority,
		task.Estimate,
		task.ActualTime,
		tagsStr,
		task.CreatedAt,
		task.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	// Add dependencies if provided
	for _, depID := range task.Dependencies {
		if err := m.AddDependency(ctx, task.ID, depID); err != nil {
			return fmt.Errorf("failed to add dependency: %w", err)
		}
	}

	return nil
}

// Get retrieves a task by ID
func (m *Manager) Get(ctx context.Context, id string) (*Task, error) {
	query := `
		SELECT id, project_id, title, description, status, priority,
		       estimate, actual_time, tags, created_at, updated_at, completed_at
		FROM tasks
		WHERE id = ?
	`

	task := &Task{}
	var tagsStr string
	var completedAt sql.NullTime

	err := m.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID,
		&task.ProjectID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.Priority,
		&task.Estimate,
		&task.ActualTime,
		&tagsStr,
		&task.CreatedAt,
		&task.UpdatedAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	if tagsStr != "" {
		task.Tags = strings.Split(tagsStr, ",")
	}

	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}

	// Load dependencies
	deps, err := m.GetDependencies(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load dependencies: %w", err)
	}
	task.Dependencies = deps

	return task, nil
}

// List retrieves tasks based on filter options
func (m *Manager) List(ctx context.Context, opts ListOptions) ([]Task, error) {
	query := `
		SELECT id, project_id, title, description, status, priority,
		       estimate, actual_time, tags, created_at, updated_at, completed_at
		FROM tasks
		WHERE 1=1
	`

	args := []interface{}{}

	if opts.ProjectID != "" {
		query += " AND project_id = ?"
		args = append(args, opts.ProjectID)
	}

	if opts.Status != "" {
		query += " AND status = ?"
		args = append(args, opts.Status)
	}

	if opts.Priority != "" {
		query += " AND priority = ?"
		args = append(args, opts.Priority)
	}

	if len(opts.Tags) > 0 {
		// Find tasks that have any of the specified tags
		placeholders := make([]string, len(opts.Tags))
		for i, tag := range opts.Tags {
			placeholders[i] = "?"
			args = append(args, "%"+tag+"%")
		}
		query += " AND (" + strings.Join(
			strings.Split(strings.Repeat("tags LIKE ? OR ", len(opts.Tags)), " OR ")[:len(opts.Tags)],
			" OR ",
		) + ")"
	}

	query += " ORDER BY created_at DESC"

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	if opts.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, opts.Offset)
	}

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		task := Task{}
		var tagsStr string
		var completedAt sql.NullTime

		err := rows.Scan(
			&task.ID,
			&task.ProjectID,
			&task.Title,
			&task.Description,
			&task.Status,
			&task.Priority,
			&task.Estimate,
			&task.ActualTime,
			&tagsStr,
			&task.CreatedAt,
			&task.UpdatedAt,
			&completedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		if tagsStr != "" {
			task.Tags = strings.Split(tagsStr, ",")
		}

		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}

		// Load dependencies for each task
		deps, err := m.GetDependencies(ctx, task.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load dependencies: %w", err)
		}
		task.Dependencies = deps

		tasks = append(tasks, task)
	}

	return tasks, rows.Err()
}

// Update updates a task with the provided field values
func (m *Manager) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	setClauses := []string{}
	args := []interface{}{}

	for field, value := range updates {
		// Convert field names to snake_case for database
		dbField := field
		switch field {
		case "projectID":
			dbField = "project_id"
		case "actualTime":
			dbField = "actual_time"
		case "createdAt":
			dbField = "created_at"
		case "updatedAt":
			dbField = "updated_at"
		case "completedAt":
			dbField = "completed_at"
		}

		// Handle special types
		if field == "tags" {
			if tags, ok := value.([]string); ok {
				value = strings.Join(tags, ",")
			}
		}

		setClauses = append(setClauses, dbField+" = ?")
		args = append(args, value)
	}

	// Always update the updated_at timestamp
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now())

	args = append(args, id)

	query := fmt.Sprintf(
		"UPDATE tasks SET %s WHERE id = ?",
		strings.Join(setClauses, ", "),
	)

	result, err := m.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("task not found: %s", id)
	}

	return nil
}

// Complete marks a task as completed
func (m *Manager) Complete(ctx context.Context, id string) error {
	now := time.Now()

	updates := map[string]interface{}{
		"status":      StatusDone,
		"completedAt": now,
	}

	return m.Update(ctx, id, updates)
}

// Delete removes a task from the system
func (m *Manager) Delete(ctx context.Context, id string) error {
	// First, remove all dependencies involving this task
	_, err := m.db.ExecContext(ctx,
		"DELETE FROM task_dependencies WHERE task_id = ? OR depends_on_id = ?",
		id, id,
	)
	if err != nil {
		return fmt.Errorf("failed to delete task dependencies: %w", err)
	}

	// Then delete the task itself
	result, err := m.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("task not found: %s", id)
	}

	return nil
}

// AddDependency adds a dependency relationship between tasks
func (m *Manager) AddDependency(ctx context.Context, taskID, dependsOnID string) error {
	// Verify both tasks exist
	if _, err := m.Get(ctx, taskID); err != nil {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if _, err := m.Get(ctx, dependsOnID); err != nil {
		return fmt.Errorf("dependency task not found: %s", dependsOnID)
	}

	// Check for cycle before adding
	if err := m.checkCycle(ctx, taskID, dependsOnID); err != nil {
		return err
	}

	query := `
		INSERT INTO task_dependencies (task_id, depends_on_id, created_at)
		VALUES (?, ?, ?)
	`

	_, err := m.db.ExecContext(ctx, query, taskID, dependsOnID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add dependency: %w", err)
	}

	return nil
}

// RemoveDependency removes a dependency relationship between tasks
func (m *Manager) RemoveDependency(ctx context.Context, taskID, dependsOnID string) error {
	query := "DELETE FROM task_dependencies WHERE task_id = ? AND depends_on_id = ?"

	result, err := m.db.ExecContext(ctx, query, taskID, dependsOnID)
	if err != nil {
		return fmt.Errorf("failed to remove dependency: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("dependency not found")
	}

	return nil
}

// GetDependencies returns the IDs of tasks that the given task depends on
func (m *Manager) GetDependencies(ctx context.Context, taskID string) ([]string, error) {
	query := `
		SELECT depends_on_id
		FROM task_dependencies
		WHERE task_id = ?
	`

	rows, err := m.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies: %w", err)
	}
	defer rows.Close()

	deps := []string{}
	for rows.Next() {
		var depID string
		if err := rows.Scan(&depID); err != nil {
			return nil, fmt.Errorf("failed to scan dependency: %w", err)
		}
		deps = append(deps, depID)
	}

	return deps, rows.Err()
}

// GetBlockers returns tasks that are blocking the given task (incomplete dependencies)
func (m *Manager) GetBlockers(ctx context.Context, taskID string) ([]Task, error) {
	query := `
		SELECT t.id, t.project_id, t.title, t.description, t.status, t.priority,
		       t.estimate, t.actual_time, t.tags, t.created_at, t.updated_at, t.completed_at
		FROM tasks t
		INNER JOIN task_dependencies td ON t.id = td.depends_on_id
		WHERE td.task_id = ? AND t.status != ?
	`

	rows, err := m.db.QueryContext(ctx, query, taskID, StatusDone)
	if err != nil {
		return nil, fmt.Errorf("failed to get blockers: %w", err)
	}
	defer rows.Close()

	blockers := []Task{}
	for rows.Next() {
		task := Task{}
		var tagsStr string
		var completedAt sql.NullTime

		err := rows.Scan(
			&task.ID,
			&task.ProjectID,
			&task.Title,
			&task.Description,
			&task.Status,
			&task.Priority,
			&task.Estimate,
			&task.ActualTime,
			&tagsStr,
			&task.CreatedAt,
			&task.UpdatedAt,
			&completedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan blocker: %w", err)
		}

		if tagsStr != "" {
			task.Tags = strings.Split(tagsStr, ",")
		}

		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}

		blockers = append(blockers, task)
	}

	return blockers, rows.Err()
}

// GetExecutionOrder returns tasks in dependency order using topological sort
func (m *Manager) GetExecutionOrder(ctx context.Context, projectID string) ([]Task, error) {
	g, err := m.buildGraph(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to build graph: %w", err)
	}

	order, err := g.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("failed to get topological order: %w", err)
	}

	tasks := make([]Task, 0, len(order))
	for _, taskID := range order {
		task, err := m.Get(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task %s: %w", taskID, err)
		}
		tasks = append(tasks, *task)
	}

	return tasks, nil
}

// SuggestNext suggests the next task to work on based on dependencies and priority
func (m *Manager) SuggestNext(ctx context.Context, projectID string) (*Task, error) {
	// Get all incomplete tasks
	tasks, err := m.List(ctx, ListOptions{
		ProjectID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	// Filter to tasks that are:
	// 1. Not done
	// 2. Not blocked (all dependencies complete)
	// 3. Not currently being worked on
	candidates := []Task{}

	for _, task := range tasks {
		if task.Status == StatusDone || task.Status == StatusDoing {
			continue
		}

		// Check if task is blocked
		blockers, err := m.GetBlockers(ctx, task.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get blockers for task %s: %w", task.ID, err)
		}

		if len(blockers) == 0 {
			candidates = append(candidates, task)
		}
	}

	if len(candidates) == 0 {
		return nil, nil // No tasks available
	}

	// Score each candidate based on priority
	// Critical = 4, High = 3, Medium = 2, Low = 1
	priorityScore := map[Priority]int{
		PriorityCritical: 4,
		PriorityHigh:     3,
		PriorityMedium:   2,
		PriorityLow:      1,
	}

	bestTask := &candidates[0]
	bestScore := priorityScore[bestTask.Priority]

	for i := 1; i < len(candidates); i++ {
		score := priorityScore[candidates[i].Priority]
		if score > bestScore {
			bestTask = &candidates[i]
			bestScore = score
		}
	}

	return bestTask, nil
}

// buildGraph builds a dependency graph for tasks in a project
func (m *Manager) buildGraph(ctx context.Context, projectID string) (*graph.Graph, error) {
	g := graph.NewGraph()

	// Get all tasks in the project
	tasks, err := m.List(ctx, ListOptions{
		ProjectID: projectID,
	})
	if err != nil {
		return nil, err
	}

	// Add all tasks as nodes
	for _, task := range tasks {
		g.AddNode(task.ID)
	}

	// Add dependency edges
	query := `
		SELECT task_id, depends_on_id
		FROM task_dependencies td
		INNER JOIN tasks t ON td.task_id = t.id
		WHERE t.project_id = ?
	`

	rows, err := m.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query dependencies: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var taskID, dependsOnID string
		if err := rows.Scan(&taskID, &dependsOnID); err != nil {
			return nil, fmt.Errorf("failed to scan dependency: %w", err)
		}

		// Edge from dependency to task (dependency must be done first)
		if err := g.AddEdge(dependsOnID, taskID); err != nil {
			return nil, fmt.Errorf("failed to add edge: %w", err)
		}
	}

	return g, rows.Err()
}

// checkCycle checks if adding a dependency would create a cycle
func (m *Manager) checkCycle(ctx context.Context, taskID, dependsOnID string) error {
	// Get the project ID from the task
	task, err := m.Get(ctx, taskID)
	if err != nil {
		return err
	}

	// Build current graph
	g, err := m.buildGraph(ctx, task.ProjectID)
	if err != nil {
		return err
	}

	// Try adding the edge temporarily
	if err := g.AddEdge(dependsOnID, taskID); err != nil {
		if strings.Contains(err.Error(), "cycle") {
			return fmt.Errorf("adding this dependency would create a cycle")
		}
		return err
	}

	return nil
}
