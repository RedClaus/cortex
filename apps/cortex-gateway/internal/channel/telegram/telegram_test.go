package telegram

import (
"testing"
)

func TestAdapterName(t *testing.T) {
adapter := NewTelegramAdapter("test")
if adapter.Name() != "telegram" {
t.Errorf("Expected telegram, got %s", adapter.Name())
}
}
