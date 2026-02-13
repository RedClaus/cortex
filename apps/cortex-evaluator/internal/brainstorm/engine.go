// Package brainstorm provides a Q&A interface for codebase exploration and understanding.
package brainstorm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cortex-evaluator/cortex-evaluator/internal/indexer"
	"github.com/cortex-evaluator/cortex-evaluator/internal/session"
	"github.com/google/uuid"
)

// Common errors for brainstorm operations.
var (
	ErrNoContext           = errors.New("no codebase context available")
	ErrEmptyQuestion       = errors.New("question cannot be empty")
	ErrProviderNotSet      = errors.New("LLM provider not configured")
	ErrSessionRequired     = errors.New("session required for brainstorm engine")
	ErrUnsupportedFileType = errors.New("unsupported file type")
	ErrFileNotFound        = errors.New("file not found")
	ErrFileReadError       = errors.New("error reading file")
	ErrEmptyArtifact       = errors.New("artifact content cannot be empty")
	ErrEmptyArtifactName   = errors.New("artifact name cannot be empty")
	ErrArtifactNotFound    = errors.New("artifact not found")
	ErrArtifactSaveFailed  = errors.New("failed to save artifact")
)

// AttachmentType represents the type of an attached file.
type AttachmentType string

const (
	AttachmentTypePDF      AttachmentType = "pdf"
	AttachmentTypeText     AttachmentType = "text"
	AttachmentTypeMarkdown AttachmentType = "markdown"
	AttachmentTypeImage    AttachmentType = "image"
)

// Attachment represents a file attached to the brainstorm session.
type Attachment struct {
	ID        string         `json:"id"`
	Path      string         `json:"path"`
	Name      string         `json:"name"`
	Type      AttachmentType `json:"type"`
	Size      int64          `json:"size"`
	Content   string         `json:"content,omitempty"`   // Extracted text content
	MimeType  string         `json:"mime_type,omitempty"`
	AddedAt   time.Time      `json:"added_at"`
}

// ArtifactType represents the type of a saved artifact.
type ArtifactType string

const (
	ArtifactTypeCode     ArtifactType = "code"
	ArtifactTypeText     ArtifactType = "text"
	ArtifactTypeDiagram  ArtifactType = "diagram"
	ArtifactTypeMarkdown ArtifactType = "markdown"
	ArtifactTypeJSON     ArtifactType = "json"
	ArtifactTypeYAML     ArtifactType = "yaml"
	ArtifactTypeOther    ArtifactType = "other"
)

// Artifact represents a valuable output saved from a brainstorm session.
// Artifacts include code snippets, text content, diagrams, and other outputs
// worth preserving for later use.
type Artifact struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Type      ArtifactType `json:"type"`
	Content   string       `json:"content"`
	FilePath  string       `json:"file_path"` // Path where artifact is saved
	Size      int64        `json:"size"`
	CreatedAt time.Time    `json:"created_at"`
	Metadata  map[string]string `json:"metadata,omitempty"` // Optional metadata (language, source, etc.)
}

// Role represents the role of a message in the conversation.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// FileReference represents a reference to a file in the codebase.
type FileReference struct {
	Path        string `json:"path"`
	Line        int    `json:"line,omitempty"`
	Description string `json:"description,omitempty"`
	Relevance   string `json:"relevance,omitempty"` // high, medium, low
}

// Message represents a single message in the conversation history.
type Message struct {
	ID         string          `json:"id"`
	Role       Role            `json:"role"`
	Content    string          `json:"content"`
	References []FileReference `json:"references,omitempty"`
	Timestamp  time.Time       `json:"timestamp"`
}

// Response represents the result of an Ask operation.
type Response struct {
	Answer     string          `json:"answer"`
	References []FileReference `json:"references"`
	MessageID  string          `json:"message_id"`
	Timestamp  time.Time       `json:"timestamp"`
}

// Prompt modes for different evaluation tasks.
const (
	ModeGeneral    = "general"         // Default codebase Q&A
	ModeCodeReview = "code-review"     // Deep code review, CR generation
	ModeIdea       = "idea-evaluation" // Paper/repo evaluation, integration proposals
)

