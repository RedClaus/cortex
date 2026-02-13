package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/facets"
	"github.com/normanking/cortex/internal/persona"
)

// ═══════════════════════════════════════════════════════════════════════════════
// PERSONA API HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// handlePersonas handles /api/v1/personas routes.
// Supports:
//   - GET /api/v1/personas - list all personas
//   - POST /api/v1/personas - create new persona
func (p *Prism) handlePersonas(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		p.handleListPersonas(w, r)
	case http.MethodPost:
		p.handleCreatePersona(w, r)
	default:
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handlePersonaByID handles /api/v1/personas/:id routes.
// Supports:
//   - GET /api/v1/personas/:id - get single persona
//   - PUT /api/v1/personas/:id - update persona
//   - DELETE /api/v1/personas/:id - delete persona
//   - POST /api/v1/personas/:id/compile - recompile system prompt
//   - POST /api/v1/personas/:id/preview - preview compiled prompt
func (p *Prism) handlePersonaByID(w http.ResponseWriter, r *http.Request) {
	// Parse persona ID from path: /api/v1/personas/{id} or /api/v1/personas/{id}/action
	path := r.URL.Path
	prefix := "/api/v1/personas/"

	if !strings.HasPrefix(path, prefix) {
		p.writeError(w, http.StatusNotFound, "not found")
		return
	}

	remainder := strings.TrimPrefix(path, prefix)
	parts := strings.Split(remainder, "/")

	if len(parts) == 0 || parts[0] == "" {
		p.writeError(w, http.StatusNotFound, "persona ID required")
		return
	}

	personaID := parts[0]

	// Check for action endpoints
	if len(parts) == 2 {
		action := parts[1]
		switch action {
		case "compile":
			p.handleCompilePersona(w, r, personaID)
		case "preview":
			p.handlePreviewPersona(w, r, personaID)
		default:
			p.writeError(w, http.StatusNotFound, "unknown action")
		}
		return
	}

	// Handle single persona operations
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			p.handleGetPersona(w, r, personaID)
		case http.MethodPut:
			p.handleUpdatePersona(w, r, personaID)
		case http.MethodDelete:
			p.handleDeletePersona(w, r, personaID)
		default:
			p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	p.writeError(w, http.StatusNotFound, "not found")
}

// handleListPersonas handles GET /api/v1/personas - list all personas.
// Supports optional ?role= query parameter for filtering by role.
func (p *Prism) handleListPersonas(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check for role filter
	roleFilter := r.URL.Query().Get("role")

	var personas []*facets.PersonaCore
	var err error

	if roleFilter != "" {
		// Filter by role
		personas, err = p.personaStore.ListByRole(ctx, roleFilter)
	} else {
		// List all
		personas, err = p.personaStore.List(ctx)
	}

	if err != nil {
		p.log.Warn("[Prism] Failed to list personas: %v", err)
		p.writeError(w, http.StatusInternalServerError, "failed to list personas")
		return
	}

	// Convert to response format
	responses := make([]PersonaResponse, len(personas))
	for i, persona := range personas {
		responses[i] = convertPersonaToResponse(persona)
	}

	response := PersonasResponse{
		Personas: responses,
		Total:    len(responses),
	}

	p.writeJSON(w, http.StatusOK, response)
}

// handleGetPersona handles GET /api/v1/personas/:id - get a single persona.
func (p *Prism) handleGetPersona(w http.ResponseWriter, r *http.Request, personaID string) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	persona, err := p.personaStore.Get(ctx, personaID)
	if err != nil {
		p.log.Warn("[Prism] Failed to get persona %s: %v", personaID, err)
		if strings.Contains(err.Error(), "not found") {
			p.writeError(w, http.StatusNotFound, "persona not found")
		} else {
			p.writeError(w, http.StatusInternalServerError, "failed to get persona")
		}
		return
	}

	response := convertPersonaToResponse(persona)
	p.writeJSON(w, http.StatusOK, response)
}

