package introspection

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/internal/memory"
	"github.com/normanking/cortex/pkg/types"
)

// AcquisitionType defines the method of knowledge acquisition.
type AcquisitionType string

const (
	AcquisitionTypeFile      AcquisitionType = "file_ingest"
	AcquisitionTypeWebSearch AcquisitionType = "web_search"
	AcquisitionTypeDocCrawl  AcquisitionType = "documentation_crawl"
)

// AcquisitionRequest represents a request to acquire knowledge.
type AcquisitionRequest struct {
	Type        AcquisitionType   `json:"type"`
	Subject     string            `json:"subject"`
	FilePath    string            `json:"file_path,omitempty"`
	SearchQuery string            `json:"search_query,omitempty"`
	DocURL      string            `json:"doc_url,omitempty"`
	Metadata    map[string]string `json:"metadata"`
}

// AcquisitionResult contains the results of an acquisition operation.
type AcquisitionResult struct {
	Success       bool          `json:"success"`
	ItemsIngested int           `json:"items_ingested"`
	Categories    []string      `json:"categories"`
	TopicCreated  string        `json:"topic_created,omitempty"`
	Duration      time.Duration `json:"duration"`
	Error         string        `json:"error,omitempty"`
	Sources       []string      `json:"sources"`
}

// Interfaces to avoid import cycles

// KnowledgeFabricCreator defines the interface for creating knowledge items.
type KnowledgeFabricCreator interface {
	Create(ctx context.Context, item *types.KnowledgeItem) error
}

// TopicStoreCreator defines the interface for creating topics.
type TopicStoreCreator interface {
	CreateTopic(ctx context.Context, topic *memory.Topic) error
}

// WebSearchResult represents a single search result.
type WebSearchResult struct {
	URL     string
	Title   string
	Content string
	Score   float64
}

// WebSearcher defines the interface for web searching.
type WebSearcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error)
}

// EventPublisher defines the interface for publishing events.
type EventPublisher interface {
	Publish(event interface{})
}

// AcquisitionEngine handles knowledge acquisition from various sources.
type AcquisitionEngine struct {
	knowledgeFabric KnowledgeFabricCreator
	topicStore      TopicStoreCreator
	webSearchTool   WebSearcher
	eventBus        EventPublisher
	llmProvider     llm.Provider
}

// NewAcquisitionEngine creates a new AcquisitionEngine instance.
func NewAcquisitionEngine(
	fabric KnowledgeFabricCreator,
	topics TopicStoreCreator,
	webSearch WebSearcher,
	eventBus EventPublisher,
	llm llm.Provider,
) *AcquisitionEngine {
	return &AcquisitionEngine{
		knowledgeFabric: fabric,
		topicStore:      topics,
		webSearchTool:   webSearch,
		eventBus:        eventBus,
		llmProvider:     llm,
	}
}

// Acquire executes the acquisition request.
func (ae *AcquisitionEngine) Acquire(ctx context.Context, req *AcquisitionRequest) (*AcquisitionResult, error) {
	start := time.Now()

	var result *AcquisitionResult
	var err error

	switch req.Type {
	case AcquisitionTypeFile:
		result, err = ae.acquireFromFile(ctx, req)
	case AcquisitionTypeWebSearch:
		result, err = ae.acquireFromWeb(ctx, req)
	case AcquisitionTypeDocCrawl:
		// reuse web search logic for now or implement specific crawl later
		// treating as web search if query provided, else error
		if req.SearchQuery != "" {
			result, err = ae.acquireFromWeb(ctx, req)
		} else {
			err = fmt.Errorf("documentation crawl not implemented without search query")
		}
	default:
		err = fmt.Errorf("unknown acquisition type: %s", req.Type)
	}

	if result == nil {
		result = &AcquisitionResult{
			Success: false,
		}
	}

	result.Duration = time.Since(start)

	if err != nil {
		result.Error = err.Error()
		result.Success = false
		return result, err
	}

	return result, nil
}

// acquireFromFile is a stub for file ingestion.
func (ae *AcquisitionEngine) acquireFromFile(ctx context.Context, req *AcquisitionRequest) (*AcquisitionResult, error) {
	return nil, fmt.Errorf("file ingestion pipeline not yet implemented")
}