// LLMProvider defines the interface for language model interactions.
// This allows plugging in different LLM backends (OpenAI, Anthropic, local models, etc.)
type LLMProvider interface {
	// Complete generates a completion for the given prompt with context.
	Complete(prompt string, context string) (string, error)

	// CompleteWithMode generates a completion using a specific prompt mode.
	// Valid modes: "general", "code-review", "idea-evaluation"
	CompleteWithMode(prompt string, context string, mode string) (string, error)
}

// Engine is the core brainstorm engine that provides Q&A functionality
// with full codebase context awareness.
type Engine struct {
	session       *session.Session
	generator     *indexer.Generator
	provider      LLMProvider
	conversation  []Message
	context       *indexer.ContextResult
	todos         *indexer.TodoResult
	insights      *indexer.InsightResult
	attachments   []Attachment
	artifacts     []Artifact
	artifactsDir  string // Directory where artifacts are persisted
	githubFetcher *GitHubFetcher
	promptMode    string // Current prompt mode (general, code-review, idea-evaluation)
	mu            sync.RWMutex
}

// Options configures the BrainstormEngine.
type Options struct {
	// Provider is the LLM provider for generating responses.
	// If nil, the engine will return context-enriched prompts without AI completion.
	Provider LLMProvider

	// IndexerOptions configures the project indexer.
	IndexerOptions *indexer.Options

	// ArtifactsDir is the directory where artifacts will be saved.
	// If empty, a default directory based on session ID will be used.
	ArtifactsDir string
}

// DefaultOptions returns default engine options.
func DefaultOptions() Options {
	return Options{
		Provider:       nil,
		IndexerOptions: nil,
	}
}

// NewEngine creates a new BrainstormEngine for the given session.
// The engine indexes the session's project path to build codebase context.
func NewEngine(sess *session.Session, opts Options) (*Engine, error) {
	if sess == nil {
		return nil, ErrSessionRequired
	}

	var gen *indexer.Generator
	if opts.IndexerOptions != nil {
		gen = indexer.NewGeneratorWithOptions(*opts.IndexerOptions)
	} else {
		gen = indexer.NewGenerator()
	}

	// Determine artifacts directory
	artifactsDir := opts.ArtifactsDir
	if artifactsDir == "" {
		// Default to a subdirectory in the project path
		artifactsDir = filepath.Join(sess.ProjectPath, ".brainstorm", sess.ID, "artifacts")
	}

	engine := &Engine{
		session:       sess,
		generator:     gen,
		provider:      opts.Provider,
		conversation:  make([]Message, 0),
		artifacts:     make([]Artifact, 0),
		artifactsDir:  artifactsDir,
		githubFetcher: NewGitHubFetcher(),
		promptMode:    ModeGeneral, // Default to general mode
	}

	return engine, nil
}

// SetMode sets the current prompt mode for the engine.
// Valid modes: "general", "code-review", "idea-evaluation"
func (e *Engine) SetMode(mode string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.promptMode = mode
}

// GetMode returns the current prompt mode.
func (e *Engine) GetMode() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.promptMode
}

// IndexProject indexes the project and builds the codebase context.
// This should be called before Ask() to populate the context.
func (e *Engine) IndexProject() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Generate context from the project
	ctx, err := e.generator.GenerateContext(e.session.ProjectPath)
	if err != nil {
		return fmt.Errorf("generating context: %w", err)
	}
	e.context = ctx

	// Generate todos
	todos, err := e.generator.GenerateTodos(e.session.ProjectPath)
	if err != nil {
		// Non-fatal: continue without todos
		e.todos = nil
	} else {
		e.todos = todos
	}

	// Generate insights
	insights, err := e.generator.GenerateInsights(e.session.ProjectPath)
	if err != nil {
		// Non-fatal: continue without insights
		e.insights = nil
	} else {
		e.insights = insights
	}

	return nil
}

