package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_NewRegistry(t *testing.T) {
	r := NewRegistry("/builtin", "/user", "/project")
	assert.NotNil(t, r)
	assert.NotNil(t, r.skills)
	assert.NotNil(t, r.byCommand)
	assert.NotNil(t, r.byExtension)
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry("", "", "")
	skill := &Skill{
		Name:        "test-skill",
		Version:     "1.0.0",
		Description: "A test skill",
		Triggers: []Trigger{
			{Type: "selection"},
			{Type: "command", Command: "test"},
			{Type: "file", Extensions: []string{".go", ".py"}},
		},
	}

	r.Register(skill)

	assert.Equal(t, 1, r.Count())
	assert.True(t, r.skills["test-skill"] != nil)
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry("", "", "")
	skill := &Skill{
		Name:    "get-test",
		Version: "1.0.0",
	}
	r.Register(skill)

	got, ok := r.Get("get-test")
	assert.True(t, ok)
	assert.Equal(t, "get-test", got.Name)

	_, ok = r.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_GetByCommand(t *testing.T) {
	r := NewRegistry("", "", "")
	skill := &Skill{
		Name:    "cmd-skill",
		Version: "1.0.0",
		Triggers: []Trigger{
			{Type: "command", Command: "mycommand"},
		},
	}
	r.Register(skill)

	got, ok := r.GetByCommand("mycommand")
	assert.True(t, ok)
	assert.Equal(t, "cmd-skill", got.Name)

	_, ok = r.GetByCommand("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_GetByFileExtension(t *testing.T) {
	r := NewRegistry("", "", "")
	skill := &Skill{
		Name:    "file-skill",
		Version: "1.0.0",
		Triggers: []Trigger{
			{Type: "file", Extensions: []string{"go", "py"}},
		},
	}
	r.Register(skill)

	skills := r.GetByFileExtension("go")
	assert.Len(t, skills, 1)
	assert.Equal(t, "file-skill", skills[0].Name)

	skills = r.GetByFileExtension(".py")
	assert.Len(t, skills, 1)

	skills = r.GetByFileExtension("js")
	assert.Len(t, skills, 0)
}

func TestRegistry_FindForSelection(t *testing.T) {
	r := NewRegistry("", "", "")
	skill := &Skill{
		Name:    "selection-skill",
		Version: "1.0.0",
		Triggers: []Trigger{
			{Type: "selection"},
		},
	}
	r.Register(skill)

	skills := r.FindForSelection()
	assert.Len(t, skills, 1)
	assert.Equal(t, "selection-skill", skills[0].Name)
}

func TestRegistry_FindForFile(t *testing.T) {
	r := NewRegistry("", "", "")
	skill := &Skill{
		Name:    "go-skill",
		Version: "1.0.0",
		Triggers: []Trigger{
			{Type: "file", Extensions: []string{"go"}},
		},
	}
	r.Register(skill)

	skills := r.FindForFile("/path/to/main.go")
	assert.Len(t, skills, 1)

	skills = r.FindForFile("/path/to/main.js")
	assert.Len(t, skills, 0)
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry("", "", "")

	// Empty registry
	assert.Len(t, r.List(), 0)

	// Add skills
	r.Register(&Skill{Name: "skill1", Version: "1.0.0"})
	r.Register(&Skill{Name: "skill2", Version: "1.0.0"})
	r.Register(&Skill{Name: "skill3", Version: "1.0.0"})

	skills := r.List()
	assert.Len(t, skills, 3)
}

func TestRegistry_ListByCategory(t *testing.T) {
	r := NewRegistry("", "", "")

	r.Register(&Skill{Name: "s1", Version: "1.0.0", Category: "cat1"})
	r.Register(&Skill{Name: "s2", Version: "1.0.0", Category: "cat1"})
	r.Register(&Skill{Name: "s3", Version: "1.0.0", Category: "cat2"})
	r.Register(&Skill{Name: "s4", Version: "1.0.0"}) // No category

	byCategory := r.ListByCategory()

	assert.Len(t, byCategory["cat1"], 2)
	assert.Len(t, byCategory["cat2"], 1)
	assert.Len(t, byCategory["uncategorized"], 1)
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry("", "", "")
	r.Register(&Skill{
		Name:    "remove-test",
		Version: "1.0.0",
		Triggers: []Trigger{
			{Type: "selection"},
		},
	})

	assert.Equal(t, 1, r.Count())

	r.Remove("remove-test")

	assert.Equal(t, 0, r.Count())
	assert.Nil(t, r.skills["remove-test"])
}

func TestRegistry_Clear(t *testing.T) {
	r := NewRegistry("", "", "")
	r.Register(&Skill{Name: "s1", Version: "1.0.0"})
	r.Register(&Skill{Name: "s2", Version: "1.0.0"})

	r.Clear()

	assert.Equal(t, 0, r.Count())
}

func TestRegistry_LoadAll(t *testing.T) {
	// Create temp directories
	tempDir := t.TempDir()
	builtinDir := filepath.Join(tempDir, "builtin")
	userDir := filepath.Join(tempDir, "user")
	projectDir := filepath.Join(tempDir, "project")

	os.MkdirAll(builtinDir, 0755)
	os.MkdirAll(userDir, 0755)
	os.MkdirAll(projectDir, 0755)

	// Create skill files with unique names
	skills := []struct {
		dir      string
		name     string
		fileName string
	}{
		{builtinDir, "builtin-skill", "skill1.yaml"},
		{userDir, "user-skill", "skill2.yaml"},
		{projectDir, "project-skill", "skill3.yaml"},
	}

	for _, s := range skills {
		skillContent := `
name: ` + s.name + `
version: "1.0.0"
description: Test skill
triggers:
  - type: selection
template: |
  Test
`
		err := os.WriteFile(filepath.Join(s.dir, s.fileName), []byte(skillContent), 0644)
		require.NoError(t, err)
	}

	r := NewRegistry(builtinDir, userDir, projectDir)
	loadErr := r.LoadAll()

	require.NoError(t, loadErr)
	assert.Equal(t, 3, r.Count())
}

func TestRegistry_GetParser(t *testing.T) {
	r := NewRegistry("", "", "")
	parser := r.GetParser()
	assert.NotNil(t, parser)
}

func TestRegistry_GetTemplateEngine(t *testing.T) {
	r := NewRegistry("", "", "")
	engine := r.GetTemplateEngine()
	assert.NotNil(t, engine)
}

func TestRegistry_GetSkillNames(t *testing.T) {
	r := NewRegistry("", "", "")
	r.Register(&Skill{Name: "explain-code", Version: "1.0.0"})
	r.Register(&Skill{Name: "refactor-code", Version: "1.0.0"})
	r.Register(&Skill{Name: "add-tests", Version: "1.0.0"})

	names := r.GetSkillNames("code")
	assert.Len(t, names, 2)
	assert.Contains(t, names, "explain-code")
	assert.Contains(t, names, "refactor-code")
}

func TestDefaultPaths(t *testing.T) {
	builtin, user, project := DefaultPaths()

	assert.Contains(t, builtin, "skills/builtin")
	assert.Contains(t, user, ".config")
	assert.Contains(t, user, "cortex-coder")
	assert.Contains(t, project, ".coder")
}

func TestEnsurePaths(t *testing.T) {
	// Patch os.Getwd for test
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	builtin, user, project, err := EnsurePaths()
	require.NoError(t, err)

	// Check directories exist
	_, err = os.Stat(builtin)
	assert.NoError(t, err)

	_, err = os.Stat(user)
	assert.NoError(t, err)

	_, err = os.Stat(project)
	assert.NoError(t, err)
}
