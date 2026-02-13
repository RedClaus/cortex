package parsers

// ParsedDocument represents a parsed source document with AST structure.
type ParsedDocument struct {
	Title    string     `json:"title"`
	Metadata Metadata   `json:"metadata,omitempty"`
	Sections []*Section `json:"sections"`
}

// Metadata holds document-level metadata.
type Metadata struct {
	Author   string            `json:"author,omitempty"`
	Date     string            `json:"date,omitempty"`
	Tags     []string          `json:"tags,omitempty"`
	Language string            `json:"language,omitempty"`
	Custom   map[string]string `json:"custom,omitempty"`
}

// Section represents a hierarchical section of a document.
type Section struct {
	Level       int          `json:"level"`        // Header level (1-6 for Markdown)
	Title       string       `json:"title"`        // Section heading
	Content     string       `json:"content"`      // Text content (excluding code blocks)
	CodeBlocks  []*CodeBlock `json:"code_blocks"`  // Extracted code blocks
	Subsections []*Section   `json:"subsections"`  // Nested sections
	StartLine   int          `json:"start_line"`   // Source line number
	EndLine     int          `json:"end_line"`     // Source line number
}

// CodeBlock represents a code snippet.
type CodeBlock struct {
	Language string `json:"language,omitempty"`
	Code     string `json:"code"`
	Caption  string `json:"caption,omitempty"`
	Line     int    `json:"line"` // Line number in source
}
