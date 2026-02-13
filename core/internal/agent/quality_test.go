package agent

import "testing"

func TestLooksLikeCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// ═══════════════════════════════════════════════════════════════════════════════
		// SHOULD DETECT AS COMMANDS
		// ═══════════════════════════════════════════════════════════════════════════════

		// Shell operators
		{"run npm install", true},
		{"for i in {1..5}; do echo $i; done", true},
		{"echo hello && echo world", true},
		{"cat file.txt | grep error", true},

		// CLI tools - our own
		{"cortex task create something", true},
		{"cortex vision analyse image.png", true},
		{"cortex remember my project is Apollo", true},

		// CLI tools - database
		{"sqlite3 ~/.cortex/db.sqlite", true},
		{"mysql -u root -p", true},
		{"psql -d mydb", true},

		// CLI tools - containers
		{"docker run nginx", true},
		{"kubectl get pods", true},
		{"docker-compose up -d", true},

		// CLI tools - package managers
		{"npm install express", true},
		{"pip install requests", true},
		{"brew install wget", true},

		// CLI tools - file operations
		{"ls -la", true},
		{"cat README.md", true},
		{"mkdir new_folder", true},
		{"rm -rf node_modules", true},

		// CLI tools - network
		{"curl https://api.example.com", true},
		{"wget https://example.com/file.zip", true},
		{"ssh user@server", true},

		// CLI tools - version control
		{"git status", true},
		{"git commit -m 'message'", true},
		{"gh pr create", true},

		// Path-based commands
		{"/usr/bin/python script.py", true},
		{"./run_tests.sh", true},
		{"~/scripts/deploy.sh", true},

		// Imperative patterns
		{"execute the build script", true},
		{"list all files", true},
		{"create a new database", true},
		{"delete old logs", true},
		{"analyze this code", true},
		{"check the status", true},

		// Web search patterns (requires tool use)
		{"what is the weather today", true},
		{"what's the current stock price of AAPL", true},

		// ═══════════════════════════════════════════════════════════════════════════════
		// SHOULD NOT DETECT AS COMMANDS
		// ═══════════════════════════════════════════════════════════════════════════════

		{"explain how this works", false},
		{"tell me about python", false},
		{"how do I learn programming", false},
		{"My project is called Apollo", false},
		{"I use vim as my editor", false},
		{"Can you help me understand React?", false},
		{"What does this error mean?", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := looksLikeCommand(tt.input)
			if result != tt.expected {
				t.Errorf("looksLikeCommand(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestContainsPredictionPatterns(t *testing.T) {
	tests := []struct {
		name     string
		response string
		expected bool
	}{
		{
			name:     "prediction pattern - output will be",
			response: "The output will be:\n1\n2\n3\n4\n5",
			expected: true,
		},
		{
			name:     "prediction pattern - this will output",
			response: "This will output the following numbers",
			expected: true,
		},
		{
			name:     "prediction pattern - would result in",
			response: "Running this command would result in an error",
			expected: true,
		},
		{
			name:     "actual execution",
			response: "I executed the command and here is the result:\n\n<tool>run_command</tool>",
			expected: false,
		},
		{
			name:     "explanation without prediction",
			response: "This function returns a sorted list of elements",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsPredictionPatterns(tt.response)
			if result != tt.expected {
				t.Errorf("containsPredictionPatterns() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsRepetitive(t *testing.T) {
	tests := []struct {
		name     string
		response string
		expected bool
	}{
		{
			name:     "short response",
			response: "Hello world",
			expected: false,
		},
		{
			name:     "normal response",
			response: "The function processes the input and returns a formatted output. It handles edge cases appropriately.",
			expected: false,
		},
		{
			name:     "repetitive lines",
			response: "Processing item 1\nProcessing item 2\nProcessing item 3\nThe result is complete with processing item done\nThe result is complete with processing item done\nThe result is complete with processing item done\nThe result is complete with processing item done",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRepetitive(tt.response)
			if result != tt.expected {
				t.Errorf("isRepetitive() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLooksComplex(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Complex requests
		{"First, create the database schema. Then, implement the API endpoints. Finally, add the frontend components.", true},
		{"Refactor the authentication system to use JWT tokens", true},
		{"Implement a new feature that handles user registration, email verification, and password reset", true},
		{string(make([]byte, 250)), true}, // Long request

		// Simple requests
		{"list files", false},
		{"what time is it", false},
		{"hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(30, len(tt.input))], func(t *testing.T) {
			result := looksComplex(tt.input)
			if result != tt.expected {
				t.Errorf("looksComplex(%q...) = %v, want %v", tt.input[:min(30, len(tt.input))], result, tt.expected)
			}
		})
	}
}

func TestContainsRefusalPatterns(t *testing.T) {
	tests := []struct {
		name     string
		response string
		expected bool
	}{
		// ═══════════════════════════════════════════════════════════════════════════════
		// SHOULD DETECT AS REFUSALS
		// ═══════════════════════════════════════════════════════════════════════════════
		{
			name:     "direct refusal - can't help",
			response: "I'm sorry, but I can't help with that request.",
			expected: true,
		},
		{
			name:     "direct refusal - cannot help",
			response: "I cannot help with executing system commands directly.",
			expected: true,
		},
		{
			name:     "capability denial",
			response: "I don't have the capability to run commands on your system.",
			expected: true,
		},
		{
			name:     "ai limitation excuse",
			response: "As an AI, I cannot execute programs or access files.",
			expected: true,
		},
		{
			name:     "image refusal",
			response: "I can't analyze images directly. Please describe what you see.",
			expected: true,
		},
		{
			name:     "deflection to user",
			response: "You would need to run the sqlite3 command yourself to see the results.",
			expected: true,
		},
		{
			name:     "execute refusal",
			response: "I'm not able to execute commands on your behalf.",
			expected: true,
		},
		{
			name:     "file access refusal",
			response: "I'm unable to access files on your system directly.",
			expected: true,
		},

		// ═══════════════════════════════════════════════════════════════════════════════
		// SHOULD NOT DETECT AS REFUSALS
		// ═══════════════════════════════════════════════════════════════════════════════
		{
			name:     "actual tool use",
			response: "I'll list the directory for you.\n<tool>list_directory</tool><params>{\"path\": \".\"}</params>",
			expected: false,
		},
		{
			name:     "helpful explanation",
			response: "The sqlite3 command connects to a SQLite database. Let me run it for you.",
			expected: false,
		},
		{
			name:     "command execution",
			response: "Running the requested command now.\n<tool>run_command</tool><params>{\"command\": \"ls -la\"}</params>",
			expected: false,
		},
		{
			name:     "normal response",
			response: "Your project is called Apollo, as you mentioned earlier.",
			expected: false,
		},
		{
			name:     "error explanation without refusal",
			response: "The command failed because the file doesn't exist. Let me check what's available.",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsRefusalPatterns(tt.response)
			if result != tt.expected {
				t.Errorf("containsRefusalPatterns() = %v, want %v\nResponse: %q", result, tt.expected, tt.response)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
