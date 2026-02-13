package router

import (
	"container/heap"
	"sort"
	"sync"

	"github.com/normanking/cortex/internal/cognitive"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MIN-HEAP FOR TOP-K SELECTION
// ═══════════════════════════════════════════════════════════════════════════════

// searchResultHeap is a min-heap for efficient top-K selection.
// Maintains K highest scores by keeping smallest at root.
// Complexity: O(n log k) vs O(n log n) for sort-then-truncate.
type searchResultHeap []SearchResult

func (h searchResultHeap) Len() int           { return len(h) }
func (h searchResultHeap) Less(i, j int) bool { return h[i].Score < h[j].Score } // Min-heap
func (h searchResultHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *searchResultHeap) Push(x any) {
	*h = append(*h, x.(SearchResult))
}

func (h *searchResultHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// topKSearchResults returns the top K search results using a min-heap.
// Results are returned in descending order (highest score first).
func topKSearchResults(results []SearchResult, k int) []SearchResult {
	if k <= 0 || len(results) == 0 {
		return nil
	}

	if len(results) <= k {
		// Sort small slice directly (heap overhead not worth it)
		sorted := make([]SearchResult, len(results))
		copy(sorted, results)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].Score > sorted[i].Score {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		return sorted
	}

	// Initialize min-heap with first k items
	h := make(searchResultHeap, k)
	copy(h, results[:k])
	heap.Init(&h)

	// Process remaining: replace min if current score is higher
	for i := k; i < len(results); i++ {
		if results[i].Score > h[0].Score {
			heap.Pop(&h)
			heap.Push(&h, results[i])
		}
	}

	// Extract in descending order
	output := make([]SearchResult, len(h))
	for i := len(h) - 1; i >= 0; i-- {
		output[i] = heap.Pop(&h).(SearchResult)
	}

	return output
}

// ═══════════════════════════════════════════════════════════════════════════════
// EMBEDDING INDEX
// ═══════════════════════════════════════════════════════════════════════════════

// IndexEntry represents a single entry in the embedding index.
type IndexEntry struct {
	ID        string              // Template ID or other identifier
	Embedding cognitive.Embedding // Normalized embedding vector
	Metadata  interface{}         // Optional associated data
}

// SearchResult represents a similarity search result.
type SearchResult struct {
	ID         string      // Entry ID
	Score      float64     // Cosine similarity score (0-1)
	Metadata   interface{} // Associated metadata
}

// EmbeddingIndex provides fast in-memory similarity search.
// It uses brute-force cosine similarity which is efficient for small to medium
// collections (< 10,000 entries). For larger collections, consider using
// approximate nearest neighbor algorithms (HNSW, etc.).
type EmbeddingIndex struct {
	mu      sync.RWMutex
	entries map[string]*IndexEntry
	ordered []*IndexEntry // For deterministic iteration
}

// NewEmbeddingIndex creates a new empty index.
func NewEmbeddingIndex() *EmbeddingIndex {
	return &EmbeddingIndex{
		entries: make(map[string]*IndexEntry),
		ordered: make([]*IndexEntry, 0),
	}
}

// Add inserts or updates an entry in the index.
// The embedding is normalized before storage.
func (idx *EmbeddingIndex) Add(id string, embedding cognitive.Embedding, metadata interface{}) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Normalize the embedding for faster cosine similarity computation
	normalized := embedding.Normalize()

	entry := &IndexEntry{
		ID:        id,
		Embedding: normalized,
		Metadata:  metadata,
	}

	// Check if updating existing entry
	if existing, exists := idx.entries[id]; exists {
		existing.Embedding = normalized
		existing.Metadata = metadata
	} else {
		idx.entries[id] = entry
		idx.ordered = append(idx.ordered, entry)
	}
}

// Remove deletes an entry from the index.
func (idx *EmbeddingIndex) Remove(id string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, exists := idx.entries[id]; !exists {
		return false
	}

	delete(idx.entries, id)

	// Remove from ordered slice
	for i, e := range idx.ordered {
		if e.ID == id {
			idx.ordered = append(idx.ordered[:i], idx.ordered[i+1:]...)
			break
		}
	}

	return true
}

// Get retrieves an entry by ID.
func (idx *EmbeddingIndex) Get(id string) (*IndexEntry, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	entry, exists := idx.entries[id]
	return entry, exists
}

// Search finds the k most similar entries to the query embedding.
// Returns results sorted by similarity score (highest first).
// Uses min-heap for O(n log k) complexity instead of O(n log n) sorting.
func (idx *EmbeddingIndex) Search(query cognitive.Embedding, k int) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.ordered) == 0 || k <= 0 {
		return nil
	}

	// Normalize query for consistent comparison
	normalizedQuery := query.Normalize()

	// Calculate similarities for all entries
	results := make([]SearchResult, 0, len(idx.ordered))
	for _, entry := range idx.ordered {
		score := normalizedQuery.CosineSimilarity(entry.Embedding)
		results = append(results, SearchResult{
			ID:       entry.ID,
			Score:    score,
			Metadata: entry.Metadata,
		})
	}

	// Use min-heap for efficient top-K selection: O(n log k) vs O(n log n)
	return topKSearchResults(results, k)
}

