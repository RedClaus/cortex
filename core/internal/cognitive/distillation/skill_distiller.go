// Package distillation provides template and skill distillation for CortexBrain.
// This file implements the Skill Distillation Agent (SkillRL-inspired experience extraction).
package distillation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
)

// ═══════════════════════════════════════════════════════════════════════════════
// SKILL DISTILLATION AGENT
// Extracts reusable skills from Observational Memory reflections
// ═══════════════════════════════════════════════════════════════════════════════

// SkillDistiller analyzes reflections and extracts reusable patterns.
type SkillDistiller struct {
	registry *cognitive.DynamicSkillRegistry
	llm      cognitive.SimpleChatProvider
	config   *cognitive.SkillDistillationConfig
	running  bool
	stopCh   chan struct{}
}

// NewSkillDistiller creates a new skill distillation agent.
func NewSkillDistiller(registry *cognitive.DynamicSkillRegistry, llm cognitive.SimpleChatProvider, config *cognitive.SkillDistillationConfig) *SkillDistiller {
	if config == nil {
		config = cognitive.DefaultSkillDistillationConfig()
	}

	return &SkillDistiller{
		registry: registry,
		llm:      llm,
		config:   config,
		stopCh:   make(chan struct{}),
	}
}

// SkillDistillerSystemPrompt guides the LLM in pattern extraction.
const SkillDistillerSystemPrompt = `You are a Skill Distillation Agent. Your job is to analyze reflection patterns and extract reusable skills and failure lessons.

## Skill Extraction Guidelines

Analyze the reflections for:
1. **Success Patterns** - Repeated strategies that led to good outcomes
2. **Failure Lessons** - Mistakes that should be avoided in the future

## Skill Classification

### Skill Types:
- GENERAL: Universal patterns applicable to any task (e.g., "always verify inputs before processing")
- TASK_SPECIFIC: Category-level patterns (e.g., "when writing tests, start with edge cases")

### For Success Patterns, extract:
- name: Short descriptive name
- type: GENERAL or TASK_SPECIFIC
- category: If TASK_SPECIFIC, what category? (coding, debugging, design, etc.)
- description: What this pattern does (1-2 sentences)
- when_to_apply: Under what conditions should this be used?
- steps: Optional numbered steps to follow
- confidence: How confident are you this is a reliable pattern? (0.0-1.0)

### For Failure Lessons, extract:
- name: Short descriptive name
- type: GENERAL or TASK_SPECIFIC
- category: If TASK_SPECIFIC, what category?
- description: What went wrong
- error_signature: How to detect this problem (keywords, patterns)
- recovery: How to fix it when it happens
- prevention: How to avoid it in the future
- confidence: How confident are you this is a real anti-pattern? (0.0-1.0)

## Output Format (YAML)

---
success_patterns:
  - name: "Pattern Name"
    type: GENERAL
    category: ""
    description: "What this pattern achieves"
    when_to_apply: "When to use this"
    steps:
      - "Step 1"
      - "Step 2"
    confidence: 0.8

failure_lessons:
  - name: "Anti-pattern Name"
    type: TASK_SPECIFIC
    category: "debugging"
    description: "What went wrong"
    error_signature: "Keywords or patterns to detect"
    recovery: "How to fix"
    prevention: "How to avoid"
    confidence: 0.75
---

Only extract patterns with confidence >= 0.6. Be conservative - it's better to miss a pattern than to create a false one.

Analyze the following reflections:`

// Run starts the background distillation loop.
func (sd *SkillDistiller) Run(ctx context.Context) {
	ticker := time.NewTicker(sd.config.Interval)
	defer ticker.Stop()

	sd.running = true

	for {
		select {
		case <-ctx.Done():
			sd.running = false
			return
		case <-sd.stopCh:
			sd.running = false
			return
		case <-ticker.C:
			// Distillation runs periodically
			// In production, this would iterate over all agents
		}
	}
}

// Stop gracefully shuts down the distiller.
func (sd *SkillDistiller) Stop() {
	if sd.running {
		close(sd.stopCh)
	}
}

