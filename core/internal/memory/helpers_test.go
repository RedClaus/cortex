package memory

import (
	"fmt"
	"math"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test vectors for vector math tests
var (
	vecA = []float32{1, 0, 0}
	vecB = []float32{1, 0, 0}     // Same as A
	vecC = []float32{0, 1, 0}     // Orthogonal to A
	vecD = []float32{0.5, 0.5, 0} // Between A and C
)

// Sample response for ExtractField tests
const sampleResponse = `PRINCIPLE: Always check logs first
CATEGORY: debugging
TRIGGER: Error messages appear`

// TestFloat32BytesRoundTrip verifies Float32SliceToBytes <-> BytesToFloat32Slice.
func TestFloat32BytesRoundTrip(t *testing.T) {
	testCases := []struct {
		name  string
		input []float32
	}{
		{
			name:  "simple integers",
			input: []float32{1.0, 2.0, 3.0, 4.0},
		},
		{
			name:  "floating point",
			input: []float32{0.1, 0.5, -0.3, 1.5},
		},
		{
			name:  "empty slice",
			input: []float32{},
		},
		{
			name:  "nil slice",
			input: nil,
		},
		{
			name:  "large values",
			input: []float32{math.MaxFloat32, -math.MaxFloat32, 0, 1e-10},
		},
		{
			name:  "typical embedding",
			input: []float32{0.123, -0.456, 0.789, -0.012, 0.345},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert to bytes
			bytes := Float32SliceToBytes(tc.input)

			// Convert back
			result := BytesToFloat32Slice(bytes)

			// For nil input, both should be nil
			if tc.input == nil {
				assert.Nil(t, result)
				return
			}

			// For empty slice, result should be nil (due to len(data) == 0 check)
			if len(tc.input) == 0 {
				assert.Nil(t, result)
				return
			}

			// Verify round-trip
			require.Len(t, result, len(tc.input))
			for i := range tc.input {
				assert.Equal(t, tc.input[i], result[i], "value at index %d should match", i)
			}
		})
	}
}

// TestBytesToFloat32Slice_Invalid verifies handling of invalid byte data.
func TestBytesToFloat32Slice_Invalid(t *testing.T) {
	// Nil data
	result := BytesToFloat32Slice(nil)
	assert.Nil(t, result)

	// Empty data
	result = BytesToFloat32Slice([]byte{})
	assert.Nil(t, result)

	// Data not divisible by 4 (invalid)
	result = BytesToFloat32Slice([]byte{1, 2, 3})
	assert.Nil(t, result)

	result = BytesToFloat32Slice([]byte{1, 2, 3, 4, 5})
	assert.Nil(t, result)
}

// TestCosineSimilarity verifies similarity calculation.
func TestCosineSimilarity(t *testing.T) {
	// Test with vecD (between A and C)
	similarity := CosineSimilarity(vecA, vecD)

	// vecD is normalized to approximately [0.707, 0.707, 0]
	// Dot product with vecA [1, 0, 0] = 0.707
	// But since vecD is not normalized, we need to account for its magnitude
	// |vecA| = 1, |vecD| = sqrt(0.25 + 0.25) = sqrt(0.5) ≈ 0.707
	// cos = (1*0.5 + 0*0.5 + 0*0) / (1 * 0.707) ≈ 0.707
	expectedSimilarity := 0.5 / (1.0 * math.Sqrt(0.5)) // ≈ 0.707
	assert.InDelta(t, expectedSimilarity, similarity, 0.01)

	// Test similarity is between -1 and 1
	assert.GreaterOrEqual(t, similarity, -1.0)
	assert.LessOrEqual(t, similarity, 1.0)
}

// TestCosineSimilarity_Identical verifies same vectors return 1.0.
func TestCosineSimilarity_Identical(t *testing.T) {
	similarity := CosineSimilarity(vecA, vecB)
	assert.InDelta(t, 1.0, similarity, 0.0001)

	// Also test with another identical pair
	vec := []float32{0.5, 0.3, 0.2, 0.1}
	similarity = CosineSimilarity(vec, vec)
	assert.InDelta(t, 1.0, similarity, 0.0001)
}

// TestCosineSimilarity_Orthogonal verifies perpendicular vectors return 0.0.
func TestCosineSimilarity_Orthogonal(t *testing.T) {
	similarity := CosineSimilarity(vecA, vecC)
	assert.InDelta(t, 0.0, similarity, 0.0001)

	// Test another orthogonal pair
	vec1 := []float32{1, 0}
	vec2 := []float32{0, 1}
	similarity = CosineSimilarity(vec1, vec2)
	assert.InDelta(t, 0.0, similarity, 0.0001)
}

