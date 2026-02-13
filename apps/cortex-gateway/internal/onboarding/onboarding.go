package onboarding

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/cortexhub/cortex-gateway/internal/config"
)

type Onboarding struct {
	logger      *slog.Logger
	configPath  string
	complete    bool
	state       map[string]interface{}
	currentStep int
}

func New(logger *slog.Logger, configPath string) *Onboarding {
	complete := fileExists(configPath)
	o := &Onboarding{
		logger:      logger,
		configPath:  configPath,
		complete:    complete,
		state:       make(map[string]interface{}),
		currentStep: 0,
	}
	if !complete {
		o.currentStep = 1
	}
	return o
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (o *Onboarding) IsNeeded() bool {
	return !o.complete
}

func (o *Onboarding) CLI() error {
	if o.complete {
		o.logger.Info("Config already exists, skipping CLI onboarding")
		return nil
	}
	scanner := bufio.NewScanner(os.Stdin)
	o.logger.Info("Welcome to Cortex-Gateway Setup")
	o.promptStep1(scanner)
	o.promptStep2(scanner)
	o.promptStep3(scanner)
	o.promptStep4(scanner)
	o.promptStep5(scanner)
	o.buildHealthRing()
	return o.writeConfig()
}

func (o *Onboarding) promptStep1(scanner *bufio.Scanner) {
	fmt.Print("Step 1: Server config\nEnter host (default 0.0.0.0): ")
	scanner.Scan()
	host := strings.TrimSpace(scanner.Text())
	if host == "" {
		host = "0.0.0.0"
	}
	fmt.Print("Enter port (default 18800): ")
	scanner.Scan()
	var port int
	fmt.Sscanf(strings.TrimSpace(scanner.Text()), "%d", &port)
	if port == 0 {
		port = 18800
	}
	o.state["step1"] = map[string]interface{}{
		"host": host,
		"port": port,
	}
}

func (o *Onboarding) promptStep2(scanner *bufio.Scanner) {
	agents := []interface{}{}
	for {
		fmt.Print("Step 2: Swarm agents\nAdd agent? (y/n): ")
		scanner.Scan()
		if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
			break
		}
		fmt.Print("Agent name: ")
		scanner.Scan()
		name := strings.TrimSpace(scanner.Text())
		fmt.Print("Host IP: ")
		scanner.Scan()
		hostIP := strings.TrimSpace(scanner.Text())
		services := map[string]interface{}{}
		for {
			fmt.Print("Add service? (y/n): ")
			scanner.Scan()
			if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
				break
			}
			fmt.Print("Service name: ")
			scanner.Scan()
			svc := strings.TrimSpace(scanner.Text())
			fmt.Print("Port: ")
			scanner.Scan()
			var p int
			fmt.Sscanf(strings.TrimSpace(scanner.Text()), "%d", &p)
			services[svc] = p
		}
		agent := map[string]interface{}{
			"name":     name,
			"host":     hostIP,
			"services": services,
		}
		agents = append(agents, agent)
	}
	o.state["step2"] = map[string]interface{}{
		"agents": agents,
		"discovery": map[string]interface{}{
			"method":        "mdns+arp",
			"scan_interval": "5m",
			"subnet":        "192.168.1.0/24",
		},
	}
}

func (o *Onboarding) promptStep3(scanner *bufio.Scanner) {
	lanes := []interface{}{
		map[string]interface{}{
			"name":      "local",
			"provider":  "ollama",
			"base_url":  "http://localhost:11434",
			"models":    []string{"cortex-coder:latest", "go-coder:latest"},
		},
	}
	defaultLane := "local"
	cloudKey := ""
	fmt.Print("Enter xAI API key for cloud lane (optional): ")
	scanner.Scan()
	cloudKey = strings.TrimSpace(scanner.Text())
	if cloudKey != "" {
		lanes = append(lanes, map[string]interface{}{
			"name":     "cloud",
			"provider": "openai-compatible",
			"base_url": "https://api.x.ai/v1",
			"api_key":  cloudKey,
			"models":   []string{"grok-3", "grok-4-fast-reasoning", "grok-code-fast-1"},
		})
	}
	freeKey := ""
	fmt.Print("Enter OpenRouter API key for free lane (optional): ")
	scanner.Scan()
	freeKey = strings.TrimSpace(scanner.Text())
	if freeKey != "" {
		lanes = append(lanes, map[string]interface{}{
			"name":     "free",
			"provider": "openai-compatible",
			"base_url": "https://openrouter.ai/api/v1",
			"api_key":  freeKey,
			"models":   []string{"openrouter/free"},
		})
	}
	o.state["step3"] = map[string]interface{}{
		"lanes":        lanes,
		"default_lane": defaultLane,
	}
}

