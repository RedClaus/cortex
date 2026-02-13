package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/getlantern/systray"
)

const cortexBrainURL = "http://localhost:8080/"

// Config represents the menu configuration
type Config struct {
	Items       []MenuItem `json:"items"`
	ScanPaths   []string   `json:"scanPaths,omitempty"`
}

// MenuItem represents a single menu item
type MenuItem struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Category string `json:"category"`
	Icon     string `json:"icon,omitempty"`
}

var configPath string
var config Config
var executablePath string

// Default paths to scan for scripts (personal directories only)
var defaultScanPaths = []string{
	"~/scripts",
	"~/bin",
	"~/.local/bin",
}

var logFile *os.File

func main() {
	// Set up logging to file
	homeDir, _ := os.UserHomeDir()
	logPath := filepath.Join(homeDir, "gomenu.log")
	var err error
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("========== GoMenu Starting ==========")

	// Set config path relative to executable
	execPath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	executablePath = execPath
	log.Printf("Executable path: %s", executablePath)

	configPath = filepath.Join(filepath.Dir(execPath), "config.json")
	log.Printf("Config path: %s", configPath)

	// Also check current directory for development
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "config.json"
		log.Printf("Config not found, using: %s", configPath)
	}

	log.Println("Starting systray...")
	systray.Run(onReady, onExit)
}

// restartApp restarts the application to reload the menu
func restartApp() {
	cmd := exec.Command(executablePath)
	cmd.Start()
	systray.Quit()
}

func onReady() {
	log.Println("onReady() called")

	// Set the red lightning bolt icon
	systray.SetIcon(getRedLightningIcon())
	systray.SetTooltip("GoMenu - Quick Launcher")
	log.Println("Icon and tooltip set")

	// Load configuration
	loadConfig()
	log.Printf("Config loaded: %d items", len(config.Items))

	// Group items by category
	categories := make(map[string][]MenuItem)
	var categoryOrder []string

	for _, item := range config.Items {
		cat := item.Category
		if cat == "" {
			cat = "General"
		}
		if _, exists := categories[cat]; !exists {
			categoryOrder = append(categoryOrder, cat)
		}
		categories[cat] = append(categories[cat], item)
	}
	log.Printf("Categories: %v", categoryOrder)

	// Store menu items and their commands
	type menuCommand struct {
		item *systray.MenuItem
		cmd  string
		name string
	}
	var menuCommands []menuCommand

	// Create menu items grouped by category
	for _, category := range categoryOrder {
		items := categories[category]

		// Add category header (disabled, acts as label)
		header := systray.AddMenuItem(fmt.Sprintf("── %s ──", category), "")
		header.Disable()

		// Add items in this category
		for _, item := range items {
			menuItem := systray.AddMenuItem(item.Name, item.Command)
			menuCommands = append(menuCommands, menuCommand{item: menuItem, cmd: item.Command, name: item.Name})
			log.Printf("Added menu item: %s -> %s", item.Name, item.Command)
		}

		systray.AddSeparator()
	}

	// Add utility items
	mAddApp := systray.AddMenuItem("Add Application...", "Browse and add an app to menu")
	mScanScripts := systray.AddMenuItem("Scan for Scripts...", "Find and add scripts to menu")
	mEditConfig := systray.AddMenuItem("Edit Config", "Open config.json in editor")
	mReload := systray.AddMenuItem("Reload Config", "Reload the configuration file")

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit GoMenu", "Quit the application")
	log.Println("All menu items created")

	// Handle all menu clicks in separate goroutines
	for _, mc := range menuCommands {
		go func(m menuCommand) {
			log.Printf("Started click handler for: %s", m.name)
			for range m.item.ClickedCh {
				log.Printf("CLICK RECEIVED: %s -> %s", m.name, m.cmd)
				executeCommand(m.cmd)
			}
		}(mc)
	}

	go func() {
		log.Println("Started Add App handler")
		for range mAddApp.ClickedCh {
			log.Println("CLICK: Add Application")
			addApplication()
		}
	}()

	go func() {
		log.Println("Started Scan handler")
		for range mScanScripts.ClickedCh {
			log.Println("CLICK: Scan for Scripts")
			scanForScripts()
		}
	}()

	go func() {
		log.Println("Started Edit Config handler")
		for range mEditConfig.ClickedCh {
			log.Println("CLICK: Edit Config")
			openConfig()
		}
	}()

	go func() {
		for range mReload.ClickedCh {
			reloadMenu()
		}
	}()

	go func() {
		for range mQuit.ClickedCh {
			systray.Quit()
		}
	}()
}