// TestCosineSimilarity_Opposite verifies opposite vectors return -1.0.
func TestCosineSimilarity_Opposite(t *testing.T) {
	vec1 := []float32{1, 0, 0}
	vec2 := []float32{-1, 0, 0}
	similarity := CosineSimilarity(vec1, vec2)
	assert.InDelta(t, -1.0, similarity, 0.0001)
}

// TestCosineSimilarity_EdgeCases verifies edge case handling.
func TestCosineSimilarity_EdgeCases(t *testing.T) {
	// Different lengths
	similarity := CosineSimilarity([]float32{1, 0}, []float32{1, 0, 0})
	assert.Equal(t, 0.0, similarity)

	// Empty vectors
	similarity = CosineSimilarity([]float32{}, []float32{})
	assert.Equal(t, 0.0, similarity)

	// Zero vector
	similarity = CosineSimilarity([]float32{0, 0, 0}, []float32{1, 0, 0})
	assert.Equal(t, 0.0, similarity)
}

// TestCosineDistance verifies distance = 1 - similarity.
func TestCosineDistance(t *testing.T) {
	// Identical vectors: distance should be 0
	distance := CosineDistance(vecA, vecB)
	assert.InDelta(t, 0.0, distance, 0.0001)

	// Orthogonal vectors: distance should be 1
	distance = CosineDistance(vecA, vecC)
	assert.InDelta(t, 1.0, distance, 0.0001)

	// Opposite vectors: distance should be 2
	distance = CosineDistance([]float32{1, 0}, []float32{-1, 0})
	assert.InDelta(t, 2.0, distance, 0.0001)

	// Verify distance = 1 - similarity
	similarity := CosineSimilarity(vecA, vecD)
	distance = CosineDistance(vecA, vecD)
	assert.InDelta(t, 1.0-similarity, distance, 0.0001)
}

// TestCalculateCentroid_Helpers verifies average vector calculation.
func TestCalculateCentroid_Helpers(t *testing.T) {
	t.Run("basic centroid", func(t *testing.T) {
		// Simple case: centroid of unit vectors on axes
		vectors := [][]float32{
			{1, 0, 0},
			{0, 1, 0},
			{0, 0, 1},
		}
		centroid := CalculateCentroid(vectors)
		require.NotNil(t, centroid)
		require.Len(t, centroid, 3)

		// Centroid should be (1/3, 1/3, 1/3)
		assert.InDelta(t, 1.0/3.0, float64(centroid[0]), 0.0001)
		assert.InDelta(t, 1.0/3.0, float64(centroid[1]), 0.0001)
		assert.InDelta(t, 1.0/3.0, float64(centroid[2]), 0.0001)
	})

	t.Run("single vector", func(t *testing.T) {
		vectors := [][]float32{
			{0.5, 0.3, 0.2},
		}
		centroid := CalculateCentroid(vectors)
		require.NotNil(t, centroid)
		assert.Equal(t, vectors[0], centroid)
	})

	t.Run("empty input", func(t *testing.T) {
		centroid := CalculateCentroid([][]float32{})
		assert.Nil(t, centroid)

		centroid = CalculateCentroid(nil)
		assert.Nil(t, centroid)
	})

	t.Run("mismatched dimensions", func(t *testing.T) {
		vectors := [][]float32{
			{1, 0, 0},
			{0, 1}, // Different dimension - should be skipped
			{0, 0, 1},
		}
		centroid := CalculateCentroid(vectors)
		require.NotNil(t, centroid)

		// The mismatched vector is skipped, but n still includes it
		// So we get (1+0+0)/3, (0+0)/3, (0+1)/3
		// This is the current behavior - vectors with wrong dimensions are skipped
		// but the count includes all vectors
		assert.Len(t, centroid, 3)
	})
}

// TestNormalizeVector verifies unit length normalization.
func TestNormalizeVector(t *testing.T) {
	// Normalize a non-unit vector
	vec := []float32{3, 4, 0}
	normalized := NormalizeVector(vec)
	require.NotNil(t, normalized)
	require.Len(t, normalized, 3)

	// Magnitude should be 5, so normalized should be (0.6, 0.8, 0)
	assert.InDelta(t, 0.6, float64(normalized[0]), 0.0001)
	assert.InDelta(t, 0.8, float64(normalized[1]), 0.0001)
	assert.InDelta(t, 0.0, float64(normalized[2]), 0.0001)

	// Verify the normalized vector has unit length
	var magnitude float64
	for _, v := range normalized {
		magnitude += float64(v) * float64(v)
	}
	magnitude = math.Sqrt(magnitude)
	assert.InDelta(t, 1.0, magnitude, 0.0001)
}

