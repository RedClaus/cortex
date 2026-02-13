// Package indexer provides project directory scanning and file analysis capabilities.
package indexer

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileAnalysis represents the analyzed content and structure of a file.
type FileAnalysis struct {
	// Path is the relative path from project root
	Path string `json:"path"`

	// Language detected from file extension
	Language string `json:"language"`

	// Purpose describes the inferred purpose of the file
	Purpose string `json:"purpose"`

	// Exports are the exported symbols (functions, types, constants, etc.)
	Exports []Export `json:"exports,omitempty"`

	// Imports are the dependencies imported by this file
	Imports []string `json:"imports,omitempty"`

	// Interfaces detected in the file (Go interfaces, TypeScript interfaces, etc.)
	Interfaces []Interface `json:"interfaces,omitempty"`

	// Protocols detected (HTTP handlers, gRPC, WebSocket, etc.)
	Protocols []Protocol `json:"protocols,omitempty"`

	// LineCount is the total number of lines in the file
	LineCount int `json:"line_count"`

	// CodeLineCount is the number of non-empty, non-comment lines
	CodeLineCount int `json:"code_line_count"`

	// HasTests indicates whether the file contains test code
	HasTests bool `json:"has_tests"`

	// IsEntryPoint indicates whether this file is a main entry point
	IsEntryPoint bool `json:"is_entry_point"`

	// Metadata contains additional key-value pairs from analysis
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Export represents an exported symbol from a file.
type Export struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // function, type, const, var, class, method
	Line int    `json:"line"` // Line number where defined
}

// Interface represents a detected interface definition.
type Interface struct {
	Name    string   `json:"name"`
	Methods []string `json:"methods,omitempty"`
	Line    int      `json:"line"`
}

// Protocol represents a detected protocol or API pattern.
type Protocol struct {
	Type     string `json:"type"`     // http, grpc, websocket, jsonrpc, rest
	Endpoint string `json:"endpoint"` // Route pattern if available
	Method   string `json:"method"`   // HTTP method if applicable
	Line     int    `json:"line"`
}

// Analyzer performs content analysis on source files.
type Analyzer struct {
	// languagePatterns maps file extensions to language names
	languagePatterns map[string]string

	// purposePatterns maps filename patterns to purpose descriptions
	purposePatterns []purposePattern
}

type purposePattern struct {
	pattern *regexp.Regexp
	purpose string
}

// NewAnalyzer creates a new file analyzer with default patterns.
func NewAnalyzer() *Analyzer {
	a := &Analyzer{
		languagePatterns: defaultLanguagePatterns(),
		purposePatterns:  defaultPurposePatterns(),
	}
	return a
}

// AnalyzeFile analyzes a single file and extracts its purpose, exports, and protocols.
func (a *Analyzer) AnalyzeFile(filePath string, rootPath string) (*FileAnalysis, error) {
	// Get relative path
	relPath, err := filepath.Rel(rootPath, filePath)
	if err != nil {
		relPath = filePath
	}

	// Read file content
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(filePath))
	baseName := filepath.Base(filePath)

	analysis := &FileAnalysis{
		Path:     relPath,
		Language: a.detectLanguage(ext),
		Metadata: make(map[string]string),
	}

	// Scan file line by line
	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	analysis.LineCount = len(lines)
	analysis.Purpose = a.detectPurpose(baseName, relPath, lines)
	analysis.HasTests = a.detectTests(baseName, lines, analysis.Language)
	analysis.IsEntryPoint = a.detectEntryPoint(baseName, lines, analysis.Language)

	// Language-specific analysis
	switch analysis.Language {
	case "go":
		a.analyzeGo(lines, analysis)
	case "typescript", "javascript":
		a.analyzeTypeScript(lines, analysis)
	case "python":
		a.analyzePython(lines, analysis)
	default:
		a.analyzeGeneric(lines, analysis)
	}

	return analysis, nil
}