// DistillFromReflections analyzes reflections and creates skills.
func (sd *SkillDistiller) DistillFromReflections(ctx context.Context, reflections []*ReflectionInput, agentID string) (*cognitive.DistillationOutput, error) {
	if len(reflections) < sd.config.MinReflections {
		return nil, fmt.Errorf("need at least %d reflections, got %d", sd.config.MinReflections, len(reflections))
	}

	// Build prompt from reflections
	var sb strings.Builder
	for i, ref := range reflections {
		sb.WriteString(fmt.Sprintf("\n### Reflection %d [%s] - Pattern: %s\n",
			i+1, ref.Timestamp.Format(time.RFC3339), ref.Pattern))
		sb.WriteString(ref.Content)
		sb.WriteString("\n---\n")
	}

	// Call LLM to extract patterns
	messages := []cognitive.ChatMessage{
		{Role: "user", Content: sb.String()},
	}

	response, err := sd.llm.Chat(ctx, messages, SkillDistillerSystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("llm distillation failed: %w", err)
	}

	// Parse the response
	output := sd.parseDistillationOutput(response, reflections, agentID)

	// Create skills in registry
	for _, pattern := range output.SuccessPatterns {
		if pattern.Confidence >= sd.config.ConfidenceThreshold {
			skill := sd.patternToSkill(pattern, agentID)
			if err := sd.registry.CreateSkill(ctx, skill); err != nil {
				// Log but continue - don't fail the whole operation
				continue
			}
		}
	}

	// Create failure patterns in registry
	for _, lesson := range output.FailureLessons {
		if lesson.Confidence >= sd.config.ConfidenceThreshold {
			fp := sd.lessonToFailurePattern(lesson, agentID)
			if err := sd.registry.CreateFailurePattern(ctx, fp); err != nil {
				continue
			}
		}
	}

	return output, nil
}

// ReflectionInput is a simplified reflection for distillation input.
type ReflectionInput struct {
	ID        string
	Content   string
	Timestamp time.Time
	Pattern   string
}

// parseDistillationOutput extracts structured output from LLM response.
func (sd *SkillDistiller) parseDistillationOutput(response string, reflections []*ReflectionInput, agentID string) *cognitive.DistillationOutput {
	output := &cognitive.DistillationOutput{
		SuccessPatterns: make([]*cognitive.SuccessPattern, 0),
		FailureLessons:  make([]*cognitive.FailureLesson, 0),
	}

	// Extract reflection IDs for provenance
	refIDs := make([]string, len(reflections))
	for i, ref := range reflections {
		refIDs[i] = ref.ID
	}

	// Parse success patterns
	output.SuccessPatterns = sd.parseSuccessPatterns(response, refIDs)

	// Parse failure lessons
	output.FailureLessons = sd.parseFailureLessons(response, refIDs)

	return output
}

// parseSuccessPatterns extracts success patterns from YAML response.
func (sd *SkillDistiller) parseSuccessPatterns(response string, sourceRefs []string) []*cognitive.SuccessPattern {
	patterns := make([]*cognitive.SuccessPattern, 0)

	// Find success_patterns section
	startIdx := strings.Index(response, "success_patterns:")
	if startIdx == -1 {
		return patterns
	}

	// Find end (next top-level key or end of YAML block)
	endMarkers := []string{"failure_lessons:", "---"}
	endIdx := len(response)
	for _, marker := range endMarkers {
		idx := strings.Index(response[startIdx+17:], marker)
		if idx != -1 && startIdx+17+idx < endIdx {
			endIdx = startIdx + 17 + idx
		}
	}

	section := response[startIdx:endIdx]

	// Parse each pattern (simplified parsing)
	patternBlocks := strings.Split(section, "- name:")
	for i, block := range patternBlocks {
		if i == 0 || strings.TrimSpace(block) == "" {
			continue
		}

		pattern := &cognitive.SuccessPattern{
			SourceRefs: sourceRefs,
		}

		// Extract fields
		pattern.Name = extractField(block, "", "\n")
		pattern.Type = extractSkillType(block)
		pattern.Category = extractField(block, "category:", "\n")
		pattern.Description = extractField(block, "description:", "\n")
		pattern.WhenToApply = extractField(block, "when_to_apply:", "\n")
		pattern.Confidence = extractConfidence(block)
		pattern.Steps = extractSteps(block)

		if pattern.Name != "" && pattern.Description != "" {
			patterns = append(patterns, pattern)
		}
	}

	return patterns
}

