package identity

import (
	"strings"
	"time"
)

// Guardian validates responses against the creed.
// All operations are deterministic and complete in <10ms.
type Guardian struct {
	creed    *CreedManager
	embedder Embedder
	config   *Config
}

// NewGuardian creates a new identity guardian.
func NewGuardian(creed *CreedManager, embedder Embedder, cfg *Config) *Guardian {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return &Guardian{
		creed:    creed,
		embedder: embedder,
		config:   cfg,
	}
}

// ValidateResponse checks if a response is consistent with the creed.
// This is a fast operation (<10ms) using embedding similarity.
func (g *Guardian) ValidateResponse(response string, responseEmbedding []float32) *ValidationResult {
	startTime := time.Now()

	result := &ValidationResult{
		Valid:              true,
		Similarity:         1.0,
		ViolatedStatements: []string{},
		Suggestions:        []string{},
	}

	// Get creed embeddings
	creedEmbeddings := g.creed.GetEmbeddings()
	creedStatements := g.creed.GetStatements()
	combinedCreed := g.creed.GetCombinedEmbedding()

	if len(creedEmbeddings) == 0 {
		result.Duration = time.Since(startTime)
		return result
	}

	// Compute similarity to combined creed
	if len(responseEmbedding) > 0 && len(combinedCreed) > 0 {
		result.Similarity = cosineSimilarity(responseEmbedding, combinedCreed)
	}

	// Check for explicit violations using pattern matching
	violations := g.checkPatternViolations(response, creedStatements)
	if len(violations) > 0 {
		result.ViolatedStatements = violations
		result.Valid = false
		result.Suggestions = g.generateSuggestions(violations)
	}

	// Check embedding-based violations
	if len(responseEmbedding) > 0 {
		for i, creedEmb := range creedEmbeddings {
			similarity := cosineSimilarity(responseEmbedding, creedEmb)
			// Low similarity to a specific creed statement is concerning
			if similarity < 0.3 {
				statement := creedStatements[i]
				if !containsString(result.ViolatedStatements, statement) {
					result.ViolatedStatements = append(result.ViolatedStatements, statement)
				}
			}
		}
	}

	// Mark as invalid if too many violations
	if len(result.ViolatedStatements) > 2 {
		result.Valid = false
	}

	// Overall similarity threshold
	if result.Similarity < 0.4 {
		result.Valid = false
	}

	result.Duration = time.Since(startTime)
	return result
}

// checkPatternViolations checks for explicit violations using pattern matching.
func (g *Guardian) checkPatternViolations(response string, creedStatements []string) []string {
	var violations []string
	lowerResponse := strings.ToLower(response)

	// Check for violations of each creed principle
	for _, statement := range creedStatements {
		if g.violatesStatement(lowerResponse, statement) {
			violations = append(violations, statement)
		}
	}

	return violations
}

// violatesStatement checks if a response violates a specific creed statement.
func (g *Guardian) violatesStatement(lowerResponse, statement string) bool {
	lowerStatement := strings.ToLower(statement)

	// Check for explicit contradictions based on statement content
	switch {
	case strings.Contains(lowerStatement, "privacy") || strings.Contains(lowerStatement, "locally"):
		// Check for privacy violations
		privacyViolations := []string{
			"send your data", "collect information", "share with third",
			"transmit to server", "upload your", "store in cloud",
		}
		for _, v := range privacyViolations {
			if strings.Contains(lowerResponse, v) {
				return true
			}
		}

	case strings.Contains(lowerStatement, "uncertainty") || strings.Contains(lowerStatement, "fabricate"):
		// Check for fabrication indicators
		fabricationIndicators := []string{
			"i'm absolutely certain", "i guarantee", "this is definitely true",
			"there's no doubt", "100% accurate",
		}
		for _, v := range fabricationIndicators {
			if strings.Contains(lowerResponse, v) {
				return true
			}
		}

	case strings.Contains(lowerStatement, "autonomy") || strings.Contains(lowerStatement, "manipulation"):
		// Check for manipulation attempts
		manipulationIndicators := []string{
			"you must", "you have to", "you need to believe",
			"don't think about", "just trust me",
		}
		for _, v := range manipulationIndicators {
			if strings.Contains(lowerResponse, v) {
				return true
			}
		}

	case strings.Contains(lowerStatement, "reflection") && strings.Contains(lowerStatement, "modification"):
		// Check for claims of external modification
		modificationIndicators := []string{
			"i've been reprogrammed", "my instructions changed",
			"i'm now different", "i've been updated to",
		}
		for _, v := range modificationIndicators {
			if strings.Contains(lowerResponse, v) {
				return true
			}
		}
	}

	return false
}

// generateSuggestions creates suggestions for addressing violations.
func (g *Guardian) generateSuggestions(violations []string) []string {
	var suggestions []string

	for _, v := range violations {
		lowerV := strings.ToLower(v)
		switch {
		case strings.Contains(lowerV, "privacy"):
			suggestions = append(suggestions, "Emphasize local-first operation and user data control")
		case strings.Contains(lowerV, "uncertainty"):
			suggestions = append(suggestions, "Acknowledge limitations and express appropriate uncertainty")
		case strings.Contains(lowerV, "autonomy"):
			suggestions = append(suggestions, "Support user decision-making rather than dictating actions")
		case strings.Contains(lowerV, "reflection"):
			suggestions = append(suggestions, "Frame improvements as self-reflection rather than external changes")
		}
	}

	return suggestions
}

// QuickValidate performs a fast validation without embedding computation.
// Use this when you only need pattern-based checking.
func (g *Guardian) QuickValidate(response string) *ValidationResult {
	startTime := time.Now()

	result := &ValidationResult{
		Valid:              true,
		Similarity:         1.0,
		ViolatedStatements: []string{},
		Suggestions:        []string{},
	}

	creedStatements := g.creed.GetStatements()
	if len(creedStatements) == 0 {
		result.Duration = time.Since(startTime)
		return result
	}

	violations := g.checkPatternViolations(response, creedStatements)
	if len(violations) > 0 {
		result.ViolatedStatements = violations
		result.Valid = len(violations) <= 1 // Allow one minor violation
		result.Suggestions = g.generateSuggestions(violations)
	}

	result.Duration = time.Since(startTime)
	return result
}

// containsString checks if a slice contains a string.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
