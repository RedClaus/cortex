package persona_test

import (
	"regexp"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/persona"
)

func TestNewModeManager(t *testing.T) {
	mm := persona.NewModeManager()

	current := mm.Current()
	if current.Type != persona.ModeNormal {
		t.Errorf("expected initial mode 'normal', got %q", current.Type)
	}
}

func TestProcessInputDebugging(t *testing.T) {
	mm := persona.NewModeManager()

	triggers := []string{
		"I need to debug this error",
		"Can you help me fix this bug?",
		"The tests are failing",
		"Got a crash in production",
		"There's an exception being thrown",
	}

	for _, trigger := range triggers {
		mm.Reset()
		changed := mm.ProcessInput(trigger)
		if !changed {
			t.Errorf("expected mode change for %q", trigger)
		}
		if mm.Current().Type != persona.ModeDebugging {
			t.Errorf("expected debugging mode for %q, got %q", trigger, mm.Current().Type)
		}
	}
}

func TestProcessInputTeaching(t *testing.T) {
	mm := persona.NewModeManager()

	triggers := []string{
		"Can you explain how this works?",
		"Teach me about goroutines",
		"Help me understand channels",
		"What is a pointer?",
		"Why does this happen?",
		"I want to learn about interfaces",
	}

	for _, trigger := range triggers {
		mm.Reset()
		changed := mm.ProcessInput(trigger)
		if !changed {
			t.Errorf("expected mode change for %q", trigger)
		}
		if mm.Current().Type != persona.ModeTeaching {
			t.Errorf("expected teaching mode for %q, got %q", trigger, mm.Current().Type)
		}
	}
}

func TestProcessInputPairProgramming(t *testing.T) {
	mm := persona.NewModeManager()

	triggers := []string{
		"Let's build a REST API",
		"Work together on this feature",
		"Pair with me on this",
		"Code with me",
		"Help me implement a cache",
	}

	for _, trigger := range triggers {
		mm.Reset()
		changed := mm.ProcessInput(trigger)
		if !changed {
			t.Errorf("expected mode change for %q", trigger)
		}
		if mm.Current().Type != persona.ModePair {
			t.Errorf("expected pair mode for %q, got %q", trigger, mm.Current().Type)
		}
	}
}

func TestProcessInputCodeReview(t *testing.T) {
	mm := persona.NewModeManager()

	triggers := []string{
		"Can you review this code?",
		"Please do a code review",
		"Check this code for issues",
		"Review my implementation",
		"Critique this solution",
	}

	for _, trigger := range triggers {
		mm.Reset()
		changed := mm.ProcessInput(trigger)
		if !changed {
			t.Errorf("expected mode change for %q", trigger)
		}
		if mm.Current().Type != persona.ModeReview {
			t.Errorf("expected review mode for %q, got %q", trigger, mm.Current().Type)
		}
	}
}

func TestProcessInputReset(t *testing.T) {
	mm := persona.NewModeManager()

	// Enter debugging mode first
	mm.ProcessInput("debug this error")
	if mm.Current().Type != persona.ModeDebugging {
		t.Fatalf("failed to enter debugging mode")
	}

	// Test reset triggers
	resetTriggers := []string{
		"reset mode",
		"normal mode",
		"back to normal",
		"exit mode",
	}

	for _, trigger := range resetTriggers {
		mm.SetMode(persona.ModeDebugging, "test")
		changed := mm.ProcessInput(trigger)
		if !changed {
			t.Errorf("expected mode change for %q", trigger)
		}
		if mm.Current().Type != persona.ModeNormal {
			t.Errorf("expected normal mode after %q, got %q", trigger, mm.Current().Type)
		}
	}
}

func TestNoModeChangeOnNonTrigger(t *testing.T) {
	mm := persona.NewModeManager()

	// These should not trigger mode changes
	// Note: avoid words like "what is" (teaching), "fix" (debug), "build" (pair)
	nonTriggers := []string{
		"Hello",
		"Run this command",
		"Show me the file",
		"List all files",
		"ping localhost",
	}

	for _, input := range nonTriggers {
		mm.Reset()
		changed := mm.ProcessInput(input)
		if changed {
			t.Errorf("unexpected mode change for %q", input)
		}
		if mm.Current().Type != persona.ModeNormal {
			t.Errorf("mode should remain normal for %q, got %q", input, mm.Current().Type)
		}
	}
}

func TestModeFromModeRequirement(t *testing.T) {
	mm := persona.NewModeManager()

	// Debugging-specific exit triggers should only work in debugging mode
	mm.SetMode(persona.ModeDebugging, "test")
	changed := mm.ProcessInput("that fixed it!")
	if !changed {
		t.Error("expected mode change from 'that fixed it'")
	}
	if mm.Current().Type != persona.ModeNormal {
		t.Errorf("expected normal mode, got %q", mm.Current().Type)
	}

	// Same trigger in normal mode should not cause change
	mm.Reset()
	changed = mm.ProcessInput("that fixed it!")
	if changed {
		t.Error("should not change mode when not in debugging")
	}
}

