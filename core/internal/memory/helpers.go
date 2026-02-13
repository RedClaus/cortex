package memory

import (
	"container/heap"
	"encoding/binary"
	"math"
	"regexp"
	"sort"
	"strings"
)

// ============================================================================
// EMBEDDING HELPERS
// ============================================================================

// Float32SliceToBytes converts a float32 slice to bytes for SQLite BLOB storage.
func Float32SliceToBytes(slice []float32) []byte {
	if slice == nil {
		return nil
	}
	buf := make([]byte, len(slice)*4)
	for i, v := range slice {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// BytesToFloat32Slice converts bytes from SQLite BLOB to float32 slice.
func BytesToFloat32Slice(data []byte) []float32 {
	if data == nil || len(data) == 0 {
		return nil
	}
	if len(data)%4 != 0 {
		return nil
	}
	result := make([]float32, len(data)/4)
	for i := range result {
		bits := binary.LittleEndian.Uint32(data[i*4:])
		result[i] = math.Float32frombits(bits)
	}
	return result
}

// ============================================================================
// VECTOR MATH
// ============================================================================

// CosineSimilarity calculates the cosine similarity between two vectors.
// Returns a value between -1 and 1, where 1 means identical direction.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// CosineDistance returns 1 - cosine_similarity (so smaller = more similar).
func CosineDistance(a, b []float32) float64 {
	return 1.0 - CosineSimilarity(a, b)
}

// EuclideanDistance calculates the Euclidean distance between two vectors.
func EuclideanDistance(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return math.Inf(1)
	}

	var sum float64
	for i := range a {
		diff := float64(a[i]) - float64(b[i])
		sum += diff * diff
	}

	return math.Sqrt(sum)
}

// CalculateCentroid computes the average vector (centroid) of a set of vectors.
func CalculateCentroid(vectors [][]float32) []float32 {
	if len(vectors) == 0 {
		return nil
	}

	dim := len(vectors[0])
	centroid := make([]float32, dim)

	for _, v := range vectors {
		if len(v) != dim {
			continue
		}
		for i, val := range v {
			centroid[i] += val
		}
	}

	n := float32(len(vectors))
	for i := range centroid {
		centroid[i] /= n
	}

	return centroid
}

// NormalizeVector normalizes a vector to unit length.
func NormalizeVector(v []float32) []float32 {
	if len(v) == 0 {
		return v
	}

	var norm float64
	for _, val := range v {
		norm += float64(val) * float64(val)
	}
	norm = math.Sqrt(norm)

	if norm == 0 {
		return v
	}

	result := make([]float32, len(v))
	for i, val := range v {
		result[i] = float32(float64(val) / norm)
	}
	return result
}

// ============================================================================
// STRING EXTRACTION HELPERS
// ============================================================================

// ExtractField extracts a field value from a formatted response.
// Example: ExtractField("PRINCIPLE: Do X before Y\nCATEGORY: debugging", "PRINCIPLE:")
// Returns: "Do X before Y"
func ExtractField(response, field string) string {
	lines := strings.Split(response, "\n")
	fieldUpper := strings.ToUpper(field)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		lineUpper := strings.ToUpper(line)

		if strings.HasPrefix(lineUpper, fieldUpper) {
			value := line[len(field):]
			return strings.TrimSpace(value)
		}
	}

	return ""
}

// ExtractFieldWithPattern extracts a field using a regex pattern.
func ExtractFieldWithPattern(response, pattern string) string {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// ParseKeywords parses a comma-separated keyword string into a slice.
func ParseKeywords(keywordsStr string) []string {
	if keywordsStr == "" {
		return nil
	}

	parts := strings.Split(keywordsStr, ",")
	keywords := make([]string, 0, len(parts))

	for _, part := range parts {
		kw := strings.TrimSpace(part)
		if kw != "" {
			keywords = append(keywords, strings.ToLower(kw))
		}
	}

	return keywords
}

// ============================================================================
// SCORING HELPERS
// ============================================================================

// ScoredItem represents an item with a similarity/relevance score.
type ScoredItem[T any] struct {
	Item  T
	Score float64
}

// ============================================================================
// MIN-HEAP FOR TOP-K SELECTION
// ============================================================================

// scoredItemHeap is a min-heap implementation for top-K selection.
// It maintains the K highest-scoring items by using a min-heap,
// allowing O(log k) insertion and O(1) access to the minimum score.
//
// Complexity: O(n log k) for processing n items to find top k,
// compared to O(n log n) for sort-then-truncate.
type scoredItemHeap[T any] []ScoredItem[T]

func (h scoredItemHeap[T]) Len() int           { return len(h) }
func (h scoredItemHeap[T]) Less(i, j int) bool { return h[i].Score < h[j].Score } // Min-heap: smallest score at root
func (h scoredItemHeap[T]) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *scoredItemHeap[T]) Push(x any) {
	*h = append(*h, x.(ScoredItem[T]))
}