// SearchWithThreshold finds entries with similarity >= threshold.
// Results are sorted by similarity score (highest first).
func (idx *EmbeddingIndex) SearchWithThreshold(query cognitive.Embedding, threshold float64) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.ordered) == 0 {
		return nil
	}

	// Normalize query
	normalizedQuery := query.Normalize()

	// Find all entries above threshold
	var results []SearchResult
	for _, entry := range idx.ordered {
		score := normalizedQuery.CosineSimilarity(entry.Embedding)
		if score >= threshold {
			results = append(results, SearchResult{
				ID:       entry.ID,
				Score:    score,
				Metadata: entry.Metadata,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// Size returns the number of entries in the index.
func (idx *EmbeddingIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}

// Clear removes all entries from the index.
func (idx *EmbeddingIndex) Clear() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.entries = make(map[string]*IndexEntry)
	idx.ordered = make([]*IndexEntry, 0)
}

// IDs returns all entry IDs in the index.
func (idx *EmbeddingIndex) IDs() []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	ids := make([]string, len(idx.ordered))
	for i, entry := range idx.ordered {
		ids[i] = entry.ID
	}
	return ids
}

// ═══════════════════════════════════════════════════════════════════════════════
// BATCH OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// BatchAdd inserts multiple entries efficiently.
func (idx *EmbeddingIndex) BatchAdd(entries []struct {
	ID        string
	Embedding cognitive.Embedding
	Metadata  interface{}
}) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for _, e := range entries {
		normalized := e.Embedding.Normalize()
		entry := &IndexEntry{
			ID:        e.ID,
			Embedding: normalized,
			Metadata:  e.Metadata,
		}

		if existing, exists := idx.entries[e.ID]; exists {
			existing.Embedding = normalized
			existing.Metadata = e.Metadata
		} else {
			idx.entries[e.ID] = entry
			idx.ordered = append(idx.ordered, entry)
		}
	}
}

// BatchRemove deletes multiple entries efficiently.
func (idx *EmbeddingIndex) BatchRemove(ids []string) int {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	removed := 0
	toRemove := make(map[string]bool)

	for _, id := range ids {
		if _, exists := idx.entries[id]; exists {
			delete(idx.entries, id)
			toRemove[id] = true
			removed++
		}
	}

	// Rebuild ordered slice without removed entries
	if removed > 0 {
		newOrdered := make([]*IndexEntry, 0, len(idx.ordered)-removed)
		for _, e := range idx.ordered {
			if !toRemove[e.ID] {
				newOrdered = append(newOrdered, e)
			}
		}
		idx.ordered = newOrdered
	}

	return removed
}
