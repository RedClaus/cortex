package grammar

import (
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════════
// JSON CONSTRAINT TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestJSONConstraint_Valid(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{"object", `{"key": "value"}`},
		{"array", `[1, 2, 3]`},
		{"string", `"hello"`},
		{"number", `42`},
		{"boolean", `true`},
		{"null", `null`},
	}

	c := &JSONConstraint{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := c.Validate(tt.output); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestJSONConstraint_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{"missing quote", `{"key": value}`},
		{"trailing comma", `{"key": "value",}`},
		{"not JSON", `hello world`},
	}

	c := &JSONConstraint{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := c.Validate(tt.output); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestJSONObjectConstraint(t *testing.T) {
	c := &JSONObjectConstraint{}

	// Valid object
	if err := c.Validate(`{"key": "value"}`); err != nil {
		t.Errorf("unexpected error for valid object: %v", err)
	}

	// Invalid: array
	if err := c.Validate(`[1, 2, 3]`); err == nil {
		t.Error("expected error for array")
	}

	// Invalid: string
	if err := c.Validate(`"hello"`); err == nil {
		t.Error("expected error for string")
	}
}

func TestJSONArrayConstraint(t *testing.T) {
	c := &JSONArrayConstraint{}

	// Valid array
	if err := c.Validate(`[1, 2, 3]`); err != nil {
		t.Errorf("unexpected error for valid array: %v", err)
	}

	// Invalid: object
	if err := c.Validate(`{"key": "value"}`); err == nil {
		t.Error("expected error for object")
	}

	// Invalid: string
	if err := c.Validate(`"hello"`); err == nil {
		t.Error("expected error for string")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCHEMA CONSTRAINT TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestSchemaConstraint_RequiredFields(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "number"}
		},
		"required": ["name", "age"]
	}`

	c, err := NewSchemaConstraint(schemaJSON)
	if err != nil {
		t.Fatalf("failed to create constraint: %v", err)
	}

	// Valid: all required fields present
	if err := c.Validate(`{"name": "John", "age": 30}`); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid: missing required field
	if err := c.Validate(`{"name": "John"}`); err == nil {
		t.Error("expected error for missing required field")
	}
}

func TestSchemaConstraint_TypeValidation(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"},
			"active": {"type": "boolean"}
		},
		"required": ["name"]
	}`

	c, err := NewSchemaConstraint(schemaJSON)
	if err != nil {
		t.Fatalf("failed to create constraint: %v", err)
	}

	// Valid types
	if err := c.Validate(`{"name": "John", "age": 30, "active": true}`); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid: wrong type for age (string instead of integer)
	if err := c.Validate(`{"name": "John", "age": "thirty"}`); err == nil {
		t.Error("expected error for wrong type")
	}

	// Invalid: float for integer field
	if err := c.Validate(`{"name": "John", "age": 30.5}`); err == nil {
		t.Error("expected error for float in integer field")
	}
}

