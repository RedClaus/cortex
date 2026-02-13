package orchestrator

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/normanking/cortex/internal/agent"
	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/cognitive"
	cogdecomp "github.com/normanking/cortex/internal/cognitive/decomposer"
	cogdistill "github.com/normanking/cortex/internal/cognitive/distillation"
	cogfeedback "github.com/normanking/cortex/internal/cognitive/feedback"
	cogrouter "github.com/normanking/cortex/internal/cognitive/router"
	cogtemplates "github.com/normanking/cortex/internal/cognitive/templates"
	"github.com/normanking/cortex/internal/eval"
	"github.com/normanking/cortex/internal/facets"
	"github.com/normanking/cortex/internal/fingerprint"
	"github.com/normanking/cortex/internal/knowledge"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/memory"
	"github.com/normanking/cortex/internal/memory/memcell"
	"github.com/normanking/cortex/internal/registrar"
	"github.com/normanking/cortex/internal/persona"
	"github.com/normanking/cortex/internal/planning/tasks"
	"github.com/normanking/cortex/internal/router"
	"github.com/normanking/cortex/internal/tools"
	"github.com/normanking/cortex/internal/tools/macos"
)

// Orchestrator coordinates all Cortex components.
type Orchestrator struct {
	// Core components
	router      *router.SmartRouter
	toolExec    *tools.Executor
	fabric      *knowledge.Fabric
	fpDetector  *fingerprint.Fingerprinter
	taskManager *tasks.Manager

	// Memory system (MemGPT-style)
	memoryStore *memory.CoreMemoryStore
	memoryTools *memory.MemoryTools
	personaCore *persona.PersonaCore

	// CRITICAL FIX: PassiveRetriever for automatic memory injection
	// This enables memory to be injected into LLM context without bypassing
	passiveRetriever *memory.PassiveRetriever

	// CR-015: Enhanced Memory System
	enhancedMem *enhancedMemory

	// Persona and Mode Management (CR-011)
	// CR-017: Phase 3 - These are now managed by PersonaCoordinator
	facetStore    *facets.PersonaStore
	modeManager   *persona.ModeManager
	activePersona *facets.PersonaCore
	activeMode    persona.ModeType

	// CR-017: Phase 3 - Persona Coordinator (preferred over legacy fields)
	persona PersonaManager

	// CR-017: Phase 3 - Memory Coordinator (preferred over legacy fields)
	memory MemorySystem

	// CR-017: Phase 4 - Tool Coordinator (preferred over legacy fields)
	tools ToolExecutor

	// CR-018: Introspection Coordinator (metacognitive self-awareness)
	introspection IntrospectionCoordinator

	// CR-018: Web search tool for introspection acquisition
	webSearchTool *tools.WebSearchTool

	// CR-019: Brain Coordinator (Executive cognitive architecture)
	brain BrainSystem

	// CR-020: Sleep Coordinator (self-improvement during idle periods)
	sleep SleepCoordinator

	// CR-024: Trace Coordinator (System 3 reasoning trace storage)
	traces TraceCoordinator

	// CR-024: Supervision Coordinator (System 3 process-supervised thought search)
	supervision SupervisionCoordinator

	// CR-024: Identity Coordinator (System 3 identity persistence and drift detection)
	identity IdentityCoordinator

	// CR-025: Model Discovery Coordinator (automatic model discovery during idle periods)
	modelDiscovery ModelDiscoveryCoordinator

	// CR-026: RAPID Framework (Reduce AI Prompt Iteration Depth)
	rapidConfig *RAPIDConfig

	// CR-027: Registrar for component registration and discovery
	registrar *registrar.Registrar

	// CR-025: Skill Library for learning from successful executions
	skillLibrary *memory.SkillLibrary

	// CR-025: NextScenePredictor for predictive memory loading
	nextScenePredictor *memory.NextScenePredictor

	// CR-027: MemCell Coordinator (atomic memory extraction)
	memcell MemCellSystem

	// Cognitive Architecture (CR-017: Phase 2 - extracted to coordinator)
	cognitive  CognitiveArchitecture
	cogEnabled bool // Kept for backward compatibility with existing options

	// Legacy cognitive fields - kept for backward compatibility with existing Options
	// These are used to construct the CognitiveCoordinator if not provided directly
	cogRouter      *cogrouter.Router
	cogRegistry    cognitive.Registry
	cogTemplateEng *cogtemplates.Engine
	cogDistiller   *cogdistill.Engine
	cogDecomposer  *cogdecomp.Decomposer
	cogFeedback    *cogfeedback.Loop
	promptManager  *cognitive.PromptManager

	// Conversation Logging & Model Capability Assessment
	convLogger    eval.ConversationLogger
	assessor      *eval.CapabilityAssessor
	recommender   *eval.ModelRecommender
	outcomeLogger eval.OutcomeLogger // Routing outcome logging for RoamPal learning
	evalEnabled   bool

	// Event Bus (CR-010)
	eventBus *bus.EventBus

	// Interrupt handling (CR-010 Track 3: Cognitive Interrupt Chain)
	interruptChan    chan struct{}
	currentStreamCtx context.Context
	cancelStream     context.CancelFunc

	// Specialists for different task types
	specialists map[router.TaskType]*Specialist

	// LLM provider (optional - nil for tool-only mode)
	llm LLMProvider

	// Agent for agentic mode (tool use, multi-step tasks)
	agentLLM    AgentLLMProvider
	agenticMode bool

	// Supervised agentic mode - checkpoint system
	checkpointHandler agent.CheckpointHandler
	supervisedConfig  agent.SupervisedConfig

	// Fallback LLM providers for timeout recovery
	fallbackLLMs    map[string]AgentLLMProvider
	primaryEndpoint string // Primary LLM endpoint (e.g., Ollama URL)
	primaryProvider string // Primary provider name (e.g., "ollama")
	primaryModel    string // Primary model name (e.g., "llama3.2:1b")

	// Learning callback for timeout recovery
	onTimeoutLearn agent.LearningCallback

	// Configuration
	config *Config

	// Active persona ID (fallback when PersonaManager not available)
	activePersonaID string

	// Statistics
	stats OrchestratorStats
	mu    sync.RWMutex

	// Logging
	log *logging.Logger
}

// AgentLLMProvider interface for the agent's LLM needs.
type AgentLLMProvider interface {
	Chat(ctx context.Context, messages []agent.ChatMessage, systemPrompt string) (string, error)
	SetModel(model string) // SetModel allows dynamic model switching
}

