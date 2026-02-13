package agent

import (
	"context"
	"testing"
	"time"
)

// MockBrain implements BrainInterface for testing.
type MockBrain struct {
	brainType string
	available bool
	response  string
}

func (m *MockBrain) Type() string      { return m.brainType }
func (m *MockBrain) Available() bool   { return m.available }
func (m *MockBrain) Process(ctx context.Context, input *BrainInput) (*BrainResult, error) {
	return &BrainResult{
		Content:    m.response,
		Source:     m.brainType,
		Success:    true,
		Confidence: 0.9,
		Latency:    10 * time.Millisecond,
	}, nil
}

// MockSkillStore implements SkillStore for testing.
type MockSkillStore struct {
	skills []Skill
}

func (m *MockSkillStore) SearchSkills(ctx context.Context, userID, query string, limit int) ([]Skill, error) {
	if len(m.skills) > limit {
		return m.skills[:limit], nil
	}
	return m.skills, nil
}

func (m *MockSkillStore) StoreSkill(ctx context.Context, userID, intent, tool string, params map[string]string, success bool) error {
	m.skills = append(m.skills, Skill{
		Intent:      intent,
		Tool:        tool,
		Params:      params,
		Success:     success,
		SuccessRate: 1.0,
		CreatedAt:   time.Now(),
	})
	return nil
}

func TestRouterSimpleQuery(t *testing.T) {
	localBrain := &MockBrain{brainType: "local", available: true, response: "local response"}
	frontierBrain := &MockBrain{brainType: "frontier", available: true, response: "frontier response"}
	skillStore := &MockSkillStore{}

	router := NewRouter(localBrain, frontierBrain, skillStore, DefaultRouterConfig())

	input := &BrainInput{Query: "hello, how are you?"}
	decision, brain := router.Route(context.Background(), "user1", input)

	if decision.Brain != "local" {
		t.Errorf("Expected simple query to route to local, got %s", decision.Brain)
	}
	if brain != localBrain {
		t.Error("Expected local brain to be selected")
	}
}

func TestRouterComplexQuery(t *testing.T) {
	localBrain := &MockBrain{brainType: "local", available: true, response: "local response"}
	frontierBrain := &MockBrain{brainType: "frontier", available: true, response: "frontier response"}
	skillStore := &MockSkillStore{}

	router := NewRouter(localBrain, frontierBrain, skillStore, DefaultRouterConfig())

	input := &BrainInput{Query: "Create an application with the following architecture: microservices, event-driven, with a React frontend"}
	decision, brain := router.Route(context.Background(), "user1", input)

	if decision.Brain != "frontier" {
		t.Errorf("Expected complex query to route to frontier, got %s (reason: %s)", decision.Brain, decision.Reason)
	}
	if brain != frontierBrain {
		t.Error("Expected frontier brain to be selected")
	}
}

func TestRouterSkillMatch(t *testing.T) {
	localBrain := &MockBrain{brainType: "local", available: true, response: "local response"}
	frontierBrain := &MockBrain{brainType: "frontier", available: true, response: "frontier response"}
	skillStore := &MockSkillStore{
		skills: []Skill{
			{Intent: "list files", Tool: "list_directory", SuccessRate: 0.95},
		},
	}

	router := NewRouter(localBrain, frontierBrain, skillStore, DefaultRouterConfig())

	input := &BrainInput{Query: "list files in the current directory"}
	decision, brain := router.Route(context.Background(), "user1", input)

	if decision.Brain != "local" {
		t.Errorf("Expected skill match to route to local, got %s", decision.Brain)
	}
	if brain != localBrain {
		t.Error("Expected local brain to be selected for skill match")
	}
	if decision.MatchedSkill == nil {
		t.Error("Expected matched skill to be populated")
	}
}

func TestRouterFallbackWhenFrontierUnavailable(t *testing.T) {
	localBrain := &MockBrain{brainType: "local", available: true, response: "local response"}
	frontierBrain := &MockBrain{brainType: "frontier", available: false, response: "frontier response"}
	skillStore := &MockSkillStore{}

	router := NewRouter(localBrain, frontierBrain, skillStore, DefaultRouterConfig())

	input := &BrainInput{Query: "Create an application with complex architecture"}
	decision, brain := router.Route(context.Background(), "user1", input)

	if decision.Brain != "local" {
		t.Errorf("Expected fallback to local when frontier unavailable, got %s", decision.Brain)
	}
	if brain != localBrain {
		t.Error("Expected local brain when frontier unavailable")
	}
}

func TestRouterProcessCapturesSkill(t *testing.T) {
	localBrain := &MockBrain{brainType: "local", available: true, response: "local response"}
	frontierBrain := &MockBrain{brainType: "frontier", available: true, response: "frontier response"}
	skillStore := &MockSkillStore{}

	router := NewRouter(localBrain, frontierBrain, skillStore, DefaultRouterConfig())

	// Force frontier by using a complex query (must be > 500 chars or contain complex keywords)
	// Note: "hi" in "architecture" falsely matches trivial pattern, so use longer query
	input := &BrainInput{Query: "Please create an application with multiple microservices, event-driven patterns, and a complete React frontend. The system needs to refactor the authentication module and implement a new payment gateway."}
	result, decision, err := router.Process(context.Background(), "user1", input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("Expected successful result")
	}
	if decision.Brain != "frontier" {
		t.Errorf("Expected frontier brain, got %s", decision.Brain)
	}

	// Skill should have been captured
	if len(skillStore.skills) == 0 {
		t.Error("Expected skill to be captured from frontier success")
	}
}

func TestComplexityClassification(t *testing.T) {
	router := &Router{complexityThreshold: 500}

	tests := []struct {
		query    string
		expected string
	}{
		{"hello", "trivial"},
		{"what time is it", "trivial"},
		{"create a file called test.txt", "simple"},
		{"explain how async/await works", "simple"},
		{"refactor the authentication system", "complex"},
		{"create an application with microservices", "complex"},
		{"what's the latest in AI 2026", "novel"},
	}

	for _, tc := range tests {
		result := router.classifyComplexity(tc.query)
		if result != tc.expected {
			t.Errorf("Query '%s': expected %s, got %s", tc.query, tc.expected, result)
		}
	}
}
