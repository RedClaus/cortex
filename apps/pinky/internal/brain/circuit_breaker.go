// Package brain provides circuit breaker functionality for resilient inference.
package brain

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	// CircuitClosed is the normal operating state - requests flow through.
	CircuitClosed CircuitState = iota
	// CircuitOpen means the circuit has tripped - requests are rejected.
	CircuitOpen
	// CircuitHalfOpen is the testing state - one request allowed to test recovery.
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker provides fault tolerance for inference backends.
// When a backend fails repeatedly, the circuit "opens" and requests
// are rejected immediately, preventing cascading failures.
//
// Usage:
//
//	cb := NewCircuitBreaker("vllm", CircuitBreakerConfig{
//	    FailureThreshold: 3,
//	    RecoveryTimeout:  30 * time.Second,
//	})
//
//	if cb.Allow() {
//	    err := makeRequest()
//	    if err != nil {
//	        cb.RecordFailure()
//	    } else {
//	        cb.RecordSuccess()
//	    }
//	} else {
//	    // Circuit is open, use fallback
//	}
type CircuitBreaker struct {
	name   string
	config CircuitBreakerConfig
	mu     sync.RWMutex

	state            CircuitState
	failures         int
	successes        int
	lastFailureTime  time.Time
	lastStateChange  time.Time
	consecutiveSucc  int // Consecutive successes in half-open state
}

// CircuitBreakerConfig configures the circuit breaker behavior.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures before opening the circuit.
	FailureThreshold int `yaml:"failure_threshold"` // Default: 3

	// RecoveryTimeout is how long to wait before trying to recover (half-open).
	RecoveryTimeout time.Duration `yaml:"recovery_timeout"` // Default: 30s

	// SuccessThreshold is the number of successes in half-open state before closing.
	SuccessThreshold int `yaml:"success_threshold"` // Default: 2

	// OnStateChange is called when the circuit state changes.
	OnStateChange func(name string, from, to CircuitState)
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  30 * time.Second,
		SuccessThreshold: 2,
	}
}

// NewCircuitBreaker creates a new circuit breaker for a named backend.
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 3
	}
	if config.RecoveryTimeout == 0 {
		config.RecoveryTimeout = 30 * time.Second
	}
	if config.SuccessThreshold == 0 {
		config.SuccessThreshold = 2
	}

	return &CircuitBreaker{
		name:            name,
		config:          config,
		state:           CircuitClosed,
		lastStateChange: time.Now(),
	}
}

// Allow checks if a request should be allowed through.
// Returns true if the request can proceed, false if it should be rejected.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true

	case CircuitOpen:
		// Check if recovery timeout has elapsed
		if time.Since(cb.lastStateChange) >= cb.config.RecoveryTimeout {
			cb.transitionTo(CircuitHalfOpen)
			return true // Allow one test request
		}
		return false

	case CircuitHalfOpen:
		// In half-open, only allow if we haven't started a test yet
		// or if we're allowing consecutive test requests
		return true

	default:
		return false
	}
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0

	switch cb.state {
	case CircuitHalfOpen:
		cb.consecutiveSucc++
		if cb.consecutiveSucc >= cb.config.SuccessThreshold {
			cb.transitionTo(CircuitClosed)
		}
	case CircuitClosed:
		// Already closed, nothing to do
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.consecutiveSucc = 0
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitClosed:
		if cb.failures >= cb.config.FailureThreshold {
			cb.transitionTo(CircuitOpen)
		}
	case CircuitHalfOpen:
		// Test request failed, reopen the circuit
		cb.transitionTo(CircuitOpen)
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns circuit breaker statistics.
func (cb *CircuitBreaker) Stats() CircuitStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitStats{
		Name:            cb.name,
		State:           cb.state.String(),
		Failures:        cb.failures,
		LastFailure:     cb.lastFailureTime,
		LastStateChange: cb.lastStateChange,
	}
}

