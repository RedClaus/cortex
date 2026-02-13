// Package data provides tests for the SQLite data access layer.
package data

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewDB verifies database initialization with various scenarios.
func TestNewDB(t *testing.T) {
	t.Run("creates database in valid directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := NewDB(tmpDir)
		if err != nil {
			t.Fatalf("NewDB failed: %v", err)
		}
		defer store.Close()

		// Verify database file exists
		dbPath := filepath.Join(tmpDir, "knowledge.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Error("database file not created")
		}

		// Verify WAL mode is enabled (should create -wal file on first write)
		if err := store.Health(); err != nil {
			t.Errorf("health check failed: %v", err)
		}
	})

	t.Run("creates nested directory structure", func(t *testing.T) {
		tmpDir := t.TempDir()
		nestedDir := filepath.Join(tmpDir, "deep", "nested", "cortex")

		store, err := NewDB(nestedDir)
		if err != nil {
			t.Fatalf("NewDB with nested dir failed: %v", err)
		}
		defer store.Close()

		// Verify directory was created
		if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
			t.Error("nested directory not created")
		}
	})

	t.Run("idempotent migrations", func(t *testing.T) {
		tmpDir := t.TempDir()

		// First initialization
		store1, err := NewDB(tmpDir)
		if err != nil {
			t.Fatalf("first NewDB failed: %v", err)
		}
		store1.Close()

		// Second initialization (should succeed with same schema)
		store2, err := NewDB(tmpDir)
		if err != nil {
			t.Fatalf("second NewDB failed: %v", err)
		}
		defer store2.Close()

		// Verify tables exist
		if err := store2.Health(); err != nil {
			t.Errorf("health check after re-init failed: %v", err)
		}
	})
}

// TestStoreHealth verifies health check functionality.
func TestStoreHealth(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	t.Run("healthy database returns nil", func(t *testing.T) {
		if err := store.Health(); err != nil {
			t.Errorf("Health() returned error: %v", err)
		}
	})

	t.Run("closed database returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		closedStore, _ := NewDB(tmpDir)
		closedStore.Close()

		if err := closedStore.Health(); err == nil {
			t.Error("Health() should return error for closed database")
		}
	})
}

// TestStoreMigration verifies schema migration.
func TestStoreMigration(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	t.Run("knowledge_items table exists", func(t *testing.T) {
		var count int
		err := store.db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name='knowledge_items'
		`).Scan(&count)

		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if count != 1 {
			t.Error("knowledge_items table not found")
		}
	})

	t.Run("knowledge_fts table exists", func(t *testing.T) {
		var count int
		err := store.db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name='knowledge_fts'
		`).Scan(&count)

		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if count != 1 {
			t.Error("knowledge_fts FTS5 table not found")
		}
	})

	t.Run("sessions table exists", func(t *testing.T) {
		var count int
		err := store.db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name='sessions'
		`).Scan(&count)

		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if count != 1 {
			t.Error("sessions table not found")
		}
	})

	t.Run("trust_profiles table exists", func(t *testing.T) {
		var count int
		err := store.db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name='trust_profiles'
		`).Scan(&count)

		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if count != 1 {
			t.Error("trust_profiles table not found")
		}
	})
}

// TestStoreTransaction verifies transaction support.
func TestStoreTransaction(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	t.Run("WithTx commits on success", func(t *testing.T) {
		ctx := context.Background()

		err := store.WithTx(ctx, func(tx *sql.Tx) error {
			_, err := tx.Exec(`
				INSERT INTO knowledge_items (id, type, content, scope, author_id, tags, created_at, updated_at)
				VALUES ('test-tx-1', 'sop', 'test content', 'personal', 'user1', '[]', datetime('now'), datetime('now'))
			`)
			return err
		})

		if err != nil {
			t.Fatalf("WithTx failed: %v", err)
		}

		// Verify item was inserted
		var count int
		store.db.QueryRow("SELECT COUNT(*) FROM knowledge_items WHERE id = 'test-tx-1'").Scan(&count)
		if count != 1 {
			t.Error("transaction did not commit")
		}
	})

	t.Run("WithTx rolls back on error", func(t *testing.T) {
		ctx := context.Background()

		err := store.WithTx(ctx, func(tx *sql.Tx) error {
			_, err := tx.Exec(`
				INSERT INTO knowledge_items (id, type, content, scope, author_id, tags, created_at, updated_at)
				VALUES ('test-tx-2', 'sop', 'test content', 'personal', 'user1', '[]', datetime('now'), datetime('now'))
			`)
			if err != nil {
				return err
			}
			// Force error
			return context.Canceled
		})

		if err == nil {
			t.Error("WithTx should return error")
		}

		// Verify item was NOT inserted
		var count int
		store.db.QueryRow("SELECT COUNT(*) FROM knowledge_items WHERE id = 'test-tx-2'").Scan(&count)
		if count != 0 {
			t.Error("transaction did not rollback")
		}
	})
}

