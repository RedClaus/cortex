package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// skipIfNoGit skips the test if git is not available
func skipIfNoGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

// createTestRepo creates a temporary git repository for testing
func createTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	cmd.Run()

	return dir
}

// createTestFile creates a file in the given directory
func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return path
}

func TestGitTool_Name(t *testing.T) {
	g := NewGitTool(nil)
	if g.Name() != "git" {
		t.Errorf("expected name 'git', got %s", g.Name())
	}
}

func TestGitTool_Category(t *testing.T) {
	g := NewGitTool(nil)
	if g.Category() != CategoryGit {
		t.Errorf("expected category CategoryGit, got %v", g.Category())
	}
}

func TestGitTool_RiskLevel(t *testing.T) {
	g := NewGitTool(nil)
	if g.RiskLevel() != RiskMedium {
		t.Errorf("expected risk level RiskMedium, got %v", g.RiskLevel())
	}
}

func TestGitTool_OperationRiskLevel(t *testing.T) {
	g := NewGitTool(nil)

	tests := []struct {
		operation string
		args      map[string]any
		expected  RiskLevel
	}{
		{GitOpStatus, nil, RiskLow},
		{GitOpDiff, nil, RiskLow},
		{GitOpLog, nil, RiskLow},
		{GitOpAdd, nil, RiskMedium},
		{GitOpCommit, nil, RiskMedium},
		{GitOpPush, nil, RiskMedium},
		{GitOpPush, map[string]any{"force": true}, RiskHigh},
		{GitOpPull, nil, RiskMedium},
		{GitOpClone, nil, RiskMedium},
	}

	for _, tt := range tests {
		t.Run(tt.operation, func(t *testing.T) {
			if got := g.OperationRiskLevel(tt.operation, tt.args); got != tt.expected {
				t.Errorf("OperationRiskLevel(%s) = %v, want %v", tt.operation, got, tt.expected)
			}
		})
	}
}

