package screens

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/normanking/cortex-key-vault/internal/service"
	"github.com/normanking/cortex-key-vault/internal/storage"
	"github.com/normanking/cortex-key-vault/internal/tui/styles"
)

// FormMode indicates whether we're creating or editing
type FormMode int

const (
	FormModeCreate FormMode = iota
	FormModeEdit
)

// FormScreen is the add/edit secret form
type FormScreen struct {
	vault    *service.VaultService
	mode     FormMode
	editID   string // ID of secret being edited (for edit mode)
	width    int
	height   int
	focused  int
	err      string
	saved    bool
	canceled bool

	// Form fields
	secretType   storage.SecretType
	typeIndex    int
	nameInput    textinput.Model
	valueInput   textinput.Model
	userInput    textinput.Model // Username (for password type)
	urlInput     textinput.Model
	notesInput   textinput.Model
	tagsInput    textinput.Model
	categoryID   string
	categoryIdx  int
	categories   []storage.Category
	showPassword bool
}

const (
	fieldType = iota
	fieldName
	fieldValue
	fieldUser
	fieldURL
	fieldNotes
	fieldTags
	fieldCategory
	fieldCount
)

// SecretSavedMsg is sent when a secret is saved
type SecretSavedMsg struct {
	Secret *storage.Secret
}

// FormCanceledMsg is sent when form is canceled
type FormCanceledMsg struct{}

// NewFormScreen creates a new form for adding a secret
func NewFormScreen(vault *service.VaultService) FormScreen {
	return newForm(vault, FormModeCreate, nil, "")
}

// NewEditFormScreen creates a form for editing an existing secret
func NewEditFormScreen(vault *service.VaultService, secret *storage.Secret, currentValue string) FormScreen {
	return newForm(vault, FormModeEdit, secret, currentValue)
}

func newForm(vault *service.VaultService, mode FormMode, secret *storage.Secret, currentValue string) FormScreen {
	// Create inputs
	nameInput := textinput.New()
	nameInput.Placeholder = "Secret name (e.g., OpenAI API Key)"
	nameInput.CharLimit = 100

	valueInput := textinput.New()
	valueInput.Placeholder = "Secret value"
	valueInput.EchoMode = textinput.EchoPassword
	valueInput.EchoCharacter = '•'

	userInput := textinput.New()
	userInput.Placeholder = "Username or email"
	userInput.CharLimit = 100

	urlInput := textinput.New()
	urlInput.Placeholder = "URL (e.g., https://platform.openai.com)"
	urlInput.CharLimit = 500

	notesInput := textinput.New()
	notesInput.Placeholder = "Additional notes"
	notesInput.CharLimit = 500

	tagsInput := textinput.New()
	tagsInput.Placeholder = "Tags (comma-separated, e.g., work, api)"
	tagsInput.CharLimit = 200

	// Load categories
	categories, _ := vault.GetCategories()

	f := FormScreen{
		vault:       vault,
		mode:        mode,
		nameInput:   nameInput,
		valueInput:  valueInput,
		userInput:   userInput,
		urlInput:    urlInput,
		notesInput:  notesInput,
		tagsInput:   tagsInput,
		categories:  categories,
		secretType:  storage.TypeAPIKey,
		categoryID:  "all",
		categoryIdx: 0,
	}

	// If editing, populate fields
	if mode == FormModeEdit && secret != nil {
		f.editID = secret.ID
		f.nameInput.SetValue(secret.Name)
		f.valueInput.SetValue(currentValue)
		f.userInput.SetValue(secret.Username)
		f.urlInput.SetValue(secret.URL)
		f.notesInput.SetValue(secret.Notes)
		f.tagsInput.SetValue(strings.Join(secret.Tags, ", "))
		f.secretType = secret.Type
		f.categoryID = secret.CategoryID

		// Find type index
		types := storage.GetSecretTypeInfo()
		for i, t := range types {
			if t.Type == secret.Type {
				f.typeIndex = i
				break
			}
		}

		// Find category index
		for i, c := range categories {
			if c.ID == secret.CategoryID {
				f.categoryIdx = i
				break
			}
		}
	}

	// Focus first field
	f.nameInput.Focus()
	f.focused = fieldName

	return f
}

