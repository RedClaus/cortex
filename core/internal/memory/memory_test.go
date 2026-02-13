package memory

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/normanking/cortex/internal/persona"
)

// testDB creates a new in-memory SQLite database for testing.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	return db
}

// TestNewUserMemory verifies default user memory creation.
func TestNewUserMemory(t *testing.T) {
	mem := NewUserMemory()

	if mem.Preferences == nil {
		t.Error("Preferences should be initialized")
	}
	if mem.CustomFacts == nil {
		t.Error("CustomFacts should be initialized")
	}
	if mem.LastUpdated.IsZero() {
		t.Error("LastUpdated should be set")
	}
}

// TestUserMemoryAddFact verifies fact addition and deduplication.
func TestUserMemoryAddFact(t *testing.T) {
	mem := NewUserMemory()

	// Add first fact
	mem.AddFact("User prefers tabs over spaces", "user_stated")
	if len(mem.CustomFacts) != 1 {
		t.Errorf("expected 1 fact, got %d", len(mem.CustomFacts))
	}

	// Add duplicate fact - should be ignored
	mem.AddFact("User prefers tabs over spaces", "user_stated")
	if len(mem.CustomFacts) != 1 {
		t.Errorf("duplicate fact should be ignored, got %d facts", len(mem.CustomFacts))
	}

	// Add different fact
	mem.AddFact("User is working on a Go project", "llm_inferred")
	if len(mem.CustomFacts) != 2 {
		t.Errorf("expected 2 facts, got %d", len(mem.CustomFacts))
	}
}

// TestUserMemoryPreferences verifies preference helpers.
func TestUserMemoryPreferences(t *testing.T) {
	mem := NewUserMemory()
	mem.Preferences = []UserPreference{
		{Category: "code_style", Preference: "tabs", Confidence: 0.9},
		{Category: "output_format", Preference: "markdown", Confidence: 0.8},
	}

	// Test HasPreference
	if !mem.HasPreference("code_style") {
		t.Error("should have code_style preference")
	}
	if mem.HasPreference("nonexistent") {
		t.Error("should not have nonexistent preference")
	}

	// Test GetPreference
	if got := mem.GetPreference("code_style"); got != "tabs" {
		t.Errorf("expected 'tabs', got '%s'", got)
	}
	if got := mem.GetPreference("nonexistent"); got != "" {
		t.Errorf("expected empty string, got '%s'", got)
	}
}

// TestUserMemoryTokenEstimate verifies token estimation.
func TestUserMemoryTokenEstimate(t *testing.T) {
	mem := NewUserMemory()
	mem.Name = "John"
	mem.OS = "macOS"
	mem.Shell = "zsh"

	tokens := mem.TokenEstimate()
	if tokens <= 0 {
		t.Error("token estimate should be positive")
	}

	// Add more content, estimate should increase
	mem.AddFact("User is a senior developer", "user_stated")
	newTokens := mem.TokenEstimate()
	if newTokens <= tokens {
		t.Error("token estimate should increase with more content")
	}
}

// TestCoreMemoryStore_UserMemory tests user memory CRUD operations.
func TestCoreMemoryStore_UserMemory(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store, err := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	userID := "test-user-1"

	// Test initial state (empty)
	mem, err := store.GetUserMemory(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get user memory: %v", err)
	}
	if mem.Name != "" {
		t.Error("new user memory should have empty name")
	}

	// Update a field
	err = store.UpdateUserField(ctx, userID, "name", "Alice", "test")
	if err != nil {
		t.Fatalf("failed to update field: %v", err)
	}

	// Verify update
	mem, _ = store.GetUserMemory(ctx, userID)
	if mem.Name != "Alice" {
		t.Errorf("expected name 'Alice', got '%s'", mem.Name)
	}

	// Update multiple fields
	store.UpdateUserField(ctx, userID, "os", "macOS", "test")
	store.UpdateUserField(ctx, userID, "shell", "zsh", "test")
	store.UpdateUserField(ctx, userID, "prefers_concise", 1, "test")

	mem, _ = store.GetUserMemory(ctx, userID)
	if mem.OS != "macOS" || mem.Shell != "zsh" || !mem.PrefersConcise {
		t.Error("multiple field updates failed")
	}

	// Test invalid field update
	err = store.UpdateUserField(ctx, userID, "invalid_field", "value", "test")
	if err == nil {
		t.Error("should reject invalid field")
	}
}

