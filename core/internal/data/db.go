// Package data provides the SQLite-based data access layer for Cortex.
// It uses modernc.org/sqlite for pure-Go, CGO-free database access.
package data

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

//go:embed migrations/001_initial_schema.sql
var initialSchema string

//go:embed migrations/002_acontext_sync.sql
var acontextSyncSchema string

//go:embed migrations/004_cognitive_tables.sql
var cognitiveSchema string

//go:embed migrations/005_conversation_eval.sql
var conversationEvalSchema string

//go:embed migrations/007_knowledge_ingestion.sql
var knowledgeIngestionSchema string

//go:embed migrations/008_persona_core.sql
var personaCoreSchema string

//go:embed migrations/009_tasks.sql
var taskManagementSchema string

//go:embed migrations/010_memory_enhanced.sql
var memoryEnhancedSchema string

//go:embed migrations/011_memory_performance.sql
var memoryPerformanceSchema string

//go:embed migrations/012_user_auth.sql
var userAuthSchema string

//go:embed migrations/013_lessons.sql
var lessonsSchema string

//go:embed migrations/014_reasoning_traces.sql
var reasoningTracesSchema string

//go:embed migrations/019_skill_library.sql
var skillLibrarySchema string

//go:embed migrations/020_memcubes.sql
var memcubesSchema string

// Store provides access to the SQLite database.
type Store struct {
	db *sql.DB
}

// NewDB creates a new database connection and initializes the schema.
// The dataDir should point to a LOCAL directory (e.g., ~/.cortex).
// Network paths are rejected to prevent SQLite corruption.
func NewDB(dataDir string) (*Store, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	// Validate path is local (not network drive)
	if err := validateLocalPath(dataDir); err != nil {
		return nil, fmt.Errorf("validate data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "knowledge.db")

	// Open database with SQLite-specific connection parameters
	// WAL mode is enabled via PRAGMA after connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(1) // SQLite works best with single writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // Connections never expire

	store := &Store{db: db}

	// Initialize SQLite PRAGMAs
	if err := store.initPragmas(); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize pragmas: %w", err)
	}

	// Run migrations
	if err := store.Migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return store, nil
}

// initPragmas configures SQLite for optimal performance and safety.
func (s *Store) initPragmas() error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",        // Write-Ahead Logging for concurrent reads
		"PRAGMA synchronous = NORMAL",      // Balance safety and performance
		"PRAGMA foreign_keys = ON",         // Enforce referential integrity
		"PRAGMA busy_timeout = 5000",       // Wait 5 seconds if locked
		"PRAGMA cache_size = -64000",       // 64MB cache (negative = KB)
		"PRAGMA temp_store = MEMORY",       // Keep temp tables in memory
		"PRAGMA mmap_size = 268435456",     // 256MB memory-mapped I/O
		"PRAGMA page_size = 4096",          // Match OS page size
		"PRAGMA auto_vacuum = INCREMENTAL", // Reclaim space gradually
	}

	for _, pragma := range pragmas {
		if _, err := s.db.Exec(pragma); err != nil {
			return fmt.Errorf("execute %s: %w", pragma, err)
		}
	}

	return nil
}

// Migrate runs all embedded schema migrations.
// This is idempotent - safe to call multiple times.
func (s *Store) Migrate() error {
	// Run each migration in order
	migrations := []struct {
		name   string
		schema string
	}{
		{"initial_schema", initialSchema},
		{"acontext_sync", acontextSyncSchema},
		{"cognitive_architecture", cognitiveSchema},
		{"conversation_eval", conversationEvalSchema},
		{"knowledge_ingestion", knowledgeIngestionSchema},
		{"persona_core", personaCoreSchema},
		{"task_management", taskManagementSchema},
		{"memory_enhanced", memoryEnhancedSchema},
		{"memory_performance", memoryPerformanceSchema},
		{"user_auth", userAuthSchema},
		{"lessons", lessonsSchema},
		{"reasoning_traces", reasoningTracesSchema},
		{"skill_library", skillLibrarySchema},
		{"memcubes", memcubesSchema},
	}

	for _, m := range migrations {
		if err := s.runMigration(m.name, m.schema); err != nil {
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
	}

	return nil
}

// runMigration executes a single migration schema.
func (s *Store) runMigration(name, schema string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the schema
	// Split by semicolon to handle multi-statement SQL
	statements := splitSQL(schema)
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("execute statement %d: %w\nSQL: %s", i+1, err, stmt)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}

	return nil
}

