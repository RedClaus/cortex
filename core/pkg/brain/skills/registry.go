// Package skills provides cognitive workflow management for CortexBrain.
// Skills are structured approaches to common task types.
package skills

import (
	"regexp"
	"strings"
)

// Skill represents a cognitive workflow.
type Skill struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Triggers    []string `yaml:"triggers"`
	Phases      []string `yaml:"phases"` // Which Double Diamond phases use this skill
	Steps       []string `yaml:"steps"`  // Workflow steps
}

// Registry manages available skills.
type Registry struct {
	skills          map[string]*Skill
	triggerPatterns map[string]*regexp.Regexp
}

// NewRegistry creates a skill registry with default skills.
func NewRegistry() *Registry {
	r := &Registry{
		skills:          make(map[string]*Skill),
		triggerPatterns: make(map[string]*regexp.Regexp),
	}
	r.loadDefaults()
	return r
}

// loadDefaults initializes the skill registry from Octopus skills.
func (r *Registry) loadDefaults() {
	// Flow skills (Double Diamond phases)
	r.register(&Skill{
		Name:        "flow-discover",
		Description: "Divergent research and exploration phase",
		Triggers:    []string{"/discover", "/probe", "research", "explore", "investigate"},
		Phases:      []string{"probe"},
		Steps: []string{
			"1. Define research questions",
			"2. Gather information from multiple sources",
			"3. Analyze findings",
			"4. Synthesize insights",
		},
	})

	r.register(&Skill{
		Name:        "flow-define",
		Description: "Convergent consensus building phase",
		Triggers:    []string{"/define", "/grasp", "define", "requirements", "scope"},
		Phases:      []string{"grasp"},
		Steps: []string{
			"1. Review research findings",
			"2. Identify key requirements",
			"3. Build consensus on approach",
			"4. Document decisions",
		},
	})

	r.register(&Skill{
		Name:        "flow-develop",
		Description: "Divergent implementation phase",
		Triggers:    []string{"/develop", "/tangle", "build", "implement", "code"},
		Phases:      []string{"tangle"},
		Steps: []string{
			"1. Break down into tasks",
			"2. Implement with quality gates",
			"3. Test incrementally",
			"4. Review and refine",
		},
	})

	r.register(&Skill{
		Name:        "flow-deliver",
		Description: "Convergent validation phase",
		Triggers:    []string{"/deliver", "/ink", "ship", "release", "deploy"},
		Phases:      []string{"ink"},
		Steps: []string{
			"1. Final quality check",
			"2. Security review",
			"3. Documentation",
			"4. Deployment",
		},
	})

	// Specialized skills
	r.register(&Skill{
		Name:        "skill-architecture",
		Description: "System architecture and API design",
		Triggers:    []string{"architect", "design system", "api design"},
		Phases:      []string{"grasp", "tangle"},
		Steps: []string{
			"1. Understand requirements",
			"2. Define service boundaries",
			"3. Design API contracts",
			"4. Plan resilience patterns",
			"5. Document architecture",
		},
	})

	r.register(&Skill{
		Name:        "skill-tdd",
		Description: "Test-driven development workflow",
		Triggers:    []string{"tdd", "test first", "test driven"},
		Phases:      []string{"tangle"},
		Steps: []string{
			"1. Write failing test",
			"2. Implement minimal code to pass",
			"3. Refactor",
			"4. Repeat",
		},
	})

	r.register(&Skill{
		Name:        "skill-debug",
		Description: "Systematic debugging approach",
		Triggers:    []string{"debug", "fix bug", "error", "not working"},
		Phases:      []string{"tangle", "ink"},
		Steps: []string{
			"1. Reproduce the issue",
			"2. Gather error information",
			"3. Form hypothesis",
			"4. Test hypothesis",
			"5. Implement fix",
			"6. Verify fix",
		},
	})

	r.register(&Skill{
		Name:        "skill-code-review",
		Description: "Code review and quality assessment",
		Triggers:    []string{"review", "code review", "pr review"},
		Phases:      []string{"ink"},
		Steps: []string{
			"1. Understand context and intent",
			"2. Check for correctness",
			"3. Evaluate design",
			"4. Check for security issues",
			"5. Assess maintainability",
			"6. Provide constructive feedback",
		},
	})

	r.register(&Skill{
		Name:        "skill-security-audit",
		Description: "Security audit and vulnerability assessment",
		Triggers:    []string{"security audit", "security review", "vulnerability"},
		Phases:      []string{"ink"},
		Steps: []string{
			"1. Identify attack surface",
			"2. Check for OWASP Top 10",
			"3. Review authentication/authorization",
			"4. Check input validation",
			"5. Review dependencies",
			"6. Document findings",
		},
	})

	r.register(&Skill{
		Name:        "skill-prd",
		Description: "Product requirements documentation",
		Triggers:    []string{"prd", "requirements doc", "product spec"},
		Phases:      []string{"grasp"},
		Steps: []string{
			"1. Define problem statement",
			"2. Identify user stories",
			"3. Define acceptance criteria",
			"4. Document non-functional requirements",
			"5. Prioritize features",
		},
	})

	r.register(&Skill{
		Name:        "skill-deep-research",
		Description: "In-depth research and analysis",
		Triggers:    []string{"deep research", "thorough analysis", "comprehensive study"},
		Phases:      []string{"probe"},
		Steps: []string{
			"1. Define research scope",
			"2. Identify primary sources",
			"3. Gather and analyze data",
			"4. Cross-reference findings",
			"5. Synthesize conclusions",
		},
	})

	r.register(&Skill{
		Name:        "skill-doc-delivery",
		Description: "Documentation creation and delivery",
		Triggers:    []string{"write docs", "documentation", "readme"},
		Phases:      []string{"ink"},
		Steps: []string{
			"1. Identify audience",
			"2. Outline structure",
			"3. Write content",
			"4. Add examples",
			"5. Review and refine",
		},
	})

	r.register(&Skill{
		Name:        "skill-debate",
		Description: "Multi-perspective analysis and debate",
		Triggers:    []string{"debate", "compare options", "pros and cons"},
		Phases:      []string{"grasp"},
		Steps: []string{
			"1. Define the question",
			"2. Present position A",
			"3. Present position B",
			"4. Analyze trade-offs",
			"5. Synthesize recommendation",
		},
	})

	r.register(&Skill{
		Name:        "skill-brainstorm",
		Description: "Creative ideation and brainstorming",
		Triggers:    []string{"brainstorm", "ideate", "generate ideas"},
		Phases:      []string{"probe"},
		Steps: []string{
			"1. Define the challenge",
			"2. Generate ideas freely",
			"3. Build on ideas",
			"4. Cluster and categorize",
			"5. Evaluate and prioritize",
		},
	})

	r.register(&Skill{
		Name:        "skill-validate",
		Description: "Validation and verification",
		Triggers:    []string{"validate", "verify", "check"},
		Phases:      []string{"ink"},
		Steps: []string{
			"1. Define success criteria",
			"2. Run validation checks",
			"3. Document results",
			"4. Address failures",
			"5. Confirm completion",
		},
	})

	r.register(&Skill{
		Name:        "skill-rollback",
		Description: "Safe rollback procedures",
		Triggers:    []string{"rollback", "revert", "undo"},
		Phases:      []string{"ink"},
		Steps: []string{
			"1. Assess current state",
			"2. Identify rollback point",
			"3. Backup current state",
			"4. Execute rollback",
			"5. Verify restoration",
		},
	})

	// Build trigger patterns
	r.buildTriggerPatterns()
}

// register adds a skill to the registry.
func (r *Registry) register(s *Skill) {
	r.skills[s.Name] = s
}

// buildTriggerPatterns creates regex patterns for skill matching.
func (r *Registry) buildTriggerPatterns() {
	for name, skill := range r.skills {
		pattern := strings.Join(skill.Triggers, "|")
		r.triggerPatterns[name] = regexp.MustCompile("(?i)" + pattern)
	}
}

// Match finds the best skill for a given input.
func (r *Registry) Match(input string) *Skill {
	// Check each skill's triggers
	for name, pattern := range r.triggerPatterns {
		if pattern.MatchString(input) {
			return r.skills[name]
		}
	}
	return nil
}

// Get retrieves a skill by name.
func (r *Registry) Get(name string) *Skill {
	return r.skills[name]
}

// List returns all registered skills.
func (r *Registry) List() []*Skill {
	result := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s)
	}
	return result
}

// ListByPhase returns skills for a specific workflow phase.
func (r *Registry) ListByPhase(phase string) []*Skill {
	var result []*Skill
	for _, s := range r.skills {
		for _, p := range s.Phases {
			if p == phase {
				result = append(result, s)
				break
			}
		}
	}
	return result
}
