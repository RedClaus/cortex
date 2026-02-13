package facets

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PersonaStore provides database persistence for personas.
type PersonaStore struct {
	db *sql.DB
}

// NewPersonaStore creates a new PersonaStore with the given database connection.
func NewPersonaStore(db *sql.DB) *PersonaStore {
	return &PersonaStore{db: db}
}

// Create inserts a new persona into the database.
// The persona's SystemPrompt is auto-generated via CompileSystemPrompt().
func (s *PersonaStore) Create(ctx context.Context, persona *PersonaCore) error {
	if persona.ID == "" {
		persona.ID = uuid.New().String()
	}
	if persona.Version == "" {
		persona.Version = "1.0"
	}

	// Validate before inserting
	if err := persona.Validate(); err != nil {
		return fmt.Errorf("validate persona: %w", err)
	}

	// Compile system prompt
	persona.SystemPrompt = persona.CompileSystemPrompt()

	// Set timestamps
	now := time.Now()
	persona.CreatedAt = now
	persona.UpdatedAt = now

	// Serialize JSON fields
	traitsJSON, err := json.Marshal(persona.Traits)
	if err != nil {
		return fmt.Errorf("marshal traits: %w", err)
	}

	valuesJSON, err := json.Marshal(persona.Values)
	if err != nil {
		return fmt.Errorf("marshal values: %w", err)
	}

	expertiseJSON, err := json.Marshal(persona.Expertise)
	if err != nil {
		return fmt.Errorf("marshal expertise: %w", err)
	}

	styleJSON, err := json.Marshal(persona.Style)
	if err != nil {
		return fmt.Errorf("marshal style: %w", err)
	}

	modesJSON, err := json.Marshal(persona.Modes)
	if err != nil {
		return fmt.Errorf("marshal modes: %w", err)
	}

	knowledgeSourcesJSON, err := json.Marshal(persona.KnowledgeSourceIDs)
	if err != nil {
		return fmt.Errorf("marshal knowledge_source_ids: %w", err)
	}

	// Insert into database
	query := `
		INSERT INTO personas (
			id, version, name, role, background, traits, "values", expertise,
			style, modes, default_mode, knowledge_source_ids, system_prompt,
			is_built_in, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		persona.ID,
		persona.Version,
		persona.Name,
		persona.Role,
		persona.Background,
		traitsJSON,
		valuesJSON,
		expertiseJSON,
		styleJSON,
		modesJSON,
		persona.DefaultMode,
		knowledgeSourcesJSON,
		persona.SystemPrompt,
		persona.IsBuiltIn,
		persona.CreatedAt,
		persona.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert persona: %w", err)
	}

	return nil
}

// Get retrieves a persona by ID.
func (s *PersonaStore) Get(ctx context.Context, id string) (*PersonaCore, error) {
	query := `
		SELECT id, version, name, role, background, traits, "values", expertise,
		       style, modes, default_mode, knowledge_source_ids, system_prompt,
		       is_built_in, created_at, updated_at
		FROM personas
		WHERE id = ?
	`

	var persona PersonaCore
	var traitsJSON, valuesJSON, expertiseJSON, styleJSON, modesJSON, knowledgeSourcesJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&persona.ID,
		&persona.Version,
		&persona.Name,
		&persona.Role,
		&persona.Background,
		&traitsJSON,
		&valuesJSON,
		&expertiseJSON,
		&styleJSON,
		&modesJSON,
		&persona.DefaultMode,
		&knowledgeSourcesJSON,
		&persona.SystemPrompt,
		&persona.IsBuiltIn,
		&persona.CreatedAt,
		&persona.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("persona not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query persona: %w", err)
	}

	// Deserialize JSON fields
	if err := json.Unmarshal(traitsJSON, &persona.Traits); err != nil {
		return nil, fmt.Errorf("unmarshal traits: %w", err)
	}
	if err := json.Unmarshal(valuesJSON, &persona.Values); err != nil {
		return nil, fmt.Errorf("unmarshal values: %w", err)
	}
	if err := json.Unmarshal(expertiseJSON, &persona.Expertise); err != nil {
		return nil, fmt.Errorf("unmarshal expertise: %w", err)
	}
	if err := json.Unmarshal(styleJSON, &persona.Style); err != nil {
		return nil, fmt.Errorf("unmarshal style: %w", err)
	}
	if err := json.Unmarshal(modesJSON, &persona.Modes); err != nil {
		return nil, fmt.Errorf("unmarshal modes: %w", err)
	}
	if err := json.Unmarshal(knowledgeSourcesJSON, &persona.KnowledgeSourceIDs); err != nil {
		return nil, fmt.Errorf("unmarshal knowledge_source_ids: %w", err)
	}

	return &persona, nil
}

// List retrieves all personas, optionally filtered by built-in status.
func (s *PersonaStore) List(ctx context.Context) ([]*PersonaCore, error) {
	query := `
		SELECT id, version, name, role, background, traits, "values", expertise,
		       style, modes, default_mode, knowledge_source_ids, system_prompt,
		       is_built_in, created_at, updated_at
		FROM personas
		ORDER BY is_built_in DESC, name ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query personas: %w", err)
	}
	defer rows.Close()

	var personas []*PersonaCore
	for rows.Next() {
		var persona PersonaCore
		var traitsJSON, valuesJSON, expertiseJSON, styleJSON, modesJSON, knowledgeSourcesJSON []byte

		err := rows.Scan(
			&persona.ID,
			&persona.Version,
			&persona.Name,
			&persona.Role,
			&persona.Background,
			&traitsJSON,
			&valuesJSON,
			&expertiseJSON,
			&styleJSON,
			&modesJSON,
			&persona.DefaultMode,
			&knowledgeSourcesJSON,
			&persona.SystemPrompt,
			&persona.IsBuiltIn,
			&persona.CreatedAt,
			&persona.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan persona: %w", err)
		}

		// Deserialize JSON fields
		if err := json.Unmarshal(traitsJSON, &persona.Traits); err != nil {
			return nil, fmt.Errorf("unmarshal traits: %w", err)
		}
		if err := json.Unmarshal(valuesJSON, &persona.Values); err != nil {
			return nil, fmt.Errorf("unmarshal values: %w", err)
		}
		if err := json.Unmarshal(expertiseJSON, &persona.Expertise); err != nil {
			return nil, fmt.Errorf("unmarshal expertise: %w", err)
		}
		if err := json.Unmarshal(styleJSON, &persona.Style); err != nil {
			return nil, fmt.Errorf("unmarshal style: %w", err)
		}
		if err := json.Unmarshal(modesJSON, &persona.Modes); err != nil {
			return nil, fmt.Errorf("unmarshal modes: %w", err)
		}
		if err := json.Unmarshal(knowledgeSourcesJSON, &persona.KnowledgeSourceIDs); err != nil {
			return nil, fmt.Errorf("unmarshal knowledge_source_ids: %w", err)
		}

		personas = append(personas, &persona)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate personas: %w", err)
	}

	return personas, nil
}

