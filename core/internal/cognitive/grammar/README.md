---
project: Cortex
component: Docs
phase: Archive
date_created: 2025-12-18T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:51.332459
---

# GBNF Grammar Engine

The grammar package provides JSON Schema to GBNF (GGML BNF) grammar conversion and token-level constraint validation for Cortex's Cognitive Architecture v2.1.

## Overview

**Purpose**: Generate GBNF grammars from JSON Schemas to constrain LLM output at the token level, making invalid JSON **impossible** during generation.

**Key Benefit**: When using Ollama with GBNF grammars, the LLM cannot generate tokens that would violate the schema - this eliminates the need for post-generation validation and retry loops.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Grammar Engine Flow                       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  JSON Schema                                                │
│       │                                                      │
│       ├─► Generator.Generate()                              │
│       │        │                                             │
│       │        ├─► Parse Schema                             │
│       │        ├─► Generate GBNF Rules                      │
│       │        └─► Output: GBNF Grammar String              │
│       │                 │                                    │
│       │                 ├─► Ollama (constrained generation)  │
│       │                 │         │                          │
│       │                 │         └─► Valid JSON Output      │
│       │                 │                                    │
│       │                 └─► ValidateAgainstGrammar()        │
│       │                           │                          │
│       │                           └─► Validation Result      │
│       │                                                      │
│       └─► Constraints (Optional Post-Validation)            │
│                │                                             │
│                ├─► SchemaConstraint                          │
│                ├─► JSONConstraint                            │
│                ├─► RegexConstraint                           │
│                └─► EnumConstraint                            │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Generator (`generator.go`)

Converts JSON Schemas to GBNF grammar format.

**Supported Types**:
- `string` - JSON strings with escape sequences
- `number` - Floating point numbers
- `integer` - Whole numbers
- `boolean` - true/false
- `array` - Arrays with typed elements
- `enum` - Limited set of allowed values
- `null` - Null value

**Limitations**:
- **Flat schemas only** - No nested objects (by design for reliability)
- Required fields must be present in generated output
- Properties appear in deterministic order

**Example**:
```go
schema := `{
    "type": "object",
    "properties": {
        "task": {"type": "string"},
        "priority": {"type": "integer"},
        "status": {
            "type": "string",
            "enum": ["pending", "done"]
        }
    },
    "required": ["task", "status"]
}`

gen := grammar.NewGenerator()
gbnfGrammar, err := gen.Generate(schema)
// gbnfGrammar can now be passed to Ollama's chat API
```

### 2. Grammar Types (`grammar.go`)

Defines GBNF grammar structure and validation.

**Key Types**:
- `Grammar` - Parsed grammar with rules and root
- `Rule` - Single production rule
- `ParsedRule` - Structured representation of rule

**Functions**:
- `Parse(grammarText)` - Parse GBNF grammar string
- `Validate()` - Check for undefined references and cycles
- `Merge(grammars...)` - Combine multiple grammars