// TestCoreMemoryStore_UserFacts tests custom fact management.
func TestCoreMemoryStore_UserFacts(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	config := DefaultCoreMemoryConfig()
	config.MaxUserFacts = 3 // Low limit for testing

	store, err := NewCoreMemoryStore(db, config)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	userID := "test-user-facts"

	// Add facts
	store.AppendUserFact(ctx, userID, "Fact 1", "test")
	store.AppendUserFact(ctx, userID, "Fact 2", "test")
	store.AppendUserFact(ctx, userID, "Fact 3", "test")

	mem, _ := store.GetUserMemory(ctx, userID)
	if len(mem.CustomFacts) != 3 {
		t.Errorf("expected 3 facts, got %d", len(mem.CustomFacts))
	}

	// Add one more - should remove oldest
	store.AppendUserFact(ctx, userID, "Fact 4", "test")

	mem, _ = store.GetUserMemory(ctx, userID)
	if len(mem.CustomFacts) != 3 {
		t.Errorf("should maintain max 3 facts, got %d", len(mem.CustomFacts))
	}

	// Verify oldest was removed
	for _, f := range mem.CustomFacts {
		if f.Fact == "Fact 1" {
			t.Error("oldest fact should have been removed")
		}
	}

	// Test duplicate prevention
	store.AppendUserFact(ctx, userID, "Fact 4", "test") // Already exists
	mem, _ = store.GetUserMemory(ctx, userID)
	if len(mem.CustomFacts) != 3 {
		t.Error("duplicate should not be added")
	}
}

// TestCoreMemoryStore_UserPreferences tests preference management.
func TestCoreMemoryStore_UserPreferences(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	config := DefaultCoreMemoryConfig()
	config.MaxPreferences = 2 // Low limit for testing

	store, err := NewCoreMemoryStore(db, config)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	userID := "test-user-prefs"

	// Add preference
	pref1 := UserPreference{
		Category:   "code_style",
		Preference: "tabs",
		Confidence: 0.9,
		LearnedFrom: "explicit",
		LearnedAt:  time.Now(),
	}
	err = store.AppendUserPreference(ctx, userID, pref1)
	if err != nil {
		t.Fatalf("failed to add preference: %v", err)
	}

	// Update existing preference (same category)
	pref1Updated := UserPreference{
		Category:   "code_style",
		Preference: "spaces", // Changed!
		Confidence: 0.95,
		LearnedFrom: "correction",
		LearnedAt:  time.Now(),
	}
	store.AppendUserPreference(ctx, userID, pref1Updated)

	mem, _ := store.GetUserMemory(ctx, userID)
	if len(mem.Preferences) != 1 {
		t.Errorf("should have 1 preference (updated), got %d", len(mem.Preferences))
	}
	if mem.Preferences[0].Preference != "spaces" {
		t.Error("preference should be updated to 'spaces'")
	}

	// Add more preferences to test limit
	pref2 := UserPreference{Category: "output", Preference: "markdown", Confidence: 0.8, LearnedAt: time.Now()}
	pref3 := UserPreference{Category: "verbosity", Preference: "concise", Confidence: 0.7, LearnedAt: time.Now()}

	store.AppendUserPreference(ctx, userID, pref2)
	store.AppendUserPreference(ctx, userID, pref3)

	mem, _ = store.GetUserMemory(ctx, userID)
	if len(mem.Preferences) != 2 {
		t.Errorf("should maintain max 2 preferences, got %d", len(mem.Preferences))
	}

	// Lowest confidence should have been removed (pref3 with 0.7)
	for _, p := range mem.Preferences {
		if p.Confidence < 0.8 {
			t.Errorf("lowest confidence preference should have been removed, but found confidence %.2f", p.Confidence)
		}
	}
}

