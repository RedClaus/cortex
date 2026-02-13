package discord

import "testing"

func TestName(t *testing.T) {
	adapter := NewDiscordAdapter("token")
	if adapter.Name() != "discord" {
		t.Errorf("expected name discord, got %s", adapter.Name())
	}
}