// Init initializes the form
func (m FormScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the form
func (m FormScreen) Update(msg tea.Msg) (FormScreen, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.canceled = true
			return m, func() tea.Msg { return FormCanceledMsg{} }

		case "ctrl+s":
			return m.save()

		case "tab", "down":
			m.focusNext()

		case "shift+tab", "up":
			m.focusPrev()

		case "enter":
			if m.focused == fieldType || m.focused == fieldCategory {
				// Cycle through options
				m.cycleOption()
			} else {
				m.focusNext()
			}

		case "ctrl+g":
			// Generate password
			if m.focused == fieldValue {
				m.valueInput.SetValue(generatePassword(24))
			}

		case "ctrl+r":
			// Toggle password visibility
			if m.showPassword {
				m.valueInput.EchoMode = textinput.EchoPassword
				m.showPassword = false
			} else {
				m.valueInput.EchoMode = textinput.EchoNormal
				m.showPassword = true
			}
		}
	}

	// Update focused input
	switch m.focused {
	case fieldName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case fieldValue:
		m.valueInput, cmd = m.valueInput.Update(msg)
	case fieldUser:
		m.userInput, cmd = m.userInput.Update(msg)
	case fieldURL:
		m.urlInput, cmd = m.urlInput.Update(msg)
	case fieldNotes:
		m.notesInput, cmd = m.notesInput.Update(msg)
	case fieldTags:
		m.tagsInput, cmd = m.tagsInput.Update(msg)
	}

	return m, cmd
}

func (m *FormScreen) focusNext() {
	m.blurAll()
	m.focused = (m.focused + 1) % fieldCount
	m.focusCurrent()
}

func (m *FormScreen) focusPrev() {
	m.blurAll()
	m.focused--
	if m.focused < 0 {
		m.focused = fieldCount - 1
	}
	m.focusCurrent()
}

func (m *FormScreen) blurAll() {
	m.nameInput.Blur()
	m.valueInput.Blur()
	m.userInput.Blur()
	m.urlInput.Blur()
	m.notesInput.Blur()
	m.tagsInput.Blur()
}

func (m *FormScreen) focusCurrent() {
	switch m.focused {
	case fieldName:
		m.nameInput.Focus()
	case fieldValue:
		m.valueInput.Focus()
	case fieldUser:
		m.userInput.Focus()
	case fieldURL:
		m.urlInput.Focus()
	case fieldNotes:
		m.notesInput.Focus()
	case fieldTags:
		m.tagsInput.Focus()
	}
}

func (m *FormScreen) cycleOption() {
	if m.focused == fieldType {
		types := storage.GetSecretTypeInfo()
		m.typeIndex = (m.typeIndex + 1) % len(types)
		m.secretType = types[m.typeIndex].Type
	} else if m.focused == fieldCategory {
		m.categoryIdx = (m.categoryIdx + 1) % len(m.categories)
		m.categoryID = m.categories[m.categoryIdx].ID
	}
}

func (m FormScreen) save() (FormScreen, tea.Cmd) {
	// Validate
	name := strings.TrimSpace(m.nameInput.Value())
	if name == "" {
		m.err = "Name is required"
		return m, nil
	}

	value := m.valueInput.Value()
	if value == "" && m.mode == FormModeCreate {
		m.err = "Value is required"
		return m, nil
	}

	// Parse tags
	tagsStr := m.tagsInput.Value()
	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	// Create/update secret
	secret := &storage.Secret{
		Name:       name,
		Type:       m.secretType,
		Username:   strings.TrimSpace(m.userInput.Value()),
		URL:        strings.TrimSpace(m.urlInput.Value()),
		Notes:      strings.TrimSpace(m.notesInput.Value()),
		CategoryID: m.categoryID,
		Tags:       tags,
	}

	var err error
	if m.mode == FormModeCreate {
		err = m.vault.CreateSecret(secret, value)
	} else {
		secret.ID = m.editID
		var valuePtr *string
		if value != "" {
			valuePtr = &value
		}
		err = m.vault.UpdateSecret(secret, valuePtr)
	}

	if err != nil {
		m.err = "Failed to save: " + err.Error()
		return m, nil
	}

	m.saved = true
	return m, func() tea.Msg { return SecretSavedMsg{Secret: secret} }
}