// TestCoreMemoryStore_ProjectMemory tests project memory operations.
func TestCoreMemoryStore_ProjectMemory(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store, err := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project-1"

	// Initial state (empty)
	mem, err := store.GetProjectMemory(ctx, projectID)
	if err != nil {
		t.Fatalf("failed to get project memory: %v", err)
	}
	if mem.Name != "" {
		t.Error("new project memory should have empty name")
	}

	// Save project memory
	projectMem := &ProjectMemory{
		Name:        "Cortex",
		Path:        "/home/user/cortex",
		Type:        "go",
		TechStack:   []string{"go", "sqlite", "cobra"},
		Conventions: []string{"gofmt", "golangci-lint"},
		GitBranch:   "main",
		Metadata: map[string]string{
			"go_version": "1.21",
		},
	}
	err = store.SaveProjectMemory(ctx, projectID, projectMem)
	if err != nil {
		t.Fatalf("failed to save project memory: %v", err)
	}

	// Retrieve and verify
	mem, _ = store.GetProjectMemory(ctx, projectID)
	if mem.Name != "Cortex" {
		t.Errorf("expected name 'Cortex', got '%s'", mem.Name)
	}
	if len(mem.TechStack) != 3 {
		t.Errorf("expected 3 tech stack items, got %d", len(mem.TechStack))
	}
	if mem.Metadata["go_version"] != "1.21" {
		t.Error("metadata not saved correctly")
	}

	// Update project memory
	projectMem.GitBranch = "feature/memory"
	projectMem.TechStack = append(projectMem.TechStack, "bubbletea")
	store.SaveProjectMemory(ctx, projectID, projectMem)

	mem, _ = store.GetProjectMemory(ctx, projectID)
	if mem.GitBranch != "feature/memory" {
		t.Error("project update failed")
	}
	if len(mem.TechStack) != 4 {
		t.Errorf("expected 4 tech stack items, got %d", len(mem.TechStack))
	}
}

// TestCoreMemoryStore_Changelog tests the audit trail.
func TestCoreMemoryStore_Changelog(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store, err := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	userID := "test-user-changelog"

	// Make some changes
	store.UpdateUserField(ctx, userID, "name", "Alice", "initial")
	store.UpdateUserField(ctx, userID, "name", "Bob", "user_correction")
	store.UpdateUserField(ctx, userID, "os", "Linux", "detected")

	// Get changelog
	changes, err := store.GetChangelog(ctx, userID, 10)
	if err != nil {
		t.Fatalf("failed to get changelog: %v", err)
	}

	if len(changes) != 3 {
		t.Errorf("expected 3 changes, got %d", len(changes))
	}

	// Verify all expected fields are present (order may vary due to timestamp resolution)
	fields := make(map[string]bool)
	for _, c := range changes {
		fields[c.Field] = true
	}
	if !fields["name"] || !fields["os"] {
		t.Error("expected both 'name' and 'os' fields in changelog")
	}

	// Check name change recorded old value
	var nameChange *MemoryChange
	for i := range changes {
		if changes[i].Field == "name" && changes[i].NewValue == "Bob" {
			nameChange = &changes[i]
			break
		}
	}
	if nameChange == nil {
		t.Fatal("name change to Bob not found")
	}
	if nameChange.OldValue != "Alice" {
		t.Errorf("expected old value 'Alice', got '%s'", nameChange.OldValue)
	}
}

