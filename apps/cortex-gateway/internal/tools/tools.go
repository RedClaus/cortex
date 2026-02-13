package tools

// Tool represents an external tool or plugin
type Tool struct {
	Name        string
	Description string
	Handler     func(input string) (string, error)
}

// NewTool creates a new tool
func NewTool(name, desc string, handler func(string) (string, error)) *Tool {
	return &Tool{
		Name:        name,
		Description: desc,
		Handler:     handler,
	}
}

// Execute runs the tool with the given input
func (t *Tool) Execute(input string) (string, error) {
	return t.Handler(input)
}
