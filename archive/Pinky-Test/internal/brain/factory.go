// Package brain provides a factory for creating Brain implementations.
package brain

import (
	"fmt"

	"github.com/normanking/pinky/internal/config"
)

// New creates a Brain implementation based on configuration.
// Returns EmbeddedBrain for "embedded" mode, RemoteBrain for "remote" mode.
func New(cfg *config.Config) (Brain, error) {
	switch cfg.Brain.Mode {
	case "embedded", "":
		return NewEmbeddedBrain(cfg.Inference), nil
	case "remote":
		if cfg.Brain.RemoteURL == "" {
			return nil, fmt.Errorf("remote brain mode requires remote_url configuration")
		}
		return NewRemoteBrain(cfg.Brain), nil
	default:
		return nil, fmt.Errorf("unknown brain mode: %s", cfg.Brain.Mode)
	}
}