func TestSchemaConstraint_EnumValidation(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"properties": {
			"status": {
				"type": "string",
				"enum": ["active", "inactive", "pending"]
			}
		},
		"required": ["status"]
	}`

	c, err := NewSchemaConstraint(schemaJSON)
	if err != nil {
		t.Fatalf("failed to create constraint: %v", err)
	}

	// Valid: allowed enum value
	if err := c.Validate(`{"status": "active"}`); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid: not in enum
	if err := c.Validate(`{"status": "deleted"}`); err == nil {
		t.Error("expected error for value not in enum")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// PATTERN CONSTRAINT TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestRegexConstraint(t *testing.T) {
	c, err := NewRegexConstraint(`^\d{3}-\d{3}-\d{4}$`, "must be phone number format")
	if err != nil {
		t.Fatalf("failed to create constraint: %v", err)
	}

	// Valid
	if err := c.Validate("555-123-4567"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid
	if err := c.Validate("not-a-phone"); err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestRegexConstraint_InvalidPattern(t *testing.T) {
	_, err := NewRegexConstraint(`[invalid(`, "")
	if err == nil {
		t.Error("expected error for invalid regex pattern")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// LENGTH CONSTRAINT TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestLengthConstraint(t *testing.T) {
	c := NewLengthConstraint(5, 10)

	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{"valid length", "hello!", false},
		{"too short", "hi", true},
		{"too long", "this is way too long", true},
		{"min boundary", "12345", false},
		{"max boundary", "1234567890", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Validate(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

func TestLengthConstraint_NoMin(t *testing.T) {
	c := NewLengthConstraint(0, 10)

	// Empty string should be valid (no minimum)
	if err := c.Validate(""); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLengthConstraint_NoMax(t *testing.T) {
	c := NewLengthConstraint(5, 0)

	// Long string should be valid (no maximum)
	if err := c.Validate("this is a very long string that goes on and on"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// VALUE CONSTRAINT TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnumConstraint(t *testing.T) {
	c := NewEnumConstraint("red", "green", "blue")

	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{"valid: red", "red", false},
		{"valid: green", "green", false},
		{"valid: blue", "blue", false},
		{"invalid: yellow", "yellow", true},
		{"valid with whitespace", "  red  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Validate(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

func TestBooleanConstraint(t *testing.T) {
	c := &BooleanConstraint{}

	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{"true", "true", false},
		{"false", "false", false},
		{"invalid: yes", "yes", true},
		{"invalid: 1", "1", true},
		{"valid with whitespace", "  true  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Validate(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

func TestIntegerConstraint(t *testing.T) {
	min := 0
	max := 100
	c := NewIntegerConstraint(&min, &max)

	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{"valid: 50", "50", false},
		{"valid: 0", "0", false},
		{"valid: 100", "100", false},
		{"invalid: -1", "-1", true},
		{"invalid: 101", "101", true},
		{"invalid: float", "50.5", true},
		{"invalid: text", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Validate(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

func TestIntegerConstraint_NoBounds(t *testing.T) {
	c := NewIntegerConstraint(nil, nil)

	// Any integer should be valid
	tests := []string{"-1000", "0", "1000", "999999"}
	for _, output := range tests {
		if err := c.Validate(output); err != nil {
			t.Errorf("unexpected error for %s: %v", output, err)
		}
	}
}

func TestNumberConstraint(t *testing.T) {
	min := 0.0
	max := 100.0
	c := NewNumberConstraint(&min, &max)

	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{"valid: 50.5", "50.5", false},
		{"valid: 0.0", "0.0", false},
		{"valid: 100.0", "100.0", false},
		{"invalid: -0.1", "-0.1", true},
		{"invalid: 100.1", "100.1", true},
		{"invalid: text", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Validate(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONSTRAINT SET TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestConstraintSet(t *testing.T) {
	cs := &ConstraintSet{}
	cs.Add(&JSONObjectConstraint{})
	cs.Add(NewLengthConstraint(10, 100))

	// Valid: JSON object with appropriate length
	if err := cs.Validate(`{"key": "value"}`); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid: too short
	if err := cs.Validate(`{}`); err == nil {
		t.Error("expected error for too short")
	}

	// Invalid: not an object
	if err := cs.Validate(`["array", "of", "values"]`); err == nil {
		t.Error("expected error for non-object")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// VALIDATION HELPER TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestValidateAgainstGrammar(t *testing.T) {
	grammar := `root ::= "hello"
ws ::= [ \t]*`

	// Valid grammar should not error
	if err := ValidateAgainstGrammar("hello", grammar); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid grammar should error
	invalidGrammar := "root = hello"
	if err := ValidateAgainstGrammar("hello", invalidGrammar); err == nil {
		t.Error("expected error for invalid grammar")
	}
}

func TestValidateAgainstGrammar_JSONGrammar(t *testing.T) {
	grammar := `root ::= object
object ::= "{" ws pair ws "}"
pair ::= string ws ":" ws string
string ::= "\"" [^"]* "\""
ws ::= [ \t\n\r]*`

	// Valid JSON object
	validOutput := `{"key": "value"}`
	if err := ValidateAgainstGrammar(validOutput, grammar); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid JSON should fail
	invalidOutput := `{key: value}`
	if err := ValidateAgainstGrammar(invalidOutput, grammar); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidateAgainstSchema(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		},
		"required": ["name"]
	}`

	// Valid output
	if err := ValidateAgainstSchema(`{"name": "John"}`, schema); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid: missing required field
	if err := ValidateAgainstSchema(`{}`, schema); err == nil {
		t.Error("expected error for missing required field")
	}

	// Invalid schema
	if err := ValidateAgainstSchema(`{}`, `invalid json`); err == nil {
		t.Error("expected error for invalid schema")
	}
}

func TestValidateJSON(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{"valid object", `{"key": "value"}`, false},
		{"valid array", `[1, 2, 3]`, false},
		{"valid string", `"hello"`, false},
		{"valid number", `42`, false},
		{"invalid", `{invalid}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSON(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateJSONObject(t *testing.T) {
	// Valid
	if err := ValidateJSONObject(`{"key": "value"}`); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid: array
	if err := ValidateJSONObject(`[1, 2, 3]`); err == nil {
		t.Error("expected error for array")
	}
}

func TestValidateJSONArray(t *testing.T) {
	// Valid
	if err := ValidateJSONArray(`[1, 2, 3]`); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid: object
	if err := ValidateJSONArray(`{"key": "value"}`); err == nil {
		t.Error("expected error for object")
	}
}
