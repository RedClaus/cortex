// Package identity tests
package identity

import (
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("NewService() returned nil")
	}
	if svc.users == nil {
		t.Fatal("users map is nil")
	}
	if svc.byChannel == nil {
		t.Fatal("byChannel map is nil")
	}
	if svc.linkCodes == nil {
		t.Fatal("linkCodes map is nil")
	}
}

func TestGetOrCreate_NewUser(t *testing.T) {
	svc := NewService()

	user := svc.GetOrCreate("telegram", "12345", "testuser")
	if user == nil {
		t.Fatal("GetOrCreate() returned nil")
	}

	if user.ID == "" {
		t.Error("user.ID is empty")
	}
	if user.PrimaryName != "testuser" {
		t.Errorf("expected PrimaryName='testuser', got %q", user.PrimaryName)
	}
	if len(user.LinkedAccounts) != 1 {
		t.Fatalf("expected 1 linked account, got %d", len(user.LinkedAccounts))
	}

	account := user.LinkedAccounts[0]
	if account.Channel != "telegram" {
		t.Errorf("expected Channel='telegram', got %q", account.Channel)
	}
	if account.ExternalID != "12345" {
		t.Errorf("expected ExternalID='12345', got %q", account.ExternalID)
	}
	if account.Username != "testuser" {
		t.Errorf("expected Username='testuser', got %q", account.Username)
	}
	if !account.Verified {
		t.Error("expected account to be verified")
	}
	if !account.Primary {
		t.Error("expected account to be primary")
	}

	// Default preferences
	if user.Persona != "professional" {
		t.Errorf("expected default persona='professional', got %q", user.Persona)
	}
	if user.Permissions != "some" {
		t.Errorf("expected default permissions='some', got %q", user.Permissions)
	}
	if user.Preferences.Verbosity != "normal" {
		t.Errorf("expected default verbosity='normal', got %q", user.Preferences.Verbosity)
	}
}

func TestGetOrCreate_ExistingUser(t *testing.T) {
	svc := NewService()

	// Create first user
	user1 := svc.GetOrCreate("telegram", "12345", "testuser")
	originalID := user1.ID

	// Get same user again
	user2 := svc.GetOrCreate("telegram", "12345", "testuser")

	if user2.ID != originalID {
		t.Errorf("expected same user ID %q, got %q", originalID, user2.ID)
	}

	// LastSeenAt should be updated
	if user2.LastSeenAt.Before(user1.LastSeenAt) {
		t.Error("LastSeenAt was not updated")
	}
}

func TestGetOrCreate_DifferentChannels(t *testing.T) {
	svc := NewService()

	// Create users on different channels
	telegramUser := svc.GetOrCreate("telegram", "12345", "telegramuser")
	discordUser := svc.GetOrCreate("discord", "67890", "discorduser")

	if telegramUser.ID == discordUser.ID {
		t.Error("different channel users should have different IDs")
	}
}

func TestGetUser(t *testing.T) {
	svc := NewService()

	// Create a user
	created := svc.GetOrCreate("telegram", "12345", "testuser")

	// Get by ID
	found, ok := svc.GetUser(created.ID)
	if !ok {
		t.Fatal("GetUser() did not find the user")
	}
	if found.ID != created.ID {
		t.Errorf("expected ID=%q, got %q", created.ID, found.ID)
	}

	// Try to get non-existent user
	_, ok = svc.GetUser("nonexistent-id")
	if ok {
		t.Error("GetUser() should return false for non-existent user")
	}
}

func TestGenerateLinkCode(t *testing.T) {
	svc := NewService()

	user := svc.GetOrCreate("telegram", "12345", "testuser")
	code := svc.GenerateLinkCode(user.ID)

	if code == "" {
		t.Fatal("GenerateLinkCode() returned empty code")
	}

	// Code format: XXXX-XXX (8 chars total with hyphen in middle)
	if len(code) != 8 {
		t.Errorf("expected code length 8, got %d", len(code))
	}
	if code[4] != '-' {
		t.Errorf("expected hyphen at position 4, got %q", string(code[4]))
	}

	// Verify code is stored
	svc.mu.RLock()
	linkCode, ok := svc.linkCodes[code]
	svc.mu.RUnlock()

	if !ok {
		t.Fatal("link code not stored in service")
	}
	if linkCode.UserID != user.ID {
		t.Errorf("expected UserID=%q, got %q", user.ID, linkCode.UserID)
	}
	if linkCode.ExpiresAt.Before(time.Now()) {
		t.Error("link code should not be expired")
	}
}