// Config configures the orchestrator.
type Config struct {
	// DefaultTimeout for request processing.
	DefaultTimeout time.Duration

	// MaxToolCalls per request.
	MaxToolCalls int

	// EnableKnowledge toggles knowledge retrieval.
	EnableKnowledge bool

	// EnableFingerprint toggles platform detection.
	EnableFingerprint bool

	// RequireConfirmation for high-risk operations.
	RequireConfirmation bool

	// SkipRoutingForSimpleCommands enables fast-path for simple shell commands
	// like ls, cd, pwd, etc. When true (default), these commands skip cognitive
	// routing (embeddings, template matching) and go directly to tool execution.
	SkipRoutingForSimpleCommands bool
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		DefaultTimeout:               5 * time.Minute, // Increased for remote LLM providers
		MaxToolCalls:                 10,
		EnableKnowledge:              true,
		EnableFingerprint:            true,
		RequireConfirmation:          true,
		SkipRoutingForSimpleCommands: true, // Fast-path for ls, cd, pwd, etc.
	}
}

func isPersonalQuestion(input string) bool {
	lower := strings.ToLower(input)

	personalPatterns := []string{
		// User identity questions (about the user)
		"who am i", "who i am", "what's my name", "what is my name",
		"do you know me", "do you remember me", "what do you know about me",
		"tell me about myself", "what have i told you", "what did i tell you",
		"what do you remember", "my name is", "i am ", "i'm ", "about me",
		"remember when", "you know me",
		// AI identity questions (about Cortex) - prevent routing as shell "who" command
		"who are you", "what are you", "who is cortex", "what is cortex",
		"who made you", "who created you", "tell me about yourself",
		"introduce yourself", "what can you do", "what do you do",
		// Family/relationship questions
		"my wife", "my husband", "my spouse", "my partner",
		"my son", "my daughter", "my child", "my children", "my kids",
		"my brother", "my sister", "my sibling",
		"my mom", "my dad", "my mother", "my father", "my parent",
		"my family", "wife's name", "husband's name", "son's name", "daughter's name",
		// Work context
		"my job", "my work", "my role", "my boss", "my team",
		"my report", "my colleague", "my manager", "my company",
		// Travel/location
		"my trip", "my flight", "my travel", "my vacation",
		// Possessive questions
		"when is my", "where is my", "who is my", "what is my",
		"what's my", "whats my", "who's my", "whos my",
		"when's my", "whens my", "where's my", "wheres my",
		// Memory recall
		"did i tell you", "have i told you",
		// Memory lookup patterns - CRITICAL for personal context retrieval
		// These should go to memory, not be answered from training data
		"what do you know about", "tell me what you know",
		"do you know anything about", "what have you learned about",
		"what did i say about", "what did i mention about",
	}

	for _, pattern := range personalPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Special handling for "who is [Name]" patterns
	// If it's "who is" followed by a capitalized name (not a common public figure),
	// it's likely asking about someone the user knows personally
	if strings.HasPrefix(lower, "who is ") {
		// Extract the name part
		namePart := strings.TrimPrefix(lower, "who is ")
		namePart = strings.TrimSuffix(namePart, "?")
		namePart = strings.TrimSpace(namePart)

		// Skip common public figures/concepts that should be answered from training
		publicFigures := []string{
			"einstein", "newton", "shakespeare", "darwin", "tesla", "edison",
			"obama", "trump", "biden", "elon musk", "jeff bezos", "bill gates",
			"steve jobs", "mark zuckerberg", "the president", "the ceo",
		}
		for _, figure := range publicFigures {
			if strings.Contains(namePart, figure) {
				return false // Let training data answer this
			}
		}

		// If it's a short name (likely a person), treat as personal
		// This catches "who is Norman", "who is John", etc.
		if len(namePart) > 0 && len(namePart) < 30 && !strings.Contains(namePart, " is ") {
			return true
		}
	}

	return false
}

// canAnswerDirectly detects factual/knowledge questions that don't need tools.
// These questions can be answered from the LLM's training data without tool use.
// This is critical for efficiency: small local models waste steps on tool calls
// for questions like "What is React?" when they can answer directly.
func canAnswerDirectly(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))

	// Patterns that indicate factual/knowledge questions answerable from training
	factualPrefixes := []string{
		"what is ", "what are ", "what's ", "whats ",
		"explain ", "describe ", "define ",
		"how does ", "how do ",
		"why is ", "why are ", "why does ", "why do ",
		"when was ", "when did ", "when is ",
		"where is ", "where are ", "where was ",
		"who is ", "who was ", "who are ",
		"can you explain ", "tell me about ", "tell me what ",
		"what does ", "what do ", "what did ",
		"give me ", "provide ", "summarize ",
		"difference between ", "compare ",
		"how to ", "how can i ", "how should i ",
	}

	// Check factual question patterns
	for _, prefix := range factualPrefixes {
		if strings.HasPrefix(lower, prefix) {
			// But NOT if it references SPECIFIC files/directories or requests actions
			// Note: Generic file questions like "How do I read a file in Python?" are OK
			// We only want to trigger tools for SPECIFIC file references
			actionIndicators := []string{
				"this file", "this folder", "this directory", "this dir", "this path",
				"the file", "the folder", "the directory",
				"my file", "my folder", "my project", "my code",
				"here", "current directory", "current folder",
				"/users/", "/home/", "~/", "./", "../", // Actual paths
				"run this", "execute this", "install this", "build this", "compile this",
				"show me the file", "list the files", "find the file", "search for file",
				"weather", "stock price", "news today", "current time",
				// System-specific queries that require tools (CR-024-FIX)
				"this machine", "this computer", "this system", "this mac", "this device",
				"my machine", "my computer", "my system", "my mac",
				"mounted", "drives", "volumes", "disk", "storage",
				"running processes", "open apps", "installed apps", "installed programs",
				"memory usage", "cpu usage", "disk space", "free space",
				"list all", "show all", "find all", // List/show/find + all typically needs tools
			}
			for _, indicator := range actionIndicators {
				if strings.Contains(lower, indicator) {
					return false // Needs tools - references specific files or actions
				}
			}
			return true // Factual question - answer directly (even if mentions generic "file")
		}
	}

	// Additional patterns that indicate knowledge questions
	knowledgePatterns := []string{
		"meaning of ", "definition of ",
		"example of ", "examples of ",
		"best practice", "best way to ",
		"pros and cons", "advantages and disadvantages",
		"should i use ", "when should i ",
		"in programming", "in software", "in computer science",
	}
	for _, pattern := range knowledgePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// isSimpleConversation detects simple greetings and chat that don't need tools.