// Ask processes a question about the codebase and returns a contextual response.
// The question is analyzed against the indexed codebase context, and relevant
// file references are extracted and included in the response.
func (e *Engine) Ask(question string) (*Response, error) {
	if strings.TrimSpace(question) == "" {
		return nil, ErrEmptyQuestion
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Ensure we have context
	if e.context == nil {
		return nil, ErrNoContext
	}

	// Detect and fetch GitHub URLs mentioned in the question
	var fetchedRepos []*GitHubRepo
	githubURLs := ExtractGitHubURLs(question)
	for _, url := range githubURLs {
		repo, err := e.githubFetcher.FetchRepo(url)
		if err == nil {
			fetchedRepos = append(fetchedRepos, repo)
		}
	}

	// Create user message
	userMsg := Message{
		ID:        uuid.New().String(),
		Role:      RoleUser,
		Content:   question,
		Timestamp: time.Now(),
	}
	e.conversation = append(e.conversation, userMsg)

	// Build prompt with context
	prompt := e.buildPrompt(question)

	// Extract relevant file references from the question
	references := e.extractReferences(question)

	var answer string
	if e.provider != nil {
		// Use LLM provider for completion
		contextStr := e.buildContextString()

		// Add fetched GitHub repos to context
		if len(fetchedRepos) > 0 {
			contextStr += "\n\n=== EXTERNAL REPOSITORIES (FETCHED FROM GITHUB) ===\n"
			for _, repo := range fetchedRepos {
				contextStr += "\n" + repo.FormatContext() + "\n"
			}
		}

		var err error
		answer, err = e.provider.CompleteWithMode(prompt, contextStr, e.promptMode)
		if err != nil {
			return nil, fmt.Errorf("LLM completion: %w", err)
		}
		// Extract additional references from the answer
		answerRefs := e.extractReferencesFromText(answer)
		references = mergeReferences(references, answerRefs)
	} else {
		// No provider: return a helpful message with context info
		answer = e.buildContextualAnswer(question, references)
	}

	// Create assistant message
	assistantMsg := Message{
		ID:         uuid.New().String(),
		Role:       RoleAssistant,
		Content:    answer,
		References: references,
		Timestamp:  time.Now(),
	}
	e.conversation = append(e.conversation, assistantMsg)

	return &Response{
		Answer:     answer,
		References: references,
		MessageID:  assistantMsg.ID,
		Timestamp:  assistantMsg.Timestamp,
	}, nil
}

// History returns the conversation history.
func (e *Engine) History() []Message {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Return a copy to prevent external modification
	history := make([]Message, len(e.conversation))
	copy(history, e.conversation)
	return history
}

// ClearHistory clears the conversation history.
func (e *Engine) ClearHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.conversation = make([]Message, 0)
}

// Context returns the current codebase context.
func (e *Engine) Context() *indexer.ContextResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.context
}

// Session returns the associated session.
func (e *Engine) Session() *session.Session {
	return e.session
}

// SetProvider sets or updates the LLM provider.
func (e *Engine) SetProvider(provider LLMProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.provider = provider
}

// AttachFile adds a file to the session context.
// Supports PDF, TXT, MD, and image files (PNG, JPG, JPEG, GIF, WEBP).
func (e *Engine) AttachFile(path string) (*Attachment, error) {
	// Check if file exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrFileNotFound, path)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFileReadError, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%w: path is a directory", ErrFileReadError)
	}

	// Determine file type
	ext := strings.ToLower(filepath.Ext(path))
	attachType, mimeType, err := detectAttachmentType(ext)
	if err != nil {
		return nil, err
	}

	// Extract content based on file type
	content, err := extractContent(path, attachType)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFileReadError, err)
	}

	attachment := Attachment{
		ID:       uuid.New().String(),
		Path:     path,
		Name:     filepath.Base(path),
		Type:     attachType,
		Size:     info.Size(),
		Content:  content,
		MimeType: mimeType,
		AddedAt:  time.Now(),
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.attachments = append(e.attachments, attachment)

	return &attachment, nil
}

// Attachments returns a copy of the current attachments.
func (e *Engine) Attachments() []Attachment {
	e.mu.RLock()
	defer e.mu.RUnlock()

	attachments := make([]Attachment, len(e.attachments))
	copy(attachments, e.attachments)
	return attachments
}

