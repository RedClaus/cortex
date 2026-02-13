package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type InventoryResult struct {
	Subject       string                 `json:"subject"`
	TotalMatches  int                    `json:"total_matches"`
	ByStore       map[string]StoreResult `json:"by_store"`
	TopResults    []InventoryItem        `json:"top_results"`
	RelatedTopics []string               `json:"related_topics"`
	Confidence    float64                `json:"confidence"`
	QueryDuration time.Duration          `json:"query_duration"`
}

type StoreResult struct {
	StoreName string `json:"store_name"`
	Count     int    `json:"count"`
	HasMore   bool   `json:"has_more"`
}

type InventoryItem struct {
	ID        string            `json:"id"`
	Source    string            `json:"source"`
	Content   string            `json:"content"`
	Summary   string            `json:"summary"`
	Relevance float64           `json:"relevance"`
	Metadata  map[string]string `json:"metadata"`
}

type KnowledgeSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]KnowledgeSearchResult, error)
}

type KnowledgeSearchResult struct {
	ID         string
	Content    string
	Summary    string
	Category   string
	Scope      string
	Confidence float64
}

type ArchivalSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]ArchivalItem, error)
}

type ArchivalItem struct {
	ID       string
	Content  string
	Score    float64
	Metadata map[string]string
}

type KnowledgeInventory struct {
	knowledgeSearcher KnowledgeSearcher
	strategicStore    *StrategicMemoryStore
	topicStore        *TopicStore
	coreStore         *CoreMemoryStore
	archivalSearcher  ArchivalSearcher
	embedder          Embedder
}

func NewKnowledgeInventory(
	knowledge KnowledgeSearcher,
	strategic *StrategicMemoryStore,
	topics *TopicStore,
	core *CoreMemoryStore,
	archival ArchivalSearcher,
	embedder Embedder,
) *KnowledgeInventory {
	return &KnowledgeInventory{
		knowledgeSearcher: knowledge,
		strategicStore:    strategic,
		topicStore:        topics,
		coreStore:         core,
		archivalSearcher:  archival,
		embedder:          embedder,
	}
}

func (ki *KnowledgeInventory) Query(ctx context.Context, subject string, searchTerms []string) (*InventoryResult, error) {
	start := time.Now()

	result := &InventoryResult{
		Subject:    subject,
		ByStore:    make(map[string]StoreResult),
		TopResults: make([]InventoryItem, 0),
	}

	searchQuery := subject
	if len(searchTerms) > 0 {
		searchQuery = subject + " " + strings.Join(searchTerms, " ")
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errCh := make(chan error, 5)

	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := ki.searchKnowledge(ctx, searchQuery)
		if err != nil {
			errCh <- err
			return
		}
		mu.Lock()
		result.ByStore["knowledge_fabric"] = StoreResult{
			StoreName: "knowledge_fabric",
			Count:     len(items),
			HasMore:   len(items) >= 10,
		}
		result.TopResults = append(result.TopResults, items...)
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := ki.searchStrategicMemory(ctx, searchQuery)
		if err != nil {
			errCh <- err
			return
		}
		mu.Lock()
		result.ByStore["strategic_memory"] = StoreResult{
			StoreName: "strategic_memory",
			Count:     len(items),
			HasMore:   len(items) >= 10,
		}
		result.TopResults = append(result.TopResults, items...)
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		topics, err := ki.searchTopics(ctx, searchQuery)
		if err != nil {
			errCh <- err
			return
		}
		mu.Lock()
		result.ByStore["topic_clusters"] = StoreResult{
			StoreName: "topic_clusters",
			Count:     len(topics),
			HasMore:   false,
		}
		result.RelatedTopics = topics
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := ki.searchCoreMemory(ctx, searchQuery)
		if err != nil {
			errCh <- err
			return
		}
		mu.Lock()
		result.ByStore["core_memory"] = StoreResult{
			StoreName: "core_memory",
			Count:     len(items),
			HasMore:   false,
		}
		result.TopResults = append(result.TopResults, items...)
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := ki.searchArchivalMemory(ctx, searchQuery)
		if err != nil {
			errCh <- err
			return
		}
		mu.Lock()
		result.ByStore["archival_memory"] = StoreResult{
			StoreName: "archival_memory",
			Count:     len(items),
			HasMore:   len(items) >= 10,
		}
		result.TopResults = append(result.TopResults, items...)
		mu.Unlock()
	}()

	wg.Wait()
	close(errCh)

	for _, storeResult := range result.ByStore {
		result.TotalMatches += storeResult.Count
	}

	sortByRelevance(result.TopResults)

	if len(result.TopResults) > 10 {
		result.TopResults = result.TopResults[:10]
	}

	result.Confidence = calculateConfidence(result)
	result.QueryDuration = time.Since(start)

	return result, nil
}