// These inputs should go directly to LLM without agentic tool execution.
func isSimpleConversation(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))

	// Very short inputs are usually greetings
	if len(lower) < 15 {
		greetings := []string{
			"hello", "hi", "hey", "hola", "howdy", "yo",
			"good morning", "good afternoon", "good evening", "good night",
			"what's up", "whats up", "sup", "wassup",
			"how are you", "how r u", "how's it going",
			"thanks", "thank you", "thx", "ty",
			"bye", "goodbye", "see you", "later", "cya",
			"ok", "okay", "sure", "yes", "no", "yep", "nope",
			"cool", "nice", "great", "awesome", "wow",
		}
		for _, g := range greetings {
			if lower == g || strings.HasPrefix(lower, g+" ") || strings.HasSuffix(lower, " "+g) {
				return true
			}
		}
	}

	// Questions about the AI itself and its capabilities
	aiQuestions := []string{
		"who are you", "what are you", "what is your name",
		"what can you do", "what do you do", "how do you work",
		"tell me about yourself", "introduce yourself",
		"are you an ai", "are you a bot", "are you real",
		"who made you", "who created you", "who built you",
		// Language/sensory capability questions (direct answers, no tools)
		"do you speak", "can you speak", "do you understand",
		"can you understand", "can you read", "can you write",
		"can you see", "can you hear", "do you have eyes",
		"do you have ears", "are you able to see", "are you able to hear",
		// NOTE: "do you know" is intentionally NOT here - it could be a knowledge lookup
	}
	for _, q := range aiQuestions {
		if strings.Contains(lower, q) {
			return true
		}
	}

	// Simple conversational patterns
	conversational := []string{
		"how are you", "how's it going", "what's new",
		"nice to meet you", "pleased to meet you",
		"good to see you", "long time no see",
	}
	for _, c := range conversational {
		if strings.Contains(lower, c) {
			return true
		}
	}

	return false
}

// isSimpleShellCommand returns true if input looks like a simple shell command
// that should skip cognitive routing. This enables fast-path execution for
// common commands like ls, cd, pwd, cat, etc.
func isSimpleShellCommand(input string) bool {
	// Normalize input
	input = strings.TrimSpace(input)
	if input == "" {
		return false
	}

	// FIRST: Check if this is a personal/memory question that should NOT be fast-pathed
	// This prevents "who am I?" from being routed as the shell "who" command
	if isPersonalQuestion(input) {
		return false
	}

	// Get first word (the command)
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}
	cmd := strings.ToLower(parts[0])

	// Check against package-level simpleShellCommands list (SSOT)
	for _, simple := range simpleShellCommands {
		if cmd == simple {
			return true
		}
	}

	// Also detect paths being executed (scripts, binaries)
	if strings.HasPrefix(cmd, "./") || strings.HasPrefix(cmd, "/") {
		return true
	}

	// Home directory paths
	if strings.HasPrefix(cmd, "~") {
		return true
	}

	return false
}

// Option configures the Orchestrator.
type Option func(*Orchestrator)

// WithRouter sets the router.
func WithRouter(r *router.SmartRouter) Option {
	return func(o *Orchestrator) {
		o.router = r
	}
}

// WithToolExecutor sets the tool executor.
func WithToolExecutor(e *tools.Executor) Option {
	return func(o *Orchestrator) {
		o.toolExec = e
	}
}

// WithKnowledgeFabric sets the knowledge fabric.
func WithKnowledgeFabric(f *knowledge.Fabric) Option {
	return func(o *Orchestrator) {
		o.fabric = f
	}
}

// WithLLMProvider sets the LLM provider.
func WithLLMProvider(llm LLMProvider) Option {
	return func(o *Orchestrator) {
		o.llm = llm
	}
}

// WithAgentLLM sets the LLM provider for agentic mode.
func WithAgentLLM(llm AgentLLMProvider) Option {
	return func(o *Orchestrator) {
		o.agentLLM = llm
		o.agenticMode = true
	}
}

// EnableAgenticMode enables agentic tool use.
func EnableAgenticMode() Option {
	return func(o *Orchestrator) {
		o.agenticMode = true
	}
}

// WithFallbackLLM adds a fallback LLM provider for timeout recovery.
func WithFallbackLLM(provider string, llm AgentLLMProvider) Option {
	return func(o *Orchestrator) {
		if o.fallbackLLMs == nil {
			o.fallbackLLMs = make(map[string]AgentLLMProvider)
		}
		o.fallbackLLMs[provider] = llm
	}
}

// WithPrimaryEndpoint sets the primary LLM endpoint info for health checks.
func WithPrimaryEndpoint(endpoint, provider, model string) Option {
	return func(o *Orchestrator) {
		o.primaryEndpoint = endpoint
		o.primaryProvider = provider
		o.primaryModel = model
	}
}

// WithTimeoutLearning sets the callback for recording timeout learnings.
func WithTimeoutLearning(callback agent.LearningCallback) Option {
	return func(o *Orchestrator) {
		o.onTimeoutLearn = callback
	}
}

// WithConfig sets custom configuration.
func WithConfig(cfg *Config) Option {
	return func(o *Orchestrator) {
		o.config = cfg
	}
}

// WithSpecialists sets custom specialists.
func WithSpecialists(specs map[router.TaskType]*Specialist) Option {
	return func(o *Orchestrator) {
		o.specialists = specs
	}
}

// WithCognitiveRouter sets the cognitive semantic router.
// Deprecated: Use WithCognitiveCoordinator instead. This option is kept for backward
// compatibility but the legacy path has been removed in CR-017 Phase 6.
func WithCognitiveRouter(r *cogrouter.Router) Option {
	return func(o *Orchestrator) {
		o.cogRouter = r
		o.cogEnabled = true
	}
}

// WithCognitiveRegistry sets the template registry.
// Deprecated: Use WithCognitiveCoordinator instead. This option is kept for backward
// compatibility but the legacy path has been removed in CR-017 Phase 6.
func WithCognitiveRegistry(reg cognitive.Registry) Option {
	return func(o *Orchestrator) {
		o.cogRegistry = reg
		o.cogEnabled = true
	}
}

// WithCognitiveTemplateEngine sets the template engine.
// Deprecated: Use WithCognitiveCoordinator instead. This option is kept for backward
// compatibility but the legacy path has been removed in CR-017 Phase 6.
func WithCognitiveTemplateEngine(eng *cogtemplates.Engine) Option {
	return func(o *Orchestrator) {
		o.cogTemplateEng = eng
	}
}

// WithCognitiveDistiller sets the distillation engine.
// Deprecated: Use WithCognitiveCoordinator instead. This option is kept for backward
// compatibility but the legacy path has been removed in CR-017 Phase 6.
func WithCognitiveDistiller(d *cogdistill.Engine) Option {
	return func(o *Orchestrator) {
		o.cogDistiller = d
	}
}

// WithCognitiveDecomposer sets the task decomposer.
// Deprecated: Use WithCognitiveCoordinator instead. This option is kept for backward
// compatibility but the legacy path has been removed in CR-017 Phase 6.
func WithCognitiveDecomposer(d *cogdecomp.Decomposer) Option {
	return func(o *Orchestrator) {
		o.cogDecomposer = d
	}
}

// WithCognitiveFeedback sets the feedback loop.
// Deprecated: Use WithCognitiveCoordinator instead. This option is kept for backward
// compatibility but the legacy path has been removed in CR-017 Phase 6.
func WithCognitiveFeedback(f *cogfeedback.Loop) Option {
	return func(o *Orchestrator) {
		o.cogFeedback = f
	}
}