func onExit() {
	// Cleanup code here
}

func loadConfig() {
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("Error reading config: %v", err)
		// Create default config if it doesn't exist
		config = Config{
			Items: []MenuItem{
				{Name: "Example Script", Command: "echo 'Hello from GoMenu!'", Category: "Examples"},
			},
			ScanPaths: defaultScanPaths,
		}
		saveConfig()
		return
	}

	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Error parsing config: %v", err)
	}

	// Set default scan paths if not configured
	if len(config.ScanPaths) == 0 {
		config.ScanPaths = defaultScanPaths
	}
}

func saveConfig() {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Printf("Error marshaling config: %v", err)
		return
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		log.Printf("Error writing config: %v", err)
	}
}

func executeCommand(command string) {
	// Show notification that we received the click
	showNotification("GoMenu", fmt.Sprintf("Running: %s", command))

	go func() {
		// For simple app-launching commands (open, osascript), execute directly
		// to avoid terminal interference from CortexBrain
		if isSimpleCommand(command) {
			log.Printf("Simple command detected, executing directly: %s", command)
			executeDirectly(command)
			return
		}

		// Try CortexBrain for complex commands, fall back to direct execution
		err := executeViaCortexBrain(command)
		if err != nil {
			log.Printf("CortexBrain failed, trying direct execution: %v", err)
			executeDirectly(command)
		}
	}()
}

// isSimpleCommand checks if a command is a simple app/file launcher
// that should bypass CortexBrain to avoid terminal interference
func isSimpleCommand(command string) bool {
	cmd := strings.TrimSpace(command)
	// Commands that launch apps or open files directly
	if strings.HasPrefix(cmd, "open ") {
		return true
	}
	// AppleScript notifications/dialogs
	if strings.HasPrefix(cmd, "osascript ") {
		return true
	}
	return false
}

