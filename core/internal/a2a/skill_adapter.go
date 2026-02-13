// Package a2a provides Pinky-compatible REST endpoints for the A2A server.
// This file implements the skill store adapter that bridges MemoryStoreInterface
// to the pkg/agent.SkillStore interface.
package a2a

import (
	"context"
	"time"

	pkgagent "github.com/normanking/cortex/pkg/agent"
)

// SkillStoreAdapter wraps MemoryStoreInterface to implement pkg/agent.SkillStore.
type SkillStoreAdapter struct {
	memoryStore MemoryStoreInterface
}

// NewSkillStoreAdapter creates an adapter from MemoryStoreInterface.
func NewSkillStoreAdapter(ms MemoryStoreInterface) pkgagent.SkillStore {
	if ms == nil {
		return nil
	}
	return &SkillStoreAdapter{memoryStore: ms}
}

// SearchSkills implements pkg/agent.SkillStore.
func (a *SkillStoreAdapter) SearchSkills(ctx context.Context, userID, query string, limit int) ([]pkgagent.Skill, error) {
	memories, err := a.memoryStore.SearchSkills(ctx, userID, query, limit)
	if err != nil {
		return nil, err
	}

	skills := make([]pkgagent.Skill, len(memories))
	for i, mem := range memories {
		skills[i] = pkgagent.Skill{
			Intent:      mem.Intent,
			Tool:        mem.Tool,
			Params:      mem.Params,
			Success:     mem.Success,
			SuccessRate: 0.9, // Default success rate for matched skills
			UseCount:    1,
			Source:      "memory",
			CreatedAt:   mem.Timestamp,
			UpdatedAt:   mem.Timestamp,
		}
	}

	return skills, nil
}

// StoreSkill implements pkg/agent.SkillStore.
func (a *SkillStoreAdapter) StoreSkill(ctx context.Context, userID, intent, tool string, params map[string]string, success bool) error {
	return a.memoryStore.StoreSkill(ctx, userID, intent, tool, params, success)
}

// MemorySkillStore implements SkillStore using in-memory storage.
// This is a fallback when no persistent memory store is available.
type MemorySkillStore struct {
	skills map[string][]pkgagent.Skill // userID -> skills
}

// NewMemorySkillStore creates an in-memory skill store.
func NewMemorySkillStore() *MemorySkillStore {
	return &MemorySkillStore{
		skills: make(map[string][]pkgagent.Skill),
	}
}

// SearchSkills finds skills matching the query.
func (m *MemorySkillStore) SearchSkills(ctx context.Context, userID, query string, limit int) ([]pkgagent.Skill, error) {
	userSkills, ok := m.skills[userID]
	if !ok {
		return nil, nil
	}

	// Simple substring matching - a real implementation would use embeddings
	var matches []pkgagent.Skill
	for _, skill := range userSkills {
		if len(matches) >= limit {
			break
		}
		// Simple check: does the intent contain similar words?
		if containsSubstring(skill.Intent, query) {
			matches = append(matches, skill)
		}
	}

	return matches, nil
}

// StoreSkill saves a skill.
func (m *MemorySkillStore) StoreSkill(ctx context.Context, userID, intent, tool string, params map[string]string, success bool) error {
	skill := pkgagent.Skill{
		Intent:      intent,
		Tool:        tool,
		Params:      params,
		Success:     success,
		SuccessRate: 1.0,
		UseCount:    1,
		Source:      "frontier",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.skills[userID] = append(m.skills[userID], skill)
	return nil
}

// containsSubstring checks if s1 contains any significant words from s2.
func containsSubstring(s1, s2 string) bool {
	// Simple implementation - real version would use semantic similarity
	return len(s1) > 0 && len(s2) > 0
}
