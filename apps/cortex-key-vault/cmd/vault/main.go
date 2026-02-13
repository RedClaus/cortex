package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/normanking/cortex-key-vault/internal/service"
	"github.com/normanking/cortex-key-vault/internal/storage"
	"github.com/normanking/cortex-key-vault/internal/tui"
	"golang.org/x/term"
)

func main() {
	// Check for subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "import":
			if len(os.Args) < 3 {
				fmt.Fprintln(os.Stderr, "Usage: vault import <file.json>")
				os.Exit(1)
			}
			runImport(os.Args[2])
			return
		case "help", "--help", "-h":
			printHelp()
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
			printHelp()
			os.Exit(1)
		}
	}

	// Default: run TUI
	runTUI()
}

func printHelp() {
	fmt.Println(`Cortex Key Vault - Secure secrets manager

Usage:
  vault              Launch interactive TUI
  vault import FILE  Import secrets from JSON file
  vault help         Show this help

Import file format:
  {
    "secrets": [
      {
        "name": "Secret Name",
        "type": "api_key|ssh_key|password|certificate",
        "value": "secret-value",
        "username": "optional",
        "url": "optional",
        "notes": "optional",
        "category": "all",
        "tags": ["tag1", "tag2"]
      }
    ]
  }`)
}

func runTUI() {
	app, err := tui.NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer app.Close()

	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

func runImport(filePath string) {
	// Check file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: file not found: %s\n", filePath)
		os.Exit(1)
	}

	// Read and parse JSON file
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var importFile storage.ImportFile
	if err := json.Unmarshal(data, &importFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
		fmt.Fprintln(os.Stderr, "Make sure the file has the correct format (see: vault help)")
		os.Exit(1)
	}

	if len(importFile.Secrets) == 0 {
		fmt.Fprintln(os.Stderr, "No secrets found in import file")
		os.Exit(1)
	}

	fmt.Printf("Found %d secrets to import\n", len(importFile.Secrets))

	// Initialize vault service
	vault, err := service.NewVaultService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing vault: %v\n", err)
		os.Exit(1)
	}
	defer vault.Close()

	// Check if vault is initialized
	if !vault.IsInitialized() {
		fmt.Println("Vault not initialized. Please run 'vault' first to create a master password.")
		os.Exit(1)
	}

	// Prompt for master password
	password, err := promptPassword("Enter master password: ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
		os.Exit(1)
	}

	// Unlock vault
	if err := vault.Unlock(password); err != nil {
		fmt.Fprintln(os.Stderr, "Error: incorrect password")
		os.Exit(1)
	}

	fmt.Println("Vault unlocked. Importing secrets...")
	fmt.Println()

	// Perform import
	result, err := vault.ImportSecrets(&importFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during import: %v\n", err)
		os.Exit(1)
	}

	// Print results
	fmt.Println("Import complete!")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("  Total:    %d\n", result.Total)
	fmt.Printf("  Imported: %d\n", result.Imported)
	fmt.Printf("  Skipped:  %d\n", result.Skipped)

	if len(result.Errors) > 0 {
		fmt.Println()
		fmt.Println("Errors:")
		for _, e := range result.Errors {
			fmt.Printf("  - %s: %s\n", e.Name, e.Reason)
		}
	}

	if result.Imported > 0 {
		fmt.Println()
		fmt.Println("Run 'vault' to view your imported secrets.")
	}
}

func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	// Try to read password securely (hidden input)
	if term.IsTerminal(int(syscall.Stdin)) {
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // newline after hidden input
		if err != nil {
			return "", err
		}
		return string(password), nil
	}

	// Fallback for non-terminal (piped input)
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(password), nil
}
