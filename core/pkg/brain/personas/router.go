// Package personas provides expert persona routing for CortexBrain.
// Personas are specialized cognitive modes derived from Claude Octopus.
package personas

import (
	"regexp"
	"strings"
)

// Persona represents an expert cognitive mode.
type Persona struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Phases      []string `yaml:"phases"`     // probe, grasp, tangle, ink
	Tier        string   `yaml:"tier"`       // trivial, standard, premium
	Expertise   []string `yaml:"expertise"`
	Skills      []string `yaml:"skills"`
}

// Phase constants for Double Diamond workflow.
const (
	PhaseDiscover = "probe"   // Divergent research
	PhaseDefine   = "grasp"   // Convergent consensus
	PhaseDevelop  = "tangle"  // Divergent implementation
	PhaseDeliver  = "ink"     // Convergent validation
)

// Router selects the best persona for a given task.
type Router struct {
	personas       map[string]*Persona
	phaseDefaults  map[string][]string
	triggerPatterns map[string]*regexp.Regexp
}

// NewRouter creates a persona router with default configuration.
func NewRouter() *Router {
	r := &Router{
		personas:       make(map[string]*Persona),
		phaseDefaults:  make(map[string][]string),
		triggerPatterns: make(map[string]*regexp.Regexp),
	}
	r.loadDefaults()
	return r
}