// ClearAttachments removes all attachments from the session.
func (e *Engine) ClearAttachments() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.attachments = make([]Attachment, 0)
}

// RemoveAttachment removes an attachment by ID.
func (e *Engine) RemoveAttachment(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, att := range e.attachments {
		if att.ID == id {
			e.attachments = append(e.attachments[:i], e.attachments[i+1:]...)
			return true
		}
	}
	return false
}

// SaveArtifact persists an artifact (code, text, diagram) to the session folder.
// The artifact is saved to disk and tracked in session metadata.
func (e *Engine) SaveArtifact(content, name string, artifactType ArtifactType) (*Artifact, error) {
	if strings.TrimSpace(content) == "" {
		return nil, ErrEmptyArtifact
	}
	if strings.TrimSpace(name) == "" {
		return nil, ErrEmptyArtifactName
	}

	// Create artifacts directory if it doesn't exist
	if err := os.MkdirAll(e.artifactsDir, 0755); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrArtifactSaveFailed, err)
	}

	// Generate unique ID and filename
	id := uuid.New().String()
	ext := artifactTypeExtension(artifactType)
	filename := sanitizeFilename(name)
	if !strings.HasSuffix(filename, ext) {
		filename = filename + ext
	}
	filePath := filepath.Join(e.artifactsDir, filename)

	// Handle duplicate filenames by appending ID suffix
	if _, err := os.Stat(filePath); err == nil {
		// File exists, add ID to make it unique
		base := strings.TrimSuffix(filename, ext)
		filename = fmt.Sprintf("%s_%s%s", base, id[:8], ext)
		filePath = filepath.Join(e.artifactsDir, filename)
	}

	// Write content to file
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrArtifactSaveFailed, err)
	}

	artifact := Artifact{
		ID:        id,
		Name:      name,
		Type:      artifactType,
		Content:   content,
		FilePath:  filePath,
		Size:      int64(len(content)),
		CreatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}

	e.mu.Lock()
	e.artifacts = append(e.artifacts, artifact)
	e.mu.Unlock()

	// Save artifact metadata to index file
	if err := e.saveArtifactIndex(); err != nil {
		// Non-fatal: artifact is saved, just metadata indexing failed
		// Log warning in production
	}

	return &artifact, nil
}

// SaveArtifactWithMetadata persists an artifact with additional metadata.
func (e *Engine) SaveArtifactWithMetadata(content, name string, artifactType ArtifactType, metadata map[string]string) (*Artifact, error) {
	artifact, err := e.SaveArtifact(content, name, artifactType)
	if err != nil {
		return nil, err
	}

	if metadata != nil {
		e.mu.Lock()
		// Find and update the artifact's metadata
		for i := range e.artifacts {
			if e.artifacts[i].ID == artifact.ID {
				e.artifacts[i].Metadata = metadata
				artifact.Metadata = metadata
				break
			}
		}
		e.mu.Unlock()

		// Update the index file
		_ = e.saveArtifactIndex()
	}

	return artifact, nil
}

// ListArtifacts returns all saved artifacts for this session.
func (e *Engine) ListArtifacts() []Artifact {
	e.mu.RLock()
	defer e.mu.RUnlock()

	artifacts := make([]Artifact, len(e.artifacts))
	copy(artifacts, e.artifacts)
	return artifacts
}

// GetArtifact retrieves an artifact by ID.
func (e *Engine) GetArtifact(id string) (*Artifact, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, art := range e.artifacts {
		if art.ID == id {
			// Return a copy
			copy := art
			return &copy, nil
		}
	}
	return nil, ErrArtifactNotFound
}

// DeleteArtifact removes an artifact by ID from both memory and disk.
func (e *Engine) DeleteArtifact(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, art := range e.artifacts {
		if art.ID == id {
			// Remove file from disk
			if err := os.Remove(art.FilePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to delete artifact file: %w", err)
			}

			// Remove from slice
			e.artifacts = append(e.artifacts[:i], e.artifacts[i+1:]...)

			// Update index
			_ = e.saveArtifactIndexLocked()
			return nil
		}
	}
	return ErrArtifactNotFound
}

