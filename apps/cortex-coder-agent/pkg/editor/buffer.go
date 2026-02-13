// Package editor provides text buffer management for the editor
package editor

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// Edit represents a single edit operation
type Edit struct {
	Type      EditType
	Line      int
	Column    int
	OldText   string
	NewText   string
	OldLines  []string
	NewLines  []string
}

// EditType represents the type of edit
type EditType int

const (
	EditTypeInsert EditType = iota
	EditTypeDelete
	EditTypeReplace
	EditTypeInsertLine
	EditTypeDeleteLine
)

// Buffer represents a text buffer with undo/redo support
type Buffer struct {
	filename    string
	lines       []string
	undoStack   []Edit
	redoStack   []Edit
	dirty       bool
	maxSize     int64 // Maximum file size in bytes
	cursorLine  int
	cursorCol   int
}

// BufferOption is a functional option for Buffer
type BufferOption func(*Buffer)

// WithMaxSize sets the maximum file size
func WithMaxSize(size int64) BufferOption {
	return func(b *Buffer) {
		b.maxSize = size
	}
}

// NewBuffer creates a new empty buffer
func NewBuffer(opts ...BufferOption) *Buffer {
	b := &Buffer{
		lines:     make([]string, 0),
		undoStack: make([]Edit, 0),
		redoStack: make([]Edit, 0),
		maxSize:   10 * 1024 * 1024, // 10MB default
	}
	
	for _, opt := range opts {
		opt(b)
	}
	
	return b
}

// LoadFromFile loads buffer content from a file
func (b *Buffer) LoadFromFile(filename string) error {
	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	
	if info.Size() > b.maxSize {
		return fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), b.maxSize)
	}
	
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	b.lines = make([]string, 0)
	scanner := bufio.NewScanner(file)
	
	// Increase buffer size for long lines
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)
	
	for scanner.Scan() {
		b.lines = append(b.lines, scanner.Text())
	}
	
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	// Handle empty file
	if len(b.lines) == 0 {
		b.lines = append(b.lines, "")
	}
	
	b.filename = filename
	b.dirty = false
	b.undoStack = b.undoStack[:0]
	b.redoStack = b.redoStack[:0]
	b.cursorLine = 0
	b.cursorCol = 0
	
	return nil
}

// SaveToFile saves buffer content to a file
func (b *Buffer) SaveToFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	writer := bufio.NewWriter(file)
	for i, line := range b.lines {
		if i > 0 {
			writer.WriteByte('\n')
		}
		writer.WriteString(line)
	}
	
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	b.filename = filename
	b.dirty = false
	
	return nil
}

// Save saves to the current filename
func (b *Buffer) Save() error {
	if b.filename == "" {
		return fmt.Errorf("no filename set")
	}
	return b.SaveToFile(b.filename)
}

// GetFilename returns the current filename
func (b *Buffer) GetFilename() string {
	return b.filename
}

// SetFilename sets the filename
func (b *Buffer) SetFilename(filename string) {
	b.filename = filename
}

// GetLines returns all lines
func (b *Buffer) GetLines() []string {
	return b.lines
}

// GetLine returns a specific line
func (b *Buffer) GetLine(lineNum int) (string, bool) {
	if lineNum < 0 || lineNum >= len(b.lines) {
		return "", false
	}
	return b.lines[lineNum], true
}

// LineCount returns the number of lines
func (b *Buffer) LineCount() int {
	return len(b.lines)
}

// IsDirty returns true if the buffer has unsaved changes
func (b *Buffer) IsDirty() bool {
	return b.dirty
}

// InsertText inserts text at the specified position
func (b *Buffer) InsertText(line, col int, text string) {
	if line < 0 || line >= len(b.lines) {
		return
	}
	
	currentLine := b.lines[line]
	
	// Ensure column is valid
	if col < 0 {
		col = 0
	}
	if col > len(currentLine) {
		col = len(currentLine)
	}
	
	// Record edit for undo
	edit := Edit{
		Type:    EditTypeInsert,
		Line:    line,
		Column:  col,
		NewText: text,
	}
	
	// Apply edit
	b.lines[line] = currentLine[:col] + text + currentLine[col:]
	
	b.undoStack = append(b.undoStack, edit)
	b.redoStack = b.redoStack[:0] // Clear redo stack
	b.dirty = true
}