// View renders the form
func (m FormScreen) View() string {
	// Handle zero dimensions
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	title := "Add New Secret"
	if m.mode == FormModeEdit {
		title = "Edit Secret"
	}

	var content strings.Builder
	content.WriteString(styles.HeaderTitle.Render(title) + "\n\n")

	types := storage.GetSecretTypeInfo()
	inputWidth := 50

	// Type selector
	typeStyle := styles.Input.Width(inputWidth)
	if m.focused == fieldType {
		typeStyle = styles.InputFocused.Width(inputWidth)
	}
	typeDisplay := fmt.Sprintf("%s %s", types[m.typeIndex].Icon, types[m.typeIndex].Name)
	content.WriteString(styles.InputLabel.Render("Type") + " " + styles.TextMutedStyle.Render("[Enter to cycle]") + "\n")
	content.WriteString(typeStyle.Render(typeDisplay) + "\n\n")

	// Name input
	content.WriteString(styles.InputLabel.Render("Name") + " " + styles.TextMutedStyle.Render("*") + "\n")
	nameStyle := styles.Input.Width(inputWidth)
	if m.focused == fieldName {
		nameStyle = styles.InputFocused.Width(inputWidth)
	}
	content.WriteString(nameStyle.Render(m.nameInput.View()) + "\n\n")

	// Value input
	valueLabel := "Value"
	if m.showPassword {
		valueLabel += " (visible)"
	}
	content.WriteString(styles.InputLabel.Render(valueLabel) + " " + styles.TextMutedStyle.Render("* [Ctrl+G] Generate [Ctrl+R] Toggle") + "\n")
	valueStyle := styles.Input.Width(inputWidth)
	if m.focused == fieldValue {
		valueStyle = styles.InputFocused.Width(inputWidth)
	}
	content.WriteString(valueStyle.Render(m.valueInput.View()) + "\n\n")

	// Username (mainly for password type, but useful for all)
	content.WriteString(styles.InputLabel.Render("Username") + "\n")
	userStyle := styles.Input.Width(inputWidth)
	if m.focused == fieldUser {
		userStyle = styles.InputFocused.Width(inputWidth)
	}
	content.WriteString(userStyle.Render(m.userInput.View()) + "\n\n")

	// URL
	content.WriteString(styles.InputLabel.Render("URL") + "\n")
	urlStyle := styles.Input.Width(inputWidth)
	if m.focused == fieldURL {
		urlStyle = styles.InputFocused.Width(inputWidth)
	}
	content.WriteString(urlStyle.Render(m.urlInput.View()) + "\n\n")

	// Notes
	content.WriteString(styles.InputLabel.Render("Notes") + "\n")
	notesStyle := styles.Input.Width(inputWidth)
	if m.focused == fieldNotes {
		notesStyle = styles.InputFocused.Width(inputWidth)
	}
	content.WriteString(notesStyle.Render(m.notesInput.View()) + "\n\n")

	// Tags
	content.WriteString(styles.InputLabel.Render("Tags") + " " + styles.TextMutedStyle.Render("(comma-separated)") + "\n")
	tagsStyle := styles.Input.Width(inputWidth)
	if m.focused == fieldTags {
		tagsStyle = styles.InputFocused.Width(inputWidth)
	}
	content.WriteString(tagsStyle.Render(m.tagsInput.View()) + "\n\n")

	// Category selector
	catStyle := styles.Input.Width(inputWidth)
	if m.focused == fieldCategory {
		catStyle = styles.InputFocused.Width(inputWidth)
	}
	catDisplay := "No categories"
	if len(m.categories) > 0 && m.categoryIdx < len(m.categories) {
		catDisplay = fmt.Sprintf("%s %s", m.categories[m.categoryIdx].Icon, m.categories[m.categoryIdx].Name)
	}
	content.WriteString(styles.InputLabel.Render("Category") + " " + styles.TextMutedStyle.Render("[Enter to cycle]") + "\n")
	content.WriteString(catStyle.Render(catDisplay) + "\n\n")

	// Error
	if m.err != "" {
		content.WriteString(styles.ErrorText.Render("⚠ " + m.err) + "\n\n")
	}

	// Help
	content.WriteString(styles.HelpText.Render("[Tab/↑↓] Navigate  [Ctrl+S] Save  [Esc] Cancel"))

	// Center in screen
	panel := styles.Panel.Width(60).Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)
}

// IsSaved returns true if the form was saved
func (m FormScreen) IsSaved() bool { return m.saved }

// IsCanceled returns true if the form was canceled
func (m FormScreen) IsCanceled() bool { return m.canceled }

// generatePassword generates a secure random password
func generatePassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[n.Int64()]
	}
	return string(result)
}