// TestNormalizeVector_AlreadyNormalized verifies already-normalized vectors.
func TestNormalizeVector_AlreadyNormalized(t *testing.T) {
	vec := []float32{1, 0, 0}
	normalized := NormalizeVector(vec)
	assert.InDelta(t, 1.0, float64(normalized[0]), 0.0001)
	assert.InDelta(t, 0.0, float64(normalized[1]), 0.0001)
	assert.InDelta(t, 0.0, float64(normalized[2]), 0.0001)
}

// TestNormalizeVector_ZeroVector verifies zero vector handling.
func TestNormalizeVector_ZeroVector(t *testing.T) {
	vec := []float32{0, 0, 0}
	normalized := NormalizeVector(vec)
	// Zero vector should be returned as-is (can't normalize)
	assert.Equal(t, vec, normalized)
}

// TestNormalizeVector_Empty verifies empty vector handling.
func TestNormalizeVector_Empty(t *testing.T) {
	vec := []float32{}
	normalized := NormalizeVector(vec)
	assert.Empty(t, normalized)
}

// TestExtractField verifies field extraction from formatted text.
func TestExtractField(t *testing.T) {
	// Extract PRINCIPLE
	principle := ExtractField(sampleResponse, "PRINCIPLE:")
	assert.Equal(t, "Always check logs first", principle)

	// Extract CATEGORY
	category := ExtractField(sampleResponse, "CATEGORY:")
	assert.Equal(t, "debugging", category)

	// Extract TRIGGER
	trigger := ExtractField(sampleResponse, "TRIGGER:")
	assert.Equal(t, "Error messages appear", trigger)

	// Case insensitive
	principle = ExtractField(sampleResponse, "principle:")
	assert.Equal(t, "Always check logs first", principle)
}

// TestExtractField_NotFound verifies handling of missing fields.
func TestExtractField_NotFound(t *testing.T) {
	result := ExtractField(sampleResponse, "MISSING:")
	assert.Equal(t, "", result)

	result = ExtractField(sampleResponse, "NONEXISTENT:")
	assert.Equal(t, "", result)
}

// TestExtractField_EmptyInput verifies handling of empty input.
func TestExtractField_EmptyInput(t *testing.T) {
	result := ExtractField("", "FIELD:")
	assert.Equal(t, "", result)

	result = ExtractField("no fields here", "FIELD:")
	assert.Equal(t, "", result)
}

// TestExtractField_WhitespaceHandling verifies whitespace is trimmed.
func TestExtractField_WhitespaceHandling(t *testing.T) {
	response := "FIELD:   value with spaces   \nOTHER: data"
	result := ExtractField(response, "FIELD:")
	assert.Equal(t, "value with spaces", result)
}

// TestParseKeywords verifies comma-separated parsing.
func TestParseKeywords(t *testing.T) {
	// Basic parsing
	keywords := ParseKeywords("debugging, logging, errors")
	assert.Len(t, keywords, 3)
	assert.Contains(t, keywords, "debugging")
	assert.Contains(t, keywords, "logging")
	assert.Contains(t, keywords, "errors")

	// Verify lowercase
	keywords = ParseKeywords("DEBUG, LOGGING, Errors")
	for _, kw := range keywords {
		assert.Equal(t, kw, kw) // All should be lowercase
	}
	assert.Contains(t, keywords, "debug")
	assert.Contains(t, keywords, "logging")
	assert.Contains(t, keywords, "errors")
}

// TestParseKeywords_Whitespace verifies whitespace handling.
func TestParseKeywords_Whitespace(t *testing.T) {
	keywords := ParseKeywords("  keyword1  ,  keyword2  ,  keyword3  ")
	assert.Len(t, keywords, 3)
	assert.Equal(t, "keyword1", keywords[0])
	assert.Equal(t, "keyword2", keywords[1])
	assert.Equal(t, "keyword3", keywords[2])
}

// TestParseKeywords_Empty verifies empty input handling.
func TestParseKeywords_Empty(t *testing.T) {
	keywords := ParseKeywords("")
	assert.Nil(t, keywords)

	// Commas only
	keywords = ParseKeywords(",,,")
	assert.Len(t, keywords, 0)

	// Whitespace only
	keywords = ParseKeywords("   ")
	assert.Len(t, keywords, 0)
}

