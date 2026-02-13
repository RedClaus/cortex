package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// Store handles SQLite database operations for secret metadata
type Store struct {
	db *sql.DB
}

// NewStore creates a new SQLite store at the default location
func NewStore() (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	dataDir := filepath.Join(homeDir, ".cortex-vault")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "vault.db")
	return NewStoreWithPath(dbPath)
}

// NewStoreWithPath creates a new SQLite store at the specified path
func NewStoreWithPath(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return store, nil
}

// migrate creates the database schema
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS categories (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		icon TEXT NOT NULL DEFAULT '📁',
		color TEXT NOT NULL DEFAULT '#808080'
	);

	CREATE TABLE IF NOT EXISTS secrets (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		username TEXT,
		url TEXT,
		notes TEXT,
		category_id TEXT NOT NULL DEFAULT 'all',
		tags TEXT DEFAULT '[]',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		FOREIGN KEY (category_id) REFERENCES categories(id)
	);

	CREATE TABLE IF NOT EXISTS tags (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		color TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_secrets_category ON secrets(category_id);
	CREATE INDEX IF NOT EXISTS idx_secrets_type ON secrets(type);
	CREATE INDEX IF NOT EXISTS idx_secrets_name ON secrets(name);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("execute schema: %w", err)
	}

	// Initialize default categories if empty
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM categories").Scan(&count); err != nil {
		return fmt.Errorf("count categories: %w", err)
	}

	if count == 0 {
		for _, cat := range DefaultCategories() {
			if _, err := s.db.Exec(
				"INSERT INTO categories (id, name, icon, color) VALUES (?, ?, ?, ?)",
				cat.ID, cat.Name, cat.Icon, cat.Color,
			); err != nil {
				return fmt.Errorf("insert category: %w", err)
			}
		}
	}

	return nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateSecret stores secret metadata (value stored in Keychain separately)
func (s *Store) CreateSecret(secret *Secret) error {
	if secret.ID == "" {
		secret.ID = uuid.New().String()
	}
	if secret.CategoryID == "" {
		secret.CategoryID = "all"
	}
	if secret.Tags == nil {
		secret.Tags = []string{}
	}

	now := time.Now()
	secret.CreatedAt = now
	secret.UpdatedAt = now

	tagsJSON, err := json.Marshal(secret.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO secrets (id, name, type, username, url, notes, category_id, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, secret.ID, secret.Name, secret.Type, secret.Username, secret.URL, secret.Notes,
		secret.CategoryID, string(tagsJSON), secret.CreatedAt, secret.UpdatedAt)

	if err != nil {
		return fmt.Errorf("insert secret: %w", err)
	}

	// Ensure tags exist
	for _, tag := range secret.Tags {
		s.ensureTag(tag)
	}

	return nil
}

// UpdateSecret updates secret metadata
func (s *Store) UpdateSecret(secret *Secret) error {
	secret.UpdatedAt = time.Now()

	tagsJSON, err := json.Marshal(secret.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	result, err := s.db.Exec(`
		UPDATE secrets SET name=?, type=?, username=?, url=?, notes=?, category_id=?, tags=?, updated_at=?
		WHERE id=?
	`, secret.Name, secret.Type, secret.Username, secret.URL, secret.Notes,
		secret.CategoryID, string(tagsJSON), secret.UpdatedAt, secret.ID)

	if err != nil {
		return fmt.Errorf("update secret: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("secret not found: %s", secret.ID)
	}

	return nil
}

// DeleteSecret removes secret metadata by ID
func (s *Store) DeleteSecret(id string) error {
	result, err := s.db.Exec("DELETE FROM secrets WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("secret not found: %s", id)
	}

	return nil
}

// GetSecret retrieves a secret by ID
func (s *Store) GetSecret(id string) (*Secret, error) {
	row := s.db.QueryRow(`
		SELECT id, name, type, username, url, notes, category_id, tags, created_at, updated_at
		FROM secrets WHERE id=?
	`, id)

	return s.scanSecret(row)
}

// ListSecrets retrieves all secrets, optionally filtered by category
func (s *Store) ListSecrets(categoryID string) ([]Secret, error) {
	var rows *sql.Rows
	var err error

	if categoryID == "" || categoryID == "all" {
		rows, err = s.db.Query(`
			SELECT id, name, type, username, url, notes, category_id, tags, created_at, updated_at
			FROM secrets ORDER BY updated_at DESC
		`)
	} else {
		rows, err = s.db.Query(`
			SELECT id, name, type, username, url, notes, category_id, tags, created_at, updated_at
			FROM secrets WHERE category_id=? ORDER BY updated_at DESC
		`, categoryID)
	}

	if err != nil {
		return nil, fmt.Errorf("query secrets: %w", err)
	}
	defer rows.Close()

	var secrets []Secret
	for rows.Next() {
		secret, err := s.scanSecretRow(rows)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, *secret)
	}

	return secrets, rows.Err()
}

// ListSecretsByType retrieves secrets filtered by type
func (s *Store) ListSecretsByType(secretType SecretType) ([]Secret, error) {
	rows, err := s.db.Query(`
		SELECT id, name, type, username, url, notes, category_id, tags, created_at, updated_at
		FROM secrets WHERE type=? ORDER BY updated_at DESC
	`, secretType)

	if err != nil {
		return nil, fmt.Errorf("query secrets by type: %w", err)
	}
	defer rows.Close()

	var secrets []Secret
	for rows.Next() {
		secret, err := s.scanSecretRow(rows)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, *secret)
	}

	return secrets, rows.Err()
}

// ListSecretsByTag retrieves secrets that have a specific tag
func (s *Store) ListSecretsByTag(tag string) ([]Secret, error) {
	// SQLite JSON contains check
	rows, err := s.db.Query(`
		SELECT id, name, type, username, url, notes, category_id, tags, created_at, updated_at
		FROM secrets WHERE tags LIKE ? ORDER BY updated_at DESC
	`, "%\""+tag+"\"%")

	if err != nil {
		return nil, fmt.Errorf("query secrets by tag: %w", err)
	}
	defer rows.Close()

	var secrets []Secret
	for rows.Next() {
		secret, err := s.scanSecretRow(rows)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, *secret)
	}

	return secrets, rows.Err()
}

