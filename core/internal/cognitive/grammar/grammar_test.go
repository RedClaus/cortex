package grammar

import (
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════════
// GRAMMAR PARSING TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestParse_ValidGrammar(t *testing.T) {
	grammarText := `root ::= "hello"
ws ::= [ \t\n\r]*`

	g, err := Parse(grammarText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if g.RuleCount() != 2 {
		t.Errorf("expected 2 rules, got %d", g.RuleCount())
	}

	if !g.HasRule("root") {
		t.Error("expected root rule")
	}

	if !g.HasRule("ws") {
		t.Error("expected ws rule")
	}
}

func TestParse_InvalidSyntax(t *testing.T) {
	tests := []struct {
		name    string
		grammar string
	}{
		{"missing ::=", "root = hello"},
		{"empty rule name", "::= hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.grammar)
			if err == nil {
				t.Error("expected error for invalid grammar")
			}
		})
	}
}

func TestParse_MissingRootRule(t *testing.T) {
	grammarText := `other ::= "hello"`

	_, err := Parse(grammarText)
	if err == nil {
		t.Error("expected error for missing root rule")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// GRAMMAR VALIDATION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestValidate_ValidGrammar(t *testing.T) {
	grammarText := `root ::= value
value ::= string | number
string ::= "\"" [^"]* "\""
number ::= [0-9]+`

	g, err := Parse(grammarText)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if err := g.Validate(); err != nil {
		t.Errorf("validation failed: %v", err)
	}
}

func TestValidate_UndefinedReference(t *testing.T) {
	grammarText := `root ::= undefined_rule`

	g, err := Parse(grammarText)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if err := g.Validate(); err == nil {
		t.Error("expected validation error for undefined reference")
	}
}

func TestValidate_DirectLeftRecursion(t *testing.T) {
	grammarText := `root ::= root "a"`

	g, err := Parse(grammarText)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if err := g.Validate(); err == nil {
		t.Error("expected validation error for direct left recursion")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// GRAMMAR UTILITIES TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestGrammar_String(t *testing.T) {
	grammarText := `root ::= "hello"
ws ::= [ \t\n\r]*`

	g, err := Parse(grammarText)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	output := g.String()
	if output == "" {
		t.Error("expected non-empty string output")
	}

	// Should contain both rules
	if !contains(output, "root") {
		t.Error("output missing root rule")
	}
	if !contains(output, "ws") {
		t.Error("output missing ws rule")
	}
}

func TestGrammar_AddRemoveRule(t *testing.T) {
	g := &Grammar{
		Rules:    make(map[string]*Rule),
		RootRule: "root",
	}

	// Add rules
	g.AddRule("root", `"hello"`)
	g.AddRule("ws", `[ \t]*`)

	if g.RuleCount() != 2 {
		t.Errorf("expected 2 rules, got %d", g.RuleCount())
	}

	// Remove rule
	g.RemoveRule("ws")

	if g.RuleCount() != 1 {
		t.Errorf("expected 1 rule after removal, got %d", g.RuleCount())
	}

	if g.HasRule("ws") {
		t.Error("ws rule should be removed")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// GRAMMAR COMPOSITION TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestMerge_MultipleGrammars(t *testing.T) {
	g1Text := `root ::= "a"
rule1 ::= "b"`

	g2Text := `root ::= "c"
rule2 ::= "d"`

	g1, err := Parse(g1Text)
	if err != nil {
		t.Fatalf("parse g1 error: %v", err)
	}

	g2, err := Parse(g2Text)
	if err != nil {
		t.Fatalf("parse g2 error: %v", err)
	}

	merged, err := Merge(g1, g2)
	if err != nil {
		t.Fatalf("merge error: %v", err)
	}

	// Should have 3 rules total (root from g2, rule1 from g1, rule2 from g2)
	if merged.RuleCount() != 3 {
		t.Errorf("expected 3 rules, got %d", merged.RuleCount())
	}

	// Root should be from g2 (last grammar wins)
	if root, ok := merged.GetRule("root"); ok {
		if root.Definition != `"c"` {
			t.Errorf("expected root from g2, got: %s", root.Definition)
		}
	} else {
		t.Error("root rule not found")
	}
}

func TestMerge_EmptyList(t *testing.T) {
	_, err := Merge()
	if err == nil {
		t.Error("expected error when merging empty list")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// VALIDATION HELPERS TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestValidateGrammarString_Valid(t *testing.T) {
	grammar := `root ::= "hello"
ws ::= [ \t]*`

	if err := ValidateGrammarString(grammar); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateGrammarString_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		grammar string
	}{
		{"invalid syntax", "root = hello"},
		{"missing root", "other ::= hello"},
		{"undefined ref", "root ::= missing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateGrammarString(tt.grammar); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestIsValidGrammar(t *testing.T) {
	tests := []struct {
		name    string
		grammar string
		valid   bool
	}{
		{"valid", `root ::= "test"`, true},
		{"invalid syntax", "root = test", false},
		{"missing root", `other ::= "test"`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidGrammar(tt.grammar)
			if result != tt.valid {
				t.Errorf("expected valid=%v, got %v", tt.valid, result)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