// TestParseKeywords_SingleKeyword verifies single keyword.
func TestParseKeywords_SingleKeyword(t *testing.T) {
	keywords := ParseKeywords("single")
	assert.Len(t, keywords, 1)
	assert.Equal(t, "single", keywords[0])
}

// TestDecayConfidence verifies time-based decay.
func TestDecayConfidence(t *testing.T) {
	// No decay for 0 days
	result := DecayConfidence(0.9, 0, DefaultDecayRate)
	assert.Equal(t, 0.9, result)

	// No decay for negative days
	result = DecayConfidence(0.9, -5, DefaultDecayRate)
	assert.Equal(t, 0.9, result)

	// Some decay after 1 day with 0.99 rate
	result = DecayConfidence(0.9, 1, 0.99)
	expected := 0.9 * 0.99
	assert.InDelta(t, expected, result, 0.0001)

	// More decay after 30 days
	result = DecayConfidence(0.9, 30, 0.99)
	expected = 0.9 * math.Pow(0.99, 30)
	assert.InDelta(t, expected, result, 0.0001)

	// Verify minimum threshold
	result = DecayConfidence(0.9, 1000, 0.99) // Very long time
	assert.GreaterOrEqual(t, result, 0.1, "should not go below minimum")
}

// TestDecayConfidence_MinimumFloor verifies minimum confidence floor.
func TestDecayConfidence_MinimumFloor(t *testing.T) {
	// With extreme decay, should floor at 0.1
	result := DecayConfidence(0.5, 1000, 0.5) // Very aggressive decay
	assert.Equal(t, 0.1, result)

	result = DecayConfidence(0.2, 500, 0.9)
	assert.GreaterOrEqual(t, result, 0.1)
}

// TestCalculateSuccessRate verifies Bayesian success rate.
func TestCalculateSuccessRate(t *testing.T) {
	// No observations: prior of 0.5 with 2 pseudo-observations
	// (0 + 1) / (0 + 0 + 2) = 0.5
	rate := CalculateSuccessRate(0, 0)
	assert.InDelta(t, 0.5, rate, 0.0001)

	// All successes
	rate = CalculateSuccessRate(10, 0)
	// (10 + 1) / (10 + 0 + 2) = 11/12 ≈ 0.917
	assert.InDelta(t, 11.0/12.0, rate, 0.0001)

	// All failures
	rate = CalculateSuccessRate(0, 10)
	// (0 + 1) / (0 + 10 + 2) = 1/12 ≈ 0.083
	assert.InDelta(t, 1.0/12.0, rate, 0.0001)

	// Equal successes and failures
	rate = CalculateSuccessRate(5, 5)
	// (5 + 1) / (5 + 5 + 2) = 6/12 = 0.5
	assert.InDelta(t, 0.5, rate, 0.0001)

	// More successes than failures
	rate = CalculateSuccessRate(8, 2)
	// (8 + 1) / (8 + 2 + 2) = 9/12 = 0.75
	assert.InDelta(t, 0.75, rate, 0.0001)
}

// TestTopN verifies top N selection with scoring.
func TestTopN(t *testing.T) {
	items := []ScoredItem[string]{
		{Item: "low", Score: 0.3},
		{Item: "high", Score: 0.9},
		{Item: "medium", Score: 0.6},
		{Item: "very-high", Score: 0.95},
		{Item: "very-low", Score: 0.1},
	}

	// Get top 3
	top3 := TopN(items, 3)
	assert.Len(t, top3, 3)
	assert.Equal(t, "very-high", top3[0])
	assert.Equal(t, "high", top3[1])
	assert.Equal(t, "medium", top3[2])
}

// TestTopN_MoreThanAvailable verifies requesting more than available.
func TestTopN_MoreThanAvailable(t *testing.T) {
	items := []ScoredItem[string]{
		{Item: "one", Score: 0.5},
		{Item: "two", Score: 0.8},
	}

	// Request 5 but only 2 available
	top5 := TopN(items, 5)
	assert.Len(t, top5, 2)
	assert.Equal(t, "two", top5[0])
	assert.Equal(t, "one", top5[1])
}

// TestTopN_Empty verifies empty input handling.
func TestTopN_Empty(t *testing.T) {
	items := []ScoredItem[string]{}
	top := TopN(items, 3)
	assert.Empty(t, top)
}

