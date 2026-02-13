package service

import (
	"fmt"

	"github.com/normanking/cortex-key-vault/internal/security"
	"github.com/normanking/cortex-key-vault/internal/storage"
)

// VaultService provides unified access to the vault
type VaultService struct {
	store    *storage.Store
	keychain *security.Keychain
	session  *security.Session
}

// NewVaultService creates a new vault service
func NewVaultService() (*VaultService, error) {
	store, err := storage.NewStore()
	if err != nil {
		return nil, fmt.Errorf("create store: %w", err)
	}

	return &VaultService{
		store:    store,
		keychain: security.NewKeychain(),
		session:  security.NewSession(),
	}, nil
}

// IsInitialized checks if the vault has been set up
func (v *VaultService) IsInitialized() bool {
	return v.keychain.IsInitialized()
}

// Initialize sets up the vault with a master password
func (v *VaultService) Initialize(masterPassword string) error {
	return v.keychain.Initialize(masterPassword)
}

// Unlock verifies the password and unlocks the vault
func (v *VaultService) Unlock(password string) error {
	if !v.keychain.Verify(password) {
		return fmt.Errorf("incorrect password")
	}

	v.session.Unlock()
	return nil
}

// Lock locks the vault
func (v *VaultService) Lock() {
	v.session.Lock()
}

// IsUnlocked returns true if the vault is unlocked
func (v *VaultService) IsUnlocked() bool {
	return v.session.IsUnlocked()
}

// Touch updates the session activity (call on user interaction)
func (v *VaultService) Touch() {
	v.session.Touch()
}

// CreateSecret creates a new secret with its value
func (v *VaultService) CreateSecret(secret *storage.Secret, value string) error {
	if !v.IsUnlocked() {
		return fmt.Errorf("vault is locked")
	}

	// Store metadata in SQLite
	if err := v.store.CreateSecret(secret); err != nil {
		return fmt.Errorf("store metadata: %w", err)
	}

	// Store value in Keychain
	if err := v.keychain.StoreSecret(secret.ID, value); err != nil {
		// Rollback metadata
		v.store.DeleteSecret(secret.ID)
		return fmt.Errorf("store value: %w", err)
	}

	return nil
}

// GetSecret retrieves a secret with its value
func (v *VaultService) GetSecret(id string) (*storage.Secret, string, error) {
	if !v.IsUnlocked() {
		return nil, "", fmt.Errorf("vault is locked")
	}

	// Get metadata
	secret, err := v.store.GetSecret(id)
	if err != nil {
		return nil, "", fmt.Errorf("get metadata: %w", err)
	}

	// Get value from Keychain
	value, err := v.keychain.GetSecret(id)
	if err != nil {
		return nil, "", fmt.Errorf("get value: %w", err)
	}

	return secret, value, nil
}

// GetSecretValue retrieves only the secret value
func (v *VaultService) GetSecretValue(id string) (string, error) {
	if !v.IsUnlocked() {
		return "", fmt.Errorf("vault is locked")
	}

	return v.keychain.GetSecret(id)
}

// UpdateSecret updates a secret's metadata and optionally its value
func (v *VaultService) UpdateSecret(secret *storage.Secret, value *string) error {
	if !v.IsUnlocked() {
		return fmt.Errorf("vault is locked")
	}

	// Update metadata
	if err := v.store.UpdateSecret(secret); err != nil {
		return fmt.Errorf("update metadata: %w", err)
	}

	// Update value if provided
	if value != nil {
		if err := v.keychain.StoreSecret(secret.ID, *value); err != nil {
			return fmt.Errorf("update value: %w", err)
		}
	}

	return nil
}

// DeleteSecret removes a secret completely
func (v *VaultService) DeleteSecret(id string) error {
	if !v.IsUnlocked() {
		return fmt.Errorf("vault is locked")
	}

	// Delete from Keychain first
	if err := v.keychain.DeleteSecret(id); err != nil {
		// Keychain might not have it, continue
	}

	// Delete metadata
	if err := v.store.DeleteSecret(id); err != nil {
		return fmt.Errorf("delete metadata: %w", err)
	}

	return nil
}

