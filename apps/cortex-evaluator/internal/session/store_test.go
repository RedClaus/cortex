package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileStore(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "sessions")

	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	if store == nil {
		t.Fatal("expected store to be non-nil")
	}

	// Verify directory was created
	info, err := os.Stat(storePath)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected path to be a directory")
	}
}

func TestFileStore_SaveAndLoad(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())
	session := NewSession("Test Session", "/path/to/project")

	// Save the session
	if err := store.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load it back
	loaded, err := store.Load(session.ID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify all fields match
	if loaded.ID != session.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, session.ID)
	}
	if loaded.Name != session.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, session.Name)
	}
	if loaded.ProjectPath != session.ProjectPath {
		t.Errorf("ProjectPath = %q, want %q", loaded.ProjectPath, session.ProjectPath)
	}
	if !loaded.CreatedAt.Equal(session.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", loaded.CreatedAt, session.CreatedAt)
	}
	if !loaded.UpdatedAt.Equal(session.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", loaded.UpdatedAt, session.UpdatedAt)
	}
}

func TestFileStore_Load_NotFound(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())

	_, err := store.Load("nonexistent-id")
	if err != ErrSessionNotFound {
		t.Errorf("Load() error = %v, want %v", err, ErrSessionNotFound)
	}
}

func TestFileStore_Load_InvalidID(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())

	_, err := store.Load("")
	if err != ErrInvalidID {
		t.Errorf("Load() error = %v, want %v", err, ErrInvalidID)
	}
}

func TestFileStore_Save_NilSession(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())

	err := store.Save(nil)
	if err == nil {
		t.Error("expected error for nil session")
	}
}

func TestFileStore_Save_EmptyID(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())
	session := &Session{ID: "", Name: "Test"}

	err := store.Save(session)
	if err != ErrInvalidID {
		t.Errorf("Save() error = %v, want %v", err, ErrInvalidID)
	}
}

func TestFileStore_List_Empty(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())

	sessions, err := store.List(false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("List() returned %d sessions, want 0", len(sessions))
	}
}

func TestFileStore_List_MultipleSessions(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())

	// Create sessions with slight time differences
	session1 := NewSession("First", "/path/1")
	time.Sleep(10 * time.Millisecond)
	session2 := NewSession("Second", "/path/2")
	time.Sleep(10 * time.Millisecond)
	session3 := NewSession("Third", "/path/3")

	// Save in random order
	store.Save(session2)
	store.Save(session1)
	store.Save(session3)

	// List should return all sessions sorted by CreatedAt (newest first)
	sessions, err := store.List(false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("List() returned %d sessions, want 3", len(sessions))
	}

	// Verify order (newest first)
	if sessions[0].Name != "Third" {
		t.Errorf("sessions[0].Name = %q, want %q", sessions[0].Name, "Third")
	}
	if sessions[1].Name != "Second" {
		t.Errorf("sessions[1].Name = %q, want %q", sessions[1].Name, "Second")
	}
	if sessions[2].Name != "First" {
		t.Errorf("sessions[2].Name = %q, want %q", sessions[2].Name, "First")
	}
}

func TestFileStore_Delete(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())
	session := NewSession("To Delete", "/path")

	// Save then delete
	store.Save(session)
	if err := store.Delete(session.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify it's gone
	_, err := store.Load(session.ID)
	if err != ErrSessionNotFound {
		t.Errorf("expected session to be deleted, got error = %v", err)
	}
}

func TestFileStore_Delete_NotFound(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())

	err := store.Delete("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("Delete() error = %v, want %v", err, ErrSessionNotFound)
	}
}

func TestFileStore_Delete_InvalidID(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())

	err := store.Delete("")
	if err != ErrInvalidID {
		t.Errorf("Delete() error = %v, want %v", err, ErrInvalidID)
	}
}