// DeleteText deletes text at the specified position
func (b *Buffer) DeleteText(line, col, length int) {
	if line < 0 || line >= len(b.lines) {
		return
	}
	
	currentLine := b.lines[line]
	
	if col < 0 || col >= len(currentLine) {
		return
	}
	
	if col+length > len(currentLine) {
		length = len(currentLine) - col
	}
	
	// Record edit for undo
	edit := Edit{
		Type:    EditTypeDelete,
		Line:    line,
		Column:  col,
		OldText: currentLine[col : col+length],
	}
	
	// Apply edit
	b.lines[line] = currentLine[:col] + currentLine[col+length:]
	
	b.undoStack = append(b.undoStack, edit)
	b.redoStack = b.redoStack[:0]
	b.dirty = true
}

// InsertLine inserts a new line at the specified position
func (b *Buffer) InsertLine(lineNum int, text string) {
	if lineNum < 0 {
		lineNum = 0
	}
	if lineNum > len(b.lines) {
		lineNum = len(b.lines)
	}
	
	// Record edit for undo
	edit := Edit{
		Type:     EditTypeInsertLine,
		Line:     lineNum,
		NewLines: []string{text},
	}
	
	// Apply edit
	b.lines = append(b.lines[:lineNum], append([]string{text}, b.lines[lineNum:]...)...)
	
	b.undoStack = append(b.undoStack, edit)
	b.redoStack = b.redoStack[:0]
	b.dirty = true
}

// DeleteLine deletes a line at the specified position
func (b *Buffer) DeleteLine(lineNum int) {
	if lineNum < 0 || lineNum >= len(b.lines) {
		return
	}
	
	// Record edit for undo
	edit := Edit{
		Type:     EditTypeDeleteLine,
		Line:     lineNum,
		OldLines: []string{b.lines[lineNum]},
	}
	
	// Apply edit
	b.lines = append(b.lines[:lineNum], b.lines[lineNum+1:]...)
	
	// Ensure at least one line remains
	if len(b.lines) == 0 {
		b.lines = append(b.lines, "")
	}
	
	b.undoStack = append(b.undoStack, edit)
	b.redoStack = b.redoStack[:0]
	b.dirty = true
}

// ReplaceLine replaces a line entirely
func (b *Buffer) ReplaceLine(lineNum int, text string) {
	if lineNum < 0 || lineNum >= len(b.lines) {
		return
	}
	
	// Record edit for undo
	edit := Edit{
		Type:     EditTypeReplace,
		Line:     lineNum,
		OldLines: []string{b.lines[lineNum]},
		NewLines: []string{text},
	}
	
	// Apply edit
	b.lines[lineNum] = text
	
	b.undoStack = append(b.undoStack, edit)
	b.redoStack = b.redoStack[:0]
	b.dirty = true
}

// ReplaceLines replaces multiple lines
func (b *Buffer) ReplaceLines(startLine, endLine int, newLines []string) {
	if startLine < 0 {
		startLine = 0
	}
	if endLine >= len(b.lines) {
		endLine = len(b.lines) - 1
	}
	if startLine > endLine {
		return
	}
	
	// Record edit for undo
	oldLines := make([]string, endLine-startLine+1)
	copy(oldLines, b.lines[startLine:endLine+1])
	
	edit := Edit{
		Type:     EditTypeReplace,
		Line:     startLine,
		OldLines: oldLines,
		NewLines: newLines,
	}
	
	// Apply edit
	b.lines = append(b.lines[:startLine], append(newLines, b.lines[endLine+1:]...)...)
	
	b.undoStack = append(b.undoStack, edit)
	b.redoStack = b.redoStack[:0]
	b.dirty = true
}

// CanUndo returns true if undo is possible
func (b *Buffer) CanUndo() bool {
	return len(b.undoStack) > 0
}

// CanRedo returns true if redo is possible
func (b *Buffer) CanRedo() bool {
	return len(b.redoStack) > 0
}

