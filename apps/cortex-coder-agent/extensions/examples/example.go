// Package example provides example extensions for demonstration
package example

import (
	"context"
)

// ExampleExtension is a sample extension
type ExampleExtension struct{}

// Name returns the extension name
func (e *ExampleExtension) Name() string {
	return "example"
}

// Version returns the extension version
func (e *ExampleExtension) Version() string {
	return "1.0.0"
}

// Description returns the extension description
func (e *ExampleExtension) Description() string {
	return "Example extension demonstrating the extension system"
}

// Execute runs the extension
func (e *ExampleExtension) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Process input and return result
	return map[string]interface{}{
		"status":  "success",
		"message": "Example extension executed",
		"input":   input,
	}, nil
}