// WithPromptManager sets the prompt manager for tier-optimized prompts.
// Deprecated: Use WithCognitiveCoordinator instead. This option is kept for backward
// compatibility but the legacy path has been removed in CR-017 Phase 6.
func WithPromptManager(pm *cognitive.PromptManager) Option {
	return func(o *Orchestrator) {
		o.promptManager = pm
	}
}

// EnableCognitive enables the cognitive architecture.
// Deprecated: Use WithCognitiveCoordinator instead. This option is kept for backward
// compatibility but the legacy path has been removed in CR-017 Phase 6.
func EnableCognitive() Option {
	return func(o *Orchestrator) {
		o.cogEnabled = true
	}
}

// WithCognitiveCoordinator sets the cognitive coordinator (CR-017).
// This is the preferred way to configure cognitive architecture.
// When set, the individual cognitive options (WithCognitiveRouter, etc.) are ignored.
func WithCognitiveCoordinator(cc CognitiveArchitecture) Option {
	return func(o *Orchestrator) {
		o.cognitive = cc
		o.cogEnabled = true
	}
}

// WithConversationLogger sets the conversation logger for eval.
func WithConversationLogger(logger eval.ConversationLogger) Option {
	return func(o *Orchestrator) {
		o.convLogger = logger
		o.evalEnabled = true
	}
}

// WithCapabilityAssessor sets the capability assessor for eval.
func WithCapabilityAssessor(assessor *eval.CapabilityAssessor) Option {
	return func(o *Orchestrator) {
		o.assessor = assessor
	}
}

// WithModelRecommender sets the model recommender for eval.
func WithModelRecommender(recommender *eval.ModelRecommender) Option {
	return func(o *Orchestrator) {
		o.recommender = recommender
	}
}

// EnableEval enables conversation logging and model capability assessment.
func EnableEval() Option {
	return func(o *Orchestrator) {
		o.evalEnabled = true
	}
}

// WithOutcomeLogger sets the outcome logger for routing outcome tracking.
// This enables RoamPal learning from routing decisions.
func WithOutcomeLogger(logger eval.OutcomeLogger) Option {
	return func(o *Orchestrator) {
		o.outcomeLogger = logger
	}
}

// WithSkillLibrary sets the skill library for learning from successful executions.
// CR-025: Enables Voyager-style skill learning from execution traces.
func WithSkillLibrary(lib *memory.SkillLibrary) Option {
	return func(o *Orchestrator) {
		o.skillLibrary = lib
	}
}

// WithNextScenePredictor sets the next scene predictor for predictive memory loading.
// CR-025: Enables predictive memory preloading based on user input patterns.
func WithNextScenePredictor(nsp *memory.NextScenePredictor) Option {
	return func(o *Orchestrator) {
		o.nextScenePredictor = nsp
	}
}

// WithEventBus sets the event bus for publishing orchestrator events.
func WithEventBus(b *bus.EventBus) Option {
	return func(o *Orchestrator) {
		o.eventBus = b
	}
}

// WithTaskManager sets the task manager for task-related tools.
// This enables the 5 task management tools: task_create, task_list, task_update,
// task_dependency, and task_next.
func WithTaskManager(db *sql.DB) Option {
	return func(o *Orchestrator) {
		if db != nil {
			o.taskManager = tasks.NewManager(db)
		}
	}
}

// WithFacetStore sets the facet store for persona management (CR-011).
// Deprecated: Use WithPersonaCoordinator instead. This option is kept for backward
// compatibility and used as a fallback when PersonaCoordinator is not configured.
func WithFacetStore(store *facets.PersonaStore) Option {
	return func(o *Orchestrator) {
		o.facetStore = store
	}
}

// WithModeManager sets the mode manager for behavioral mode switching (CR-011).
// Deprecated: Use WithPersonaCoordinator instead. This option is kept for backward
// compatibility and used as a fallback when PersonaCoordinator is not configured.
func WithModeManager(manager *persona.ModeManager) Option {
	return func(o *Orchestrator) {
		o.modeManager = manager
	}
}

// WithPersonaCoordinator sets the persona coordinator (CR-017 Phase 3).
// This is the preferred way to configure persona and mode management.
// When set, the legacy persona options (WithFacetStore, WithModeManager) are used
// only if PersonaCoordinator is nil.
func WithPersonaCoordinator(pc PersonaManager) Option {
	return func(o *Orchestrator) {
		o.persona = pc
	}
}

// WithMemoryCoordinator sets the memory coordinator (CR-017 Phase 3).
// This is the preferred way to configure memory operations.
// When set, the legacy memory options (WithMemoryStore) are used
// only if MemoryCoordinator is nil.
func WithMemoryCoordinator(mc MemorySystem) Option {
	return func(o *Orchestrator) {
		o.memory = mc
	}
}

// WithToolCoordinator sets the tool coordinator (CR-017 Phase 4).
// This is the preferred way to configure tool execution.
// When set, the legacy tool options (WithToolExecutor) are used
// only if ToolCoordinator is nil.
func WithToolCoordinator(tc ToolExecutor) Option {
	return func(o *Orchestrator) {
		o.tools = tc
	}
}

func WithBrainCoordinator(bc BrainSystem) Option {
	return func(o *Orchestrator) {
		o.brain = bc
	}
}

// WithMemCellCoordinator sets the MemCell coordinator (CR-027).
// This enables atomic memory extraction from conversations.
func WithMemCellCoordinator(mc MemCellSystem) Option {
	return func(o *Orchestrator) {
		o.memcell = mc
	}
}

// WithWebSearchTool sets the web search tool for introspection acquisition.
// When provided, this tool is used instead of creating a new one.
func WithWebSearchTool(wst *tools.WebSearchTool) Option {
	return func(o *Orchestrator) {
		o.webSearchTool = wst
	}
}