// parseFailureLessons extracts failure lessons from YAML response.
func (sd *SkillDistiller) parseFailureLessons(response string, sourceRefs []string) []*cognitive.FailureLesson {
	lessons := make([]*cognitive.FailureLesson, 0)

	// Find failure_lessons section
	startIdx := strings.Index(response, "failure_lessons:")
	if startIdx == -1 {
		return lessons
	}

	section := response[startIdx:]

	// Parse each lesson (simplified parsing)
	lessonBlocks := strings.Split(section, "- name:")
	for i, block := range lessonBlocks {
		if i == 0 || strings.TrimSpace(block) == "" {
			continue
		}

		lesson := &cognitive.FailureLesson{
			SourceRefs: sourceRefs,
		}

		// Extract fields
		lesson.Name = extractField(block, "", "\n")
		lesson.Type = extractSkillType(block)
		lesson.Category = extractField(block, "category:", "\n")
		lesson.Description = extractField(block, "description:", "\n")
		lesson.ErrorSignature = extractField(block, "error_signature:", "\n")
		lesson.Recovery = extractField(block, "recovery:", "\n")
		lesson.Prevention = extractField(block, "prevention:", "\n")
		lesson.Confidence = extractConfidence(block)

		if lesson.Name != "" && lesson.Description != "" {
			lessons = append(lessons, lesson)
		}
	}

	return lessons
}