func TestGitTool_Validate(t *testing.T) {
	g := NewGitTool(nil)

	tests := []struct {
		name    string
		input   *ToolInput
		wantErr error
	}{
		{
			name:    "empty command",
			input:   &ToolInput{},
			wantErr: ErrMissingOperation,
		},
		{
			name:    "status - valid",
			input:   &ToolInput{Command: GitOpStatus},
			wantErr: nil,
		},
		{
			name:    "add - missing files",
			input:   &ToolInput{Command: GitOpAdd},
			wantErr: ErrMissingFiles,
		},
		{
			name:    "add - with files",
			input:   &ToolInput{Command: GitOpAdd, Args: map[string]any{"files": "test.txt"}},
			wantErr: nil,
		},
		{
			name:    "commit - missing message",
			input:   &ToolInput{Command: GitOpCommit},
			wantErr: ErrMissingMessage,
		},
		{
			name:    "commit - with message",
			input:   &ToolInput{Command: GitOpCommit, Args: map[string]any{"message": "test"}},
			wantErr: nil,
		},
		{
			name:    "clone - missing url",
			input:   &ToolInput{Command: GitOpClone},
			wantErr: ErrMissingRepoURL,
		},
		{
			name:    "clone - with url",
			input:   &ToolInput{Command: GitOpClone, Args: map[string]any{"url": "https://github.com/test/test"}},
			wantErr: nil,
		},
		{
			name:    "checkout - missing ref",
			input:   &ToolInput{Command: GitOpCheckout},
			wantErr: ErrMissingBranchName,
		},
		{
			name:    "checkout - with ref",
			input:   &ToolInput{Command: GitOpCheckout, Args: map[string]any{"ref": "main"}},
			wantErr: nil,
		},
		{
			name:    "invalid operation",
			input:   &ToolInput{Command: "invalid"},
			wantErr: ErrInvalidOperation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := g.Validate(tt.input)
			if tt.wantErr == nil && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
			if tt.wantErr != nil && err == nil {
				t.Errorf("Validate() expected error %v, got nil", tt.wantErr)
			}
			if tt.wantErr != nil && err != nil && !strings.Contains(err.Error(), tt.wantErr.Error()) {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestGitTool_Validate_PushConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     *GitConfig
		input      *ToolInput
		wantErr    error
	}{
		{
			name:       "push allowed by default",
			config:     DefaultGitConfig(),
			input:      &ToolInput{Command: GitOpPush},
			wantErr:    nil,
		},
		{
			name:       "push not allowed",
			config:     &GitConfig{AllowPush: false},
			input:      &ToolInput{Command: GitOpPush},
			wantErr:    ErrPushNotAllowed,
		},
		{
			name:       "force push not allowed by default",
			config:     DefaultGitConfig(),
			input:      &ToolInput{Command: GitOpPush, Args: map[string]any{"force": true}},
			wantErr:    ErrForceNotAllowed,
		},
		{
			name:       "force push allowed when configured",
			config:     &GitConfig{AllowPush: true, AllowForce: true},
			input:      &ToolInput{Command: GitOpPush, Args: map[string]any{"force": true}},
			wantErr:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGitTool(tt.config)
			err := g.Validate(tt.input)
			if tt.wantErr == nil && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
			if tt.wantErr != nil && err == nil {
				t.Errorf("Validate() expected error %v, got nil", tt.wantErr)
			}
		})
	}
}

func TestGitTool_ExecuteStatus(t *testing.T) {
	skipIfNoGit(t)
	dir := createTestRepo(t)
	g := NewGitTool(nil)
	ctx := context.Background()

	// Execute status on empty repo
	output, err := g.Execute(ctx, &ToolInput{
		Command:    GitOpStatus,
		WorkingDir: dir,
	})
	if err != nil {
		t.Fatalf("Execute(status) failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Execute(status) should succeed, got error: %s", output.Error)
	}

	// Add a file and check status again
	createTestFile(t, dir, "test.txt", "hello")

	output, err = g.Execute(ctx, &ToolInput{
		Command:    GitOpStatus,
		WorkingDir: dir,
	})
	if err != nil {
		t.Fatalf("Execute(status) failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Execute(status) should succeed, got error: %s", output.Error)
	}
	if !strings.Contains(output.Output, "test.txt") {
		t.Errorf("status output should contain test.txt, got: %s", output.Output)
	}
}

func TestGitTool_ExecuteAdd(t *testing.T) {
	skipIfNoGit(t)
	dir := createTestRepo(t)
	g := NewGitTool(nil)
	ctx := context.Background()

	// Create a test file
	createTestFile(t, dir, "test.txt", "hello")

	// Add the file
	output, err := g.Execute(ctx, &ToolInput{
		Command:    GitOpAdd,
		WorkingDir: dir,
		Args:       map[string]any{"files": "test.txt"},
	})
	if err != nil {
		t.Fatalf("Execute(add) failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Execute(add) should succeed, got error: %s", output.Error)
	}

	// Verify file is staged
	status, _ := g.Execute(ctx, &ToolInput{
		Command:    GitOpStatus,
		WorkingDir: dir,
	})
	if !strings.Contains(status.Output, "A") || !strings.Contains(status.Output, "test.txt") {
		t.Errorf("file should be staged, got status: %s", status.Output)
	}
}

func TestGitTool_ExecuteCommit(t *testing.T) {
	skipIfNoGit(t)
	dir := createTestRepo(t)
	g := NewGitTool(nil)
	ctx := context.Background()

	// Create and add a test file
	createTestFile(t, dir, "test.txt", "hello")
	g.Execute(ctx, &ToolInput{
		Command:    GitOpAdd,
		WorkingDir: dir,
		Args:       map[string]any{"files": "test.txt"},
	})

	// Commit
	output, err := g.Execute(ctx, &ToolInput{
		Command:    GitOpCommit,
		WorkingDir: dir,
		Args:       map[string]any{"message": "test commit"},
	})
	if err != nil {
		t.Fatalf("Execute(commit) failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Execute(commit) should succeed, got error: %s", output.Error)
	}

	// Verify commit with log
	log, _ := g.Execute(ctx, &ToolInput{
		Command:    GitOpLog,
		WorkingDir: dir,
		Args:       map[string]any{"limit": 1},
	})
	if !strings.Contains(log.Output, "test commit") {
		t.Errorf("log should contain commit message, got: %s", log.Output)
	}
}

func TestGitTool_ExecuteDiff(t *testing.T) {
	skipIfNoGit(t)
	dir := createTestRepo(t)
	g := NewGitTool(nil)
	ctx := context.Background()

	// Create, add, and commit a file
	createTestFile(t, dir, "test.txt", "hello")
	g.Execute(ctx, &ToolInput{
		Command:    GitOpAdd,
		WorkingDir: dir,
		Args:       map[string]any{"files": "test.txt"},
	})
	g.Execute(ctx, &ToolInput{
		Command:    GitOpCommit,
		WorkingDir: dir,
		Args:       map[string]any{"message": "initial"},
	})

	// Modify the file
	createTestFile(t, dir, "test.txt", "hello world")

	// Execute diff
	output, err := g.Execute(ctx, &ToolInput{
		Command:    GitOpDiff,
		WorkingDir: dir,
	})
	if err != nil {
		t.Fatalf("Execute(diff) failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Execute(diff) should succeed, got error: %s", output.Error)
	}
	if !strings.Contains(output.Output, "world") {
		t.Errorf("diff should show change, got: %s", output.Output)
	}
}

func TestGitTool_ExecuteBranch(t *testing.T) {
	skipIfNoGit(t)
	dir := createTestRepo(t)
	g := NewGitTool(nil)
	ctx := context.Background()

	// Create initial commit
	createTestFile(t, dir, "test.txt", "hello")
	g.Execute(ctx, &ToolInput{
		Command:    GitOpAdd,
		WorkingDir: dir,
		Args:       map[string]any{"files": "test.txt"},
	})
	g.Execute(ctx, &ToolInput{
		Command:    GitOpCommit,
		WorkingDir: dir,
		Args:       map[string]any{"message": "initial"},
	})

	// List branches
	output, err := g.Execute(ctx, &ToolInput{
		Command:    GitOpBranch,
		WorkingDir: dir,
	})
	if err != nil {
		t.Fatalf("Execute(branch) failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Execute(branch) should succeed, got error: %s", output.Error)
	}

	// Create new branch
	output, err = g.Execute(ctx, &ToolInput{
		Command:    GitOpBranch,
		WorkingDir: dir,
		Args:       map[string]any{"name": "feature"},
	})
	if err != nil {
		t.Fatalf("Execute(branch create) failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Execute(branch create) should succeed, got error: %s", output.Error)
	}

	// Verify branch exists
	output, err = g.Execute(ctx, &ToolInput{
		Command:    GitOpBranch,
		WorkingDir: dir,
	})
	if !strings.Contains(output.Output, "feature") {
		t.Errorf("branch list should contain 'feature', got: %s", output.Output)
	}
}

func TestGitTool_ExecuteCheckout(t *testing.T) {
	skipIfNoGit(t)
	dir := createTestRepo(t)
	g := NewGitTool(nil)
	ctx := context.Background()

	// Create initial commit
	createTestFile(t, dir, "test.txt", "hello")
	g.Execute(ctx, &ToolInput{
		Command:    GitOpAdd,
		WorkingDir: dir,
		Args:       map[string]any{"files": "test.txt"},
	})
	g.Execute(ctx, &ToolInput{
		Command:    GitOpCommit,
		WorkingDir: dir,
		Args:       map[string]any{"message": "initial"},
	})

	// Create and checkout new branch
	output, err := g.Execute(ctx, &ToolInput{
		Command:    GitOpCheckout,
		WorkingDir: dir,
		Args:       map[string]any{"ref": "feature", "create": true},
	})
	if err != nil {
		t.Fatalf("Execute(checkout -b) failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Execute(checkout -b) should succeed, got error: %s", output.Error)
	}

	// Verify we're on the new branch
	branch, err := g.GetCurrentBranch(ctx, dir)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "feature" {
		t.Errorf("expected branch 'feature', got %s", branch)
	}
}

func TestGitTool_ExecuteLog(t *testing.T) {
	skipIfNoGit(t)
	dir := createTestRepo(t)
	g := NewGitTool(nil)
	ctx := context.Background()

	// Create multiple commits
	for i := 0; i < 3; i++ {
		createTestFile(t, dir, "test.txt", strings.Repeat("x", i+1))
		g.Execute(ctx, &ToolInput{
			Command:    GitOpAdd,
			WorkingDir: dir,
			Args:       map[string]any{"files": "test.txt"},
		})
		g.Execute(ctx, &ToolInput{
			Command:    GitOpCommit,
			WorkingDir: dir,
			Args:       map[string]any{"message": strings.Repeat("commit ", i+1)},
		})
	}

	// Execute log with limit
	output, err := g.Execute(ctx, &ToolInput{
		Command:    GitOpLog,
		WorkingDir: dir,
		Args:       map[string]any{"limit": 2},
	})
	if err != nil {
		t.Fatalf("Execute(log) failed: %v", err)
	}
	if !output.Success {
		t.Errorf("Execute(log) should succeed, got error: %s", output.Error)
	}

	// Should show 2 commits
	lines := strings.Split(strings.TrimSpace(output.Output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 log entries, got %d: %s", len(lines), output.Output)
	}
}

func TestGitTool_NotGitRepository(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir() // Not a git repo
	g := NewGitTool(nil)
	ctx := context.Background()

	output, err := g.Execute(ctx, &ToolInput{
		Command:    GitOpStatus,
		WorkingDir: dir,
	})

	if err == nil {
		t.Error("expected error for non-git directory")
	}
	if output.Success {
		t.Error("expected success=false for non-git directory")
	}
}

func TestGitTool_HelperMethods(t *testing.T) {
	skipIfNoGit(t)
	dir := createTestRepo(t)
	g := NewGitTool(nil)
	ctx := context.Background()

	// Create initial commit
	createTestFile(t, dir, "test.txt", "hello")
	g.Execute(ctx, &ToolInput{
		Command:    GitOpAdd,
		WorkingDir: dir,
		Args:       map[string]any{"files": "test.txt"},
	})
	g.Execute(ctx, &ToolInput{
		Command:    GitOpCommit,
		WorkingDir: dir,
		Args:       map[string]any{"message": "initial"},
	})

	// Test IsClean
	clean, err := g.IsClean(ctx, dir)
	if err != nil {
		t.Fatalf("IsClean failed: %v", err)
	}
	if !clean {
		t.Error("repo should be clean after commit")
	}

	// Modify file
	createTestFile(t, dir, "test.txt", "modified")

	// Test IsClean (should be false now)
	clean, err = g.IsClean(ctx, dir)
	if err != nil {
		t.Fatalf("IsClean failed: %v", err)
	}
	if clean {
		t.Error("repo should not be clean after modification")
	}

	// Test GetCurrentBranch - handle both 'master' and 'main' as valid defaults
	branch, err := g.GetCurrentBranch(ctx, dir)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "main" && branch != "master" {
		t.Errorf("expected branch 'main' or 'master', got %s", branch)
	}
}

func TestParseGitStatus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []GitStatusEntry
	}{
		{
			name:   "empty",
			input:  "",
			expect: nil,
		},
		{
			name:  "untracked file",
			input: "? newfile.txt",
			expect: []GitStatusEntry{
				{Status: "?", Path: "newfile.txt", Worktree: true},
			},
		},
		{
			name:  "modified file (porcelain v2)",
			input: "1 .M N... 100644 100644 100644 abc123 def456 test.txt",
			expect: []GitStatusEntry{
				{Status: ".M", Path: "test.txt", Staged: false, Worktree: true},
			},
		},
		{
			name:  "staged file (porcelain v2)",
			input: "1 A. N... 100644 100644 100644 abc123 def456 new.txt",
			expect: []GitStatusEntry{
				{Status: "A.", Path: "new.txt", Staged: true, Worktree: false},
			},
		},
		{
			name:   "comment line",
			input:  "# branch.oid abc123",
			expect: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseGitStatus(tt.input)
			if len(got) != len(tt.expect) {
				t.Errorf("ParseGitStatus() returned %d entries, want %d", len(got), len(tt.expect))
				return
			}
			for i, e := range tt.expect {
				if got[i].Status != e.Status {
					t.Errorf("entry[%d].Status = %s, want %s", i, got[i].Status, e.Status)
				}
				if got[i].Path != e.Path {
					t.Errorf("entry[%d].Path = %s, want %s", i, got[i].Path, e.Path)
				}
				if got[i].Staged != e.Staged {
					t.Errorf("entry[%d].Staged = %v, want %v", i, got[i].Staged, e.Staged)
				}
				if got[i].Worktree != e.Worktree {
					t.Errorf("entry[%d].Worktree = %v, want %v", i, got[i].Worktree, e.Worktree)
				}
			}
		})
	}
}

