// Package grammar provides GBNF grammar generation from JSON Schema.
// GBNF (GGML BNF) is used by llama.cpp and Ollama for constrained generation.
package grammar

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// GBNF GRAMMAR GENERATOR
// ═══════════════════════════════════════════════════════════════════════════════

// Generator converts JSON Schema to GBNF grammar.
type Generator struct {
	// Rules accumulates grammar rules during generation
	rules []string
	// Used tracks which rules have been defined
	defined map[string]bool
}

// NewGenerator creates a new GBNF generator.
func NewGenerator() *Generator {
	return &Generator{
		rules:   make([]string, 0),
		defined: make(map[string]bool),
	}
}

// Generate converts a JSON Schema to GBNF grammar.
// The schema must be flat (no nested objects) for reliable generation.
func (g *Generator) Generate(schemaJSON string) (string, error) {
	var schema Schema
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return "", fmt.Errorf("parse schema: %w", err)
	}

	// Reset state
	g.rules = make([]string, 0)
	g.defined = make(map[string]bool)

	// Add basic primitives
	g.addPrimitiveRules()

	// Generate root object rule
	if err := g.generateObject(&schema); err != nil {
		return "", err
	}

	// Build final grammar
	return strings.Join(g.rules, "\n"), nil
}

// Schema represents a JSON Schema for parsing.
type Schema struct {
	Type       string             `json:"type"`
	Properties map[string]*Schema `json:"properties,omitempty"`
	Required   []string           `json:"required,omitempty"`
	Items      *Schema            `json:"items,omitempty"`
	Enum       []interface{}      `json:"enum,omitempty"`
	Default    interface{}        `json:"default,omitempty"`
	MinLength  *int               `json:"minLength,omitempty"`
	MaxLength  *int               `json:"maxLength,omitempty"`
	Minimum    *float64           `json:"minimum,omitempty"`
	Maximum    *float64           `json:"maximum,omitempty"`
}

// addPrimitiveRules adds the basic GBNF rules for primitives.
func (g *Generator) addPrimitiveRules() {
	// Whitespace
	g.addRule("ws", `[ \t\n\r]*`)

	// String (JSON-escaped)
	g.addRule("string", `"\"" ([^"\\] | "\\" (["\\/bfnrt] | "u" [0-9a-fA-F]{4}))* "\""`)

	// Number
	g.addRule("number", `"-"? ([0-9] | [1-9] [0-9]*) ("." [0-9]+)? ([eE] [+-]? [0-9]+)?`)

	// Integer
	g.addRule("integer", `"-"? ([0-9] | [1-9] [0-9]*)`)

	// Boolean
	g.addRule("boolean", `"true" | "false"`)

	// Null
	g.addRule("null", `"null"`)
}

// addRule adds a rule if not already defined.
func (g *Generator) addRule(name, definition string) {
	if g.defined[name] {
		return
	}
	g.rules = append(g.rules, fmt.Sprintf("%s ::= %s", name, definition))
	g.defined[name] = true
}

// generateObject generates GBNF rules for an object schema.
func (g *Generator) generateObject(schema *Schema) error {
	if len(schema.Properties) == 0 {
		// Empty object
		g.addRule("root", `"{" ws "}"`)
		return nil
	}

	// Build required set
	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	// Generate property rules
	var propRules []string
	propNames := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		propNames = append(propNames, name)
	}

	// Sort for deterministic output (important for testing)
	sort.Strings(propNames)

	for _, name := range propNames {
		propSchema := schema.Properties[name]
		propRule, err := g.generateProperty(name, propSchema, requiredSet[name])
		if err != nil {
			return fmt.Errorf("property '%s': %w", name, err)
		}
		propRules = append(propRules, propRule)
	}

	// Build root rule
	// For simplicity, we require all properties in fixed order
	// This is more reliable than trying to handle optional/reorderable properties
	rootDef := `"{" ws ` + strings.Join(propRules, ` "," ws `) + ` ws "}"`
	g.addRule("root", rootDef)

	return nil
}

// generateProperty generates GBNF for a single property.
func (g *Generator) generateProperty(name string, schema *Schema, required bool) (string, error) {
	// Generate value rule based on type
	valueRule, err := g.generateValue(name, schema)
	if err != nil {
		return "", err
	}

	// Property is: "name" : value
	return fmt.Sprintf(`"\"" "%s" "\"" ws ":" ws %s`, name, valueRule), nil
}

// generateValue generates GBNF for a value of a given type.
func (g *Generator) generateValue(name string, schema *Schema) (string, error) {
	// Handle enum first
	if len(schema.Enum) > 0 {
		return g.generateEnum(name, schema.Enum)
	}

	switch schema.Type {
	case "string":
		return "string", nil

	case "number":
		return "number", nil

	case "integer":
		return "integer", nil

	case "boolean":
		return "boolean", nil

	case "null":
		return "null", nil

	case "array":
		return g.generateArray(name, schema)

	case "object":
		// Nested objects not allowed in flat schema
		return "", fmt.Errorf("nested objects not allowed (schema must be flat)")

	default:
		// Default to string
		return "string", nil
	}
}

