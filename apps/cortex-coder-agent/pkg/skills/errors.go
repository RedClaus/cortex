// Package skills provides skill management for the Cortex Coder Agent
package skills

import (
	"fmt"
)

// Errors provides structured error types for skills package
type Errors struct {
	errors []error
}

// NewErrors creates a new errors container
func NewErrors() *Errors {
	return &Errors{}
}

// Add adds an error to the container
func (e *Errors) Add(err error) {
	if err != nil {
		e.errors = append(e.errors, err)
	}
}

// Addf adds a formatted error
func (e *Errors) Addf(format string, args ...interface{}) {
	e.errors = append(e.errors, fmt.Errorf(format, args...))
}

// Error returns the combined error message
func (e *Errors) Error() string {
	if len(e.errors) == 0 {
		return ""
	}
	msgs := make([]string, len(e.errors))
	for i, err := range e.errors {
		msgs[i] = err.Error()
	}
	return fmt.Sprintf("%d errors: %s", len(e.errors), joinErrors(msgs, "; "))
}

// HasErrors returns true if there are any errors
func (e *Errors) HasErrors() bool {
	return len(e.errors) > 0
}

// Len returns the number of errors
func (e *Errors) Len() int {
	return len(e.errors)
}

// Errors returns all errors
func (e *Errors) Errors() []error {
	return e.errors
}

// Clear clears all errors
func (e *Errors) Clear() {
	e.errors = e.errors[:0]
}

// Join joins error messages with a separator
func joinErrors(errors []string, sep string) string {
	if len(errors) == 0 {
		return ""
	}
	if len(errors) == 1 {
		return errors[0]
	}
	result := errors[0]
	for i := 1; i < len(errors); i++ {
		result += sep + errors[i]
	}
	return result
}
