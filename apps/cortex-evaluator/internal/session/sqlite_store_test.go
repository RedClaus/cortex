package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSQLiteStore_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subdir", "sessions.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestSQLiteStore_Save_Load_RoundTrip(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	// Create a session
	session := NewSession("Test Session", "/path/to/project")

	// Save it
	if err := store.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load it back
	loaded, err := store.Load(session.ID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify all fields
	if loaded.ID != session.ID {
		t.Errorf("ID = %v, want %v", loaded.ID, session.ID)
	}
	if loaded.Name != session.Name {
		t.Errorf("Name = %v, want %v", loaded.Name, session.Name)
	}
	if loaded.ProjectPath != session.ProjectPath {
		t.Errorf("ProjectPath = %v, want %v", loaded.ProjectPath, session.ProjectPath)
	}
	if !loaded.CreatedAt.Equal(session.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", loaded.CreatedAt, session.CreatedAt)
	}
	if !loaded.UpdatedAt.Equal(session.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", loaded.UpdatedAt, session.UpdatedAt)
	}
	if loaded.ArchivedAt != nil {
		t.Errorf("ArchivedAt = %v, want nil", loaded.ArchivedAt)
	}
}

func TestSQLiteStore_Save_Update(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	session := NewSession("Original", "/original/path")
	if err := store.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Update the session
	session.Name = "Updated"
	session.ProjectPath = "/updated/path"
	session.UpdatedAt = time.Now()

	if err := store.Save(session); err != nil {
		t.Fatalf("Save() update error = %v", err)
	}

	// Load and verify
	loaded, err := store.Load(session.ID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Name != "Updated" {
		t.Errorf("Name = %v, want Updated", loaded.Name)
	}
	if loaded.ProjectPath != "/updated/path" {
		t.Errorf("ProjectPath = %v, want /updated/path", loaded.ProjectPath)
	}
}

func TestSQLiteStore_Save_NilSession(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	err := store.Save(nil)
	if err == nil {
		t.Error("Save(nil) should return error")
	}
}

func TestSQLiteStore_Save_EmptyID(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	session := &Session{Name: "Test", ProjectPath: "/path"}
	err := store.Save(session)
	if err != ErrInvalidID {
		t.Errorf("Save() error = %v, want ErrInvalidID", err)
	}
}

func TestSQLiteStore_Load_NotFound(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	_, err := store.Load("nonexistent-id")
	if err != ErrSessionNotFound {
		t.Errorf("Load() error = %v, want ErrSessionNotFound", err)
	}
}

func TestSQLiteStore_Load_EmptyID(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	_, err := store.Load("")
	if err != ErrInvalidID {
		t.Errorf("Load() error = %v, want ErrInvalidID", err)
	}
}

func TestSQLiteStore_List_Empty(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	sessions, err := store.List(false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("List() returned %d sessions, want 0", len(sessions))
	}
}

func TestSQLiteStore_List_SortedByCreatedAt(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	// Create sessions with different creation times
	s1 := NewSession("First", "/first")
	time.Sleep(10 * time.Millisecond)
	s2 := NewSession("Second", "/second")
	time.Sleep(10 * time.Millisecond)
	s3 := NewSession("Third", "/third")

	// Save in random order
	for _, s := range []*Session{s2, s1, s3} {
		if err := store.Save(s); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// List should return newest first
	sessions, err := store.List(false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(sessions) != 3 {
		t.Fatalf("List() returned %d sessions, want 3", len(sessions))
	}

	if sessions[0].Name != "Third" {
		t.Errorf("sessions[0].Name = %v, want Third", sessions[0].Name)
	}
	if sessions[1].Name != "Second" {
		t.Errorf("sessions[1].Name = %v, want Second", sessions[1].Name)
	}
	if sessions[2].Name != "First" {
		t.Errorf("sessions[2].Name = %v, want First", sessions[2].Name)
	}
}

func TestSQLiteStore_List_ExcludeArchived(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	// Create sessions
	active := NewSession("Active", "/active")
	archived := NewSession("Archived", "/archived")
	archived.Archive()

	for _, s := range []*Session{active, archived} {
		if err := store.Save(s); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// List without archived
	sessions, err := store.List(false)
	if err != nil {
		t.Fatalf("List(false) error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("List(false) returned %d sessions, want 1", len(sessions))
	}
	if sessions[0].Name != "Active" {
		t.Errorf("sessions[0].Name = %v, want Active", sessions[0].Name)
	}
}

func TestSQLiteStore_List_IncludeArchived(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	// Create sessions
	active := NewSession("Active", "/active")
	archived := NewSession("Archived", "/archived")
	archived.Archive()

	for _, s := range []*Session{active, archived} {
		if err := store.Save(s); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// List with archived
	sessions, err := store.List(true)
	if err != nil {
		t.Fatalf("List(true) error = %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("List(true) returned %d sessions, want 2", len(sessions))
	}
}

func TestSQLiteStore_Delete(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	session := NewSession("To Delete", "/delete")
	if err := store.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := store.Delete(session.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := store.Load(session.ID)
	if err != ErrSessionNotFound {
		t.Errorf("Load() after Delete() error = %v, want ErrSessionNotFound", err)
	}
}

func TestSQLiteStore_Delete_NotFound(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	err := store.Delete("nonexistent-id")
	if err != ErrSessionNotFound {
		t.Errorf("Delete() error = %v, want ErrSessionNotFound", err)
	}
}

func TestSQLiteStore_Delete_EmptyID(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	err := store.Delete("")
	if err != ErrInvalidID {
		t.Errorf("Delete() error = %v, want ErrInvalidID", err)
	}
}

func TestSQLiteStore_Archive(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	session := NewSession("To Archive", "/archive")
	if err := store.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := store.Archive(session.ID); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	loaded, err := store.Load(session.ID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !loaded.IsArchived() {
		t.Error("session should be archived")
	}
	if loaded.ArchivedAt == nil {
		t.Error("ArchivedAt should not be nil")
	}
}

func TestSQLiteStore_Archive_NotFound(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	err := store.Archive("nonexistent-id")
	if err != ErrSessionNotFound {
		t.Errorf("Archive() error = %v, want ErrSessionNotFound", err)
	}
}

func TestSQLiteStore_Archive_EmptyID(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	err := store.Archive("")
	if err != ErrInvalidID {
		t.Errorf("Archive() error = %v, want ErrInvalidID", err)
	}
}

func TestSQLiteStore_PersistenceAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sessions.db")

	// Create store, save session, close
	store1, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}

	session := NewSession("Persistent", "/persistent/path")
	if err := store1.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	store1.Close()

	// Open new store, load session
	store2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() reopened error = %v", err)
	}
	defer store2.Close()

	loaded, err := store2.Load(session.ID)
	if err != nil {
		t.Fatalf("Load() after reopen error = %v", err)
	}

	if loaded.Name != session.Name {
		t.Errorf("Name = %v, want %v", loaded.Name, session.Name)
	}
	if loaded.ProjectPath != session.ProjectPath {
		t.Errorf("ProjectPath = %v, want %v", loaded.ProjectPath, session.ProjectPath)
	}
}

func TestSQLiteStore_ArchivedAt_Persistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sessions.db")

	// Create store, save archived session
	store1, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}

	session := NewSession("Archived Session", "/archived/path")
	session.Archive()
	if err := store1.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	store1.Close()

	// Reopen and verify archived state persisted
	store2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() reopened error = %v", err)
	}
	defer store2.Close()

	loaded, err := store2.Load(session.ID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !loaded.IsArchived() {
		t.Error("session should be archived after reload")
	}
	if loaded.ArchivedAt == nil {
		t.Error("ArchivedAt should not be nil after reload")
	}
}

// newTestSQLiteStore creates a new SQLiteStore for testing with a temporary database.
func newTestSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	return store
}