// detectLanguage maps file extension to language name.
func (a *Analyzer) detectLanguage(ext string) string {
	if lang, ok := a.languagePatterns[ext]; ok {
		return lang
	}
	return "unknown"
}

// detectPurpose infers the file's purpose from its name and content.
func (a *Analyzer) detectPurpose(baseName, relPath string, lines []string) string {
	lowerName := strings.ToLower(baseName)
	lowerPath := strings.ToLower(relPath)

	// Prioritize test file detection (check before other patterns)
	if strings.HasSuffix(lowerName, "_test.go") ||
		strings.HasSuffix(lowerName, ".test.ts") ||
		strings.HasSuffix(lowerName, ".test.js") ||
		strings.HasSuffix(lowerName, ".spec.ts") ||
		strings.HasSuffix(lowerName, ".spec.js") ||
		strings.HasSuffix(lowerName, "_test.py") ||
		strings.HasPrefix(lowerName, "test_") {
		return "test file"
	}

	// Check filename patterns
	for _, p := range a.purposePatterns {
		if p.pattern.MatchString(lowerName) || p.pattern.MatchString(lowerPath) {
			return p.purpose
		}
	}

	// Check content for clues
	if len(lines) > 0 {
		// Check first few lines for package/module declarations
		for i := 0; i < min(10, len(lines)); i++ {
			line := strings.TrimSpace(lines[i])
			if strings.HasPrefix(line, "// Package ") {
				// Go package doc comment
				return strings.TrimPrefix(line, "// Package ")
			}
			if strings.HasPrefix(line, "\"\"\"") || strings.HasPrefix(line, "'''") {
				// Python docstring might follow
				continue
			}
		}
	}

	return "source file"
}

// detectTests checks if the file contains test code.
func (a *Analyzer) detectTests(baseName string, lines []string, lang string) bool {
	lowerName := strings.ToLower(baseName)

	// Check filename patterns
	if strings.HasSuffix(lowerName, "_test.go") ||
		strings.HasSuffix(lowerName, ".test.ts") ||
		strings.HasSuffix(lowerName, ".test.js") ||
		strings.HasSuffix(lowerName, ".spec.ts") ||
		strings.HasSuffix(lowerName, ".spec.js") ||
		strings.HasSuffix(lowerName, "_test.py") ||
		strings.HasPrefix(lowerName, "test_") {
		return true
	}

	// Check content for test patterns
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch lang {
		case "go":
			if strings.HasPrefix(trimmed, "func Test") {
				return true
			}
		case "typescript", "javascript":
			if strings.Contains(trimmed, "describe(") ||
				strings.Contains(trimmed, "it(") ||
				strings.Contains(trimmed, "test(") {
				return true
			}
		case "python":
			if strings.HasPrefix(trimmed, "def test_") ||
				strings.Contains(trimmed, "@pytest") {
				return true
			}
		}
	}

	return false
}

// detectEntryPoint checks if the file is a main entry point.
func (a *Analyzer) detectEntryPoint(baseName string, lines []string, lang string) bool {
	lowerName := strings.ToLower(baseName)

	// Check filename patterns
	if lowerName == "main.go" ||
		lowerName == "main.py" ||
		lowerName == "index.ts" ||
		lowerName == "index.js" ||
		lowerName == "app.ts" ||
		lowerName == "app.js" {
		return true
	}

	// Check content
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch lang {
		case "go":
			if trimmed == "func main() {" || strings.HasPrefix(trimmed, "func main()") {
				return true
			}
		case "python":
			if strings.Contains(trimmed, "if __name__") && strings.Contains(trimmed, "__main__") {
				return true
			}
		}
	}

	return false
}

