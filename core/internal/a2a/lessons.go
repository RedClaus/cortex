package a2a

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Lesson represents a saved learning session
type Lesson struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	PersonaID    string    `json:"persona_id"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	Status       string    `json:"status"` // active, completed, archived
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// LessonMessage represents a single message in a lesson
type LessonMessage struct {
	ID        int       `json:"id"`
	LessonID  string    `json:"lesson_id"`
	Role      string    `json:"role"` // user, assistant
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// LessonStore handles CRUD operations for lessons
type LessonStore struct {
	db *sql.DB
}

// NewLessonStore creates a new LessonStore
func NewLessonStore(db *sql.DB) *LessonStore {
	return &LessonStore{db: db}
}

// Create creates a new lesson
func (s *LessonStore) Create(ctx context.Context, userID, personaID, title string) (*Lesson, error) {
	id := "lesson_" + uuid.New().String()[:8]
	now := time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO lessons (id, user_id, persona_id, title, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'active', ?, ?)
	`, id, userID, personaID, title, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert lesson: %w", err)
	}

	return &Lesson{
		ID:        id,
		UserID:    userID,
		PersonaID: personaID,
		Title:     title,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Get retrieves a lesson by ID
func (s *LessonStore) Get(ctx context.Context, id string) (*Lesson, error) {
	var lesson Lesson
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, persona_id, title, COALESCE(summary, ''), status, message_count, created_at, updated_at
		FROM lessons
		WHERE id = ?
	`, id).Scan(
		&lesson.ID, &lesson.UserID, &lesson.PersonaID, &lesson.Title,
		&lesson.Summary, &lesson.Status, &lesson.MessageCount,
		&lesson.CreatedAt, &lesson.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("lesson not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query lesson: %w", err)
	}
	return &lesson, nil
}

// List retrieves lessons for a user, optionally filtered by persona
func (s *LessonStore) List(ctx context.Context, userID string, personaID string, limit int) ([]*Lesson, error) {
	if limit <= 0 {
		limit = 50
	}

	var rows *sql.Rows
	var err error

	if personaID != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, user_id, persona_id, title, COALESCE(summary, ''), status, message_count, created_at, updated_at
			FROM lessons
			WHERE user_id = ? AND persona_id = ?
			ORDER BY updated_at DESC
			LIMIT ?
		`, userID, personaID, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, user_id, persona_id, title, COALESCE(summary, ''), status, message_count, created_at, updated_at
			FROM lessons
			WHERE user_id = ?
			ORDER BY updated_at DESC
			LIMIT ?
		`, userID, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("query lessons: %w", err)
	}
	defer rows.Close()

	var lessons []*Lesson
	for rows.Next() {
		var lesson Lesson
		if err := rows.Scan(
			&lesson.ID, &lesson.UserID, &lesson.PersonaID, &lesson.Title,
			&lesson.Summary, &lesson.Status, &lesson.MessageCount,
			&lesson.CreatedAt, &lesson.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan lesson: %w", err)
		}
		lessons = append(lessons, &lesson)
	}

	return lessons, nil
}

// Delete removes a lesson and its messages
func (s *LessonStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM lessons WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete lesson: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("lesson not found: %s", id)
	}

	return nil
}

// UpdateStatus updates the lesson status
func (s *LessonStore) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE lessons SET status = ?, updated_at = ? WHERE id = ?
	`, status, time.Now(), id)
	return err
}

// UpdateSummary updates the lesson summary
func (s *LessonStore) UpdateSummary(ctx context.Context, id, summary string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE lessons SET summary = ?, updated_at = ? WHERE id = ?
	`, summary, time.Now(), id)
	return err
}

// AddMessage adds a message to a lesson
func (s *LessonStore) AddMessage(ctx context.Context, lessonID, role, content string) (*LessonMessage, error) {
	now := time.Now()

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO lesson_messages (lesson_id, role, content, created_at)
		VALUES (?, ?, ?, ?)
	`, lessonID, role, content, now)
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}

	id, _ := result.LastInsertId()

	return &LessonMessage{
		ID:        int(id),
		LessonID:  lessonID,
		Role:      role,
		Content:   content,
		CreatedAt: now,
	}, nil
}

