package webchat

import "testing"

func TestName(t *testing.T) {
	adapter := NewWebChatAdapter(8080)
	if adapter.Name() != "webchat" {
		t.Errorf("expected name webchat, got %s", adapter.Name())
	}
}
