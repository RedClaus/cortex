// Package extensions provides the extension system for the Cortex Coder Agent
package extensions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
)

// Manager handles extension loading and execution
type Manager struct {
	extensionsPath string
	extensions     map[string]Extension
	plugins        map[string]*plugin.Plugin
}

// Extension represents a loaded extension
type Extension interface {
	Name() string
	Version() string
	Description() string
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

// ExtensionInfo provides metadata about an extension
type ExtensionInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Path        string `json:"path"`
}

// NewManager creates a new extension manager
func NewManager(extensionsPath string) *Manager {
	return &Manager{
		extensionsPath: extensionsPath,
		extensions:     make(map[string]Extension),
		plugins:        make(map[string]*plugin.Plugin),
	}
}

// LoadAll loads all extensions from the extensions directory
func (m *Manager) LoadAll() error {
	entries, err := os.ReadDir(m.extensionsPath)
	if err != nil {
		return fmt.Errorf("failed to read extensions directory: %w", err)
	}
	
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		path := filepath.Join(m.extensionsPath, entry.Name())
		if err := m.Load(path); err != nil {
			return fmt.Errorf("failed to load extension %s: %w", entry.Name(), err)
		}
	}
	
	return nil
}

// Load loads an extension from a directory
func (m *Manager) Load(path string) error {
	// Try to load as a Go plugin
	soPath := filepath.Join(path, fmt.Sprintf("%s.so", filepath.Base(path)))
	
	p, err := plugin.Open(soPath)
	if err != nil {
		// Not a plugin, skip
		return nil
	}
	
	sym, err := p.Lookup("Extension")
	if err != nil {
		return fmt.Errorf("extension must export 'Extension' symbol: %w", err)
	}
	
	ext, ok := sym.(Extension)
	if !ok {
		return fmt.Errorf("invalid extension type")
	}
	
	name := ext.Name()
	m.extensions[name] = ext
	m.plugins[name] = p
	return nil
}

// Get retrieves an extension by name
func (m *Manager) Get(name string) (Extension, bool) {
	ext, ok := m.extensions[name]
	return ext, ok
}

// List returns all loaded extensions
func (m *Manager) List() []ExtensionInfo {
	infos := make([]ExtensionInfo, 0, len(m.extensions))
	for _, ext := range m.extensions {
		infos = append(infos, ExtensionInfo{
			Name:        ext.Name(),
			Version:     ext.Version(),
			Description: ext.Description(),
		})
	}
	return infos
}

// Execute runs an extension by name
func (m *Manager) Execute(ctx context.Context, name string, input map[string]interface{}) (map[string]interface{}, error) {
	ext, ok := m.extensions[name]
	if !ok {
		return nil, fmt.Errorf("extension not found: %s", name)
	}
	return ext.Execute(ctx, input)
}