// executeViaCortexBrain sends a command to CortexBrain for execution
func executeViaCortexBrain(command string) error {
	// JSON-RPC request to CortexBrain
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tasks/send",
		"id":      time.Now().UnixNano(),
		"params": map[string]interface{}{
			"id": fmt.Sprintf("gomenu-%d", time.Now().UnixNano()),
			"message": map[string]interface{}{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf("Execute this command: %s", command),
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return err
	}

	resp, err := http.Post(cortexBrainURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("CortexBrain response: %s", string(body))

	// Check if response contains an error
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	if _, hasError := result["error"]; hasError {
		return fmt.Errorf("CortexBrain error: %s", string(body))
	}

	return nil
}

// executeDirectly runs the command through bash as a fallback
func executeDirectly(command string) {
	log.Printf("executeDirectly: running %s", command)

	// Use /bin/bash -c without -l (login shell) to avoid terminal interference
	cmd := exec.Command("/bin/bash", "-c", command)

	// Completely detach from parent process - no stdin/stdout/stderr
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Create a new process group so the child is fully independent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session, detach from controlling terminal
	}

	if err := cmd.Start(); err != nil {
		log.Printf("executeDirectly: FAILED to start: %v", err)
		showNotification("GoMenu Error", fmt.Sprintf("Failed: %s", err))
		return
	}

	log.Printf("executeDirectly: started successfully, PID=%d", cmd.Process.Pid)
	// Don't wait - let the process run completely independently
	go cmd.Wait()
}

func openConfig() {
	cmd := exec.Command("open", "-e", configPath)
	if err := cmd.Start(); err != nil {
		// Try VS Code as fallback
		cmd = exec.Command("code", configPath)
		cmd.Start()
	}
}

func reloadMenu() {
	showNotification("GoMenu", "Configuration reloaded. Restart app for menu changes.")
	loadConfig()
}

func showNotification(title, message string) {
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
	exec.Command("osascript", "-e", script).Run()
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

// showScanOptionsDialog shows a dialog to configure scan options and returns selected values
func showScanOptionsDialog() (timePeriod string, scanPath string, cancelled bool) {
	// First, ask for time period
	timeScript := `
set timeOptions to {"1 Week", "1 Month", "2 Months", "6 Months", "1 Year", "All Time"}
set selectedTime to choose from list timeOptions with title "Scan Options" with prompt "How far back should we look for scripts?" default items {"2 Months"}
if selectedTime is false then
	return "CANCELLED"
end if
return item 1 of selectedTime
`
	cmd := exec.Command("osascript", "-e", timeScript)
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "CANCELLED" {
		return "", "", true
	}
	timePeriod = strings.TrimSpace(string(output))

	// Then ask for location
	// Build the location options from config
	locationOptions := []string{"All Configured Paths"}
	for _, p := range config.ScanPaths {
		locationOptions = append(locationOptions, p)
	}
	locationOptions = append(locationOptions, "Choose Custom Folder...")

	locationScript := fmt.Sprintf(`
set locationOptions to {%s}
set selectedLocation to choose from list locationOptions with title "Scan Location" with prompt "Where should we scan for scripts?" default items {"All Configured Paths"}
if selectedLocation is false then
	return "CANCELLED"
end if
return item 1 of selectedLocation
`, formatListForAppleScript(locationOptions))

	cmd = exec.Command("osascript", "-e", locationScript)
	output, err = cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "CANCELLED" {
		return "", "", true
	}
	scanPath = strings.TrimSpace(string(output))

	// If custom folder selected, show folder picker
	if scanPath == "Choose Custom Folder..." {
		folderScript := `
set selectedFolder to choose folder with prompt "Select folder to scan for scripts:"
return POSIX path of selectedFolder
`
		cmd = exec.Command("osascript", "-e", folderScript)
		output, err = cmd.Output()
		if err != nil {
			return "", "", true
		}
		scanPath = strings.TrimSpace(string(output))

		// Ask if they want to save this path to config
		saveScript := fmt.Sprintf(`
display dialog "Add '%s' to your saved scan paths?" buttons {"No", "Yes"} default button "Yes"
return button returned of result
`, scanPath)
		cmd = exec.Command("osascript", "-e", saveScript)
		output, _ = cmd.Output()
		if strings.TrimSpace(string(output)) == "Yes" {
			// Add to config
			config.ScanPaths = append(config.ScanPaths, scanPath)
			saveConfig()
			showNotification("GoMenu", "Scan path saved to configuration.")
		}
	}

	return timePeriod, scanPath, false
}

// parseTimePeriod converts a time period string to a duration
func parseTimePeriod(period string) time.Duration {
	switch period {
	case "1 Week":
		return 7 * 24 * time.Hour
	case "1 Month":
		return 30 * 24 * time.Hour
	case "2 Months":
		return 60 * 24 * time.Hour
	case "6 Months":
		return 180 * 24 * time.Hour
	case "1 Year":
		return 365 * 24 * time.Hour
	case "All Time":
		return 0 // Special case: no time limit
	default:
		return 60 * 24 * time.Hour // Default to 2 months
	}
}

// addApplication opens a file picker to select an app and adds it to the menu
func addApplication() {
	// Show file picker dialog starting at /Applications
	script := `
set defaultFolder to POSIX file "/Applications" as alias
try
	set selectedApp to choose file of type {"app"} with prompt "Select an application to add to GoMenu:" default location defaultFolder
	return POSIX path of selectedApp
on error
	return "CANCELLED"
end try
`
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("App picker cancelled or error: %v", err)
		return
	}

	appPath := strings.TrimSpace(string(output))
	if appPath == "" || appPath == "CANCELLED" {
		return
	}

	// Extract app name from path (e.g., "/Applications/Safari.app" -> "Safari")
	appName := filepath.Base(appPath)
	appName = strings.TrimSuffix(appName, ".app")

	// Check if already in config
	for _, item := range config.Items {
		if strings.Contains(item.Command, appName) {
			showNotification("GoMenu", fmt.Sprintf("%s is already in the menu.", appName))
			return
		}
	}

	// Ask for category
	categoryScript := `
set categoryOptions to {"Apps", "Development", "Utilities", "Quick Access", "Custom..."}
set selectedCategory to choose from list categoryOptions with title "Select Category" with prompt "Which category should this app be in?" default items {"Apps"}
if selectedCategory is false then
	return "CANCELLED"
end if
return item 1 of selectedCategory
`
	cmd = exec.Command("osascript", "-e", categoryScript)
	output, err = cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "CANCELLED" {
		return
	}

	category := strings.TrimSpace(string(output))

	// If custom category selected, ask for name
	if category == "Custom..." {
		customScript := `
set customCategory to display dialog "Enter category name:" default answer "My Apps" with title "Custom Category"
return text returned of customCategory
`
		cmd = exec.Command("osascript", "-e", customScript)
		output, err = cmd.Output()
		if err != nil {
			return
		}
		category = strings.TrimSpace(string(output))
		if category == "" {
			category = "Apps"
		}
	}

	// Create the menu item
	newItem := MenuItem{
		Name:     appName,
		Command:  fmt.Sprintf("open -a '%s'", appName),
		Category: category,
	}

	config.Items = append(config.Items, newItem)
	saveConfig()

	showNotification("GoMenu", fmt.Sprintf("Added %s to %s. Restarting...", appName, category))

	// Restart app to show new menu item
	go func() {
		time.Sleep(1 * time.Second)
		restartApp()
	}()
}

// scanForScripts scans configured paths for scripts and shows a dialog to add them
func scanForScripts() {
	// Show options dialog first
	timePeriod, scanPath, cancelled := showScanOptionsDialog()
	if cancelled {
		return
	}

	scripts := findScriptsWithOptions(timePeriod, scanPath)

	if len(scripts) == 0 {
		showNotification("GoMenu", "No new scripts found in selected location.")
		return
	}

	// Build the list for the dialog
	scriptList := make([]string, len(scripts))
	for i, s := range scripts {
		scriptList[i] = fmt.Sprintf("%d. %s", i+1, s)
	}

	// Show dialog using osascript
	dialogScript := fmt.Sprintf(`
set scriptList to {%s}
set selectedItems to choose from list scriptList with title "Add Scripts to GoMenu" with prompt "Select scripts to add:" with multiple selections allowed
if selectedItems is false then
	return ""
end if
set AppleScript's text item delimiters to ","
return selectedItems as text
`, formatListForAppleScript(scripts))

	cmd := exec.Command("osascript", "-e", dialogScript)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Dialog cancelled or error: %v", err)
		return
	}

	selected := strings.TrimSpace(string(output))
	if selected == "" {
		return
	}

	// Parse selected items and add to config
	selectedScripts := strings.Split(selected, ", ")
	addedCount := 0

	for _, scriptPath := range selectedScripts {
		// Check if already in config
		alreadyExists := false
		for _, item := range config.Items {
			if item.Command == scriptPath || item.Command == "~/"+getRelativeToHome(scriptPath) {
				alreadyExists = true
				break
			}
		}

		if !alreadyExists {
			name := filepath.Base(scriptPath)
			// Remove extension for display name
			name = strings.TrimSuffix(name, filepath.Ext(name))
			// Convert to title case
			name = strings.Title(strings.ReplaceAll(name, "-", " "))
			name = strings.Title(strings.ReplaceAll(name, "_", " "))

			newItem := MenuItem{
				Name:     name,
				Command:  scriptPath,
				Category: "Scripts",
			}
			config.Items = append(config.Items, newItem)
			addedCount++
		}
	}

	if addedCount > 0 {
		saveConfig()
		showNotification("GoMenu", fmt.Sprintf("Added %d script(s). Reloading menu...", addedCount))
		// Give notification time to show, then restart
		go func() {
			exec.Command("sleep", "1").Run()
			restartApp()
		}()
	} else {
		showNotification("GoMenu", "Selected scripts are already in menu.")
	}
}

