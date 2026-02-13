package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/normanking/cortex/internal/fingerprint"
	"github.com/normanking/cortex/internal/orchestrator"
	"github.com/normanking/cortex/internal/planning/tasks"
)

// ProjectHandler handles project context and planning endpoints.
// It provides REST APIs for project detection, fingerprinting, and task planning.
// CR-017: Uses interface for decoupling from concrete orchestrator.
type ProjectHandler struct {
	orch orchestrator.Interface
}

// NewProjectHandler creates a new project handler.
// CR-017: Accepts interface for decoupling.
func NewProjectHandler(orch orchestrator.Interface) *ProjectHandler {
	return &ProjectHandler{
		orch: orch,
	}
}

// ===========================================================================
// REQUEST/RESPONSE TYPES
// ===========================================================================

// DetectProjectRequest is the request body for POST /api/v1/project/detect.
type DetectProjectRequest struct {
	Path string `json:"path"` // Directory path to analyze
}

// DetectProjectResponse is the response for project detection.
type DetectProjectResponse struct {
	Fingerprint *fingerprint.Fingerprint `json:"fingerprint"`
	Summary     string                   `json:"summary"`
	DetectedAt  time.Time                `json:"detected_at"`
}

// DecomposeTaskRequest is the request body for POST /api/v1/planning/decompose.
type DecomposeTaskRequest struct {
	Request   string `json:"request"`    // The task to decompose
	ProjectID string `json:"project_id"` // Optional project ID
}

// DecomposeTaskResponse is the response for task decomposition.
type DecomposeTaskResponse struct {
	Tasks          []*tasks.Task       `json:"tasks"`
	Dependencies   map[string][]string `json:"dependencies"` // taskID -> [dependencyIDs]
	ExecutionOrder []string            `json:"execution_order,omitempty"`
}

// ===========================================================================
// HANDLERS
// ===========================================================================

// DetectProject handles POST /api/v1/project/detect.
// It performs project-level fingerprinting for the given path.
//
// Request:
//
//	POST /api/v1/project/detect
//	Content-Type: application/json
//	{
//	  "path": "/path/to/project"
//	}
//
// Response:
//
//	{
//	  "fingerprint": { ... },
//	  "summary": "Platform: darwin/arm64\nProject: go\n...",
//	  "detected_at": "2024-12-14T20:30:00Z"
//	}
func (h *ProjectHandler) DetectProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DetectProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	// Perform detection
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	fp, err := h.orch.DetectProject(ctx, req.Path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Detection failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	resp := DetectProjectResponse{
		Fingerprint: fp,
		Summary:     fp.Summary(),
		DetectedAt:  time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetProjectContext handles GET /api/v1/project/context.
// It returns the current cached project context, if available.
//
// Request:
//
//	GET /api/v1/project/context
//
// Response:
//
//	{
//	  "fingerprint": { ... },
//	  "summary": "...",
//	  "detected_at": "..."
//	}
//
// Or if no context is available:
//
//	{
//	  "error": "No project context available"
//	}
func (h *ProjectHandler) GetProjectContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fp := h.orch.GetProjectContext()
	if fp == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "No project context available. Use POST /api/v1/project/detect to detect a project.",
		})
		return
	}

	resp := DetectProjectResponse{
		Fingerprint: fp,
		Summary:     fp.Summary(),
		DetectedAt:  fp.FingerprintedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// DecomposeTask handles POST /api/v1/planning/decompose.
// It breaks down a complex request into subtasks.
//
// Request:
//
//	POST /api/v1/planning/decompose
//	Content-Type: application/json
//	{
//	  "request": "Build a REST API with authentication",
//	  "project_id": "proj-123"
//	}
//
// Response:
//
//	{
//	  "tasks": [
//	    {
//	      "id": "task-1",
//	      "title": "Create database schema",
//	      "description": "...",
//	      ...
//	    }
//	  ],
//	  "dependencies": {
//	    "task-2": ["task-1"],
//	    "task-3": ["task-1", "task-2"]
//	  },
//	  "execution_order": ["task-1", "task-2", "task-3"]
//	}
func (h *ProjectHandler) DecomposeTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DecomposeTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Request == "" {
		http.Error(w, "request is required", http.StatusBadRequest)
		return
	}

	// Default project ID if not provided
	if req.ProjectID == "" {
		req.ProjectID = "default"
	}

	// Decompose the task
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	taskList, err := h.orch.DecomposeTask(ctx, req.Request, req.ProjectID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Task decomposition failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Build dependency map
	dependencies := make(map[string][]string)
	for _, task := range taskList {
		if len(task.Dependencies) > 0 {
			dependencies[task.ID] = task.Dependencies
		}
	}

	// TODO: Compute execution order using topological sort
	// For now, this is left empty as the DecomposeTask method is a stub
	var executionOrder []string

	resp := DecomposeTaskResponse{
		Tasks:          taskList,
		Dependencies:   dependencies,
		ExecutionOrder: executionOrder,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ExecutePlan handles POST /api/v1/planning/execute.
// It executes a pre-defined task plan in dependency order.
//
// Request:
//
//	POST /api/v1/planning/execute
//	Content-Type: application/json
//	{
//	  "project_id": "proj-123"
//	}
//
// Response:
//
//	{
//	  "success": true,
//	  "executed_count": 5,
//	  "failed_count": 0,
//	  "duration_ms": 12345
//	}
func (h *ProjectHandler) ExecutePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.ProjectID == "" {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}

	// Execute the plan
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	startTime := time.Now()
	err := h.orch.ExecutePlan(ctx, req.ProjectID)
	duration := time.Since(startTime)

	if err != nil {
		http.Error(w, fmt.Sprintf("Plan execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	// TODO: Gather execution statistics (executed count, failed count, etc.)
	// For now, return success
	resp := map[string]interface{}{
		"success":     true,
		"duration_ms": duration.Milliseconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// SuggestNext handles GET /api/v1/planning/suggest-next.
// It suggests the next task to work on based on dependencies and priority.
//
// Request:
//
//	GET /api/v1/planning/suggest-next?project_id=proj-123
//
// Response:
//
//	{
//	  "task": { ... }
//	}
//
// Or if no tasks are available:
//
//	{
//	  "task": null,
//	  "message": "No tasks available to work on"
//	}
func (h *ProjectHandler) SuggestNext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		http.Error(w, "project_id query parameter is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	task, err := h.orch.SuggestNextTask(ctx, projectID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to suggest next task: %v", err), http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{}
	if task == nil {
		resp["task"] = nil
		resp["message"] = "No tasks available to work on"
	} else {
		resp["task"] = task
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