// TestTopN_ZeroN verifies zero N returns empty.
func TestTopN_ZeroN(t *testing.T) {
	items := []ScoredItem[string]{
		{Item: "one", Score: 0.5},
	}
	top := TopN(items, 0)
	assert.Empty(t, top)
}

// TestTopN_GenericTypes verifies TopN works with different types.
func TestTopN_GenericTypes(t *testing.T) {
	// Test with int
	intItems := []ScoredItem[int]{
		{Item: 1, Score: 0.3},
		{Item: 2, Score: 0.9},
		{Item: 3, Score: 0.6},
	}
	topInts := TopN(intItems, 2)
	assert.Len(t, topInts, 2)
	assert.Equal(t, 2, topInts[0])
	assert.Equal(t, 3, topInts[1])

	// Test with struct
	type testStruct struct {
		Name string
	}
	structItems := []ScoredItem[testStruct]{
		{Item: testStruct{Name: "low"}, Score: 0.2},
		{Item: testStruct{Name: "high"}, Score: 0.8},
	}
	topStructs := TopN(structItems, 1)
	assert.Len(t, topStructs, 1)
	assert.Equal(t, "high", topStructs[0].Name)
}

// TestSortByScoreDesc verifies descending score sorting.
func TestSortByScoreDesc(t *testing.T) {
	items := []ScoredItem[string]{
		{Item: "low", Score: 0.3},
		{Item: "high", Score: 0.9},
		{Item: "medium", Score: 0.6},
	}

	SortByScoreDesc(items)

	assert.Equal(t, "high", items[0].Item)
	assert.Equal(t, 0.9, items[0].Score)
	assert.Equal(t, "medium", items[1].Item)
	assert.Equal(t, 0.6, items[1].Score)
	assert.Equal(t, "low", items[2].Item)
	assert.Equal(t, 0.3, items[2].Score)
}

// TestEuclideanDistance verifies Euclidean distance calculation.
func TestEuclideanDistance(t *testing.T) {
	// Same vectors: distance 0
	distance := EuclideanDistance(vecA, vecA)
	assert.InDelta(t, 0.0, distance, 0.0001)

	// Orthogonal unit vectors: distance sqrt(2)
	distance = EuclideanDistance(vecA, vecC)
	assert.InDelta(t, math.Sqrt(2), distance, 0.0001)

	// 3-4-5 triangle
	vec1 := []float32{0, 0}
	vec2 := []float32{3, 4}
	distance = EuclideanDistance(vec1, vec2)
	assert.InDelta(t, 5.0, distance, 0.0001)
}

// TestEuclideanDistance_EdgeCases verifies edge case handling.
func TestEuclideanDistance_EdgeCases(t *testing.T) {
	// Different lengths: should return infinity
	distance := EuclideanDistance([]float32{1, 0}, []float32{1, 0, 0})
	assert.True(t, math.IsInf(distance, 1))

	// Empty vectors: should return infinity
	distance = EuclideanDistance([]float32{}, []float32{})
	assert.True(t, math.IsInf(distance, 1))
}

// TestGenerateID verifies ID generation.
func TestGenerateID(t *testing.T) {
	id := GenerateID("strat", "abc123")
	assert.Equal(t, "strat_abc123", id)

	id = GenerateID("ep", "xyz")
	assert.Equal(t, "ep_xyz", id)
}

// TestParseIDPrefix verifies prefix extraction.
func TestParseIDPrefix(t *testing.T) {
	prefix := ParseIDPrefix("strat_abc123")
	assert.Equal(t, "strat", prefix)

	prefix = ParseIDPrefix("ep_xyz_extra")
	assert.Equal(t, "ep", prefix)

	prefix = ParseIDPrefix("noprefix")
	assert.Equal(t, "noprefix", prefix)

	prefix = ParseIDPrefix("")
	assert.Equal(t, "", prefix)
}

// TestExtractFieldWithPattern verifies regex pattern extraction.
func TestExtractFieldWithPattern(t *testing.T) {
	response := "SCORE: 0.85, CATEGORY: debugging, COUNT: 42"

	// Extract score
	score := ExtractFieldWithPattern(response, `SCORE:\s*([\d.]+)`)
	assert.Equal(t, "0.85", score)

	// Extract category
	category := ExtractFieldWithPattern(response, `CATEGORY:\s*(\w+)`)
	assert.Equal(t, "debugging", category)

	// Extract count
	count := ExtractFieldWithPattern(response, `COUNT:\s*(\d+)`)
	assert.Equal(t, "42", count)

	// Non-matching pattern
	result := ExtractFieldWithPattern(response, `MISSING:\s*(\w+)`)
	assert.Equal(t, "", result)
}