// New creates a new Orchestrator with the given options.
func New(opts ...Option) *Orchestrator {
	o := &Orchestrator{
		config:      DefaultConfig(),
		specialists: DefaultSpecialists(),
		fpDetector:  fingerprint.NewFingerprinter(),
		log:         logging.Global(),
		stats: OrchestratorStats{
			RouteDistribution: make(map[router.TaskType]int64),
		},
	}

	for _, opt := range opts {
		opt(o)
	}

	// Initialize persona if not provided
	if o.personaCore == nil {
		o.personaCore = persona.NewPersonaCore()
	}

	// Initialize mode manager if not provided (CR-011)
	if o.modeManager == nil {
		o.modeManager = persona.NewModeManager()
	}

	// Set default mode to normal
	o.activeMode = persona.ModeNormal

	// Create default router if not provided
	if o.router == nil {
		o.router = router.NewSmartRouter(nil)
	}

	// Create default tool executor if not provided
	if o.toolExec == nil {
		o.toolExec = tools.NewExecutor()
	}

	// CR-017: Create CognitiveCoordinator from legacy options if not provided directly
	if o.cognitive == nil && o.cogEnabled {
		o.cognitive = NewCognitiveCoordinator(&CognitiveConfig{
			Router:         o.cogRouter,
			Registry:       o.cogRegistry,
			TemplateEngine: o.cogTemplateEng,
			Distiller:      o.cogDistiller,
			Decomposer:     o.cogDecomposer,
			Feedback:       o.cogFeedback,
			PromptManager:  o.promptManager,
			Enabled:        o.cogEnabled,
		})
	}

	// CR-017 Phase 3: Create PersonaCoordinator from legacy options if not provided directly
	if o.persona == nil {
		o.persona = NewPersonaCoordinator(&PersonaCoordinatorConfig{
			FacetStore:  o.facetStore,
			ModeManager: o.modeManager,
			PersonaCore: o.personaCore,
		})
	}

	// CR-017 Phase 3: Create MemoryCoordinator from legacy options if not provided directly
	if o.memory == nil && (o.memoryStore != nil || o.memoryTools != nil || o.fabric != nil) {
		o.memory = NewMemoryCoordinator(&MemoryCoordinatorConfig{
			CoreStore:   o.memoryStore,
			MemoryTools: o.memoryTools,
			Fabric:      o.fabric,
		})
	}

	// CR-017 Phase 4: Create ToolCoordinator from legacy options if not provided directly
	if o.tools == nil {
		o.tools = NewToolCoordinator(&ToolCoordinatorConfig{
			Executor:      o.toolExec,
			Fingerprinter: o.fpDetector,
			TaskManager:   o.taskManager,
		})
	}

	// Always register default tools (bash, read, write, etc.)
	o.registerDefaultTools()

	// CR-027: Initialize registrar with builtins and agent tools
	o.registrar = registrar.New(registrar.DefaultConfig)
	registrar.RegisterBuiltins(o.registrar)
	agent.RegisterAgentTools(o.registrar)

	return o
}

// registerDefaultTools adds the standard tools.
func (o *Orchestrator) registerDefaultTools() {
	o.toolExec.Register(tools.NewBashTool())
	o.toolExec.Register(tools.NewReadTool())
	o.toolExec.Register(tools.NewWriteTool())
	o.toolExec.Register(tools.NewEditTool())
	o.toolExec.Register(tools.NewGlobTool())
	o.toolExec.Register(tools.NewGrepTool())

	// Register web search tool if Tavily API key is available
	if tavilyKey := os.Getenv("TAVILY_API_KEY"); tavilyKey != "" {
		log := logging.Global()
		log.Info("Registering web search tool (Tavily API key found)")
		// Use provided web search tool if available, otherwise create new one
		if o.webSearchTool != nil {
			o.toolExec.Register(o.webSearchTool)
		} else {
			o.webSearchTool = tools.NewWebSearchTool(tools.WithAPIKey(tavilyKey))
			o.toolExec.Register(o.webSearchTool)
		}
	}

	// Register task management tools if task manager is available
	if o.taskManager != nil {
		log := logging.Global()
		log.Info("Registering task management tools (5 tools)")
		o.toolExec.Register(tasks.NewTaskCreateTool(o.taskManager))
		o.toolExec.Register(tasks.NewTaskListTool(o.taskManager))
		o.toolExec.Register(tasks.NewTaskUpdateTool(o.taskManager))
		o.toolExec.Register(tasks.NewTaskDependencyTool(o.taskManager))
		o.toolExec.Register(tasks.NewTaskNextTool(o.taskManager))
	}

	// Register memory tools if memory store and knowledge fabric are available
	if o.memoryStore != nil && o.fabric != nil {
		log := logging.Global()
		log.Info("Registering memory tools (5 tools)")

		// Create memory tools instance
		o.memoryTools = memory.NewMemoryTools(
			o.memoryStore,
			o.fabric,
			memory.DefaultMemoryToolsConfig(),
		)

		// Use a default user ID for now - in production this would come from auth
		userID := "default-user"
		o.toolExec.Register(tools.NewRecallSearchTool(o.memoryTools, userID))
		o.toolExec.Register(tools.NewCoreMemoryReadTool(o.memoryTools, userID))
		o.toolExec.Register(tools.NewCoreMemoryAppendTool(o.memoryTools, userID))
		o.toolExec.Register(tools.NewArchivalSearchTool(o.memoryTools, userID))
		o.toolExec.Register(tools.NewArchivalInsertTool(o.memoryTools, userID))
	}

	// Register macOS automation tools (only on darwin)
	if err := macos.RegisterAll(o.toolExec); err != nil {
		log := logging.Global()
		log.Error("Failed to register macOS tools: %v", err)
	} else {
		log := logging.Global()
		log.Info("Registered macOS automation tools (11 tools)")
	}
}

