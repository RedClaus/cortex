package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cortexhub/cortex-gateway/internal/config"
)

type Agent struct {
	Name     string            `json:"name"`
	IP       string            `json:"ip"`
	Services map[string]int    `json:"services"`
	Status   string            `json:"status"`
	LastSeen time.Time         `json:"last_seen"`
}

type DiscoveredNode struct {
	IP       string            `json:"ip"`
	Services map[string]bool   `json:"services"`
}

func generateIPs(network *net.IPNet) []net.IP {
	ones, bits := network.Mask.Size()
	if ones != 24 || bits != 32 {
		return nil
	}
	ips := make([]net.IP, 0, 254)
	base := network.IP.To4()
	for i := byte(1); i < 255; i++ {
		ip := make(net.IP, 4)
		copy(ip, base)
		ip[3] = i
		ips = append(ips, ip)
	}
	return ips
}

type Discovery struct {
	registry     map[string]*Agent
	logger       *slog.Logger
	scanInterval time.Duration
	method       string
	subnet       string
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewDiscovery(cfg config.SwarmConfig, logger *slog.Logger) *Discovery {
	d := &Discovery{
		registry:     make(map[string]*Agent),
		logger:       logger,
		scanInterval: cfg.Discovery.ScanInterval,
		method:       cfg.Discovery.Method,
		subnet:       cfg.Discovery.Subnet,
	}
	for _, ac := range cfg.Agents {
		agent := &Agent{
			Name:     ac.Name,
			IP:       ac.Host,
			Services: ac.Services,
			Status:   "unknown",
			LastSeen: time.Time{},
		}
		d.registry[ac.Name] = agent
	}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	d.startBackground()
	return d
}

func (d *Discovery) startBackground() {
	go d.backgroundScan()
}

func (d *Discovery) backgroundScan() {
	ticker := time.NewTicker(d.scanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.scan()
		}
	}
}

func (d *Discovery) scan() {
	for name, agent := range d.registry {
		var newIP string
		if strings.Contains(d.method, "mdns") {
			ips, err := net.LookupHost(name + ".local")
			if err == nil && len(ips) > 0 {
				newIP = net.ParseIP(ips[0]).String()
				if newIP != agent.IP {
					d.logger.Info("IP updated via mDNS", "agent", name, "new_ip", newIP)
					agent.IP = newIP
				}
			}
		}
		if newIP == "" {
			newIP = agent.IP
		}
		up := d.checkConnectivity(newIP, agent.Services)
		agent.Status = "up"
		if !up {
			agent.Status = "down"
		}
		if up {
			agent.LastSeen = time.Now()
		}
		d.logger.Debug("Agent scan", "name", name, "ip", newIP, "status", agent.Status)
	}
}

func (d *Discovery) checkConnectivity(ip string, services map[string]int) bool {
	if d.pingTCP(ip, 22) {
		return true
	}
	for _, port := range services {
		if d.pingTCP(ip, port) {
			return true
		}
	}
	return false
}

func (d *Discovery) pingTCP(ip string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 3*time.Second)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

func (d *Discovery) ScanSubnet(ctx context.Context, subnetCIDR string) (<-chan *DiscoveredNode, error) {
	_, network, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return nil, err
	}
	ch := make(chan *DiscoveredNode, 100)
	go func() {
		defer close(ch)
		ips := generateIPs(network)
		if len(ips) == 0 {
			return
		}
		var wg sync.WaitGroup
		sem := make(chan struct{}, 64)
		for _, ip := range ips {
			select {
			case <-ctx.Done():
				return
			default:
			}
			wg.Add(1)
			go func(ip net.IP) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				node := &DiscoveredNode{
					IP:       ip.String(),
					Services: make(map[string]bool),
				}
				ports := []int{22, 80, 443, 11434, 18800, 18802, 18892, 6379}
				for _, port := range ports {
					if d.pingTCP(ip.String(), port) {
						node.Services[fmt.Sprintf("tcp:%d", port)] = true
					}
				}
				for _, port := range []int{18800, 18802, 18892, 11434} {
					url := fmt.Sprintf("http://%s:%d/health", ip.String(), port)
					client := &http.Client{Timeout: 2 * time.Second}
					resp, err := client.Get(url)
					if err == nil && resp.StatusCode == 200 {
						node.Services[fmt.Sprintf("http/health:%d", port)] = true
						resp.Body.Close()
					}
				}
				select {
				case <-ctx.Done():
					return
				case ch <- node:
				}
			}(ip)
		}
		wg.Wait()
	}()
	return ch, nil
}

func (d *Discovery) Resolve(name string) (string, error) {
	agent, ok := d.registry[name]
	if !ok {
		return "", fmt.Errorf("agent %s not found", name)
	}
	if agent.Status != "up" {
		return "", fmt.Errorf("agent %s is down", name)
	}
	return agent.IP, nil
}

func (d *Discovery) ServiceURL(name, service string) (string, error) {
	agent, ok := d.registry[name]
	if !ok {
		return "", fmt.Errorf("agent %s not found", name)
	}
	port, ok := agent.Services[service]
	if !ok {
		return "", fmt.Errorf("service %s not found for agent %s", service, name)
	}
	return fmt.Sprintf("http://%s:%d", agent.IP, port), nil
}

func (d *Discovery) ListAgents() []*Agent {
	list := make([]*Agent, 0, len(d.registry))
	for _, a := range d.registry {
		list = append(list, a)
	}
	return list
}

func (d *Discovery) GetAgent(name string) (*Agent, error) {
	a, ok := d.registry[name]
	if !ok {
		return nil, fmt.Errorf("agent not found")
	}
	return a, nil
}

func (d *Discovery) GetAgentsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		agents := d.ListAgents()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(agents); err != nil {
			http.Error(w, "Encode error", http.StatusInternalServerError)
			return
		}
	}
}

func (d *Discovery) GetAgentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/swarm/agents/")
		if name == "" {
			http.Error(w, "Agent name required", http.StatusBadRequest)
			return
		}
		agent, err := d.GetAgent(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(agent); err != nil {
			http.Error(w, "Encode error", http.StatusInternalServerError)
			return
		}
	}
}

func (d *Discovery) Shutdown() {
	d.cancel()
}
