package grammar_test

import (
	"fmt"
	"log"

	"github.com/normanking/cortex/internal/cognitive/grammar"
)

// Example demonstrates basic GBNF grammar generation from a JSON Schema.
func Example() {
	// Define a JSON Schema for template variables
	schema := `{
		"type": "object",
		"properties": {
			"language": {"type": "string"},
			"framework": {"type": "string"},
			"task_type": {
				"type": "string",
				"enum": ["bug_fix", "feature", "refactor"]
			}
		},
		"required": ["language", "task_type"]
	}`

	// Generate GBNF grammar
	gen := grammar.NewGenerator()
	gbnfGrammar, err := gen.Generate(schema)
	if err != nil {
		log.Fatalf("Failed to generate grammar: %v", err)
	}

	// Validate the generated grammar
	if err := grammar.ValidateGrammarString(gbnfGrammar); err != nil {
		log.Fatalf("Generated grammar is invalid: %v", err)
	}

	fmt.Println("Grammar generated successfully!")
	// Output: Grammar generated successfully!
}

// ExampleGenerator_Generate demonstrates generating a GBNF grammar for a code review template.
func ExampleGenerator_Generate() {
	schema := `{
		"type": "object",
		"properties": {
			"file_path": {"type": "string"},
			"severity": {
				"type": "string",
				"enum": ["critical", "major", "minor"]
			},
			"line_number": {"type": "integer"}
		},
		"required": ["file_path", "severity"]
	}`

	gen := grammar.NewGenerator()
	gbnfGrammar, err := gen.Generate(schema)
	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	// The grammar can now be passed to Ollama's chat API
	// to constrain the LLM's output to match this exact schema
	_ = gbnfGrammar

	fmt.Println("Code review grammar generated")
	// Output: Code review grammar generated
}

// ExampleValidateAgainstSchema demonstrates validating LLM output against a schema.
func ExampleValidateAgainstSchema() {
	schema := `{
		"type": "object",
		"properties": {
			"task": {"type": "string"},
			"priority": {"type": "integer"}
		},
		"required": ["task", "priority"]
	}`

	// Valid output from LLM
	validOutput := `{"task": "Fix bug", "priority": 1}`

	if err := grammar.ValidateAgainstSchema(validOutput, schema); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	fmt.Println("Output is valid")
	// Output: Output is valid
}

// ExampleNewSchemaConstraint demonstrates using schema constraints.
func ExampleNewSchemaConstraint() {
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		},
		"required": ["name"]
	}`

	constraint, err := grammar.NewSchemaConstraint(schema)
	if err != nil {
		log.Fatalf("Failed to create constraint: %v", err)
	}

	// Validate output
	output := `{"name": "Alice", "age": 30}`
	if err := constraint.Validate(output); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	fmt.Println("Schema validation passed")
	// Output: Schema validation passed
}

// ExampleConstraintSet demonstrates combining multiple constraints.
func ExampleConstraintSet() {
	cs := &grammar.ConstraintSet{}

	// Add JSON object constraint
	cs.Add(&grammar.JSONObjectConstraint{})

	// Add length constraint (10-100 characters)
	cs.Add(grammar.NewLengthConstraint(10, 100))

	// Validate output against all constraints
	output := `{"key": "value"}`
	if err := cs.Validate(output); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	fmt.Println("All constraints passed")
	// Output: All constraints passed
}

// ExampleNewEnumConstraint demonstrates enum value validation.
func ExampleNewEnumConstraint() {
	// Only allow specific values
	constraint := grammar.NewEnumConstraint("pending", "approved", "rejected")

	// Valid value
	if err := constraint.Validate("approved"); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	fmt.Println("Enum validation passed")
	// Output: Enum validation passed
}

// ExampleNewIntegerConstraint demonstrates integer range validation.
func ExampleNewIntegerConstraint() {
	min := 1
	max := 100
	constraint := grammar.NewIntegerConstraint(&min, &max)

	// Valid integer within range
	if err := constraint.Validate("50"); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	fmt.Println("Integer validation passed")
	// Output: Integer validation passed
}

// ExampleJSONGrammar demonstrates using the built-in JSON grammar.
func ExampleJSONGrammar() {
	gbnfGrammar := grammar.JSONGrammar()

	// This grammar accepts any valid JSON
	if err := grammar.ValidateGrammarString(gbnfGrammar); err != nil {
		log.Fatalf("Grammar validation failed: %v", err)
	}

	fmt.Println("JSON grammar is valid")
	// Output: JSON grammar is valid
}

// ExampleBooleanGrammar demonstrates using the built-in boolean grammar.
func ExampleBooleanGrammar() {
	gbnfGrammar := grammar.BooleanGrammar()

	// This grammar only accepts "true" or "false"
	// Use this when you want a yes/no decision from the LLM
	_ = gbnfGrammar

	fmt.Println("Boolean grammar generated")
	// Output: Boolean grammar generated
}

// ExampleParse demonstrates parsing a GBNF grammar string.
func ExampleParse() {
	grammarText := `root ::= "hello"
ws ::= [ \t\n\r]*`

	g, err := grammar.Parse(grammarText)
	if err != nil {
		log.Fatalf("Parse failed: %v", err)
	}

	// Check if grammar has required rules
	if !g.HasRule("root") {
		log.Fatal("Missing root rule")
	}

	fmt.Printf("Grammar has %d rules\n", g.RuleCount())
	// Output: Grammar has 2 rules
}

// ExampleGrammar_Validate demonstrates validating a parsed grammar.
func ExampleGrammar_Validate() {
	grammarText := `root ::= value
value ::= string | number
string ::= "\"" [^"]* "\""
number ::= [0-9]+`

	g, err := grammar.Parse(grammarText)
	if err != nil {
		log.Fatalf("Parse failed: %v", err)
	}

	// Validate checks for undefined references and cycles
	if err := g.Validate(); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	fmt.Println("Grammar is valid")
	// Output: Grammar is valid
}

// ExampleMerge demonstrates merging multiple grammars.
func ExampleMerge() {
	grammar1 := `root ::= greeting
greeting ::= "hello"`

	grammar2 := `root ::= greeting
farewell ::= "goodbye"`

	g1, err := grammar.Parse(grammar1)
	if err != nil {
		log.Fatalf("Parse g1 failed: %v", err)
	}

	g2, err := grammar.Parse(grammar2)
	if err != nil {
		log.Fatalf("Parse g2 failed: %v", err)
	}

	merged, err := grammar.Merge(g1, g2)
	if err != nil {
		log.Fatalf("Merge failed: %v", err)
	}

	fmt.Printf("Merged grammar has %d rules\n", merged.RuleCount())
	// Output: Merged grammar has 3 rules
}