// Process handles a request through the full pipeline.
func (o *Orchestrator) Process(ctx context.Context, req *Request) (*Response, error) {
	start := time.Now()
	log := logging.Global()

	log.Info("[Orchestrator] Process called, input=%q", truncateLog(req.Input, 50))

	// Ensure request has an ID
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	req.Timestamp = time.Now()

	// CR-017 Phase 5: Publish RequestReceived event
	if o.eventBus != nil {
		o.eventBus.Publish(bus.NewRequestReceivedEvent(req.ID, req.Input, req.ID))
	}
	log.Info("[Orchestrator] Event published, building pipeline...")

	// Create pipeline state
	state := NewPipelineState(req)

	// Apply timeout
	timeout := o.config.DefaultTimeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Fast-path detection for simple shell commands
	// Skip cognitive routing (embeddings, template matching) for commands like ls, cd, pwd
	// This can save 1-60+ seconds of latency for simple operations
	skipCognitive := false
	if o.config.SkipRoutingForSimpleCommands && isSimpleShellCommand(req.Input) {
		log.Info("[Orchestrator] Simple command detected, skipping cognitive routing: %s", strings.Fields(req.Input)[0])
		skipCognitive = true
	}

	// Build pipeline stages based on whether we're skipping cognitive routing
	var stages []Stage
	if skipCognitive {
		// Fast-path: Skip cognitive stage for simple shell commands
		// RAPID gate also skipped since simple commands have high confidence
		stages = []Stage{
			&fingerprintStage{o: o},
			&routingStage{o: o},
			&introspectionStage{o: o}, // CR-018: Metacognitive check
			// rapidGateStage SKIPPED (simple commands bypass)
			// cognitiveStage SKIPPED for simple commands
			&knowledgeStage{o: o},
			&toolExecutionStage{o: o},
			&llmStage{o: o},
		}
	} else {
		// Full pipeline with RAPID gating and cognitive routing
		// CR-026: RAPID gate inserted after routing to evaluate confidence
		// CR-018: Introspection stage runs after rapid gate to handle metacognitive queries
		stages = []Stage{
			&fingerprintStage{o: o},   // Level 1: Automatic context inference
			&routingStage{o: o},       // Level 2: Intent classification
			&rapidGateStage{o: o},     // Level 2-4: Confidence gate + compound question
			&introspectionStage{o: o}, // CR-018: Metacognitive self-awareness
			&cognitiveStage{o: o},     // Template matching and distillation
			&knowledgeStage{o: o},     // Knowledge retrieval
			&toolExecutionStage{o: o},
			&llmStage{o: o},           // Level 5: Execute with confidence
		}
	}

	log.Info("[Pipeline] Starting %d stages (skipCognitive=%v)", len(stages), skipCognitive)

	// CR-026: Use labeled break to exit from select AND for loop
stageLoop:
	for _, stage := range stages {
		select {
		case <-ctx.Done():
			state.Cancelled = true
			state.AddError(ctx.Err())
			break stageLoop // Exit for loop, not just select
		default:
			stageName := stage.Name()
			log.Info("[Pipeline] Starting stage: %s", stageName)
			stageStart := time.Now()
			if err := stage.Execute(ctx, state); err != nil {
				state.AddError(err)
				log.Warn("[Pipeline] Stage %s error: %v", stageName, err)
			}
			state.StageMetrics[stageName] = time.Since(stageStart)
			log.Info("[Pipeline] Completed stage: %s (took %v)", stageName, time.Since(stageStart))

			// CR-026: RAPID early return - if rapidGateStage set LLMResponse, skip remaining stages
			if state.RAPIDDecision != nil && state.RAPIDDecision.ClarificationNeeded && state.LLMResponse != "" {
				log.Info("[RAPID] Early exit: clarification question set, skipping remaining stages")
				break stageLoop // Exit for loop, not just select
			}

			// CR-018: Introspection early return - if introspectionStage handled the query, skip remaining stages
			if state.IntrospectionResult != nil && state.IntrospectionResult.IsHandled && state.LLMResponse != "" {
				log.Info("[Introspection] Early exit: metacognitive query handled, skipping remaining stages")
				break stageLoop // Exit for loop, not just select
			}
		}

		if state.Cancelled {
			break stageLoop
		}
	}

	// Build response
	resp := o.buildResponse(state)
	resp.Duration = time.Since(start)

	// Update stats
	o.updateStats(state, resp)

	// CR-017 Phase 5: Publish ResponseGenerated event
	if o.eventBus != nil {
		templateID := ""
		if state.Cognitive != nil && state.Cognitive.Template != nil {
			templateID = state.Cognitive.Template.ID
		}
		o.eventBus.Publish(bus.NewResponseGeneratedEvent(
			req.ID,
			resp.Content,
			templateID,
			resp.Duration,
			resp.Success,
		))
	}

	// CR-027: Extract MemCells from conversation (async, non-blocking)
	if o.memcell != nil && resp.Success {
		go o.extractMemCells(ctx, req, resp)
	}

	return resp, nil
}

// ProcessSimple is a convenience method for simple requests.
func (o *Orchestrator) ProcessSimple(ctx context.Context, input string) (*Response, error) {
	return o.Process(ctx, &Request{
		Type:  RequestChat,
		Input: input,
	})
}

// ExecuteTool directly executes a tool without the full pipeline.
// CR-017 Phase 6: Delegates to ToolCoordinator if available.
func (o *Orchestrator) ExecuteTool(ctx context.Context, toolReq *tools.ToolRequest) (*tools.ToolResult, error) {
	// CR-017: Prefer ToolCoordinator
	if o.tools != nil {
		return o.tools.Execute(ctx, toolReq)
	}

	// Legacy fallback
	return o.toolExec.Execute(ctx, toolReq)
}

// Route classifies input without processing.
func (o *Orchestrator) Route(input string) *router.RoutingDecision {
	return o.router.RouteSimple(input)
}

// GetSpecialist returns the specialist for a task type.
func (o *Orchestrator) GetSpecialist(taskType router.TaskType) *Specialist {
	if spec, ok := o.specialists[taskType]; ok {
		return spec
	}
	return o.specialists[router.TaskGeneral]
}

// MemoryStore returns the CoreMemoryStore instance, or nil if not configured.
func (o *Orchestrator) MemoryStore() *memory.CoreMemoryStore {
	return o.memoryStore
}

// GetPassiveRetriever returns the PassiveRetriever instance, or nil if not configured.
// CRITICAL: This is used by memory_integration.go to inject memories into LLM context.
func (o *Orchestrator) GetPassiveRetriever() *memory.PassiveRetriever {
	return o.passiveRetriever
}

// Stats returns current statistics.
func (o *Orchestrator) Stats() OrchestratorStats {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Copy to avoid race
	stats := o.stats
	stats.RouteDistribution = make(map[router.TaskType]int64)
	for k, v := range o.stats.RouteDistribution {
		stats.RouteDistribution[k] = v
	}
	return stats
}

// EventBus returns the event bus instance.
func (o *Orchestrator) EventBus() *bus.EventBus {
	return o.eventBus
}

// GetRecentLogs returns recent conversation logs for display in the TUI.
func (o *Orchestrator) GetRecentLogs(limit int) ([]*eval.ConversationLog, error) {
	if !o.evalEnabled || o.convLogger == nil {
		return nil, nil
	}
	return o.convLogger.ListLogs(context.Background(), limit)
}

// GetWebSearchTool returns the web search tool for introspection acquisition.
func (o *Orchestrator) GetWebSearchTool() *tools.WebSearchTool {
	return o.webSearchTool
}

// SetOutcomeLogger sets the outcome logger for routing outcome tracking.
// This enables RoamPal learning from routing decisions.
// Safe to call even if logger is nil (outcome recording will be skipped).
func (o *Orchestrator) SetOutcomeLogger(logger eval.OutcomeLogger) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.outcomeLogger = logger
}

// OutcomeLogger returns the current outcome logger (may be nil).
func (o *Orchestrator) OutcomeLogger() eval.OutcomeLogger {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.outcomeLogger
}

// SetPersona sets the active persona by ID (CR-011).
// Returns an error if the persona is not found or the facet store is not configured.
// CR-017 Phase 6: Delegates to PersonaCoordinator if available.
func (o *Orchestrator) SetPersona(ctx context.Context, personaID string) error {
	log := logging.Global()
	log.Info("[Orchestrator] SetPersona called with: %s", personaID)

	// Always store the persona ID for fallback
	o.mu.Lock()
	o.activePersonaID = personaID
	o.mu.Unlock()

	// CR-017: Prefer PersonaCoordinator
	if o.persona != nil {
		if err := o.persona.SetActivePersona(ctx, personaID); err != nil {
			log.Info("[Orchestrator] PersonaCoordinator.SetActivePersona failed: %v (using ID fallback)", err)
			// Don't return error - we've stored the ID as fallback
		} else {
			log.Info("[Orchestrator] Active persona set via coordinator: %s", personaID)
			return nil
		}
	}

	// Legacy fallback: Use facet store directly
	if o.facetStore != nil {
		persona, err := o.facetStore.Get(ctx, personaID)
		if err != nil {
			log.Info("[Orchestrator] FacetStore.Get failed: %v (using ID fallback)", err)
			// Don't return error - we've stored the ID as fallback
		} else {
			o.mu.Lock()
			o.activePersona = persona
			o.mu.Unlock()
			log.Info("[Orchestrator] Active persona set via facet store: %s (%s)", persona.Name, persona.ID)
			return nil
		}
	}

	// Simple fallback - just use the stored ID
	log.Info("[Orchestrator] Active persona set (simple mode): %s", personaID)
	return nil
}