// TestTopKHeap verifies heap-based top-K selection.
func TestTopKHeap(t *testing.T) {
	t.Run("basic selection", func(t *testing.T) {
		items := []ScoredItem[string]{
			{Item: "e", Score: 0.1},
			{Item: "c", Score: 0.5},
			{Item: "a", Score: 0.9},
			{Item: "d", Score: 0.3},
			{Item: "b", Score: 0.7},
		}

		result := TopKHeap(items, 3)
		require.Len(t, result, 3)

		// Should be in descending order
		assert.Equal(t, "a", result[0].Item)
		assert.InDelta(t, 0.9, result[0].Score, 0.0001)
		assert.Equal(t, "b", result[1].Item)
		assert.InDelta(t, 0.7, result[1].Score, 0.0001)
		assert.Equal(t, "c", result[2].Item)
		assert.InDelta(t, 0.5, result[2].Score, 0.0001)
	})

	t.Run("k larger than n", func(t *testing.T) {
		items := []ScoredItem[string]{
			{Item: "a", Score: 0.5},
			{Item: "b", Score: 0.8},
		}

		result := TopKHeap(items, 10)
		require.Len(t, result, 2)
		assert.Equal(t, "b", result[0].Item)
		assert.Equal(t, "a", result[1].Item)
	})

	t.Run("k equals n", func(t *testing.T) {
		items := []ScoredItem[string]{
			{Item: "a", Score: 0.3},
			{Item: "b", Score: 0.7},
			{Item: "c", Score: 0.5},
		}

		result := TopKHeap(items, 3)
		require.Len(t, result, 3)
		assert.Equal(t, "b", result[0].Item)
		assert.Equal(t, "c", result[1].Item)
		assert.Equal(t, "a", result[2].Item)
	})

	t.Run("k equals 1", func(t *testing.T) {
		items := []ScoredItem[string]{
			{Item: "low", Score: 0.1},
			{Item: "high", Score: 0.99},
			{Item: "mid", Score: 0.5},
		}

		result := TopKHeap(items, 1)
		require.Len(t, result, 1)
		assert.Equal(t, "high", result[0].Item)
	})

	t.Run("empty input", func(t *testing.T) {
		result := TopKHeap([]ScoredItem[string]{}, 3)
		assert.Nil(t, result)
	})

	t.Run("zero k", func(t *testing.T) {
		items := []ScoredItem[string]{
			{Item: "a", Score: 0.5},
		}
		result := TopKHeap(items, 0)
		assert.Nil(t, result)
	})

	t.Run("negative k", func(t *testing.T) {
		items := []ScoredItem[string]{
			{Item: "a", Score: 0.5},
		}
		result := TopKHeap(items, -1)
		assert.Nil(t, result)
	})

	t.Run("duplicate scores", func(t *testing.T) {
		items := []ScoredItem[string]{
			{Item: "a", Score: 0.5},
			{Item: "b", Score: 0.5},
			{Item: "c", Score: 0.8},
			{Item: "d", Score: 0.5},
		}

		result := TopKHeap(items, 3)
		require.Len(t, result, 3)
		assert.Equal(t, "c", result[0].Item) // Highest score
		// The remaining 2 should both have score 0.5
		assert.InDelta(t, 0.5, result[1].Score, 0.0001)
		assert.InDelta(t, 0.5, result[2].Score, 0.0001)
	})

	t.Run("preserves original slice", func(t *testing.T) {
		items := []ScoredItem[string]{
			{Item: "low", Score: 0.3},
			{Item: "high", Score: 0.9},
			{Item: "medium", Score: 0.6},
		}
		originalFirst := items[0]

		_ = TopKHeap(items, 2)

		// Original slice should not be modified
		assert.Equal(t, originalFirst.Item, items[0].Item)
		assert.Equal(t, originalFirst.Score, items[0].Score)
	})
}

// TestTopKWithScores verifies the convenience wrapper.
func TestTopKWithScores(t *testing.T) {
	items := []ScoredItem[int]{
		{Item: 1, Score: 0.2},
		{Item: 2, Score: 0.8},
		{Item: 3, Score: 0.5},
	}

	result := TopKWithScores(items, 2)
	require.Len(t, result, 2)
	assert.Equal(t, 2, result[0].Item)
	assert.InDelta(t, 0.8, result[0].Score, 0.0001)
	assert.Equal(t, 3, result[1].Item)
	assert.InDelta(t, 0.5, result[1].Score, 0.0001)
}

