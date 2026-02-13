// Package orchestrator provides the central coordination layer for Cortex.
package orchestrator

import (
	"context"

	"github.com/normanking/cortex/internal/agent"
	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/eval"
	"github.com/normanking/cortex/internal/fingerprint"
	"github.com/normanking/cortex/internal/planning/tasks"
	"github.com/normanking/cortex/internal/registrar"
	"github.com/normanking/cortex/pkg/brain/sleep"
)

// Interface defines the contract for orchestrator implementations.
// This interface enables decoupling consumers (TUI, Server) from the concrete
// Orchestrator implementation, supporting testing and alternative implementations.
//
// CR-017: Orchestrator Decoupling
type Interface interface {
	// Process handles a request through the full pipeline.
	// This is the primary entry point for TUI interactions.
	Process(ctx context.Context, req *Request) (*Response, error)

	// EventBus returns the event bus for subscribing to orchestrator events.
	// Used by TUI for metrics collection and real-time updates.
	EventBus() *bus.EventBus

	// Interrupt cancels the current processing stream.
	// Used by Server interrupt handler (CR-010 Track 3).
	Interrupt(reason string) error

	// DetectProject analyzes a path to identify the project type.
	// Used by Server project handler for project detection endpoint.
	DetectProject(ctx context.Context, path string) (*fingerprint.Fingerprint, error)

	// GetProjectContext returns the current project fingerprint.
	// Note: Currently returns nil (stub implementation).
	GetProjectContext() *fingerprint.Fingerprint

	// DecomposeTask breaks down a request into subtasks.
	// Note: Currently returns empty slice (stub implementation).
	DecomposeTask(ctx context.Context, request string, projectID string) ([]*tasks.Task, error)

	// ExecutePlan runs all pending tasks for a project in order.
	// Used by Server project handler for plan execution endpoint.
	ExecutePlan(ctx context.Context, projectID string) error

	// SuggestNextTask recommends the next task to work on.
	// Used by Server project handler for task suggestion endpoint.
	SuggestNextTask(ctx context.Context, projectID string) (*tasks.Task, error)

	// Stats returns current orchestrator statistics.
	// Used by TUI /status command.
	Stats() OrchestratorStats

	// GetRecentLogs returns recent conversation logs for display.
	// Used by TUI /logs command.
	GetRecentLogs(limit int) ([]*eval.ConversationLog, error)

	// Sleep returns the sleep coordinator for self-improvement operations.
	// CR-020: Sleep Cycle Self-Improvement
	// Used by TUI /sleep, /personality, /proposals commands.
	Sleep() SleepCoordinator

	// EnterSleep initiates a sleep cycle and returns the wake report.
	// CR-020: Sleep Cycle Self-Improvement
	// Used by TUI /sleep command for triggering sleep cycles.
	EnterSleep(ctx context.Context) (*sleep.WakeReport, error)

	// SetPersona sets the active persona by ID.
	// Used by TUI /persona command for persona switching.
	SetPersona(ctx context.Context, personaID string) error

	// GetPersona returns the current active persona ID.
	GetPersona() string

	// Registrar returns the capability registrar for capability discovery.
	// CR-025: Registrar Integration
	// Used by TUI /capabilities and /capability commands.
	Registrar() *registrar.Registrar

	// SetCheckpointHandler sets the handler for supervised agentic mode checkpoints.
	// When the agent hits a checkpoint (loop, step limit, error), it calls this handler
	// to pause and ask the user what to do next.
	SetCheckpointHandler(handler agent.CheckpointHandler)

	// SetSupervisedConfig sets the configuration for supervised agentic mode.
	SetSupervisedConfig(config agent.SupervisedConfig)
}

// Verify that Orchestrator implements Interface at compile time.
var _ Interface = (*Orchestrator)(nil)
