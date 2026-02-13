// Package sdk provides the public SDK for the Cortex Coder Agent
package sdk

import (
	"context"
	"fmt"

	"github.com/RedClaus/cortex-coder-agent/pkg/agent"
	"github.com/RedClaus/cortex-coder-agent/pkg/cortexbrain"
	"github.com/RedClaus/cortex-coder-agent/pkg/extensions"
	"github.com/RedClaus/cortex-coder-agent/pkg/skills"
	"github.com/RedClaus/cortex-coder-agent/pkg/tools"
)

// CortexCoder is the main SDK entry point
type CortexCoder struct {
	agent       *agent.Agent
	skillsMgr   *skills.Registry
	extensions  *extensions.Manager
	toolsMgr    *tools.Manager
	cortexBrain *cortexbrain.Client
}

// Option configures the SDK
type Option func(*CortexCoder)

// WithSkillsPath sets the skills path
func WithSkillsPath(path string) Option {
	return func(c *CortexCoder) {
		c.skillsMgr = skills.NewRegistry(path, "", "")
	}
}

// WithExtensionsPath sets the extensions path
func WithExtensionsPath(path string) Option {
	return func(c *CortexCoder) {
		c.extensions = extensions.NewManager(path)
	}
}

// WithCortexBrain sets the CortexBrain endpoint
func WithCortexBrain(baseURL, wsURL, token string) Option {
	return func(c *CortexCoder) {
		c.cortexBrain = cortexbrain.NewClient(baseURL, wsURL, token)
	}
}

// New creates a new CortexCoder instance
func New(name, model string, opts ...Option) *CortexCoder {
	c := &CortexCoder{
		agent:      agent.New(name, model),
		skillsMgr:  skills.NewRegistry("./skills", "", ""),
		extensions: extensions.NewManager("./extensions"),
		toolsMgr:   tools.NewManager(),
	}
	
	for _, opt := range opts {
		opt(c)
	}
	
	return c
}

// Agent returns the underlying agent
func (c *CortexCoder) Agent() *agent.Agent {
	return c.agent
}

// Skills returns the skills manager
func (c *CortexCoder) Skills() *skills.Registry {
	return c.skillsMgr
}

// Extensions returns the extensions manager
func (c *CortexCoder) Extensions() *extensions.Manager {
	return c.extensions
}

// Tools returns the tools manager
func (c *CortexCoder) Tools() *tools.Manager {
	return c.toolsMgr
}

// CortexBrain returns the CortexBrain client
func (c *CortexCoder) CortexBrain() *cortexbrain.Client {
	return c.cortexBrain
}

// Execute runs the agent with the given input
func (c *CortexCoder) Execute(ctx context.Context, input string) (string, error) {
	// Check CortexBrain for relevant knowledge if available
	if c.cortexBrain != nil {
		result, err := c.cortexBrain.SearchKnowledge(ctx, cortexbrain.SearchRequest{
			Query: input,
			Limit: 5,
		})
		if err == nil && len(result.Results) > 0 {
			// Add relevant knowledge to context
			ctx = context.WithValue(ctx, "knowledge", result.Results)
		}
	}
	
	return c.agent.Execute(ctx, input)
}

// LoadSkills loads all skills from the skills directory
func (c *CortexCoder) LoadSkills() error {
	return c.skillsMgr.LoadAll()
}

// LoadExtensions loads all extensions from the extensions directory
func (c *CortexCoder) LoadExtensions() error {
	return c.extensions.LoadAll()
}

// Health checks the health of all components
func (c *CortexCoder) Health(ctx context.Context) map[string]string {
	status := make(map[string]string)
	status["agent"] = "ready"
	status["skills"] = fmt.Sprintf("%d loaded", len(c.skillsMgr.List()))
	status["extensions"] = fmt.Sprintf("%d loaded", len(c.extensions.List()))
	status["tools"] = fmt.Sprintf("%d loaded", len(c.toolsMgr.List()))
	
	if c.cortexBrain != nil {
		if err := c.cortexBrain.Ping(ctx); err != nil {
			status["cortexbrain"] = "unreachable: " + err.Error()
		} else {
			status["cortexbrain"] = "connected"
		}
	} else {
		status["cortexbrain"] = "not configured"
	}
	
	return status
}