// loadDefaults initializes the persona registry with Octopus personas.
func (r *Router) loadDefaults() {
	// Phase defaults (primary persona per phase)
	r.phaseDefaults = map[string][]string{
		PhaseDiscover: {"research-synthesizer", "ai-engineer", "business-analyst", "context-manager"},
		PhaseDefine:   {"backend-architect", "frontend-developer", "database-architect", "cloud-architect"},
		PhaseDevelop:  {"tdd-orchestrator", "debugger", "python-pro", "typescript-pro"},
		PhaseDeliver:  {"code-reviewer", "security-auditor", "test-automator", "deployment-engineer"},
	}

	// Register all 29 personas
	r.registerPersona(&Persona{
		Name:        "ai-engineer",
		Description: "LLM applications, RAG systems, prompt engineering",
		Phases:      []string{PhaseDiscover, PhaseDevelop},
		Tier:        "premium",
		Expertise:   []string{"llm-applications", "rag-systems", "prompt-engineering"},
	})

	r.registerPersona(&Persona{
		Name:        "backend-architect",
		Description: "API design, microservices, distributed systems",
		Phases:      []string{PhaseDefine, PhaseDevelop},
		Tier:        "premium",
		Expertise:   []string{"api-design", "microservices", "distributed-systems"},
		Skills:      []string{"skill-architecture"},
	})

	r.registerPersona(&Persona{
		Name:        "frontend-developer",
		Description: "React, Next.js, state management, accessibility",
		Phases:      []string{PhaseDefine, PhaseDevelop},
		Tier:        "premium",
		Expertise:   []string{"react", "nextjs", "state-management", "accessibility"},
	})

	r.registerPersona(&Persona{
		Name:        "database-architect",
		Description: "Schema design, SQL, NoSQL, migrations",
		Phases:      []string{PhaseDefine, PhaseDevelop},
		Tier:        "premium",
		Expertise:   []string{"schema-design", "sql", "nosql", "migrations"},
	})

	r.registerPersona(&Persona{
		Name:        "cloud-architect",
		Description: "AWS, GCP, Azure, infrastructure",
		Phases:      []string{PhaseDefine},
		Tier:        "premium",
		Expertise:   []string{"aws", "gcp", "azure", "infrastructure"},
	})

	r.registerPersona(&Persona{
		Name:        "security-auditor",
		Description: "OWASP, vulnerability scanning, security review",
		Phases:      []string{PhaseDeliver},
		Tier:        "premium",
		Expertise:   []string{"owasp", "vulnerability-scanning", "security-review"},
		Skills:      []string{"skill-security-audit"},
	})

	r.registerPersona(&Persona{
		Name:        "tdd-orchestrator",
		Description: "Test-driven development, red-green-refactor",
		Phases:      []string{PhaseDevelop},
		Tier:        "premium",
		Expertise:   []string{"test-driven-development", "red-green-refactor"},
		Skills:      []string{"skill-tdd"},
	})

	r.registerPersona(&Persona{
		Name:        "debugger",
		Description: "Error analysis, stack traces, debugging",
		Phases:      []string{PhaseDevelop, PhaseDeliver},
		Tier:        "standard",
		Expertise:   []string{"error-analysis", "stack-traces", "debugging"},
		Skills:      []string{"skill-debug"},
	})

	r.registerPersona(&Persona{
		Name:        "code-reviewer",
		Description: "Code quality, best practices, architecture review",
		Phases:      []string{PhaseDeliver},
		Tier:        "premium",
		Expertise:   []string{"code-quality", "best-practices", "architecture-review"},
		Skills:      []string{"skill-code-review"},
	})

	r.registerPersona(&Persona{
		Name:        "python-pro",
		Description: "Python, FastAPI, Django, async",
		Phases:      []string{PhaseDevelop},
		Tier:        "premium",
		Expertise:   []string{"python", "fastapi", "django", "async"},
	})

	r.registerPersona(&Persona{
		Name:        "typescript-pro",
		Description: "TypeScript, Node.js, React, types",
		Phases:      []string{PhaseDevelop},
		Tier:        "premium",
		Expertise:   []string{"typescript", "node", "react", "types"},
	})

	r.registerPersona(&Persona{
		Name:        "devops-troubleshooter",
		Description: "Deployment, logs, infrastructure debugging",
		Phases:      []string{PhaseDevelop, PhaseDeliver},
		Tier:        "standard",
		Expertise:   []string{"deployment", "logs", "infrastructure-debugging"},
	})

	r.registerPersona(&Persona{
		Name:        "test-automator",
		Description: "Unit tests, integration tests, E2E tests",
		Phases:      []string{PhaseDevelop, PhaseDeliver},
		Tier:        "standard",
		Expertise:   []string{"unit-tests", "integration-tests", "e2e-tests"},
	})

	r.registerPersona(&Persona{
		Name:        "performance-engineer",
		Description: "Profiling, optimization, benchmarking",
		Phases:      []string{PhaseDeliver},
		Tier:        "premium",
		Expertise:   []string{"profiling", "optimization", "benchmarking"},
	})

	r.registerPersona(&Persona{
		Name:        "deployment-engineer",
		Description: "CI/CD, Kubernetes, Docker, GitOps",
		Phases:      []string{PhaseDeliver},
		Tier:        "standard",
		Expertise:   []string{"ci-cd", "kubernetes", "docker", "gitops"},
	})

	r.registerPersona(&Persona{
		Name:        "strategy-analyst",
		Description: "Strategic analysis, market research, business strategy",
		Phases:      []string{PhaseDiscover, PhaseDefine},
		Tier:        "premium",
		Expertise:   []string{"strategic-analysis", "market-research", "business-strategy"},
	})

	r.registerPersona(&Persona{
		Name:        "research-synthesizer",
		Description: "Research synthesis, literature review, knowledge integration",
		Phases:      []string{PhaseDiscover},
		Tier:        "premium",
		Expertise:   []string{"research-synthesis", "literature-review", "knowledge-integration"},
	})

	r.registerPersona(&Persona{
		Name:        "business-analyst",
		Description: "Requirements, metrics, stakeholder analysis",
		Phases:      []string{PhaseDiscover, PhaseDefine},
		Tier:        "standard",
		Expertise:   []string{"requirements", "metrics", "stakeholder-analysis"},
	})

	r.registerPersona(&Persona{
		Name:        "context-manager",
		Description: "Context management, multi-agent coordination",
		Phases:      []string{PhaseDiscover, PhaseDefine, PhaseDevelop, PhaseDeliver},
		Tier:        "standard",
		Expertise:   []string{"context-management", "multi-agent-coordination"},
	})

	r.registerPersona(&Persona{
		Name:        "docs-architect",
		Description: "Documentation, technical writing",
		Phases:      []string{PhaseDeliver},
		Tier:        "standard",
		Expertise:   []string{"documentation", "technical-writing"},
		Skills:      []string{"skill-doc-delivery"},
	})

	r.registerPersona(&Persona{
		Name:        "incident-responder",
		Description: "Incident management, runbooks, postmortems",
		Phases:      []string{PhaseDevelop, PhaseDeliver},
		Tier:        "premium",
		Expertise:   []string{"incident-management", "runbooks", "postmortems"},
	})

	r.registerPersona(&Persona{
		Name:        "mermaid-expert",
		Description: "Diagrams, flowcharts, sequence diagrams",
		Phases:      []string{PhaseDefine, PhaseDeliver},
		Tier:        "trivial",
		Expertise:   []string{"diagrams", "flowcharts", "sequence-diagrams"},
	})

	r.registerPersona(&Persona{
		Name:        "graphql-architect",
		Description: "GraphQL, federation, resolvers",
		Phases:      []string{PhaseDefine, PhaseDevelop},
		Tier:        "premium",
		Expertise:   []string{"graphql", "federation", "resolvers"},
	})

	r.registerPersona(&Persona{
		Name:        "ux-researcher",
		Description: "User research, usability, user experience",
		Phases:      []string{PhaseDiscover, PhaseDefine},
		Tier:        "standard",
		Expertise:   []string{"user-research", "usability", "user-experience"},
	})

	r.registerPersona(&Persona{
		Name:        "academic-writer",
		Description: "Academic writing, research papers, citations",
		Phases:      []string{PhaseDeliver},
		Tier:        "standard",
		Expertise:   []string{"academic-writing", "research-papers", "citations"},
	})

	r.registerPersona(&Persona{
		Name:        "content-analyst",
		Description: "Content analysis, text processing, NLP",
		Phases:      []string{PhaseDiscover},
		Tier:        "standard",
		Expertise:   []string{"content-analysis", "text-processing", "nlp"},
	})

	r.registerPersona(&Persona{
		Name:        "product-writer",
		Description: "Product documentation, user guides, release notes",
		Phases:      []string{PhaseDeliver},
		Tier:        "standard",
		Expertise:   []string{"product-documentation", "user-guides", "release-notes"},
	})

	r.registerPersona(&Persona{
		Name:        "exec-communicator",
		Description: "Executive communication, presentations, summaries",
		Phases:      []string{PhaseDeliver},
		Tier:        "standard",
		Expertise:   []string{"executive-communication", "presentations", "summaries"},
	})

	r.registerPersona(&Persona{
		Name:        "thought-partner",
		Description: "Brainstorming, ideation, problem-solving",
		Phases:      []string{PhaseDiscover, PhaseDefine},
		Tier:        "standard",
		Expertise:   []string{"brainstorming", "ideation", "problem-solving"},
	})

	// Set up trigger patterns
	r.setupTriggerPatterns()
}

