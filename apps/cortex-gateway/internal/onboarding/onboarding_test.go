package onboarding

import (
	"log/slog"
	"os"
	"testing"
)

func TestIsNeeded_NoConfig(t *testing.T) {
	o := New(slog.Default(), "/tmp/nonexistent-config-12345.yaml")
	if !o.IsNeeded() {
		t.Error("Expected IsNeeded=true when config does not exist")
	}
}

func TestIsNeeded_WithConfig(t *testing.T) {
	f, _ := os.CreateTemp("", "config-*.yaml")
	f.WriteString("server:\n  port: 18800\n")
	f.Close()
	defer os.Remove(f.Name())

	o := New(slog.Default(), f.Name())
	if o.IsNeeded() {
		t.Error("Expected IsNeeded=false when config exists")
	}
}