// SetMode sets the active behavioral mode (CR-011).
// CR-017 Phase 6: Delegates to PersonaCoordinator if available.
func (o *Orchestrator) SetMode(mode persona.ModeType) {
	o.SetModeWithTrigger(mode, "manual")
}

// SetModeWithTrigger sets the active behavioral mode with a custom trigger (CR-017 Phase 5).
// CR-017 Phase 6: Delegates to PersonaCoordinator if available.
func (o *Orchestrator) SetModeWithTrigger(mode persona.ModeType, trigger string) {
	// CR-017: Prefer PersonaCoordinator
	if o.persona != nil {
		previousMode := o.persona.GetActiveMode()
		o.persona.SetMode(mode, trigger)

		// CR-017 Phase 5: Publish ModeChanged event
		if o.eventBus != nil {
			o.eventBus.Publish(bus.NewModeChangedEvent(string(previousMode), string(mode), trigger))
		}
		return
	}

	// Legacy fallback
	o.mu.Lock()
	previousMode := o.activeMode
	o.activeMode = mode
	o.mu.Unlock()

	// Update mode manager
	if o.modeManager != nil {
		o.modeManager.SetMode(mode, trigger)
	}

	log := logging.Global()
	log.Info("[Orchestrator] Active mode set to: %s (trigger: %s)", mode, trigger)

	// CR-017 Phase 5: Publish ModeChanged event
	if o.eventBus != nil {
		o.eventBus.Publish(bus.NewModeChangedEvent(string(previousMode), string(mode), trigger))
	}
}

// GetActivePersona returns the currently active persona (CR-011).
// Returns nil if no persona is set.
// CR-017 Phase 6: Delegates to PersonaCoordinator if available.
func (o *Orchestrator) GetActivePersona() *facets.PersonaCore {
	// CR-017: Prefer PersonaCoordinator
	if o.persona != nil {
		return o.persona.GetActivePersona()
	}

	// Legacy fallback
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.activePersona
}

// GetActiveMode returns the currently active behavioral mode (CR-011).
// CR-017 Phase 6: Delegates to PersonaCoordinator if available.
func (o *Orchestrator) GetActiveMode() persona.ModeType {
	// CR-017: Prefer PersonaCoordinator
	if o.persona != nil {
		return o.persona.GetActiveMode()
	}

	// Legacy fallback
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.activeMode
}

// PersonaCoordinator returns the persona coordinator (CR-017 Phase 3).
// Returns nil if not configured.
func (o *Orchestrator) PersonaCoordinator() PersonaManager {
	return o.persona
}

// MemoryCoordinator returns the memory coordinator (CR-017 Phase 3).
// Returns nil if not configured.
func (o *Orchestrator) MemoryCoordinator() MemorySystem {
	return o.memory
}

// CognitiveCoordinator returns the cognitive coordinator (CR-017 Phase 2).
// Returns nil if not configured.
func (o *Orchestrator) CognitiveCoordinator() CognitiveArchitecture {
	return o.cognitive
}

// ToolCoordinator returns the tool coordinator (CR-017 Phase 4).
// Returns nil if not configured.
func (o *Orchestrator) ToolCoordinator() ToolExecutor {
	return o.tools
}

func (o *Orchestrator) BrainCoordinator() BrainSystem {
	return o.brain
}

// Registrar returns the component registrar (CR-027).
// Returns nil if not configured.
func (o *Orchestrator) Registrar() *registrar.Registrar {
	return o.registrar
}

// SetCheckpointHandler sets the handler for supervised agentic mode checkpoints.
// When the agent hits a checkpoint (loop, step limit, error), it calls this handler
// to pause and ask the user what to do next.
func (o *Orchestrator) SetCheckpointHandler(handler agent.CheckpointHandler) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.checkpointHandler = handler
}

// SetSupervisedConfig sets the configuration for supervised agentic mode.
func (o *Orchestrator) SetSupervisedConfig(config agent.SupervisedConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.supervisedConfig = config
}

// SkillLibrary returns the skill library for execution learning (CR-025).
// Returns nil if not configured.
func (o *Orchestrator) SkillLibrary() *memory.SkillLibrary {
	return o.skillLibrary
}

// NextScenePredictor returns the next scene predictor for predictive memory loading (CR-025).
// Returns nil if not configured.
func (o *Orchestrator) NextScenePredictor() *memory.NextScenePredictor {
	return o.nextScenePredictor
}

// EnhancedMemoryStores returns the CR-015 enhanced memory stores.
func (o *Orchestrator) EnhancedMemoryStores() *EnhancedMemoryStores {
	if o.enhancedMem == nil {
		return nil
	}
	return o.enhancedMem.stores
}

// StartEnhancedMemoryJobs starts the background maintenance jobs for enhanced memory.
func (o *Orchestrator) StartEnhancedMemoryJobs() {
	if o.enhancedMem != nil && o.enhancedMem.stores != nil {
		StartMemoryJobs(o.enhancedMem.stores)
	}
}

// StopEnhancedMemoryJobs stops the background maintenance jobs for enhanced memory.
func (o *Orchestrator) StopEnhancedMemoryJobs() {
	if o.enhancedMem != nil && o.enhancedMem.stores != nil {
		StopMemoryJobs(o.enhancedMem.stores)
	}
}

// Interrupt cancels the current LLM stream (CR-010 Track 3: Cognitive Interrupt Chain).
// This implements "cognitive interrupt" - when a user speaks during AI response,
// we cancel the LLM's thought process itself, not just audio playback.
func (o *Orchestrator) Interrupt(reason string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	log := logging.Global()
	log.Info("[Orchestrator] Interrupt triggered (reason: %s)", reason)

	// Cancel the current stream if one is active
	if o.cancelStream != nil {
		o.cancelStream()
		o.cancelStream = nil
		o.currentStreamCtx = nil

		// Publish interrupt event
		if o.eventBus != nil {
			evt := bus.NewInterruptEvent(reason)
			o.eventBus.Publish(evt)
			log.Debug("[Orchestrator] Published interrupt event")
		}

		log.Info("[Orchestrator] Stream cancelled successfully")
		return nil
	}

	log.Debug("[Orchestrator] No active stream to interrupt")
	return nil
}