// GetMessages retrieves all messages for a lesson
func (s *LessonStore) GetMessages(ctx context.Context, lessonID string) ([]*LessonMessage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, lesson_id, role, content, created_at
		FROM lesson_messages
		WHERE lesson_id = ?
		ORDER BY created_at ASC
	`, lessonID)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []*LessonMessage
	for rows.Next() {
		var msg LessonMessage
		if err := rows.Scan(&msg.ID, &msg.LessonID, &msg.Role, &msg.Content, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

// Search finds lessons matching a query using FTS
func (s *LessonStore) Search(ctx context.Context, userID, query string, limit int) ([]*Lesson, error) {
	if limit <= 0 {
		limit = 10
	}

	// Search in lesson titles/summaries and message content
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT l.id, l.user_id, l.persona_id, l.title, COALESCE(l.summary, ''),
		       l.status, l.message_count, l.created_at, l.updated_at
		FROM lessons l
		LEFT JOIN lesson_messages lm ON l.id = lm.lesson_id
		WHERE l.user_id = ? AND (
			l.title LIKE ? OR
			l.summary LIKE ? OR
			lm.content LIKE ?
		)
		ORDER BY l.updated_at DESC
		LIMIT ?
	`, userID, "%"+query+"%", "%"+query+"%", "%"+query+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("search lessons: %w", err)
	}
	defer rows.Close()

	var lessons []*Lesson
	for rows.Next() {
		var lesson Lesson
		if err := rows.Scan(
			&lesson.ID, &lesson.UserID, &lesson.PersonaID, &lesson.Title,
			&lesson.Summary, &lesson.Status, &lesson.MessageCount,
			&lesson.CreatedAt, &lesson.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan lesson: %w", err)
		}
		lessons = append(lessons, &lesson)
	}

	return lessons, nil
}

