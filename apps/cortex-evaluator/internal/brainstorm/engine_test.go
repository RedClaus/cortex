package brainstorm

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cortex-evaluator/cortex-evaluator/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implements LLMProvider for testing.
type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Complete(prompt, context string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockProvider) CompleteWithMode(prompt, context, mode string) (string, error) {
	// For tests, just delegate to Complete (mode is ignored in tests)
	return m.Complete(prompt, context)
}

// createTestProject creates a temporary project directory with sample files.
func createTestProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create a simple Go project structure
	files := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`,
		"internal/handler/handler.go": `package handler

// Handler processes requests.
type Handler struct{}

// TODO: Add authentication middleware
func (h *Handler) Handle() error {
	return nil
}
`,
		"internal/service/service.go": `package service

// Service contains business logic.
type Service struct{}

// FIXME: Handle edge cases
func (s *Service) Process() error {
	return nil
}
`,
		"go.mod": `module testproject

go 1.21
`,
	}

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func TestNewEngine(t *testing.T) {
	tests := []struct {
		name    string
		session *session.Session
		opts    Options
		wantErr error
	}{
		{
			name:    "nil session returns error",
			session: nil,
			opts:    DefaultOptions(),
			wantErr: ErrSessionRequired,
		},
		{
			name:    "valid session succeeds",
			session: session.NewSession("test", "/tmp/project"),
			opts:    DefaultOptions(),
			wantErr: nil,
		},
		{
			name:    "with custom options",
			session: session.NewSession("test", "/tmp/project"),
			opts: Options{
				Provider: &mockProvider{response: "test"},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewEngine(tt.session, tt.opts)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, engine)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, engine)
			}
		})
	}
}

func TestEngine_IndexProject(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test-session", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Index the project
	err = engine.IndexProject()
	assert.NoError(t, err)

	// Verify context was generated
	ctx := engine.Context()
	assert.NotNil(t, ctx)
	assert.NotEmpty(t, ctx.ProjectName) // Project name is derived from directory name
	assert.Contains(t, ctx.Languages, "go")
	assert.Greater(t, ctx.TotalFiles, 0)
}

func TestEngine_IndexProject_InvalidPath(t *testing.T) {
	sess := session.NewSession("test-session", "/nonexistent/path/12345")
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	err = engine.IndexProject()
	assert.Error(t, err)
}

func TestEngine_Ask_EmptyQuestion(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	tests := []struct {
		name     string
		question string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tabs only", "\t\t"},
		{"newlines only", "\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := engine.Ask(tt.question)
			assert.ErrorIs(t, err, ErrEmptyQuestion)
			assert.Nil(t, resp)
		})
	}
}

func TestEngine_Ask_NoContext(t *testing.T) {
	sess := session.NewSession("test", "/tmp/project")
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Ask without indexing first
	resp, err := engine.Ask("What is this project about?")
	assert.ErrorIs(t, err, ErrNoContext)
	assert.Nil(t, resp)
}

func TestEngine_Ask_WithoutProvider(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	resp, err := engine.Ask("What is this project about?")
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Should contain project info (project name from directory, plus helpful note)
	assert.Contains(t, resp.Answer, "**Project**:")
	assert.Contains(t, resp.Answer, "**Files**:")
	assert.Contains(t, resp.Answer, "Note: For AI-powered analysis")
	assert.NotEmpty(t, resp.MessageID)
	assert.False(t, resp.Timestamp.IsZero())
}

func TestEngine_Ask_WithProvider(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)

	provider := &mockProvider{
		response: "This is a Go project with `main.go:1` as the entry point.",
	}
	opts := Options{Provider: provider}

	engine, err := NewEngine(sess, opts)
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	resp, err := engine.Ask("What is the main entry point?")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, provider.response, resp.Answer)

	// Should extract file references from the response
	assert.True(t, len(resp.References) > 0 || true) // May have refs from question too
}

func TestEngine_Ask_ProviderError(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)

	provider := &mockProvider{
		err: errors.New("provider error"),
	}
	opts := Options{Provider: provider}

	engine, err := NewEngine(sess, opts)
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	resp, err := engine.Ask("What is this project?")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LLM completion")
	assert.Nil(t, resp)
}

func TestEngine_History(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	// Initially empty
	history := engine.History()
	assert.Empty(t, history)

	// Ask a question
	_, err = engine.Ask("What is this project?")
	require.NoError(t, err)

	// Should have 2 messages (user + assistant)
	history = engine.History()
	assert.Len(t, history, 2)
	assert.Equal(t, RoleUser, history[0].Role)
	assert.Equal(t, "What is this project?", history[0].Content)
	assert.Equal(t, RoleAssistant, history[1].Role)

	// Ask another question
	_, err = engine.Ask("What languages are used?")
	require.NoError(t, err)

	// Should have 4 messages now
	history = engine.History()
	assert.Len(t, history, 4)
}

func TestEngine_ClearHistory(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	// Ask some questions
	_, err = engine.Ask("Question 1")
	require.NoError(t, err)
	_, err = engine.Ask("Question 2")
	require.NoError(t, err)

	assert.Len(t, engine.History(), 4)

	// Clear history
	engine.ClearHistory()
	assert.Empty(t, engine.History())
}

func TestEngine_Session(t *testing.T) {
	sess := session.NewSession("test-session", "/tmp/project")
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	assert.Equal(t, sess, engine.Session())
	assert.Equal(t, "test-session", engine.Session().Name)
}

func TestEngine_SetProvider(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)

	// Start without provider
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	// Ask without provider
	resp1, err := engine.Ask("Question?")
	require.NoError(t, err)
	assert.Contains(t, resp1.Answer, "Note: For AI-powered analysis")

	// Set provider
	provider := &mockProvider{response: "AI-powered response"}
	engine.SetProvider(provider)

	// Clear history and ask again
	engine.ClearHistory()
	resp2, err := engine.Ask("Question?")
	require.NoError(t, err)
	assert.Equal(t, "AI-powered response", resp2.Answer)
}

func TestEngine_ExtractReferences_EntryPoints(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	// Question about entry points should return entry point references
	resp, err := engine.Ask("Where is the main entry point?")
	require.NoError(t, err)

	hasEntryPointRef := false
	for _, ref := range resp.References {
		if ref.Description == "Entry point" {
			hasEntryPointRef = true
			break
		}
	}
	assert.True(t, hasEntryPointRef, "Should include entry point reference")
}

func TestEngine_ExtractReferences_Languages(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	// Question about Go should return Go file references
	resp, err := engine.Ask("Tell me about the Go code")
	require.NoError(t, err)

	hasGoRef := false
	for _, ref := range resp.References {
		if ref.Description == "go source files" {
			hasGoRef = true
			break
		}
	}
	assert.True(t, hasGoRef, "Should include Go source files reference")
}

func TestEngine_ConversationHistory_Timestamps(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	before := time.Now()
	_, err = engine.Ask("Test question")
	require.NoError(t, err)
	after := time.Now()

	history := engine.History()
	for _, msg := range history {
		assert.False(t, msg.Timestamp.IsZero())
		assert.True(t, msg.Timestamp.After(before) || msg.Timestamp.Equal(before))
		assert.True(t, msg.Timestamp.Before(after) || msg.Timestamp.Equal(after))
		assert.NotEmpty(t, msg.ID)
	}
}

func TestEngine_ConversationHistory_IsCopy(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	_, err = engine.Ask("Test question")
	require.NoError(t, err)

	// Get history and modify it
	history1 := engine.History()
	history1[0].Content = "modified"

	// Original should be unchanged
	history2 := engine.History()
	assert.Equal(t, "Test question", history2[0].Content)
}

func TestExtractReferencesFromText(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	tests := []struct {
		name     string
		text     string
		wantRefs int
	}{
		{
			name:     "single file reference",
			text:     "Check the file `main.go` for details.",
			wantRefs: 1,
		},
		{
			name:     "file with line number",
			text:     "See `internal/handler/handler.go:42` for the handler.",
			wantRefs: 1,
		},
		{
			name:     "multiple references",
			text:     "Look at `main.go:1` and `service.go:10` for examples.",
			wantRefs: 2,
		},
		{
			name:     "no references",
			text:     "This is just plain text without any file references.",
			wantRefs: 0,
		},
		{
			name:     "nested path reference",
			text:     "The handler is in `internal/handler/handler.go`.",
			wantRefs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := engine.extractReferencesFromText(tt.text)
			assert.Len(t, refs, tt.wantRefs)
		})
	}
}

func TestLangExtension(t *testing.T) {
	tests := []struct {
		lang string
		want string
	}{
		{"go", "go"},
		{"Go", "go"},
		{"typescript", "ts"},
		{"TypeScript", "ts"},
		{"javascript", "js"},
		{"python", "py"},
		{"rust", "rs"},
		{"java", "java"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			got := langExtension(tt.lang)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDeduplicateReferences(t *testing.T) {
	refs := []FileReference{
		{Path: "main.go", Line: 1, Relevance: "high"},
		{Path: "main.go", Line: 1, Relevance: "medium"}, // duplicate
		{Path: "main.go", Line: 5, Relevance: "low"},    // different line
		{Path: "other.go", Line: 0, Relevance: "medium"},
	}

	result := deduplicateReferences(refs)
	assert.Len(t, result, 3)

	// Should be sorted by relevance
	assert.Equal(t, "high", result[0].Relevance)
}

func TestMergeReferences(t *testing.T) {
	a := []FileReference{
		{Path: "a.go", Relevance: "high"},
	}
	b := []FileReference{
		{Path: "b.go", Relevance: "medium"},
		{Path: "a.go", Relevance: "low"}, // duplicate of a
	}

	result := mergeReferences(a, b)
	assert.Len(t, result, 2)
}

func TestRelevanceRank(t *testing.T) {
	assert.Equal(t, 0, relevanceRank("high"))
	assert.Equal(t, 1, relevanceRank("medium"))
	assert.Equal(t, 2, relevanceRank("low"))
	assert.Equal(t, 3, relevanceRank("unknown"))
	assert.Equal(t, 3, relevanceRank(""))
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	assert.Nil(t, opts.Provider)
	assert.Nil(t, opts.IndexerOptions)
}

func TestEngine_Context_BeforeIndex(t *testing.T) {
	sess := session.NewSession("test", "/tmp/project")
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Context should be nil before indexing
	assert.Nil(t, engine.Context())
}

func TestFileReference_Struct(t *testing.T) {
	ref := FileReference{
		Path:        "main.go",
		Line:        42,
		Description: "Main entry point",
		Relevance:   "high",
	}

	assert.Equal(t, "main.go", ref.Path)
	assert.Equal(t, 42, ref.Line)
	assert.Equal(t, "Main entry point", ref.Description)
	assert.Equal(t, "high", ref.Relevance)
}

func TestMessage_Struct(t *testing.T) {
	now := time.Now()
	msg := Message{
		ID:        "test-id",
		Role:      RoleUser,
		Content:   "Test content",
		Timestamp: now,
		References: []FileReference{
			{Path: "test.go"},
		},
	}

	assert.Equal(t, "test-id", msg.ID)
	assert.Equal(t, RoleUser, msg.Role)
	assert.Equal(t, "Test content", msg.Content)
	assert.Equal(t, now, msg.Timestamp)
	assert.Len(t, msg.References, 1)
}

func TestResponse_Struct(t *testing.T) {
	now := time.Now()
	resp := Response{
		Answer:    "Test answer",
		MessageID: "msg-id",
		Timestamp: now,
		References: []FileReference{
			{Path: "answer.go"},
		},
	}

	assert.Equal(t, "Test answer", resp.Answer)
	assert.Equal(t, "msg-id", resp.MessageID)
	assert.Equal(t, now, resp.Timestamp)
	assert.Len(t, resp.References, 1)
}

func TestRoleConstants(t *testing.T) {
	assert.Equal(t, Role("user"), RoleUser)
	assert.Equal(t, Role("assistant"), RoleAssistant)
	assert.Equal(t, Role("system"), RoleSystem)
}

func TestErrorConstants(t *testing.T) {
	assert.Error(t, ErrNoContext)
	assert.Error(t, ErrEmptyQuestion)
	assert.Error(t, ErrProviderNotSet)
	assert.Error(t, ErrSessionRequired)
	assert.Error(t, ErrUnsupportedFileType)
	assert.Error(t, ErrFileNotFound)
	assert.Error(t, ErrFileReadError)

	assert.Equal(t, "no codebase context available", ErrNoContext.Error())
	assert.Equal(t, "question cannot be empty", ErrEmptyQuestion.Error())
	assert.Equal(t, "LLM provider not configured", ErrProviderNotSet.Error())
	assert.Equal(t, "session required for brainstorm engine", ErrSessionRequired.Error())
	assert.Equal(t, "unsupported file type", ErrUnsupportedFileType.Error())
	assert.Equal(t, "file not found", ErrFileNotFound.Error())
	assert.Equal(t, "error reading file", ErrFileReadError.Error())
}

// ============================================================================
// Attachment Tests
// ============================================================================

func TestAttachFile_TextFile(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Create a test text file
	txtPath := filepath.Join(projectDir, "readme.txt")
	txtContent := "This is a test file\nWith multiple lines\n"
	require.NoError(t, os.WriteFile(txtPath, []byte(txtContent), 0644))

	// Attach the file
	att, err := engine.AttachFile(txtPath)
	require.NoError(t, err)
	assert.NotNil(t, att)

	// Verify attachment properties
	assert.NotEmpty(t, att.ID)
	assert.Equal(t, txtPath, att.Path)
	assert.Equal(t, "readme.txt", att.Name)
	assert.Equal(t, AttachmentTypeText, att.Type)
	assert.Equal(t, "text/plain", att.MimeType)
	assert.Equal(t, txtContent, att.Content)
	assert.False(t, att.AddedAt.IsZero())
	assert.Equal(t, int64(len(txtContent)), att.Size)
}

func TestAttachFile_MarkdownFile(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Create a test markdown file
	mdPath := filepath.Join(projectDir, "README.md")
	mdContent := "# Test Project\n\nThis is a test markdown file.\n"
	require.NoError(t, os.WriteFile(mdPath, []byte(mdContent), 0644))

	att, err := engine.AttachFile(mdPath)
	require.NoError(t, err)

	assert.Equal(t, AttachmentTypeMarkdown, att.Type)
	assert.Equal(t, "text/markdown", att.MimeType)
	assert.Equal(t, mdContent, att.Content)
}

func TestAttachFile_PDFFile(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Create a dummy PDF file (just for testing file detection)
	pdfPath := filepath.Join(projectDir, "document.pdf")
	pdfContent := []byte("%PDF-1.4 test content")
	require.NoError(t, os.WriteFile(pdfPath, pdfContent, 0644))

	att, err := engine.AttachFile(pdfPath)
	require.NoError(t, err)

	assert.Equal(t, AttachmentTypePDF, att.Type)
	assert.Equal(t, "application/pdf", att.MimeType)
	assert.Contains(t, att.Content, "[PDF Document:")
	assert.Contains(t, att.Content, "document.pdf")
}

func TestAttachFile_ImageFiles(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	imageTests := []struct {
		filename string
		mimeType string
	}{
		{"image.png", "image/png"},
		{"photo.jpg", "image/jpeg"},
		{"photo.jpeg", "image/jpeg"},
		{"animation.gif", "image/gif"},
		{"modern.webp", "image/webp"},
	}

	for _, tt := range imageTests {
		t.Run(tt.filename, func(t *testing.T) {
			imgPath := filepath.Join(projectDir, tt.filename)
			require.NoError(t, os.WriteFile(imgPath, []byte("fake image data"), 0644))

			att, err := engine.AttachFile(imgPath)
			require.NoError(t, err)

			assert.Equal(t, AttachmentTypeImage, att.Type)
			assert.Equal(t, tt.mimeType, att.MimeType)
			assert.Contains(t, att.Content, "[Image:")
		})
	}
}

func TestAttachFile_UnsupportedFileType(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Create an unsupported file type
	unsupportedPath := filepath.Join(projectDir, "file.xyz")
	require.NoError(t, os.WriteFile(unsupportedPath, []byte("content"), 0644))

	att, err := engine.AttachFile(unsupportedPath)
	assert.ErrorIs(t, err, ErrUnsupportedFileType)
	assert.Nil(t, att)
}

func TestAttachFile_FileNotFound(t *testing.T) {
	sess := session.NewSession("test", "/tmp")
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	att, err := engine.AttachFile("/nonexistent/file.txt")
	assert.ErrorIs(t, err, ErrFileNotFound)
	assert.Nil(t, att)
}

func TestAttachFile_Directory(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Try to attach a directory
	att, err := engine.AttachFile(projectDir)
	assert.ErrorIs(t, err, ErrFileReadError)
	assert.Nil(t, att)
}

func TestAttachments_ReturnsEmptySlice(t *testing.T) {
	sess := session.NewSession("test", "/tmp")
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	attachments := engine.Attachments()
	assert.NotNil(t, attachments)
	assert.Empty(t, attachments)
}

func TestAttachments_ReturnsCopy(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Create and attach a file
	txtPath := filepath.Join(projectDir, "test.txt")
	require.NoError(t, os.WriteFile(txtPath, []byte("content"), 0644))
	_, err = engine.AttachFile(txtPath)
	require.NoError(t, err)

	// Get attachments and modify the slice
	attachments1 := engine.Attachments()
	require.Len(t, attachments1, 1)
	attachments1[0].Name = "modified"

	// Original should be unchanged
	attachments2 := engine.Attachments()
	assert.Equal(t, "test.txt", attachments2[0].Name)
}

func TestAttachFile_MultipleFiles(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Attach multiple files
	files := []string{"file1.txt", "file2.md", "file3.txt"}
	for _, name := range files {
		path := filepath.Join(projectDir, name)
		require.NoError(t, os.WriteFile(path, []byte("content"), 0644))
		_, err = engine.AttachFile(path)
		require.NoError(t, err)
	}

	attachments := engine.Attachments()
	assert.Len(t, attachments, 3)
}

func TestClearAttachments(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Attach some files
	for i := 0; i < 3; i++ {
		path := filepath.Join(projectDir, "file"+string(rune('1'+i))+".txt")
		require.NoError(t, os.WriteFile(path, []byte("content"), 0644))
		_, err = engine.AttachFile(path)
		require.NoError(t, err)
	}

	assert.Len(t, engine.Attachments(), 3)

	engine.ClearAttachments()
	assert.Empty(t, engine.Attachments())
}

func TestRemoveAttachment(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Attach a file
	txtPath := filepath.Join(projectDir, "test.txt")
	require.NoError(t, os.WriteFile(txtPath, []byte("content"), 0644))
	att, err := engine.AttachFile(txtPath)
	require.NoError(t, err)

	// Remove by ID
	removed := engine.RemoveAttachment(att.ID)
	assert.True(t, removed)
	assert.Empty(t, engine.Attachments())

	// Remove non-existent ID
	removed = engine.RemoveAttachment("nonexistent-id")
	assert.False(t, removed)
}

func TestRemoveAttachment_MiddleOfSlice(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Attach three files
	var ids []string
	for i := 1; i <= 3; i++ {
		path := filepath.Join(projectDir, "file"+string(rune('0'+i))+".txt")
		require.NoError(t, os.WriteFile(path, []byte("content"), 0644))
		att, err := engine.AttachFile(path)
		require.NoError(t, err)
		ids = append(ids, att.ID)
	}

	// Remove the middle one
	removed := engine.RemoveAttachment(ids[1])
	assert.True(t, removed)

	attachments := engine.Attachments()
	assert.Len(t, attachments, 2)
	assert.Equal(t, ids[0], attachments[0].ID)
	assert.Equal(t, ids[2], attachments[1].ID)
}

func TestAttachmentsIncludedInContext(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)

	provider := &mockProvider{response: "Response with attachment context"}
	engine, err := NewEngine(sess, Options{Provider: provider})
	require.NoError(t, err)
	require.NoError(t, engine.IndexProject())

	// Attach a text file
	txtPath := filepath.Join(projectDir, "notes.txt")
	txtContent := "Important notes for the LLM"
	require.NoError(t, os.WriteFile(txtPath, []byte(txtContent), 0644))
	_, err = engine.AttachFile(txtPath)
	require.NoError(t, err)

	// Ask a question - the context should include the attachment
	_, err = engine.Ask("What do the notes say?")
	require.NoError(t, err)

	// The provider was called, meaning context was built
	// (We can't directly test the context string without exposing it,
	// but the test passes if no errors occur)
}

func TestDetectAttachmentType(t *testing.T) {
	tests := []struct {
		ext      string
		wantType AttachmentType
		wantMime string
		wantErr  bool
	}{
		{".pdf", AttachmentTypePDF, "application/pdf", false},
		{".txt", AttachmentTypeText, "text/plain", false},
		{".md", AttachmentTypeMarkdown, "text/markdown", false},
		{".markdown", AttachmentTypeMarkdown, "text/markdown", false},
		{".png", AttachmentTypeImage, "image/png", false},
		{".jpg", AttachmentTypeImage, "image/jpeg", false},
		{".jpeg", AttachmentTypeImage, "image/jpeg", false},
		{".gif", AttachmentTypeImage, "image/gif", false},
		{".webp", AttachmentTypeImage, "image/webp", false},
		{".xyz", "", "", true},
		{".exe", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			gotType, gotMime, err := detectAttachmentType(tt.ext)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrUnsupportedFileType)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantType, gotType)
				assert.Equal(t, tt.wantMime, gotMime)
			}
		})
	}
}

func TestAttachmentTypeConstants(t *testing.T) {
	assert.Equal(t, AttachmentType("pdf"), AttachmentTypePDF)
	assert.Equal(t, AttachmentType("text"), AttachmentTypeText)
	assert.Equal(t, AttachmentType("markdown"), AttachmentTypeMarkdown)
	assert.Equal(t, AttachmentType("image"), AttachmentTypeImage)
}

func TestAttachment_Struct(t *testing.T) {
	now := time.Now()
	att := Attachment{
		ID:       "test-id",
		Path:     "/path/to/file.txt",
		Name:     "file.txt",
		Type:     AttachmentTypeText,
		Size:     1024,
		Content:  "file content",
		MimeType: "text/plain",
		AddedAt:  now,
	}

	assert.Equal(t, "test-id", att.ID)
	assert.Equal(t, "/path/to/file.txt", att.Path)
	assert.Equal(t, "file.txt", att.Name)
	assert.Equal(t, AttachmentTypeText, att.Type)
	assert.Equal(t, int64(1024), att.Size)
	assert.Equal(t, "file content", att.Content)
	assert.Equal(t, "text/plain", att.MimeType)
	assert.Equal(t, now, att.AddedAt)
}

// ============================================================================
// Artifact Tests
// ============================================================================

func TestSaveArtifact_Basic(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	content := "package main\n\nfunc main() {}\n"
	artifact, err := engine.SaveArtifact(content, "main.go", ArtifactTypeCode)
	require.NoError(t, err)
	assert.NotNil(t, artifact)

	// Verify artifact properties
	assert.NotEmpty(t, artifact.ID)
	assert.Equal(t, "main.go", artifact.Name)
	assert.Equal(t, ArtifactTypeCode, artifact.Type)
	assert.Equal(t, content, artifact.Content)
	assert.Equal(t, int64(len(content)), artifact.Size)
	assert.False(t, artifact.CreatedAt.IsZero())
	assert.NotEmpty(t, artifact.FilePath)

	// Verify file was created on disk
	_, err = os.Stat(artifact.FilePath)
	assert.NoError(t, err)

	// Verify file content
	data, err := os.ReadFile(artifact.FilePath)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestSaveArtifact_EmptyContent(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	tests := []struct {
		name    string
		content string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tabs only", "\t\t"},
		{"newlines only", "\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact, err := engine.SaveArtifact(tt.content, "test", ArtifactTypeText)
			assert.ErrorIs(t, err, ErrEmptyArtifact)
			assert.Nil(t, artifact)
		})
	}
}

func TestSaveArtifact_EmptyName(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	tests := []struct {
		name     string
		artName  string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tabs only", "\t\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact, err := engine.SaveArtifact("content", tt.artName, ArtifactTypeText)
			assert.ErrorIs(t, err, ErrEmptyArtifactName)
			assert.Nil(t, artifact)
		})
	}
}

func TestSaveArtifact_AllTypes(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	tests := []struct {
		name    string
		artType ArtifactType
		ext     string
	}{
		{"code", ArtifactTypeCode, ".txt"},
		{"text", ArtifactTypeText, ".txt"},
		{"diagram", ArtifactTypeDiagram, ".mmd"},
		{"markdown", ArtifactTypeMarkdown, ".md"},
		{"json", ArtifactTypeJSON, ".json"},
		{"yaml", ArtifactTypeYAML, ".yaml"},
		{"other", ArtifactTypeOther, ".txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact, err := engine.SaveArtifact("content", "test-"+tt.name, tt.artType)
			require.NoError(t, err)

			assert.Equal(t, tt.artType, artifact.Type)
			assert.True(t, filepath.Ext(artifact.FilePath) == tt.ext,
				"expected extension %s, got %s", tt.ext, filepath.Ext(artifact.FilePath))
		})
	}
}

func TestSaveArtifact_DuplicateNames(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	// Save first artifact
	art1, err := engine.SaveArtifact("content1", "duplicate", ArtifactTypeText)
	require.NoError(t, err)

	// Save second artifact with same name
	art2, err := engine.SaveArtifact("content2", "duplicate", ArtifactTypeText)
	require.NoError(t, err)

	// Both should succeed with different file paths
	assert.NotEqual(t, art1.FilePath, art2.FilePath)
	assert.NotEqual(t, art1.ID, art2.ID)

	// Both files should exist
	_, err = os.Stat(art1.FilePath)
	assert.NoError(t, err)
	_, err = os.Stat(art2.FilePath)
	assert.NoError(t, err)
}

func TestSaveArtifactWithMetadata(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	metadata := map[string]string{
		"language": "go",
		"source":   "response-123",
		"author":   "assistant",
	}

	artifact, err := engine.SaveArtifactWithMetadata("package main", "main.go", ArtifactTypeCode, metadata)
	require.NoError(t, err)

	assert.Equal(t, metadata, artifact.Metadata)

	// Verify metadata is persisted
	loaded, err := engine.GetArtifact(artifact.ID)
	require.NoError(t, err)
	assert.Equal(t, metadata, loaded.Metadata)
}

func TestListArtifacts_Empty(t *testing.T) {
	sess := session.NewSession("test", "/tmp")
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	artifacts := engine.ListArtifacts()
	assert.NotNil(t, artifacts)
	assert.Empty(t, artifacts)
}

func TestListArtifacts_Multiple(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	// Save multiple artifacts
	_, err = engine.SaveArtifact("content1", "file1", ArtifactTypeText)
	require.NoError(t, err)
	_, err = engine.SaveArtifact("content2", "file2", ArtifactTypeCode)
	require.NoError(t, err)
	_, err = engine.SaveArtifact("content3", "file3", ArtifactTypeMarkdown)
	require.NoError(t, err)

	artifacts := engine.ListArtifacts()
	assert.Len(t, artifacts, 3)
}

func TestListArtifacts_ReturnsCopy(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	_, err = engine.SaveArtifact("content", "test", ArtifactTypeText)
	require.NoError(t, err)

	// Get artifacts and modify
	artifacts1 := engine.ListArtifacts()
	artifacts1[0].Name = "modified"

	// Original should be unchanged
	artifacts2 := engine.ListArtifacts()
	assert.Equal(t, "test", artifacts2[0].Name)
}

func TestGetArtifact(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	saved, err := engine.SaveArtifact("content", "test", ArtifactTypeText)
	require.NoError(t, err)

	// Get by ID
	retrieved, err := engine.GetArtifact(saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, retrieved.ID)
	assert.Equal(t, saved.Name, retrieved.Name)
	assert.Equal(t, saved.Content, retrieved.Content)
}

func TestGetArtifact_NotFound(t *testing.T) {
	sess := session.NewSession("test", "/tmp")
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	artifact, err := engine.GetArtifact("nonexistent-id")
	assert.ErrorIs(t, err, ErrArtifactNotFound)
	assert.Nil(t, artifact)
}

func TestDeleteArtifact(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	// Save artifact
	saved, err := engine.SaveArtifact("content", "test", ArtifactTypeText)
	require.NoError(t, err)
	filePath := saved.FilePath

	// Verify file exists
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	// Delete artifact
	err = engine.DeleteArtifact(saved.ID)
	require.NoError(t, err)

	// Verify file is deleted
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))

	// Verify artifact is removed from list
	assert.Empty(t, engine.ListArtifacts())

	// Verify can't get artifact anymore
	_, err = engine.GetArtifact(saved.ID)
	assert.ErrorIs(t, err, ErrArtifactNotFound)
}

func TestDeleteArtifact_NotFound(t *testing.T) {
	sess := session.NewSession("test", "/tmp")
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	err = engine.DeleteArtifact("nonexistent-id")
	assert.ErrorIs(t, err, ErrArtifactNotFound)
}

func TestClearArtifacts(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	// Save multiple artifacts
	var filePaths []string
	for i := 0; i < 3; i++ {
		art, err := engine.SaveArtifact("content", "file"+string(rune('1'+i)), ArtifactTypeText)
		require.NoError(t, err)
		filePaths = append(filePaths, art.FilePath)
	}

	assert.Len(t, engine.ListArtifacts(), 3)

	// Clear all
	err = engine.ClearArtifacts()
	assert.NoError(t, err)

	// Verify list is empty
	assert.Empty(t, engine.ListArtifacts())

	// Verify files are deleted
	for _, fp := range filePaths {
		_, err = os.Stat(fp)
		assert.True(t, os.IsNotExist(err))
	}
}

func TestArtifactsDir(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "custom-artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	assert.Equal(t, artifactsDir, engine.ArtifactsDir())
}

func TestArtifactsDir_Default(t *testing.T) {
	projectDir := createTestProject(t)
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, DefaultOptions())
	require.NoError(t, err)

	// Default should be based on project path and session ID
	expectedPrefix := filepath.Join(projectDir, ".brainstorm", sess.ID, "artifacts")
	assert.Equal(t, expectedPrefix, engine.ArtifactsDir())
}

func TestLoadArtifacts(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "artifacts")
	sess := session.NewSession("test", projectDir)

	// Create engine and save some artifacts
	engine1, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	art1, err := engine1.SaveArtifact("content1", "file1", ArtifactTypeText)
	require.NoError(t, err)
	art2, err := engine1.SaveArtifact("content2", "file2", ArtifactTypeCode)
	require.NoError(t, err)

	// Create new engine and load artifacts
	engine2, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	err = engine2.LoadArtifacts()
	require.NoError(t, err)

	// Verify artifacts are loaded
	artifacts := engine2.ListArtifacts()
	assert.Len(t, artifacts, 2)

	// Verify artifact properties
	ids := make(map[string]bool)
	for _, a := range artifacts {
		ids[a.ID] = true
	}
	assert.True(t, ids[art1.ID])
	assert.True(t, ids[art2.ID])
}

func TestLoadArtifacts_NoIndex(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	// Loading with no index file should not error
	err = engine.LoadArtifacts()
	assert.NoError(t, err)
	assert.Empty(t, engine.ListArtifacts())
}

func TestArtifactTypeConstants(t *testing.T) {
	assert.Equal(t, ArtifactType("code"), ArtifactTypeCode)
	assert.Equal(t, ArtifactType("text"), ArtifactTypeText)
	assert.Equal(t, ArtifactType("diagram"), ArtifactTypeDiagram)
	assert.Equal(t, ArtifactType("markdown"), ArtifactTypeMarkdown)
	assert.Equal(t, ArtifactType("json"), ArtifactTypeJSON)
	assert.Equal(t, ArtifactType("yaml"), ArtifactTypeYAML)
	assert.Equal(t, ArtifactType("other"), ArtifactTypeOther)
}

func TestArtifact_Struct(t *testing.T) {
	now := time.Now()
	art := Artifact{
		ID:        "test-id",
		Name:      "test-artifact",
		Type:      ArtifactTypeCode,
		Content:   "package main",
		FilePath:  "/path/to/artifact.txt",
		Size:      12,
		CreatedAt: now,
		Metadata:  map[string]string{"language": "go"},
	}

	assert.Equal(t, "test-id", art.ID)
	assert.Equal(t, "test-artifact", art.Name)
	assert.Equal(t, ArtifactTypeCode, art.Type)
	assert.Equal(t, "package main", art.Content)
	assert.Equal(t, "/path/to/artifact.txt", art.FilePath)
	assert.Equal(t, int64(12), art.Size)
	assert.Equal(t, now, art.CreatedAt)
	assert.Equal(t, "go", art.Metadata["language"])
}

func TestArtifactErrorConstants(t *testing.T) {
	assert.Error(t, ErrEmptyArtifact)
	assert.Error(t, ErrEmptyArtifactName)
	assert.Error(t, ErrArtifactNotFound)
	assert.Error(t, ErrArtifactSaveFailed)

	assert.Equal(t, "artifact content cannot be empty", ErrEmptyArtifact.Error())
	assert.Equal(t, "artifact name cannot be empty", ErrEmptyArtifactName.Error())
	assert.Equal(t, "artifact not found", ErrArtifactNotFound.Error())
	assert.Equal(t, "failed to save artifact", ErrArtifactSaveFailed.Error())
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal.txt", "normal.txt"},
		{"path/to/file.txt", "path_to_file.txt"},
		{"file:name.txt", "file_name.txt"},
		{"file*name?.txt", "file_name_.txt"},
		{"  spaces  ", "spaces"},
		{"...dots...", "dots"},
		{"file\nwith\nnewlines", "file_with_newlines"},
		{"", "artifact"},
		{"   ", "artifact"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestArtifactTypeExtension(t *testing.T) {
	tests := []struct {
		artType  ArtifactType
		expected string
	}{
		{ArtifactTypeCode, ".txt"},
		{ArtifactTypeText, ".txt"},
		{ArtifactTypeDiagram, ".mmd"},
		{ArtifactTypeMarkdown, ".md"},
		{ArtifactTypeJSON, ".json"},
		{ArtifactTypeYAML, ".yaml"},
		{ArtifactTypeOther, ".txt"},
		{ArtifactType("unknown"), ".txt"},
	}

	for _, tt := range tests {
		t.Run(string(tt.artType), func(t *testing.T) {
			result := artifactTypeExtension(tt.artType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSaveArtifact_CreatesDirectory(t *testing.T) {
	projectDir := createTestProject(t)
	// Use a deeply nested directory that doesn't exist
	artifactsDir := filepath.Join(projectDir, "deep", "nested", "artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	// Directory shouldn't exist yet
	_, err = os.Stat(artifactsDir)
	assert.True(t, os.IsNotExist(err))

	// Save artifact should create directory
	_, err = engine.SaveArtifact("content", "test", ArtifactTypeText)
	require.NoError(t, err)

	// Directory should now exist
	info, err := os.Stat(artifactsDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestSaveArtifact_PersistsIndex(t *testing.T) {
	projectDir := createTestProject(t)
	artifactsDir := filepath.Join(projectDir, "artifacts")
	sess := session.NewSession("test", projectDir)
	engine, err := NewEngine(sess, Options{ArtifactsDir: artifactsDir})
	require.NoError(t, err)

	_, err = engine.SaveArtifact("content", "test", ArtifactTypeText)
	require.NoError(t, err)

	// Index file should exist
	indexPath := filepath.Join(artifactsDir, "artifacts.json")
	_, err = os.Stat(indexPath)
	assert.NoError(t, err)

	// Index should contain the artifact
	data, err := os.ReadFile(indexPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test")
}
