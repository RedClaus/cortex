// Package auth provides user authentication and session management for CortexBrain.
// It uses JWT for access tokens and secure refresh tokens stored in the database.
package auth

import (
	"time"
)

// User represents a registered user in the system.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email,omitempty"`
	DisplayName  string    `json:"displayName,omitempty"`
	PasswordHash string    `json:"-"` // Never expose in JSON
	IsActive     bool      `json:"isActive"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Session represents an active user session with refresh token.
type Session struct {
	ID               string    `json:"id"`
	UserID           string    `json:"userId"`
	RefreshTokenHash string    `json:"-"` // Never expose
	ExpiresAt        time.Time `json:"expiresAt"`
	UserAgent        string    `json:"userAgent,omitempty"`
	IPAddress        string    `json:"ipAddress,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	LastUsedAt       time.Time `json:"lastUsedAt"`
}

// UserPersona represents the assignment of a persona to a user.
type UserPersona struct {
	UserID     string    `json:"userId"`
	PersonaID  string    `json:"personaId"`
	IsDefault  bool      `json:"isDefault"`
	AssignedAt time.Time `json:"assignedAt"`
}

// Claims represents the JWT token claims.
type Claims struct {
	UserID   string `json:"sub"`       // Subject (user ID)
	Username string `json:"username"`
	Type     string `json:"type"`      // "access" or "refresh"
	IssuedAt int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// TokenPair contains both access and refresh tokens.
type TokenPair struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`    // When access token expires
	TokenType    string    `json:"tokenType"`    // "Bearer"
}

// ───────────────────────────────────────────────────────────────────────────────
// REQUEST/RESPONSE TYPES
// ───────────────────────────────────────────────────────────────────────────────

// RegisterRequest is the payload for user registration.
type RegisterRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

// LoginRequest is the payload for user login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RefreshRequest is the payload for token refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// AuthResponse is the response for successful authentication.
type AuthResponse struct {
	User   *User      `json:"user"`
	Tokens *TokenPair `json:"tokens"`
}

// UserResponse is a simplified user response without sensitive data.
type UserResponse struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email,omitempty"`
	DisplayName string    `json:"displayName,omitempty"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ToResponse converts a User to a safe UserResponse.
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt,
	}
}

// ───────────────────────────────────────────────────────────────────────────────
// ERROR TYPES
// ───────────────────────────────────────────────────────────────────────────────

// AuthError represents an authentication-related error.
type AuthError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *AuthError) Error() string {
	return e.Message
}

// Common auth errors
var (
	ErrInvalidCredentials = &AuthError{Code: "INVALID_CREDENTIALS", Message: "invalid username or password"}
	ErrUserNotFound       = &AuthError{Code: "USER_NOT_FOUND", Message: "user not found"}
	ErrUserExists         = &AuthError{Code: "USER_EXISTS", Message: "username already exists"}
	ErrUserDisabled       = &AuthError{Code: "USER_DISABLED", Message: "user account is disabled"}
	ErrInvalidToken       = &AuthError{Code: "INVALID_TOKEN", Message: "invalid or expired token"}
	ErrTokenExpired       = &AuthError{Code: "TOKEN_EXPIRED", Message: "token has expired"}
	ErrSessionExpired     = &AuthError{Code: "SESSION_EXPIRED", Message: "session has expired"}
	ErrMissingToken       = &AuthError{Code: "MISSING_TOKEN", Message: "authorization token required"}
	ErrWeakPassword       = &AuthError{Code: "WEAK_PASSWORD", Message: "password must be at least 8 characters"}
)

// ───────────────────────────────────────────────────────────────────────────────
// CONFIG
// ───────────────────────────────────────────────────────────────────────────────

// Config holds authentication configuration.
type Config struct {
	// JWTSecret is the secret key for signing JWTs.
	// In production, this should be loaded from environment/secrets manager.
	JWTSecret string

	// AccessTokenDuration is how long access tokens are valid.
	AccessTokenDuration time.Duration

	// RefreshTokenDuration is how long refresh tokens are valid.
	RefreshTokenDuration time.Duration

	// BcryptCost is the cost factor for bcrypt password hashing.
	BcryptCost int
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() *Config {
	return &Config{
		JWTSecret:            "", // Must be set!
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour,
		BcryptCost:           12,
	}
}
