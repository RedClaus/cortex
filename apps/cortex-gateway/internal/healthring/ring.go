package healthring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cortexhub/cortex-gateway/internal/config"
	"github.com/cortexhub/cortex-gateway/internal/discovery"
)

type HealthCheckResult struct {
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

type MemberStatus struct {
	Name    string               `json:"name"`
	Status  string               `json:"status"`
	History []HealthCheckResult  `json:"history"`
}

type HealthRing struct {
	memberStatuses map[string]*MemberStatus
	memberConfigs  map[string][]config.HealthCheckConfig
	discovery      *discovery.Discovery
	interval       time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
	logger         *slog.Logger
	historySize    int
}

func NewHealthRing(cfg config.HealthRingConfig, disc *discovery.Discovery, logger *slog.Logger) *HealthRing {
	if !cfg.Enabled {
		return nil
	}
	h := &HealthRing{
		memberStatuses: make(map[string]*MemberStatus),
		memberConfigs:  make(map[string][]config.HealthCheckConfig),
		discovery:      disc,
		interval:       cfg.CheckInterval,
		logger:         logger,
		historySize:    10,
	}
	for _, mc := range cfg.Members {
		h.memberConfigs[mc.Name] = mc.Checks
		status := &MemberStatus{
			Name:    mc.Name,
			Status:  "unknown",
			History: make([]HealthCheckResult, 0),
		}
		h.memberStatuses[mc.Name] = status
	}
	h.ctx, h.cancel = context.WithCancel(context.Background())
	go h.runChecks()
	return h
}

func (h *HealthRing) runChecks() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.performChecks()
		}
	}
}

func (h *HealthRing) performChecks() {
	for name, status := range h.memberStatuses {
		checks := h.memberConfigs[name]
		success := true
		var errors []string
		for _, check := range checks {
			res := h.performCheck(name, check)
			if !res.Success {
				success = false
				if res.Error != "" {
					errors = append(errors, res.Error)
				}
			}
		}
		overallRes := HealthCheckResult{
			Timestamp: time.Now(),
			Success:   success,
		}
		if !success && len(errors) > 0 {
			overallRes.Error = strings.Join(errors, "; ")
		}
		status.Status = "up"
		if !success {
			status.Status = "down"
		}
		status.History = append(status.History, overallRes)
		if len(status.History) > h.historySize {
			status.History = status.History[1:]
		}
		h.logger.Debug("Health check for member", "name", name, "status", status.Status)
	}
}

func (h *HealthRing) performCheck(memberName string, check config.HealthCheckConfig) HealthCheckResult {
	res := HealthCheckResult{Timestamp: time.Now()}
	if check.Type == "tcp" {
		if check.Port == nil {
			res.Success = false
			res.Error = "port required for tcp check"
			return res
		}
		ip, err := h.discovery.Resolve(memberName)
		if err != nil {
			res.Success = false
			res.Error = fmt.Sprintf("resolve failed: %v", err)
			return res
		}
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, *check.Port), 5*time.Second)
		if err != nil {
			res.Success = false
			res.Error = err.Error()
		} else {
			conn.Close()
			res.Success = true
		}
		return res
	}
	if check.Type == "http" {
		if check.URL == "" {
			res.Success = false
			res.Error = "url required for http check"
			return res
		}
		fullURL := h.resolveTemplate(check.URL)
		if fullURL == "" {
			res.Success = false
			res.Error = "failed to resolve URL"
			return res
		}
		client := &http.Client{Timeout: 5 * time.Second}
		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
			res.Success = false
			res.Error = err.Error()
			return res
		}
		resp, err := client.Do(req)
		if err != nil {
			res.Success = false
			res.Error = err.Error()
			return res
		}
		defer resp.Body.Close()
		expect := 200
		if check.ExpectStatus != nil {
			expect = *check.ExpectStatus
		}
		res.Success = resp.StatusCode == expect
		if !res.Success {
			res.Error = fmt.Sprintf("status %d expected %d", resp.StatusCode, expect)
		}
		return res
	}
	res.Success = false
	res.Error = fmt.Sprintf("unknown check type %s", check.Type)
	return res
}

func (h *HealthRing) resolveTemplate(tmpl string) string {
	if !strings.Contains(tmpl, "{{resolve ") {
		return tmpl
	}
	idx := strings.Index(tmpl, "{{resolve ")
	if idx == -1 {
		return tmpl
	}
	endIdx := strings.Index(tmpl[idx:], "}}")
	if endIdx == -1 {
		return tmpl
	}
	template := tmpl[idx+10 : idx+endIdx]
	parts := strings.Fields(template)
	if len(parts) < 2 {
		return tmpl
	}
	agent := parts[0]
	service := parts[1]
	surl, err := h.discovery.ServiceURL(agent, service)
	if err != nil {
		h.logger.Warn("resolve service failed", "agent", agent, "service", service, "error", err)
		return ""
	}
	path := tmpl[idx+endIdx+2:]
	return surl + path
}

func (h *HealthRing) Status() map[string]*MemberStatus {
	m := make(map[string]*MemberStatus)
	for k, v := range h.memberStatuses {
		m[k] = v
	}
	return m
}

func (h *HealthRing) GetMemberStatus(name string) (*MemberStatus, error) {
	s, ok := h.memberStatuses[name]
	if !ok {
		return nil, fmt.Errorf("member not found")
	}
	return s, nil
}

func (h *HealthRing) GetStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		status := h.Status()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			http.Error(w, "Encode error", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HealthRing) GetMemberHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/healthring/")
		if name == "" {
			http.Error(w, "Member name required", http.StatusBadRequest)
			return
		}
		member, err := h.GetMemberStatus(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(member); err != nil {
			http.Error(w, "Encode error", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HealthRing) Shutdown() {
	h.cancel()
}
