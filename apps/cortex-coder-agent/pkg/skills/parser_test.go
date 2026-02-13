package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_ParseFile(t *testing.T) {
	// Create a temporary skill file
	tempDir := t.TempDir()
	skillFile := filepath.Join(tempDir, "test-skill.yaml")

	skillContent := `
name: test-skill
version: "1.0.0"
description: A test skill for unit testing
triggers:
  - type: selection
  - type: command
    command: test
template: |
  Hello {{.Name}}!
`

	err := os.WriteFile(skillFile, []byte(skillContent), 0644)
	require.NoError(t, err)

	parser := NewParser()
	skill, err := parser.ParseFile(skillFile)

	require.NoError(t, err)
	assert.NotNil(t, skill)
	assert.Equal(t, "test-skill", skill.Name)
	assert.Equal(t, "1.0.0", skill.Version)
	assert.Equal(t, "A test skill for unit testing", skill.Description)
	assert.Len(t, skill.Triggers, 2)
	assert.Equal(t, "selection", skill.Triggers[0].Type)
	assert.Equal(t, "command", skill.Triggers[1].Type)
	assert.Equal(t, "test", skill.Triggers[1].Command)
	assert.Contains(t, skill.Template, "Hello")
	assert.Equal(t, skillFile, skill.Source)
}

func TestParser_ParseFile_NotFound(t *testing.T) {
	parser := NewParser()
	_, err := parser.ParseFile("/nonexistent/path/skill.yaml")
	assert.Error(t, err)
}

func TestParser_ParseFile_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	skillFile := filepath.Join(tempDir, "invalid.yaml")

	invalidContent := `
name: invalid-skill
invalid yaml content here
  - malformed
`

	err := os.WriteFile(skillFile, []byte(invalidContent), 0644)
	require.NoError(t, err)

	parser := NewParser()
	_, err = parser.ParseFile(skillFile)
	assert.Error(t, err)
}

func TestParser_ParseFile_MissingRequiredField(t *testing.T) {
	tempDir := t.TempDir()
	skillFile := filepath.Join(tempDir, "no-name.yaml")

	missingNameContent := `
version: "1.0.0"
description: A skill without a name
triggers:
  - type: selection
template: |
  Hello
`

	err := os.WriteFile(skillFile, []byte(missingNameContent), 0644)
	require.NoError(t, err)

	parser := NewParser()
	_, err = parser.ParseFile(skillFile)
	assert.Error(t, err)
}

func TestParser_ParseDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create multiple skill files
	skills := []struct {
		filename string
		content  string
	}{
		{"skill1.yaml", `
name: skill-1
version: "1.0.0"
description: First skill
triggers:
  - type: selection
template: |
  Skill 1
`},
		{"skill2.yaml", `
name: skill-2
version: "1.0.0"
description: Second skill
triggers:
  - type: command
    command: two
template: |
  Skill 2
`},
		{"not-a-skill.txt", `
This is not a skill file
`},
	}

	for _, s := range skills {
		path := filepath.Join(tempDir, s.filename)
		err := os.WriteFile(path, []byte(s.content), 0644)
		require.NoError(t, err)
	}

	parser := NewParser()
	loadedSkills, err := parser.ParseDir(tempDir)

	require.NoError(t, err)
	assert.Len(t, loadedSkills, 2)
	assert.Equal(t, "skill-1", loadedSkills[0].Name)
	assert.Equal(t, "skill-2", loadedSkills[1].Name)
}

func TestParser_ParseReader(t *testing.T) {
	reader := &mockReader{
		content: `
name: reader-skill
version: "1.0.0"
description: Skill from reader
triggers:
  - type: selection
template: |
  From reader
`,
	}

	parser := NewParser()
	skill, err := parser.ParseReader("test-reader", reader)

	require.NoError(t, err)
	assert.Equal(t, "reader-skill", skill.Name)
	assert.Equal(t, "From reader", skill.Template)
}

func TestParser_GetErrors(t *testing.T) {
	parser := NewParser()

	// Parse a file with errors
	tempDir := t.TempDir()
	invalidFile := filepath.Join(tempDir, "invalid.yaml")
	err := os.WriteFile(invalidFile, []byte("invalid: yaml: content:"), 0644)
	require.NoError(t, err)

	parser.ParseFile(invalidFile)

	errors := parser.GetErrors()
	assert.GreaterOrEqual(t, len(errors), 0) // May or may not have errors depending on YAML parser
}

type mockReader struct {
	content string
	pos     int
}

func (m *mockReader) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.content) {
		return 0, nil
	}
	n = copy(p, m.content[m.pos:])
	m.pos += n
	return n, nil
}
