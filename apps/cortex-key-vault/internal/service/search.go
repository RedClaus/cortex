package service

import (
	"sort"
	"strings"

	"github.com/normanking/cortex-key-vault/internal/storage"
	"github.com/sahilm/fuzzy"
)

// SearchResult represents a fuzzy search match
type SearchResult struct {
	Secret       storage.Secret
	Score        int
	MatchedChars []int // Indices of matched characters
}

// SearchService provides fuzzy search capabilities
type SearchService struct {
	vault *VaultService
}

// NewSearchService creates a new search service
func NewSearchService(vault *VaultService) *SearchService {
	return &SearchService{vault: vault}
}

// secretSource wraps secrets for fuzzy matching
type secretSource struct {
	secrets []storage.Secret
}

func (s secretSource) String(i int) string {
	return strings.ToLower(s.secrets[i].Name)
}

func (s secretSource) Len() int {
	return len(s.secrets)
}

// FuzzySearch performs fuzzy search on secrets
func (s *SearchService) FuzzySearch(query string) ([]SearchResult, error) {
	if query == "" {
		return nil, nil
	}

	secrets, err := s.vault.ListSecrets("")
	if err != nil {
		return nil, err
	}

	if len(secrets) == 0 {
		return nil, nil
	}

	source := secretSource{secrets: secrets}
	matches := fuzzy.FindFrom(strings.ToLower(query), source)

	results := make([]SearchResult, len(matches))
	for i, match := range matches {
		results[i] = SearchResult{
			Secret:       secrets[match.Index],
			Score:        match.Score,
			MatchedChars: match.MatchedIndexes,
		}
	}

	return results, nil
}

// FilterByCategory filters secrets by category ID
func (s *SearchService) FilterByCategory(categoryID string) ([]storage.Secret, error) {
	return s.vault.ListSecrets(categoryID)
}

// FilterByType filters secrets by type
func (s *SearchService) FilterByType(secretType storage.SecretType) ([]storage.Secret, error) {
	return s.vault.ListSecretsByType(secretType)
}

// FilterByTag filters secrets by tag
func (s *SearchService) FilterByTag(tag string) ([]storage.Secret, error) {
	return s.vault.ListSecretsByTag(tag)
}

// CombinedSearch performs a combined search with multiple filters
func (s *SearchService) CombinedSearch(query, categoryID string, secretType *storage.SecretType, tag string) ([]SearchResult, error) {
	// Start with all secrets or fuzzy search results
	var candidates []storage.Secret
	var err error

	if query != "" {
		results, err := s.FuzzySearch(query)
		if err != nil {
			return nil, err
		}
		for _, r := range results {
			candidates = append(candidates, r.Secret)
		}
	} else {
		candidates, err = s.vault.ListSecrets("")
		if err != nil {
			return nil, err
		}
	}

	// Apply filters
	var filtered []storage.Secret
	for _, secret := range candidates {
		// Category filter
		if categoryID != "" && categoryID != "all" && secret.CategoryID != categoryID {
			continue
		}

		// Type filter
		if secretType != nil && secret.Type != *secretType {
			continue
		}

		// Tag filter
		if tag != "" && !containsTag(secret.Tags, tag) {
			continue
		}

		filtered = append(filtered, secret)
	}

	// Convert to results with scores
	results := make([]SearchResult, len(filtered))
	for i, secret := range filtered {
		results[i] = SearchResult{
			Secret: secret,
			Score:  0,
		}
	}

	return results, nil
}

// SortBy sorts secrets by a field
type SortField int

const (
	SortByName SortField = iota
	SortByType
	SortByUpdated
	SortByCreated
)

// SortSecrets sorts a list of secrets
func SortSecrets(secrets []storage.Secret, field SortField, ascending bool) {
	sort.Slice(secrets, func(i, j int) bool {
		var less bool
		switch field {
		case SortByName:
			less = strings.ToLower(secrets[i].Name) < strings.ToLower(secrets[j].Name)
		case SortByType:
			less = secrets[i].Type < secrets[j].Type
		case SortByUpdated:
			less = secrets[i].UpdatedAt.Before(secrets[j].UpdatedAt)
		case SortByCreated:
			less = secrets[i].CreatedAt.Before(secrets[j].CreatedAt)
		default:
			less = secrets[i].UpdatedAt.After(secrets[j].UpdatedAt) // Most recent first
		}

		if !ascending {
			return !less
		}
		return less
	})
}

func containsTag(tags []string, target string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, target) {
			return true
		}
	}
	return false
}