func TestSanitizeCommitMessage(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"normal message", "normal message"},
		{"message\x00with null", "messagewith null"},
		{"tabs\tand\nnewlines", "tabs\tand\nnewlines"}, // tabs and newlines are allowed
		{"control\x01chars\x1f", "controlchars"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := SanitizeCommitMessage(tt.input); got != tt.expect {
				t.Errorf("SanitizeCommitMessage() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestExtractFiles(t *testing.T) {
	g := NewGitTool(nil)

	tests := []struct {
		name   string
		input  any
		expect []string
	}{
		{
			name:   "string with single file",
			input:  "file.txt",
			expect: []string{"file.txt"},
		},
		{
			name:   "string with multiple files",
			input:  "file1.txt file2.txt",
			expect: []string{"file1.txt", "file2.txt"},
		},
		{
			name:   "slice of strings",
			input:  []string{"a.txt", "b.txt"},
			expect: []string{"a.txt", "b.txt"},
		},
		{
			name:   "slice of any",
			input:  []any{"a.txt", "b.txt"},
			expect: []string{"a.txt", "b.txt"},
		},
		{
			name:   "nil input",
			input:  nil,
			expect: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.extractFiles(tt.input)
			if len(got) != len(tt.expect) {
				t.Errorf("extractFiles() = %v, want %v", got, tt.expect)
				return
			}
			for i, v := range tt.expect {
				if got[i] != v {
					t.Errorf("extractFiles()[%d] = %s, want %s", i, got[i], v)
				}
			}
		})
	}
}