// Update updates an existing persona.
// Built-in personas cannot be updated.
func (s *PersonaStore) Update(ctx context.Context, persona *PersonaCore) error {
	// Check if persona exists and is not built-in
	existing, err := s.Get(ctx, persona.ID)
	if err != nil {
		return err
	}

	if existing.IsBuiltIn {
		return fmt.Errorf("cannot update built-in persona: %s", persona.ID)
	}

	// Validate before updating
	if err := persona.Validate(); err != nil {
		return fmt.Errorf("validate persona: %w", err)
	}

	// Recompile system prompt
	persona.SystemPrompt = persona.CompileSystemPrompt()

	// Update timestamp
	persona.UpdatedAt = time.Now()

	// Serialize JSON fields
	traitsJSON, err := json.Marshal(persona.Traits)
	if err != nil {
		return fmt.Errorf("marshal traits: %w", err)
	}

	valuesJSON, err := json.Marshal(persona.Values)
	if err != nil {
		return fmt.Errorf("marshal values: %w", err)
	}

	expertiseJSON, err := json.Marshal(persona.Expertise)
	if err != nil {
		return fmt.Errorf("marshal expertise: %w", err)
	}

	styleJSON, err := json.Marshal(persona.Style)
	if err != nil {
		return fmt.Errorf("marshal style: %w", err)
	}

	modesJSON, err := json.Marshal(persona.Modes)
	if err != nil {
		return fmt.Errorf("marshal modes: %w", err)
	}

	knowledgeSourcesJSON, err := json.Marshal(persona.KnowledgeSourceIDs)
	if err != nil {
		return fmt.Errorf("marshal knowledge_source_ids: %w", err)
	}

	// Update in database
	query := `
		UPDATE personas
		SET version = ?, name = ?, role = ?, background = ?, traits = ?, "values" = ?,
		    expertise = ?, style = ?, modes = ?, default_mode = ?,
		    knowledge_source_ids = ?, system_prompt = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query,
		persona.Version,
		persona.Name,
		persona.Role,
		persona.Background,
		traitsJSON,
		valuesJSON,
		expertiseJSON,
		styleJSON,
		modesJSON,
		persona.DefaultMode,
		knowledgeSourcesJSON,
		persona.SystemPrompt,
		persona.UpdatedAt,
		persona.ID,
	)

	if err != nil {
		return fmt.Errorf("update persona: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("persona not found: %s", persona.ID)
	}

	return nil
}

// Delete removes a persona by ID.
// Built-in personas cannot be deleted.
func (s *PersonaStore) Delete(ctx context.Context, id string) error {
	// Check if persona exists and is not built-in
	persona, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if persona.IsBuiltIn {
		return fmt.Errorf("cannot delete built-in persona: %s", id)
	}

	// Delete from database
	query := `DELETE FROM personas WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete persona: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("persona not found: %s", id)
	}

	return nil
}