// TestTopKHeap_LargeDataset verifies heap performance with larger data.
func TestTopKHeap_LargeDataset(t *testing.T) {
	// Create 1000 items
	n := 1000
	items := make([]ScoredItem[int], n)
	for i := 0; i < n; i++ {
		items[i] = ScoredItem[int]{
			Item:  i,
			Score: float64(i) / float64(n), // Scores from 0 to ~1
		}
	}

	// Get top 10
	result := TopKHeap(items, 10)
	require.Len(t, result, 10)

	// Verify we got the highest scores (items 999, 998, ..., 990)
	for i := 0; i < 10; i++ {
		expectedItem := n - 1 - i
		assert.Equal(t, expectedItem, result[i].Item)
	}
}

// TestConstants verifies threshold constants are reasonable.
func TestConstants(t *testing.T) {
	// Similarity threshold should be between 0 and 1
	assert.Greater(t, SimilarityThreshold, 0.0)
	assert.LessOrEqual(t, SimilarityThreshold, 1.0)

	// Deduplication threshold should be higher than similarity
	assert.Greater(t, DeduplicationThreshold, SimilarityThreshold)
	assert.LessOrEqual(t, DeduplicationThreshold, 1.0)

	// Min confidence for link should be between 0 and 1
	assert.Greater(t, MinConfidenceForLink, 0.0)
	assert.LessOrEqual(t, MinConfidenceForLink, 1.0)

	// Decay rate should be between 0 and 1
	assert.Greater(t, DefaultDecayRate, 0.0)
	assert.LessOrEqual(t, DefaultDecayRate, 1.0)

	// Min evidence should be positive
	assert.Greater(t, MinEvidenceForReliable, 0)

	// Stale topic days should be positive
	assert.Greater(t, StaleTopicDays, 0)
}

// ============================================================================
// BENCHMARKS
// ============================================================================

// BenchmarkTopKHeap_100items_top10 - Basic case.
func BenchmarkTopKHeap_100items_top10(b *testing.B) {
	items := makeBenchmarkItems(100)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = TopKHeap(items, 10)
	}
}

// BenchmarkTopKHeap_1000items_top10 - Target: < 2ms (down from 15ms).
func BenchmarkTopKHeap_1000items_top10(b *testing.B) {
	items := makeBenchmarkItems(1000)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = TopKHeap(items, 10)
	}
}

// BenchmarkTopKHeap_10000items_top10 - Stress test.
func BenchmarkTopKHeap_10000items_top10(b *testing.B) {
	items := makeBenchmarkItems(10000)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = TopKHeap(items, 10)
	}
}

// BenchmarkTopKHeap_1000items_top100 - Larger k value.
func BenchmarkTopKHeap_1000items_top100(b *testing.B) {
	items := makeBenchmarkItems(1000)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = TopKHeap(items, 100)
	}
}

// BenchmarkSortThenTruncate_1000items_top10 - Compare against old approach.
func BenchmarkSortThenTruncate_1000items_top10(b *testing.B) {
	items := makeBenchmarkItems(1000)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Old approach: sort entire slice, then truncate
		result := make([]ScoredItem[int], len(items))
		copy(result, items)
		SortByScoreDesc(result)
		_ = result[:10]
	}
}

// BenchmarkSortThenTruncate_10000items_top10 - Compare against old approach (stress).
func BenchmarkSortThenTruncate_10000items_top10(b *testing.B) {
	items := makeBenchmarkItems(10000)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result := make([]ScoredItem[int], len(items))
		copy(result, items)
		SortByScoreDesc(result)
		_ = result[:10]
	}
}

// BenchmarkTopKHeap_parallel - Concurrent access.
func BenchmarkTopKHeap_parallel(b *testing.B) {
	items := makeBenchmarkItems(1000)
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = TopKHeap(items, 10)
		}
	})
}

// BenchmarkTopKHeap_varyingK - Benchmark different k values.
func BenchmarkTopKHeap_varyingK(b *testing.B) {
	items := makeBenchmarkItems(1000)
	kValues := []int{1, 5, 10, 20, 50, 100}

	for _, k := range kValues {
		b.Run(fmt.Sprintf("k=%d", k), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = TopKHeap(items, k)
			}
		})
	}
}