func (o *Orchestrator) buildResponse(state *PipelineState) *Response {
	resp := state.Response

	if state.HasErrors() {
		resp.Success = false
		var errMsgs []string
		for _, err := range state.Errors {
			errMsgs = append(errMsgs, err.Error())
		}
		resp.Error = strings.Join(errMsgs, "; ")
	} else {
		resp.Success = true
	}

	resp.Routing = state.Routing
	resp.ToolResults = state.ToolResults
	resp.KnowledgeUsed = state.Knowledge
	resp.Content = state.LLMResponse

	// If no LLM response but we have tool results, summarize them
	if resp.Content == "" && len(state.ToolResults) > 0 {
		resp.Content = o.summarizeToolResults(state.ToolResults)
	}

	// Preserve existing metadata (e.g., new_working_dir from cd command)
	// and add standard fields
	if resp.Metadata == nil {
		resp.Metadata = make(map[string]interface{})
	}
	resp.Metadata["stage_metrics"] = state.StageMetrics
	resp.Metadata["cancelled"] = state.Cancelled

	// Build token metrics if we have usage data
	if state.LLMTokensUsed > 0 || state.LLMProvider != "" {
		metrics := &TokenMetrics{
			TotalTokens: state.LLMTokensUsed,
		}

		// Determine if this is a local or external provider
		provider := strings.ToLower(state.LLMProvider)

		// If provider is "unknown", try to infer from model name
		if provider == "unknown" || provider == "" {
			inferred := inferProviderFromModel(state.LLMModel)
			if inferred != "" {
				provider = inferred
			}
		}

		// Check both provider name AND model name for local inference
		isLocal := isLocalProvider(provider) || isLocalModel(state.LLMModel)

		if isLocal {
			metrics.LocalTokens = state.LLMTokensUsed
			// Use inferred provider if original was unknown
			if provider == "unknown" || provider == "" {
				inferred := inferProviderFromModel(state.LLMModel)
				if inferred != "" {
					metrics.LocalProvider = inferred
				} else {
					metrics.LocalProvider = "local"
				}
			} else {
				metrics.LocalProvider = state.LLMProvider
			}
			metrics.LocalModel = state.LLMModel
		} else if provider != "" {
			metrics.ExternalTokens = state.LLMTokensUsed
			metrics.ExternalProvider = state.LLMProvider
			metrics.ExternalModel = state.LLMModel
		}

		resp.TokenMetrics = metrics
	}

	return resp
}

// isLocalProvider returns true if the provider is a local inference provider.
func isLocalProvider(provider string) bool {
	localProviders := []string{"ollama", "mlx", "dnet", "local", "llama.cpp", "llamacpp"}
	for _, lp := range localProviders {
		if strings.Contains(provider, lp) {
			return true
		}
	}
	return false
}

// inferProviderFromModel attempts to determine the provider from the model name.
// This is used when the provider is "unknown" but the model name contains hints.
// FIX: Added cloud provider detection to prevent 404 errors when routing
// Claude/GPT/Gemini/Grok models to Ollama by mistake.
func inferProviderFromModel(model string) string {
	model = strings.ToLower(model)

	// === CLOUD PROVIDERS (check first - most specific patterns) ===

	// Anthropic Claude models (claude-3, claude-sonnet, claude-opus, etc.)
	if strings.Contains(model, "claude") {
		return "anthropic"
	}

	// OpenAI models (gpt-4, gpt-3.5, o1, o3, etc.)
	if strings.HasPrefix(model, "gpt") || strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3") {
		return "openai"
	}

	// Google Gemini models
	if strings.Contains(model, "gemini") {
		return "gemini"
	}

	// xAI Grok models (grok-2, grok-3, etc.)
	if strings.HasPrefix(model, "grok") {
		return "grok"
	}

	// === LOCAL PROVIDERS ===

	// MLX models from HuggingFace use "mlx-community/" prefix
	if strings.Contains(model, "mlx-community") || strings.Contains(model, "mlx_") {
		return "mlx"
	}

	// dnet models might have specific patterns
	if strings.Contains(model, "dnet") {
		return "dnet"
	}

	// Ollama models often have tags like ":7b", ":latest", etc.
	// This check comes last as it's the broadest pattern
	if strings.Contains(model, ":") && !strings.Contains(model, "/") {
		return "ollama"
	}

	return ""
}

// isLocalModel returns true if the model name suggests local inference.
func isLocalModel(model string) bool {
	model = strings.ToLower(model)

	// MLX models
	if strings.Contains(model, "mlx-community") || strings.Contains(model, "mlx_") {
		return true
	}

	// Ollama-style models (name:tag format without org prefix)
	if strings.Contains(model, ":") && !strings.Contains(model, "/") {
		return true
	}

	// Common local model patterns
	localPatterns := []string{"llama", "mistral", "qwen", "phi", "gemma", "codellama"}
	for _, pattern := range localPatterns {
		if strings.HasPrefix(model, pattern) {
			return true
		}
	}

	return false
}

func (o *Orchestrator) summarizeToolResults(results []*tools.ToolResult) string {
	var sb strings.Builder
	for i, r := range results {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		if r.Success {
			sb.WriteString(r.Output)
		} else {
			sb.WriteString(fmt.Sprintf("Error: %s", r.Error))
		}
	}
	return sb.String()
}

func (o *Orchestrator) updateStats(state *PipelineState, resp *Response) {
	o.mu.Lock()
	defer o.mu.Unlock()

	atomic.AddInt64(&o.stats.TotalRequests, 1)
	if resp.Success {
		atomic.AddInt64(&o.stats.SuccessCount, 1)
	} else {
		atomic.AddInt64(&o.stats.FailureCount, 1)
	}

	if state.Routing != nil {
		o.stats.RouteDistribution[state.Routing.TaskType]++
	}

	atomic.AddInt64(&o.stats.TotalToolCalls, int64(len(state.ToolResults)))

	if len(state.Knowledge) > 0 {
		atomic.AddInt64(&o.stats.KnowledgeHits, 1)
	}
}

// extractMemCells extracts atomic memories from a conversation exchange.
// CR-027: MemCell Atomic Memory Extraction
// This runs asynchronously after successful responses.
func (o *Orchestrator) extractMemCells(ctx context.Context, req *Request, resp *Response) {
	log := logging.Global()

	// Create a background context with timeout (don't block on parent cancellation)
	extractCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now()

	// Build conversation turns from request and response
	turns := []memcell.ConversationTurn{
		{
			Role:      "user",
			Content:   req.Input,
			Timestamp: now.Add(-resp.Duration), // Approximate request time
		},
		{
			Role:      "assistant",
			Content:   resp.Content,
			Timestamp: now,
		},
	}

	// Extract and store MemCells
	if err := o.memcell.ExtractFromConversation(extractCtx, turns); err != nil {
		log.Debug("[MemCell] Extraction failed: %v", err)
		return
	}

	log.Debug("[MemCell] Extraction complete for request %s", req.ID)
}
