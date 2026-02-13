// Package templates provides template rendering and variable extraction.
package templates

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// TEMPLATE ENGINE
// ═══════════════════════════════════════════════════════════════════════════════

// Engine handles template rendering and variable extraction.
type Engine struct {
	log *logging.Logger
}

// NewEngine creates a new template engine.
func NewEngine() *Engine {
	return &Engine{
		log: logging.Global(),
	}
}

// RenderResult contains the output of a template render operation.
type RenderResult struct {
	Output      string        `json:"output"`
	Variables   map[string]interface{} `json:"variables"`
	RenderMs    int           `json:"render_ms"`
	TemplateID  string        `json:"template_id"`
}

// Render executes a template with the given variables.
func (e *Engine) Render(ctx context.Context, t *cognitive.Template, variables map[string]interface{}) (*RenderResult, error) {
	start := time.Now()

	// Parse the template
	tmpl, err := template.New(t.ID).Funcs(templateFuncs()).Parse(t.TemplateBody)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	// Execute the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return &RenderResult{
		Output:     buf.String(),
		Variables:  variables,
		RenderMs:   int(time.Since(start).Milliseconds()),
		TemplateID: t.ID,
	}, nil
}

// Compile validates a template body without executing it.
// Returns an error if the template is invalid.
func (e *Engine) Compile(templateBody string) error {
	_, err := template.New("compile").Funcs(templateFuncs()).Parse(templateBody)
	if err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}
	return nil
}