func (ki *KnowledgeInventory) searchKnowledge(ctx context.Context, query string) ([]InventoryItem, error) {
	if ki.knowledgeSearcher == nil {
		return nil, nil
	}

	results, err := ki.knowledgeSearcher.Search(ctx, query, 10)
	if err != nil {
		return nil, err
	}

	items := make([]InventoryItem, 0, len(results))
	for _, r := range results {
		items = append(items, InventoryItem{
			ID:        r.ID,
			Source:    "knowledge_fabric",
			Content:   truncate(r.Content, 200),
			Summary:   r.Summary,
			Relevance: r.Confidence,
			Metadata: map[string]string{
				"category": r.Category,
				"scope":    r.Scope,
			},
		})
	}

	return items, nil
}

func (ki *KnowledgeInventory) searchStrategicMemory(ctx context.Context, query string) ([]InventoryItem, error) {
	if ki.strategicStore == nil {
		return nil, nil
	}

	memories, err := ki.strategicStore.SearchSimilar(ctx, query, 10)
	if err != nil {
		return nil, err
	}

	items := make([]InventoryItem, 0, len(memories))
	for _, mem := range memories {
		items = append(items, InventoryItem{
			ID:        mem.ID,
			Source:    "strategic_memory",
			Content:   mem.Principle,
			Summary:   fmt.Sprintf("Principle: %s (%.0f%% success rate)", mem.Principle, mem.SuccessRate*100),
			Relevance: mem.Confidence,
			Metadata: map[string]string{
				"category":     mem.Category,
				"success_rate": fmt.Sprintf("%.2f", mem.SuccessRate),
			},
		})
	}

	return items, nil
}

func (ki *KnowledgeInventory) searchTopics(ctx context.Context, query string) ([]string, error) {
	if ki.topicStore == nil {
		return nil, nil
	}

	topic, _, err := ki.topicStore.GetActiveTopic(ctx, query)
	if err != nil {
		return nil, nil
	}

	if topic != nil {
		return []string{topic.Name}, nil
	}

	return nil, nil
}

func (ki *KnowledgeInventory) searchCoreMemory(ctx context.Context, query string) ([]InventoryItem, error) {
	if ki.coreStore == nil {
		return nil, nil
	}

	items := make([]InventoryItem, 0)
	queryLower := strings.ToLower(query)

	userMem, err := ki.coreStore.GetUserMemory(ctx, "default")
	if err == nil && userMem != nil {
		for _, fact := range userMem.CustomFacts {
			if strings.Contains(strings.ToLower(fact.Fact), queryLower) {
				items = append(items, InventoryItem{
					ID:        "user_fact_" + fact.Fact[:min(20, len(fact.Fact))],
					Source:    "core_memory",
					Content:   fact.Fact,
					Summary:   fmt.Sprintf("User fact: %s", truncate(fact.Fact, 50)),
					Relevance: 0.7,
					Metadata: map[string]string{
						"type":   "user_fact",
						"source": fact.Source,
					},
				})
			}
		}
	}

	projectMem, err := ki.coreStore.GetProjectMemory(ctx, "default")
	if err == nil && projectMem != nil {
		for _, conv := range projectMem.Conventions {
			if strings.Contains(strings.ToLower(conv), queryLower) {
				items = append(items, InventoryItem{
					ID:        "project_convention_" + conv[:min(20, len(conv))],
					Source:    "core_memory",
					Content:   conv,
					Summary:   fmt.Sprintf("Project convention: %s", truncate(conv, 50)),
					Relevance: 0.7,
					Metadata: map[string]string{
						"type":    "project_convention",
						"project": projectMem.Name,
					},
				})
			}
		}

		for _, tech := range projectMem.TechStack {
			if strings.Contains(strings.ToLower(tech), queryLower) {
				items = append(items, InventoryItem{
					ID:        "project_tech_" + tech,
					Source:    "core_memory",
					Content:   tech,
					Summary:   fmt.Sprintf("Project tech: %s", tech),
					Relevance: 0.6,
					Metadata: map[string]string{
						"type":    "tech_stack",
						"project": projectMem.Name,
					},
				})
			}
		}
	}

	return items, nil
}

func (ki *KnowledgeInventory) searchArchivalMemory(ctx context.Context, query string) ([]InventoryItem, error) {
	if ki.archivalSearcher == nil {
		return nil, nil
	}

	archival, err := ki.archivalSearcher.Search(ctx, query, 10)
	if err != nil {
		return nil, err
	}

	items := make([]InventoryItem, 0, len(archival))
	for _, item := range archival {
		items = append(items, InventoryItem{
			ID:        item.ID,
			Source:    "archival_memory",
			Content:   truncate(item.Content, 200),
			Relevance: item.Score,
			Metadata:  item.Metadata,
		})
	}

	return items, nil
}

func sortByRelevance(items []InventoryItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Relevance > items[j].Relevance
	})
}

func calculateConfidence(result *InventoryResult) float64 {
	if result.TotalMatches == 0 {
		return 0.0
	}

	avgRelevance := 0.0
	for _, item := range result.TopResults {
		avgRelevance += item.Relevance
	}
	if len(result.TopResults) > 0 {
		avgRelevance /= float64(len(result.TopResults))
	}

	matchFactor := math.Min(float64(result.TotalMatches)/10.0, 1.0)

	return avgRelevance * matchFactor
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
