package parsers

import (
	"bufio"
	"path/filepath"
	"strings"
)

// CodeParser parses source code files.
type CodeParser struct{}

// NewCodeParser creates a new code parser.
func NewCodeParser() *CodeParser {
	return &CodeParser{}
}

// Parse parses code content into a structured document.
// Treats the entire file as a single code block.
func (p *CodeParser) Parse(content string) (*ParsedDocument, error) {
	language := detectLanguage(content)

	doc := &ParsedDocument{
		Title: "Code Document",
		Metadata: Metadata{
			Language: language,
		},
		Sections: []*Section{
			{
				Level:   1,
				Title:   "Source Code",
				Content: "", // No text content for pure code files
				CodeBlocks: []*CodeBlock{
					{
						Language: language,
						Code:     strings.TrimSpace(content),
						Line:     1,
					},
				},
				Subsections: make([]*Section, 0),
				StartLine:   1,
				EndLine:     countLinesInCode(content),
			},
		},
	}

	return doc, nil
}

// ParseWithPath parses code with file path for language detection.
func (p *CodeParser) ParseWithPath(content string, path string) (*ParsedDocument, error) {
	language := detectLanguageFromPath(path)
	if language == "" {
		language = detectLanguage(content)
	}

	filename := filepath.Base(path)

	doc := &ParsedDocument{
		Title: filename,
		Metadata: Metadata{
			Language: language,
		},
		Sections: []*Section{
			{
				Level:   1,
				Title:   filename,
				Content: "", // No text content for pure code files
				CodeBlocks: []*CodeBlock{
					{
						Language: language,
						Code:     strings.TrimSpace(content),
						Line:     1,
					},
				},
				Subsections: make([]*Section, 0),
				StartLine:   1,
				EndLine:     countLinesInCode(content),
			},
		},
	}

	return doc, nil
}

// Format returns the format identifier.
func (p *CodeParser) Format() string {
	return "code"
}

// detectLanguageFromPath detects language from file extension.
func detectLanguageFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	langMap := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".jsx":  "javascript",
		".tsx":  "typescript",
		".java": "java",
		".c":    "c",
		".cpp":  "cpp",
		".h":    "c",
		".hpp":  "cpp",
		".rs":   "rust",
		".rb":   "ruby",
		".php":  "php",
		".sh":   "bash",
		".bash": "bash",
		".zsh":  "zsh",
		".fish": "fish",
		".sql":  "sql",
		".yaml": "yaml",
		".yml":  "yaml",
		".json": "json",
		".xml":  "xml",
		".html": "html",
		".css":  "css",
		".scss": "scss",
		".sass": "sass",
		".vim":  "vim",
		".lua":  "lua",
		".r":    "r",
		".swift": "swift",
		".kt":   "kotlin",
		".scala": "scala",
		".clj":  "clojure",
		".ex":   "elixir",
		".exs":  "elixir",
		".erl":  "erlang",
		".hs":   "haskell",
		".ml":   "ocaml",
		".v":    "verilog",
		".vhdl": "vhdl",
	}

	if lang, ok := langMap[ext]; ok {
		return lang
	}

	return "text"
}

// detectLanguage attempts to detect language from content.
func detectLanguage(content string) string {
	// Look for shebangs
	if strings.HasPrefix(content, "#!/") {
		firstLine := strings.Split(content, "\n")[0]
		if strings.Contains(firstLine, "python") {
			return "python"
		}
		if strings.Contains(firstLine, "bash") || strings.Contains(firstLine, "sh") {
			return "bash"
		}
		if strings.Contains(firstLine, "node") {
			return "javascript"
		}
		if strings.Contains(firstLine, "ruby") {
			return "ruby"
		}
	}

	// Look for common language patterns
	patterns := []struct {
		pattern  string
		language string
	}{
		{"package main", "go"},
		{"func main()", "go"},
		{"import (", "go"},
		{"def __init__", "python"},
		{"import numpy", "python"},
		{"from typing import", "python"},
		{"function ", "javascript"},
		{"const ", "javascript"},
		{"export default", "javascript"},
		{"public class ", "java"},
		{"public static void main", "java"},
		{"fn main()", "rust"},
		{"use std::", "rust"},
		{"<?php", "php"},
		{"SELECT * FROM", "sql"},
		{"CREATE TABLE", "sql"},
	}

	for _, p := range patterns {
		if strings.Contains(content, p.pattern) {
			return p.language
		}
	}

	return "text"
}

// countLinesInCode counts the number of lines in code.
func countLinesInCode(content string) int {
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		count++
	}
	return count
}
