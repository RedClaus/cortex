// Package config provides backward compatibility - use pkg/config directly
package config

import (
	"github.com/RedClaus/cortex-coder-agent/pkg/config"
)

// Re-export types from pkg/config for backward compatibility
type (
	Config              = config.Config
	AgentConfig         = config.AgentConfig
	CortexBrainConfig   = config.CortexBrainConfig
	TUIConfig           = config.TUIConfig
	SkillsConfig        = config.SkillsConfig
	SessionConfig       = config.SessionConfig
)

// Re-export functions
var (
	DefaultConfig   = config.DefaultConfig
	Load            = config.Load
	GetConfigPath   = config.GetConfigPath
	ConfigFileExists = config.ConfigFileExists
)
