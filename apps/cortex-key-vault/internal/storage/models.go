package storage

import (
	"time"
)

// SecretType represents the type of secret stored
type SecretType string

const (
	TypeAPIKey      SecretType = "api_key"
	TypeSSHKey      SecretType = "ssh_key"
	TypePassword    SecretType = "password"
	TypeCertificate SecretType = "certificate"
)

// Secret represents a stored secret with metadata
type Secret struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       SecretType `json:"type"`
	Username   string     `json:"username,omitempty"`   // For password type
	URL        string     `json:"url,omitempty"`        // Associated service URL
	Notes      string     `json:"notes,omitempty"`      // Additional notes
	CategoryID string     `json:"category_id"`          // Category reference
	Tags       []string   `json:"tags,omitempty"`       // Tags for organization
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Category represents a grouping for secrets
type Category struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`  // Emoji or icon name
	Color string `json:"color"` // Hex color code
}

// Tag represents a tag for organizing secrets
type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// DefaultCategories returns the default categories for a new vault
func DefaultCategories() []Category {
	return []Category{
		{ID: "all", Name: "All", Icon: "📁", Color: "#808080"},
		{ID: "api_keys", Name: "API Keys", Icon: "🔑", Color: "#FFD700"},
		{ID: "ssh_keys", Name: "SSH Keys", Icon: "🔐", Color: "#00CED1"},
		{ID: "passwords", Name: "Passwords", Icon: "🔒", Color: "#FF6B6B"},
		{ID: "certificates", Name: "Certificates", Icon: "📜", Color: "#98D8C8"},
	}
}

// SecretTypeInfo provides display info for secret types
type SecretTypeInfo struct {
	Type        SecretType
	Name        string
	Icon        string
	Description string
}

// GetSecretTypeInfo returns display info for all secret types
func GetSecretTypeInfo() []SecretTypeInfo {
	return []SecretTypeInfo{
		{Type: TypeAPIKey, Name: "API Key", Icon: "🔑", Description: "API keys and tokens"},
		{Type: TypeSSHKey, Name: "SSH Key", Icon: "🔐", Description: "SSH keys and keypairs"},
		{Type: TypePassword, Name: "Password", Icon: "🔒", Description: "Login credentials"},
		{Type: TypeCertificate, Name: "Certificate", Icon: "📜", Description: "SSL/TLS certificates"},
	}
}

// GetIconForType returns the icon for a secret type
func GetIconForType(t SecretType) string {
	switch t {
	case TypeAPIKey:
		return "🔑"
	case TypeSSHKey:
		return "🔐"
	case TypePassword:
		return "🔒"
	case TypeCertificate:
		return "📜"
	default:
		return "❓"
	}
}

// ImportSecret represents a secret to be imported from JSON
type ImportSecret struct {
	Name     string     `json:"name"`
	Type     SecretType `json:"type"`
	Value    string     `json:"value"`
	Username string     `json:"username,omitempty"`
	URL      string     `json:"url,omitempty"`
	Notes    string     `json:"notes,omitempty"`
	Category string     `json:"category,omitempty"` // Category ID (defaults to "all")
	Tags     []string   `json:"tags,omitempty"`
}

// ImportFile represents the JSON import file structure
type ImportFile struct {
	Secrets []ImportSecret `json:"secrets"`
}

// ImportResult contains the results of an import operation
type ImportResult struct {
	Total    int
	Imported int
	Skipped  int
	Errors   []ImportError
}

// ImportError describes a failed import
type ImportError struct {
	Name   string
	Reason string
}

// ValidSecretTypes returns valid secret type strings for validation
func ValidSecretTypes() map[SecretType]bool {
	return map[SecretType]bool{
		TypeAPIKey:      true,
		TypeSSHKey:      true,
		TypePassword:    true,
		TypeCertificate: true,
	}
}