// ClearArtifacts removes all artifacts from memory and disk.
func (e *Engine) ClearArtifacts() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var lastErr error
	for _, art := range e.artifacts {
		if err := os.Remove(art.FilePath); err != nil && !os.IsNotExist(err) {
			lastErr = err
		}
	}

	e.artifacts = make([]Artifact, 0)

	// Remove index file
	indexPath := filepath.Join(e.artifactsDir, "artifacts.json")
	_ = os.Remove(indexPath)

	return lastErr
}

// ArtifactsDir returns the directory where artifacts are stored.
func (e *Engine) ArtifactsDir() string {
	return e.artifactsDir
}

// LoadArtifacts loads artifacts from the session's artifact index file.
// This is useful for resuming a session.
func (e *Engine) LoadArtifacts() error {
	indexPath := filepath.Join(e.artifactsDir, "artifacts.json")

	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No artifacts yet
		}
		return fmt.Errorf("failed to read artifacts index: %w", err)
	}

	var artifacts []Artifact
	if err := json.Unmarshal(data, &artifacts); err != nil {
		return fmt.Errorf("failed to parse artifacts index: %w", err)
	}

	e.mu.Lock()
	e.artifacts = artifacts
	e.mu.Unlock()

	return nil
}

// saveArtifactIndex persists the artifact metadata to an index file.
func (e *Engine) saveArtifactIndex() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.saveArtifactIndexLocked()
}

// saveArtifactIndexLocked saves the index without acquiring locks (caller must hold lock).
func (e *Engine) saveArtifactIndexLocked() error {
	indexPath := filepath.Join(e.artifactsDir, "artifacts.json")

	data, err := json.MarshalIndent(e.artifacts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal artifacts: %w", err)
	}

	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write artifacts index: %w", err)
	}

	return nil
}

// artifactTypeExtension returns the file extension for an artifact type.
func artifactTypeExtension(t ArtifactType) string {
	switch t {
	case ArtifactTypeCode:
		return ".txt" // Generic, could be specified via metadata
	case ArtifactTypeText:
		return ".txt"
	case ArtifactTypeDiagram:
		return ".mmd" // Mermaid diagram format
	case ArtifactTypeMarkdown:
		return ".md"
	case ArtifactTypeJSON:
		return ".json"
	case ArtifactTypeYAML:
		return ".yaml"
	default:
		return ".txt"
	}
}

// sanitizeFilename removes or replaces characters that are invalid in filenames.
func sanitizeFilename(name string) string {
	// Replace invalid characters with underscores
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\n", "\r", "\t"}
	result := name
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}

	// Trim spaces and dots from beginning/end
	result = strings.Trim(result, " .")

	// Limit length
	if len(result) > 200 {
		result = result[:200]
	}

	// Ensure non-empty
	if result == "" {
		result = "artifact"
	}

	return result
}

// detectAttachmentType determines the attachment type and MIME type from file extension.
func detectAttachmentType(ext string) (AttachmentType, string, error) {
	switch ext {
	case ".pdf":
		return AttachmentTypePDF, "application/pdf", nil
	case ".txt":
		return AttachmentTypeText, "text/plain", nil
	case ".md", ".markdown":
		return AttachmentTypeMarkdown, "text/markdown", nil
	case ".png":
		return AttachmentTypeImage, "image/png", nil
	case ".jpg", ".jpeg":
		return AttachmentTypeImage, "image/jpeg", nil
	case ".gif":
		return AttachmentTypeImage, "image/gif", nil
	case ".webp":
		return AttachmentTypeImage, "image/webp", nil
	default:
		return "", "", fmt.Errorf("%w: %s", ErrUnsupportedFileType, ext)
	}
}

// extractContent extracts text content from a file based on its type.
func extractContent(path string, attachType AttachmentType) (string, error) {
	switch attachType {
	case AttachmentTypeText, AttachmentTypeMarkdown:
		return extractTextContent(path)
	case AttachmentTypePDF:
		return extractPDFContent(path)
	case AttachmentTypeImage:
		return extractImageContent(path)
	default:
		return "", fmt.Errorf("unknown attachment type: %s", attachType)
	}
}