func (o *Onboarding) promptStep4(scanner *bufio.Scanner) {
	fmt.Print("Step 4: Channels\nTelegram token (optional): ")
	scanner.Scan()
	token := strings.TrimSpace(scanner.Text())
	telegram := map[string]interface{}{"enabled": token != "", "token": token}
	fmt.Print("Discord token (optional): ")
	scanner.Scan()
	token = strings.TrimSpace(scanner.Text())
	discord := map[string]interface{}{"enabled": token != "", "token": token}
	webchatPort := 18793
	fmt.Print("WebChat port (default 18793): ")
	scanner.Scan()
	var wp int
	fmt.Sscanf(strings.TrimSpace(scanner.Text()), "%d", &wp)
	if wp != 0 {
		webchatPort = wp
	}
	webchat := map[string]interface{}{"enabled": true, "port": webchatPort}
	o.state["step4"] = map[string]interface{}{
		"telegram": telegram,
		"discord":  discord,
		"webchat":  webchat,
	}
}

func (o *Onboarding) promptStep5(scanner *bufio.Scanner) {
	cbURL := "http://localhost:18892"
	fmt.Print("Step 5: CortexBrain\nURL (default http://localhost:18892): ")
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())
	if input != "" {
		cbURL = input
	}
	jwtSecret := "cortex-gateway-jwt-secret-change-in-production"
	fmt.Print("JWT secret (optional): ")
	scanner.Scan()
	input = strings.TrimSpace(scanner.Text())
	if input != "" {
		jwtSecret = input
	}
	o.state["step5"] = map[string]interface{}{
		"url":         cbURL,
		"jwt_secret":  jwtSecret,
	}
}

func (o *Onboarding) buildHealthRing() {
	agentsI, ok := o.state["step2"].(map[string]interface{})["agents"].([]interface{})
	if !ok {
		return
	}
	members := []interface{}{}
	for _, aI := range agentsI {
		a, ok := aI.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := a["name"].(string)
		checks := []interface{}{
			map[string]interface{}{"type": "tcp", "port": 22},
		}
		services, ok := a["services"].(map[string]interface{})
		if ok {
			if _, ok := services["bridge"]; ok {
				checks = append(checks, map[string]interface{}{
					"type":           "http",
					"url":            fmt.Sprintf("{{resolve %s bridge}}/health", name),
					"expect_status":  200,
				})
			}
			if _, ok := services["cortexbrain"]; ok {
				checks = append(checks, map[string]interface{}{
					"type": "http",
					"url":  fmt.Sprintf("{{resolve %s cortexbrain}}/health", name),
				})
			}
			if _, ok := services["ollama"]; ok {
				checks = append(checks, map[string]interface{}{
					"type": "http",
					"url":  fmt.Sprintf("{{resolve %s ollama}}/api/tags", name),
				})
			}
			if portI, ok := services["redis"]; ok {
				port := int(portI.(float64))
				checks = append(checks, map[string]interface{}{
					"type": "tcp",
					"port": port,
				})
			}
		}
		members = append(members, map[string]interface{}{
			"name":   name,
			"checks": checks,
		})
	}
	o.state["step6"] = map[string]interface{}{
		"enabled":        true,
		"check_interval": "30s",
		"members":        members,
	}
}