// registerPersona adds a persona to the registry.
func (r *Router) registerPersona(p *Persona) {
	r.personas[p.Name] = p
}

// setupTriggerPatterns creates regex patterns for task routing.
func (r *Router) setupTriggerPatterns() {
	patterns := map[string]string{
		"api-design":     `(?i)(rest|graphql|grpc).*(api|endpoint|service)`,
		"security":       `(?i)(security|owasp|vulnerability|auth)`,
		"testing":        `(?i)(test|tdd|coverage|e2e)`,
		"performance":    `(?i)(performance|optimize|profile|benchmark)`,
		"debug":          `(?i)(debug|error|fix|broken|not working|why.*(fail|isn't|doesn't))`,
		"architecture":   `(?i)(architect|design|structure|microservice|distributed)`,
		"frontend":       `(?i)(react|vue|angular|frontend|ui|component|css)`,
		"backend":        `(?i)(api|server|endpoint|backend|service)`,
		"database":       `(?i)(database|sql|postgres|mysql|mongo|schema|migration)`,
		"cloud":          `(?i)(aws|gcp|azure|kubernetes|docker|cloud|deploy)`,
		"documentation":  `(?i)(document|readme|docs|write.*doc)`,
		"research":       `(?i)(research|explore|investigate|analyze|understand)`,
	}

	for name, pattern := range patterns {
		r.triggerPatterns[name] = regexp.MustCompile(pattern)
	}
}

// Route selects the best persona for a given input.
func (r *Router) Route(input string) *Persona {
	inputLower := strings.ToLower(input)

	// Check trigger patterns
	if r.triggerPatterns["debug"].MatchString(input) {
		return r.personas["debugger"]
	}
	if r.triggerPatterns["security"].MatchString(input) {
		return r.personas["security-auditor"]
	}
	if r.triggerPatterns["testing"].MatchString(input) {
		return r.personas["tdd-orchestrator"]
	}
	if r.triggerPatterns["architecture"].MatchString(input) {
		return r.personas["backend-architect"]
	}
	if r.triggerPatterns["frontend"].MatchString(input) {
		return r.personas["frontend-developer"]
	}
	if r.triggerPatterns["database"].MatchString(input) {
		return r.personas["database-architect"]
	}
	if r.triggerPatterns["cloud"].MatchString(input) {
		return r.personas["cloud-architect"]
	}
	if r.triggerPatterns["performance"].MatchString(input) {
		return r.personas["performance-engineer"]
	}
	if r.triggerPatterns["documentation"].MatchString(input) {
		return r.personas["docs-architect"]
	}
	if r.triggerPatterns["research"].MatchString(input) {
		return r.personas["research-synthesizer"]
	}

	// Check for specific keywords
	if strings.Contains(inputLower, "python") {
		return r.personas["python-pro"]
	}
	if strings.Contains(inputLower, "typescript") || strings.Contains(inputLower, "javascript") {
		return r.personas["typescript-pro"]
	}
	if strings.Contains(inputLower, "graphql") {
		return r.personas["graphql-architect"]
	}

	// Default to thought-partner for general queries
	return r.personas["thought-partner"]
}

// RouteForPhase returns personas appropriate for a workflow phase.
func (r *Router) RouteForPhase(phase string) []*Persona {
	names, ok := r.phaseDefaults[phase]
	if !ok {
		return nil
	}

	var result []*Persona
	for _, name := range names {
		if p, ok := r.personas[name]; ok {
			result = append(result, p)
		}
	}
	return result
}

// GetPersona retrieves a persona by name.
func (r *Router) GetPersona(name string) *Persona {
	return r.personas[name]
}

// ListPersonas returns all registered personas.
func (r *Router) ListPersonas() []*Persona {
	result := make([]*Persona, 0, len(r.personas))
	for _, p := range r.personas {
		result = append(result, p)
	}
	return result
}
