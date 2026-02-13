package inference

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/cortexhub/cortex-gateway/internal/config"
)

// Client is the interface for inference providers
type Client interface {
	Infer(req *Request) (*Response, error)
	Health() error
}

// Request represents an inference request
type Request struct {
	Prompt     string
	Model      string
	Options    map[string]interface{}
	SessionID  string
}

// Response represents an inference response
type Response struct {
	Content    string
	Model      string
	TokensUsed int
	SessionID  string
	Lane       string // add for response
}

// Router manages inference engines and lanes
type Router struct {
	lanes    map[string]*Lane
	engines  map[string]*Engine
	defaultLane string
	mu       sync.RWMutex
}

// Lane represents an inference routing lane
type Lane struct {
	Engine   *Engine
	Strategy string
}

// Engine represents a runtime inference engine
type Engine struct {
	Name     string
	Type     string
	URL      string
	Models   []string
	Default  string
	Hardware string
	Client   Client
}

// NewRouter creates a new inference router from config
func NewRouter(ctx context.Context, cfg *config.Config) (*Router, error) {
	r := &Router{
		lanes:     make(map[string]*Lane),
		engines:   make(map[string]*Engine),
		defaultLane: cfg.Inference.DefaultLane,
	}

	infCfg := &cfg.Inference

	// Auto-detect local engines
	if infCfg.AutoDetect {
		subnet := ""
		if cfg.Swarm.Discovery.Subnet != "" {
			subnet = cfg.Swarm.Discovery.Subnet
		}
		detected, err := DetectEngines(ctx, subnet)
		if err != nil {
			// log but continue
			fmt.Printf("Auto-detect failed: %v\\n", err)
		} else {
			for _, d := range detected {
				name := generateEngineName(d.Type, d.URL)
				client, err := createClient(d.Type, d.URL, d.Default, "")
				if err == nil {
					e := &Engine{
						Name:     name,
						Type:     d.Type,
						URL:      d.URL,
						Models:   d.Models,
						Default:  d.Default,
						Hardware: d.Hardware,
						Client:   client,
					}
					r.engines[name] = e
				} else {
					fmt.Printf("Failed to create client for %s: %v\\n", name, err)
				}
			}
		}
	}

	// Explicit engines from config
	for _, ec := range infCfg.Engines {
		name := ec.Name
		models := ec.Models
		if len(models) == 0 {
			models = ec.PreferredModels
		}
		if len(models) == 0 {
			models = []string{"default"}
		}
		defaultModel := models[0]
		client, err := createClient(ec.Type, ec.URL, defaultModel, ec.APIKey)
		if err != nil {
			fmt.Printf("Failed to create client for %s: %v\\n", name, err)
			continue
		}
		e := &Engine{
			Name:     name,
			Type:     ec.Type,
			URL:      ec.URL,
			Models:   models,
			Default:  defaultModel,
			Hardware: "",
			Client:   client,
		}
		r.engines[name] = e
	}

	// Lanes
	for _, lc := range infCfg.Lanes {
		var eng *Engine
		if lc.Engine != "" {
			if e, ok := r.engines[lc.Engine]; ok {
				eng = e
			} else {
				fmt.Printf("Engine %s not found for lane %s\\n", lc.Engine, lc.Name)
				continue
			}
		} else if lc.Provider != "" {
			// Backward compat: create implicit engine from lane
			typ := lc.Provider
			if typ == "openai" || typ == "openrouter" {
				typ = "openai-compatible"
			}
			url := lc.BaseURL
			apikey := lc.APIKey
			models := lc.Models
			if len(models) == 0 {
				models = []string{"gpt-3.5-turbo"} // default
			}
			defaultModel := models[0]
			name := lc.Name
			client, err := createClient(typ, url, defaultModel, apikey)
			if err != nil {
				fmt.Printf("Failed to create implicit client for lane %s: %v\\n", lc.Name, err)
				continue
			}
			eng = &Engine{
				Name:     name,
				Type:     typ,
				URL:      url,
				Models:   models,
				Default:  defaultModel,
				Hardware: "",
				Client:   client,
			}
			r.engines[name] = eng
		} else {
			fmt.Printf("Lane %s has no engine or provider\\n", lc.Name)
			continue
		}

		r.lanes[lc.Name] = &Lane{
			Engine:   eng,
			Strategy: lc.Strategy,
		}
	}

	if r.defaultLane != "" {
		if _, ok := r.lanes[r.defaultLane]; !ok {
			return nil, fmt.Errorf("default lane %s not found", r.defaultLane)
		}
	} else if len(r.lanes) > 0 {
		for name := range r.lanes {
			r.defaultLane = name
			break
		}
	}

	return r, nil
}

