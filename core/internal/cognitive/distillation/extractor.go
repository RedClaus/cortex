package distillation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// XML SECTION EXTRACTOR
// ═══════════════════════════════════════════════════════════════════════════════

// ExtractedSections contains the parsed sections from a frontier model response.
type ExtractedSections struct {
	Solution string `json:"solution"`
	Template string `json:"template"`
	Schema   string `json:"schema"`
	Intent   string `json:"intent"`
}

// ExtractSections parses XML-tagged sections from a frontier model response.
func ExtractSections(response string) (*ExtractedSections, error) {
	sections := &ExtractedSections{}
	var errs []string

	// Extract solution
	solution, err := extractSection(response, "solution")
	if err != nil {
		errs = append(errs, "solution: "+err.Error())
	} else {
		sections.Solution = solution
	}

	// Extract template
	template, err := extractSection(response, "template")
	if err != nil {
		errs = append(errs, "template: "+err.Error())
	} else {
		sections.Template = template
	}

	// Extract schema
	schema, err := extractSection(response, "schema")
	if err != nil {
		errs = append(errs, "schema: "+err.Error())
	} else {
		sections.Schema = schema
	}

	// Extract intent
	intent, err := extractSection(response, "intent")
	if err != nil {
		errs = append(errs, "intent: "+err.Error())
	} else {
		sections.Intent = intent
	}

	// Require at least solution and template
	if sections.Solution == "" && sections.Template == "" {
		return nil, fmt.Errorf("missing required sections: %s", strings.Join(errs, "; "))
	}

	return sections, nil
}

// extractSection extracts content between XML tags.
func extractSection(text, tagName string) (string, error) {
	// Build regex pattern for the tag
	pattern := fmt.Sprintf(`(?s)<%s>\s*(.*?)\s*</%s>`, tagName, tagName)
	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return "", fmt.Errorf("section not found")
	}

	return strings.TrimSpace(matches[1]), nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// FALLBACK EXTRACTION
// ═══════════════════════════════════════════════════════════════════════════════

// ExtractSolutionOnly extracts just the solution when template extraction fails.
// This is used as a fallback to still provide a useful response to the user.
func ExtractSolutionOnly(response string) string {
	// Try to extract from XML tags first
	solution, err := extractSection(response, "solution")
	if err == nil && solution != "" {
		return solution
	}

	// If no tags, assume the entire response is the solution
	// Strip any partial XML that might be present
	cleaned := response

	// Remove any incomplete XML tags
	patterns := []string{
		`<solution>.*`,
		`<template>.*`,
		`<schema>.*`,
		`<intent>.*`,
	}

	for _, p := range patterns {
		re := regexp.MustCompile(`(?s)` + p)
		cleaned = re.ReplaceAllString(cleaned, "")
	}

	return strings.TrimSpace(cleaned)
}

// ═══════════════════════════════════════════════════════════════════════════════
// VALIDATION
// ═══════════════════════════════════════════════════════════════════════════════

// ValidateSections checks if extracted sections are valid.
func ValidateSections(sections *ExtractedSections) []string {
	var issues []string

	if sections.Solution == "" {
		issues = append(issues, "missing solution")
	}

	if sections.Template == "" {
		issues = append(issues, "missing template")
	}

	if sections.Schema == "" {
		issues = append(issues, "missing schema")
	} else {
		// Validate JSON using proper parser
		if !json.Valid([]byte(sections.Schema)) {
			issues = append(issues, "schema is not valid JSON")
		}
	}

	if sections.Intent == "" {
		issues = append(issues, "missing intent")
	}

	return issues
}

// ═══════════════════════════════════════════════════════════════════════════════
// GRADING RESPONSE PARSING
// ═══════════════════════════════════════════════════════════════════════════════

// GradeResponse represents a parsed grading response.
type GradeResponse struct {
	Grade            string  `json:"grade"`
	CorrectnessScore float64 `json:"correctness_score"`
	CompletenessScore float64 `json:"completeness_score"`
	Reason           string  `json:"reason"`
}

// ParseGradeResponse parses a JSON grading response.
func ParseGradeResponse(response string) (*GradeResponse, error) {
	// Clean up the response
	cleaned := strings.TrimSpace(response)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	// Find JSON object
	startIdx := strings.Index(cleaned, "{")
	endIdx := strings.LastIndex(cleaned, "}")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	jsonStr := cleaned[startIdx : endIdx+1]

	// Parse JSON using standard library
	var grade GradeResponse
	if err := json.Unmarshal([]byte(jsonStr), &grade); err != nil {
		return nil, fmt.Errorf("failed to parse grade response: %w", err)
	}

	// Validate grade value
	if grade.Grade != "pass" && grade.Grade != "fail" && grade.Grade != "partial" {
		return nil, fmt.Errorf("invalid grade value: %s", grade.Grade)
	}

	return &grade, nil
}