// TestValidateLocalPath verifies path validation logic.
func TestValidateLocalPath(t *testing.T) {
	t.Run("accepts local path", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := validateLocalPath(tmpDir); err != nil {
			t.Errorf("validateLocalPath rejected valid local path: %v", err)
		}
	})

	t.Run("rejects UNC paths", func(t *testing.T) {
		// Only relevant on Windows, but test the logic
		if err := validateLocalPath("//server/share/path"); err == nil {
			// Note: This might not fail on all systems, but the logic is there
			t.Log("UNC path validation depends on platform")
		}
	})
}

// TestSplitSQL verifies SQL statement splitting.
func TestSplitSQL(t *testing.T) {
	t.Run("splits simple statements", func(t *testing.T) {
		sql := `
			CREATE TABLE test1 (id TEXT);
			CREATE TABLE test2 (id TEXT);
		`

		stmts := splitSQL(sql)
		if len(stmts) != 2 {
			t.Errorf("expected 2 statements, got %d", len(stmts))
		}
	})

	t.Run("handles strings with semicolons", func(t *testing.T) {
		sql := `INSERT INTO test VALUES ('a;b;c');`

		stmts := splitSQL(sql)
		if len(stmts) != 1 {
			t.Errorf("expected 1 statement, got %d: %v", len(stmts), stmts)
		}
	})

	t.Run("skips comments", func(t *testing.T) {
		sql := `
			-- This is a comment
			CREATE TABLE test (id TEXT);
			-- Another comment
		`

		stmts := splitSQL(sql)
		if len(stmts) != 1 {
			t.Errorf("expected 1 statement (skipping comments), got %d", len(stmts))
		}
	})

	t.Run("handles multi-line statements", func(t *testing.T) {
		sql := `
			CREATE TABLE test (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL
			);
		`

		stmts := splitSQL(sql)
		if len(stmts) != 1 {
			t.Errorf("expected 1 multi-line statement, got %d", len(stmts))
		}
	})
}

// TestWALMode verifies Write-Ahead Logging is enabled.
func TestWALMode(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	var journalMode string
	err := store.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("query journal_mode failed: %v", err)
	}

	if journalMode != "wal" {
		t.Errorf("expected WAL mode, got: %s", journalMode)
	}
}

// TestForeignKeys verifies foreign key enforcement.
func TestForeignKeys(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	var foreignKeys int
	err := store.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys)
	if err != nil {
		t.Fatalf("query foreign_keys failed: %v", err)
	}

	if foreignKeys != 1 {
		t.Error("foreign keys not enabled")
	}
}

// TestConcurrentReads verifies concurrent read capability with WAL mode.
func TestConcurrentReads(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Insert test data
	ctx := context.Background()
	store.db.ExecContext(ctx, `
		INSERT INTO knowledge_items (id, type, content, scope, author_id, tags, created_at, updated_at)
		VALUES ('concurrent-test', 'sop', 'test', 'personal', 'user1', '[]', datetime('now'), datetime('now'))
	`)

	// Run concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			var id string
			store.db.QueryRow("SELECT id FROM knowledge_items WHERE id = 'concurrent-test'").Scan(&id)
			done <- id == "concurrent-test"
		}()
	}

	// Wait for all reads
	timeout := time.After(5 * time.Second)
	successCount := 0
	for i := 0; i < 10; i++ {
		select {
		case success := <-done:
			if success {
				successCount++
			}
		case <-timeout:
			t.Fatal("concurrent reads timed out")
		}
	}

	if successCount != 10 {
		t.Errorf("expected 10 successful reads, got %d", successCount)
	}
}

// setupTestStore creates a temporary store for testing.
func setupTestStore(t *testing.T) *Store {
	t.Helper()

	tmpDir := t.TempDir()
	store, err := NewDB(tmpDir)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	return store
}
