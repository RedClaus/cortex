package modals

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ═══════════════════════════════════════════════════════════════════════════════
// DEVICE SELECTION MESSAGES
// ═══════════════════════════════════════════════════════════════════════════════

// DeviceSelectedMsg is sent when a device is selected.
type DeviceSelectedMsg struct {
	Device    AudioDevice
	IsInput   bool // true = microphone, false = speaker
}

// DevicesLoadedMsg is sent when devices are loaded from the voice orchestrator.
type DevicesLoadedMsg struct {
	InputDevices  []AudioDevice
	OutputDevices []AudioDevice
	CurrentInput  *int
	CurrentOutput *int
	Error         error
}

// ═══════════════════════════════════════════════════════════════════════════════
// AUDIO DEVICE TYPE
// ═══════════════════════════════════════════════════════════════════════════════

// AudioDevice represents an audio input or output device.
type AudioDevice struct {
	Index      int     `json:"index"`
	Name       string  `json:"name"`
	Channels   int     `json:"channels"`
	SampleRate float64 `json:"sample_rate"`
	IsInput    bool    `json:"is_input"`
	IsDefault  bool    `json:"is_default"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// LIST ITEM WRAPPER
// ═══════════════════════════════════════════════════════════════════════════════

// deviceItem wraps an AudioDevice for use in the bubbles list.
type deviceItem struct {
	device    AudioDevice
	isCurrent bool
}

// FilterValue implements list.Item interface.
func (i deviceItem) FilterValue() string {
	return i.device.Name
}

// Title implements list.DefaultItem interface.
func (i deviceItem) Title() string {
	prefix := ""
	if i.isCurrent {
		prefix = "✓ "
	}
	return prefix + i.device.Name
}

// Description implements list.DefaultItem interface.
func (i deviceItem) Description() string {
	parts := []string{}

	if i.device.Channels > 0 {
		if i.device.Channels == 1 {
			parts = append(parts, "Mono")
		} else if i.device.Channels == 2 {
			parts = append(parts, "Stereo")
		} else {
			parts = append(parts, fmt.Sprintf("%d channels", i.device.Channels))
		}
	}

	if i.device.SampleRate > 0 {
		parts = append(parts, fmt.Sprintf("%.0f Hz", i.device.SampleRate))
	}

	if i.device.IsDefault {
		parts = append(parts, "System Default")
	}

	if i.isCurrent {
		parts = append(parts, "Active")
	}

	return strings.Join(parts, " • ")
}

// ═══════════════════════════════════════════════════════════════════════════════
// DEVICE SELECTOR MODAL
// ═══════════════════════════════════════════════════════════════════════════════

// DeviceSelector is a modal for selecting audio input/output devices.
type DeviceSelector struct {
	inputList     list.Model
	outputList    list.Model
	width         int
	height        int
	activeTab     int // 0 = inputs (microphones), 1 = outputs (speakers)
	loading       bool
	error         error
	inputDevices  []AudioDevice
	outputDevices []AudioDevice
	currentInput  *int
	currentOutput *int
}

// NewDeviceSelector creates a new device selector modal.
func NewDeviceSelector() *DeviceSelector {
	// Create empty lists - will be populated when devices are loaded
	delegate := newDeviceDelegate()

	inputList := list.New([]list.Item{}, delegate, 60, 10)
	inputList.Title = "🎤 Microphones"
	inputList.SetShowHelp(false)
	inputList.SetFilteringEnabled(false)
	inputList.SetShowStatusBar(false)
	inputList.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7aa2f7")).
		Bold(true).
		Padding(0, 1)

	outputList := list.New([]list.Item{}, delegate, 60, 10)
	outputList.Title = "🔊 Speakers"
	outputList.SetShowHelp(false)
	outputList.SetFilteringEnabled(false)
	outputList.SetShowStatusBar(false)
	outputList.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#bb9af7")).
		Bold(true).
		Padding(0, 1)

	return &DeviceSelector{
		inputList:  inputList,
		outputList: outputList,
		width:      70,
		height:     24,
		activeTab:  0,
		loading:    true, // Start in loading state
	}
}

// newDeviceDelegate creates a custom delegate for device items.
func newDeviceDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()

	// Active/selected item styling
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(lipgloss.Color("#7aa2f7")).
		Bold(true).
		Padding(0, 1)

	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(lipgloss.Color("#7aa2f7")).
		Padding(0, 1)

	// Normal item styling
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5")).
		Padding(0, 1)

	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89")).
		Padding(0, 1)

	// Dimmed styling for non-active tab
	delegate.Styles.DimmedTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#414868")).
		Padding(0, 1)

	delegate.Styles.DimmedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3b4261")).
		Padding(0, 1)

	return delegate
}

// SetSize sets the modal dimensions.
func (d *DeviceSelector) SetSize(width, height int) {
	d.width = width
	d.height = height

	listWidth := width - 6
	listHeight := (height - 10) / 2 // Split between two lists

	d.inputList.SetSize(listWidth, listHeight)
	d.outputList.SetSize(listWidth, listHeight)
}

// SetDevices sets the available devices and current selections.
func (d *DeviceSelector) SetDevices(msg DevicesLoadedMsg) {
	d.loading = false
	d.error = msg.Error

	if msg.Error != nil {
		return
	}

	d.inputDevices = msg.InputDevices
	d.outputDevices = msg.OutputDevices
	d.currentInput = msg.CurrentInput
	d.currentOutput = msg.CurrentOutput

	// Convert to list items
	inputItems := make([]list.Item, len(msg.InputDevices))
	for i, dev := range msg.InputDevices {
		dev.IsInput = true
		isCurrent := msg.CurrentInput != nil && *msg.CurrentInput == dev.Index
		inputItems[i] = deviceItem{device: dev, isCurrent: isCurrent}
	}
	d.inputList.SetItems(inputItems)

	outputItems := make([]list.Item, len(msg.OutputDevices))
	for i, dev := range msg.OutputDevices {
		dev.IsInput = false
		isCurrent := msg.CurrentOutput != nil && *msg.CurrentOutput == dev.Index
		outputItems[i] = deviceItem{device: dev, isCurrent: isCurrent}
	}
	d.outputList.SetItems(outputItems)

	// Select current devices in lists
	if msg.CurrentInput != nil {
		for i, dev := range msg.InputDevices {
			if dev.Index == *msg.CurrentInput {
				d.inputList.Select(i)
				break
			}
		}
	}

	if msg.CurrentOutput != nil {
		for i, dev := range msg.OutputDevices {
			if dev.Index == *msg.CurrentOutput {
				d.outputList.Select(i)
				break
			}
		}
	}
}

// Init implements tea.Model.
func (d *DeviceSelector) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (d *DeviceSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case DevicesLoadedMsg:
		d.SetDevices(msg)
		return d, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Close modal - parent should handle this
			return d, nil

		case "tab", "shift+tab":
			// Switch between input and output lists
			if d.activeTab == 0 {
				d.activeTab = 1
			} else {
				d.activeTab = 0
			}
			return d, nil

		case "1":
			d.activeTab = 0
			return d, nil

		case "2":
			d.activeTab = 1
			return d, nil

		case "enter":
			// Select current device
			if d.activeTab == 0 {
				if item, ok := d.inputList.SelectedItem().(deviceItem); ok {
					return d, func() tea.Msg {
						return DeviceSelectedMsg{Device: item.device, IsInput: true}
					}
				}
			} else {
				if item, ok := d.outputList.SelectedItem().(deviceItem); ok {
					return d, func() tea.Msg {
						return DeviceSelectedMsg{Device: item.device, IsInput: false}
					}
				}
			}
			return d, nil
		}
	}

	// Update the active list
	var cmd tea.Cmd
	if d.activeTab == 0 {
		d.inputList, cmd = d.inputList.Update(msg)
	} else {
		d.outputList, cmd = d.outputList.Update(msg)
	}

	return d, cmd
}

// View implements tea.Model and renders the device selector.
func (d *DeviceSelector) View() string {
	if d.loading {
		return d.renderLoading()
	}

	if d.error != nil {
		return d.renderError()
	}

	return d.renderContent()
}

// renderLoading renders the loading state.
func (d *DeviceSelector) renderLoading() string {
	style := lipgloss.NewStyle().
		Width(d.width).
		Height(d.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color("#7aa2f7"))

	content := style.Render("⏳ Loading audio devices...")

	return d.renderBox(content)
}

// renderError renders the error state.
func (d *DeviceSelector) renderError() string {
	style := lipgloss.NewStyle().
		Width(d.width - 4).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color("#f7768e"))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5")).
		Width(d.width - 4)

	content := lipgloss.JoinVertical(lipgloss.Center,
		style.Render("❌ Failed to load audio devices"),
		"",
		errorStyle.Render(d.error.Error()),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render("Press ESC to close"),
	)

	return d.renderBox(content)
}

// renderContent renders the main device selection UI.
func (d *DeviceSelector) renderContent() string {
	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5")).
		Bold(true).
		Width(d.width - 4).
		Align(lipgloss.Center).
		MarginBottom(1)

	title := titleStyle.Render("🎙️ Audio Device Settings")

	// Tab headers
	activeTabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(lipgloss.Color("#7aa2f7")).
		Bold(true).
		Padding(0, 2)

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89")).
		Background(lipgloss.Color("#24283b")).
		Padding(0, 2)

	var tab1Style, tab2Style lipgloss.Style
	if d.activeTab == 0 {
		tab1Style = activeTabStyle
		tab2Style = inactiveTabStyle
	} else {
		tab1Style = inactiveTabStyle
		tab2Style = activeTabStyle
	}

	tabs := lipgloss.JoinHorizontal(lipgloss.Top,
		tab1Style.Render("1. Microphones"),
		"  ",
		tab2Style.Render("2. Speakers"),
	)

	tabRow := lipgloss.NewStyle().
		Width(d.width - 4).
		Align(lipgloss.Center).
		MarginBottom(1).
		Render(tabs)

	// Active list
	var listContent string
	if d.activeTab == 0 {
		listContent = d.inputList.View()
	} else {
		listContent = d.outputList.View()
	}

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89")).
		Width(d.width - 4).
		Align(lipgloss.Center).
		MarginTop(1)

	help := helpStyle.Render("Tab: switch • ↑/↓: select • Enter: apply • Esc: close")

	// Combine all parts
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		tabRow,
		listContent,
		help,
	)

	return d.renderBox(content)
}

// renderBox wraps content in a styled box.
func (d *DeviceSelector) renderBox(content string) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7aa2f7")).
		Padding(1, 2).
		Width(d.width)

	return boxStyle.Render(content)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// GetActiveTab returns the currently active tab (0 = input, 1 = output).
func (d *DeviceSelector) GetActiveTab() int {
	return d.activeTab
}

// IsLoading returns whether devices are still loading.
func (d *DeviceSelector) IsLoading() bool {
	return d.loading
}

// HasError returns whether there was an error loading devices.
func (d *DeviceSelector) HasError() bool {
	return d.error != nil
}
