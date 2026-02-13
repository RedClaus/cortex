package discovery

import (
	"log/slog"
	"testing"
	"time"

	"github.com/cortexhub/cortex-gateway/internal/config"
)

func testDiscovery() *Discovery {
	cfg := config.SwarmConfig{
		Agents: []config.AgentConfig{
			{Name: "harold", Host: "192.168.1.128", Services: map[string]int{"bridge": 18802}},
			{Name: "pink", Host: "192.168.1.186", Services: map[string]int{"cortexbrain": 18892}},
		},
	}
	cfg.Discovery = config.DiscoveryConfig{ScanInterval: 5 * time.Minute, Subnet: "192.168.1.0/24", Method: "arp"}
	return NewDiscovery(cfg, slog.Default())
}

func TestNewDiscovery(t *testing.T) {
	d := testDiscovery()
	if d == nil {
		t.Fatal("Expected non-nil Discovery")
	}
}

func TestResolveUnknownAgent(t *testing.T) {
	d := testDiscovery()
	_, err := d.Resolve("nonexistent")
	if err == nil {
		t.Error("Expected error for unknown agent")
	}
}

func TestResolveDownAgent(t *testing.T) {
	d := testDiscovery()
	// Agents start as "unknown" status, Resolve should error
	_, err := d.Resolve("harold")
	if err == nil {
		t.Error("Expected error for agent with unknown status")
	}
}

func TestServiceURL(t *testing.T) {
	d := testDiscovery()
	url, err := d.ServiceURL("harold", "bridge")
	if err != nil {
		t.Fatalf("ServiceURL failed: %v", err)
	}
	if url != "http://192.168.1.128:18802" {
		t.Errorf("Expected http://192.168.1.128:18802, got %s", url)
	}
}
