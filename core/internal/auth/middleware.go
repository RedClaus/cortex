package auth

import (
	"context"
	"net/http"
	"strings"
)

// Context keys for storing auth data in request context
type contextKey string

const (
	// UserContextKey is the context key for the authenticated user
	UserContextKey contextKey = "auth_user"
	// ClaimsContextKey is the context key for JWT claims
	ClaimsContextKey contextKey = "auth_claims"
)

// Middleware provides HTTP middleware for authentication.
type Middleware struct {
	service *Service
}

// NewMiddleware creates a new auth middleware.
func NewMiddleware(service *Service) *Middleware {
	return &Middleware{service: service}
}

// RequireAuth is middleware that requires a valid access token.
// Requests without valid auth will receive a 401 response.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := m.extractAndValidateToken(r)
		if err != nil {
			writeAuthError(w, err)
			return
		}

		// Get user from database
		user, err := m.service.store.GetUserByID(r.Context(), claims.UserID)
		if err != nil {
			writeAuthError(w, ErrInvalidToken)
			return
		}

		if !user.IsActive {
			writeAuthError(w, ErrUserDisabled)
			return
		}

		// Add user and claims to context
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		ctx = context.WithValue(ctx, ClaimsContextKey, claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth is middleware that validates auth if present, but doesn't require it.
// Useful for endpoints that have different behavior for authenticated vs anonymous users.
func (m *Middleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := m.extractAndValidateToken(r)
		if err != nil {
			// No valid auth, but that's okay - continue without user context
			next.ServeHTTP(w, r)
			return
		}

		// Get user from database
		user, err := m.service.store.GetUserByID(r.Context(), claims.UserID)
		if err != nil || !user.IsActive {
			// Invalid user, continue without user context
			next.ServeHTTP(w, r)
			return
		}

		// Add user and claims to context
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		ctx = context.WithValue(ctx, ClaimsContextKey, claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractAndValidateToken extracts the token from the Authorization header and validates it.
func (m *Middleware) extractAndValidateToken(r *http.Request) (*Claims, error) {
	// Get Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, ErrMissingToken
	}

	// Check for Bearer prefix
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, ErrInvalidToken
	}

	// Extract token
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return nil, ErrMissingToken
	}

	// Validate token
	return m.service.ValidateToken(token)
}

// ───────────────────────────────────────────────────────────────────────────────
// CONTEXT HELPERS
// ───────────────────────────────────────────────────────────────────────────────

// UserFromContext retrieves the authenticated user from the request context.
// Returns nil if no user is authenticated.
func UserFromContext(ctx context.Context) *User {
	user, ok := ctx.Value(UserContextKey).(*User)
	if !ok {
		return nil
	}
	return user
}

// ClaimsFromContext retrieves the JWT claims from the request context.
// Returns nil if no claims are present.
func ClaimsFromContext(ctx context.Context) *Claims {
	claims, ok := ctx.Value(ClaimsContextKey).(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// UserIDFromContext retrieves the user ID from the request context.
// Returns empty string if no user is authenticated.
func UserIDFromContext(ctx context.Context) string {
	user := UserFromContext(ctx)
	if user == nil {
		return ""
	}
	return user.ID
}

// ───────────────────────────────────────────────────────────────────────────────
// ERROR RESPONSE HELPER
// ───────────────────────────────────────────────────────────────────────────────

func writeAuthError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	authErr, ok := err.(*AuthError)
	if !ok {
		authErr = &AuthError{Code: "AUTH_ERROR", Message: err.Error()}
	}

	// Map errors to HTTP status codes
	status := http.StatusUnauthorized
	switch authErr {
	case ErrUserDisabled:
		status = http.StatusForbidden
	case ErrUserExists, ErrWeakPassword:
		status = http.StatusBadRequest
	}

	w.WriteHeader(status)
	w.Write([]byte(`{"error":"` + authErr.Code + `","message":"` + authErr.Message + `"}`))
}
