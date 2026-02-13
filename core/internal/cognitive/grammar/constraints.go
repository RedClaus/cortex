// Package grammar provides token-level constraint validation.
package grammar

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// CONSTRAINT TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Constraint represents a validation constraint for LLM output.
type Constraint interface {
	// Validate checks if the output satisfies the constraint
	Validate(output string) error

	// Type returns the constraint type name
	Type() string
}

// ConstraintSet is a collection of constraints that must all be satisfied.
type ConstraintSet struct {
	Constraints []Constraint
}

// Validate checks all constraints in the set.
func (cs *ConstraintSet) Validate(output string) error {
	for i, constraint := range cs.Constraints {
		if err := constraint.Validate(output); err != nil {
			return fmt.Errorf("constraint %d (%s): %w", i, constraint.Type(), err)
		}
	}
	return nil
}

// Add adds a constraint to the set.
func (cs *ConstraintSet) Add(c Constraint) {
	cs.Constraints = append(cs.Constraints, c)
}

// ═══════════════════════════════════════════════════════════════════════════════
// JSON CONSTRAINTS
// ═══════════════════════════════════════════════════════════════════════════════

// JSONConstraint validates that output is valid JSON.
type JSONConstraint struct{}

func (c *JSONConstraint) Type() string { return "json" }