// ListByRole retrieves personas filtered by role (case-insensitive partial match).
func (s *PersonaStore) ListByRole(ctx context.Context, role string) ([]*PersonaCore, error) {
	query := `
		SELECT id, version, name, role, background, traits, "values", expertise,
		       style, modes, default_mode, knowledge_source_ids, system_prompt,
		       is_built_in, created_at, updated_at
		FROM personas
		WHERE role LIKE ?
		ORDER BY is_built_in DESC, name ASC
	`

	rows, err := s.db.QueryContext(ctx, query, "%"+role+"%")
	if err != nil {
		return nil, fmt.Errorf("query personas by role: %w", err)
	}
	defer rows.Close()

	var personas []*PersonaCore
	for rows.Next() {
		var persona PersonaCore
		var traitsJSON, valuesJSON, expertiseJSON, styleJSON, modesJSON, knowledgeSourcesJSON []byte

		err := rows.Scan(
			&persona.ID,
			&persona.Version,
			&persona.Name,
			&persona.Role,
			&persona.Background,
			&traitsJSON,
			&valuesJSON,
			&expertiseJSON,
			&styleJSON,
			&modesJSON,
			&persona.DefaultMode,
			&knowledgeSourcesJSON,
			&persona.SystemPrompt,
			&persona.IsBuiltIn,
			&persona.CreatedAt,
			&persona.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan persona: %w", err)
		}

		// Deserialize JSON fields
		json.Unmarshal(traitsJSON, &persona.Traits)
		json.Unmarshal(valuesJSON, &persona.Values)
		json.Unmarshal(expertiseJSON, &persona.Expertise)
		json.Unmarshal(styleJSON, &persona.Style)
		json.Unmarshal(modesJSON, &persona.Modes)
		json.Unmarshal(knowledgeSourcesJSON, &persona.KnowledgeSourceIDs)

		personas = append(personas, &persona)
	}

	return personas, rows.Err()
}

// InitBuiltIns initializes the built-in personas in the database.
// This is idempotent - existing built-ins are not re-inserted.
func (s *PersonaStore) InitBuiltIns(ctx context.Context) error {
	// Initialize built-in personas (compiles system prompts)
	InitializeBuiltInPersonas()

	// Insert each built-in persona if it doesn't exist
	for i := range BuiltInPersonas {
		persona := &BuiltInPersonas[i]

		// Check if persona already exists
		_, err := s.Get(ctx, persona.ID)
		if err == nil {
			// Persona already exists, skip
			continue
		}

		// Insert built-in persona
		if err := s.Create(ctx, persona); err != nil {
			return fmt.Errorf("insert built-in persona %s: %w", persona.ID, err)
		}
	}

	return nil
}