// analyzeGo performs Go-specific file analysis.
func (a *Analyzer) analyzeGo(lines []string, analysis *FileAnalysis) {
	var inImport bool
	var inInterface bool
	var currentInterface *Interface

	// Match exported declarations: func, type, const, var
	exportPattern := regexp.MustCompile(`^(func|type|const|var)\s+([A-Z][a-zA-Z0-9_]*)`)
	// Match interface definitions (captures both single-line and multi-line)
	interfacePattern := regexp.MustCompile(`^type\s+([A-Z][a-zA-Z0-9_]*)\s+interface\s*\{?`)
	methodPattern := regexp.MustCompile(`^\s*([A-Z][a-zA-Z0-9_]*)\s*\(`)
	httpHandlerPattern := regexp.MustCompile(`\.(Get|Post|Put|Delete|Patch|Handle|HandleFunc)\s*\(\s*["']([^"']+)["']`)
	grpcPattern := regexp.MustCompile(`(Register\w+Server|pb\.\w+Server)`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineNum := i + 1

		// Track imports
		if strings.HasPrefix(trimmed, "import (") {
			inImport = true
			continue
		}
		if inImport {
			if trimmed == ")" {
				inImport = false
			} else if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
				imp := strings.Trim(trimmed, `"`)
				imp = strings.TrimSpace(strings.Split(imp, " ")[len(strings.Split(imp, " "))-1])
				imp = strings.Trim(imp, `"`)
				if imp != "" {
					analysis.Imports = append(analysis.Imports, imp)
				}
			}
			continue
		}
		if strings.HasPrefix(trimmed, "import ") && !strings.Contains(trimmed, "(") {
			parts := strings.Split(trimmed, `"`)
			if len(parts) >= 2 {
				analysis.Imports = append(analysis.Imports, parts[1])
			}
			continue
		}

		// Track interfaces
		if matches := interfacePattern.FindStringSubmatch(trimmed); matches != nil {
			inInterface = true
			currentInterface = &Interface{
				Name: matches[1],
				Line: lineNum,
			}
			continue
		}
		if inInterface {
			if trimmed == "}" {
				analysis.Interfaces = append(analysis.Interfaces, *currentInterface)
				inInterface = false
				currentInterface = nil
			} else if matches := methodPattern.FindStringSubmatch(trimmed); matches != nil {
				currentInterface.Methods = append(currentInterface.Methods, matches[1])
			}
			continue
		}

		// Detect exports (capitalized names)
		if matches := exportPattern.FindStringSubmatch(trimmed); matches != nil {
			export := Export{
				Kind: matches[1],
				Name: matches[2],
				Line: lineNum,
			}
			analysis.Exports = append(analysis.Exports, export)
		}

		// Detect HTTP handlers
		if matches := httpHandlerPattern.FindStringSubmatch(line); matches != nil {
			protocol := Protocol{
				Type:     "http",
				Method:   strings.ToUpper(matches[1]),
				Endpoint: matches[2],
				Line:     lineNum,
			}
			if strings.HasSuffix(matches[1], "Func") || matches[1] == "Handle" {
				protocol.Method = "ANY"
			}
			analysis.Protocols = append(analysis.Protocols, protocol)
		}

		// Detect gRPC
		if grpcPattern.MatchString(line) {
			analysis.Protocols = append(analysis.Protocols, Protocol{
				Type: "grpc",
				Line: lineNum,
			})
		}

		// Count code lines (non-empty, non-comment)
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			analysis.CodeLineCount++
		}
	}
}

