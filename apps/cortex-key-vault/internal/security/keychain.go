package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/pbkdf2"
)

const (
	// ServiceName is the keychain service identifier
	ServiceName = "cortex-key-vault"

	// MasterKeyAccount stores the hashed master password
	MasterKeyAccount = "_master_key"

	// SaltAccount stores the password salt
	SaltAccount = "_salt"
)

// Keychain provides secure storage using macOS Keychain
type Keychain struct {
	service string
}

// NewKeychain creates a new Keychain adapter
func NewKeychain() *Keychain {
	return &Keychain{service: ServiceName}
}

// IsInitialized checks if a master password has been set
func (k *Keychain) IsInitialized() bool {
	_, err := keyring.Get(k.service, MasterKeyAccount)
	return err == nil
}

// Initialize sets up the vault with a master password
func (k *Keychain) Initialize(masterPassword string) error {
	if k.IsInitialized() {
		return fmt.Errorf("vault already initialized")
	}

	// Generate a random salt
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}

	// Hash the master password with PBKDF2
	hashedPassword := k.hashPassword(masterPassword, salt)

	// Store the salt
	if err := keyring.Set(k.service, SaltAccount, base64.StdEncoding.EncodeToString(salt)); err != nil {
		return fmt.Errorf("store salt: %w", err)
	}

	// Store the hashed password
	if err := keyring.Set(k.service, MasterKeyAccount, hashedPassword); err != nil {
		return fmt.Errorf("store master key: %w", err)
	}

	return nil
}

// Verify checks if the provided password matches the master password
func (k *Keychain) Verify(password string) bool {
	// Get the salt
	saltB64, err := keyring.Get(k.service, SaltAccount)
	if err != nil {
		return false
	}

	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return false
	}

	// Get the stored hash
	storedHash, err := keyring.Get(k.service, MasterKeyAccount)
	if err != nil {
		return false
	}

	// Hash the provided password and compare
	hashedPassword := k.hashPassword(password, salt)
	return hashedPassword == storedHash
}

// ChangeMasterPassword changes the master password
func (k *Keychain) ChangeMasterPassword(oldPassword, newPassword string) error {
	if !k.Verify(oldPassword) {
		return fmt.Errorf("incorrect current password")
	}

	// Get the salt
	saltB64, err := keyring.Get(k.service, SaltAccount)
	if err != nil {
		return fmt.Errorf("get salt: %w", err)
	}

	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return fmt.Errorf("decode salt: %w", err)
	}

	// Hash new password
	newHash := k.hashPassword(newPassword, salt)

	// Store the new hash
	if err := keyring.Set(k.service, MasterKeyAccount, newHash); err != nil {
		return fmt.Errorf("store new master key: %w", err)
	}

	return nil
}

// StoreSecret stores a secret value in the keychain
func (k *Keychain) StoreSecret(id, value string) error {
	if err := keyring.Set(k.service, id, value); err != nil {
		return fmt.Errorf("store secret: %w", err)
	}
	return nil
}

// GetSecret retrieves a secret value from the keychain
func (k *Keychain) GetSecret(id string) (string, error) {
	value, err := keyring.Get(k.service, id)
	if err != nil {
		return "", fmt.Errorf("get secret: %w", err)
	}
	return value, nil
}

// DeleteSecret removes a secret from the keychain
func (k *Keychain) DeleteSecret(id string) error {
	if err := keyring.Delete(k.service, id); err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	return nil
}

// hashPassword uses PBKDF2 to derive a key from the password
func (k *Keychain) hashPassword(password string, salt []byte) string {
	// Use PBKDF2 with SHA256, 100000 iterations, 32 byte key
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
	return base64.StdEncoding.EncodeToString(key)
}

// Reset removes all vault data (dangerous!)
func (k *Keychain) Reset() error {
	// Note: go-keyring doesn't support listing all keys
	// We can only delete known keys
	keyring.Delete(k.service, MasterKeyAccount)
	keyring.Delete(k.service, SaltAccount)
	return nil
}
