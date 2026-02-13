// Package main provides the CLI entry point for Cortex Evaluator.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cortex-evaluator/cortex-evaluator/internal/session"
	"github.com/cortex-evaluator/cortex-evaluator/internal/tui"
	"github.com/spf13/cobra"
)

var (
	// Version information (set at build time)
	version = "dev"

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))
)

// getDataDir returns the data directory for storing sessions
func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cortex-evaluator"
	}
	return filepath.Join(home, ".cortex-evaluator")
}

// getStore returns a session store
func getStore() (session.Store, error) {
	dataDir := getDataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	dbPath := filepath.Join(dataDir, "sessions.db")
	return session.NewSQLiteStore(dbPath)
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "evaluator",
		Short: "Cortex Evaluator - AI-powered brainstorming and planning tool",
		Long: titleStyle.Render("Cortex Evaluator") + `

A standalone brainstorming and planning tool that enables you to:
• Create persistent brainstorming sessions
• Analyze codebases with AI assistance
• Generate PRDs and Change Requests
• Monitor code execution progress

` + dimStyle.Render("Use 'evaluator [command] --help' for more information."),
		Version: version,
	}

	// new command - create a new session
	newCmd := &cobra.Command{
		Use:   "new [name] [project-path]",
		Short: "Create a new brainstorming session",
		Long:  "Create a new brainstorming session with a name and target project folder.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			projectPath := args[1]

			// Resolve absolute path
			absPath, err := filepath.Abs(projectPath)
			if err != nil {
				return fmt.Errorf("invalid project path: %w", err)
			}

			// Verify path exists
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				return fmt.Errorf("project path does not exist: %s", absPath)
			}

			// Create session
			sess := session.NewSession(name, absPath)

			// Save to store
			store, err := getStore()
			if err != nil {
				return err
			}
			defer store.Close()

			if err := store.Save(sess); err != nil {
				return fmt.Errorf("failed to save session: %w", err)
			}

			fmt.Println(successStyle.Render("✓ Session created successfully!"))
			fmt.Println()
			fmt.Printf("  ID:      %s\n", dimStyle.Render(sess.ID))
			fmt.Printf("  Name:    %s\n", sess.Name)
			fmt.Printf("  Project: %s\n", sess.ProjectPath)
			fmt.Println()
			fmt.Println(dimStyle.Render("Run 'evaluator tui' to start brainstorming."))

			return nil
		},
	}

	// list command - list all sessions
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all brainstorming sessions",
		Long:  "List all saved sessions, optionally including archived ones.",
		RunE: func(cmd *cobra.Command, args []string) error {
			includeArchived, _ := cmd.Flags().GetBool("archived")

			store, err := getStore()
			if err != nil {
				return err
			}
			defer store.Close()

			sessions, err := store.List(includeArchived)
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}

			if len(sessions) == 0 {
				fmt.Println(dimStyle.Render("No sessions found. Create one with 'evaluator new [name] [path]'"))
				return nil
			}

			fmt.Println(titleStyle.Render("Sessions"))
			fmt.Println()

			for _, sess := range sessions {
				status := successStyle.Render("●")
				if sess.IsArchived() {
					status = dimStyle.Render("○")
				}

				fmt.Printf("%s %s\n", status, sess.Name)
				fmt.Printf("  %s\n", dimStyle.Render(sess.ID[:8]+"... | "+sess.ProjectPath))
				fmt.Println()
			}

			return nil
		},
	}
	listCmd.Flags().Bool("archived", false, "Include archived sessions")

	// resume command - resume a session
	resumeCmd := &cobra.Command{
		Use:   "resume [session-id]",
		Short: "Resume a previous session",
		Long:  "Resume a previous brainstorming session by ID or partial ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]

			store, err := getStore()
			if err != nil {
				return err
			}
			defer store.Close()

			// Try to find session by ID or partial ID
			sessions, err := store.List(true)
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}

			var found *session.Session
			for _, sess := range sessions {
				if sess.ID == sessionID || strings.HasPrefix(sess.ID, sessionID) {
					found = sess
					break
				}
			}

			if found == nil {
				return fmt.Errorf("session not found: %s", sessionID)
			}

			fmt.Println(successStyle.Render("✓ Resuming session: " + found.Name))
			fmt.Printf("  Project: %s\n", found.ProjectPath)
			fmt.Println()

			// Launch TUI with this session
			return tui.Run(found, store)
		},
	}

	// archive command - archive a session
	archiveCmd := &cobra.Command{
		Use:   "archive [session-id]",
		Short: "Archive a completed session",
		Long:  "Archive a session to keep your workspace clean. Archived sessions can still be listed with --archived.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]

			store, err := getStore()
			if err != nil {
				return err
			}
			defer store.Close()

			if err := store.Archive(sessionID); err != nil {
				return fmt.Errorf("failed to archive session: %w", err)
			}

			fmt.Println(successStyle.Render("✓ Session archived successfully"))
			return nil
		},
	}

	// tui command - launch interactive TUI
	tuiCmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive TUI mode",
		Long:  "Launch the interactive terminal user interface for brainstorming sessions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			defer store.Close()

			return tui.Run(nil, store)
		},
	}

	// Add commands
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(tuiCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("Error: "+err.Error()))
		os.Exit(1)
	}
}