// TestContextBuilder_FastLane tests fast lane context generation.
func TestContextBuilder_FastLane(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store, _ := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	builder := NewContextBuilder(store, DefaultContextBuilderConfig())

	ctx := context.Background()
	userID := "test-user-ctx"

	// Setup user with preferences
	store.UpdateUserField(ctx, userID, "os", "macOS", "test")
	store.UpdateUserField(ctx, userID, "shell", "zsh", "test")
	store.UpdateUserField(ctx, userID, "prefers_concise", 1, "test")

	// Create persona
	personaCore := persona.NewPersonaCore()

	// Create project
	project := &ProjectMemory{
		Name:        "TestProject",
		TechStack:   []string{"go", "sqlite"},
		Conventions: []string{"use gofmt"},
	}

	// Build fast lane context
	laneCtx, err := builder.BuildForLane(ctx, LaneFast, userID, personaCore, project, nil)
	if err != nil {
		t.Fatalf("failed to build context: %v", err)
	}

	// Verify it's a fast lane context
	if laneCtx.Lane != LaneFast {
		t.Error("should be fast lane")
	}

	// Should contain persona name
	if !strings.Contains(laneCtx.SystemPrompt, "Cortex") {
		t.Error("should contain persona name")
	}

	// Should contain user preferences
	if !strings.Contains(laneCtx.SystemPrompt, "concise") {
		t.Error("should mention user preference for concise")
	}

	// Should contain tech stack
	if !strings.Contains(laneCtx.SystemPrompt, "go, sqlite") {
		t.Error("should contain tech stack")
	}

	// Should NOT contain conventions (fast lane minimizes)
	if strings.Contains(laneCtx.SystemPrompt, "use gofmt") {
		t.Error("fast lane should NOT contain conventions")
	}

	// Should NOT contain memory tool instructions
	if strings.Contains(laneCtx.SystemPrompt, "recall_memory_search") {
		t.Error("fast lane should NOT contain memory tool instructions")
	}

	// Should be under token budget
	if laneCtx.TokenCount > 400 {
		t.Errorf("fast lane should be under 400 tokens, got %d", laneCtx.TokenCount)
	}

	// Should contain passive retrieval placeholder
	if !strings.Contains(laneCtx.SystemPrompt, "{{PASSIVE_RETRIEVAL}}") {
		t.Error("should contain passive retrieval placeholder")
	}
}

// TestContextBuilder_SmartLane tests smart lane context generation.
func TestContextBuilder_SmartLane(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store, _ := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	builder := NewContextBuilder(store, DefaultContextBuilderConfig())

	ctx := context.Background()
	userID := "test-user-smart"

	// Setup rich user profile
	store.UpdateUserField(ctx, userID, "name", "Alice", "test")
	store.UpdateUserField(ctx, userID, "role", "Senior Developer", "test")
	store.UpdateUserField(ctx, userID, "os", "Linux", "test")
	store.AppendUserFact(ctx, userID, "Works on backend services", "llm_inferred")
	store.AppendUserPreference(ctx, userID, UserPreference{
		Category: "code_style", Preference: "tabs over spaces", Confidence: 0.9, LearnedAt: time.Now(),
	})

	personaCore := persona.NewPersonaCore()
	project := &ProjectMemory{
		Name:        "Backend API",
		TechStack:   []string{"go", "postgresql", "redis"},
		Conventions: []string{"use database transactions", "log all errors"},
	}

	// Build smart lane context
	laneCtx, err := builder.BuildForLane(ctx, LaneSmart, userID, personaCore, project, nil)
	if err != nil {
		t.Fatalf("failed to build context: %v", err)
	}

	// Verify it's smart lane
	if laneCtx.Lane != LaneSmart {
		t.Error("should be smart lane")
	}

	// Should contain full user memory
	if !strings.Contains(laneCtx.SystemPrompt, "Alice") {
		t.Error("should contain user name")
	}
	if !strings.Contains(laneCtx.SystemPrompt, "Senior Developer") {
		t.Error("should contain user role")
	}
	if !strings.Contains(laneCtx.SystemPrompt, "Works on backend services") {
		t.Error("should contain user facts")
	}
	if !strings.Contains(laneCtx.SystemPrompt, "tabs over spaces") {
		t.Error("should contain user preferences")
	}

	// Should contain conventions (smart lane includes full context)
	if !strings.Contains(laneCtx.SystemPrompt, "use database transactions") {
		t.Error("smart lane should contain conventions")
	}

	// Should contain memory tool instructions
	if !strings.Contains(laneCtx.SystemPrompt, "recall_memory_search") {
		t.Error("smart lane should contain memory tool instructions")
	}
	if !strings.Contains(laneCtx.SystemPrompt, "archival_memory_search") {
		t.Error("smart lane should contain archival memory tool")
	}

	// Smart lane context will be larger but should still be reasonable
	if laneCtx.TokenCount > 3000 {
		t.Errorf("smart lane context too large: %d tokens", laneCtx.TokenCount)
	}
}

