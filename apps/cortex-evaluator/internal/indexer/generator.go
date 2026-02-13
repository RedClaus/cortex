// Package indexer provides project directory scanning, file analysis, and context generation.
package indexer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Generator creates context files for memory compaction and recovery.
type Generator struct {
	analyzer *Analyzer
	indexer  *Indexer
}

// NewGenerator creates a new Generator with default options.
func NewGenerator() *Generator {
	return &Generator{
		analyzer: NewAnalyzer(),
		indexer:  New(DefaultOptions()),
	}
}

// NewGeneratorWithOptions creates a new Generator with custom indexer options.
func NewGeneratorWithOptions(opts Options) *Generator {
	return &Generator{
		analyzer: NewAnalyzer(),
		indexer:  New(opts),
	}
}

// ContextResult contains the generated context.md content and metadata.
type ContextResult struct {
	Content      string            `json:"content"`       // The markdown content
	ProjectName  string            `json:"project_name"`  // Detected project name
	Languages    []string          `json:"languages"`     // Detected languages
	EntryPoints  []string          `json:"entry_points"`  // Main entry points
	TotalFiles   int               `json:"total_files"`   // Total files analyzed
	Dependencies map[string]int    `json:"dependencies"`  // Dependency count by category
	GeneratedAt  time.Time         `json:"generated_at"`  // When context was generated
}

// TodoItem represents a TODO or FIXME found in the codebase.
type TodoItem struct {
	File     string `json:"file"`     // Relative path to file
	Line     int    `json:"line"`     // Line number
	Type     string `json:"type"`     // TODO, FIXME, HACK, XXX, etc.
	Text     string `json:"text"`     // The todo text content
	Priority string `json:"priority"` // high, medium, low (inferred)
}

// TodoResult contains the generated todos.md content and extracted items.
type TodoResult struct {
	Content     string     `json:"content"`      // The markdown content
	Items       []TodoItem `json:"items"`        // All extracted todo items
	TotalCount  int        `json:"total_count"`  // Total todos found
	ByType      map[string]int `json:"by_type"`  // Count by type (TODO, FIXME, etc.)
	GeneratedAt time.Time  `json:"generated_at"` // When todos were extracted
}

// InsightItem represents a learning or pattern discovered in the codebase.
type InsightItem struct {
	Category    string   `json:"category"`    // architecture, patterns, conventions, etc.
	Title       string   `json:"title"`       // Short title
	Description string   `json:"description"` // Detailed description
	Evidence    []string `json:"evidence"`    // File paths or code snippets as evidence
}

// InsightResult contains the generated insights.md content and items.
type InsightResult struct {
	Content     string        `json:"content"`      // The markdown content
	Items       []InsightItem `json:"items"`        // All generated insights
	GeneratedAt time.Time     `json:"generated_at"` // When insights were generated
}

