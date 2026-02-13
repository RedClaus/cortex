package grammar

import (
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════════
// GENERATOR TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestGenerator_SimpleSchema(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		},
		"required": ["name", "age"]
	}`

	gen := NewGenerator()
	grammar, err := gen.Generate(schema)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Grammar should not be empty
	if grammar == "" {
		t.Error("expected non-empty grammar")
	}

	// Should contain root rule
	if !strings.Contains(grammar, "root ::=") {
		t.Error("grammar missing root rule")
	}

	// Should contain primitive rules
	if !strings.Contains(grammar, "string ::=") {
		t.Error("grammar missing string rule")
	}
	if !strings.Contains(grammar, "integer ::=") {
		t.Error("grammar missing integer rule")
	}

	// Validate the generated grammar
	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("generated grammar is invalid: %v", err)
	}
}

func TestGenerator_AllTypes(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"text": {"type": "string"},
			"count": {"type": "integer"},
			"price": {"type": "number"},
			"active": {"type": "boolean"}
		},
		"required": ["text", "count", "price", "active"]
	}`

	gen := NewGenerator()
	grammar, err := gen.Generate(schema)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Should contain all primitive types
	expectedRules := []string{"string", "integer", "number", "boolean"}
	for _, rule := range expectedRules {
		if !strings.Contains(grammar, rule+" ::=") {
			t.Errorf("grammar missing %s rule", rule)
		}
	}

	// Validate
	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("generated grammar is invalid: %v", err)
	}
}

func TestGenerator_EnumType(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"status": {
				"type": "string",
				"enum": ["active", "inactive", "pending"]
			}
		},
		"required": ["status"]
	}`

	gen := NewGenerator()
	grammar, err := gen.Generate(schema)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Should contain enum values
	if !strings.Contains(grammar, "active") {
		t.Error("grammar missing enum value 'active'")
	}
	if !strings.Contains(grammar, "inactive") {
		t.Error("grammar missing enum value 'inactive'")
	}
	if !strings.Contains(grammar, "pending") {
		t.Error("grammar missing enum value 'pending'")
	}

	// Should use alternation (|)
	if !strings.Contains(grammar, "|") {
		t.Error("grammar should use alternation for enum")
	}

	// Validate
	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("generated grammar is invalid: %v", err)
	}
}

func TestGenerator_ArrayType(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"tags": {
				"type": "array",
				"items": {"type": "string"}
			}
		},
		"required": ["tags"]
	}`

	gen := NewGenerator()
	grammar, err := gen.Generate(schema)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Should contain array rule
	if !strings.Contains(grammar, "array_") {
		t.Error("grammar missing array rule")
	}

	// Should contain array brackets
	if !strings.Contains(grammar, `"["`) || !strings.Contains(grammar, `"]"`) {
		t.Error("grammar missing array brackets")
	}

	// Validate
	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("generated grammar is invalid: %v", err)
	}
}

func TestGenerator_EmptyObject(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {}
	}`

	gen := NewGenerator()
	grammar, err := gen.Generate(schema)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Should generate empty object rule
	if !strings.Contains(grammar, `"{"`) || !strings.Contains(grammar, `"}"`) {
		t.Error("grammar should contain object braces")
	}

	// Validate
	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("generated grammar is invalid: %v", err)
	}
}

func TestGenerator_InvalidJSON(t *testing.T) {
	schema := `{invalid json}`

	gen := NewGenerator()
	_, err := gen.Generate(schema)
	if err == nil {
		t.Error("expected error for invalid JSON schema")
	}
}

func TestGenerator_NestedObject(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"user": {
				"type": "object",
				"properties": {
					"name": {"type": "string"}
				}
			}
		}
	}`

	gen := NewGenerator()
	_, err := gen.Generate(schema)

	// Should error because nested objects are not allowed
	if err == nil {
		t.Error("expected error for nested object (flat schema required)")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMMON GRAMMARS TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestJSONGrammar(t *testing.T) {
	grammar := JSONGrammar()

	// Should not be empty
	if grammar == "" {
		t.Error("JSONGrammar returned empty string")
	}

	// Should contain root rule
	if !strings.Contains(grammar, "root ::=") {
		t.Error("missing root rule")
	}

	// Should contain basic types
	expectedRules := []string{"value", "object", "array", "string", "number"}
	for _, rule := range expectedRules {
		if !strings.Contains(grammar, rule) {
			t.Errorf("missing rule: %s", rule)
		}
	}

	// Validate
	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("JSONGrammar is invalid: %v", err)
	}
}

func TestSimpleObjectGrammar(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
	}{
		{"no fields", []string{}},
		{"one field", []string{"name"}},
		{"multiple fields", []string{"name", "email", "age"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grammar := SimpleObjectGrammar(tt.fields...)

			// Should not be empty
			if grammar == "" {
				t.Error("SimpleObjectGrammar returned empty string")
			}

			// Should contain root rule
			if !strings.Contains(grammar, "root ::=") {
				t.Error("missing root rule")
			}

			// Should contain field names
			for _, field := range tt.fields {
				if !strings.Contains(grammar, field) {
					t.Errorf("missing field: %s", field)
				}
			}

			// Validate
			if err := ValidateGrammarString(grammar); err != nil {
				t.Errorf("SimpleObjectGrammar is invalid: %v", err)
			}
		})
	}
}

