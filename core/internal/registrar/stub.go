// Package registrar provides stub types for capability registration.
// This is a minimal stub to allow compilation. Full implementation TBD.
package registrar

import (
	"context"
	"time"
)

// Domain represents a capability domain.
type Domain string

const (
	DomainStorage Domain = "storage"
	DomainSystem  Domain = "system"
	DomainNetwork Domain = "network"
)

// Type represents a capability type.
type Type string

const (
	TypeTool    Type = "tool"
	TypeService Type = "service"
)

// Status represents a capability status.
type Status string

const (
	StatusEnabled  Status = "enabled"
	StatusDisabled Status = "disabled"
)

// CapabilityInput represents input to a capability.
type CapabilityInput struct {
	Type   string
	Data   []byte
	Params map[string]interface{}
}

// CapabilityOutput represents output from a capability.
type CapabilityOutput struct {
	Type    string
	Data    []byte
	Success bool
	Error   string
}

// CapabilityHandler handles capability invocation.
type CapabilityHandler func(ctx context.Context, input CapabilityInput) (CapabilityOutput, error)

// Capability represents a registered capability.
type Capability struct {
	ID             string
	Name           string
	Description    string
	Version        string
	Domain         Domain
	Type           Type
	Tags           []string
	InputTypes     []string
	OutputTypes    []string
	IntentPatterns []string
	Examples       []string
	Timeout        time.Duration
	Concurrency    int
	Status         Status
	Handler        CapabilityHandler
	Author         string
	Source         string
	Metadata       map[string]string
}

// Registrar manages capability registration.
type Registrar struct{}

// NewRegistrar creates a new registrar.
func NewRegistrar() *Registrar { return &Registrar{} }

// Register registers a capability.
func (r *Registrar) Register(cap *Capability) error { return nil }

// Get retrieves a capability by ID.
func (r *Registrar) Get(id string) (*Capability, bool) { return nil, false }

// List lists all registered capabilities.
func (r *Registrar) List() []*Capability { return nil }
