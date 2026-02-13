package onboarding

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/cortexhub/cortex-gateway/internal/config"
)

// Known swarm nodes
var knownNodes = []struct {
	Name string
	IP   string
	Services map[string]int
}{
	{"harold", "192.168.1.128", map[string]int{"bridge": 18802, "gateway": 18789}},
	{"pink", "192.168.1.186", map[string]int{"cortexbrain": 18892, "ollama": 11434, "redis": 6379}},
	{"red", "192.168.1.188", map[string]int{}},
	{"kentaro", "192.168.1.149", map[string]int{}},
}

// SwarmDiscover auto-discovers the existing swarm and generates config
func (o *Onboarding) SwarmDiscover() error {
	fmt.Println("🔍 Scanning for existing swarm nodes...")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	agents := []config.AgentConfig{}
	var cortexbrainURL, bridgeURL, ollamaURL string

	for _, node := range knownNodes {
		fmt.Printf("  Probing %s (%s)...", node.Name, node.IP)

		// Check SSH first
		reachable := false
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:22", node.IP), 3*time.Second)
		if err == nil {
			conn.Close()
			reachable = true
		}

		if !reachable {
			fmt.Println(" ❌ unreachable")
			continue
		}
		fmt.Print(" ✅ up")

		// Probe known services
		foundServices := map[string]int{}
		for svc, port := range node.Services {
			if probeService(ctx, node.IP, port) {
				foundServices[svc] = port
				fmt.Printf(" [%s:%d ✓]", svc, port)

				// Track key URLs
				switch svc {
				case "cortexbrain":
					cortexbrainURL = fmt.Sprintf("http://%s:%d", node.IP, port)
				case "bridge":
					bridgeURL = fmt.Sprintf("http://%s:%d", node.IP, port)
				case "ollama":
					ollamaURL = fmt.Sprintf("http://%s:%d", node.IP, port)
				}
			}
		}
		fmt.Println()

		agents = append(agents, config.AgentConfig{
			Name:     node.Name,
			Host:     node.IP,
			Services: foundServices,
		})
	}

	fmt.Println()

	if len(agents) == 0 {
		return fmt.Errorf("no swarm nodes found — check network connectivity")
	}

	fmt.Printf("✅ Found %d nodes\n", len(agents))

	// Build config
	if cortexbrainURL == "" {
		cortexbrainURL = "http://localhost:18892"
	}
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	if bridgeURL == "" {
		bridgeURL = "http://192.168.1.128:18802"
	}

	cfg := &config.Config{
		Server: config.ServerConfig{Host: "0.0.0.0", Port: 18800},
		CortexBrain: config.CortexBrainConfig{
			URL:       cortexbrainURL,
			JWTSecret: "cortex-gateway-jwt-secret-change-in-production",
			Timeout:   "30s",
		},
		Bridge: config.BridgeConfig{URL: bridgeURL},
		Ollama: config.OllamaConfig{
			URL:          ollamaURL,
			DefaultModel: "cortex-coder:latest",
			Timeout:      "60s",
		},
		Channels: config.ChannelsConfig{
			WebChat: config.WebChatConfig{Enabled: true, Port: 18793},
		},
		Inference: config.InferenceConfig{
			AutoDetect:  false,
			DefaultLane: "local",
			Lanes: []config.LaneConfig{
				{
					Name:     "local",
					Provider: "ollama",
					BaseURL:  ollamaURL,
					Models:   []string{"cortex-coder:latest", "go-coder:latest", "deepseek-coder-v2:latest"},
				},
			},
		},
		Swarm: config.SwarmConfig{
			Discovery: config.DiscoveryConfig{
				Method:       "mdns+arp",
				ScanInterval: 5 * time.Minute,
				Subnet:       "192.168.1.0/24",
			},
			Agents: agents,
		},
		HealthRing: config.HealthRingConfig{
			Enabled:       true,
			CheckInterval: 30 * time.Second,
		},
		Logging: config.LoggingConfig{Level: "info", Format: "json"},
	}

	// Build health ring members from agents
	for _, a := range agents {
		sshPort := 22
		member := config.HealthMemberConfig{
			Name: a.Name,
			Checks: []config.HealthCheckConfig{
				{Type: "tcp", Port: &sshPort},
			},
		}
		for svc, port := range a.Services {
			p := port
			switch svc {
			case "bridge", "cortexbrain", "ollama":
				url := fmt.Sprintf("http://%s:%d/health", a.Host, p)
				if svc == "ollama" {
					url = fmt.Sprintf("http://%s:%d/api/tags", a.Host, p)
				}
				expectStatus := 200
				member.Checks = append(member.Checks, config.HealthCheckConfig{
					Type:         "http",
					URL:          url,
					ExpectStatus: &expectStatus,
				})
			case "redis":
				member.Checks = append(member.Checks, config.HealthCheckConfig{
					Type: "tcp",
					Port: &p,
				})
			}
		}
		cfg.HealthRing.Members = append(cfg.HealthRing.Members, member)
	}

	// Write config
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := writeFile(o.configPath, data); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("\n✅ Config written to %s\n", o.configPath)
	fmt.Printf("   CortexBrain: %s\n", cortexbrainURL)
	fmt.Printf("   Ollama:      %s\n", ollamaURL)
	fmt.Printf("   Bridge:      %s\n", bridgeURL)
	fmt.Printf("   Agents:      %d nodes\n\n", len(agents))

	return nil
}

func probeService(ctx context.Context, ip string, port int) bool {
	// Try HTTP health first
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://%s:%d/health", ip, port)
	resp, err := client.Get(url)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return true
		}
	}
	// Try Ollama tags endpoint
	url = fmt.Sprintf("http://%s:%d/api/tags", ip, port)
	resp, err = client.Get(url)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return true
		}
	}
	// Fallback to TCP
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 2*time.Second)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
