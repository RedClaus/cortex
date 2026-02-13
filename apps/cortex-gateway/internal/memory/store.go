package memory

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Store represents a memory store that reads from and writes to markdown files
type Store struct {
	rootPath string
}

// Entry represents a memory entry
type Entry struct {
	Content    string                 `json:"content"`
	Timestamp  time.Time              `json:"timestamp"`
	File       string                 `json:"file"`
	Type       string                 `json:"type"` // episodic, knowledge
	Importance float64                `json:"importance"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Stats represents memory statistics
type Stats struct {
	TotalEntries   int       `json:"total_entries"`
	EpisodicCount  int       `json:"episodic_count"`
	KnowledgeCount int       `json:"knowledge_count"`
	LastUpdate     time.Time `json:"last_update"`
	FilesCount     int       `json:"files_count"`
}

// NewStore creates a new memory store
func NewStore(rootPath string) *Store {
	// Expand ~ to home directory
	if strings.HasPrefix(rootPath, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			rootPath = filepath.Join(home, rootPath[1:])
		}
	}

	return &Store{
		rootPath: rootPath,
	}
}

// Search searches for memories matching the query
func (s *Store) Search(query string, limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 50
	}

	entries := []Entry{}

	// Search in memory directory
	memoryDir := filepath.Join(s.rootPath, "memory")
	if err := s.searchInDirectory(memoryDir, query, &entries); err != nil {
		// If memory directory doesn't exist, try root
		if err := s.searchInDirectory(s.rootPath, query, &entries); err != nil {
			return nil, fmt.Errorf("failed to search memories: %w", err)
		}
	}

	// Sort by timestamp (most recent first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	// Apply limit
	if len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

// searchInDirectory searches for memories in a specific directory
func (s *Store) searchInDirectory(dir string, query string, entries *[]Entry) error {
	// Create regex for case-insensitive search
	pattern, err := regexp.Compile("(?i)" + regexp.QuoteMeta(query))
	if err != nil {
		return fmt.Errorf("invalid search pattern: %w", err)
	}

	// Walk through markdown files
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Only process markdown files
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		// Read file content
		file, err := os.Open(path)
		if err != nil {
			return nil // Skip files that can't be opened
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// Check if line matches query
			if pattern.MatchString(line) {
				entry := Entry{
					Content:    strings.TrimSpace(line),
					File:       path,
					Type:       "episodic",
					Importance: 0.5,
					Timestamp:  info.ModTime(),
					Metadata: map[string]interface{}{
						"line": lineNum,
					},
				}

				// Determine type based on file name
				if strings.Contains(filepath.Base(path), "knowledge") {
					entry.Type = "knowledge"
					entry.Importance = 0.8
				}

				*entries = append(*entries, entry)
			}
		}

		return nil
	})
}

// Store stores a new memory entry
func (s *Store) Store(content string, importance float64, memType string) error {
	if content == "" {
		return fmt.Errorf("content cannot be empty")
	}

	// Default type
	if memType == "" {
		memType = "episodic"
	}

	// Create memory directory if it doesn't exist
	memoryDir := filepath.Join(s.rootPath, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Determine file based on type
	var filePath string
	now := time.Now()

	if memType == "knowledge" {
		filePath = filepath.Join(memoryDir, "knowledge.md")
	} else {
		// Use daily episodic memory file
		dateStr := now.Format("2006-01-02")
		filePath = filepath.Join(memoryDir, fmt.Sprintf("%s.md", dateStr))
	}

	// Open file for appending (create if doesn't exist)
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open memory file: %w", err)
	}
	defer file.Close()

	// Format entry with timestamp
	timestamp := now.Format("15:04:05")
	entry := fmt.Sprintf("\n## %s\n%s\n", timestamp, content)

	// Write entry
	if _, err := file.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write memory entry: %w", err)
	}

	return nil
}

// Recent retrieves recent memories
func (s *Store) Recent(limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 10
	}

	entries := []Entry{}

	// Search in memory directory
	memoryDir := filepath.Join(s.rootPath, "memory")
	if err := s.getAllEntries(memoryDir, &entries); err != nil {
		// If memory directory doesn't exist, try root
		if err := s.getAllEntries(s.rootPath, &entries); err != nil {
			return nil, fmt.Errorf("failed to get recent memories: %w", err)
		}
	}

	// Sort by timestamp (most recent first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	// Apply limit
	if len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

// getAllEntries retrieves all memory entries from a directory
func (s *Store) getAllEntries(dir string, entries *[]Entry) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Only process markdown files
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		// Read file content
		file, err := os.Open(path)
		if err != nil {
			return nil // Skip files that can't be opened
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// Skip empty lines and headers
			if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
				continue
			}

			entry := Entry{
				Content:    strings.TrimSpace(line),
				File:       path,
				Type:       "episodic",
				Importance: 0.5,
				Timestamp:  info.ModTime(),
				Metadata: map[string]interface{}{
					"line": lineNum,
				},
			}

			// Determine type based on file name
			if strings.Contains(filepath.Base(path), "knowledge") {
				entry.Type = "knowledge"
				entry.Importance = 0.8
			}

			*entries = append(*entries, entry)
		}

		return nil
	})
}

// Stats returns memory statistics
func (s *Store) Stats() (*Stats, error) {
	entries := []Entry{}

	// Get all entries
	memoryDir := filepath.Join(s.rootPath, "memory")
	if err := s.getAllEntries(memoryDir, &entries); err != nil {
		if err := s.getAllEntries(s.rootPath, &entries); err != nil {
			return nil, fmt.Errorf("failed to get memory stats: %w", err)
		}
	}

	// Count by type
	episodicCount := 0
	knowledgeCount := 0
	var lastUpdate time.Time

	for _, entry := range entries {
		if entry.Type == "episodic" {
			episodicCount++
		} else {
			knowledgeCount++
		}

		if entry.Timestamp.After(lastUpdate) {
			lastUpdate = entry.Timestamp
		}
	}

	// Count files
	filesCount := 0
	filepath.Walk(memoryDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".md") {
			filesCount++
		}
		return nil
	})

	return &Stats{
		TotalEntries:   len(entries),
		EpisodicCount:  episodicCount,
		KnowledgeCount: knowledgeCount,
		LastUpdate:     lastUpdate,
		FilesCount:     filesCount,
	}, nil
}