func TestBooleanGrammar(t *testing.T) {
	grammar := BooleanGrammar()

	// Should contain root rule
	if !strings.Contains(grammar, "root ::=") {
		t.Error("missing root rule")
	}

	// Should contain true and false
	if !strings.Contains(grammar, "true") || !strings.Contains(grammar, "false") {
		t.Error("grammar should contain true and false")
	}

	// Validate
	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("BooleanGrammar is invalid: %v", err)
	}
}

func TestYesNoGrammar(t *testing.T) {
	grammar := YesNoGrammar()

	// Should contain root rule
	if !strings.Contains(grammar, "root ::=") {
		t.Error("missing root rule")
	}

	// Should contain yes and no
	if !strings.Contains(grammar, "yes") || !strings.Contains(grammar, "no") {
		t.Error("grammar should contain yes and no")
	}

	// Validate
	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("YesNoGrammar is invalid: %v", err)
	}
}

func TestIntegerGrammar(t *testing.T) {
	grammar := IntegerGrammar()

	// Should contain root rule
	if !strings.Contains(grammar, "root ::=") {
		t.Error("missing root rule")
	}

	// Should contain digit pattern
	if !strings.Contains(grammar, "[0-9]") {
		t.Error("grammar should contain digit pattern")
	}

	// Validate
	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("IntegerGrammar is invalid: %v", err)
	}
}

func TestListGrammar(t *testing.T) {
	grammar := ListGrammar()

	// Should contain root rule
	if !strings.Contains(grammar, "root ::=") {
		t.Error("missing root rule")
	}

	// Should contain array brackets
	if !strings.Contains(grammar, `"["`) || !strings.Contains(grammar, `"]"`) {
		t.Error("grammar should contain array brackets")
	}

	// Should contain string rule
	if !strings.Contains(grammar, "string") {
		t.Error("grammar should contain string rule")
	}

	// Validate
	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("ListGrammar is invalid: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestEscapeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`hello`, `hello`},
		{`"quoted"`, `\"quoted\"`},
		{`back\slash`, `back\\slash`},
		{"tab\there", "tab\\there"},
		{"new\nline", "new\\nline"},
	}

	for _, tt := range tests {
		result := escapeString(tt.input)
		if result != tt.expected {
			t.Errorf("escapeString(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with_dash"},
		{"with.dot", "with_dot"},
		{"with spaces", "with_spaces"},
		{"MixedCase123", "MixedCase123"},
		{"special!@#$%", "special_____"},
	}

	for _, tt := range tests {
		result := sanitizeName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeName(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// INTEGRATION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestGeneratorIntegration_ComplexSchema(t *testing.T) {
	// This schema represents a realistic template variable schema
	schema := `{
		"type": "object",
		"properties": {
			"task_type": {
				"type": "string",
				"enum": ["bug_fix", "feature", "refactor"]
			},
			"description": {"type": "string"},
			"priority": {"type": "integer"},
			"tags": {
				"type": "array",
				"items": {"type": "string"}
			},
			"estimated_hours": {"type": "number"},
			"completed": {"type": "boolean"}
		},
		"required": ["task_type", "description", "priority"]
	}`

	gen := NewGenerator()
	grammar, err := gen.Generate(schema)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Validate the generated grammar
	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("generated grammar is invalid: %v", err)
	}

	// Parse and verify structure
	g, err := Parse(grammar)
	if err != nil {
		t.Fatalf("failed to parse generated grammar: %v", err)
	}

	// Should have a root rule
	if !g.HasRule("root") {
		t.Error("missing root rule")
	}

	// Should have basic primitive rules
	primitives := []string{"ws", "string", "integer", "number", "boolean"}
	for _, prim := range primitives {
		if !g.HasRule(prim) {
			t.Errorf("missing primitive rule: %s", prim)
		}
	}
}

func TestGeneratorIntegration_RealWorldExample(t *testing.T) {
	// Real-world example: Code review template variables
	schema := `{
		"type": "object",
		"properties": {
			"language": {"type": "string"},
			"file_path": {"type": "string"},
			"code_snippet": {"type": "string"},
			"line_numbers": {
				"type": "array",
				"items": {"type": "integer"}
			},
			"severity": {
				"type": "string",
				"enum": ["critical", "major", "minor", "info"]
			},
			"auto_fix": {"type": "boolean"}
		},
		"required": ["language", "code_snippet", "severity"]
	}`

	gen := NewGenerator()
	grammar, err := gen.Generate(schema)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// The grammar should be valid
	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("generated grammar is invalid: %v", err)
	}

	// The grammar should enforce the required structure
	// When used with Ollama, this will prevent the LLM from generating
	// invalid JSON at the token level
	if !strings.Contains(grammar, "language") {
		t.Error("grammar should reference language field")
	}
	if !strings.Contains(grammar, "severity") {
		t.Error("grammar should reference severity field")
	}
}
