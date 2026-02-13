// Package feedback implements the learning feedback loop for the cognitive architecture.
// It handles template lifecycle management, grading orchestration, and continuous improvement.
package feedback

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// FEEDBACK LOOP
// ═══════════════════════════════════════════════════════════════════════════════

// Grader evaluates template executions.
type Grader interface {
	GradeTemplateExecution(ctx context.Context, templateID string, usageLogID int64, userInput, renderedOutput string) (*cognitive.GradingResult, error)
}

// LoopConfig configures the feedback loop.
type LoopConfig struct {
	// GradingBatchSize is the number of pending grades to process per cycle.
	GradingBatchSize int

	// PromotionInterval is how often to run the promotion cycle.
	PromotionInterval time.Duration

	// MinGradesForValidation is the minimum grades needed for validation.
	MinGradesForValidation int

	// MinUsesForPromotion is the minimum uses needed for promotion.
	MinUsesForPromotion int

	// MinPassRateForValidation is the pass rate needed for validation.
	MinPassRateForValidation float64

	// MinSuccessRateForPromotion is the success rate needed for promotion.
	MinSuccessRateForPromotion float64

	// MaxFailureRate is the failure rate that triggers deprecation.
	MaxFailureRate float64
}

// DefaultLoopConfig returns the default feedback loop configuration.
func DefaultLoopConfig() *LoopConfig {
	return &LoopConfig{
		GradingBatchSize:           10,
		PromotionInterval:          15 * time.Minute,
		MinGradesForValidation:     3,
		MinUsesForPromotion:        5,
		MinPassRateForValidation:   0.90,
		MinSuccessRateForPromotion: 0.90,
		MaxFailureRate:             0.30,
	}
}