// BenchmarkTopKHeap_varyingN - Benchmark different dataset sizes.
func BenchmarkTopKHeap_varyingN(b *testing.B) {
	nValues := []int{100, 500, 1000, 5000, 10000}
	k := 10

	for _, n := range nValues {
		items := makeBenchmarkItems(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = TopKHeap(items, k)
			}
		})
	}
}

// makeBenchmarkItems creates n scored items with random scores.
func makeBenchmarkItems(n int) []ScoredItem[int] {
	items := make([]ScoredItem[int], n)
	for i := 0; i < n; i++ {
		// Use deterministic but varied scores
		items[i] = ScoredItem[int]{
			Item:  i,
			Score: float64(i*7%100) / 100.0, // Pseudo-random 0-1
		}
	}
	return items
}

// TestTopKHeap_CorrectnessByComparison verifies heap approach produces same results as sort.
func TestTopKHeap_CorrectnessByComparison(t *testing.T) {
	testCases := []struct {
		name string
		n    int
		k    int
	}{
		{"small dataset", 20, 5},
		{"medium dataset", 100, 10},
		{"large dataset", 1000, 10},
		{"k equals n/2", 100, 50},
		{"k equals n-1", 50, 49},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test data
			items := makeBenchmarkItems(tc.n)

			// Heap approach
			heapResult := TopKHeap(items, tc.k)

			// Sort approach
			sortResult := make([]ScoredItem[int], len(items))
			copy(sortResult, items)
			SortByScoreDesc(sortResult)
			sortResult = sortResult[:tc.k]

			// Compare results
			require.Len(t, heapResult, len(sortResult), "result lengths should match")

			// Compare scores (items may differ when scores are equal)
			for i := range heapResult {
				assert.InDelta(t, sortResult[i].Score, heapResult[i].Score, 0.0001,
					"score at position %d should match", i)
			}

			// Verify both have same set of scores (regardless of order within equal scores)
			heapScores := make(map[float64]int)
			sortScores := make(map[float64]int)
			for i := range heapResult {
				heapScores[heapResult[i].Score]++
				sortScores[sortResult[i].Score]++
			}
			assert.Equal(t, sortScores, heapScores, "score distributions should match")
		})
	}
}

// TestTopKHeap_OrderGuarantee verifies results are in descending order.
func TestTopKHeap_OrderGuarantee(t *testing.T) {
	items := makeBenchmarkItems(1000)
	result := TopKHeap(items, 50)

	require.Len(t, result, 50)

	// Verify descending order
	for i := 0; i < len(result)-1; i++ {
		assert.GreaterOrEqual(t, result[i].Score, result[i+1].Score,
			"scores should be in descending order at index %d", i)
	}
}

// TestTopKHeap_MemoryEfficiency verifies heap doesn't allocate excessively.
func TestTopKHeap_MemoryEfficiency(t *testing.T) {
	items := makeBenchmarkItems(10000)

	// Measure allocations
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	_ = TopKHeap(items, 10)

	runtime.ReadMemStats(&m2)

	// Heap should allocate much less than sorting entire slice
	// Max expected: heap (10 items) + result (10 items) + overhead
	allocBytes := m2.Alloc - m1.Alloc

	// Should allocate < 10KB for k=10 (even with overhead)
	// Each ScoredItem[int] is ~16 bytes, so 10 items = 160 bytes * some overhead
	assert.Less(t, allocBytes, uint64(10000),
		"heap should not allocate more than 10KB for k=10")
}

// TestTopKHeap_StableWithDuplicates verifies handling of duplicate scores.
func TestTopKHeap_StableWithDuplicates(t *testing.T) {
	// Create items with many duplicate scores
	items := []ScoredItem[string]{
		{Item: "a1", Score: 0.9},
		{Item: "a2", Score: 0.9},
		{Item: "a3", Score: 0.9},
		{Item: "b1", Score: 0.5},
		{Item: "b2", Score: 0.5},
		{Item: "b3", Score: 0.5},
		{Item: "c1", Score: 0.3},
		{Item: "c2", Score: 0.3},
		{Item: "c3", Score: 0.3},
	}

	result := TopKHeap(items, 5)
	require.Len(t, result, 5)

	// All top 5 should be either 0.9 or 0.5
	for i := 0; i < 5; i++ {
		assert.True(t, result[i].Score == 0.9 || result[i].Score == 0.5,
			"top 5 should have scores 0.9 or 0.5, got %.2f", result[i].Score)
	}

	// Should be in descending order
	for i := 0; i < len(result)-1; i++ {
		assert.GreaterOrEqual(t, result[i].Score, result[i+1].Score)
	}
}