// GenerateContext analyzes the project at projectPath and generates context.md content.
// It creates a project overview including structure, languages, entry points, and dependencies.
func (g *Generator) GenerateContext(projectPath string) (*ContextResult, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	// Index the project
	manifest, err := g.indexer.IndexProject(absPath)
	if err != nil {
		return nil, fmt.Errorf("indexing project: %w", err)
	}

	// Analyze files and collect insights
	languages := make(map[string]int)
	entryPoints := make([]string, 0)
	allExports := make([]Export, 0)
	allInterfaces := make([]Interface, 0)
	allProtocols := make([]Protocol, 0)
	imports := make(map[string]int)

	for _, file := range manifest.Files {
		if file.Type != FileTypeRegular {
			continue
		}

		// Only analyze source files
		ext := strings.ToLower(file.Extension)
		if !isSourceFile(ext) {
			continue
		}

		analysis, err := g.analyzer.AnalyzeFile(file.AbsolutePath, absPath)
		if err != nil {
			continue // Skip files that can't be analyzed
		}

		// Count languages
		if analysis.Language != "unknown" {
			languages[analysis.Language]++
		}

		// Collect entry points
		if analysis.IsEntryPoint {
			entryPoints = append(entryPoints, analysis.Path)
		}

		// Collect exports, interfaces, protocols
		allExports = append(allExports, analysis.Exports...)
		allInterfaces = append(allInterfaces, analysis.Interfaces...)
		allProtocols = append(allProtocols, analysis.Protocols...)

		// Count imports/dependencies
		for _, imp := range analysis.Imports {
			imports[imp]++
		}
	}

	// Detect project name
	projectName := filepath.Base(absPath)

	// Sort languages by count
	langList := make([]string, 0, len(languages))
	for lang := range languages {
		langList = append(langList, lang)
	}
	sort.Slice(langList, func(i, j int) bool {
		return languages[langList[i]] > languages[langList[j]]
	})

	// Generate markdown content
	var sb strings.Builder
	sb.WriteString("# Project Context: ")
	sb.WriteString(projectName)
	sb.WriteString("\n\n")
	sb.WriteString("*Generated: ")
	sb.WriteString(time.Now().Format(time.RFC3339))
	sb.WriteString("*\n\n")

	// Overview section
	sb.WriteString("## Overview\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Files**: %d\n", manifest.TotalFiles))
	sb.WriteString(fmt.Sprintf("- **Total Directories**: %d\n", manifest.TotalDirs))
	sb.WriteString(fmt.Sprintf("- **Total Size**: %s\n", formatBytes(manifest.TotalSize)))
	sb.WriteString("\n")

	// Languages section
	if len(langList) > 0 {
		sb.WriteString("## Languages\n\n")
		for _, lang := range langList {
			sb.WriteString(fmt.Sprintf("- **%s**: %d files\n", lang, languages[lang]))
		}
		sb.WriteString("\n")
	}

	// Entry points section
	if len(entryPoints) > 0 {
		sb.WriteString("## Entry Points\n\n")
		for _, ep := range entryPoints {
			sb.WriteString(fmt.Sprintf("- `%s`\n", ep))
		}
		sb.WriteString("\n")
	}

	// Key interfaces section
	if len(allInterfaces) > 0 {
		sb.WriteString("## Key Interfaces\n\n")
		// Limit to top 10 interfaces
		count := len(allInterfaces)
		if count > 10 {
			count = 10
		}
		for i := 0; i < count; i++ {
			iface := allInterfaces[i]
			sb.WriteString(fmt.Sprintf("- **%s**", iface.Name))
			if len(iface.Methods) > 0 {
				sb.WriteString(fmt.Sprintf(" (%d methods)", len(iface.Methods)))
			}
			sb.WriteString("\n")
		}
		if len(allInterfaces) > 10 {
			sb.WriteString(fmt.Sprintf("- *...and %d more*\n", len(allInterfaces)-10))
		}
		sb.WriteString("\n")
	}

	// Protocols/APIs section
	if len(allProtocols) > 0 {
		sb.WriteString("## Detected APIs\n\n")
		protocolTypes := make(map[string][]Protocol)
		for _, p := range allProtocols {
			protocolTypes[p.Type] = append(protocolTypes[p.Type], p)
		}
		for pType, protocols := range protocolTypes {
			sb.WriteString(fmt.Sprintf("### %s\n\n", strings.ToUpper(pType)))
			// Limit to top 5 per type
			count := len(protocols)
			if count > 5 {
				count = 5
			}
			for i := 0; i < count; i++ {
				p := protocols[i]
				if p.Endpoint != "" {
					sb.WriteString(fmt.Sprintf("- `%s %s`\n", p.Method, p.Endpoint))
				} else {
					sb.WriteString(fmt.Sprintf("- %s endpoint (line %d)\n", p.Type, p.Line))
				}
			}
			if len(protocols) > 5 {
				sb.WriteString(fmt.Sprintf("- *...and %d more*\n", len(protocols)-5))
			}
			sb.WriteString("\n")
		}
	}

	// Key exports section
	if len(allExports) > 0 {
		sb.WriteString("## Key Exports\n\n")
		// Group by kind
		exportsByKind := make(map[string][]Export)
		for _, exp := range allExports {
			exportsByKind[exp.Kind] = append(exportsByKind[exp.Kind], exp)
		}
		for kind, exports := range exportsByKind {
			sb.WriteString(fmt.Sprintf("### %s\n\n", strings.Title(kind)))
			// Limit to top 10 per kind
			count := len(exports)
			if count > 10 {
				count = 10
			}
			for i := 0; i < count; i++ {
				sb.WriteString(fmt.Sprintf("- `%s`\n", exports[i].Name))
			}
			if len(exports) > 10 {
				sb.WriteString(fmt.Sprintf("- *...and %d more*\n", len(exports)-10))
			}
			sb.WriteString("\n")
		}
	}

	// Dependencies section
	if len(imports) > 0 {
		sb.WriteString("## Dependencies\n\n")
		// Categorize dependencies
		depCategories := categorizeDependencies(imports)
		for category, deps := range depCategories {
			if len(deps) == 0 {
				continue
			}
			sb.WriteString(fmt.Sprintf("### %s\n\n", category))
			// Sort by frequency
			sort.Slice(deps, func(i, j int) bool {
				return imports[deps[i]] > imports[deps[j]]
			})
			// Limit to top 10 per category
			count := len(deps)
			if count > 10 {
				count = 10
			}
			for i := 0; i < count; i++ {
				sb.WriteString(fmt.Sprintf("- `%s` (%d references)\n", deps[i], imports[deps[i]]))
			}
			if len(deps) > 10 {
				sb.WriteString(fmt.Sprintf("- *...and %d more*\n", len(deps)-10))
			}
			sb.WriteString("\n")
		}
	}

	result := &ContextResult{
		Content:      sb.String(),
		ProjectName:  projectName,
		Languages:    langList,
		EntryPoints:  entryPoints,
		TotalFiles:   manifest.TotalFiles,
		Dependencies: make(map[string]int),
		GeneratedAt:  time.Now(),
	}

	// Count dependencies by category
	depCats := categorizeDependencies(imports)
	for cat, deps := range depCats {
		result.Dependencies[cat] = len(deps)
	}

	return result, nil
}