// Loop manages the continuous learning feedback loop.
type Loop struct {
	registry cognitive.Registry
	grader   Grader
	config   *LoopConfig
	log      *logging.Logger

	// Background worker state
	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewLoop creates a new feedback loop.
func NewLoop(registry cognitive.Registry, grader Grader, config *LoopConfig) *Loop {
	if config == nil {
		config = DefaultLoopConfig()
	}

	return &Loop{
		registry: registry,
		grader:   grader,
		config:   config,
		log:      logging.Global(),
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// RECORDING
// ═══════════════════════════════════════════════════════════════════════════════

// RecordSuccess records a successful template execution.
func (l *Loop) RecordSuccess(ctx context.Context, templateID string, userInput, renderedOutput string, latencyMs int) error {
	l.log.Debug("[Feedback] Recording success for template %s", templateID)

	// Record the usage
	logID, err := l.registry.RecordUsage(ctx, &cognitive.UsageLog{
		TemplateID:     templateID,
		UserInput:      userInput,
		RenderedOutput: renderedOutput,
		Success:        true,
		LatencyMs:      latencyMs,
		CreatedAt:      time.Now(),
	})
	if err != nil {
		return err
	}

	// Queue for grading if template is in probation
	template, err := l.registry.Get(ctx, templateID)
	if err != nil {
		l.log.Warn("[Feedback] Failed to get template for grading check: %v", err)
		return nil // Don't fail the success recording
	}

	if template.Status == cognitive.StatusProbation {
		l.log.Debug("[Feedback] Queueing probationary template %s for grading (usage %d)", templateID, logID)
		// Grading will be processed by background worker
	}

	return nil
}

// RecordFailure records a failed template execution.
func (l *Loop) RecordFailure(ctx context.Context, templateID string, userInput string, errorMsg string, latencyMs int) error {
	l.log.Debug("[Feedback] Recording failure for template %s: %s", templateID, errorMsg)

	// Record the failed usage
	_, err := l.registry.RecordUsage(ctx, &cognitive.UsageLog{
		TemplateID:     templateID,
		UserInput:      userInput,
		RenderedOutput: "",
		Success:        false,
		ErrorMessage:   errorMsg,
		LatencyMs:      latencyMs,
		CreatedAt:      time.Now(),
	})
	if err != nil {
		return err
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// GRADING
// ═══════════════════════════════════════════════════════════════════════════════

// TriggerGrading triggers immediate grading of a usage log.
func (l *Loop) TriggerGrading(ctx context.Context, templateID string, usageLogID int64, userInput, renderedOutput string) (*cognitive.GradingResult, error) {
	if l.grader == nil {
		return nil, nil // No grader configured
	}

	l.log.Debug("[Feedback] Triggering grading for template %s, usage %d", templateID, usageLogID)

	result, err := l.grader.GradeTemplateExecution(ctx, templateID, usageLogID, userInput, renderedOutput)
	if err != nil {
		l.log.Warn("[Feedback] Grading failed: %v", err)
		return nil, err
	}

	l.log.Info("[Feedback] Template %s graded: %s (delta: %+.2f)", templateID, result.Grade, result.ConfidenceDelta)

	return result, nil
}

// ProcessPendingGrades processes a batch of pending usage logs for grading.
func (l *Loop) ProcessPendingGrades(ctx context.Context) (int, error) {
	if l.grader == nil {
		return 0, nil // No grader configured
	}

	logs, err := l.registry.GetPendingGrades(ctx, l.config.GradingBatchSize)
	if err != nil {
		return 0, err
	}

	graded := 0
	for _, log := range logs {
		_, err := l.grader.GradeTemplateExecution(ctx, log.TemplateID, log.ID, log.UserInput, log.RenderedOutput)
		if err != nil {
			l.log.Warn("[Feedback] Failed to grade usage %d: %v", log.ID, err)
			continue
		}
		graded++
	}

	if graded > 0 {
		l.log.Info("[Feedback] Processed %d/%d pending grades", graded, len(logs))
	}

	return graded, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// PROMOTION CYCLE
// ═══════════════════════════════════════════════════════════════════════════════

// PromotionReport contains the results of a promotion cycle.
type PromotionReport struct {
	Timestamp      time.Time          `json:"timestamp"`
	Promoted       []PromotionAction  `json:"promoted"`
	Validated      []PromotionAction  `json:"validated"`
	Deprecated     []PromotionAction  `json:"deprecated"`
	Unchanged      int                `json:"unchanged"`
	Errors         []string           `json:"errors,omitempty"`
}

// PromotionAction records a template status change.
type PromotionAction struct {
	TemplateID    string                   `json:"template_id"`
	TemplateName  string                   `json:"template_name"`
	FromStatus    cognitive.TemplateStatus `json:"from_status"`
	ToStatus      cognitive.TemplateStatus `json:"to_status"`
	Reason        string                   `json:"reason"`
}

// RunPromotionCycle evaluates all templates and updates their status.
func (l *Loop) RunPromotionCycle(ctx context.Context) (*PromotionReport, error) {
	l.log.Info("[Feedback] Starting promotion cycle")
	start := time.Now()

	report := &PromotionReport{
		Timestamp:  start,
		Promoted:   make([]PromotionAction, 0),
		Validated:  make([]PromotionAction, 0),
		Deprecated: make([]PromotionAction, 0),
		Errors:     make([]string, 0),
	}

	// Process probationary templates for validation
	if err := l.processProbationaryTemplates(ctx, report); err != nil {
		report.Errors = append(report.Errors, "probation processing: "+err.Error())
	}

	// Process validated templates for promotion
	if err := l.processValidatedTemplates(ctx, report); err != nil {
		report.Errors = append(report.Errors, "promotion processing: "+err.Error())
	}

	// Check for templates to deprecate
	if err := l.checkForDeprecation(ctx, report); err != nil {
		report.Errors = append(report.Errors, "deprecation check: "+err.Error())
	}

	l.log.Info("[Feedback] Promotion cycle complete in %v: %d promoted, %d validated, %d deprecated",
		time.Since(start), len(report.Promoted), len(report.Validated), len(report.Deprecated))

	return report, nil
}

// processProbationaryTemplates checks probationary templates for validation.
func (l *Loop) processProbationaryTemplates(ctx context.Context, report *PromotionReport) error {
	templates, err := l.registry.ListByStatus(ctx, cognitive.StatusProbation)
	if err != nil {
		return err
	}

	for _, tmpl := range templates {
		metrics, err := l.registry.GetTemplateMetrics(ctx, tmpl.ID)
		if err != nil {
			report.Errors = append(report.Errors, "metrics for "+tmpl.ID+": "+err.Error())
			continue
		}

		// Check validation criteria: 3+ passing grades, 90%+ pass rate
		totalGrades := metrics.PassCount + metrics.FailCount + metrics.PartialCount
		if totalGrades < l.config.MinGradesForValidation {
			report.Unchanged++
			continue // Not enough grades yet
		}

		passRate := float64(metrics.PassCount) / float64(totalGrades)
		if passRate >= l.config.MinPassRateForValidation {
			// Promote to validated
			if err := l.registry.UpdateStatus(ctx, tmpl.ID, cognitive.StatusValidated); err != nil {
				report.Errors = append(report.Errors, "validate "+tmpl.ID+": "+err.Error())
				continue
			}

			report.Validated = append(report.Validated, PromotionAction{
				TemplateID:   tmpl.ID,
				TemplateName: tmpl.Name,
				FromStatus:   cognitive.StatusProbation,
				ToStatus:     cognitive.StatusValidated,
				Reason:       formatReason("pass rate %.1f%% >= %.1f%% with %d grades", passRate*100, l.config.MinPassRateForValidation*100, totalGrades),
			})

			l.log.Info("[Feedback] Template %s validated (pass rate: %.1f%%)", tmpl.ID, passRate*100)
		} else if 1-passRate >= l.config.MaxFailureRate {
			// Too many failures - deprecate
			if err := l.registry.UpdateStatus(ctx, tmpl.ID, cognitive.StatusDeprecated); err != nil {
				report.Errors = append(report.Errors, "deprecate "+tmpl.ID+": "+err.Error())
				continue
			}

			report.Deprecated = append(report.Deprecated, PromotionAction{
				TemplateID:   tmpl.ID,
				TemplateName: tmpl.Name,
				FromStatus:   cognitive.StatusProbation,
				ToStatus:     cognitive.StatusDeprecated,
				Reason:       formatReason("failure rate %.1f%% >= %.1f%%", (1-passRate)*100, l.config.MaxFailureRate*100),
			})

			l.log.Warn("[Feedback] Template %s deprecated (failure rate: %.1f%%)", tmpl.ID, (1-passRate)*100)
		} else {
			report.Unchanged++
		}
	}

	return nil
}

// processValidatedTemplates checks validated templates for promotion.
func (l *Loop) processValidatedTemplates(ctx context.Context, report *PromotionReport) error {
	templates, err := l.registry.ListByStatus(ctx, cognitive.StatusValidated)
	if err != nil {
		return err
	}

	for _, tmpl := range templates {
		metrics, err := l.registry.GetTemplateMetrics(ctx, tmpl.ID)
		if err != nil {
			report.Errors = append(report.Errors, "metrics for "+tmpl.ID+": "+err.Error())
			continue
		}

		// Check promotion criteria: 5+ uses, 90%+ success rate
		if metrics.UseCount < l.config.MinUsesForPromotion {
			report.Unchanged++
			continue // Not enough uses yet
		}

		if metrics.SuccessRate >= l.config.MinSuccessRateForPromotion {
			// Promote to promoted status
			if err := l.registry.UpdateStatus(ctx, tmpl.ID, cognitive.StatusPromoted); err != nil {
				report.Errors = append(report.Errors, "promote "+tmpl.ID+": "+err.Error())
				continue
			}

			report.Promoted = append(report.Promoted, PromotionAction{
				TemplateID:   tmpl.ID,
				TemplateName: tmpl.Name,
				FromStatus:   cognitive.StatusValidated,
				ToStatus:     cognitive.StatusPromoted,
				Reason:       formatReason("success rate %.1f%% >= %.1f%% with %d uses", metrics.SuccessRate*100, l.config.MinSuccessRateForPromotion*100, metrics.UseCount),
			})

			l.log.Info("[Feedback] Template %s promoted (success rate: %.1f%%)", tmpl.ID, metrics.SuccessRate*100)
		} else {
			report.Unchanged++
		}
	}

	return nil
}

// checkForDeprecation checks all active templates for deprecation.
func (l *Loop) checkForDeprecation(ctx context.Context, report *PromotionReport) error {
	// Check validated and promoted templates
	for _, status := range []cognitive.TemplateStatus{cognitive.StatusValidated, cognitive.StatusPromoted} {
		templates, err := l.registry.ListByStatus(ctx, status)
		if err != nil {
			return err
		}

		for _, tmpl := range templates {
			metrics, err := l.registry.GetTemplateMetrics(ctx, tmpl.ID)
			if err != nil {
				continue
			}

			// Check for high failure rate
			if metrics.UseCount >= 5 && metrics.SuccessRate < (1-l.config.MaxFailureRate) {
				if err := l.registry.UpdateStatus(ctx, tmpl.ID, cognitive.StatusDeprecated); err != nil {
					report.Errors = append(report.Errors, "deprecate "+tmpl.ID+": "+err.Error())
					continue
				}

				report.Deprecated = append(report.Deprecated, PromotionAction{
					TemplateID:   tmpl.ID,
					TemplateName: tmpl.Name,
					FromStatus:   status,
					ToStatus:     cognitive.StatusDeprecated,
					Reason:       formatReason("success rate %.1f%% dropped below %.1f%%", metrics.SuccessRate*100, (1-l.config.MaxFailureRate)*100),
				})

				l.log.Warn("[Feedback] Template %s deprecated (success rate dropped: %.1f%%)", tmpl.ID, metrics.SuccessRate*100)
			}
		}
	}

	return nil
}

// formatReason formats a promotion reason string.
func formatReason(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// ═══════════════════════════════════════════════════════════════════════════════
// BACKGROUND WORKER
// ═══════════════════════════════════════════════════════════════════════════════

// Start begins the background feedback loop worker.
func (l *Loop) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return nil // Already running
	}

	l.running = true
	l.stopCh = make(chan struct{})
	l.doneCh = make(chan struct{})

	go l.runWorker(ctx)

	l.log.Info("[Feedback] Background worker started (promotion interval: %v)", l.config.PromotionInterval)
	return nil
}

// Stop stops the background feedback loop worker.
func (l *Loop) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return
	}

	close(l.stopCh)
	<-l.doneCh

	l.running = false
	l.log.Info("[Feedback] Background worker stopped")
}

// runWorker is the main background worker loop.
func (l *Loop) runWorker(ctx context.Context) {
	defer close(l.doneCh)

	promotionTicker := time.NewTicker(l.config.PromotionInterval)
	gradingTicker := time.NewTicker(30 * time.Second) // Check for pending grades every 30s
	defer promotionTicker.Stop()
	defer gradingTicker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ctx.Done():
			return
		case <-gradingTicker.C:
			if _, err := l.ProcessPendingGrades(ctx); err != nil {
				l.log.Warn("[Feedback] Grading cycle failed: %v", err)
			}
		case <-promotionTicker.C:
			if _, err := l.RunPromotionCycle(ctx); err != nil {
				l.log.Warn("[Feedback] Promotion cycle failed: %v", err)
			}
		}
	}
}

// Running returns whether the background worker is running.
func (l *Loop) Running() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}