func (o *Onboarding) writeConfig() error {
	for i := 1; i <= 6; i++ {
		key := fmt.Sprintf("step%d", i)
		if d, ok := o.state[key]; ok {
			switch i {
			case 1:
				o.state["server"] = d
			case 2:
				o.state["swarm"] = d
			case 3:
				o.state["inference"] = d
			case 4:
				o.state["channels"] = d
			case 5:
				o.state["cortexbrain"] = d
			case 6:
				o.state["healthring"] = d
			}
			delete(o.state, key)
		}
	}

	cfg := &config.Config{
		Logging: config.LoggingConfig{Level: "info", Format: "json"},
		Ollama:  config.OllamaConfig{URL: "http://localhost:11434"},
		Bridge:  config.BridgeConfig{URL: "http://192.168.1.229:18802"},
	}

	if server, ok := o.state["server"].(map[string]interface{}); ok {
		cfg.Server.Host = server["host"].(string)
		cfg.Server.Port = int(server["port"].(float64))
	}

	if cortexbrain, ok := o.state["cortexbrain"].(map[string]interface{}); ok {
		cfg.CortexBrain.URL = cortexbrain["url"].(string)
		cfg.CortexBrain.JWTSecret = cortexbrain["jwt_secret"].(string)
	}

	if channels, ok := o.state["channels"].(map[string]interface{}); ok {
		if tg, ok := channels["telegram"].(map[string]interface{}); ok {
			cfg.Channels.Telegram.Enabled = tg["enabled"].(bool)
			cfg.Channels.Telegram.Token = tg["token"].(string)
		}
		if dc, ok := channels["discord"].(map[string]interface{}); ok {
			cfg.Channels.Discord.Enabled = dc["enabled"].(bool)
			cfg.Channels.Discord.Token = dc["token"].(string)
		}
		if wc, ok := channels["webchat"].(map[string]interface{}); ok {
			cfg.Channels.WebChat.Enabled = wc["enabled"].(bool)
			cfg.Channels.WebChat.Port = int(wc["port"].(float64))
		}
	}

	if inference, ok := o.state["inference"].(map[string]interface{}); ok {
		cfg.Inference.DefaultLane = inference["default_lane"].(string)
		if lanesI, ok := inference["lanes"].([]interface{}); ok {
			for _, lI := range lanesI {
				if l, ok := lI.(map[string]interface{}); ok {
					lane := config.LaneConfig{
						Name:     l["name"].(string),
						Provider: l["provider"].(string),
						BaseURL:  l["base_url"].(string),
					}
					if keyI, ok := l["api_key"]; ok && keyI != "" {
						lane.APIKey = keyI.(string)
					}
					if modelsI, ok := l["models"].([]interface{}); ok {
						for _, mI := range modelsI {
							lane.Models = append(lane.Models, mI.(string))
						}
					}
					cfg.Inference.Lanes = append(cfg.Inference.Lanes, lane)
				}
			}
		}
	}

	if swarm, ok := o.state["swarm"].(map[string]interface{}); ok {
		cfg.Swarm.Discovery.Method = swarm["discovery"].(map[string]interface{})["method"].(string)
		scanStr := swarm["discovery"].(map[string]interface{})["scan_interval"].(string)
		cfg.Swarm.Discovery.ScanInterval, _ = time.ParseDuration(scanStr)
		cfg.Swarm.Discovery.Subnet = swarm["discovery"].(map[string]interface{})["subnet"].(string)
		if agentsI, ok := swarm["agents"].([]interface{}); ok {
			for _, aI := range agentsI {
				if a, ok := aI.(map[string]interface{}); ok {
					agent := config.AgentConfig{
						Name: a["name"].(string),
						Host: a["host"].(string),
					}
					if servicesI, ok := a["services"].(map[string]interface{}); ok {
						for k, vI := range servicesI {
							v := int(vI.(float64))
							agent.Services[k] = v
						}
					}
					cfg.Swarm.Agents = append(cfg.Swarm.Agents, agent)
				}
			}
		}
	}

	if healthring, ok := o.state["healthring"].(map[string]interface{}); ok {
		cfg.HealthRing.Enabled = healthring["enabled"].(bool)
		checkStr := healthring["check_interval"].(string)
		cfg.HealthRing.CheckInterval, _ = time.ParseDuration(checkStr)
		if membersI, ok := healthring["members"].([]interface{}); ok {
			for _, mI := range membersI {
				if m, ok := mI.(map[string]interface{}); ok {
					member := config.HealthMemberConfig{Name: m["name"].(string)}
					if checksI, ok := m["checks"].([]interface{}); ok {
						for _, cI := range checksI {
							if c, ok := cI.(map[string]interface{}); ok {
								check := config.HealthCheckConfig{Type: c["type"].(string)}
								if url, ok := c["url"]; ok {
									check.URL = url.(string)
								}
								if portI, ok := c["port"]; ok {
									p := int(portI.(float64))
									check.Port = &p
								}
								if expectI, ok := c["expect_status"]; ok {
									e := int(expectI.(float64))
									check.ExpectStatus = &e
								}
								member.Checks = append(member.Checks, check)
							}
						}
					}
					cfg.HealthRing.Members = append(cfg.HealthRing.Members, member)
				}
			}
		}
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(o.configPath, data, 0644)
}

// API Handlers

func (o *Onboarding) StatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"needed": o.IsNeeded()})
	}
}

