// Package skills provides skill management for the Cortex Coder Agent
package skills

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parser handles YAML parsing for skills
type Parser struct {
	errors []ParseError
}

// ParseError represents a parsing error with line number information
type ParseError struct {
	File     string
	Line     int
	Column   int
	Message  string
	Inner    error
}

func (e ParseError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("line %d: %s", e.Line, e.Message)
}

func (e ParseError) Unwrap() error {
	return e.Inner
}

// NewParser creates a new YAML parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile parses a skill from a YAML file
func (p *Parser) ParseFile(path string) (*Skill, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, &ParseError{
			File:    path,
			Message: fmt.Sprintf("failed to open file: %v", err),
			Inner:   err,
		}
	}
	defer file.Close()

	return p.ParseReader(path, file)
}

// ParseReader parses a skill from an io.Reader
func (p *Parser) ParseReader(name string, r io.Reader) (*Skill, error) {
	var skill Skill
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)

	if err := decoder.Decode(&skill); err != nil {
		// Try to extract line number from YAML error
		parseErr := &ParseError{
			File:    name,
			Message: fmt.Sprintf("failed to parse YAML: %v", err),
			Inner:   err,
		}

		// Extract position information if available
		if strings.Contains(err.Error(), "line ") {
			parts := strings.Split(err.Error(), "line ")
			if len(parts) > 1 {
				linePart := strings.Split(parts[1], ":")
				if len(linePart) > 0 {
					fmt.Sscanf(linePart[0], "%d", &parseErr.Line)
				}
				if len(linePart) > 1 {
					fmt.Sscanf(linePart[1], "%d", &parseErr.Column)
				}
			}
		}

		return nil, parseErr
	}

	// Validate the parsed skill
	if errs := skill.Validate(); len(errs) > 0 {
		return nil, &ParseError{
			File:    name,
			Message: "validation failed",
			Inner:   fmt.Errorf("%v", errs),
		}
	}

	skill.Source = name
	return &skill, nil
}

// ParseDir parses all YAML files in a directory
func (p *Parser) ParseDir(dir string) ([]*Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var skills []*Skill
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		skill, err := p.ParseFile(path)
		if err != nil {
			p.errors = append(p.errors, ParseError{
				File:    path,
				Message: err.Error(),
				Inner:   err,
			})
			continue
		}

		skills = append(skills, skill)
	}

	return skills, nil
}

// GetErrors returns all parsing errors encountered
func (p *Parser) GetErrors() []ParseError {
	return p.errors
}

// ClearErrors clears all parsing errors
func (p *Parser) ClearErrors() {
	p.errors = p.errors[:0]
}

// ParseTrigger parses a trigger from YAML node
func (p *Parser) ParseTrigger(node *yaml.Node) (Trigger, error) {
	var trigger Trigger

	// Navigate to the map content
	for i, n := range node.Content {
		if n.Kind == yaml.ScalarNode {
			key := n.Value
			var valueNode *yaml.Node
			if i+1 < len(node.Content) {
				valueNode = node.Content[i+1]
			}

			switch key {
			case "type":
				if valueNode != nil {
					trigger.Type = valueNode.Value
				}
			case "pattern":
				if valueNode != nil {
					trigger.Pattern = valueNode.Value
				}
			case "extensions":
				if valueNode != nil && valueNode.Kind == yaml.SequenceNode {
					for _, extNode := range valueNode.Content {
						trigger.Extensions = append(trigger.Extensions, extNode.Value)
					}
				}
			case "command":
				if valueNode != nil {
					trigger.Command = valueNode.Value
				}
			}
		}
	}

	return trigger, nil
}

// SafeParseFile parses a skill file and returns errors instead of failing
func (p *Parser) SafeParseFile(path string) (*Skill, []ParseError) {
	skill, err := p.ParseFile(path)
	if err != nil {
		return nil, []ParseError{
			{
				File:    path,
				Message: err.Error(),
				Inner:   err,
			},
		}
	}
	return skill, nil
}