// Health checks if the database connection is alive and responsive.
func (s *Store) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Simple query to verify connectivity
	var result int
	err := s.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("health check returned unexpected value: %d", result)
	}

	return nil
}

// Close closes the database connection.
// This should be called when shutting down the application.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}

	// Run checkpoint to flush WAL to main database
	if _, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		// Log but don't fail - we still want to close
		fmt.Fprintf(os.Stderr, "Warning: WAL checkpoint failed: %v\n", err)
	}

	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	return nil
}

// DB returns the underlying *sql.DB for advanced operations.
// Use with caution - prefer the Store methods when possible.
func (s *Store) DB() *sql.DB {
	return s.db
}

// validateLocalPath ensures the path is on a local filesystem.
// Network paths (SMB, NFS, etc.) can cause SQLite corruption.
func validateLocalPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve absolute path: %w", err)
	}

	// Check for common network path patterns
	networkPrefixes := []string{
		"//",     // UNC paths (Windows)
		"\\\\",   // UNC paths (Windows alternative)
		"/mnt/",  // Common Linux NFS/CIFS mount point
		"/net/",  // macOS network mounts
		"/Volumes/", // macOS external/network volumes (may be local, but risky)
	}

	for _, prefix := range networkPrefixes {
		if strings.HasPrefix(absPath, prefix) {
			return fmt.Errorf("network path detected: %s (SQLite requires local filesystem)", absPath)
		}
	}

	// Additional check: ensure directory is writable
	testFile := filepath.Join(path, ".cortex-write-test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("directory not writable: %w", err)
	}
	os.Remove(testFile)

	return nil
}

// splitSQL splits a multi-statement SQL string into individual statements.
// Handles comments, empty lines, strings, and BEGIN...END blocks (for triggers).
func splitSQL(sql string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := rune(0)
	beginDepth := 0 // Track nested BEGIN...END blocks

	lines := strings.Split(sql, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and pure comment lines
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}

		// Check for BEGIN/END keywords (case-insensitive) outside of strings
		upperLine := strings.ToUpper(trimmed)
		if !inString {
			// Check for BEGIN keyword (start of trigger/block)
			if strings.Contains(upperLine, "BEGIN") && !strings.Contains(upperLine, "BEGIN TRANSACTION") {
				beginDepth++
			}
		}

		// Process line character by character to handle strings
		for i, ch := range line {
			// Toggle string state
			if (ch == '\'' || ch == '"') && !inString {
				inString = true
				stringChar = ch
			} else if ch == stringChar && inString {
				inString = false
				stringChar = 0
			}

			current.WriteRune(ch)

			// Check for END; at end of line (trigger termination)
			if ch == ';' && !inString {
				// Look back to see if this is "END;"
				currentStr := current.String()
				trimmedCurrent := strings.TrimSpace(currentStr)
				upperCurrent := strings.ToUpper(trimmedCurrent)

				// Check if this semicolon ends an END statement
				if beginDepth > 0 && strings.HasSuffix(upperCurrent, "END;") {
					beginDepth--
				}

				// Split on semicolon only if not inside BEGIN...END block
				if beginDepth == 0 {
					stmt := strings.TrimSpace(currentStr)
					if stmt != "" && !strings.HasPrefix(stmt, "--") {
						statements = append(statements, stmt)
					}
					current.Reset()
				}
			}
			_ = i // avoid unused variable warning
		}

		current.WriteRune('\n')
	}

	// Add any remaining statement
	if final := strings.TrimSpace(current.String()); final != "" && !strings.HasPrefix(final, "--") {
		statements = append(statements, final)
	}

	return statements
}

// BeginTx starts a new transaction with the given context and options.
// Use this for operations that need to be atomic.
func (s *Store) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	tx, err := s.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return tx, nil
}

// WithTx executes a function within a transaction.
// If the function returns an error, the transaction is rolled back.
// Otherwise, it is committed.
func (s *Store) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// ══════════════════════════════════════════════════════════════════════════════
// Global store accessor for cross-package access
// ══════════════════════════════════════════════════════════════════════════════

var globalStore *Store

// SetGlobalStore sets the global store instance.
// This should be called once during application startup.
func SetGlobalStore(s *Store) {
	globalStore = s
}

// GetGlobalStore returns the global store instance, or nil if not set.
// Use this for cross-package access when dependency injection isn't practical.
func GetGlobalStore() *Store {
	return globalStore
}