// findScriptsWithOptions searches for executable scripts with configurable options
func findScriptsWithOptions(timePeriod string, scanLocation string) []string {
	var scripts []string
	seen := make(map[string]bool)

	// Calculate cutoff time based on selected period
	duration := parseTimePeriod(timePeriod)
	var cutoffTime time.Time
	if duration > 0 {
		cutoffTime = time.Now().Add(-duration)
	}
	// If duration is 0 (All Time), cutoffTime stays as zero value and we skip the time check

	// Get existing commands to filter out
	existingCommands := make(map[string]bool)
	for _, item := range config.Items {
		existingCommands[item.Command] = true
		existingCommands[expandPath(item.Command)] = true
	}

	// Determine which paths to scan
	var pathsToScan []string
	if scanLocation == "All Configured Paths" {
		pathsToScan = config.ScanPaths
	} else {
		pathsToScan = []string{scanLocation}
	}

	for _, scanPath := range pathsToScan {
		fullPath := expandPath(scanPath)

		entries, err := os.ReadDir(fullPath)
		if err != nil {
			log.Printf("Cannot read directory %s: %v", fullPath, err)
			continue // Skip paths that don't exist
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			filePath := filepath.Join(fullPath, entry.Name())

			// Check if file is executable
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Check if executable (any execute bit set)
			if info.Mode()&0111 == 0 {
				continue
			}

			// Skip files older than cutoff (unless "All Time" selected)
			if duration > 0 && info.ModTime().Before(cutoffTime) {
				continue
			}

			// Skip if already in config
			if existingCommands[filePath] {
				continue
			}

			// Skip common non-script executables
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}

			// Avoid duplicates
			if seen[filePath] {
				continue
			}
			seen[filePath] = true

			scripts = append(scripts, filePath)
		}
	}

	sort.Strings(scripts)
	return scripts
}