func (c *JSONConstraint) Validate(output string) error {
	var v interface{}
	if err := json.Unmarshal([]byte(output), &v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// JSONObjectConstraint validates that output is a JSON object.
type JSONObjectConstraint struct{}

func (c *JSONObjectConstraint) Type() string { return "json_object" }

func (c *JSONObjectConstraint) Validate(output string) error {
	var v map[string]interface{}
	if err := json.Unmarshal([]byte(output), &v); err != nil {
		return fmt.Errorf("not a valid JSON object: %w", err)
	}
	return nil
}

// JSONArrayConstraint validates that output is a JSON array.
type JSONArrayConstraint struct{}

func (c *JSONArrayConstraint) Type() string { return "json_array" }

func (c *JSONArrayConstraint) Validate(output string) error {
	var v []interface{}
	if err := json.Unmarshal([]byte(output), &v); err != nil {
		return fmt.Errorf("not a valid JSON array: %w", err)
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCHEMA CONSTRAINTS
// ═══════════════════════════════════════════════════════════════════════════════

// SchemaConstraint validates output against a JSON Schema.
type SchemaConstraint struct {
	Schema map[string]interface{}
}

func (c *SchemaConstraint) Type() string { return "schema" }

func (c *SchemaConstraint) Validate(output string) error {
	// Parse output as JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Get required fields from schema
	properties, _ := c.Schema["properties"].(map[string]interface{})
	required, _ := c.Schema["required"].([]interface{})

	// Check required fields
	for _, reqField := range required {
		fieldName, ok := reqField.(string)
		if !ok {
			continue
		}

		if _, exists := data[fieldName]; !exists {
			return fmt.Errorf("missing required field: %s", fieldName)
		}
	}

	// Validate field types
	for fieldName, fieldSchema := range properties {
		value, exists := data[fieldName]
		if !exists {
			continue
		}

		schema, ok := fieldSchema.(map[string]interface{})
		if !ok {
			continue
		}

		expectedType, _ := schema["type"].(string)
		if err := validateType(fieldName, value, expectedType); err != nil {
			return err
		}

		// Validate enum if present
		if enum, ok := schema["enum"].([]interface{}); ok {
			if err := validateEnum(fieldName, value, enum); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateType checks if a value matches the expected JSON Schema type.
func validateType(fieldName string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field '%s': expected string, got %T", fieldName, value)
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("field '%s': expected number, got %T", fieldName, value)
		}
	case "integer":
		num, ok := value.(float64)
		if !ok {
			return fmt.Errorf("field '%s': expected integer, got %T", fieldName, value)
		}
		if num != float64(int(num)) {
			return fmt.Errorf("field '%s': expected integer, got float", fieldName)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field '%s': expected boolean, got %T", fieldName, value)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("field '%s': expected array, got %T", fieldName, value)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("field '%s': expected object, got %T", fieldName, value)
		}
	case "null":
		if value != nil {
			return fmt.Errorf("field '%s': expected null, got %T", fieldName, value)
		}
	}
	return nil
}

// validateEnum checks if a value is in the allowed enum values.
func validateEnum(fieldName string, value interface{}, enum []interface{}) error {
	for _, allowed := range enum {
		if value == allowed {
			return nil
		}
	}
	return fmt.Errorf("field '%s': value %v not in allowed enum values", fieldName, value)
}

// ═══════════════════════════════════════════════════════════════════════════════
// GRAMMAR CONSTRAINTS
// ═══════════════════════════════════════════════════════════════════════════════

// GrammarConstraint validates output against a GBNF grammar.
// This is a basic check - full GBNF validation would require a parser.
type GrammarConstraint struct {
	Grammar string
}

func (c *GrammarConstraint) Type() string { return "grammar" }

func (c *GrammarConstraint) Validate(output string) error {
	// Basic validation: check that the grammar is valid
	if err := ValidateGrammarString(c.Grammar); err != nil {
		return fmt.Errorf("invalid grammar: %w", err)
	}

	// For now, we don't do full parsing of output against grammar
	// That would require implementing a GBNF parser
	// Instead, we just ensure the grammar is well-formed
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// PATTERN CONSTRAINTS
// ═══════════════════════════════════════════════════════════════════════════════

// RegexConstraint validates output against a regular expression.
type RegexConstraint struct {
	Pattern *regexp.Regexp
	Message string
}

func (c *RegexConstraint) Type() string { return "regex" }

func (c *RegexConstraint) Validate(output string) error {
	if !c.Pattern.MatchString(output) {
		if c.Message != "" {
			return fmt.Errorf("%s", c.Message)
		}
		return fmt.Errorf("output does not match pattern: %s", c.Pattern.String())
	}
	return nil
}

// NewRegexConstraint creates a regex constraint from a pattern string.
func NewRegexConstraint(pattern, message string) (*RegexConstraint, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}
	return &RegexConstraint{
		Pattern: re,
		Message: message,
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// LENGTH CONSTRAINTS
// ═══════════════════════════════════════════════════════════════════════════════

// LengthConstraint validates output length.
type LengthConstraint struct {
	MinLength int
	MaxLength int
}

func (c *LengthConstraint) Type() string { return "length" }

func (c *LengthConstraint) Validate(output string) error {
	length := len(output)
	if c.MinLength > 0 && length < c.MinLength {
		return fmt.Errorf("output too short: %d chars (min: %d)", length, c.MinLength)
	}
	if c.MaxLength > 0 && length > c.MaxLength {
		return fmt.Errorf("output too long: %d chars (max: %d)", length, c.MaxLength)
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// VALUE CONSTRAINTS
// ═══════════════════════════════════════════════════════════════════════════════

// EnumConstraint validates that output is one of the allowed values.
type EnumConstraint struct {
	AllowedValues []string
}

func (c *EnumConstraint) Type() string { return "enum" }

func (c *EnumConstraint) Validate(output string) error {
	trimmed := strings.TrimSpace(output)
	for _, allowed := range c.AllowedValues {
		if trimmed == allowed {
			return nil
		}
	}
	return fmt.Errorf("output '%s' not in allowed values: %v", trimmed, c.AllowedValues)
}

// BooleanConstraint validates that output is a boolean value.
type BooleanConstraint struct{}

func (c *BooleanConstraint) Type() string { return "boolean" }

func (c *BooleanConstraint) Validate(output string) error {
	trimmed := strings.TrimSpace(output)
	if trimmed != "true" && trimmed != "false" {
		return fmt.Errorf("not a valid boolean: %s", trimmed)
	}
	return nil
}

// IntegerConstraint validates that output is an integer.
type IntegerConstraint struct {
	Min *int
	Max *int
}

func (c *IntegerConstraint) Type() string { return "integer" }

func (c *IntegerConstraint) Validate(output string) error {
	trimmed := strings.TrimSpace(output)
	num, err := strconv.Atoi(trimmed)
	if err != nil {
		return fmt.Errorf("not a valid integer: %s", trimmed)
	}

	if c.Min != nil && num < *c.Min {
		return fmt.Errorf("integer %d below minimum: %d", num, *c.Min)
	}
	if c.Max != nil && num > *c.Max {
		return fmt.Errorf("integer %d above maximum: %d", num, *c.Max)
	}

	return nil
}

// NumberConstraint validates that output is a number.
type NumberConstraint struct {
	Min *float64
	Max *float64
}

func (c *NumberConstraint) Type() string { return "number" }

func (c *NumberConstraint) Validate(output string) error {
	trimmed := strings.TrimSpace(output)
	num, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return fmt.Errorf("not a valid number: %s", trimmed)
	}

	if c.Min != nil && num < *c.Min {
		return fmt.Errorf("number %f below minimum: %f", num, *c.Min)
	}
	if c.Max != nil && num > *c.Max {
		return fmt.Errorf("number %f above maximum: %f", num, *c.Max)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONSTRAINT BUILDERS
// ═══════════════════════════════════════════════════════════════════════════════

// NewSchemaConstraint creates a schema constraint from a JSON Schema string.
func NewSchemaConstraint(schemaJSON string) (*SchemaConstraint, error) {
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return nil, fmt.Errorf("invalid schema JSON: %w", err)
	}
	return &SchemaConstraint{Schema: schema}, nil
}

// NewGrammarConstraint creates a grammar constraint from a GBNF grammar string.
func NewGrammarConstraint(grammar string) *GrammarConstraint {
	return &GrammarConstraint{Grammar: grammar}
}

// NewEnumConstraint creates an enum constraint from allowed values.
func NewEnumConstraint(values ...string) *EnumConstraint {
	return &EnumConstraint{AllowedValues: values}
}

// NewLengthConstraint creates a length constraint with min/max bounds.
func NewLengthConstraint(min, max int) *LengthConstraint {
	return &LengthConstraint{
		MinLength: min,
		MaxLength: max,
	}
}

// NewIntegerConstraint creates an integer constraint with optional min/max.
func NewIntegerConstraint(min, max *int) *IntegerConstraint {
	return &IntegerConstraint{
		Min: min,
		Max: max,
	}
}

// NewNumberConstraint creates a number constraint with optional min/max.
func NewNumberConstraint(min, max *float64) *NumberConstraint {
	return &NumberConstraint{
		Min: min,
		Max: max,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// VALIDATION HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// ValidateAgainstGrammar validates output against a GBNF grammar string.
// This is the main function called by the template engine.
func ValidateAgainstGrammar(output string, grammarText string) error {
	// First, validate the grammar itself
	if err := ValidateGrammarString(grammarText); err != nil {
		return fmt.Errorf("invalid grammar: %w", err)
	}

	// For JSON-based grammars, also validate as JSON
	if strings.Contains(grammarText, "root ::=") {
		// Try to parse as JSON first
		constraint := &JSONConstraint{}
		if err := constraint.Validate(output); err != nil {
			// If it's meant to be JSON but isn't valid, that's an error
			if strings.Contains(strings.ToLower(grammarText), "object") ||
				strings.Contains(strings.ToLower(grammarText), "array") {
				return fmt.Errorf("output is not valid JSON: %w", err)
			}
		}
	}

	// Full GBNF parsing would go here
	// For now, we rely on Ollama to enforce the grammar during generation
	return nil
}

// ValidateAgainstSchema validates output against a JSON Schema.
func ValidateAgainstSchema(output string, schemaJSON string) error {
	constraint, err := NewSchemaConstraint(schemaJSON)
	if err != nil {
		return err
	}
	return constraint.Validate(output)
}

// ValidateJSON validates that output is valid JSON.
func ValidateJSON(output string) error {
	return (&JSONConstraint{}).Validate(output)
}

// ValidateJSONObject validates that output is a valid JSON object.
func ValidateJSONObject(output string) error {
	return (&JSONObjectConstraint{}).Validate(output)
}

// ValidateJSONArray validates that output is a valid JSON array.
func ValidateJSONArray(output string) error {
	return (&JSONArrayConstraint{}).Validate(output)
}
