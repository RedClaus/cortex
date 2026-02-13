package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Store provides database operations for authentication.
type Store struct {
	db *sql.DB
}

// NewStore creates a new auth store with the given database connection.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ───────────────────────────────────────────────────────────────────────────────
// USER OPERATIONS
// ───────────────────────────────────────────────────────────────────────────────

// CreateUser inserts a new user into the database.
func (s *Store) CreateUser(ctx context.Context, user *User) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	query := `
		INSERT INTO users (id, username, password_hash, email, display_name, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.IsActive = true

	_, err := s.db.ExecContext(ctx, query,
		user.ID,
		user.Username,
		user.PasswordHash,
		user.Email,
		user.DisplayName,
		boolToInt(user.IsActive),
		user.CreatedAt,
		user.UpdatedAt,
	)

	if err != nil {
		// Check for unique constraint violation
		if isUniqueConstraintError(err) {
			return ErrUserExists
		}
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

// GetUserByID retrieves a user by their ID.
func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, username, password_hash, email, display_name, is_active, created_at, updated_at
		FROM users
		WHERE id = ?
	`

	user := &User{}
	var isActive int

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.DisplayName,
		&isActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	user.IsActive = isActive == 1
	return user, nil
}

// GetUserByUsername retrieves a user by their username.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	query := `
		SELECT id, username, password_hash, email, display_name, is_active, created_at, updated_at
		FROM users
		WHERE username = ?
	`

	user := &User{}
	var isActive int

	err := s.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.DisplayName,
		&isActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}

	user.IsActive = isActive == 1
	return user, nil
}

// UpdateUser updates user information.
func (s *Store) UpdateUser(ctx context.Context, user *User) error {
	query := `
		UPDATE users
		SET email = ?, display_name = ?, is_active = ?, updated_at = ?
		WHERE id = ?
	`

	user.UpdatedAt = time.Now()

	result, err := s.db.ExecContext(ctx, query,
		user.Email,
		user.DisplayName,
		boolToInt(user.IsActive),
		user.UpdatedAt,
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

// ───────────────────────────────────────────────────────────────────────────────
// SESSION OPERATIONS
// ───────────────────────────────────────────────────────────────────────────────

// CreateSession inserts a new session into the database.
func (s *Store) CreateSession(ctx context.Context, session *Session) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}

	query := `
		INSERT INTO user_sessions (id, user_id, refresh_token_hash, expires_at, user_agent, ip_address, created_at, last_used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	session.CreatedAt = now
	session.LastUsedAt = now

	_, err := s.db.ExecContext(ctx, query,
		session.ID,
		session.UserID,
		session.RefreshTokenHash,
		session.ExpiresAt,
		session.UserAgent,
		session.IPAddress,
		session.CreatedAt,
		session.LastUsedAt,
	)

	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	return nil
}

// GetSessionByTokenHash retrieves a session by its refresh token hash.
func (s *Store) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	query := `
		SELECT id, user_id, refresh_token_hash, expires_at, user_agent, ip_address, created_at, last_used_at
		FROM user_sessions
		WHERE refresh_token_hash = ?
	`

	session := &Session{}
	err := s.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&session.ID,
		&session.UserID,
		&session.RefreshTokenHash,
		&session.ExpiresAt,
		&session.UserAgent,
		&session.IPAddress,
		&session.CreatedAt,
		&session.LastUsedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrSessionExpired
	}
	if err != nil {
		return nil, fmt.Errorf("get session by token: %w", err)
	}

	return session, nil
}