// acquireFromWeb executes web search acquisition.
func (ae *AcquisitionEngine) acquireFromWeb(ctx context.Context, req *AcquisitionRequest) (*AcquisitionResult, error) {
	queries := ae.generateSearchQueries(req.Subject)
	if req.SearchQuery != "" {
		queries = append([]string{req.SearchQuery}, queries...)
	}

	var allResults []WebSearchResult
	seenURLs := make(map[string]bool)

	// Execute searches
	for _, query := range queries {
		results, err := ae.webSearchTool.Search(ctx, query, 5) // fetch top 5 per query
		if err != nil {
			continue // skip failed searches
		}

		for _, res := range results {
			if !seenURLs[res.URL] {
				seenURLs[res.URL] = true
				allResults = append(allResults, res)
			}
		}
	}

	if len(allResults) == 0 {
		return &AcquisitionResult{
			Success:       true,
			ItemsIngested: 0,
			Sources:       []string{},
		}, nil
	}

	// Process results
	ingestedCount := 0
	var sources []string
	var allCategories []string

	category := ae.categorizeSubject(req.Subject)
	allCategories = append(allCategories, category)

	// Create a new topic for this acquisition
	topicID := fmt.Sprintf("topic_%s", uuid.New().String()[:8])
	topic := &memory.Topic{
		ID:           topicID,
		Name:         fmt.Sprintf("Acquisition: %s", req.Subject),
		Description:  fmt.Sprintf("Knowledge acquired about %s", req.Subject),
		Keywords:     ae.extractTags(req.Subject),
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		IsActive:     true,
		MemberCount:  0,
	}

	for _, res := range allResults {
		knowledgeItems, err := ae.extractKnowledge(ctx, req.Subject, res.Content)
		if err != nil {
			continue
		}

		for _, k := range knowledgeItems {
			item := &types.KnowledgeItem{
				ID:         uuid.New().String(),
				Type:       types.TypeDocument,
				Title:      fmt.Sprintf("%s - %s", req.Subject, res.Title),
				Content:    k,
				Tags:       append(topic.Keywords, category),
				Scope:      types.ScopeGlobal, // Default to global for acquired knowledge
				AuthorID:   "system_acquisition",
				AuthorName: "Cortex Acquisition Engine",
				Confidence: 0.8,
				TrustScore: 1.0, // Trusted source?
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				SyncStatus: "pending",
				Version:    1,
			}

			// Guard against nil knowledgeFabric (CR-018 fix)
			if ae.knowledgeFabric != nil {
				if err := ae.knowledgeFabric.Create(ctx, item); err == nil {
					ingestedCount++
				}
			}
		}
		sources = append(sources, res.URL)
	}

	topic.MemberCount = ingestedCount

	// Try to create the topic, ignore error if store not ready (CR-018 fix)
	if ingestedCount > 0 && ae.topicStore != nil {
		_ = ae.topicStore.CreateTopic(ctx, topic)
	}

	return &AcquisitionResult{
		Success:       true,
		ItemsIngested: ingestedCount,
		Categories:    allCategories,
		TopicCreated:  topic.ID,
		Sources:       ae.uniqueSources(sources),
	}, nil
}

// extractKnowledge uses LLM to extract structured knowledge facts.
func (ae *AcquisitionEngine) extractKnowledge(ctx context.Context, subject, content string) ([]string, error) {
	promptTemplate := `Extract structured knowledge about "%s" from the following content.

Content:
%s

Return a JSON array of knowledge items. Each item should be:
- A single, atomic fact or piece of information
- Self-contained and understandable without context
- Accurate to the source content

Example output format:
["The 'ls' command lists directory contents", "ls -la shows hidden files with details"]

Extract knowledge items:`

	// Truncate content to avoid token limits (simple heuristic)
	content = ae.truncateForSummary(content, 4000)

	prompt := fmt.Sprintf(promptTemplate, subject, content)

	resp, err := ae.llmProvider.Chat(ctx, &llm.ChatRequest{
		Model: ae.llmProvider.Name(), // Use default model
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.1, // Low temp for extraction
	})
	if err != nil {
		return nil, err
	}

	// Clean response to ensure valid JSON (remove markdown blocks if present)
	cleaned := strings.TrimSpace(resp.Content)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var items []string
	if err := json.Unmarshal([]byte(cleaned), &items); err != nil {
		return nil, fmt.Errorf("parse extracted knowledge: %w", err)
	}

	return items, nil
}

// emitEvent publishes an event to the bus.
func (ae *AcquisitionEngine) emitEvent(eventType string, data map[string]any) {
	if ae.eventBus != nil {
		ae.eventBus.Publish(map[string]any{
			"type":      eventType,
			"timestamp": time.Now(),
			"data":      data,
		})
	}
}

// Helpers

func (ae *AcquisitionEngine) generateSearchQueries(subject string) []string {
	return []string{
		subject,
		fmt.Sprintf("%s reference", subject),
		fmt.Sprintf("%s examples", subject),
	}
}

func (ae *AcquisitionEngine) categorizeSubject(subject string) string {
	// Simple keyword based categorization
	s := strings.ToLower(subject)
	if strings.Contains(s, "cmd") || strings.Contains(s, "command") || strings.Contains(s, "cli") {
		return "commands"
	}
	if strings.Contains(s, "go") || strings.Contains(s, "python") || strings.Contains(s, "js") || strings.Contains(s, "code") {
		return "programming"
	}
	if strings.Contains(s, "deploy") || strings.Contains(s, "docker") || strings.Contains(s, "k8s") {
		return "devops"
	}
	return "general"
}

func (ae *AcquisitionEngine) extractTags(subject string) []string {
	parts := strings.Fields(subject)
	var tags []string
	for _, p := range parts {
		if len(p) > 3 {
			tags = append(tags, strings.ToLower(p))
		}
	}
	return tags
}

func (ae *AcquisitionEngine) uniqueSources(sources []string) []string {
	seen := make(map[string]bool)
	var unique []string
	for _, s := range sources {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}
	return unique
}

func (ae *AcquisitionEngine) truncateForSummary(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