// GenerateTodos scans the project for TODO, FIXME, HACK, and XXX comments.
func (g *Generator) GenerateTodos(projectPath string) (*TodoResult, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	// Index the project
	manifest, err := g.indexer.IndexProject(absPath)
	if err != nil {
		return nil, fmt.Errorf("indexing project: %w", err)
	}

	// Regular expressions for todo patterns
	todoPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(TODO)\s*[:\-]?\s*(.+)`),
		regexp.MustCompile(`(?i)\b(FIXME)\s*[:\-]?\s*(.+)`),
		regexp.MustCompile(`(?i)\b(HACK)\s*[:\-]?\s*(.+)`),
		regexp.MustCompile(`(?i)\b(XXX)\s*[:\-]?\s*(.+)`),
		regexp.MustCompile(`(?i)\b(BUG)\s*[:\-]?\s*(.+)`),
		regexp.MustCompile(`(?i)\b(NOTE)\s*[:\-]?\s*(.+)`),
	}

	items := make([]TodoItem, 0)
	byType := make(map[string]int)

	for _, file := range manifest.Files {
		if file.Type != FileTypeRegular {
			continue
		}

		// Only scan source files and markdown
		ext := strings.ToLower(file.Extension)
		if !isSourceFile(ext) && ext != ".md" {
			continue
		}

		// Read and scan file
		f, err := os.Open(file.AbsolutePath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			for _, pattern := range todoPatterns {
				if matches := pattern.FindStringSubmatch(line); matches != nil {
					todoType := strings.ToUpper(matches[1])
					text := strings.TrimSpace(matches[2])

					// Clean up the text (remove trailing comment markers)
					text = strings.TrimSuffix(text, "*/")
					text = strings.TrimSuffix(text, "-->")
					text = strings.TrimSpace(text)

					item := TodoItem{
						File:     file.Path,
						Line:     lineNum,
						Type:     todoType,
						Text:     text,
						Priority: inferPriority(todoType, text),
					}

					items = append(items, item)
					byType[todoType]++
					break // Only match first pattern per line
				}
			}
		}
		f.Close()
	}

	// Sort items: high priority first, then by file and line
	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return priorityRank(items[i].Priority) < priorityRank(items[j].Priority)
		}
		if items[i].File != items[j].File {
			return items[i].File < items[j].File
		}
		return items[i].Line < items[j].Line
	})

	// Generate markdown content
	var sb strings.Builder
	sb.WriteString("# Code TODOs\n\n")
	sb.WriteString("*Generated: ")
	sb.WriteString(time.Now().Format(time.RFC3339))
	sb.WriteString("*\n\n")

	// Summary
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("Total items: **%d**\n\n", len(items)))
	if len(byType) > 0 {
		sb.WriteString("| Type | Count |\n")
		sb.WriteString("|------|-------|\n")
		// Sort types for consistent output
		types := make([]string, 0, len(byType))
		for t := range byType {
			types = append(types, t)
		}
		sort.Strings(types)
		for _, t := range types {
			sb.WriteString(fmt.Sprintf("| %s | %d |\n", t, byType[t]))
		}
		sb.WriteString("\n")
	}

	// Items by priority
	if len(items) > 0 {
		// Group by priority
		byPriority := make(map[string][]TodoItem)
		for _, item := range items {
			byPriority[item.Priority] = append(byPriority[item.Priority], item)
		}

		priorities := []string{"high", "medium", "low"}
		for _, priority := range priorities {
			prItems := byPriority[priority]
			if len(prItems) == 0 {
				continue
			}

			sb.WriteString(fmt.Sprintf("## %s Priority\n\n", strings.Title(priority)))
			for _, item := range prItems {
				sb.WriteString(fmt.Sprintf("- **[%s]** `%s:%d` - %s\n", item.Type, item.File, item.Line, item.Text))
			}
			sb.WriteString("\n")
		}
	}

	return &TodoResult{
		Content:     sb.String(),
		Items:       items,
		TotalCount:  len(items),
		ByType:      byType,
		GeneratedAt: time.Now(),
	}, nil
}

// GenerateInsights analyzes the project and generates insights about patterns and architecture.
func (g *Generator) GenerateInsights(projectPath string) (*InsightResult, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	// Index the project
	manifest, err := g.indexer.IndexProject(absPath)
	if err != nil {
		return nil, fmt.Errorf("indexing project: %w", err)
	}

	insights := make([]InsightItem, 0)

	// Analyze project structure
	dirStructure := analyzeDirStructure(manifest)
	if len(dirStructure) > 0 {
		insights = append(insights, InsightItem{
			Category:    "architecture",
			Title:       "Project Structure",
			Description: "The project follows a structured directory layout.",
			Evidence:    dirStructure,
		})
	}

	// Analyze languages and their distribution
	languages := make(map[string][]string)
	testFiles := make([]string, 0)
	configFiles := make([]string, 0)

	for _, file := range manifest.Files {
		if file.Type != FileTypeRegular {
			continue
		}

		ext := strings.ToLower(file.Extension)
		if !isSourceFile(ext) {
			// Check for config files
			if isConfigFile(filepath.Base(file.Path)) {
				configFiles = append(configFiles, file.Path)
			}
			continue
		}

		analysis, err := g.analyzer.AnalyzeFile(file.AbsolutePath, absPath)
		if err != nil {
			continue
		}

		if analysis.Language != "unknown" {
			languages[analysis.Language] = append(languages[analysis.Language], file.Path)
		}

		if analysis.HasTests {
			testFiles = append(testFiles, file.Path)
		}
	}

	// Language insights
	if len(languages) > 1 {
		langNames := make([]string, 0, len(languages))
		for lang := range languages {
			langNames = append(langNames, lang)
		}
		insights = append(insights, InsightItem{
			Category:    "conventions",
			Title:       "Multi-language Codebase",
			Description: fmt.Sprintf("The project uses %d languages: %s", len(languages), strings.Join(langNames, ", ")),
			Evidence:    langNames,
		})
	} else if len(languages) == 1 {
		for lang, files := range languages {
			insights = append(insights, InsightItem{
				Category:    "conventions",
				Title:       "Single Language Codebase",
				Description: fmt.Sprintf("The project is primarily written in %s with %d source files.", lang, len(files)),
				Evidence:    []string{lang},
			})
		}
	}

	// Testing insights
	if len(testFiles) > 0 {
		testRatio := float64(len(testFiles)) / float64(manifest.TotalFiles) * 100
		insights = append(insights, InsightItem{
			Category:    "patterns",
			Title:       "Test Coverage Present",
			Description: fmt.Sprintf("Found %d test files (%.1f%% of source files).", len(testFiles), testRatio),
			Evidence:    truncateList(testFiles, 5),
		})
	} else {
		insights = append(insights, InsightItem{
			Category:    "patterns",
			Title:       "No Tests Detected",
			Description: "No test files were found in the project. Consider adding tests.",
			Evidence:    []string{},
		})
	}

	// Configuration insights
	if len(configFiles) > 0 {
		insights = append(insights, InsightItem{
			Category:    "conventions",
			Title:       "Configuration Files",
			Description: fmt.Sprintf("Found %d configuration files.", len(configFiles)),
			Evidence:    truncateList(configFiles, 5),
		})
	}

	// Detect common patterns
	patterns := detectPatterns(manifest, absPath, g.analyzer)
	insights = append(insights, patterns...)

	// Generate markdown content
	var sb strings.Builder
	sb.WriteString("# Project Insights\n\n")
	sb.WriteString("*Generated: ")
	sb.WriteString(time.Now().Format(time.RFC3339))
	sb.WriteString("*\n\n")

	// Group insights by category
	byCategory := make(map[string][]InsightItem)
	for _, insight := range insights {
		byCategory[insight.Category] = append(byCategory[insight.Category], insight)
	}

	// Output in preferred order
	categories := []string{"architecture", "patterns", "conventions"}
	for _, category := range categories {
		items := byCategory[category]
		if len(items) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s\n\n", strings.Title(category)))
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("### %s\n\n", item.Title))
			sb.WriteString(item.Description)
			sb.WriteString("\n\n")
			if len(item.Evidence) > 0 {
				sb.WriteString("**Evidence:**\n")
				for _, e := range item.Evidence {
					sb.WriteString(fmt.Sprintf("- `%s`\n", e))
				}
				sb.WriteString("\n")
			}
		}
	}

	return &InsightResult{
		Content:     sb.String(),
		Items:       insights,
		GeneratedAt: time.Now(),
	}, nil
}

// SaveToSession saves the generated content to the specified session folder.
func (g *Generator) SaveToSession(sessionPath string, context *ContextResult, todos *TodoResult, insights *InsightResult) error {
	// Create session directory if it doesn't exist
	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		return fmt.Errorf("creating session directory: %w", err)
	}

	// Save context.md
	if context != nil {
		contextPath := filepath.Join(sessionPath, "context.md")
		if err := os.WriteFile(contextPath, []byte(context.Content), 0644); err != nil {
			return fmt.Errorf("writing context.md: %w", err)
		}
	}

	// Save todos.md
	if todos != nil {
		todosPath := filepath.Join(sessionPath, "todos.md")
		if err := os.WriteFile(todosPath, []byte(todos.Content), 0644); err != nil {
			return fmt.Errorf("writing todos.md: %w", err)
		}
	}

	// Save insights.md
	if insights != nil {
		insightsPath := filepath.Join(sessionPath, "insights.md")
		if err := os.WriteFile(insightsPath, []byte(insights.Content), 0644); err != nil {
			return fmt.Errorf("writing insights.md: %w", err)
		}
	}

	return nil
}

// Helper functions

func isSourceFile(ext string) bool {
	sourceExts := map[string]bool{
		".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".py": true, ".rs": true, ".java": true, ".kt": true, ".swift": true,
		".c": true, ".cpp": true, ".h": true, ".hpp": true, ".cs": true,
		".rb": true, ".php": true, ".sh": true, ".bash": true,
	}
	return sourceExts[ext]
}

func isConfigFile(name string) bool {
	lowerName := strings.ToLower(name)
	configPatterns := []string{
		"config", "settings", ".env", "yaml", "yml", "json", "toml",
		"dockerfile", "makefile", "gemfile", "podfile", "package.json",
		"go.mod", "cargo.toml", "requirements.txt", "pyproject.toml",
	}
	for _, pattern := range configPatterns {
		if strings.Contains(lowerName, pattern) {
			return true
		}
	}
	return false
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func categorizeDependencies(imports map[string]int) map[string][]string {
	categories := map[string][]string{
		"Standard Library": {},
		"External":         {},
		"Internal":         {},
	}

	for imp := range imports {
		if strings.HasPrefix(imp, ".") || strings.HasPrefix(imp, "@/") {
			categories["Internal"] = append(categories["Internal"], imp)
		} else if !strings.Contains(imp, "/") && !strings.Contains(imp, ".") {
			categories["Standard Library"] = append(categories["Standard Library"], imp)
		} else {
			categories["External"] = append(categories["External"], imp)
		}
	}

	return categories
}

func inferPriority(todoType, text string) string {
	lowerText := strings.ToLower(text)

	// FIXME and BUG are high priority
	if todoType == "FIXME" || todoType == "BUG" {
		return "high"
	}

	// Check for urgency keywords
	highKeywords := []string{"urgent", "critical", "asap", "important", "security", "crash", "broken"}
	for _, kw := range highKeywords {
		if strings.Contains(lowerText, kw) {
			return "high"
		}
	}

	// HACK and XXX are medium priority
	if todoType == "HACK" || todoType == "XXX" {
		return "medium"
	}

	// NOTE is low priority
	if todoType == "NOTE" {
		return "low"
	}

	// Default TODO to medium
	return "medium"
}

func priorityRank(priority string) int {
	switch priority {
	case "high":
		return 0
	case "medium":
		return 1
	case "low":
		return 2
	default:
		return 3
	}
}

func analyzeDirStructure(manifest *Manifest) []string {
	dirs := make(map[string]bool)
	for _, file := range manifest.Files {
		if file.Type == FileTypeDirectory {
			dirs[file.Path] = true
		}
	}

	// Detect common patterns
	patterns := make([]string, 0)
	knownDirs := []struct {
		name    string
		pattern string
	}{
		{"cmd", "Command/CLI entry points"},
		{"internal", "Private packages (Go convention)"},
		{"pkg", "Public packages"},
		{"src", "Source code"},
		{"lib", "Library code"},
		{"api", "API definitions"},
		{"test", "Test files"},
		{"tests", "Test files"},
		{"docs", "Documentation"},
		{"config", "Configuration"},
		{"scripts", "Build/deployment scripts"},
		{"frontend", "Frontend code"},
		{"backend", "Backend code"},
	}

	for _, kd := range knownDirs {
		if dirs[kd.name] {
			patterns = append(patterns, fmt.Sprintf("%s/ - %s", kd.name, kd.pattern))
		}
	}

	return patterns
}

func detectPatterns(manifest *Manifest, rootPath string, analyzer *Analyzer) []InsightItem {
	insights := make([]InsightItem, 0)

	// Detect MVC/Clean Architecture patterns
	hasHandlers := false
	hasServices := false
	hasRepositories := false
	hasModels := false

	for _, file := range manifest.Files {
		if file.Type != FileTypeRegular {
			continue
		}
		lowerPath := strings.ToLower(file.Path)
		if strings.Contains(lowerPath, "handler") || strings.Contains(lowerPath, "controller") {
			hasHandlers = true
		}
		if strings.Contains(lowerPath, "service") {
			hasServices = true
		}
		if strings.Contains(lowerPath, "repository") || strings.Contains(lowerPath, "repo") {
			hasRepositories = true
		}
		if strings.Contains(lowerPath, "model") || strings.Contains(lowerPath, "entity") {
			hasModels = true
		}
	}

	if hasHandlers && hasServices && hasRepositories {
		insights = append(insights, InsightItem{
			Category:    "patterns",
			Title:       "Layered Architecture",
			Description: "The project follows a layered architecture pattern with handlers/controllers, services, and repositories.",
			Evidence:    []string{"handlers/controllers", "services", "repositories"},
		})
	}

	if hasModels {
		insights = append(insights, InsightItem{
			Category:    "patterns",
			Title:       "Domain Models",
			Description: "The project uses domain models/entities for data representation.",
			Evidence:    []string{"models or entities directory"},
		})
	}

	return insights
}

func truncateList(items []string, max int) []string {
	if len(items) <= max {
		return items
	}
	result := make([]string, max+1)
	copy(result, items[:max])
	result[max] = fmt.Sprintf("...and %d more", len(items)-max)
	return result
}