func TestLinkAccount_Success(t *testing.T) {
	svc := NewService()

	// Create initial user via Telegram
	telegramUser := svc.GetOrCreate("telegram", "12345", "testuser")
	code := svc.GenerateLinkCode(telegramUser.ID)

	// Link Discord account using the code
	linkedUser, err := svc.LinkAccount(code, "discord", "67890", "discorduser")
	if err != nil {
		t.Fatalf("LinkAccount() failed: %v", err)
	}

	if linkedUser.ID != telegramUser.ID {
		t.Error("linked user should be the same as original user")
	}

	if len(linkedUser.LinkedAccounts) != 2 {
		t.Fatalf("expected 2 linked accounts, got %d", len(linkedUser.LinkedAccounts))
	}

	// Find Discord account
	var discordAccount *LinkedAccount
	for i := range linkedUser.LinkedAccounts {
		if linkedUser.LinkedAccounts[i].Channel == "discord" {
			discordAccount = &linkedUser.LinkedAccounts[i]
			break
		}
	}

	if discordAccount == nil {
		t.Fatal("Discord account not found in linked accounts")
	}
	if discordAccount.ExternalID != "67890" {
		t.Errorf("expected ExternalID='67890', got %q", discordAccount.ExternalID)
	}
	if discordAccount.Primary {
		t.Error("linked account should not be primary")
	}

	// Code should be consumed
	svc.mu.RLock()
	_, ok := svc.linkCodes[code]
	svc.mu.RUnlock()
	if ok {
		t.Error("link code should be deleted after use")
	}

	// Discord lookup should work now
	discordLookup := svc.GetOrCreate("discord", "67890", "discorduser")
	if discordLookup.ID != telegramUser.ID {
		t.Error("Discord lookup should return the linked user")
	}
}

func TestLinkAccount_InvalidCode(t *testing.T) {
	svc := NewService()

	_, err := svc.LinkAccount("INVALID-CODE", "discord", "67890", "discorduser")
	if err != ErrInvalidCode {
		t.Errorf("expected ErrInvalidCode, got %v", err)
	}
}

func TestLinkAccount_ExpiredCode(t *testing.T) {
	svc := NewService()

	user := svc.GetOrCreate("telegram", "12345", "testuser")
	code := svc.GenerateLinkCode(user.ID)

	// Manually expire the code
	svc.mu.Lock()
	svc.linkCodes[code].ExpiresAt = time.Now().Add(-time.Hour)
	svc.mu.Unlock()

	_, err := svc.LinkAccount(code, "discord", "67890", "discorduser")
	if err != ErrInvalidCode {
		t.Errorf("expected ErrInvalidCode for expired code, got %v", err)
	}
}

func TestLinkAccount_UserNotFound(t *testing.T) {
	svc := NewService()

	// Create a user and get a code
	user := svc.GetOrCreate("telegram", "12345", "testuser")
	code := svc.GenerateLinkCode(user.ID)

	// Delete the user (simulating edge case)
	svc.mu.Lock()
	delete(svc.users, user.ID)
	svc.mu.Unlock()

	_, err := svc.LinkAccount(code, "discord", "67890", "discorduser")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestIdentityError(t *testing.T) {
	if ErrInvalidCode.Error() != "invalid or expired link code" {
		t.Errorf("unexpected error message: %s", ErrInvalidCode.Error())
	}
	if ErrUserNotFound.Error() != "user not found" {
		t.Errorf("unexpected error message: %s", ErrUserNotFound.Error())
	}
}

func TestUserPreferences(t *testing.T) {
	svc := NewService()

	user := svc.GetOrCreate("telegram", "12345", "testuser")

	// Modify preferences
	user.Preferences.Verbosity = "verbose"
	user.Preferences.Timezone = "America/New_York"
	user.Preferences.Language = "es"

	// Retrieve and verify
	found, ok := svc.GetUser(user.ID)
	if !ok {
		t.Fatal("GetUser() failed")
	}

	if found.Preferences.Verbosity != "verbose" {
		t.Errorf("expected Verbosity='verbose', got %q", found.Preferences.Verbosity)
	}
	if found.Preferences.Timezone != "America/New_York" {
		t.Errorf("expected Timezone='America/New_York', got %q", found.Preferences.Timezone)
	}
	if found.Preferences.Language != "es" {
		t.Errorf("expected Language='es', got %q", found.Preferences.Language)
	}
}

func TestConcurrentAccess(t *testing.T) {
	svc := NewService()

	// Create a user first
	user := svc.GetOrCreate("telegram", "12345", "testuser")

	// Concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			svc.GetOrCreate("telegram", "12345", "testuser")
			done <- true
		}()
		go func() {
			svc.GetUser(user.ID)
			done <- true
		}()
		go func() {
			svc.GenerateLinkCode(user.ID)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 30; i++ {
		<-done
	}
}