// analyzeTypeScript performs TypeScript/JavaScript-specific file analysis.
func (a *Analyzer) analyzeTypeScript(lines []string, analysis *FileAnalysis) {
	exportPattern := regexp.MustCompile(`^export\s+(const|let|var|function|class|interface|type|enum)\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	defaultExportPattern := regexp.MustCompile(`^export\s+default\s+(function|class)?\s*([a-zA-Z_][a-zA-Z0-9_]*)?`)
	importPattern := regexp.MustCompile(`^import\s+.*from\s+['"]([^'"]+)['"]`)
	interfacePattern := regexp.MustCompile(`^(export\s+)?interface\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*(\{|extends)`)
	httpPattern := regexp.MustCompile(`\.(get|post|put|delete|patch)\s*\(\s*['"]([^'"]+)['"]`)
	wsPattern := regexp.MustCompile(`new\s+WebSocket\s*\(|\.on\s*\(\s*['"]message['"]`)

	var inInterface bool
	var currentInterface *Interface

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineNum := i + 1

		// Track imports
		if matches := importPattern.FindStringSubmatch(trimmed); matches != nil {
			analysis.Imports = append(analysis.Imports, matches[1])
		}

		// Track interfaces
		if matches := interfacePattern.FindStringSubmatch(trimmed); matches != nil {
			inInterface = true
			currentInterface = &Interface{
				Name: matches[2],
				Line: lineNum,
			}
			if strings.HasSuffix(trimmed, "{") && !strings.Contains(trimmed, "}") {
				continue
			}
		}
		if inInterface && strings.Contains(trimmed, "}") {
			if currentInterface != nil {
				analysis.Interfaces = append(analysis.Interfaces, *currentInterface)
			}
			inInterface = false
			currentInterface = nil
		}

		// Detect exports
		if matches := exportPattern.FindStringSubmatch(trimmed); matches != nil {
			analysis.Exports = append(analysis.Exports, Export{
				Kind: matches[1],
				Name: matches[2],
				Line: lineNum,
			})
		}
		if matches := defaultExportPattern.FindStringSubmatch(trimmed); matches != nil {
			name := matches[2]
			if name == "" {
				name = "default"
			}
			kind := matches[1]
			if kind == "" {
				kind = "default"
			}
			analysis.Exports = append(analysis.Exports, Export{
				Kind: kind,
				Name: name,
				Line: lineNum,
			})
		}

		// Detect HTTP endpoints
		if matches := httpPattern.FindStringSubmatch(strings.ToLower(line)); matches != nil {
			analysis.Protocols = append(analysis.Protocols, Protocol{
				Type:     "http",
				Method:   strings.ToUpper(matches[1]),
				Endpoint: matches[2],
				Line:     lineNum,
			})
		}

		// Detect WebSocket
		if wsPattern.MatchString(line) {
			analysis.Protocols = append(analysis.Protocols, Protocol{
				Type: "websocket",
				Line: lineNum,
			})
		}

		// Count code lines
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "/*") && !strings.HasPrefix(trimmed, "*") {
			analysis.CodeLineCount++
		}
	}
}

// analyzePython performs Python-specific file analysis.
func (a *Analyzer) analyzePython(lines []string, analysis *FileAnalysis) {
	funcPattern := regexp.MustCompile(`^def\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
	classPattern := regexp.MustCompile(`^class\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\(]`)
	importPattern := regexp.MustCompile(`^(?:from\s+([a-zA-Z_][a-zA-Z0-9_.]*)\s+)?import\s+`)
	decoratorPattern := regexp.MustCompile(`^@(app|router)\.(get|post|put|delete|patch|route)\s*\(\s*['"]([^'"]+)['"]`)
	protocolClassPattern := regexp.MustCompile(`class\s+([a-zA-Z_]+)\s*\(\s*Protocol\s*\)`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineNum := i + 1

		// Track imports
		if importPattern.MatchString(trimmed) {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				if parts[0] == "from" && len(parts) >= 4 {
					analysis.Imports = append(analysis.Imports, parts[1])
				} else if parts[0] == "import" {
					analysis.Imports = append(analysis.Imports, parts[1])
				}
			}
		}

		// Track functions (exported = doesn't start with _)
		if matches := funcPattern.FindStringSubmatch(trimmed); matches != nil {
			if !strings.HasPrefix(matches[1], "_") {
				analysis.Exports = append(analysis.Exports, Export{
					Kind: "function",
					Name: matches[1],
					Line: lineNum,
				})
			}
		}

		// Track classes
		if matches := classPattern.FindStringSubmatch(trimmed); matches != nil {
			if !strings.HasPrefix(matches[1], "_") {
				analysis.Exports = append(analysis.Exports, Export{
					Kind: "class",
					Name: matches[1],
					Line: lineNum,
				})
			}
		}

		// Detect Protocol classes (typing.Protocol)
		if matches := protocolClassPattern.FindStringSubmatch(trimmed); matches != nil {
			analysis.Interfaces = append(analysis.Interfaces, Interface{
				Name: matches[1],
				Line: lineNum,
			})
		}

		// Detect HTTP endpoints (FastAPI, Flask decorators)
		if matches := decoratorPattern.FindStringSubmatch(trimmed); matches != nil {
			analysis.Protocols = append(analysis.Protocols, Protocol{
				Type:     "http",
				Method:   strings.ToUpper(matches[2]),
				Endpoint: matches[3],
				Line:     lineNum,
			})
		}

		// Count code lines
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			analysis.CodeLineCount++
		}
	}
}

// analyzeGeneric performs basic analysis for unknown languages.
func (a *Analyzer) analyzeGeneric(lines []string, analysis *FileAnalysis) {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			analysis.CodeLineCount++
		}
	}
}

// defaultLanguagePatterns returns the default extension-to-language mappings.
func defaultLanguagePatterns() map[string]string {
	return map[string]string{
		".go":    "go",
		".ts":    "typescript",
		".tsx":   "typescript",
		".js":    "javascript",
		".jsx":   "javascript",
		".py":    "python",
		".rs":    "rust",
		".java":  "java",
		".kt":    "kotlin",
		".swift": "swift",
		".c":     "c",
		".cpp":   "cpp",
		".h":     "c",
		".hpp":   "cpp",
		".cs":    "csharp",
		".rb":    "ruby",
		".php":   "php",
		".sh":    "shell",
		".bash":  "shell",
		".zsh":   "shell",
		".sql":   "sql",
		".html":  "html",
		".css":   "css",
		".scss":  "scss",
		".sass":  "sass",
		".less":  "less",
		".json":  "json",
		".yaml":  "yaml",
		".yml":   "yaml",
		".xml":   "xml",
		".md":    "markdown",
		".proto": "protobuf",
		".vue":   "vue",
		".svelte": "svelte",
	}
}

// defaultPurposePatterns returns the default filename-to-purpose patterns.
func defaultPurposePatterns() []purposePattern {
	patterns := []struct {
		regex   string
		purpose string
	}{
		{`main\.(go|py|ts|js)$`, "application entry point"},
		{`index\.(ts|js|html)$`, "module entry point"},
		{`app\.(ts|js|go|py)$`, "application bootstrap"},
		{`server\.(go|ts|js|py)$`, "server implementation"},
		{`client\.(go|ts|js|py)$`, "client implementation"},
		{`handler`, "request handler"},
		{`controller`, "controller logic"},
		{`service`, "service layer"},
		{`repository`, "data access layer"},
		{`model`, "data model"},
		{`entity`, "domain entity"},
		{`util`, "utility functions"},
		{`helper`, "helper functions"},
		{`config`, "configuration"},
		{`middleware`, "middleware"},
		{`router`, "routing"},
		{`route`, "routing"},
		{`store`, "state management"},
		{`_test\.go$`, "Go test file"},
		{`\.test\.(ts|js)$`, "test file"},
		{`\.spec\.(ts|js)$`, "test specification"},
		{`test_.*\.py$`, "Python test file"},
		{`_test\.py$`, "Python test file"},
		{`readme`, "documentation"},
		{`license`, "license file"},
		{`dockerfile`, "container definition"},
		{`makefile`, "build automation"},
		{`\.proto$`, "protocol buffer definition"},
		{`\.graphql$`, "GraphQL schema"},
		{`schema`, "schema definition"},
		{`migration`, "database migration"},
		{`seed`, "database seed data"},
	}

	result := make([]purposePattern, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile("(?i)" + p.regex)
		if err != nil {
			continue
		}
		result = append(result, purposePattern{
			pattern: re,
			purpose: p.purpose,
		})
	}
	return result
}