// formatListForAppleScript formats a string slice for AppleScript list
func formatListForAppleScript(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		// Escape quotes and format for AppleScript
		escaped := strings.ReplaceAll(item, `"`, `\"`)
		quoted[i] = fmt.Sprintf(`"%s"`, escaped)
	}
	return strings.Join(quoted, ", ")
}

// getRelativeToHome returns path relative to home directory
func getRelativeToHome(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return strings.TrimPrefix(path, home+"/")
	}
	return path
}

// getRedLightningIcon generates a red lightning bolt icon for the menu bar
func getRedLightningIcon() []byte {
	// Create a 22x22 image (standard macOS menu bar size)
	size := 22
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Red color for the lightning bolt
	red := color.RGBA{255, 59, 48, 255} // Apple's system red

	// Fill the lightning bolt using scanline
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if isInsideLightningBolt(x, y, size) {
				img.Set(x, y, red)
			}
		}
	}

	// Encode to PNG
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// isInsideLightningBolt checks if a point is inside the lightning bolt shape
func isInsideLightningBolt(x, y, size int) bool {
	// Define lightning bolt as a polygon
	// Scaled for 22x22 pixels
	// Top triangle: (11,0) -> (5,10) -> (11,10)
	// Bottom triangle: (11,10) -> (5,22) -> (11,22) shifted

	// Simplified lightning bolt using line segments
	// Upper part (y 0-10): from top point spreading down
	if y >= 0 && y <= 10 {
		// Left edge: from (11,0) to (5,10)
		leftX := 11 - int(float64(y)*0.6)
		// Right edge: from (11,0) to (13,10)
		rightX := 11 + int(float64(y)*0.2)
		if x >= leftX && x <= rightX {
			return true
		}
	}

	// Middle bulge (y 8-12)
	if y >= 8 && y <= 12 {
		if x >= 5 && x <= 14 {
			return true
		}
	}

	// Lower part (y 10-21): from middle spreading to bottom point
	if y >= 10 && y <= 21 {
		progress := float64(y-10) / 11.0
		// Narrowing down to point at bottom
		centerX := 8.0
		width := 4.0 * (1.0 - progress)
		leftX := int(centerX - width)
		rightX := int(centerX + width)
		if x >= leftX && x <= rightX {
			return true
		}
	}

	return false
}