// ListSecrets returns all secrets (metadata only)
func (v *VaultService) ListSecrets(categoryID string) ([]storage.Secret, error) {
	if !v.IsUnlocked() {
		return nil, fmt.Errorf("vault is locked")
	}

	return v.store.ListSecrets(categoryID)
}

// ListSecretsByType returns secrets filtered by type
func (v *VaultService) ListSecretsByType(secretType storage.SecretType) ([]storage.Secret, error) {
	if !v.IsUnlocked() {
		return nil, fmt.Errorf("vault is locked")
	}

	return v.store.ListSecretsByType(secretType)
}

// ListSecretsByTag returns secrets with a specific tag
func (v *VaultService) ListSecretsByTag(tag string) ([]storage.Secret, error) {
	if !v.IsUnlocked() {
		return nil, fmt.Errorf("vault is locked")
	}

	return v.store.ListSecretsByTag(tag)
}

// SearchSecrets searches secrets by name/notes/username
func (v *VaultService) SearchSecrets(query string) ([]storage.Secret, error) {
	if !v.IsUnlocked() {
		return nil, fmt.Errorf("vault is locked")
	}

	return v.store.SearchSecrets(query)
}

// GetCategories returns all categories
func (v *VaultService) GetCategories() ([]storage.Category, error) {
	return v.store.GetCategories()
}

// GetCategoryCount returns the count of secrets in a category
func (v *VaultService) GetCategoryCount(categoryID string) (int, error) {
	return v.store.GetCategoryCount(categoryID)
}

// GetTags returns all tags
func (v *VaultService) GetTags() ([]storage.Tag, error) {
	return v.store.GetTags()
}

// GetTagCount returns the count of secrets with a tag
func (v *VaultService) GetTagCount(tag string) (int, error) {
	return v.store.GetTagCount(tag)
}

// Close cleans up resources
func (v *VaultService) Close() error {
	v.session.Close()
	return v.store.Close()
}

// ChangeMasterPassword changes the master password
func (v *VaultService) ChangeMasterPassword(oldPassword, newPassword string) error {
	if !v.IsUnlocked() {
		return fmt.Errorf("vault is locked")
	}

	return v.keychain.ChangeMasterPassword(oldPassword, newPassword)
}

// ImportSecrets imports secrets from an ImportFile structure
func (v *VaultService) ImportSecrets(importData *storage.ImportFile) (*storage.ImportResult, error) {
	if !v.IsUnlocked() {
		return nil, fmt.Errorf("vault is locked")
	}

	result := &storage.ImportResult{
		Total:  len(importData.Secrets),
		Errors: []storage.ImportError{},
	}

	validTypes := storage.ValidSecretTypes()

	for _, imp := range importData.Secrets {
		// Validate required fields
		if imp.Name == "" {
			result.Errors = append(result.Errors, storage.ImportError{
				Name:   "(unnamed)",
				Reason: "name is required",
			})
			result.Skipped++
			continue
		}

		if imp.Value == "" {
			result.Errors = append(result.Errors, storage.ImportError{
				Name:   imp.Name,
				Reason: "value is required",
			})
			result.Skipped++
			continue
		}

		// Validate type
		if !validTypes[imp.Type] {
			result.Errors = append(result.Errors, storage.ImportError{
				Name:   imp.Name,
				Reason: fmt.Sprintf("invalid type '%s' (use: api_key, ssh_key, password, certificate)", imp.Type),
			})
			result.Skipped++
			continue
		}

		// Set defaults
		categoryID := imp.Category
		if categoryID == "" {
			categoryID = "all"
		}

		tags := imp.Tags
		if tags == nil {
			tags = []string{}
		}

		// Create the secret
		secret := &storage.Secret{
			Name:       imp.Name,
			Type:       imp.Type,
			Username:   imp.Username,
			URL:        imp.URL,
			Notes:      imp.Notes,
			CategoryID: categoryID,
			Tags:       tags,
		}

		if err := v.CreateSecret(secret, imp.Value); err != nil {
			result.Errors = append(result.Errors, storage.ImportError{
				Name:   imp.Name,
				Reason: err.Error(),
			})
			result.Skipped++
			continue
		}

		result.Imported++
	}

	return result, nil
}
