package skills

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkill_Validate(t *testing.T) {
	tests := []struct {
		name    string
		skill   Skill
		wantErr int
	}{
		{
			name: "valid skill",
			skill: Skill{
				Name:        "test-skill",
				Version:     "1.0.0",
				Description: "A test skill",
				Triggers:    []Trigger{{Type: "selection"}},
				Template:    "Hello {{.Name}}",
			},
			wantErr: 0,
		},
		{
			name: "missing name",
			skill: Skill{
				Version:     "1.0.0",
				Description: "A test skill",
				Triggers:    []Trigger{{Type: "selection"}},
				Template:    "Hello",
			},
			wantErr: 1,
		},
		{
			name: "missing version",
			skill: Skill{
				Name:        "test-skill",
				Description: "A test skill",
				Triggers:    []Trigger{{Type: "selection"}},
				Template:    "Hello",
			},
			wantErr: 1,
		},
		{
			name: "missing description",
			skill: Skill{
				Name:     "test-skill",
				Version:  "1.0.0",
				Triggers: []Trigger{{Type: "selection"}},
				Template: "Hello",
			},
			wantErr: 1,
		},
		{
			name: "missing triggers",
			skill: Skill{
				Name:        "test-skill",
				Version:     "1.0.0",
				Description: "A test skill",
				Template:    "Hello",
			},
			wantErr: 1,
		},
		{
			name: "missing template",
			skill: Skill{
				Name:        "test-skill",
				Version:     "1.0.0",
				Description: "A test skill",
				Triggers:    []Trigger{{Type: "selection"}},
			},
			wantErr: 1,
		},
		{
			name: "empty trigger type",
			skill: Skill{
				Name:        "test-skill",
				Version:     "1.0.0",
				Description: "A test skill",
				Triggers:    []Trigger{{Type: ""}},
				Template:    "Hello",
			},
			wantErr: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.skill.Validate()
			assert.Equal(t, tt.wantErr, len(errs), "expected %d errors, got %d", tt.wantErr, len(errs))
		})
	}
}

func TestSkill_HasTrigger(t *testing.T) {
	skill := Skill{
		Name:        "test-skill",
		Version:     "1.0.0",
		Description: "A test skill",
		Triggers: []Trigger{
			{Type: "selection"},
			{Type: "command", Command: "test"},
		},
	}

	assert.True(t, skill.HasTrigger("selection"))
	assert.True(t, skill.HasTrigger("command"))
	assert.False(t, skill.HasTrigger("file"))
}

func TestSkill_HasCommandTrigger(t *testing.T) {
	skill := Skill{
		Name:        "test-skill",
		Version:     "1.0.0",
		Description: "A test skill",
		Triggers: []Trigger{
			{Type: "command", Command: "explain"},
			{Type: "command", Command: "test"},
		},
	}

	assert.True(t, skill.HasCommandTrigger("explain"))
	assert.True(t, skill.HasCommandTrigger("test"))
	assert.False(t, skill.HasCommandTrigger("refactor"))
}

func TestSkill_HasFileExtensionTrigger(t *testing.T) {
	skill := Skill{
		Name:        "test-skill",
		Version:     "1.0.0",
		Description: "A test skill",
		Triggers: []Trigger{
			{Type: "file", Extensions: []string{".go", ".py"}},
		},
	}

	assert.True(t, skill.HasFileExtensionTrigger(".go"))
	assert.True(t, skill.HasFileExtensionTrigger("go"))
	assert.True(t, skill.HasFileExtensionTrigger(".py"))
	assert.True(t, skill.HasFileExtensionTrigger("py"))
	assert.False(t, skill.HasFileExtensionTrigger(".js"))
	assert.False(t, skill.HasFileExtensionTrigger("js"))
}

func TestValidationError_Error(t *testing.T) {
	err := ValidationError{
		Field:   "name",
		Message: "skill name is required",
	}
	assert.Equal(t, "name: skill name is required", err.Error())
}