func (o *Onboarding) StartHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if o.IsNeeded() {
			o.currentStep = 1
			o.state = make(map[string]interface{})
			stepInfo := map[string]interface{}{
				"step":        1,
				"description": "Server configuration",
				"fields":      []string{"host", "port"},
			}
			json.NewEncoder(w).Encode(stepInfo)
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"complete": true})
		}
	}
}

func (o *Onboarding) StepHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		stepStr := strings.TrimPrefix(r.URL.Path, "/api/v1/onboarding/step/")
		step, err := strconv.Atoi(stepStr)
		if err != nil {
			http.Error(w, "Invalid step", http.StatusBadRequest)
			return
		}
		var data map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		o.state[fmt.Sprintf("step%d", step)] = data
		nextStep := step + 1
		if nextStep > 6 {
			nextStep = 6
		}
		stepInfo := map[string]interface{}{
			"next_step": nextStep,
		}
		if nextStep == 6 {
			o.buildHealthRing()
			stepInfo["description"] = "Health ring auto-configured, ready to complete"
		}
		json.NewEncoder(w).Encode(stepInfo)
	}
}

func (o *Onboarding) CompleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		err := o.writeConfig()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		o.complete = true
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func (o *Onboarding) ImportHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Read body failed", http.StatusBadRequest)
			return
		}
		var swarmCfg config.SwarmConfig
		err = yaml.Unmarshal(body, &swarmCfg)
		if err != nil {
			err = json.Unmarshal(body, &swarmCfg)
			if err != nil {
				http.Error(w, "Invalid YAML/JSON for swarm config", http.StatusBadRequest)
				return
			}
		}
		o.state["step2"] = map[string]interface{}{
			"agents":    swarmCfg.Agents,
			"discovery": swarmCfg.Discovery,
		}
		o.buildHealthRing()
		o.state["step1"] = map[string]interface{}{"host": "0.0.0.0", "port": 18800}
		o.state["step3"] = map[string]interface{}{
			"default_lane": "local",
			"lanes": []interface{}{
				map[string]interface{}{
					"name":      "local",
					"provider":  "ollama",
					"base_url":  "http://localhost:11434",
					"models":    []string{"cortex-coder:latest"},
				},
			},
		}
		o.state["step4"] = map[string]interface{}{
			"telegram": map[string]interface{}{"enabled": false, "token": ""},
			"discord":  map[string]interface{}{"enabled": false, "token": ""},
			"webchat":  map[string]interface{}{"enabled": true, "port": 18793},
		}
		o.state["step5"] = map[string]interface{}{
			"url":        "http://localhost:18892",
			"jwt_secret": "cortex-gateway-jwt-secret-change-in-production",
		}
		err = o.writeConfig()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		o.complete = true
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "imported": "swarm config"})
	}
}