// RenderSimple renders a template body string with variables, without requiring a full Template object.
// This is a convenience method for quick rendering in the pipeline.
func (e *Engine) RenderSimple(templateBody string, variables map[string]interface{}) (string, error) {
	tmpl, err := template.New("simple").Funcs(templateFuncs()).Parse(templateBody)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// EXTRACTION PROMPT GENERATION
// ═══════════════════════════════════════════════════════════════════════════════

// ExtractionPrompt contains the prompt for extracting variables from user input.
type ExtractionPrompt struct {
	SystemPrompt string `json:"system_prompt"`
	UserPrompt   string `json:"user_prompt"`
	Schema       string `json:"schema"`       // JSON Schema for expected output
	Grammar      string `json:"grammar"`      // GBNF grammar for constrained generation
}

// RenderExtractionPrompt creates a prompt for extracting variables from user input.
func (e *Engine) RenderExtractionPrompt(t *cognitive.Template, userInput string) *ExtractionPrompt {
	// Build the system prompt
	systemPrompt := fmt.Sprintf(`You are a variable extraction assistant. Your task is to extract specific variables from user input.

TEMPLATE: %s
DESCRIPTION: %s

You must extract the following variables according to this JSON Schema:
%s

RULES:
1. Extract ONLY the variables defined in the schema
2. Use the exact field names from the schema
3. If a value cannot be determined, use the default value or null
4. Output ONLY valid JSON - no explanations, no markdown
5. Match the types exactly as specified in the schema`, t.Name, t.Description, t.VariableSchema)

	// Build the user prompt
	userPrompt := fmt.Sprintf(`Extract variables from this input:

"%s"

Output the extracted variables as JSON:`, userInput)

	return &ExtractionPrompt{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Schema:       t.VariableSchema,
		Grammar:      t.GBNFGrammar,
	}
}

// ParseExtractedVariables parses the LLM response into a variable map.
func (e *Engine) ParseExtractedVariables(response string) (map[string]interface{}, error) {
	// Clean up the response - remove any markdown formatting
	cleaned := strings.TrimSpace(response)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var variables map[string]interface{}
	if err := json.Unmarshal([]byte(cleaned), &variables); err != nil {
		return nil, fmt.Errorf("parse variables JSON: %w", err)
	}

	return variables, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCHEMA VALIDATION
// ═══════════════════════════════════════════════════════════════════════════════

// ValidateSchema checks if a JSON schema is valid and flat (no nested objects).
// The cognitive architecture requires flat schemas for reliable GBNF generation.
func (e *Engine) ValidateSchema(schemaJSON string) error {
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Check for properties
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("schema must have 'properties' object")
	}

	// Validate each property
	for name, propDef := range props {
		prop, ok := propDef.(map[string]interface{})
		if !ok {
			return fmt.Errorf("property '%s' must be an object", name)
		}

		propType, _ := prop["type"].(string)

		// Check for nested objects (not allowed)
		if propType == "object" {
			return fmt.Errorf("property '%s': nested objects are not allowed (schema must be flat)", name)
		}

		// Check array items for nested objects
		if propType == "array" {
			items, _ := prop["items"].(map[string]interface{})
			if items != nil {
				itemType, _ := items["type"].(string)
				if itemType == "object" {
					return fmt.Errorf("property '%s': arrays of objects are not allowed", name)
				}
			}
		}
	}

	return nil
}

// IsFlatSchema returns true if the schema has no nested objects.
func (e *Engine) IsFlatSchema(schemaJSON string) bool {
	return e.ValidateSchema(schemaJSON) == nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// TEMPLATE FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// templateFuncs returns the custom functions available in templates.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		// String functions
		"lower":      strings.ToLower,
		"upper":      strings.ToUpper,
		"title":      strings.Title,
		"trim":       strings.TrimSpace,
		"replace":    strings.ReplaceAll,
		"contains":   strings.Contains,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,
		"split":      strings.Split,
		"join":       strings.Join,

		// JSON functions
		"toJSON": func(v interface{}) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
		"toPrettyJSON": func(v interface{}) string {
			b, _ := json.MarshalIndent(v, "", "  ")
			return string(b)
		},

		// Control flow
		"default": func(defaultVal, val interface{}) interface{} {
			if val == nil || val == "" {
				return defaultVal
			}
			return val
		},

		// List functions
		"first": func(list []interface{}) interface{} {
			if len(list) > 0 {
				return list[0]
			}
			return nil
		},
		"last": func(list []interface{}) interface{} {
			if len(list) > 0 {
				return list[len(list)-1]
			}
			return nil
		},
		"len": func(list interface{}) int {
			switch v := list.(type) {
			case []interface{}:
				return len(v)
			case []string:
				return len(v)
			case string:
				return len(v)
			default:
				return 0
			}
		},

		// Formatting
		"indent": func(spaces int, s string) string {
			pad := strings.Repeat(" ", spaces)
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				lines[i] = pad + line
			}
			return strings.Join(lines, "\n")
		},
		"wrap": func(width int, s string) string {
			// Simple word wrap
			if len(s) <= width {
				return s
			}
			var result strings.Builder
			words := strings.Fields(s)
			lineLen := 0
			for i, word := range words {
				if lineLen+len(word)+1 > width && lineLen > 0 {
					result.WriteString("\n")
					lineLen = 0
				} else if i > 0 {
					result.WriteString(" ")
					lineLen++
				}
				result.WriteString(word)
				lineLen += len(word)
			}
			return result.String()
		},

		// Code generation helpers
		"codeBlock": func(lang, code string) string {
			return fmt.Sprintf("```%s\n%s\n```", lang, code)
		},
		"bullet": func(items []string) string {
			var result strings.Builder
			for _, item := range items {
				result.WriteString("- ")
				result.WriteString(item)
				result.WriteString("\n")
			}
			return result.String()
		},
		"numbered": func(items []string) string {
			var result strings.Builder
			for i, item := range items {
				result.WriteString(fmt.Sprintf("%d. %s\n", i+1, item))
			}
			return result.String()
		},
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// TEMPLATE BUILDER
// ═══════════════════════════════════════════════════════════════════════════════

// TemplateBuilder helps construct templates programmatically.
type TemplateBuilder struct {
	template *cognitive.Template
}

// NewTemplateBuilder creates a new template builder.
func NewTemplateBuilder(id, name string) *TemplateBuilder {
	return &TemplateBuilder{
		template: &cognitive.Template{
			ID:              id,
			Name:            name,
			Status:          cognitive.StatusProbation,
			SourceType:      cognitive.SourceManual,
			ConfidenceScore: 0.5,
			ComplexityScore: 50,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}
}

// WithIntent sets the intent and optional keywords.
func (b *TemplateBuilder) WithIntent(intent string, keywords ...string) *TemplateBuilder {
	b.template.Intent = intent
	b.template.IntentKeywords = keywords
	return b
}

// WithDescription sets the description.
func (b *TemplateBuilder) WithDescription(desc string) *TemplateBuilder {
	b.template.Description = desc
	return b
}

// WithBody sets the template body.
func (b *TemplateBuilder) WithBody(body string) *TemplateBuilder {
	b.template.TemplateBody = body
	return b
}

// WithSchema sets the variable schema (JSON).
func (b *TemplateBuilder) WithSchema(schema string) *TemplateBuilder {
	b.template.VariableSchema = schema
	return b
}

// WithTaskType sets the task type.
func (b *TemplateBuilder) WithTaskType(taskType cognitive.TaskType) *TemplateBuilder {
	b.template.TaskType = taskType
	return b
}

// WithDomain sets the domain.
func (b *TemplateBuilder) WithDomain(domain string) *TemplateBuilder {
	b.template.Domain = domain
	return b
}

// Build returns the constructed template.
func (b *TemplateBuilder) Build() *cognitive.Template {
	return b.template
}
