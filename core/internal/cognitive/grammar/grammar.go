// Package grammar provides GBNF grammar types and validation.
package grammar

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// GRAMMAR TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Grammar represents a parsed GBNF grammar.
type Grammar struct {
	// Rules maps rule names to their definitions
	Rules map[string]*Rule

	// RootRule is the entry point rule (typically "root")
	RootRule string
}

// Rule represents a single GBNF production rule.
type Rule struct {
	Name       string
	Definition string
	Parsed     *ParsedRule // Parsed structure for validation
}

// ParsedRule represents the parsed structure of a rule definition.
type ParsedRule struct {
	Type        RuleType
	Alternatives []Alternative // For choice rules (|)
	Sequence    []Element     // For sequence rules
}

// RuleType categorizes the type of production rule.
type RuleType string

const (
	RuleChoice    RuleType = "choice"    // rule1 | rule2
	RuleSequence  RuleType = "sequence"  // rule1 rule2
	RuleTerminal  RuleType = "terminal"  // "literal"
	RuleReference RuleType = "reference" // other_rule
	RuleCharClass RuleType = "charclass" // [a-z]
	RuleRepeat    RuleType = "repeat"    // rule*
	RuleOptional  RuleType = "optional"  // rule?
	RuleGroup     RuleType = "group"     // (rule1 rule2)
)

// Alternative represents one option in a choice rule.
type Alternative struct {
	Elements []Element
}

// Element represents a single element in a rule (terminal, reference, etc).
type Element struct {
	Type      RuleType
	Value     string // Literal value, rule name, or character class
	Quantifier string // *, +, ?, or empty
	Nested    []Element // For groups
}

// ═══════════════════════════════════════════════════════════════════════════════
// GRAMMAR PARSING
// ═══════════════════════════════════════════════════════════════════════════════

// Parse parses a GBNF grammar string into a Grammar object.
func Parse(grammarText string) (*Grammar, error) {
	g := &Grammar{
		Rules:    make(map[string]*Rule),
		RootRule: "root",
	}

	// Split into lines
	lines := strings.Split(grammarText, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse rule: name ::= definition
		parts := strings.SplitN(line, "::=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid rule syntax: %s", line)
		}

		name := strings.TrimSpace(parts[0])
		definition := strings.TrimSpace(parts[1])

		if name == "" {
			return nil, fmt.Errorf("empty rule name in: %s", line)
		}

		g.Rules[name] = &Rule{
			Name:       name,
			Definition: definition,
		}
	}

	// Validate that root rule exists
	if _, ok := g.Rules[g.RootRule]; !ok {
		return nil, fmt.Errorf("missing root rule")
	}

	return g, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// GRAMMAR VALIDATION
// ═══════════════════════════════════════════════════════════════════════════════

// Validate performs comprehensive validation of a grammar.
func (g *Grammar) Validate() error {
	// Check that root rule exists
	if _, ok := g.Rules[g.RootRule]; !ok {
		return fmt.Errorf("missing root rule: %s", g.RootRule)
	}

	// Check for undefined rule references
	for name, rule := range g.Rules {
		refs := extractRuleReferences(rule.Definition)
		for _, ref := range refs {
			// Skip built-in patterns
			if isBuiltInPattern(ref) {
				continue
			}
			if _, ok := g.Rules[ref]; !ok {
				return fmt.Errorf("rule '%s' references undefined rule '%s'", name, ref)
			}
		}
	}

	// Check for cycles (indirect recursion is OK, but direct left recursion is not)
	if err := g.checkForDirectLeftRecursion(); err != nil {
		return err
	}

	return nil
}