// TestContextBuilder_PassiveResultsInjection tests passive retrieval integration.
func TestContextBuilder_PassiveResultsInjection(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store, _ := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	builder := NewContextBuilder(store, DefaultContextBuilderConfig())

	ctx := context.Background()
	personaCore := persona.NewPersonaCore()

	// Build fast lane context
	laneCtx, _ := builder.BuildForLane(ctx, LaneFast, "test-user", personaCore, nil, nil)

	// Inject empty results - should remove placeholder
	builder.InjectPassiveResults(laneCtx, nil)
	if strings.Contains(laneCtx.SystemPrompt, "{{PASSIVE_RETRIEVAL}}") {
		t.Error("placeholder should be removed when no results")
	}

	// Build new context and inject actual results
	laneCtx, _ = builder.BuildForLane(ctx, LaneFast, "test-user", personaCore, nil, nil)
	results := []PassiveResult{
		{ID: "1", Summary: "Use kubectl rollout restart for ingress issues", Confidence: 0.87},
		{ID: "2", Summary: "Check pod logs with kubectl logs -f", Confidence: 0.75},
	}
	builder.InjectPassiveResults(laneCtx, results)

	// Should contain results
	if !strings.Contains(laneCtx.SystemPrompt, "kubectl rollout restart") {
		t.Error("should contain first result")
	}
	if !strings.Contains(laneCtx.SystemPrompt, "kubectl logs -f") {
		t.Error("should contain second result")
	}
	if !strings.Contains(laneCtx.SystemPrompt, "<relevant_knowledge>") {
		t.Error("should wrap in relevant_knowledge tags")
	}

	// Should have stored results
	if len(laneCtx.PassiveResults) != 2 {
		t.Errorf("expected 2 passive results, got %d", len(laneCtx.PassiveResults))
	}
}

// TestContextBuilder_BehavioralMode tests mode integration.
func TestContextBuilder_BehavioralMode(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store, _ := NewCoreMemoryStore(db, DefaultCoreMemoryConfig())
	builder := NewContextBuilder(store, DefaultContextBuilderConfig())

	ctx := context.Background()
	personaCore := persona.NewPersonaCore()

	// Create debugging mode
	debugMode := &persona.BehavioralMode{
		Type: persona.ModeDebugging,
	}

	// Build smart lane context with mode
	laneCtx, _ := builder.BuildForLane(ctx, LaneSmart, "test-user", personaCore, nil, debugMode)

	// Should contain debugging mode instructions
	if !strings.Contains(laneCtx.SystemPrompt, "DEBUGGING") {
		t.Error("should contain debugging mode reference")
	}
}

// TestTokenEstimate verifies token estimation accuracy.
func TestTokenEstimate(t *testing.T) {
	cases := []struct {
		text     string
		minTokens int
		maxTokens int
	}{
		{"hello", 1, 3},
		{"hello world", 2, 5},
		{strings.Repeat("a", 100), 20, 30},
		{strings.Repeat("word ", 100), 100, 150},
	}

	for _, tc := range cases {
		tokens := estimateTokens(tc.text)
		if tokens < tc.minTokens || tokens > tc.maxTokens {
			t.Errorf("estimateTokens(%q) = %d, expected between %d and %d",
				tc.text[:min(20, len(tc.text))], tokens, tc.minTokens, tc.maxTokens)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
