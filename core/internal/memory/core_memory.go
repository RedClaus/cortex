// Package memory implements CR-003's LLM-managed memory system (MemGPT pattern).
// It provides a three-tier memory hierarchy:
//   - Core Memory: Always in context (persona, user facts, project info)
//   - Recall Memory: Recent conversation buffer
//   - Archival Memory: Long-term knowledge store (via knowledge package)
package memory

import (
	"time"
)

// CoreMemory holds always-in-context information.
// This is injected into every prompt, with size varying by lane.
type CoreMemory struct {
	Persona *PersonaMemory `json:"persona"`
	User    *UserMemory    `json:"user"`
	Project *ProjectMemory `json:"project"`
}

// PersonaMemory stores AI identity information.
// This links to CR-002's PersonaCore but stores runtime state.
type PersonaMemory struct {
	Name           string   `json:"name"`
	Role           string   `json:"role"`
	CurrentMode    string   `json:"current_mode,omitempty"`
	Expertise      []string `json:"expertise,omitempty"`
	ActiveBehavior string   `json:"active_behavior,omitempty"`
}

// UserMemory stores learned facts about the user.
// This enables personalized responses and remembering preferences.
type UserMemory struct {
	// Core facts
	Name       string `json:"name,omitempty"`
	Role       string `json:"role,omitempty"`
	Experience string `json:"experience,omitempty"` // e.g., "senior", "junior", "student"

	// Environment
	OS     string `json:"os,omitempty"`
	Shell  string `json:"shell,omitempty"`
	Editor string `json:"editor,omitempty"`

	// Preferences
	Preferences    []UserPreference `json:"preferences,omitempty"`
	PrefersConcise bool             `json:"prefers_concise,omitempty"`
	PrefersVerbose bool             `json:"prefers_verbose,omitempty"`

	// Custom facts (learned from conversations)
	CustomFacts []UserFact `json:"custom_facts,omitempty"`

	LastUpdated time.Time `json:"last_updated"`
}

// UserPreference represents a learned user preference.
type UserPreference struct {
	Category    string    `json:"category"`     // e.g., "code_style", "output_format", "tools"
	Preference  string    `json:"preference"`   // e.g., "prefers tabs over spaces"
	Confidence  float64   `json:"confidence"`   // 0.0-1.0, how confident we are
	LearnedFrom string    `json:"learned_from"` // Source: "explicit", "inferred", "correction"
	LearnedAt   time.Time `json:"learned_at"`
}

// UserFact represents a custom fact about the user.
type UserFact struct {
	Fact      string    `json:"fact"`       // The fact itself
	Source    string    `json:"source"`     // How we learned it: "user_stated", "llm_inferred"
	CreatedAt time.Time `json:"created_at"`
}

// ProjectMemory stores current project context.
// This is automatically populated from fingerprinting.
type ProjectMemory struct {
	Name        string            `json:"name,omitempty"`
	Path        string            `json:"path,omitempty"`
	Type        string            `json:"type,omitempty"` // e.g., "go", "node", "python"
	TechStack   []string          `json:"tech_stack,omitempty"`
	Conventions []string          `json:"conventions,omitempty"` // e.g., "uses gofmt", "prettier for formatting"
	GitBranch   string            `json:"git_branch,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"` // Additional project-specific info
	LastUpdated time.Time         `json:"last_updated"`
}

// NewUserMemory creates a new empty UserMemory.
func NewUserMemory() *UserMemory {
	return &UserMemory{
		Preferences: make([]UserPreference, 0),
		CustomFacts: make([]UserFact, 0),
		LastUpdated: time.Now(),
	}
}

// NewProjectMemory creates a new empty ProjectMemory.
func NewProjectMemory() *ProjectMemory {
	return &ProjectMemory{
		TechStack:   make([]string, 0),
		Conventions: make([]string, 0),
		Metadata:    make(map[string]string),
		LastUpdated: time.Now(),
	}
}

// HasPreference checks if a preference exists in the given category.
func (u *UserMemory) HasPreference(category string) bool {
	for _, p := range u.Preferences {
		if p.Category == category {
			return true
		}
	}
	return false
}

// GetPreference returns the preference for a category, or empty string if not found.
func (u *UserMemory) GetPreference(category string) string {
	for _, p := range u.Preferences {
		if p.Category == category {
			return p.Preference
		}
	}
	return ""
}

// AddFact adds a new custom fact if it doesn't already exist.
func (u *UserMemory) AddFact(fact, source string) {
	// Check for duplicate
	for _, f := range u.CustomFacts {
		if f.Fact == fact {
			return
		}
	}

	u.CustomFacts = append(u.CustomFacts, UserFact{
		Fact:      fact,
		Source:    source,
		CreatedAt: time.Now(),
	})
	u.LastUpdated = time.Now()
}

// TokenEstimate returns an approximate token count for the user memory.
// Uses the rough estimate of 1 token per 4 characters.
func (u *UserMemory) TokenEstimate() int {
	chars := 0

	chars += len(u.Name) + len(u.Role) + len(u.Experience)
	chars += len(u.OS) + len(u.Shell) + len(u.Editor)

	for _, p := range u.Preferences {
		chars += len(p.Category) + len(p.Preference) + 20 // overhead
	}

	for _, f := range u.CustomFacts {
		chars += len(f.Fact) + 10 // overhead
	}

	return chars / CharsPerToken
}

// TokenEstimate returns an approximate token count for project memory.
func (p *ProjectMemory) TokenEstimate() int {
	chars := 0

	chars += len(p.Name) + len(p.Path) + len(p.Type)

	for _, t := range p.TechStack {
		chars += len(t) + 2
	}

	for _, c := range p.Conventions {
		chars += len(c) + 2
	}

	chars += len(p.GitBranch)

	for k, v := range p.Metadata {
		chars += len(k) + len(v) + 4
	}

	return chars / CharsPerToken
}