func TestFileStore_PersistenceAcrossRestarts(t *testing.T) {
	// This test simulates application restarts by creating a new store
	// pointing to the same directory
	dir := t.TempDir()

	// "First run" - create and save session
	store1, _ := NewFileStore(dir)
	session := NewSession("Persistent Session", "/project/path")
	store1.Save(session)

	// "Second run" - create new store instance (simulates restart)
	store2, _ := NewFileStore(dir)

	// Verify session is still accessible
	loaded, err := store2.Load(session.ID)
	if err != nil {
		t.Fatalf("session not persisted across restart: %v", err)
	}

	if loaded.ID != session.ID {
		t.Errorf("ID mismatch after restart: got %q, want %q", loaded.ID, session.ID)
	}
	if loaded.Name != session.Name {
		t.Errorf("Name mismatch after restart: got %q, want %q", loaded.Name, session.Name)
	}
	if loaded.ProjectPath != session.ProjectPath {
		t.Errorf("ProjectPath mismatch after restart: got %q, want %q", loaded.ProjectPath, session.ProjectPath)
	}
}

func TestFileStore_List_IgnoresNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileStore(dir)

	// Save a valid session
	session := NewSession("Valid", "/path")
	store.Save(session)

	// Create non-JSON files that should be ignored
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0644)
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("also ignore"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	// List should only return the valid session
	sessions, err := store.List(false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("List() returned %d sessions, want 1", len(sessions))
	}
}

func TestFileStore_List_SkipsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileStore(dir)

	// Save a valid session
	session := NewSession("Valid", "/path")
	store.Save(session)

	// Create an invalid JSON file
	os.WriteFile(filepath.Join(dir, "invalid.json"), []byte("not valid json{"), 0644)

	// List should only return the valid session
	sessions, err := store.List(false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("List() returned %d sessions, want 1 (should skip invalid)", len(sessions))
	}
}

func TestFileStore_Update(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())
	session := NewSession("Original Name", "/path")
	store.Save(session)

	// Update the session
	session.Name = "Updated Name"
	session.UpdatedAt = time.Now()
	store.Save(session)

	// Load and verify update
	loaded, _ := store.Load(session.ID)
	if loaded.Name != "Updated Name" {
		t.Errorf("Name = %q, want %q", loaded.Name, "Updated Name")
	}
}

func TestFileStore_Archive(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())
	session := NewSession("Test Session", "/path")
	store.Save(session)

	// Archive the session
	if err := store.Archive(session.ID); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	// Load and verify archived
	loaded, err := store.Load(session.ID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !loaded.IsArchived() {
		t.Error("expected session to be archived")
	}
	if loaded.ArchivedAt == nil {
		t.Error("expected ArchivedAt to be set")
	}
}

func TestFileStore_Archive_NotFound(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())

	err := store.Archive("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("Archive() error = %v, want %v", err, ErrSessionNotFound)
	}
}

func TestFileStore_Archive_InvalidID(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())

	err := store.Archive("")
	if err != ErrInvalidID {
		t.Errorf("Archive() error = %v, want %v", err, ErrInvalidID)
	}
}

func TestFileStore_List_ExcludesArchivedByDefault(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())

	// Create sessions
	active := NewSession("Active", "/path/active")
	archived := NewSession("Archived", "/path/archived")
	store.Save(active)
	store.Save(archived)

	// Archive one session
	store.Archive(archived.ID)

	// List without archived should only return active
	sessions, err := store.List(false)
	if err != nil {
		t.Fatalf("List(false) error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("List(false) returned %d sessions, want 1", len(sessions))
	}
	if sessions[0].Name != "Active" {
		t.Errorf("expected active session, got %q", sessions[0].Name)
	}
}

func TestFileStore_List_IncludesArchivedWhenRequested(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())

	// Create sessions
	active := NewSession("Active", "/path/active")
	archived := NewSession("Archived", "/path/archived")
	store.Save(active)
	store.Save(archived)

	// Archive one session
	store.Archive(archived.ID)

	// List with includeArchived=true should return both
	sessions, err := store.List(true)
	if err != nil {
		t.Fatalf("List(true) error = %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("List(true) returned %d sessions, want 2", len(sessions))
	}
}

func TestSession_Archive(t *testing.T) {
	session := NewSession("Test", "/path")

	if session.IsArchived() {
		t.Error("new session should not be archived")
	}

	session.Archive()

	if !session.IsArchived() {
		t.Error("session should be archived after Archive()")
	}
	if session.ArchivedAt == nil {
		t.Error("ArchivedAt should be set")
	}
}

func TestSession_IsArchived(t *testing.T) {
	session := NewSession("Test", "/path")

	// Initially not archived
	if session.IsArchived() {
		t.Error("new session should not be archived")
	}

	// After archiving
	session.Archive()
	if !session.IsArchived() {
		t.Error("session should be archived")
	}
}
