package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Service provides authentication operations.
type Service struct {
	store  *Store
	config *Config
}

// NewService creates a new auth service.
func NewService(store *Store, config *Config) *Service {
	if config == nil {
		config = DefaultConfig()
	}
	return &Service{
		store:  store,
		config: config,
	}
}

// ───────────────────────────────────────────────────────────────────────────────
// REGISTRATION & LOGIN
// ───────────────────────────────────────────────────────────────────────────────

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*User, error) {
	// Validate password strength
	if len(req.Password) < 8 {
		return nil, ErrWeakPassword
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.config.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Email:        req.Email,
		DisplayName:  req.DisplayName,
		IsActive:     true,
	}

	if err := s.store.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// Login authenticates a user and returns tokens.
func (s *Service) Login(ctx context.Context, req *LoginRequest, userAgent, ipAddress string) (*AuthResponse, error) {
	// Get user
	user, err := s.store.GetUserByUsername(ctx, req.Username)
	if err != nil {
		if err == ErrUserNotFound {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Check if user is active
	if !user.IsActive {
		return nil, ErrUserDisabled
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Generate tokens
	tokens, session, err := s.generateTokens(user, userAgent, ipAddress)
	if err != nil {
		return nil, err
	}

	// Save session
	if err := s.store.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:   user,
		Tokens: tokens,
	}, nil
}

// RefreshToken exchanges a refresh token for new tokens.
func (s *Service) RefreshToken(ctx context.Context, refreshToken string, userAgent, ipAddress string) (*TokenPair, error) {
	// Hash the refresh token to look it up
	tokenHash := hashToken(refreshToken)

	// Find session
	session, err := s.store.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Check expiration
	if time.Now().After(session.ExpiresAt) {
		s.store.DeleteSession(ctx, session.ID)
		return nil, ErrSessionExpired
	}

	// Get user
	user, err := s.store.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrUserDisabled
	}

	// Delete old session
	s.store.DeleteSession(ctx, session.ID)

	// Generate new tokens
	tokens, newSession, err := s.generateTokens(user, userAgent, ipAddress)
	if err != nil {
		return nil, err
	}

	// Save new session
	if err := s.store.CreateSession(ctx, newSession); err != nil {
		return nil, err
	}

	return tokens, nil
}

// Logout invalidates a user's session.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := hashToken(refreshToken)

	session, err := s.store.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		// Session not found is fine - already logged out
		return nil
	}

	return s.store.DeleteSession(ctx, session.ID)
}

// LogoutAll invalidates all sessions for a user.
func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	return s.store.DeleteUserSessions(ctx, userID)
}

// ───────────────────────────────────────────────────────────────────────────────
// TOKEN VALIDATION
// ───────────────────────────────────────────────────────────────────────────────

// ValidateToken validates an access token and returns the claims.
func (s *Service) ValidateToken(token string) (*Claims, error) {
	claims, err := s.parseToken(token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Check expiration
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	// Check token type
	if claims.Type != "access" {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GetUserFromToken validates token and returns the user.
func (s *Service) GetUserFromToken(ctx context.Context, token string) (*User, error) {
	claims, err := s.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	user, err := s.store.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrUserDisabled
	}

	return user, nil
}

// ───────────────────────────────────────────────────────────────────────────────
// USER-PERSONA OPERATIONS
// ───────────────────────────────────────────────────────────────────────────────

// AssignPersona assigns a persona to a user.
func (s *Service) AssignPersona(ctx context.Context, userID, personaID string, isDefault bool) error {
	return s.store.AssignPersonaToUser(ctx, userID, personaID, isDefault)
}

// UnassignPersona removes a persona from a user.
func (s *Service) UnassignPersona(ctx context.Context, userID, personaID string) error {
	return s.store.UnassignPersonaFromUser(ctx, userID, personaID)
}

// SetDefaultPersona sets the default persona for a user.
func (s *Service) SetDefaultPersona(ctx context.Context, userID, personaID string) error {
	return s.store.SetDefaultPersona(ctx, userID, personaID)
}

// GetUserPersonas gets all personas assigned to a user.
func (s *Service) GetUserPersonas(ctx context.Context, userID string) ([]UserPersona, error) {
	return s.store.GetUserPersonas(ctx, userID)
}

// GetUserDefaultPersonaID gets the default persona ID for a user.
func (s *Service) GetUserDefaultPersonaID(ctx context.Context, userID string) (string, error) {
	return s.store.GetUserDefaultPersonaID(ctx, userID)
}

// ───────────────────────────────────────────────────────────────────────────────
// TOKEN GENERATION (Simple JWT-like implementation)
// ───────────────────────────────────────────────────────────────────────────────

func (s *Service) generateTokens(user *User, userAgent, ipAddress string) (*TokenPair, *Session, error) {
	now := time.Now()

	// Generate access token
	accessClaims := &Claims{
		UserID:    user.ID,
		Username:  user.Username,
		Type:      "access",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(s.config.AccessTokenDuration).Unix(),
	}
	accessToken, err := s.signToken(accessClaims)
	if err != nil {
		return nil, nil, fmt.Errorf("sign access token: %w", err)
	}

	// Generate refresh token (random bytes)
	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return nil, nil, fmt.Errorf("generate refresh token: %w", err)
	}
	refreshToken := base64.URLEncoding.EncodeToString(refreshBytes)

	// Create session
	session := &Session{
		UserID:           user.ID,
		RefreshTokenHash: hashToken(refreshToken),
		ExpiresAt:        now.Add(s.config.RefreshTokenDuration),
		UserAgent:        userAgent,
		IPAddress:        ipAddress,
	}

	tokens := &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    now.Add(s.config.AccessTokenDuration),
		TokenType:    "Bearer",
	}

	return tokens, session, nil
}

// signToken creates a simple signed token (base64 encoded JSON + HMAC signature).
// For production, use a proper JWT library.
func (s *Service) signToken(claims *Claims) (string, error) {
	// Encode claims as JSON
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	// Base64 encode
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Create signature
	signature := s.sign(payload)

	// Combine: payload.signature
	return payload + "." + signature, nil
}

// parseToken parses and validates a token.
func (s *Service) parseToken(token string) (*Claims, error) {
	// Split payload and signature
	parts := splitToken(token)
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}

	payload, signature := parts[0], parts[1]

	// Verify signature
	expectedSig := s.sign(payload)
	if signature != expectedSig {
		return nil, ErrInvalidToken
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	return &claims, nil
}

// sign creates an HMAC-SHA256 signature.
func (s *Service) sign(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	h.Write([]byte(s.config.JWTSecret))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// splitToken splits a token into parts by '.'.
func splitToken(token string) []string {
	var parts []string
	start := 0
	for i, c := range token {
		if c == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	if start < len(token) {
		parts = append(parts, token[start:])
	}
	return parts
}

// hashToken creates a SHA-256 hash of a token.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