// handleCreatePersona handles POST /api/v1/personas - create new persona.
func (p *Prism) handleCreatePersona(w http.ResponseWriter, r *http.Request) {
	var req CreatePersonaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		p.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Role == "" {
		p.writeError(w, http.StatusBadRequest, "role is required")
		return
	}

	// Convert request to PersonaCore
	persona := &facets.PersonaCore{
		Name:               req.Name,
		Role:               req.Role,
		Background:         req.Background,
		Traits:             req.Traits,
		Values:             req.Values,
		Expertise:          convertExpertiseDomains(req.Expertise),
		Style:              convertCommunicationStyle(req.Style),
		Modes:              convertBehavioralModes(req.Modes),
		DefaultMode:        req.DefaultMode,
		KnowledgeSourceIDs: req.KnowledgeSourceIDs,
		IsBuiltIn:          false,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := p.personaStore.Create(ctx, persona); err != nil {
		p.log.Warn("[Prism] Failed to create persona: %v", err)
		if strings.Contains(err.Error(), "validate") {
			p.writeError(w, http.StatusBadRequest, err.Error())
		} else {
			p.writeError(w, http.StatusInternalServerError, "failed to create persona")
		}
		return
	}

	p.log.Info("[Prism] Created persona: %s (%s)", persona.Name, persona.ID)

	response := convertPersonaToResponse(persona)
	p.writeJSON(w, http.StatusCreated, response)
}

// handleUpdatePersona handles PUT /api/v1/personas/:id - update a persona.
func (p *Prism) handleUpdatePersona(w http.ResponseWriter, r *http.Request, personaID string) {
	var req UpdatePersonaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get existing persona
	persona, err := p.personaStore.Get(ctx, personaID)
	if err != nil {
		p.log.Warn("[Prism] Failed to get persona %s: %v", personaID, err)
		if strings.Contains(err.Error(), "not found") {
			p.writeError(w, http.StatusNotFound, "persona not found")
		} else {
			p.writeError(w, http.StatusInternalServerError, "failed to get persona")
		}
		return
	}

	// Update fields if provided
	if req.Name != "" {
		persona.Name = req.Name
	}
	if req.Role != "" {
		persona.Role = req.Role
	}
	if req.Background != "" {
		persona.Background = req.Background
	}
	if req.Traits != nil {
		persona.Traits = req.Traits
	}
	if req.Values != nil {
		persona.Values = req.Values
	}
	if req.Expertise != nil {
		persona.Expertise = convertExpertiseDomains(req.Expertise)
	}
	if req.Style != nil {
		persona.Style = convertCommunicationStyle(*req.Style)
	}
	if req.Modes != nil {
		persona.Modes = convertBehavioralModes(req.Modes)
	}
	if req.DefaultMode != "" {
		persona.DefaultMode = req.DefaultMode
	}
	if req.KnowledgeSourceIDs != nil {
		persona.KnowledgeSourceIDs = req.KnowledgeSourceIDs
	}

	if err := p.personaStore.Update(ctx, persona); err != nil {
		p.log.Warn("[Prism] Failed to update persona %s: %v", personaID, err)
		if strings.Contains(err.Error(), "built-in") {
			p.writeError(w, http.StatusForbidden, "cannot update built-in persona")
		} else if strings.Contains(err.Error(), "validate") {
			p.writeError(w, http.StatusBadRequest, err.Error())
		} else {
			p.writeError(w, http.StatusInternalServerError, "failed to update persona")
		}
		return
	}

	p.log.Info("[Prism] Updated persona: %s (%s)", persona.Name, persona.ID)

	response := convertPersonaToResponse(persona)
	p.writeJSON(w, http.StatusOK, response)
}

// handleDeletePersona handles DELETE /api/v1/personas/:id - delete a persona.
func (p *Prism) handleDeletePersona(w http.ResponseWriter, r *http.Request, personaID string) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := p.personaStore.Delete(ctx, personaID); err != nil {
		p.log.Warn("[Prism] Failed to delete persona %s: %v", personaID, err)
		if strings.Contains(err.Error(), "not found") {
			p.writeError(w, http.StatusNotFound, "persona not found")
		} else if strings.Contains(err.Error(), "built-in") {
			p.writeError(w, http.StatusForbidden, "cannot delete built-in persona")
		} else {
			p.writeError(w, http.StatusInternalServerError, "failed to delete persona")
		}
		return
	}

	p.log.Info("[Prism] Deleted persona: %s", personaID)
	w.WriteHeader(http.StatusNoContent)
}

// handleCompilePersona handles POST /api/v1/personas/:id/compile - recompile system prompt.
func (p *Prism) handleCompilePersona(w http.ResponseWriter, r *http.Request, personaID string) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	persona, err := p.personaStore.Get(ctx, personaID)
	if err != nil {
		p.log.Warn("[Prism] Failed to get persona %s: %v", personaID, err)
		if strings.Contains(err.Error(), "not found") {
			p.writeError(w, http.StatusNotFound, "persona not found")
		} else {
			p.writeError(w, http.StatusInternalServerError, "failed to get persona")
		}
		return
	}

	// Recompile system prompt
	persona.SystemPrompt = persona.CompileSystemPrompt()
	persona.UpdatedAt = time.Now()

	if err := p.personaStore.Update(ctx, persona); err != nil {
		p.log.Warn("[Prism] Failed to update persona %s: %v", personaID, err)
		p.writeError(w, http.StatusInternalServerError, "failed to compile persona")
		return
	}

	response := CompilePromptResponse{
		SystemPrompt: persona.SystemPrompt,
		UpdatedAt:    persona.UpdatedAt.Format(time.RFC3339),
	}

	p.writeJSON(w, http.StatusOK, response)
}

