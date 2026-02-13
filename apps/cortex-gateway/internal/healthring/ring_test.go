package healthring

import (
	"log/slog"
	"testing"
	"time"

	"github.com/cortexhub/cortex-gateway/internal/config"
	"github.com/cortexhub/cortex-gateway/internal/discovery"
)

func TestNewHealthRing(t *testing.T) {
	swarmCfg := config.SwarmConfig{
		Agents: []config.AgentConfig{
			{Name: "pink", Host: "192.168.1.186", Services: map[string]int{"cortexbrain": 18892}},
		},
		Discovery: config.DiscoveryConfig{ScanInterval: 5 * time.Minute, Subnet: "192.168.1.0/24", Method: "arp"},
	}
	disc := discovery.NewDiscovery(swarmCfg, slog.Default())
	port := 22
	hrCfg := config.HealthRingConfig{
		Enabled:       true,
		CheckInterval: 30 * time.Second,
		Members: []config.HealthMemberConfig{
			{Name: "pink", Checks: []config.HealthCheckConfig{{Type: "tcp", Port: &port}}},
		},
	}
	hr := NewHealthRing(hrCfg, disc, slog.Default())
	if hr == nil {
		t.Fatal("Expected non-nil HealthRing")
	}
}