// generateEnum generates GBNF for an enum type.
func (g *Generator) generateEnum(name string, values []interface{}) (string, error) {
	if len(values) == 0 {
		return "", fmt.Errorf("enum must have at least one value")
	}

	var alternatives []string
	for _, v := range values {
		switch val := v.(type) {
		case string:
			// Escape and quote
			escaped := escapeString(val)
			alternatives = append(alternatives, fmt.Sprintf(`"\"" "%s" "\""`, escaped))
		case float64:
			alternatives = append(alternatives, fmt.Sprintf(`"%v"`, val))
		case bool:
			if val {
				alternatives = append(alternatives, `"true"`)
			} else {
				alternatives = append(alternatives, `"false"`)
			}
		case nil:
			alternatives = append(alternatives, `"null"`)
		default:
			return "", fmt.Errorf("unsupported enum value type: %T", v)
		}
	}

	// Create enum rule
	ruleName := "enum_" + sanitizeName(name)
	g.addRule(ruleName, strings.Join(alternatives, " | "))

	return ruleName, nil
}

// generateArray generates GBNF for an array type.
func (g *Generator) generateArray(name string, schema *Schema) (string, error) {
	if schema.Items == nil {
		// Array of any - use string as default
		return g.generateGenericArray("string"), nil
	}

	// Get item type
	itemRule, err := g.generateValue(name+"_item", schema.Items)
	if err != nil {
		return "", fmt.Errorf("array items: %w", err)
	}

	return g.generateGenericArray(itemRule), nil
}

// generateGenericArray generates GBNF for an array of a specific item rule.
func (g *Generator) generateGenericArray(itemRule string) string {
	// Array: [] or [item, item, ...]
	ruleName := "array_" + itemRule
	if g.defined[ruleName] {
		return ruleName
	}

	// Build array rule
	arrayDef := fmt.Sprintf(`"[" ws (%s (ws "," ws %s)*)? ws "]"`, itemRule, itemRule)
	g.addRule(ruleName, arrayDef)

	return ruleName
}

// escapeString escapes a string for GBNF.
func escapeString(s string) string {
	// Escape special characters
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// sanitizeName converts a property name to a valid GBNF rule name.
func sanitizeName(name string) string {
	// Replace non-alphanumeric with underscore
	var result strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result.WriteRune(c)
		} else {
			result.WriteRune('_')
		}
	}
	return result.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMMON GRAMMARS
// ═══════════════════════════════════════════════════════════════════════════════

// JSONGrammar returns a grammar that accepts any valid JSON.
func JSONGrammar() string {
	return `root ::= value
ws ::= [ \t\n\r]*
value ::= object | array | string | number | "true" | "false" | "null"
object ::= "{" ws (pair (ws "," ws pair)*)? ws "}"
pair ::= string ws ":" ws value
array ::= "[" ws (value (ws "," ws value)*)? ws "]"
string ::= "\"" ([^"\\] | "\\" (["\\/bfnrt] | "u" [0-9a-fA-F]{4}))* "\""
number ::= "-"? ([0-9] | [1-9] [0-9]*) ("." [0-9]+)? ([eE] [+-]? [0-9]+)?`
}

// SimpleObjectGrammar returns a grammar for a simple flat JSON object.
func SimpleObjectGrammar(fields ...string) string {
	if len(fields) == 0 {
		return `root ::= "{" ws "}"`
	}

	var fieldRules []string
	for _, f := range fields {
		fieldRules = append(fieldRules, fmt.Sprintf(`"\"" "%s" "\"" ws ":" ws string`, f))
	}

	return fmt.Sprintf(`root ::= "{" ws %s ws "}"
ws ::= [ \t\n\r]*
string ::= "\"" ([^"\\] | "\\" (["\\/bfnrt] | "u" [0-9a-fA-F]{4}))* "\""`,
		strings.Join(fieldRules, ` "," ws `))
}

// BooleanGrammar returns a grammar that only accepts true or false.
func BooleanGrammar() string {
	return `root ::= "true" | "false"`
}

// YesNoGrammar returns a grammar that only accepts "yes" or "no".
func YesNoGrammar() string {
	return `root ::= "\"yes\"" | "\"no\""`
}

// IntegerGrammar returns a grammar that only accepts integers.
func IntegerGrammar() string {
	return `root ::= "-"? ([0-9] | [1-9] [0-9]*)`
}

// ListGrammar returns a grammar for a list of strings.
func ListGrammar() string {
	return `root ::= "[" ws (string (ws "," ws string)*)? ws "]"
ws ::= [ \t\n\r]*
string ::= "\"" ([^"\\] | "\\" (["\\/bfnrt] | "u" [0-9a-fA-F]{4}))* "\""`
}