// extractRuleReferences extracts all rule references from a definition.
func extractRuleReferences(definition string) []string {
	var refs []string
	seen := make(map[string]bool)

	// Remove all quoted content to avoid false positives
	// This handles both regular strings and escaped quotes
	cleaned := regexp.MustCompile(`"(?:[^"\\]|\\.)*"`).ReplaceAllString(definition, "")

	// Remove character classes to avoid false positives
	cleaned = regexp.MustCompile(`\[[^\]]*\]`).ReplaceAllString(cleaned, "")

	// Remove escape sequences
	cleaned = regexp.MustCompile(`\\[a-z]`).ReplaceAllString(cleaned, "")

	// Match identifiers (must be at least 2 characters to avoid single letter false positives)
	re := regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]+)\b`)
	matches := re.FindAllString(cleaned, -1)

	for _, match := range matches {
		if !seen[match] {
			refs = append(refs, match)
			seen[match] = true
		}
	}

	return refs
}

// isBuiltInPattern checks if a pattern is a built-in GBNF pattern.
func isBuiltInPattern(pattern string) bool {
	// Common built-in patterns (this is not exhaustive)
	builtIns := map[string]bool{
		"ws":      true,
		"string":  true,
		"number":  true,
		"integer": true,
		"boolean": true,
		"null":    true,
	}
	return builtIns[pattern]
}

// checkForDirectLeftRecursion detects direct left recursion (A ::= A ...).
func (g *Grammar) checkForDirectLeftRecursion() error {
	for name, rule := range g.Rules {
		// Simple check: does the definition start with the rule name?
		trimmed := strings.TrimSpace(rule.Definition)
		if strings.HasPrefix(trimmed, name+" ") || strings.HasPrefix(trimmed, name+"|") {
			return fmt.Errorf("direct left recursion detected in rule '%s'", name)
		}
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// GRAMMAR UTILITIES
// ═══════════════════════════════════════════════════════════════════════════════

// GetRule retrieves a rule by name.
func (g *Grammar) GetRule(name string) (*Rule, bool) {
	rule, ok := g.Rules[name]
	return rule, ok
}

// RuleCount returns the number of rules in the grammar.
func (g *Grammar) RuleCount() int {
	return len(g.Rules)
}

// HasRule checks if a rule exists.
func (g *Grammar) HasRule(name string) bool {
	_, ok := g.Rules[name]
	return ok
}

// String returns the grammar as a string (for debugging).
func (g *Grammar) String() string {
	var sb strings.Builder

	// Always put root rule first
	if root, ok := g.Rules[g.RootRule]; ok {
		sb.WriteString(fmt.Sprintf("%s ::= %s\n", root.Name, root.Definition))
	}

	// Add other rules in alphabetical order
	var names []string
	for name := range g.Rules {
		if name != g.RootRule {
			names = append(names, name)
		}
	}

	// Sort alphabetically
	sort.Strings(names)

	for _, name := range names {
		rule := g.Rules[name]
		sb.WriteString(fmt.Sprintf("%s ::= %s\n", rule.Name, rule.Definition))
	}

	return sb.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// VALIDATION HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// ValidateGrammarString is a convenience function to validate a GBNF grammar string.
func ValidateGrammarString(grammarText string) error {
	g, err := Parse(grammarText)
	if err != nil {
		return fmt.Errorf("parse grammar: %w", err)
	}

	if err := g.Validate(); err != nil {
		return fmt.Errorf("validate grammar: %w", err)
	}

	return nil
}

// IsValidGrammar checks if a grammar string is valid.
func IsValidGrammar(grammarText string) bool {
	return ValidateGrammarString(grammarText) == nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// GRAMMAR COMPOSITION
// ═══════════════════════════════════════════════════════════════════════════════

// Merge combines multiple grammars into one.
// Later grammars override rules with the same name.
func Merge(grammars ...*Grammar) (*Grammar, error) {
	if len(grammars) == 0 {
		return nil, fmt.Errorf("no grammars to merge")
	}

	merged := &Grammar{
		Rules:    make(map[string]*Rule),
		RootRule: grammars[0].RootRule,
	}

	for _, g := range grammars {
		for name, rule := range g.Rules {
			merged.Rules[name] = rule
		}
	}

	// Use the root rule from the last grammar
	if len(grammars) > 0 {
		merged.RootRule = grammars[len(grammars)-1].RootRule
	}

	return merged, nil
}

// AddRule adds a rule to the grammar.
func (g *Grammar) AddRule(name, definition string) {
	g.Rules[name] = &Rule{
		Name:       name,
		Definition: definition,
	}
}

// RemoveRule removes a rule from the grammar.
func (g *Grammar) RemoveRule(name string) {
	delete(g.Rules, name)
}