func (h *scoredItemHeap[T]) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// TopKHeap efficiently finds the top K highest-scoring items using a min-heap.
// This is O(n log k) compared to O(n log n) for sorting, which is significantly
// faster when k << n (typical for similarity search where k=3-10 and n=1000+).
//
// The result is returned in descending order (highest score first).
func TopKHeap[T any](items []ScoredItem[T], k int) []ScoredItem[T] {
	if k <= 0 || len(items) == 0 {
		return nil
	}

	// For very small datasets or when k >= n, just sort
	// The heap overhead isn't worth it for small slices
	if len(items) <= k || k >= len(items) {
		result := make([]ScoredItem[T], len(items))
		copy(result, items)
		// Sort descending by score
		for i := 0; i < len(result)-1; i++ {
			for j := i + 1; j < len(result); j++ {
				if result[j].Score > result[i].Score {
					result[i], result[j] = result[j], result[i]
				}
			}
		}
		return result
	}

	// Initialize min-heap with first k items
	h := make(scoredItemHeap[T], k)
	copy(h, items[:k])
	heap.Init(&h)

	// Process remaining items: if score > min in heap, replace min
	for i := k; i < len(items); i++ {
		if items[i].Score > h[0].Score {
			heap.Pop(&h)
			heap.Push(&h, items[i])
		}
	}

	// Extract items in descending order (reverse of heap order)
	result := make([]ScoredItem[T], len(h))
	for i := len(h) - 1; i >= 0; i-- {
		result[i] = heap.Pop(&h).(ScoredItem[T])
	}

	return result
}

// TopN returns the top N items from a scored list.
// Uses min-heap for O(n log k) performance instead of O(n log n) sorting.
func TopN[T any](items []ScoredItem[T], n int) []T {
	topK := TopKHeap(items, n)
	result := make([]T, len(topK))
	for i, item := range topK {
		result[i] = item.Item
	}
	return result
}

// TopKWithScores returns the top K items with their scores in descending order.
// This is the preferred method when you need both items and scores.
func TopKWithScores[T any](items []ScoredItem[T], k int) []ScoredItem[T] {
	return TopKHeap(items, k)
}

// SortByScoreDesc sorts scored items by score in descending order (in-place).
// Deprecated: Prefer TopKWithScores for top-K selection (O(n log k) vs O(n log n)).
// This function is retained for backward compatibility.
func SortByScoreDesc[T any](items []ScoredItem[T]) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
}

// ============================================================================
// CONFIDENCE/DECAY HELPERS
// ============================================================================

// DecayConfidence applies time-based decay to a confidence score.
// decayRate is the daily decay factor (e.g., 0.99 means 1% decay per day).
func DecayConfidence(confidence float64, daysSinceLastUse int, decayRate float64) float64 {
	if daysSinceLastUse <= 0 {
		return confidence
	}

	decayed := confidence * math.Pow(decayRate, float64(daysSinceLastUse))

	// Don't go below a minimum threshold
	if decayed < 0.1 {
		return 0.1
	}
	return decayed
}

// CalculateSuccessRate calculates success rate with Bayesian smoothing.
// Uses a prior of 0.5 with 2 pseudo-observations.
func CalculateSuccessRate(successes, failures int) float64 {
	// Bayesian estimate with Beta(1,1) prior (uniform)
	// Posterior mean = (successes + 1) / (successes + failures + 2)
	return float64(successes+1) / float64(successes+failures+2)
}

// ============================================================================
// ID GENERATION HELPERS
// ============================================================================

// GenerateID generates a prefixed ID for a memory type.
// Example: GenerateID("strat", "abc123") returns "strat_abc123"
func GenerateID(prefix, suffix string) string {
	return prefix + "_" + suffix
}

// ParseIDPrefix extracts the prefix from a generated ID.
func ParseIDPrefix(id string) string {
	parts := strings.SplitN(id, "_", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// ============================================================================
// THRESHOLD CONSTANTS
// ============================================================================

const (
	// SimilarityThreshold is the minimum similarity for considering memories related.
	SimilarityThreshold = 0.7

	// DeduplicationThreshold is the minimum similarity for considering memories duplicates.
	DeduplicationThreshold = 0.85

	// MinEvidenceForReliable is the minimum observations for a principle to be "reliable".
	MinEvidenceForReliable = 3

	// MinConfidenceForLink is the minimum confidence for auto-creating a link.
	MinConfidenceForLink = 0.6

	// DefaultDecayRate is the default daily confidence decay rate.
	DefaultDecayRate = 0.99

	// StaleTopicDays is the number of days after which a topic is considered stale.
	StaleTopicDays = 30
)
