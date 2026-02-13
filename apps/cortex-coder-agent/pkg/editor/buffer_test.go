// Package editor provides text buffer management for the editor
package editor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBuffer(t *testing.T) {
	buffer := NewBuffer()

	assert.NotNil(t, buffer)
	assert.NotNil(t, buffer.lines)
	assert.NotNil(t, buffer.undoStack)
	assert.NotNil(t, buffer.redoStack)
	assert.Equal(t, 0, buffer.LineCount())
	assert.False(t, buffer.IsDirty())
}

func TestLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "line1\nline2\nline3"
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	buffer := NewBuffer()
	err = buffer.LoadFromFile(testFile)

	assert.NoError(t, err)
	assert.Equal(t, 3, buffer.LineCount())
	assert.Equal(t, "line1", buffer.lines[0])
	assert.Equal(t, "line2", buffer.lines[1])
	assert.Equal(t, "line3", buffer.lines[2])
	assert.False(t, buffer.IsDirty())
}

func TestLoadFromFileLarge(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create file larger than max size (default 10MB)
	largeContent := string(make([]byte, 11*1024*1024))
	err := os.WriteFile(testFile, []byte(largeContent), 0644)
	require.NoError(t, err)

	buffer := NewBuffer()
	err = buffer.LoadFromFile(testFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestLoadFromFileWithCustomMaxSize(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create content larger than max size (100 bytes)
	content := string(make([]byte, 150)) // 150 bytes
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	buffer := NewBuffer(WithMaxSize(100)) // Very small limit
	err = buffer.LoadFromFile(testFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestSaveToFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	buffer := NewBuffer()
	buffer.SetContent("line1\nline2")

	err := buffer.SaveToFile(testFile)
	assert.NoError(t, err)

	savedContent, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, "line1\nline2", string(savedContent))
}

func TestInsertText(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("hello world")

	buffer.InsertText(0, 6, "beautiful ")
	assert.Equal(t, "hello beautiful world", buffer.lines[0])
	assert.True(t, buffer.IsDirty())
	assert.True(t, buffer.CanUndo())
}

func TestDeleteText(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("hello beautiful world")

	buffer.DeleteText(0, 6, 10)
	assert.Equal(t, "hello world", buffer.lines[0])
	assert.True(t, buffer.IsDirty())
	assert.True(t, buffer.CanUndo())
}

func TestInsertLine(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("line1")

	buffer.InsertLine(1, "line2")
	assert.Equal(t, 2, buffer.LineCount())
	assert.Equal(t, "line1", buffer.lines[0])
	assert.Equal(t, "line2", buffer.lines[1])
	assert.True(t, buffer.IsDirty())
}

func TestDeleteLine(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("line1\nline2\nline3")

	buffer.DeleteLine(1)
	assert.Equal(t, 2, buffer.LineCount())
	assert.Equal(t, "line1", buffer.lines[0])
	assert.Equal(t, "line3", buffer.lines[1])
	assert.True(t, buffer.IsDirty())
}

func TestReplaceLine(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("line1\nline2\nline3")

	buffer.ReplaceLine(1, "new line2")
	assert.Equal(t, "new line2", buffer.lines[1])
	assert.True(t, buffer.IsDirty())
}

func TestUndo(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("original")

	buffer.InsertText(0, 8, " text")
	assert.Equal(t, "original text", buffer.lines[0])

	buffer.Undo()
	assert.Equal(t, "original", buffer.lines[0])
	assert.True(t, buffer.CanRedo())
}

func TestRedo(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("original")

	buffer.InsertText(0, 8, " text")
	buffer.Undo()
	assert.Equal(t, "original", buffer.lines[0])

	buffer.Redo()
	assert.Equal(t, "original text", buffer.lines[0])
	assert.False(t, buffer.CanRedo())
}

func TestMultipleUndoRedo(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("a")

	buffer.InsertText(0, 1, "b")
	buffer.InsertText(0, 2, "c")
	buffer.InsertText(0, 3, "d")
	assert.Equal(t, "abcd", buffer.lines[0])

	// Undo all
	buffer.Undo()
	buffer.Undo()
	buffer.Undo()
	assert.Equal(t, "a", buffer.lines[0])

	// Redo all
	buffer.Redo()
	buffer.Redo()
	buffer.Redo()
	assert.Equal(t, "abcd", buffer.lines[0])
}

func TestCursorMovement(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("line1\nline2\nline3")

	buffer.SetCursor(0, 0)
	line, col := buffer.GetCursor()
	assert.Equal(t, 0, line)
	assert.Equal(t, 0, col)

	buffer.MoveCursor(1, 2)
	line, col = buffer.GetCursor()
	assert.Equal(t, 1, line)
	assert.Equal(t, 2, col)

	// Test boundary handling
	buffer.MoveCursor(10, 10)
	line, col = buffer.GetCursor()
	assert.Equal(t, 2, line) // Last line
	assert.Equal(t, 5, col)  // Length of "line3"
}

func TestSetContent(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("line1\nline2\nline3")

	assert.Equal(t, 3, buffer.LineCount())
	assert.Equal(t, "line1", buffer.lines[0])
	assert.Equal(t, "line2", buffer.lines[1])
	assert.Equal(t, "line3", buffer.lines[2])
	assert.True(t, buffer.IsDirty())
}

func TestEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	err := os.WriteFile(testFile, []byte{}, 0644)
	require.NoError(t, err)

	buffer := NewBuffer()
	err = buffer.LoadFromFile(testFile)

	assert.NoError(t, err)
	assert.Equal(t, 1, buffer.LineCount())
	assert.Equal(t, "", buffer.lines[0])
}

func TestSetFilename(t *testing.T) {
	buffer := NewBuffer()

	assert.Equal(t, "", buffer.GetFilename())

	buffer.SetFilename("/path/to/file.txt")
	assert.Equal(t, "/path/to/file.txt", buffer.GetFilename())
}

func TestGetLine(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("line1\nline2\nline3")

	line, ok := buffer.GetLine(0)
	assert.True(t, ok)
	assert.Equal(t, "line1", line)

	line, ok = buffer.GetLine(10)
	assert.False(t, ok)
	assert.Equal(t, "", line)
}

func TestGetLineLength(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("hello\n世界\n")

	assert.Equal(t, 5, buffer.GetLineLength(0))
	assert.Equal(t, 2, buffer.GetLineLength(1)) // Chinese characters count as 2 runes
	assert.Equal(t, 0, buffer.GetLineLength(2))
}

func TestGetContent(t *testing.T) {
	buffer := NewBuffer()
	buffer.SetContent("line1\nline2\nline3")

	content := buffer.GetContent()
	assert.Equal(t, "line1\nline2\nline3", content)
}