func generateEngineName(typ, urlStr string) string {
	u, _ := url.Parse(urlStr)
	host := u.Host
	return fmt.Sprintf("auto-%s-%s", typ, host)
}

func createClient(typ, baseURL, defaultModel, apiKey string) (Client, error) {
	switch typ {
	case "ollama":
		return NewOllamaClient(&OllamaConfig{URL: baseURL, DefaultModel: defaultModel})
	case "openai-compatible", "vllm", "mlx", "openai", "openrouter":
		return NewOpenAIClient(&OpenAIConfig{BaseURL: baseURL, APIKey: apiKey, Model: defaultModel})
	case "tgi":
		return NewTGIClient(baseURL), nil
	case "llamacpp":
		return NewLlamaCPPClient(baseURL), nil
	default:
		return nil, fmt.Errorf("unsupported inference type: %s", typ)
	}
}

// Infer routes the request to the appropriate engine
func (r *Router) Infer(lane string, req *Request) (*Response, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if lane == "" {
		lane = r.defaultLane
	}

	targetLane, ok := r.lanes[lane]
	if !ok {
		return nil, fmt.Errorf("lane %s not found", lane)
	}

	// Select model based on strategy if not specified
	if req.Model == "" {
		model := targetLane.Engine.Default
		if targetLane.Strategy == "fastest" {
			model = pickFastestModel(targetLane.Engine.Models)
		} // TODO: cheapest, roundrobin
		req.Model = model
	}

	// Check if model available
	found := false
	for _, m := range targetLane.Engine.Models {
		if m == req.Model {
			found = true
			break
		}
	}
	if !found {
		// fallback to default
		req.Model = targetLane.Engine.Default
	}

	res, err := targetLane.Engine.Client.Infer(req)
	if err == nil {
		res.Lane = lane
	}
	return res, err
}

// Health checks all engines
func (r *Router) Health() map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]error)
	for name, eng := range r.engines {
		results[name] = eng.Client.Health()
	}
	return results
}

// ListEngines returns list of engines
func (r *Router) ListEngines() []Engine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Engine, 0, len(r.engines))
	for _, e := range r.engines {
		list = append(list, *e)
	}
	return list
}

// ListModels returns flat list of all models
func (r *Router) ListModels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modelSet := make(map[string]bool)
	for _, e := range r.engines {
		for _, m := range e.Models {
			modelSet[m] = true
		}
	}
	models := make([]string, 0, len(modelSet))
	for m := range modelSet {
		models = append(models, m)
	}
	sort.Strings(models)
	return models
}

func pickFastestModel(models []string) string {
	if len(models) == 0 {
		return ""
	}
	sort.Slice(models, func(i, j int) bool {
		pi := parseParams(models[i])
		pj := parseParams(models[j])
		return pi < pj
	})
	return models[0]
}

func parseParams(name string) int {
	// Extract number before B or b
	i := strings.Index(name, "B")
	if i < 0 {
		i = strings.Index(name, "b")
	}
	if i > 0 {
		numStr := name[:i]
		// remove non-digits
		var numB int
		_, err := fmt.Sscanf(numStr, "%d", &numB)
		if err == nil {
			return numB
		}
	}
	return 999 // large if no params
}