// SearchSecrets searches secrets by name
func (s *Store) SearchSecrets(query string) ([]Secret, error) {
	rows, err := s.db.Query(`
		SELECT id, name, type, username, url, notes, category_id, tags, created_at, updated_at
		FROM secrets WHERE name LIKE ? OR notes LIKE ? OR username LIKE ?
		ORDER BY updated_at DESC
	`, "%"+query+"%", "%"+query+"%", "%"+query+"%")

	if err != nil {
		return nil, fmt.Errorf("search secrets: %w", err)
	}
	defer rows.Close()

	var secrets []Secret
	for rows.Next() {
		secret, err := s.scanSecretRow(rows)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, *secret)
	}

	return secrets, rows.Err()
}

// GetCategories returns all categories
func (s *Store) GetCategories() ([]Category, error) {
	// Sort with "All" first, then alphabetically by name
	rows, err := s.db.Query("SELECT id, name, icon, color FROM categories ORDER BY CASE WHEN id='all' THEN 0 ELSE 1 END, name")
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var cat Category
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Icon, &cat.Color); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, cat)
	}

	return categories, rows.Err()
}

// GetCategoryCount returns the count of secrets per category
func (s *Store) GetCategoryCount(categoryID string) (int, error) {
	var count int
	var err error

	if categoryID == "all" {
		err = s.db.QueryRow("SELECT COUNT(*) FROM secrets").Scan(&count)
	} else {
		err = s.db.QueryRow("SELECT COUNT(*) FROM secrets WHERE category_id=?", categoryID).Scan(&count)
	}

	if err != nil {
		return 0, fmt.Errorf("count category: %w", err)
	}

	return count, nil
}

// GetTags returns all unique tags
func (s *Store) GetTags() ([]Tag, error) {
	rows, err := s.db.Query("SELECT id, name, color FROM tags ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		var color sql.NullString
		if err := rows.Scan(&tag.ID, &tag.Name, &color); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		if color.Valid {
			tag.Color = color.String
		}
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

// GetTagCount returns the count of secrets with a specific tag
func (s *Store) GetTagCount(tag string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM secrets WHERE tags LIKE ?`, "%\""+tag+"\"%").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count tag: %w", err)
	}
	return count, nil
}

// ensureTag creates a tag if it doesn't exist
func (s *Store) ensureTag(name string) {
	s.db.Exec("INSERT OR IGNORE INTO tags (id, name) VALUES (?, ?)", uuid.New().String(), name)
}

// scanSecret scans a single row into a Secret
func (s *Store) scanSecret(row *sql.Row) (*Secret, error) {
	var secret Secret
	var tagsJSON string
	var username, url, notes sql.NullString

	err := row.Scan(
		&secret.ID, &secret.Name, &secret.Type,
		&username, &url, &notes,
		&secret.CategoryID, &tagsJSON,
		&secret.CreatedAt, &secret.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan secret: %w", err)
	}

	if username.Valid {
		secret.Username = username.String
	}
	if url.Valid {
		secret.URL = url.String
	}
	if notes.Valid {
		secret.Notes = notes.String
	}

	if err := json.Unmarshal([]byte(tagsJSON), &secret.Tags); err != nil {
		secret.Tags = []string{}
	}

	return &secret, nil
}

// scanSecretRow scans a rows result into a Secret
func (s *Store) scanSecretRow(rows *sql.Rows) (*Secret, error) {
	var secret Secret
	var tagsJSON string
	var username, url, notes sql.NullString

	err := rows.Scan(
		&secret.ID, &secret.Name, &secret.Type,
		&username, &url, &notes,
		&secret.CategoryID, &tagsJSON,
		&secret.CreatedAt, &secret.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan secret row: %w", err)
	}

	if username.Valid {
		secret.Username = username.String
	}
	if url.Valid {
		secret.URL = url.String
	}
	if notes.Valid {
		secret.Notes = notes.String
	}

	if err := json.Unmarshal([]byte(tagsJSON), &secret.Tags); err != nil {
		secret.Tags = []string{}
	}

	return &secret, nil
}

// VaultExists checks if a vault database exists
func VaultExists() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	dbPath := filepath.Join(homeDir, ".cortex-vault", "vault.db")
	_, err = os.Stat(dbPath)
	return err == nil
}