func TestSetModeExplicit(t *testing.T) {
	mm := persona.NewModeManager()

	mm.SetMode(persona.ModeTeaching, "explicit test")

	if mm.Current().Type != persona.ModeTeaching {
		t.Errorf("expected teaching mode, got %q", mm.Current().Type)
	}
	if mm.Current().Trigger != "explicit test" {
		t.Errorf("expected trigger 'explicit test', got %q", mm.Current().Trigger)
	}
}

func TestModeHistory(t *testing.T) {
	mm := persona.NewModeManager()

	// Make some transitions
	mm.ProcessInput("debug this error")
	mm.ProcessInput("back to normal")
	mm.ProcessInput("explain this concept")

	history := mm.History()

	if len(history) != 3 {
		t.Errorf("expected 3 transitions, got %d", len(history))
	}

	// Check first transition
	if history[0].From != persona.ModeNormal {
		t.Errorf("first transition should be from normal")
	}
	if history[0].To != persona.ModeDebugging {
		t.Errorf("first transition should be to debugging")
	}
}

func TestModeAdjustments(t *testing.T) {
	presets := persona.ModePresets

	// Check debugging adjustments
	debug := presets[persona.ModeDebugging]
	if debug.Verbosity <= 0.5 {
		t.Error("debugging should have higher verbosity")
	}
	if debug.ThinkingDepth < 0.7 {
		t.Error("debugging should have high thinking depth")
	}

	// Check teaching adjustments
	teaching := presets[persona.ModeTeaching]
	if teaching.Verbosity < 0.8 {
		t.Error("teaching should be very verbose")
	}
	if teaching.CodeVsExplain > 0.3 {
		t.Error("teaching should favor explanation over code")
	}

	// Check pair adjustments
	pair := presets[persona.ModePair]
	if pair.Verbosity > 0.5 {
		t.Error("pair should be concise")
	}
	if pair.CodeVsExplain < 0.6 {
		t.Error("pair should favor code")
	}

	// Check review adjustments
	review := presets[persona.ModeReview]
	if review.ThinkingDepth < 0.8 {
		t.Error("review should have deep thinking")
	}
}

func TestModeInstructions(t *testing.T) {
	modes := []struct {
		mode     persona.ModeType
		contains []string
	}{
		{persona.ModeDebugging, []string{"DEBUGGING", "clarifying", "error", "root cause"}},
		{persona.ModeTeaching, []string{"TEACHING", "explain", "fundamentals", "understanding"}},
		{persona.ModePair, []string{"PAIR PROGRAMMING", "collaboratively", "reasoning"}},
		{persona.ModeReview, []string{"CODE REVIEW", "quality", "correctness", "bugs"}},
	}

	for _, tt := range modes {
		m := &persona.BehavioralMode{
			Type:      tt.mode,
			EnteredAt: time.Now(),
		}

		instructions := m.GetInstructions()
		for _, expected := range tt.contains {
			if !containsCaseInsensitive(instructions, expected) {
				t.Errorf("mode %s instructions should contain %q", tt.mode, expected)
			}
		}
	}
}

func containsCaseInsensitive(s, substr string) bool {
	return regexp.MustCompile(`(?i)` + regexp.QuoteMeta(substr)).MatchString(s)
}

func TestDuration(t *testing.T) {
	mm := persona.NewModeManager()

	// Mode starts with current time
	time.Sleep(10 * time.Millisecond)

	duration := mm.Duration()
	if duration < 10*time.Millisecond {
		t.Errorf("duration should be at least 10ms, got %v", duration)
	}
}

func TestIsInMode(t *testing.T) {
	mm := persona.NewModeManager()

	if !mm.IsInMode(persona.ModeNormal) {
		t.Error("should start in normal mode")
	}

	mm.SetMode(persona.ModeDebugging, "test")

	if !mm.IsInMode(persona.ModeDebugging) {
		t.Error("should be in debugging mode")
	}
	if mm.IsInMode(persona.ModeNormal) {
		t.Error("should not be in normal mode")
	}
}

func TestAddCustomTransition(t *testing.T) {
	mm := persona.NewModeManager()

	// Add custom high-priority transition
	mm.AddTransition(persona.TransitionRule{
		Name:     "custom",
		Pattern:  regexp.MustCompile(`(?i)\bspecial trigger\b`),
		Keywords: []string{"special trigger"},
		FromMode: "",
		ToMode:   persona.ModeTeaching,
		Priority: 200, // Higher than all defaults
	})

	changed := mm.ProcessInput("special trigger please")
	if !changed {
		t.Error("custom transition should trigger")
	}
	if mm.Current().Type != persona.ModeTeaching {
		t.Errorf("should be in teaching mode, got %q", mm.Current().Type)
	}
}

func TestConcurrentModeManager(t *testing.T) {
	mm := persona.NewModeManager()

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				mm.ProcessInput("debug this")
				mm.Current()
				mm.IsInMode(persona.ModeDebugging)
				mm.ProcessInput("back to normal")
				mm.Duration()
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestModeString(t *testing.T) {
	m := &persona.BehavioralMode{
		Type: persona.ModeDebugging,
		Adjustments: persona.ModeAdjustments{
			Verbosity:     0.7,
			ThinkingDepth: 0.8,
		},
		EnteredAt: time.Now(),
		Trigger:   "test",
	}

	str := m.String()
	if str == "" {
		t.Error("String() should return non-empty")
	}
}