// CircuitStats contains circuit breaker statistics.
type CircuitStats struct {
	Name            string    `json:"name"`
	State           string    `json:"state"`
	Failures        int       `json:"failures"`
	LastFailure     time.Time `json:"last_failure,omitempty"`
	LastStateChange time.Time `json:"last_state_change"`
}

// transitionTo changes the circuit state (must hold lock).
func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()

	if newState == CircuitClosed {
		cb.failures = 0
		cb.consecutiveSucc = 0
	}

	if cb.config.OnStateChange != nil {
		// Call callback without holding lock
		go cb.config.OnStateChange(cb.name, oldState, newState)
	}
}

// Reset forces the circuit to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.transitionTo(CircuitClosed)
}

// ═══════════════════════════════════════════════════════════════════════════════
// CIRCUIT BREAKER REGISTRY
// Manages circuit breakers for multiple backends
// ═══════════════════════════════════════════════════════════════════════════════

// CircuitBreakerRegistry manages circuit breakers for multiple inference backends.
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	config   CircuitBreakerConfig
}

// NewCircuitBreakerRegistry creates a new registry with default config.
func NewCircuitBreakerRegistry(config CircuitBreakerConfig) *CircuitBreakerRegistry {
	if config.FailureThreshold == 0 {
		config = DefaultCircuitBreakerConfig()
	}
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
	}
}

// Get returns the circuit breaker for a backend, creating one if needed.
func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	r.mu.RLock()
	cb, ok := r.breakers[name]
	r.mu.RUnlock()

	if ok {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, ok := r.breakers[name]; ok {
		return cb
	}

	cb = NewCircuitBreaker(name, r.config)
	r.breakers[name] = cb
	return cb
}

// AllStats returns statistics for all circuit breakers.
func (r *CircuitBreakerRegistry) AllStats() []CircuitStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make([]CircuitStats, 0, len(r.breakers))
	for _, cb := range r.breakers {
		stats = append(stats, cb.Stats())
	}
	return stats
}

// ═══════════════════════════════════════════════════════════════════════════════
// FALLBACK LANE SELECTOR
// Automatically selects fallback lane when primary is unavailable
// ═══════════════════════════════════════════════════════════════════════════════

// FallbackConfig configures automatic fallback behavior.
type FallbackConfig struct {
	// Primary is the preferred lane to use.
	Primary string `yaml:"primary"`

	// Fallbacks is the ordered list of fallback lanes.
	Fallbacks []string `yaml:"fallbacks"`

	// CircuitBreaker enables circuit breaker for the primary lane.
	UseCircuitBreaker bool `yaml:"use_circuit_breaker"`
}

// LaneSelectorWithFallback wraps lane selection with circuit breaker and fallback.
type LaneSelectorWithFallback struct {
	config   FallbackConfig
	registry *CircuitBreakerRegistry
}

// NewLaneSelectorWithFallback creates a new fallback-aware lane selector.
func NewLaneSelectorWithFallback(config FallbackConfig, registry *CircuitBreakerRegistry) *LaneSelectorWithFallback {
	return &LaneSelectorWithFallback{
		config:   config,
		registry: registry,
	}
}

// SelectLane returns the best available lane, considering circuit breaker state.
func (ls *LaneSelectorWithFallback) SelectLane(ctx context.Context) (string, error) {
	// Try primary lane first
	if ls.config.Primary != "" {
		cb := ls.registry.Get(ls.config.Primary)
		if cb.Allow() {
			return ls.config.Primary, nil
		}
	}

	// Try fallbacks in order
	for _, fallback := range ls.config.Fallbacks {
		cb := ls.registry.Get(fallback)
		if cb.Allow() {
			return fallback, nil
		}
	}

	return "", fmt.Errorf("all lanes unavailable (primary: %s, fallbacks: %v)",
		ls.config.Primary, ls.config.Fallbacks)
}
