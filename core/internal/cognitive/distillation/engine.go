package distillation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/cognitive/grammar"
	"github.com/normanking/cortex/internal/cognitive/router"
	"github.com/normanking/cortex/internal/cognitive/templates"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// DISTILLATION ENGINE
// ═══════════════════════════════════════════════════════════════════════════════

// Engine orchestrates runtime distillation from frontier models.
type Engine struct {
	llm      cognitive.SimpleChatProvider
	registry cognitive.Registry
	embedder router.Embedder
	tmplEng  *templates.Engine
	gramGen  *grammar.Generator
	log      *logging.Logger

	// Configuration
	frontierModel string
	graderModel   string
}

// EngineConfig configures the distillation engine.
type EngineConfig struct {
	LLM           cognitive.SimpleChatProvider
	Registry      cognitive.Registry
	Embedder      router.Embedder
	FrontierModel string // Model for teaching (e.g., "claude-sonnet-4")
	GraderModel   string // Model for grading (e.g., "claude-sonnet-4")
}

// NewEngine creates a new distillation engine.
func NewEngine(cfg *EngineConfig) *Engine {
	frontierModel := cfg.FrontierModel
	if frontierModel == "" {
		frontierModel = "claude-sonnet-4-20250514"
	}

	graderModel := cfg.GraderModel
	if graderModel == "" {
		graderModel = "claude-sonnet-4-20250514"
	}

	return &Engine{
		llm:           cfg.LLM,
		registry:      cfg.Registry,
		embedder:      cfg.Embedder,
		tmplEng:       templates.NewEngine(),
		gramGen:       grammar.NewGenerator(),
		log:           logging.Global(),
		frontierModel: frontierModel,
		graderModel:   graderModel,
	}
}

// generateID creates a random ID for tracking.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// SolveAndTeach handles a novel request by calling a frontier model
// and extracting a reusable template from the response.
func (e *Engine) SolveAndTeach(ctx context.Context, userInput string, taskType cognitive.TaskType) (*cognitive.DistillationResult, error) {
	start := time.Now()
	requestID := generateID()

	e.log.Info("[Distillation] Starting for request %s", requestID)

	// Call frontier model with teaching prompt
	messages := []cognitive.ChatMessage{
		{Role: "user", Content: userInput},
	}

	response, err := e.llm.Chat(ctx, messages, TeacherSystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("frontier call failed: %w", err)
	}

	frontierMs := int(time.Since(start).Milliseconds())
	e.log.Debug("[Distillation] Frontier response received in %dms", frontierMs)

	// Extract sections from response
	extractStart := time.Now()
	sections, err := ExtractSections(response)

	result := &cognitive.DistillationResult{
		Solution: ExtractSolutionOnly(response),
	}

	if err != nil {
		e.log.Warn("[Distillation] Section extraction failed: %v", err)
		// Record the attempt even if extraction failed
		e.recordDistillation(ctx, requestID, userInput, taskType, result, frontierMs, int(time.Since(extractStart).Milliseconds()), err.Error())
		return result, nil // Return solution without template
	}

	// Set solution from extracted sections
	result.Solution = sections.Solution

	// Validate and process extracted template
	template, validationErr := e.processExtractedTemplate(ctx, requestID, sections, taskType)
	if validationErr != nil {
		e.log.Warn("[Distillation] Template validation failed: %v", validationErr)
		e.recordDistillation(ctx, requestID, userInput, taskType, result, frontierMs, int(time.Since(extractStart).Milliseconds()), validationErr.Error())
		return result, nil // Return solution without template
	}

	result.Template = template
	result.CompilationPassed = true
	result.SchemaValid = true
	result.GrammarGenerated = template.GBNFGrammar != ""

	// Save template to registry
	if err := e.registry.Create(ctx, template); err != nil {
		e.log.Error("[Distillation] Failed to save template: %v", err)
		// Still return the result, just don't persist
	} else {
		e.log.Info("[Distillation] Created template %s: %s", template.ID, template.Name)
	}

	// Record successful distillation
	e.recordDistillation(ctx, requestID, userInput, taskType, result, frontierMs, int(time.Since(extractStart).Milliseconds()), "")

	return result, nil
}

// processExtractedTemplate validates and creates a Template from extracted sections.
func (e *Engine) processExtractedTemplate(ctx context.Context, requestID string, sections *ExtractedSections, taskType cognitive.TaskType) (*cognitive.Template, error) {
	// Safety Valve 1: Validate schema is flat
	if err := e.tmplEng.ValidateSchema(sections.Schema); err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}

	// Safety Valve 2: Compile template to check syntax
	if err := e.tmplEng.Compile(sections.Template); err != nil {
		return nil, fmt.Errorf("template compilation failed: %w", err)
	}

	// Safety Valve 3: Generate GBNF grammar
	gbnfGrammar, err := e.gramGen.Generate(sections.Schema)
	if err != nil {
		e.log.Warn("[Distillation] GBNF generation failed: %v", err)
		// This is not fatal - template can work without grammar
		gbnfGrammar = ""
	}

	// Generate embedding for intent
	var intentEmbedding cognitive.Embedding
	if e.embedder != nil && e.embedder.Available() {
		embedding, err := e.embedder.Embed(ctx, sections.Intent)
		if err != nil {
			e.log.Warn("[Distillation] Intent embedding failed: %v", err)
		} else {
			intentEmbedding = embedding
		}
	}

	// Create template
	templateID := generateID()
	template := &cognitive.Template{
		ID:              templateID,
		Name:            generateTemplateName(sections.Intent),
		Description:     sections.Intent,
		Intent:          sections.Intent,
		IntentEmbedding: intentEmbedding,
		TemplateBody:    sections.Template,
		VariableSchema:  sections.Schema,
		GBNFGrammar:     gbnfGrammar,
		TaskType:        taskType,
		Status:          cognitive.StatusProbation,
		ConfidenceScore: 0.5, // Start at 50%
		ComplexityScore: 50,  // Default complexity
		SourceType:      cognitive.SourceDistillation,
		SourceModel:     e.frontierModel,
		SourceRequestID: requestID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	return template, nil
}