// UpdateSessionLastUsed updates the last used timestamp for a session.
func (s *Store) UpdateSessionLastUsed(ctx context.Context, sessionID string) error {
	query := `UPDATE user_sessions SET last_used_at = ? WHERE id = ?`

	_, err := s.db.ExecContext(ctx, query, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return nil
}

// DeleteSession removes a session from the database.
func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM user_sessions WHERE id = ?`

	_, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// DeleteUserSessions removes all sessions for a user.
func (s *Store) DeleteUserSessions(ctx context.Context, userID string) error {
	query := `DELETE FROM user_sessions WHERE user_id = ?`

	_, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}

	return nil
}

// CleanupExpiredSessions removes all expired sessions.
func (s *Store) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	query := `DELETE FROM user_sessions WHERE expires_at < ?`

	result, err := s.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("cleanup sessions: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows, nil
}

// ───────────────────────────────────────────────────────────────────────────────
// USER-PERSONA OPERATIONS
// ───────────────────────────────────────────────────────────────────────────────

// AssignPersonaToUser assigns a persona to a user.
func (s *Store) AssignPersonaToUser(ctx context.Context, userID, personaID string, isDefault bool) error {
	query := `
		INSERT INTO user_personas (user_id, persona_id, is_default, assigned_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, persona_id) DO UPDATE SET is_default = excluded.is_default
	`

	_, err := s.db.ExecContext(ctx, query, userID, personaID, boolToInt(isDefault), time.Now())
	if err != nil {
		return fmt.Errorf("assign persona: %w", err)
	}

	return nil
}

// UnassignPersonaFromUser removes a persona assignment from a user.
func (s *Store) UnassignPersonaFromUser(ctx context.Context, userID, personaID string) error {
	query := `DELETE FROM user_personas WHERE user_id = ? AND persona_id = ?`

	_, err := s.db.ExecContext(ctx, query, userID, personaID)
	if err != nil {
		return fmt.Errorf("unassign persona: %w", err)
	}

	return nil
}

// SetDefaultPersona sets a persona as the default for a user.
// This unsets any existing default first.
func (s *Store) SetDefaultPersona(ctx context.Context, userID, personaID string) error {
	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Unset current default
	_, err = tx.ExecContext(ctx, `UPDATE user_personas SET is_default = 0 WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("unset default: %w", err)
	}

	// Set new default
	result, err := tx.ExecContext(ctx, `UPDATE user_personas SET is_default = 1 WHERE user_id = ? AND persona_id = ?`, userID, personaID)
	if err != nil {
		return fmt.Errorf("set default: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Persona not assigned to user, assign it first
		_, err = tx.ExecContext(ctx,
			`INSERT INTO user_personas (user_id, persona_id, is_default, assigned_at) VALUES (?, ?, 1, ?)`,
			userID, personaID, time.Now())
		if err != nil {
			return fmt.Errorf("insert default persona: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// GetUserPersonas retrieves all personas assigned to a user.
func (s *Store) GetUserPersonas(ctx context.Context, userID string) ([]UserPersona, error) {
	query := `
		SELECT user_id, persona_id, is_default, assigned_at
		FROM user_personas
		WHERE user_id = ?
		ORDER BY is_default DESC, assigned_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query user personas: %w", err)
	}
	defer rows.Close()

	var personas []UserPersona
	for rows.Next() {
		var up UserPersona
		var isDefault int
		if err := rows.Scan(&up.UserID, &up.PersonaID, &isDefault, &up.AssignedAt); err != nil {
			return nil, fmt.Errorf("scan persona: %w", err)
		}
		up.IsDefault = isDefault == 1
		personas = append(personas, up)
	}

	return personas, rows.Err()
}

// GetUserDefaultPersonaID retrieves the default persona ID for a user.
func (s *Store) GetUserDefaultPersonaID(ctx context.Context, userID string) (string, error) {
	query := `SELECT persona_id FROM user_personas WHERE user_id = ? AND is_default = 1`

	var personaID string
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&personaID)
	if err == sql.ErrNoRows {
		return "", nil // No default set
	}
	if err != nil {
		return "", fmt.Errorf("get default persona: %w", err)
	}

	return personaID, nil
}

// ───────────────────────────────────────────────────────────────────────────────
// HELPERS
// ───────────────────────────────────────────────────────────────────────────────

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isUniqueConstraintError(err error) bool {
	// SQLite unique constraint error message contains "UNIQUE constraint failed"
	return err != nil && (
		contains(err.Error(), "UNIQUE constraint failed") ||
		contains(err.Error(), "unique constraint"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