// GetRecentContext retrieves recent lesson messages for context injection
func (s *LessonStore) GetRecentContext(ctx context.Context, userID, personaID string, maxMessages int) (string, error) {
	if maxMessages <= 0 {
		maxMessages = 20
	}

	// Get recent messages from user's lessons with this persona
	rows, err := s.db.QueryContext(ctx, `
		SELECT lm.role, lm.content, l.title
		FROM lesson_messages lm
		JOIN lessons l ON lm.lesson_id = l.id
		WHERE l.user_id = ? AND l.persona_id = ?
		ORDER BY lm.created_at DESC
		LIMIT ?
	`, userID, personaID, maxMessages)
	if err != nil {
		return "", fmt.Errorf("query recent context: %w", err)
	}
	defer rows.Close()

	var context strings.Builder
	context.WriteString("## Previous Learning History\n\n")

	var messages []struct {
		Role    string
		Content string
		Title   string
	}

	for rows.Next() {
		var msg struct {
			Role    string
			Content string
			Title   string
		}
		if err := rows.Scan(&msg.Role, &msg.Content, &msg.Title); err != nil {
			return "", fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	// Reverse to show in chronological order
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		// Truncate long messages
		content := msg.Content
		if len(content) > 300 {
			content = content[:300] + "..."
		}
		context.WriteString(fmt.Sprintf("**%s**: %s\n\n", strings.Title(msg.Role), content))
	}

	return context.String(), nil
}

// GetRecentMessages returns recent messages as structured objects for multi-turn chat
// BRAIN AUDIT FIX: Returns proper message turns instead of formatted text
func (s *LessonStore) GetRecentMessages(ctx context.Context, userID, personaID string, maxMessages int) ([]*LessonMessage, error) {
	if maxMessages <= 0 {
		maxMessages = 10
	}

	// Get recent messages from user's lessons with this persona
	rows, err := s.db.QueryContext(ctx, `
		SELECT lm.id, lm.lesson_id, lm.role, lm.content, lm.created_at
		FROM lesson_messages lm
		JOIN lessons l ON lm.lesson_id = l.id
		WHERE l.user_id = ? AND l.persona_id = ?
		ORDER BY lm.created_at DESC
		LIMIT ?
	`, userID, personaID, maxMessages)
	if err != nil {
		return nil, fmt.Errorf("query recent messages: %w", err)
	}
	defer rows.Close()

	var messages []*LessonMessage
	for rows.Next() {
		msg := &LessonMessage{}
		if err := rows.Scan(&msg.ID, &msg.LessonID, &msg.Role, &msg.Content, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	// Reverse to show in chronological order (oldest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// SearchRelevant finds lessons relevant to the current query using FTS5 for semantic matching
func (s *LessonStore) SearchRelevant(ctx context.Context, userID, personaID, query string, limit int) (string, error) {
	if limit <= 0 {
		limit = 5
	}

	// Clean query for FTS - remove special characters that could break FTS syntax
	ftsQuery := cleanFTSQuery(query)
	if ftsQuery == "" {
		return "", nil
	}

	// Use FTS5 for semantic search with porter stemming
	// This will find "running" when searching for "run", etc.
	rows, err := s.db.QueryContext(ctx, `
		SELECT lm.role, lm.content, l.title,
		       bm25(lesson_messages_fts) AS rank
		FROM lesson_messages_fts fts
		JOIN lesson_messages lm ON fts.rowid = lm.id
		JOIN lessons l ON lm.lesson_id = l.id
		WHERE l.user_id = ? AND l.persona_id = ?
		  AND lesson_messages_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, userID, personaID, ftsQuery, limit)

	// If FTS fails, fall back to LIKE search
	if err != nil {
		return s.searchRelevantFallback(ctx, userID, personaID, query, limit)
	}
	defer rows.Close()

	var context strings.Builder
	context.WriteString("## Relevant Past Discussions\n\n")

	found := false
	for rows.Next() {
		var role, content, title string
		var rank float64
		if err := rows.Scan(&role, &content, &title, &rank); err != nil {
			continue
		}
		found = true
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		context.WriteString(fmt.Sprintf("From \"%s\":\n- %s: %s\n\n", title, role, content))
	}

	if !found {
		return "", nil
	}

	return context.String(), nil
}

// cleanFTSQuery prepares a query string for FTS5 MATCH
func cleanFTSQuery(query string) string {
	// Remove FTS special characters and build OR query from words
	words := strings.Fields(strings.ToLower(query))
	var cleanWords []string

	for _, word := range words {
		// Remove punctuation and special chars
		clean := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}
			return -1
		}, word)

		if len(clean) > 2 { // Skip short words
			cleanWords = append(cleanWords, clean)
		}
	}

	if len(cleanWords) == 0 {
		return ""
	}

	// Use OR between words for broader matching
	return strings.Join(cleanWords, " OR ")
}

// searchRelevantFallback uses LIKE queries when FTS is unavailable
func (s *LessonStore) searchRelevantFallback(ctx context.Context, userID, personaID, query string, limit int) (string, error) {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return "", nil
	}

	var conditions []string
	var args []interface{}
	args = append(args, userID, personaID)

	for _, word := range words {
		if len(word) > 2 {
			conditions = append(conditions, "lm.content LIKE ?")
			args = append(args, "%"+word+"%")
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}

	querySQL := fmt.Sprintf(`
		SELECT DISTINCT lm.role, lm.content, l.title
		FROM lesson_messages lm
		JOIN lessons l ON lm.lesson_id = l.id
		WHERE l.user_id = ? AND l.persona_id = ? AND (%s)
		ORDER BY lm.created_at DESC
		LIMIT ?
	`, strings.Join(conditions, " OR "))
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return "", fmt.Errorf("search relevant fallback: %w", err)
	}
	defer rows.Close()

	var context strings.Builder
	context.WriteString("## Relevant Past Discussions\n\n")

	found := false
	for rows.Next() {
		var role, content, title string
		if err := rows.Scan(&role, &content, &title); err != nil {
			continue
		}
		found = true
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		context.WriteString(fmt.Sprintf("From \"%s\":\n- %s: %s\n\n", title, role, content))
	}

	if !found {
		return "", nil
	}

	return context.String(), nil
}