// extractTextContent reads plain text or markdown files.
func extractTextContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// extractPDFContent extracts text from PDF files.
// Note: Full PDF parsing requires a library like pdfcpu or unipdf.
// This implementation provides a placeholder that returns metadata.
func extractPDFContent(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	// Return metadata placeholder - full PDF text extraction would require
	// a PDF parsing library (e.g., pdfcpu, unipdf, or calling pdftotext)
	return fmt.Sprintf("[PDF Document: %s, Size: %d bytes]", filepath.Base(path), info.Size()), nil
}

// extractImageContent returns metadata for image files.
// Images are included in context as references for vision-capable LLMs.
func extractImageContent(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[Image: %s, Size: %d bytes]", filepath.Base(path), info.Size()), nil
}

// buildPrompt constructs the prompt for the LLM including conversation history.
func (e *Engine) buildPrompt(question string) string {
	var sb strings.Builder

	// Include recent conversation history (last 5 exchanges)
	historyStart := 0
	if len(e.conversation) > 10 {
		historyStart = len(e.conversation) - 10
	}
	if historyStart > 0 {
		sb.WriteString("## Previous Conversation\n")
		for i := historyStart; i < len(e.conversation); i++ {
			msg := e.conversation[i]
			if msg.Role == RoleUser {
				fmt.Fprintf(&sb, "**User:** %s\n\n", msg.Content)
			} else {
				fmt.Fprintf(&sb, "**Assistant:** %s\n\n", msg.Content)
			}
		}
	}

	sb.WriteString("## Question\n")
	sb.WriteString(question)

	return sb.String()
}