**Example**:
```go
grammarText := `root ::= value
value ::= string | number
string ::= "\"" [^"]* "\""
number ::= [0-9]+`

g, err := grammar.Parse(grammarText)
if err := g.Validate(); err != nil {
    // Handle invalid grammar
}
```

### 3. Constraints (`constraints.go`)

Token-level validation for LLM output.

**Constraint Types**:

| Constraint | Purpose | Example |
|------------|---------|---------|
| `JSONConstraint` | Valid JSON | Any JSON value |
| `JSONObjectConstraint` | JSON object only | `{"key": "value"}` |
| `JSONArrayConstraint` | JSON array only | `[1, 2, 3]` |
| `SchemaConstraint` | Match JSON Schema | Required fields, types |
| `RegexConstraint` | Pattern matching | Phone numbers, emails |
| `EnumConstraint` | Limited values | `["red", "green", "blue"]` |
| `BooleanConstraint` | true/false only | Boolean values |
| `IntegerConstraint` | Integer with range | Min/max bounds |
| `NumberConstraint` | Number with range | Min/max bounds |
| `LengthConstraint` | String length | Min/max characters |

**Example**:
```go
// Combine multiple constraints
cs := &grammar.ConstraintSet{}
cs.Add(&grammar.JSONObjectConstraint{})
cs.Add(grammar.NewLengthConstraint(10, 500))

output := `{"key": "value"}`
if err := cs.Validate(output); err != nil {
    // Output doesn't meet constraints
}
```

## Integration with Cognitive Architecture

### Template Engine Integration

The grammar engine integrates with the template engine for variable extraction:

```go
// In templates/engine.go
type ExtractionPrompt struct {
    SystemPrompt string
    UserPrompt   string
    Schema       string  // JSON Schema
    Grammar      string  // GBNF Grammar (pre-computed)
}

// When calling Ollama
resp, err := ollama.Chat(ctx, &ollama.ChatRequest{
    Model: "llama3.2:3b",
    Messages: []ollama.Message{
        {Role: "system", Content: prompt.SystemPrompt},
        {Role: "user", Content: prompt.UserPrompt},
    },
    Options: map[string]interface{}{
        "grammar": prompt.Grammar, // GBNF grammar constrains output
    },
})
```

### Distillation Engine Integration

The distillation engine generates grammars for new templates:

```go
// In distillation/engine.go
func (e *Engine) extractTemplate(solution string) (*DistillationResult, error) {
    // ... extract schema from frontier model response ...

    // Generate GBNF grammar (Safety Valve 3)
    gbnfGrammar, err := e.gramGen.Generate(sections.Schema)
    if err != nil {
        // Non-fatal: template can work without grammar
        gbnfGrammar = ""
    }

    template := &cognitive.Template{
        VariableSchema: sections.Schema,
        GBNFGrammar:    gbnfGrammar,  // Stored for future use
        // ... other fields ...
    }

    return &DistillationResult{
        Template:         template,
        GrammarGenerated: gbnfGrammar != "",
    }, nil
}
```

## GBNF Format

GBNF (GGML BNF) is the grammar format used by llama.cpp and Ollama.

**Syntax**:
```
rule_name ::= definition

Operators:
  |   - Alternation (choice)
  *   - Zero or more
  +   - One or more
  ?   - Optional
  ()  - Grouping
  []  - Character class
  ""  - Literal string
```

**Example GBNF Grammar**:
```
root ::= object
object ::= "{" ws members ws "}"
members ::= pair ("," ws pair)*
pair ::= string ":" ws value
value ::= string | number | "true" | "false" | "null"
string ::= "\"" ([^"\\] | "\\" (["\\/bfnrt] | "u" [0-9a-fA-F]{4}))* "\""
number ::= "-"? ([0-9] | [1-9] [0-9]*) ("." [0-9]+)? ([eE] [+-]? [0-9]+)?
ws ::= [ \t\n\r]*
```

## Common Grammars

The package provides pre-built grammars for common use cases:

```go
// Any valid JSON
grammar.JSONGrammar()

// Simple flat object with specific fields
grammar.SimpleObjectGrammar("name", "email", "age")

// Boolean only (true/false)
grammar.BooleanGrammar()

// Yes/No (as JSON strings)
grammar.YesNoGrammar()

// Integer only
grammar.IntegerGrammar()

// List of strings
grammar.ListGrammar()
```

## Validation Helpers

```go
// Validate GBNF grammar string
err := grammar.ValidateGrammarString(gbnfGrammar)

// Check if grammar is valid (bool)
valid := grammar.IsValidGrammar(gbnfGrammar)

// Validate output against grammar
err := grammar.ValidateAgainstGrammar(output, gbnfGrammar)

// Validate output against JSON Schema
err := grammar.ValidateAgainstSchema(output, schemaJSON)

// Quick JSON validation
err := grammar.ValidateJSON(output)
err := grammar.ValidateJSONObject(output)
err := grammar.ValidateJSONArray(output)
```

## Testing

Run tests with coverage:
```bash
go test -v -cover ./internal/cognitive/grammar/...
```

Run examples:
```bash
go test -v ./internal/cognitive/grammar/... -run "^Example"
```

## Performance Characteristics

- **Generation**: O(n) where n = number of schema properties
- **Validation**: O(n) where n = number of grammar rules
- **Constraint Check**: O(1) for most constraints, O(n) for schema validation

**Benchmarks**:
- Simple schema (3 fields): ~0.1ms generation
- Complex schema (10 fields): ~0.3ms generation
- Grammar validation: ~0.05ms
- JSON constraint: ~0.01ms

## Best Practices

### 1. Keep Schemas Flat
```go
// ✅ GOOD - Flat schema
{
    "type": "object",
    "properties": {
        "name": {"type": "string"},
        "age": {"type": "integer"}
    }
}

// ❌ BAD - Nested objects (not supported)
{
    "type": "object",
    "properties": {
        "user": {
            "type": "object",
            "properties": {
                "name": {"type": "string"}
            }
        }
    }
}
```

### 2. Use Enums for Limited Choices
```go
// Better constraint than pattern matching
{
    "type": "string",
    "enum": ["pending", "approved", "rejected"]
}
```

### 3. Pre-compute Grammars
```go
// Generate once, store in template
template := &cognitive.Template{
    VariableSchema: schemaJSON,
    GBNFGrammar:    precomputedGrammar, // Don't regenerate on every use
}
```

### 4. Combine Constraints for Extra Safety
```go
// Grammar constrains during generation
// Constraints validate after generation
cs := &grammar.ConstraintSet{}
cs.Add(grammar.NewSchemaConstraint(schema))
cs.Add(grammar.NewLengthConstraint(10, 1000))
```

## Error Handling

```go
gen := grammar.NewGenerator()
gbnfGrammar, err := gen.Generate(schema)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "nested objects"):
        // Schema needs to be flattened
    case strings.Contains(err.Error(), "invalid JSON"):
        // Schema is not valid JSON
    default:
        // Other generation error
    }
}
```

## Future Enhancements

Potential future additions (not yet implemented):

- [ ] Full GBNF parser for output validation
- [ ] Support for JSON Schema references (`$ref`)
- [ ] Recursive grammar generation for nested objects
- [ ] Grammar optimization (rule deduplication)
- [ ] GBNF to JSON Schema reverse conversion
- [ ] Performance optimizations for large schemas

## References

- [GBNF Documentation](https://github.com/ggerganov/llama.cpp/blob/master/grammars/README.md)
- [JSON Schema Specification](https://json-schema.org/specification.html)
- [Ollama API Documentation](https://github.com/ollama/ollama/blob/main/docs/api.md)
- Cortex Cognitive Architecture v2.1 Design Document

## License

Part of the Cortex project.
