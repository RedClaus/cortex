package inference

import (
	
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	
	"sort"
	
	
	"time"
)

// DetectedEngine represents a detected inference engine
type DetectedEngine struct {
	Type     string   // "ollama", "mlx", "vllm", "llamacpp", "tgi"
	URL      string
	Models   []string // auto-discovered available models
	Default  string   // fastest/recommended model
	Hardware string   // "cpu", "cuda", "metal", "rocm"
}

// DetectEngines probes for local inference engines
// subnet can be empty for local only, or CIDR like "192.168.1.0/24"
func DetectEngines(ctx context.Context, subnet string) ([]DetectedEngine, error) {
	hosts := getHosts(subnet)
	var engines []DetectedEngine
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, host := range hosts {
		engines = append(engines, probeHost(ctx, host, client)...)
	}

	// Remove duplicates by URL
	uniqueEngines := make(map[string]DetectedEngine)
	for _, e := range engines {
		u := e.URL
		if existing, ok := uniqueEngines[u]; ok {
			// Merge models
			mergedModels := append(existing.Models, e.Models...)
			sort.Strings(mergedModels)
			uniqueModels := make([]string, 0)
			seen := make(map[string]bool)
			for _, m := range mergedModels {
				if !seen[m] {
					seen[m] = true
					uniqueModels = append(uniqueModels, m)
				}
			}
			uniqueEngines[u] = DetectedEngine{
				Type:     existing.Type, // keep first type
				URL:      u,
				Models:   uniqueModels,
				Default:  pickDefault(uniqueModels),
				Hardware: existing.Hardware,
			}
		} else {
			e.Default = pickDefault(e.Models)
			uniqueEngines[u] = e
		}
	}

	result := make([]DetectedEngine, 0, len(uniqueEngines))
	for _, e := range uniqueEngines {
		result = append(result, e)
	}
	return result, nil
}

func getHosts(subnet string) []string {
	hosts := []string{"127.0.0.1", "localhost"}
	if subnet == "" {
		return hosts
	}

	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return hosts
	}

	// Generate IPs in subnet (simple for /24)
	ones, bits := ipNet.Mask.Size()
	if bits != 32 || ones != 24 { // only support /24 for now
		return hosts
	}

	ip := ipNet.IP.To4()
	for i := 1; i < 255; i++ { // skip .0 and .255
		hIP := make(net.IP, 4)
		copy(hIP, ip)
		hIP[3] = byte(i)
		hosts = append(hosts, hIP.String())
	}
	return hosts
}

func probeHost(ctx context.Context, host string, client *http.Client) []DetectedEngine {
	var engines []DetectedEngine

	// Ollama on 11434
	if ollama, ok := probeOllama(ctx, host, client); ok {
		engines = append(engines, ollama)
	}

	// vLLM on 8000
	if vllm, ok := probeVLLM(ctx, host, client); ok {
		engines = append(engines, vllm)
	}

	// Port 8080 probes
	port8080Engines := probePort8080(ctx, host, client)
	engines = append(engines, port8080Engines...)

	return engines
}

func probeOllama(ctx context.Context, host string, client *http.Client) (DetectedEngine, bool) {
	u := fmt.Sprintf("http://%s:11434/api/tags", host)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return DetectedEngine{}, false
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return DetectedEngine{}, false
	}
	defer resp.Body.Close()

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return DetectedEngine{}, false
	}

	models := make([]string, len(tags.Models))
	for i, m := range tags.Models {
		models[i] = m.Name
	}
	sort.Strings(models)

	return DetectedEngine{
		Type:     "ollama",
		URL:      fmt.Sprintf("http://%s:11434", host),
		Models:   models,
		Default:  pickDefault(models),
		Hardware: "auto", // detect later if needed
	}, true
}

func probeVLLM(ctx context.Context, host string, client *http.Client) (DetectedEngine, bool) {
	u := fmt.Sprintf("http://%s:8000/v1/models", host)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return DetectedEngine{}, false
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return DetectedEngine{}, false
	}
	defer resp.Body.Close()

	_, models, ok := parseOpenAIModels(resp.Body)
	if !ok {
		return DetectedEngine{}, false
	}

	return DetectedEngine{
		Type:     "vllm",
		URL:      fmt.Sprintf("http://%s:8000", host),
		Models:   models,
		Default:  pickDefault(models),
		Hardware: "cuda", // vLLM typically CUDA
	}, true
}

func probePort8080(ctx context.Context, host string, client *http.Client) []DetectedEngine {
	var engines []DetectedEngine

	// Try TGI first: /info
	u := fmt.Sprintf("http://%s:8080/info", host)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		// Parse TGI info: {"model_name": "...", "max_input_tokens": ...}
		var info map[string]interface{}
		json.Unmarshal(body, &info)
		if modelName, ok := info["model_name"].(string); ok && modelName != "" {
			engines = append(engines, DetectedEngine{
				Type:    "tgi",
				URL:     fmt.Sprintf("http://%s:8080", host),
				Models:  []string{modelName},
				Default: modelName,
				Hardware: getHardwareFromInfo(info),
			})
			return engines // if TGI, stop
		}
	}

	// Try llama.cpp: /health then /props
	u = fmt.Sprintf("http://%s:8080/health", host)
	req, _ = http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
	} else {
		defer resp.Body.Close()
		// Now /props
		u = fmt.Sprintf("http://%s:8080/props", host) // assume /props endpoint, adjust if needed
		req, _ = http.NewRequestWithContext(ctx, "GET", u, nil)
		resp2, err2 := client.Do(req)
		if err2 == nil && resp2.StatusCode == 200 {
			defer resp2.Body.Close()
			body, _ := io.ReadAll(resp2.Body)
			var props map[string]interface{}
			json.Unmarshal(body, &props)
			if modelName, ok := props["model"].(string); ok {
				hardware := "cpu"
				if gpuLayers, ok := props["n_gpu_layers"].(float64); ok && gpuLayers > 0 {
					hardware = "cuda"
				}
				engines = append(engines, DetectedEngine{
					Type:    "llamacpp",
					URL:     fmt.Sprintf("http://%s:8080", host),
					Models:  []string{modelName},
					Default: modelName,
					Hardware: hardware,
				})
				return engines // if llama, stop
			}
		}
	}

	// Try OpenAI-compatible (MLX etc): /v1/models
	u = fmt.Sprintf("http://%s:8080/v1/models", host)
	req, _ = http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err = client.Do(req)
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		e, _, ok := parseOpenAIModels(resp.Body)
		if ok {
			e.Type = "mlx" // assume MLX on 8080
			e.URL = fmt.Sprintf("http://%s:8080", host)
			e.Hardware = "metal" // MLX is Apple Metal
			engines = append(engines, e)
		}
	}

	return engines
}

func parseOpenAIModels(r io.Reader) (DetectedEngine, []string, bool) {
	body, err := io.ReadAll(r)
	if err != nil {
		return DetectedEngine{}, nil, false
	}

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return DetectedEngine{}, nil, false
	}

	models := make([]string, len(resp.Data))
	for i, m := range resp.Data {
		models[i] = m.ID
	}
	sort.Strings(models)

	e := DetectedEngine{
		Type:   "openai-compatible",
		Models: models,
	}
	return e, models, len(models) > 0
}

func getHardwareFromInfo(info map[string]interface{}) string {
	// TODO: parse from TGI info
	return "cuda" // typical
}

func pickDefault(models []string) string {
	if len(models) == 0 {
		return ""
	}
	// Simple: first after sort, assumes sorted by name, smaller first
	return models[0]
}

// Note: For llama.cpp /props, endpoint might be different; adjust based on actual server.