// handlePreviewPersona handles POST /api/v1/personas/:id/preview - preview compiled prompt.
func (p *Prism) handlePreviewPersona(w http.ResponseWriter, r *http.Request, personaID string) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	persona, err := p.personaStore.Get(ctx, personaID)
	if err != nil {
		p.log.Warn("[Prism] Failed to get persona %s: %v", personaID, err)
		if strings.Contains(err.Error(), "not found") {
			p.writeError(w, http.StatusNotFound, "persona not found")
		} else {
			p.writeError(w, http.StatusInternalServerError, "failed to get persona")
		}
		return
	}

	// Compile system prompt without saving
	systemPrompt := persona.CompileSystemPrompt()

	response := PreviewPromptResponse{
		SystemPrompt: systemPrompt,
	}

	p.writeJSON(w, http.StatusOK, response)
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONVERSION HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// convertPersonaToResponse converts a facets.PersonaCore to a PersonaResponse.
func convertPersonaToResponse(persona *facets.PersonaCore) PersonaResponse {
	return PersonaResponse{
		ID:                 persona.ID,
		Version:            persona.Version,
		Name:               persona.Name,
		Role:               persona.Role,
		Background:         persona.Background,
		Traits:             persona.Traits,
		Values:             persona.Values,
		Expertise:          convertExpertiseDomainsFromFacets(persona.Expertise),
		Style:              convertCommunicationStyleFromFacets(persona.Style),
		Modes:              convertBehavioralModesFromFacets(persona.Modes),
		DefaultMode:        persona.DefaultMode,
		KnowledgeSourceIDs: persona.KnowledgeSourceIDs,
		SystemPrompt:       persona.SystemPrompt,
		CompiledPrompt:     persona.SystemPrompt, // Alias for clarity
		IsBuiltIn:          persona.IsBuiltIn,
		CreatedAt:          persona.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          persona.UpdatedAt.Format(time.RFC3339),
	}
}

// convertExpertiseDomains converts API ExpertiseDomain to facets.ExpertiseDomain.
func convertExpertiseDomains(domains []ExpertiseDomain) []facets.ExpertiseDomain {
	result := make([]facets.ExpertiseDomain, len(domains))
	for i, d := range domains {
		result[i] = facets.ExpertiseDomain{
			Domain:      d.Domain,
			Depth:       d.Depth,
			Specialties: d.Specialties,
			Boundaries:  d.Boundaries,
		}
	}
	return result
}

// convertExpertiseDomainsFromFacets converts facets.ExpertiseDomain to API ExpertiseDomain.
func convertExpertiseDomainsFromFacets(domains []facets.ExpertiseDomain) []ExpertiseDomain {
	result := make([]ExpertiseDomain, len(domains))
	for i, d := range domains {
		result[i] = ExpertiseDomain{
			Domain:      d.Domain,
			Depth:       d.Depth,
			Specialties: d.Specialties,
			Boundaries:  d.Boundaries,
		}
	}
	return result
}

// convertCommunicationStyle converts API CommunicationStyle to facets.CommunicationStyle.
func convertCommunicationStyle(style CommunicationStyle) facets.CommunicationStyle {
	return facets.CommunicationStyle{
		Tone:       style.Tone,
		Verbosity:  style.Verbosity,
		Formatting: style.Formatting,
		Patterns:   style.Patterns,
		Avoids:     style.Avoids,
	}
}

// convertCommunicationStyleFromFacets converts facets.CommunicationStyle to API CommunicationStyle.
func convertCommunicationStyleFromFacets(style facets.CommunicationStyle) CommunicationStyle {
	return CommunicationStyle{
		Tone:       style.Tone,
		Verbosity:  style.Verbosity,
		Formatting: style.Formatting,
		Patterns:   style.Patterns,
		Avoids:     style.Avoids,
	}
}

// convertBehavioralModes converts API BehavioralMode to facets.BehavioralMode.
func convertBehavioralModes(modes []BehavioralMode) []facets.BehavioralMode {
	result := make([]facets.BehavioralMode, len(modes))
	for i, m := range modes {
		result[i] = facets.BehavioralMode{
			ID:            m.ID,
			Name:          m.Name,
			Description:   m.Description,
			PromptAugment: m.PromptAugment,
			EntryKeywords: m.EntryKeywords,
			ExitKeywords:  m.ExitKeywords,
			ManualTrigger: m.ManualTrigger,
			ForceVerbose:  m.ForceVerbose,
			ForceConcise:  m.ForceConcise,
			SortOrder:     m.SortOrder,
		}
	}
	return result
}