// buildContextString creates the context string for the LLM.
func (e *Engine) buildContextString() string {
	var sb strings.Builder

	if e.context != nil {
		sb.WriteString("=== PROJECT CONTEXT ===\n")
		sb.WriteString(e.context.Content)
		sb.WriteString("\n\n")
	}

	if e.insights != nil && len(e.insights.Items) > 0 {
		sb.WriteString("=== PROJECT INSIGHTS ===\n")
		sb.WriteString(e.insights.Content)
		sb.WriteString("\n\n")
	}

	if e.todos != nil && len(e.todos.Items) > 0 {
		sb.WriteString("=== CURRENT TODOS ===\n")
		// Include just high priority todos for context
		for _, item := range e.todos.Items {
			if item.Priority == "high" {
				fmt.Fprintf(&sb, "- [%s] %s:%d - %s\n",
					item.Type, item.File, item.Line, item.Text)
			}
		}
		sb.WriteString("\n")
	}

	// Include attached files in context
	if len(e.attachments) > 0 {
		sb.WriteString("=== ATTACHED FILES ===\n")
		for _, att := range e.attachments {
			fmt.Fprintf(&sb, "\n--- %s (%s) ---\n", att.Name, att.Type)
			if att.Content != "" {
				sb.WriteString(att.Content)
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// buildContextualAnswer builds a contextual answer when no LLM provider is available.
func (e *Engine) buildContextualAnswer(question string, refs []FileReference) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Regarding your question: %q\n\n", question)
	sb.WriteString("Based on the indexed codebase:\n\n")

	// Provide project overview
	if e.context != nil {
		fmt.Fprintf(&sb, "**Project**: %s\n", e.context.ProjectName)
		fmt.Fprintf(&sb, "**Files**: %d total\n", e.context.TotalFiles)
		if len(e.context.Languages) > 0 {
			fmt.Fprintf(&sb, "**Languages**: %s\n", strings.Join(e.context.Languages, ", "))
		}
		if len(e.context.EntryPoints) > 0 {
			fmt.Fprintf(&sb, "**Entry Points**: %d found\n", len(e.context.EntryPoints))
		}
		sb.WriteString("\n")
	}

	// Include relevant file references
	if len(refs) > 0 {
		sb.WriteString("**Potentially Relevant Files**:\n")
		for _, ref := range refs {
			if ref.Line > 0 {
				fmt.Fprintf(&sb, "- `%s:%d`", ref.Path, ref.Line)
			} else {
				fmt.Fprintf(&sb, "- `%s`", ref.Path)
			}
			if ref.Description != "" {
				fmt.Fprintf(&sb, " - %s", ref.Description)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Suggest what could be done with an LLM provider
	sb.WriteString("*Note: For AI-powered analysis, configure an LLM provider.*")

	return sb.String()
}

// extractReferences extracts file references based on the question content.
func (e *Engine) extractReferences(question string) []FileReference {
	refs := make([]FileReference, 0)
	questionLower := strings.ToLower(question)

	if e.context == nil {
		return refs
	}

	// Check if question mentions entry points
	if strings.Contains(questionLower, "entry") ||
		strings.Contains(questionLower, "main") ||
		strings.Contains(questionLower, "start") {
		for _, ep := range e.context.EntryPoints {
			refs = append(refs, FileReference{
				Path:        ep,
				Description: "Entry point",
				Relevance:   "high",
			})
		}
	}

	// Check for language-specific questions
	for _, lang := range e.context.Languages {
		if strings.Contains(questionLower, strings.ToLower(lang)) {
			refs = append(refs, FileReference{
				Path:        fmt.Sprintf("*.%s files", langExtension(lang)),
				Description: fmt.Sprintf("%s source files", lang),
				Relevance:   "medium",
			})
		}
	}

	// Add references from insights evidence
	if e.insights != nil {
		for _, insight := range e.insights.Items {
			for _, word := range strings.Fields(questionLower) {
				if len(word) > 3 && strings.Contains(strings.ToLower(insight.Title), word) {
					for _, evidence := range insight.Evidence {
						refs = append(refs, FileReference{
							Path:        evidence,
							Description: insight.Title,
							Relevance:   "medium",
						})
					}
					break
				}
			}
		}
	}

	// Deduplicate and limit references
	refs = deduplicateReferences(refs)
	if len(refs) > 10 {
		refs = refs[:10]
	}

	return refs
}

// extractReferencesFromText extracts file references from response text.
func (e *Engine) extractReferencesFromText(text string) []FileReference {
	refs := make([]FileReference, 0)

	// Pattern to match file paths like `path/to/file.go:123` or `path/to/file.go`
	pathPattern := regexp.MustCompile("`([a-zA-Z0-9_/.-]+\\.[a-zA-Z]+)(?::([0-9]+))?`")
	matches := pathPattern.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		ref := FileReference{
			Path:      match[1],
			Relevance: "high",
		}
		if len(match) > 2 && match[2] != "" {
			fmt.Sscanf(match[2], "%d", &ref.Line)
		}
		refs = append(refs, ref)
	}

	return refs
}

// langExtension returns the file extension for a language.
func langExtension(lang string) string {
	extensions := map[string]string{
		"go":         "go",
		"typescript": "ts",
		"javascript": "js",
		"python":     "py",
		"rust":       "rs",
		"java":       "java",
		"c":          "c",
		"c++":        "cpp",
	}
	if ext, ok := extensions[strings.ToLower(lang)]; ok {
		return ext
	}
	return strings.ToLower(lang)
}

// deduplicateReferences removes duplicate file references.
func deduplicateReferences(refs []FileReference) []FileReference {
	seen := make(map[string]bool)
	result := make([]FileReference, 0)

	for _, ref := range refs {
		key := fmt.Sprintf("%s:%d", ref.Path, ref.Line)
		if !seen[key] {
			seen[key] = true
			result = append(result, ref)
		}
	}

	// Sort by relevance (high first)
	sort.Slice(result, func(i, j int) bool {
		return relevanceRank(result[i].Relevance) < relevanceRank(result[j].Relevance)
	})

	return result
}

// mergeReferences combines two slices of references, removing duplicates.
func mergeReferences(a, b []FileReference) []FileReference {
	combined := append(a, b...)
	return deduplicateReferences(combined)
}

// relevanceRank returns a numeric rank for sorting by relevance.
func relevanceRank(relevance string) int {
	switch relevance {
	case "high":
		return 0
	case "medium":
		return 1
	case "low":
		return 2
	default:
		return 3
	}
}
