package introspection

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/normanking/cortex/internal/memory"
)

// MetacognitiveResponder generates natural language responses for introspection queries.
type MetacognitiveResponder struct {
	templates map[ResponseTemplate]*template.Template
	funcMap   template.FuncMap
}

// NewMetacognitiveResponder creates a new responder with templates.
func NewMetacognitiveResponder() *MetacognitiveResponder {
	r := &MetacognitiveResponder{
		templates: make(map[ResponseTemplate]*template.Template),
	}
	r.funcMap = template.FuncMap{
		"add":     func(a, b int) int { return a + b },
		"mul":     func(a, b float64) float64 { return a * b },
		"join":    strings.Join,
		"percent": func(f float64) string { return fmt.Sprintf("%.0f%%", f*100) },
	}
	r.loadTemplates()
	return r
}

// loadTemplates initializes all response templates.
func (r *MetacognitiveResponder) loadTemplates() {
	// Knowledge found template
	r.templates[TemplateKnowledgeFound] = template.Must(
		template.New("found").Funcs(r.funcMap).Parse(`I searched my memory and found **{{.MatchCount}} items** related to "{{.Subject}}".

{{if .TopResults}}**Top Results:**
{{range $i, $item := .TopResults}}{{add $i 1}}. {{if $item.Summary}}{{$item.Summary}}{{else}}{{$item.Content}}{{end}} (from {{$item.Source}})
{{end}}{{end}}
{{if .RelatedTopics}}**Related Topics:** {{join .RelatedTopics ", "}}
{{end}}
Would you like me to retrieve more details or search for additional information?`))

	// Not found but can answer template
	r.templates[TemplateKnowledgeNotFoundCanAnswer] = template.Must(
		template.New("not_found_can").Funcs(r.funcMap).Parse(`I searched my memory and found **0 items** stored about "{{.Subject}}".

However, I can answer questions about {{.Subject}} from my general training (confidence: {{percent .LLMConfidence}}).

Would you like me to:
1. Answer your question from my general knowledge
2. Search the internet and add {{.Subject}} to my memory
3. Ingest a file containing {{.Subject}} information`))

	// Not found and cannot answer template
	r.templates[TemplateKnowledgeNotFoundCannotAnswer] = template.Must(
		template.New("not_found_cannot").Funcs(r.funcMap).Parse(`I searched my memory and found **0 items** stored about "{{.Subject}}".

My general knowledge on this topic is also limited (confidence: {{percent .LLMConfidence}}).

I can learn about {{.Subject}} by:
{{range $i, $opt := .AcquisitionOptions}}{{add $i 1}}. {{$opt.Description}}
{{end}}
Which would you prefer?`))

	// Acquisition offer template
	r.templates[TemplateAcquisitionOffer] = template.Must(
		template.New("offer").Funcs(r.funcMap).Parse(`I don't have "{{.Subject}}" in my memory. Would you like me to learn it?

**Options:**
{{range $i, $opt := .AcquisitionOptions}}{{add $i 1}}. **{{$opt.Type}}** - {{$opt.Description}} ({{$opt.Effort}} effort)
{{end}}
Just say which option you'd prefer, or provide a file path to ingest.`))

	// Acquisition started template
	r.templates[TemplateAcquisitionStarted] = template.Must(
		template.New("started").Funcs(r.funcMap).Parse(`Starting to learn about "{{.Subject}}" via {{.AcquisitionType}}...

I'll let you know when I'm done.`))

	// Acquisition complete template
	r.templates[TemplateAcquisitionComplete] = template.Must(
		template.New("complete").Funcs(r.funcMap).Parse(`**Learning Complete!**

I successfully ingested **{{.ItemsIngested}} items** about "{{.Subject}}".

{{if .Categories}}**Categories:** {{join .Categories ", "}}
{{end}}
I now have {{.Subject}} in my memory. Feel free to ask me anything about it!`))

	// Acquisition failed template
	r.templates[TemplateAcquisitionFailed] = template.Must(
		template.New("failed").Funcs(r.funcMap).Parse(`I encountered an issue while trying to learn "{{.Subject}}":

> {{.ErrorMessage}}

Would you like me to try a different approach?`))
}

// Generate creates a response from a template and context.
func (r *MetacognitiveResponder) Generate(tmpl ResponseTemplate, ctx *ResponseContext) (string, error) {
	t, ok := r.templates[tmpl]
	if !ok {
		return "", fmt.Errorf("template not found: %s", tmpl)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return strings.TrimSpace(buf.String()), nil
}

// SelectTemplate chooses the appropriate template based on gap analysis.
func (r *MetacognitiveResponder) SelectTemplate(analysis *GapAnalysis) ResponseTemplate {
	// Has stored knowledge
	if analysis.HasStoredKnowledge {
		return TemplateKnowledgeFound
	}

	// No stored knowledge but LLM can help
	if analysis.LLMCanAnswer && analysis.LLMConfidence > 0.6 {
		return TemplateKnowledgeNotFoundCanAnswer
	}

	// No stored knowledge and LLM has limited capability
	return TemplateKnowledgeNotFoundCannotAnswer
}

// GenerateFromAnalysis creates a response from gap analysis and inventory results.
func (r *MetacognitiveResponder) GenerateFromAnalysis(analysis *GapAnalysis, inventory *memory.InventoryResult) (string, error) {
	tmpl := r.SelectTemplate(analysis)

	ctx := &ResponseContext{
		Subject:            analysis.Subject,
		MatchCount:         analysis.StoredKnowledgeCount,
		LLMCanAnswer:       analysis.LLMCanAnswer,
		LLMConfidence:      analysis.LLMConfidence,
		AcquisitionOptions: analysis.AcquisitionOptions,
	}

	if inventory != nil {
		ctx.RelatedTopics = inventory.RelatedTopics
		// Convert inventory items
		for _, item := range inventory.TopResults {
			ctx.TopResults = append(ctx.TopResults, InventoryItem{
				ID:        item.ID,
				Source:    item.Source,
				Content:   item.Content,
				Summary:   item.Summary,
				Relevance: item.Relevance,
				Metadata:  item.Metadata,
			})
		}
	}

	return r.Generate(tmpl, ctx)
}