// convertBehavioralModesFromFacets converts facets.BehavioralMode to API BehavioralMode.
func convertBehavioralModesFromFacets(modes []facets.BehavioralMode) []BehavioralMode {
	result := make([]BehavioralMode, len(modes))
	for i, m := range modes {
		result[i] = BehavioralMode{
			ID:            m.ID,
			Name:          m.Name,
			Description:   m.Description,
			PromptAugment: m.PromptAugment,
			EntryKeywords: m.EntryKeywords,
			ExitKeywords:  m.ExitKeywords,
			ManualTrigger: m.ManualTrigger,
			ForceVerbose:  m.ForceVerbose,
			ForceConcise:  m.ForceConcise,
			SortOrder:     m.SortOrder,
		}
	}
	return result
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODE API HANDLERS (CR-011)
// ═══════════════════════════════════════════════════════════════════════════════

// handleListModes handles GET /api/v1/modes - list available behavioral modes.
// This returns the standard modes from the persona package.
func (p *Prism) handleListModes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get built-in personas to extract their modes
	// We'll use the mode tracker's registered personas
	modes := []ModeResponse{
		{
			ID:          string(persona.ModeNormal),
			Name:        "Normal",
			Description: "Standard assistance mode",
		},
		{
			ID:          string(persona.ModeDebugging),
			Name:        "Debugging",
			Description: "Focused troubleshooting and debugging mode",
		},
		{
			ID:          string(persona.ModeTeaching),
			Name:        "Teaching",
			Description: "Educational mode with detailed explanations",
		},
		{
			ID:          string(persona.ModePair),
			Name:        "Pair Programming",
			Description: "Collaborative coding mode",
		},
		{
			ID:          string(persona.ModeReview),
			Name:        "Code Review",
			Description: "Code review and quality analysis mode",
		},
	}

	response := ModesResponse{
		Modes: modes,
		Total: len(modes),
	}

	p.writeJSON(w, http.StatusOK, response)
}

// handleGetCurrentMode handles GET /api/v1/mode/current - get current mode.
// This returns information about the currently active behavioral mode.
// Note: This is a placeholder for when orchestrator integration is complete.
func (p *Prism) handleGetCurrentMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// For now, return normal mode as default
	// When orchestrator is integrated with the server, this would call
	// orchestrator.GetActiveMode()
	response := ModeResponse{
		ID:          string(persona.ModeNormal),
		Name:        "Normal",
		Description: "Standard assistance mode",
	}

	p.writeJSON(w, http.StatusOK, response)
}

// handleSetMode handles POST /api/v1/mode/set - set active mode.
// This is a placeholder for when orchestrator integration is complete.
func (p *Prism) handleSetMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req SetModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ModeID == "" {
		p.writeError(w, http.StatusBadRequest, "mode_id is required")
		return
	}

	// Validate mode ID
	validModes := map[string]bool{
		string(persona.ModeNormal):    true,
		string(persona.ModeDebugging): true,
		string(persona.ModeTeaching):  true,
		string(persona.ModePair):      true,
		string(persona.ModeReview):    true,
	}

	if !validModes[req.ModeID] {
		p.writeError(w, http.StatusBadRequest, "invalid mode_id")
		return
	}

	// When orchestrator is integrated with the server, this would call:
	// orchestrator.SetMode(persona.ModeType(req.ModeID))

	p.log.Info("[Prism] Mode set to: %s (orchestrator integration pending)", req.ModeID)

	// Return success
	response := ModeResponse{
		ID:          req.ModeID,
		Name:        getModeDisplayName(req.ModeID),
		Description: getModeDescription(req.ModeID),
	}

	p.writeJSON(w, http.StatusOK, response)
}

// getModeDisplayName returns the display name for a mode ID.
func getModeDisplayName(modeID string) string {
	switch persona.ModeType(modeID) {
	case persona.ModeNormal:
		return "Normal"
	case persona.ModeDebugging:
		return "Debugging"
	case persona.ModeTeaching:
		return "Teaching"
	case persona.ModePair:
		return "Pair Programming"
	case persona.ModeReview:
		return "Code Review"
	default:
		return modeID
	}
}

// getModeDescription returns the description for a mode ID.
func getModeDescription(modeID string) string {
	switch persona.ModeType(modeID) {
	case persona.ModeNormal:
		return "Standard assistance mode"
	case persona.ModeDebugging:
		return "Focused troubleshooting and debugging mode"
	case persona.ModeTeaching:
		return "Educational mode with detailed explanations"
	case persona.ModePair:
		return "Collaborative coding mode"
	case persona.ModeReview:
		return "Code review and quality analysis mode"
	default:
		return ""
	}
}
