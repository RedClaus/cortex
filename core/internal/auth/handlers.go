package auth

import (
	"encoding/json"
	"net/http"
)

// Handlers provides HTTP handlers for authentication endpoints.
type Handlers struct {
	service *Service
}

// NewHandlers creates new auth handlers.
func NewHandlers(service *Service) *Handlers {
	return &Handlers{service: service}
}

// RegisterRoutes registers auth routes on a mux.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("POST /api/auth/refresh", h.Refresh)
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/auth/me", h.Me)
}

// Register handles user registration.
// POST /api/auth/register
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Validate required fields
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "Username and password are required")
		return
	}

	user, err := h.service.Register(r.Context(), &req)
	if err != nil {
		if authErr, ok := err.(*AuthError); ok {
			status := http.StatusBadRequest
			if authErr == ErrUserExists {
				status = http.StatusConflict
			}
			writeError(w, status, authErr.Code, authErr.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "REGISTER_FAILED", "Failed to register user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user": user.ToResponse(),
	})
}

// Login handles user login.
// POST /api/auth/login
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Validate required fields
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "Username and password are required")
		return
	}

	// Get client info
	userAgent := r.Header.Get("User-Agent")
	ipAddress := getClientIP(r)

	resp, err := h.service.Login(r.Context(), &req, userAgent, ipAddress)
	if err != nil {
		if authErr, ok := err.(*AuthError); ok {
			status := http.StatusUnauthorized
			if authErr == ErrUserDisabled {
				status = http.StatusForbidden
			}
			writeError(w, status, authErr.Code, authErr.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to login")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":   resp.User.ToResponse(),
		"tokens": resp.Tokens,
	})
}

// Refresh handles token refresh.
// POST /api/auth/refresh
func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "MISSING_TOKEN", "Refresh token is required")
		return
	}

	// Get client info
	userAgent := r.Header.Get("User-Agent")
	ipAddress := getClientIP(r)

	tokens, err := h.service.RefreshToken(r.Context(), req.RefreshToken, userAgent, ipAddress)
	if err != nil {
		if authErr, ok := err.(*AuthError); ok {
			writeError(w, http.StatusUnauthorized, authErr.Code, authErr.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "REFRESH_FAILED", "Failed to refresh token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tokens": tokens,
	})
}

// Logout handles user logout.
// POST /api/auth/logout
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If no body, try to get token from header
		req.RefreshToken = ""
	}

	if req.RefreshToken != "" {
		h.service.Logout(r.Context(), req.RefreshToken)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// Me returns the current authenticated user.
// GET /api/auth/me
// Requires: Authorization header with Bearer token
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	// Extract token from header
	token := extractBearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "MISSING_TOKEN", "Authorization token required")
		return
	}

	user, err := h.service.GetUserFromToken(r.Context(), token)
	if err != nil {
		if authErr, ok := err.(*AuthError); ok {
			writeError(w, http.StatusUnauthorized, authErr.Code, authErr.Message)
			return
		}
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid token")
		return
	}

	// Get user's personas
	personas, _ := h.service.GetUserPersonas(r.Context(), user.ID)
	defaultPersonaID, _ := h.service.GetUserDefaultPersonaID(r.Context(), user.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":             user.ToResponse(),
		"personas":         personas,
		"defaultPersonaId": defaultPersonaID,
	})
}

// ───────────────────────────────────────────────────────────────────────────────
// USER-PERSONA HANDLERS
// ───────────────────────────────────────────────────────────────────────────────

// GetUserPersonas returns personas assigned to a user.
// GET /api/v1/users/:userId/personas
func (h *Handlers) GetUserPersonas(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_USER_ID", "User ID is required")
		return
	}

	personas, err := h.service.GetUserPersonas(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch personas")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"personas": personas,
	})
}

// AssignPersona assigns a persona to a user.
// POST /api/v1/users/:userId/personas/:personaId
func (h *Handlers) AssignPersona(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	personaID := r.PathValue("personaId")

	if userID == "" || personaID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_IDS", "User ID and Persona ID are required")
		return
	}

	// Check for isDefault in query or body
	isDefault := r.URL.Query().Get("default") == "true"

	if err := h.service.AssignPersona(r.Context(), userID, personaID, isDefault); err != nil {
		writeError(w, http.StatusInternalServerError, "ASSIGN_FAILED", "Failed to assign persona")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// UnassignPersona removes a persona from a user.
// DELETE /api/v1/users/:userId/personas/:personaId
func (h *Handlers) UnassignPersona(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	personaID := r.PathValue("personaId")

	if userID == "" || personaID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_IDS", "User ID and Persona ID are required")
		return
	}

	if err := h.service.UnassignPersona(r.Context(), userID, personaID); err != nil {
		writeError(w, http.StatusInternalServerError, "UNASSIGN_FAILED", "Failed to unassign persona")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// SetDefaultPersona sets a persona as default for a user.
// PUT /api/v1/users/:userId/personas/:personaId/default
func (h *Handlers) SetDefaultPersona(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	personaID := r.PathValue("personaId")

	if userID == "" || personaID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_IDS", "User ID and Persona ID are required")
		return
	}

	if err := h.service.SetDefaultPersona(r.Context(), userID, personaID); err != nil {
		writeError(w, http.StatusInternalServerError, "SET_DEFAULT_FAILED", "Failed to set default persona")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// ───────────────────────────────────────────────────────────────────────────────
// HELPERS
// ───────────────────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(auth) > len(prefix) && auth[:len(prefix)] == prefix {
		return auth[len(prefix):]
	}
	return ""
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	// Remove port if present
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
