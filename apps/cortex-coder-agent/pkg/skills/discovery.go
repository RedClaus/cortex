// Package skills provides skill management for the Cortex Coder Agent
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Registry manages skill discovery and registration
type Registry struct {
	skills         map[string]*Skill
	byCommand      map[string]*Skill
	byExtension    map[string][]*Skill
	byTriggerType  map[string][]*Skill
	builtinPath    string
	userPath       string
	projectPath    string
	parser         *Parser
	templateEngine *Engine
}

// NewRegistry creates a new skill registry
func NewRegistry(builtinPath, userPath, projectPath string) *Registry {
	return &Registry{
		skills:        make(map[string]*Skill),
		byCommand:     make(map[string]*Skill),
		byExtension:   make(map[string][]*Skill),
		byTriggerType: make(map[string][]*Skill),
		builtinPath:   builtinPath,
		userPath:      userPath,
		projectPath:   projectPath,
		parser:        NewParser(),
		templateEngine: NewEngine(),
	}
}

// LoadAll loads all skills from all configured paths
func (r *Registry) LoadAll() error {
	// Load built-in skills
	if err := r.loadFromDir(r.builtinPath); err != nil {
		return fmt.Errorf("failed to load built-in skills: %w", err)
	}

	// Load user skills
	if err := r.loadFromDir(r.userPath); err != nil {
		return fmt.Errorf("failed to load user skills: %w", err)
	}

	// Load project skills
	if err := r.loadFromDir(r.projectPath); err != nil {
		return fmt.Errorf("failed to load project skills: %w", err)
	}

	return nil
}

// loadFromDir loads all YAML files from a directory
func (r *Registry) loadFromDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, skip
	}

	skills, err := r.parser.ParseDir(dir)
	if err != nil {
		return err
	}

	for _, skill := range skills {
		r.Register(skill)
	}

	return nil
}

// Register registers a skill in the registry
func (r *Registry) Register(skill *Skill) {
	if skill == nil {
		return
	}

	// Register by name
	r.skills[skill.Name] = skill

	// Register by triggers
	for _, trigger := range skill.Triggers {
		switch trigger.Type {
		case "command":
			if trigger.Command != "" {
				r.byCommand[trigger.Command] = skill
			}
		case "file":
			for _, ext := range trigger.Extensions {
				if ext[0] == '.' {
					ext = ext[1:]
				}
				r.byExtension[ext] = append(r.byExtension[ext], skill)
			}
		case "selection":
			r.byTriggerType["selection"] = append(r.byTriggerType["selection"], skill)
		case "explicit":
			r.byTriggerType["explicit"] = append(r.byTriggerType["explicit"], skill)
		}
	}
}

// Get retrieves a skill by name
func (r *Registry) Get(name string) (*Skill, bool) {
	skill, ok := r.skills[name]
	return skill, ok
}

// GetByCommand retrieves a skill by command name
func (r *Registry) GetByCommand(cmd string) (*Skill, bool) {
	skill, ok := r.byCommand[cmd]
	return skill, ok
}

// GetByFileExtension retrieves skills that handle a specific file extension
func (r *Registry) GetByFileExtension(ext string) []*Skill {
	if ext[0] == '.' {
		ext = ext[1:]
	}
	return r.byExtension[ext]
}

// GetByTriggerType retrieves skills by trigger type
func (r *Registry) GetByTriggerType(triggerType string) []*Skill {
	return r.byTriggerType[triggerType]
}

// FindForSelection returns skills that can handle a text selection
func (r *Registry) FindForSelection() []*Skill {
	return r.byTriggerType["selection"]
}

// FindForFile returns skills that can handle the given file
func (r *Registry) FindForFile(path string) []*Skill {
	ext := filepath.Ext(path)
	if ext == "" {
		return nil
	}
	return r.GetByFileExtension(ext)
}

// List returns all registered skills
func (r *Registry) List() []*Skill {
	skills := make([]*Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		skills = append(skills, skill)
	}
	return skills
}

// ListByCategory returns skills grouped by category
func (r *Registry) ListByCategory() map[string][]*Skill {
	result := make(map[string][]*Skill)
	for _, skill := range r.skills {
		cat := skill.Category
		if cat == "" {
			cat = "uncategorized"
		}
		result[cat] = append(result[cat], skill)
	}
	return result
}

// Count returns the total number of registered skills
func (r *Registry) Count() int {
	return len(r.skills)
}

// Remove removes a skill from the registry
func (r *Registry) Remove(name string) {
	skill, ok := r.Get(name)
	if !ok {
		return
	}

	delete(r.skills, name)

	// Clean up trigger indexes
	for _, trigger := range skill.Triggers {
		switch trigger.Type {
		case "command":
			delete(r.byCommand, trigger.Command)
		case "file":
			for _, ext := range trigger.Extensions {
				if ext[0] == '.' {
					ext = ext[1:]
				}
				skills := r.byExtension[ext]
				for i, s := range skills {
					if s.Name == name {
						r.byExtension[ext] = append(skills[:i], skills[i+1:]...)
						break
					}
				}
			}
		case "selection":
			skills := r.byTriggerType["selection"]
			for i, s := range skills {
				if s.Name == name {
					r.byTriggerType["selection"] = append(skills[:i], skills[i+1:]...)
					break
				}
			}
		}
	}
}

// Clear removes all skills from the registry
func (r *Registry) Clear() {
	r.skills = make(map[string]*Skill)
	r.byCommand = make(map[string]*Skill)
	r.byExtension = make(map[string][]*Skill)
	r.byTriggerType = make(map[string][]*Skill)
}

// GetParser returns the registry's parser
func (r *Registry) GetParser() *Parser {
	return r.parser
}

// GetTemplateEngine returns the registry's template engine
func (r *Registry) GetTemplateEngine() *Engine {
	return r.templateEngine
}

// DefaultPaths returns the default skill paths
func DefaultPaths() (builtin, user, project string) {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	builtin = "skills/builtin"
	user = filepath.Join(home, ".config", "cortex-coder", "skills")
	project = filepath.Join(cwd, ".coder", "skills")

	return
}

// EnsurePaths creates skill directories if they don't exist
func EnsurePaths() (builtin, user, project string, err error) {
	builtin, user, project = DefaultPaths()

	// Create built-in skills directory
	if err := os.MkdirAll(builtin, 0755); err != nil {
		return "", "", "", fmt.Errorf("failed to create built-in skills dir: %w", err)
	}

	// Create user skills directory
	if err := os.MkdirAll(user, 0755); err != nil {
		return "", "", "", fmt.Errorf("failed to create user skills dir: %w", err)
	}

	// Create project skills directory
	if err := os.MkdirAll(project, 0755); err != nil {
		return "", "", "", fmt.Errorf("failed to create project skills dir: %w", err)
	}

	return
}

// GetSkillNames returns all skill names that match the given pattern
func (r *Registry) GetSkillNames(pattern string) []string {
	var names []string
	for name := range r.skills {
		if strings.Contains(name, pattern) {
			names = append(names, name)
		}
	}
	return names
}