// Undo reverts the last edit
func (b *Buffer) Undo() {
	if len(b.undoStack) == 0 {
		return
	}
	
	// Pop from undo stack
	edit := b.undoStack[len(b.undoStack)-1]
	b.undoStack = b.undoStack[:len(b.undoStack)-1]
	
	// Apply inverse edit
	switch edit.Type {
	case EditTypeInsert:
		line := b.lines[edit.Line]
		b.lines[edit.Line] = line[:edit.Column] + line[edit.Column+len(edit.NewText):]
		
	case EditTypeDelete:
		line := b.lines[edit.Line]
		b.lines[edit.Line] = line[:edit.Column] + edit.OldText + line[edit.Column:]
		
	case EditTypeInsertLine:
		b.lines = append(b.lines[:edit.Line], b.lines[edit.Line+1:]...)
		
	case EditTypeDeleteLine:
		b.lines = append(b.lines[:edit.Line], append(edit.OldLines, b.lines[edit.Line:]...)...)
		
	case EditTypeReplace:
		b.lines = append(b.lines[:edit.Line], append(edit.OldLines, b.lines[edit.Line+len(edit.NewLines):]...)...)
	}
	
	// Push to redo stack
	b.redoStack = append(b.redoStack, edit)
}

// Redo reapplies the last undone edit
func (b *Buffer) Redo() {
	if len(b.redoStack) == 0 {
		return
	}
	
	// Pop from redo stack
	edit := b.redoStack[len(b.redoStack)-1]
	b.redoStack = b.redoStack[:len(b.redoStack)-1]
	
	// Reapply edit
	switch edit.Type {
	case EditTypeInsert:
		line := b.lines[edit.Line]
		b.lines[edit.Line] = line[:edit.Column] + edit.NewText + line[edit.Column:]
		
	case EditTypeDelete:
		line := b.lines[edit.Line]
		b.lines[edit.Line] = line[:edit.Column] + line[edit.Column+len(edit.OldText):]
		
	case EditTypeInsertLine:
		b.lines = append(b.lines[:edit.Line], append(edit.NewLines, b.lines[edit.Line:]...)...)
		
	case EditTypeDeleteLine:
		b.lines = append(b.lines[:edit.Line], b.lines[edit.Line+1:]...)
		
	case EditTypeReplace:
		b.lines = append(b.lines[:edit.Line], append(edit.NewLines, b.lines[edit.Line+len(edit.OldLines):]...)...)
	}
	
	// Push to undo stack
	b.undoStack = append(b.undoStack, edit)
}

// GetCursor returns the cursor position
func (b *Buffer) GetCursor() (line, col int) {
	return b.cursorLine, b.cursorCol
}

// SetCursor sets the cursor position
func (b *Buffer) SetCursor(line, col int) {
	if line < 0 {
		line = 0
	}
	if line >= len(b.lines) {
		line = len(b.lines) - 1
	}
	
	lineText := b.lines[line]
	if col < 0 {
		col = 0
	}
	if col > len(lineText) {
		col = len(lineText)
	}
	
	b.cursorLine = line
	b.cursorCol = col
}

// MoveCursor moves the cursor by the given delta
func (b *Buffer) MoveCursor(deltaLine, deltaCol int) {
	newLine := b.cursorLine + deltaLine
	newCol := b.cursorCol + deltaCol
	
	if newLine < 0 {
		newLine = 0
		newCol = 0
	}
	if newLine >= len(b.lines) {
		newLine = len(b.lines) - 1
		newCol = len(b.lines[newLine])
	}
	
	// Adjust column if moving between lines
	if deltaLine != 0 {
		lineLen := len(b.lines[newLine])
		if newCol > lineLen {
			newCol = lineLen
		}
	} else {
		if newCol < 0 {
			newCol = 0
		}
		lineLen := len(b.lines[newLine])
		if newCol > lineLen {
			newCol = lineLen
		}
	}
	
	b.cursorLine = newLine
	b.cursorCol = newCol
}

// GetLineLength returns the length of a line in runes
func (b *Buffer) GetLineLength(lineNum int) int {
	if lineNum < 0 || lineNum >= len(b.lines) {
		return 0
	}
	return utf8.RuneCountInString(b.lines[lineNum])
}

// GetContent returns the entire buffer content as a string
func (b *Buffer) GetContent() string {
	return strings.Join(b.lines, "\n")
}

// SetContent sets the entire buffer content
func (b *Buffer) SetContent(content string) {
	b.lines = strings.Split(content, "\n")
	if len(b.lines) == 0 {
		b.lines = append(b.lines, "")
	}
	b.dirty = true
	b.undoStack = b.undoStack[:0]
	b.redoStack = b.redoStack[:0]
}

// GetLineRunes returns a line as runes for proper Unicode handling
func (b *Buffer) GetLineRunes(lineNum int) []rune {
	if lineNum < 0 || lineNum >= len(b.lines) {
		return []rune{}
	}
	return []rune(b.lines[lineNum])
}