// recordDistillation saves distillation attempt to the registry.
func (e *Engine) recordDistillation(ctx context.Context, requestID, userInput string, taskType cognitive.TaskType, result *cognitive.DistillationResult, frontierMs, extractionMs int, errMsg string) {
	req := &cognitive.DistillationRequest{
		ID:                requestID,
		UserInput:         userInput,
		TaskType:          taskType,
		FrontierModel:     e.frontierModel,
		Solution:          result.Solution,
		TemplateCreated:   result.Template != nil,
		CompilationPassed: result.CompilationPassed,
		SchemaValid:       result.SchemaValid,
		GrammarGenerated:  result.GrammarGenerated,
		FrontierMs:        frontierMs,
		ExtractionMs:      extractionMs,
		ExtractionError:   errMsg,
		CreatedAt:         time.Now(),
	}

	if result.Template != nil {
		req.TemplateID = result.Template.ID
	}

	if err := e.registry.RecordDistillation(ctx, req); err != nil {
		e.log.Error("[Distillation] Failed to record distillation: %v", err)
	}
}

// generateTemplateName creates a human-readable name from an intent.
func generateTemplateName(intent string) string {
	// Truncate and clean up
	name := intent
	if len(name) > 50 {
		name = name[:50] + "..."
	}
	return name
}

// ═══════════════════════════════════════════════════════════════════════════════
// GRADING
// ═══════════════════════════════════════════════════════════════════════════════

// GradeTemplateExecution evaluates a template execution and updates confidence.
func (e *Engine) GradeTemplateExecution(ctx context.Context, templateID string, usageLogID int64, userInput, renderedOutput string) (*cognitive.GradingResult, error) {
	e.log.Debug("[Distillation] Grading template %s", templateID)

	// Get the template
	template, err := e.registry.Get(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}

	// Build grading prompt
	gradingPrompt := fmt.Sprintf(`Evaluate this template-generated response:

ORIGINAL USER REQUEST:
%s

TEMPLATE USED:
Name: %s
Intent: %s

GENERATED RESPONSE:
%s

Provide your evaluation as JSON.`, userInput, template.Name, template.Intent, renderedOutput)

	messages := []cognitive.ChatMessage{
		{Role: "user", Content: gradingPrompt},
	}

	// Call grader model
	response, err := e.llm.Chat(ctx, messages, GraderSystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("grading call failed: %w", err)
	}

	// Parse grading response
	gradeResp, err := ParseGradeResponse(response)
	if err != nil {
		return nil, fmt.Errorf("parse grade response: %w", err)
	}

	// Calculate confidence delta
	var confidenceDelta float64
	switch gradeResp.Grade {
	case "pass":
		confidenceDelta = 0.1
	case "partial":
		confidenceDelta = 0.0
	case "fail":
		confidenceDelta = -0.1
	}

	// Create grading result
	result := &cognitive.GradingResult{
		TemplateID:        templateID,
		UsageLogID:        &usageLogID,
		GraderModel:       e.graderModel,
		Grade:             cognitive.GradeType(gradeResp.Grade),
		GradeReason:       gradeResp.Reason,
		CorrectnessScore:  gradeResp.CorrectnessScore,
		CompletenessScore: gradeResp.CompletenessScore,
		ConfidenceDelta:   confidenceDelta,
		CreatedAt:         time.Now(),
	}

	// Record the grade
	if err := e.registry.RecordGrade(ctx, result); err != nil {
		return nil, fmt.Errorf("record grade: %w", err)
	}

	e.log.Info("[Distillation] Template %s graded: %s (confidence delta: %+.2f)", templateID, result.Grade, confidenceDelta)

	return result, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// BATCH GRADING
// ═══════════════════════════════════════════════════════════════════════════════

// GradePendingUsages grades all pending usage logs for probationary templates.
func (e *Engine) GradePendingUsages(ctx context.Context, limit int) (int, error) {
	// Get pending usage logs
	logs, err := e.registry.GetPendingGrades(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("get pending grades: %w", err)
	}

	graded := 0
	for _, log := range logs {
		_, err := e.GradeTemplateExecution(ctx, log.TemplateID, log.ID, log.UserInput, log.RenderedOutput)
		if err != nil {
			e.log.Warn("[Distillation] Failed to grade usage %d: %v", log.ID, err)
			continue
		}
		graded++
	}

	return graded, nil
}