// patternToSkill converts a success pattern to a DynamicSkill.
func (sd *SkillDistiller) patternToSkill(pattern *cognitive.SuccessPattern, agentID string) *cognitive.DynamicSkill {
	now := time.Now()
	return &cognitive.DynamicSkill{
		ID:                generateID(),
		Name:              pattern.Name,
		Type:              pattern.Type,
		Category:          pattern.Category,
		Source:            cognitive.SkillSourceDistilled,
		Status:            cognitive.SkillStatusProbation,
		Description:       pattern.Description,
		WhenToApply:       pattern.WhenToApply,
		Steps:             pattern.Steps,
		SourceReflections: pattern.SourceRefs,
		SourceAgents:      []string{agentID},
		Confidence:        pattern.Confidence,
		Version:           1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// lessonToFailurePattern converts a failure lesson to a FailurePattern.
func (sd *SkillDistiller) lessonToFailurePattern(lesson *cognitive.FailureLesson, agentID string) *cognitive.FailurePattern {
	return &cognitive.FailurePattern{
		ID:                generateID(),
		Name:              lesson.Name,
		Type:              lesson.Type,
		Category:          lesson.Category,
		Description:       lesson.Description,
		ErrorSignature:    lesson.ErrorSignature,
		Recovery:          lesson.Recovery,
		Prevention:        lesson.Prevention,
		SourceReflections: lesson.SourceRefs,
		SourceAgents:      []string{agentID},
		Confidence:        lesson.Confidence,
		CreatedAt:         time.Now(),
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// CROSS-AGENT LEARNING
// ═══════════════════════════════════════════════════════════════════════════════

// ShareSkillCrossAgent creates a shared copy of a high-confidence skill.
func (sd *SkillDistiller) ShareSkillCrossAgent(ctx context.Context, skillID, targetAgentID string) (*cognitive.DynamicSkill, error) {
	if !sd.config.CrossAgentLearning {
		return nil, fmt.Errorf("cross-agent learning is disabled")
	}

	// Get the original skill
	original, err := sd.registry.GetSkill(ctx, skillID)
	if err != nil {
		return nil, fmt.Errorf("get skill for sharing: %w", err)
	}

	// Check if it meets sharing threshold
	if original.Confidence < sd.config.ShareThreshold {
		return nil, fmt.Errorf("skill confidence %.2f below share threshold %.2f",
			original.Confidence, sd.config.ShareThreshold)
	}

	// Create shared copy
	shared := &cognitive.DynamicSkill{
		ID:                generateID(),
		Name:              original.Name,
		Type:              original.Type,
		Category:          original.Category,
		Source:            cognitive.SkillSourceShared,
		Status:            cognitive.SkillStatusProbation, // New agent must validate
		Description:       original.Description,
		WhenToApply:       original.WhenToApply,
		Steps:             original.Steps,
		Examples:          original.Examples,
		Intent:            original.Intent,
		Keywords:          original.Keywords,
		SourceReflections: original.SourceReflections,
		SourceAgents:      append(original.SourceAgents, targetAgentID),
		Confidence:        original.Confidence * 0.8, // Reduce confidence for new context
		ParentID:          original.ID,
		Version:           1,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if err := sd.registry.CreateSkill(ctx, shared); err != nil {
		return nil, fmt.Errorf("create shared skill: %w", err)
	}

	return shared, nil
}

// GetShareableSkills returns skills that meet the cross-agent sharing threshold.
func (sd *SkillDistiller) GetShareableSkills(ctx context.Context) ([]*cognitive.DynamicSkill, error) {
	if !sd.config.CrossAgentLearning {
		return nil, nil
	}

	activeStatus := cognitive.SkillStatusActive
	skills, err := sd.registry.ListSkills(ctx, nil, &activeStatus)
	if err != nil {
		return nil, err
	}

	shareable := make([]*cognitive.DynamicSkill, 0)
	for _, skill := range skills {
		if skill.Confidence >= sd.config.ShareThreshold {
			shareable = append(shareable, skill)
		}
	}

	return shareable, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// SKILL MATCHING FOR ROUTING
// ═══════════════════════════════════════════════════════════════════════════════

// MatchSkills finds relevant skills for a given input.
func (sd *SkillDistiller) MatchSkills(ctx context.Context, userInput string, limit int) (*cognitive.SkillMatchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	result := &cognitive.SkillMatchResult{
		Skills:   make([]*cognitive.SkillMatch, 0),
		Warnings: make([]*cognitive.FailureWarning, 0),
	}

	// Search skills by keywords
	skills, err := sd.registry.SearchSkills(ctx, userInput, limit)
	if err != nil {
		return nil, fmt.Errorf("search skills: %w", err)
	}

	for _, skill := range skills {
		match := &cognitive.SkillMatch{
			Skill:       skill,
			MatchMethod: "keyword",
			// In production, would compute actual similarity score
			SimilarityScore: skill.Confidence,
		}

		// Determine similarity level
		if match.SimilarityScore >= 0.9 {
			match.SimilarityLevel = cognitive.SimilarityHigh
		} else if match.SimilarityScore >= 0.7 {
			match.SimilarityLevel = cognitive.SimilarityMedium
		} else {
			match.SimilarityLevel = cognitive.SimilarityLow
		}

		result.Skills = append(result.Skills, match)
	}

	// Check for failure pattern warnings
	failurePatterns, err := sd.registry.SearchFailurePatterns(ctx, userInput, 5)
	if err == nil {
		for _, fp := range failurePatterns {
			warning := &cognitive.FailureWarning{
				Pattern:    fp,
				Confidence: fp.Confidence,
				Triggered:  "keyword match",
			}
			result.Warnings = append(result.Warnings, warning)
		}
	}

	result.HasMatch = len(result.Skills) > 0 && result.Skills[0].SimilarityScore >= 0.7

	return result, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// generateID is defined in engine.go

func extractField(block, prefix, suffix string) string {
	var startIdx int
	if prefix == "" {
		startIdx = 0
	} else {
		startIdx = strings.Index(block, prefix)
		if startIdx == -1 {
			return ""
		}
		startIdx += len(prefix)
	}

	endIdx := strings.Index(block[startIdx:], suffix)
	if endIdx == -1 {
		return strings.TrimSpace(block[startIdx:])
	}

	return strings.TrimSpace(strings.Trim(block[startIdx:startIdx+endIdx], "\"' "))
}

func extractSkillType(block string) cognitive.SkillType {
	typeStr := extractField(block, "type:", "\n")
	typeStr = strings.ToUpper(strings.TrimSpace(typeStr))

	switch typeStr {
	case "GENERAL":
		return cognitive.SkillTypeGeneral
	case "TASK_SPECIFIC":
		return cognitive.SkillTypeTaskSpecific
	case "AGENT_SPECIFIC":
		return cognitive.SkillTypeAgentSpecific
	default:
		return cognitive.SkillTypeGeneral
	}
}

func extractConfidence(block string) float64 {
	confStr := extractField(block, "confidence:", "\n")
	var conf float64
	fmt.Sscanf(confStr, "%f", &conf)
	if conf <= 0 || conf > 1 {
		return 0.5 // Default
	}
	return conf
}

func extractSteps(block string) []string {
	// Find steps section
	startIdx := strings.Index(block, "steps:")
	if startIdx == -1 {
		return nil
	}

	// Find end of steps (next field or end)
	endMarkers := []string{"confidence:", "when_to_apply:", "description:", "---"}
	endIdx := len(block)
	for _, marker := range endMarkers {
		idx := strings.Index(block[startIdx:], marker)
		if idx != -1 && idx > 6 && startIdx+idx < endIdx {
			endIdx = startIdx + idx
		}
	}

	section := block[startIdx+6 : endIdx]
	lines := strings.Split(section, "\n")

	steps := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			step := strings.TrimPrefix(line, "- ")
			step = strings.Trim(step, "\"' ")
			if step != "" {
				steps = append(steps, step)
			}
		}
	}

	return steps
}
