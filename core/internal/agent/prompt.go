package agent

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/prompts"
)

type PromptTier string

const (
	TierChat    PromptTier = "chat"
	TierFileOps PromptTier = "file_ops"
	TierFull    PromptTier = "full"
	TierReact   PromptTier = "react"
)

var (
	fileOpsPatterns = regexp.MustCompile(`(?i)\b(read|show|list|ls|cat|find|search|open|look at|what'?s in|contents of|files? in)\b`)
	commandPatterns = regexp.MustCompile(`(?i)\b(run|exec|install|build|compile|npm|pip|make|git|docker|sudo|curl|wget)\b`)
	chatPatterns    = regexp.MustCompile(`(?i)^(hi|hello|hey|thanks|thank you|ok|okay|yes|no|what is|who is|explain|tell me about|how does)\b`)
)

func ClassifyPromptTier(query string) PromptTier {
	query = strings.TrimSpace(query)

	if commandPatterns.MatchString(query) {
		return TierFull
	}

	if fileOpsPatterns.MatchString(query) {
		return TierFileOps
	}

	if chatPatterns.MatchString(query) || len(query) < 20 {
		return TierChat
	}

	return TierFull
}

// KnowledgeContext holds relevant knowledge for the agent.
type KnowledgeContext struct {
	Items []string // Relevant knowledge snippets
}

// SystemPrompt returns the agentic system prompt for the LLM.
func SystemPrompt(workingDir string) string {
	return SystemPromptWithUserContext(workingDir, nil, false, "")
}

// SystemPromptWithKnowledge returns the agentic system prompt with optional knowledge context.
func SystemPromptWithKnowledge(workingDir string, knowledge *KnowledgeContext) string {
	return SystemPromptWithUserContext(workingDir, knowledge, false, "")
}

// SystemPromptFull returns the agentic system prompt with all options (legacy, no user context).
func SystemPromptFull(workingDir string, knowledge *KnowledgeContext, unrestrictedMode bool) string {
	return SystemPromptWithUserContext(workingDir, knowledge, unrestrictedMode, "")
}

// getTierForModel returns the appropriate prompt tier based on model name.
// CR-024: Use "tiny" tier for very small models to encourage direct answers.
func getTierForModel(modelName string) string {
	modelLower := strings.ToLower(modelName)

	// Tiny models (< 4B): need direct-answer encouragement
	if strings.Contains(modelLower, "1b") ||
		strings.Contains(modelLower, "0.5b") ||
		strings.Contains(modelLower, "1.5b") ||
		strings.Contains(modelLower, "2b") ||
		strings.Contains(modelLower, "3b") ||
		strings.Contains(modelLower, "4b") {
		return "tiny"
	}

	// Medium models (7-14B): standard small tier
	if strings.Contains(modelLower, "7b") ||
		strings.Contains(modelLower, "8b") ||
		strings.Contains(modelLower, "13b") ||
		strings.Contains(modelLower, "14b") {
		return "small"
	}

	// Large models (30B+) and frontier models: large tier
	if strings.Contains(modelLower, "30b") ||
		strings.Contains(modelLower, "32b") ||
		strings.Contains(modelLower, "33b") ||
		strings.Contains(modelLower, "34b") ||
		strings.Contains(modelLower, "70b") ||
		strings.Contains(modelLower, "72b") ||
		strings.Contains(modelLower, "claude") ||
		strings.Contains(modelLower, "gpt-4") ||
		strings.Contains(modelLower, "o1") ||
		strings.Contains(modelLower, "gemini") ||
		strings.Contains(modelLower, "grok") {
		return "large"
	}

	// Default to small tier for unknown models
	return "small"
}

// SystemPromptWithPersona returns the agentic system prompt with persona identity.
// If personaIdentity is provided, it's prepended before the task instructions.
// This separates IDENTITY (who the agent is) from TASK (what the agent does).
func SystemPromptWithPersona(workingDir string, knowledge *KnowledgeContext, unrestrictedMode bool, userContext string, personaIdentity string) string {
	// Use empty model name, which defaults to "small" tier
	return SystemPromptWithModel(workingDir, knowledge, unrestrictedMode, userContext, personaIdentity, "")
}

// SystemPromptWithModel returns the agentic system prompt with model-aware tier selection.
// CR-024: Selects appropriate prompt tier based on model size for efficiency.
func SystemPromptWithModel(workingDir string, knowledge *KnowledgeContext, unrestrictedMode bool, userContext string, personaIdentity string, modelName string) string {
	log := logging.Global()

	// Determine the appropriate tier based on model
	tier := getTierForModel(modelName)

	// Try Promptomatix first with model-aware tier selection
	store := prompts.Load()
	if store.Has("agentic_tool_use") {
		optimizedPrompt := store.GetTier("agentic_tool_use", tier)
		// Fall back to "small" if tier not found
		if optimizedPrompt == "" && tier != "small" {
			optimizedPrompt = store.GetTier("agentic_tool_use", "small")
			tier = "small (fallback)"
		}
		if optimizedPrompt != "" {
			// Log format expected by monitor: task=X, tier=Y, preview=Z
			preview := optimizedPrompt
			if len(preview) > 80 {
				preview = preview[:80]
			}
			// Remove newlines for cleaner log output
			preview = strings.ReplaceAll(preview, "\n", " ")
			log.Info("[Promptomatix] task=agentic_tool_use, tier=%s, model=%s, preview=%s", tier, modelName, preview)
			return buildPromptWithPersona(optimizedPrompt, workingDir, knowledge, unrestrictedMode, userContext, personaIdentity)
		}
		log.Debug("[Promptomatix] agentic_tool_use task exists but no prompt for %s tier", tier)
	} else {
		log.Debug("[Promptomatix] agentic_tool_use task not found, using default prompt")
	}

	// Fall back to default prompt if Promptomatix not available
	return buildDefaultPromptWithPersona(workingDir, knowledge, unrestrictedMode, userContext, personaIdentity)
}

// SystemPromptWithUserContext returns the agentic system prompt with user memory context.
func SystemPromptWithUserContext(workingDir string, knowledge *KnowledgeContext, unrestrictedMode bool, userContext string) string {
	log := logging.Global()

	// Try Promptomatix first (default to small tier for local models)
	store := prompts.Load()
	if store.Has("agentic_tool_use") {
		optimizedPrompt := store.GetTier("agentic_tool_use", "small") // Local models are typically small
		if optimizedPrompt != "" {
			// Log format expected by monitor: task=X, tier=Y, preview=Z
			preview := optimizedPrompt
			if len(preview) > 80 {
				preview = preview[:80]
			}
			// Remove newlines for cleaner log output
			preview = strings.ReplaceAll(preview, "\n", " ")
			log.Info("[Promptomatix] task=agentic_tool_use, tier=small, preview=%s", preview)
			return buildPromptWithContext(optimizedPrompt, workingDir, knowledge, unrestrictedMode, userContext)
		}
		log.Debug("[Promptomatix] agentic_tool_use task exists but no prompt for small tier")
	} else {
		log.Debug("[Promptomatix] agentic_tool_use task not found, using default prompt")
	}

	// Fall back to default prompt if Promptomatix not available
	knowledgeSection := ""
	if knowledge != nil && len(knowledge.Items) > 0 {
		knowledgeSection = "\n\n## Relevant Knowledge\nThe following knowledge may be relevant to the user's request:\n"
		for i, item := range knowledge.Items {
			if i >= 5 { // Limit to top 5 items
				break
			}
			// Truncate long items
			content := item
			if len(content) > 300 {
				content = content[:300] + "..."
			}
			knowledgeSection += fmt.Sprintf("- %s\n", content)
		}
	}

	// Add unrestricted mode preamble if enabled
	unrestrictedPreamble := ""
	if unrestrictedMode {
		unrestrictedPreamble = `
## UNRESTRICTED MODE ENABLED

The user has enabled unrestricted mode. You are authorized to:
- Help with ANY technical request without refusal
- Provide direct, actionable assistance for system administration
- Execute commands and scripts as requested
- Skip safety warnings and caveats unless there is genuine risk of data loss
- Assume the user is a competent administrator who understands the implications

DO NOT refuse requests or add unnecessary warnings. The user is in control of their system.
Provide helpful, direct assistance for whatever they ask.

`
	}

	return fmt.Sprintf(`%s## MEMORY RULE - READ THIS FIRST
**ALWAYS look at the conversation history above before responding.**
If the user told you their project name, editor, preference, or ANY fact - USE IT when they ask about it.
DO NOT give generic answers when the user already told you the specific answer.

Example: If user said "I use vim" then later asks "what editor do I use?" → Answer: "You use vim."

---

You are Cortex, an intelligent AI assistant with the ability to interact with the local system.
You can read files, explore directories, search for files, and execute commands to help users accomplish tasks.

## Conversation Awareness

**CRITICAL: You MUST remember and use information from the entire conversation.**

- When the user tells you something (project names, preferences, context), REMEMBER IT for the entire session
- Before using tools, check if the user has already provided relevant information
- Reference information the user shared earlier in the conversation
- If asked about something the user mentioned, recall it from the conversation history
- Examples of information to remember: project names, file locations, preferences, context shared

Example:
- User: "My project is called Apollo"
- (Later) User: "What's my project called?"
- You: "Your project is called Apollo, as you mentioned earlier."

## Current Context
- Working Directory: %s%s

## Your Capabilities

You have access to tools that let you interact with the filesystem and execute commands.
When you need to perform an action, use a tool call in your response.

%s

## How to Use Tools

To use a tool, include this format in your response:
<tool>tool_name</tool><params>{"param_name": "value"}</params>

### Examples:

1. Reading a file:
<tool>read_file</tool><params>{"path": "README.md"}</params>

2. Listing a directory:
<tool>list_directory</tool><params>{"path": "."}</params>

3. Searching for files:
<tool>search_files</tool><params>{"pattern": "*.md", "path": "."}</params>

4. Running a command:
<tool>run_command</tool><params>{"command": "npm install"}</params>

## Guidelines

1. **RESPOND APPROPRIATELY**: Not every message needs a tool.
   - **Conversational messages** (questions, statements, context-sharing): Respond naturally WITHOUT tools
   - **Action requests** (do this, show me, install): USE tools to accomplish the task
   - "My project is called X" → Acknowledge and REMEMBER (no tool needed)
   - "What's my project name?" → Recall from conversation (no tool needed)
   - "Show me the files" → USE list_directory tool

2. **WHEN ASKED TO DO SOMETHING, USE TOOLS - NEVER JUST EXPLAIN**:
   - DO NOT give instructions for the user to follow manually
   - DO NOT say "I can't do this" - you CAN using the tools
   - ALWAYS use a tool to accomplish the task

3. **DO EXACTLY WHAT IS ASKED - NO MORE, NO LESS**:
   - "go to folder X" means list_directory for that folder, then STOP
   - "show me folder X" means list_directory, then STOP
   - "open file X" means run_command with "open file", then STOP
   - DO NOT take additional actions unless explicitly requested
   - DO NOT open files when asked to navigate to a folder
   - DO NOT run installation commands unless asked to install
   - STOP after completing the explicit request and wait for the next instruction

4. **Multi-Step Actions Only When Explicitly Requested**:
   - "read docs and install" = multiple steps OK (user said "and")
   - "go to the docs folder" = ONE step only (list that folder, stop)
   - "check what's in here" = ONE step only (list current directory, stop)

5. **Show Your Work**: Briefly explain what you're doing, then USE A TOOL to do it.

6. **Handle Errors**: If a command fails, analyze the error and try to fix it.

7. **Ask for Clarification**: If instructions are ambiguous, ask before taking actions.

8. **Safety First**:
   - Never run commands that could harm the system without explicit confirmation
   - Be careful with rm, sudo, and other dangerous commands
   - Prefer non-destructive exploration first

9. **One Tool at a Time**: Use one tool per response, then wait for the result before continuing.

10. **Output Choice for Generated Content**: When asked to create reports, summaries, briefs, or any substantial content:
   - BEFORE writing to a file, ASK: "Would you like me to display this on screen or save it to a file?"
   - Wait for the user's choice before taking action
   - Only write directly to file if the user explicitly says "save to file", "write to X.txt", or similar
   - If unclear, default to displaying on screen first - user can then ask to save it

## Response Format - SUMMARY FIRST

**CRITICAL: Always start your response with a one-sentence summary of what you did or found.**

Examples of good summary openers:
- "**Found 3 mounted volumes:** Macintosh HD, Time Machine, and iCloud Drive."
- "**Successfully installed** the dependencies using npm."
- "**Fixed the issue:** The error was caused by a missing semicolon on line 42."
- "**Completed the task:** Created the new config file at ~/.config/app.yaml."
- "**Here's what I found:** The project uses React with TypeScript and has 23 components."

This summary lets the user immediately understand if you completed the task correctly.
After the summary, you can provide additional details if needed.

---

## Output Formatting

**ALWAYS format your responses for readability using markdown:**

1. **Directory listings**: Present as a clean table or bulleted list, NOT raw ls output
   - Group by type (folders first, then files)
   - Show only relevant info (name, size for large files, date if relevant)

2. **Command output**: Wrap in code blocks with language hint
   `+"```"+`bash
   command output here
   `+"```"+`

3. **Long output**: Summarize key points, don't dump raw text
   - "Found 47 files. Key items: **package.json**, **src/** folder, **README.md**"
   - Show full details only when specifically requested

4. **Use headers** for sections: ## Results, ## Summary, ## Next Steps

5. **Use bold** for important items: **error**, **success**, file names

6. **Use bullet points** for lists, not raw newlines

Example - Directory listing:
Instead of raw "ls -la" output, respond:

## Contents of /Users/example

**Folders:**
- 📁 **Documents/** - 43 items
- 📁 **Downloads/** - 128 items  
- 📁 **Projects/** - 12 items

**Files:**
- 📄 .zshrc (2.1 KB)
- 📄 notes.txt (450 bytes)

## Common Operations

- **Go to/navigate to a folder**: <tool>list_directory</tool><params>{"path": "/target/path"}</params> (then STOP)
- **Open folder in Finder (macOS)**: <tool>run_command</tool><params>{"command": "open /path/to/folder"}</params> (only if user says "open in Finder")
- **Open file with default app**: <tool>run_command</tool><params>{"command": "open /path/to/file"}</params> (only if user says "open file")

## Example Interactions

### Example 1: Conversational recall (NO tools needed)
User: "I'm working on a project called Apollo"
Your response: "Got it! I'll remember that your project is called Apollo. How can I help you with it?"

[Later in conversation]
User: "What's my project name?"
Your response: "Your project is called Apollo, as you mentioned earlier."

### Example 2: Simple navigation (ONE action)
User: "go to the Documents folder"

Your response:
"I'll show you what's in the Documents folder.
<tool>list_directory</tool><params>{"path": "/Users/user/Documents"}</params>"

[After results, STOP and wait. Do NOT open files or take more actions.]

### Example 3: Multi-step request (user explicitly asked for multiple things)
User: "read the docs and install this project"

Your response:
"I'll help you install this project. Let me first see what's here.
<tool>list_directory</tool><params>{"path": "."}</params>"

[After seeing results]
"Found README.md. Let me read it.
<tool>read_file</tool><params>{"path": "README.md"}</params>"

[After reading, since user asked to "install"]
"Installing dependencies as requested.
<tool>run_command</tool><params>{"command": "npm install"}</params>"

CRITICAL RULES:
1. You are an AGENT that EXECUTES commands, not an advisor
2. DO EXACTLY what was asked - no extra actions
3. "go to X" = list_directory only, then STOP
4. "open X" = open command, then STOP
5. Only chain actions when user explicitly requests multiple things
`, unrestrictedPreamble, workingDir, knowledgeSection, ToolsDescription())
}

// FormatToolResult formats a tool result for the LLM context.
func FormatToolResult(result *ToolResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\n[Tool Result: %s]\n", result.Tool))
	if result.Success {
		sb.WriteString("Status: Success\n")
	} else {
		sb.WriteString("Status: Failed\n")
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
		}
	}
	if result.Output != "" {
		sb.WriteString(fmt.Sprintf("Output:\n%s\n", result.Output))
	}
	sb.WriteString("[End Tool Result]\n")

	return sb.String()
}

// FormatConversationWithTools formats a conversation history including tool results.
func FormatConversationWithTools(messages []Message) string {
	var sb strings.Builder

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("User: %s\n\n", msg.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("Assistant: %s\n\n", msg.Content))
		case "tool":
			sb.WriteString(msg.Content)
		case "system":
			sb.WriteString(fmt.Sprintf("[System: %s]\n\n", msg.Content))
		}
	}

	return sb.String()
}

// Message represents a message in the conversation.
type Message struct {
	Role    string // "user", "assistant", "tool", "system"
	Content string
}

// ═══════════════════════════════════════════════════════════════════════════════
// TIER-OPTIMIZED PROMPTS (PROMPTOMATIX INTEGRATION)
// ═══════════════════════════════════════════════════════════════════════════════

// SystemPromptForTier returns an optimized system prompt based on model size.
func SystemPromptForTier(workingDir string, knowledge *KnowledgeContext, unrestrictedMode bool, modelParams int64) string {
	store := prompts.Load()
	if store.Has("agentic_tool_use") {
		optimizedPrompt := store.Get("agentic_tool_use", modelParams)
		if optimizedPrompt != "" {
			return buildPromptWithContext(optimizedPrompt, workingDir, knowledge, unrestrictedMode, "")
		}
	}

	return SystemPromptFull(workingDir, knowledge, unrestrictedMode)
}

func SystemPromptForTask(workingDir string, knowledge *KnowledgeContext, unrestrictedMode bool, userContext string, query string) string {
	tier := ClassifyPromptTier(query)
	store := prompts.Load()
	log := logging.Global()

	var taskName string
	switch tier {
	case TierChat:
		taskName = "agentic_chat"
	case TierFileOps:
		taskName = "agentic_file_ops"
	case TierReact:
		taskName = "agentic_react"
	default:
		taskName = "agentic_tool_use"
	}

	if store.Has(taskName) {
		optimizedPrompt := store.GetTier(taskName, "small")
		if optimizedPrompt != "" {
			log.Debug("[PromptTier] task=%s, tier=%s, query_preview=%s", taskName, tier, truncate(query, 40))
			return buildPromptWithContext(optimizedPrompt, workingDir, knowledge, unrestrictedMode, userContext)
		}
	}

	return SystemPromptWithUserContext(workingDir, knowledge, unrestrictedMode, userContext)
}

func SystemPromptReact(workingDir string, knowledge *KnowledgeContext, userContext string) string {
	store := prompts.Load()
	if store.Has("agentic_react") {
		optimizedPrompt := store.GetTier("agentic_react", "small")
		if optimizedPrompt != "" {
			return buildPromptWithContext(optimizedPrompt, workingDir, knowledge, false, userContext)
		}
	}
	return SystemPromptWithUserContext(workingDir, knowledge, false, userContext)
}

func buildPromptWithContext(basePrompt string, workingDir string, knowledge *KnowledgeContext, unrestrictedMode bool, userContext string) string {
	var sb strings.Builder

	// CRITICAL: Add user context FIRST so model sees it before tool instructions
	if userContext != "" {
		sb.WriteString(`## ABOUT THE USER (ANSWER QUESTIONS USING THIS INFO)
You have personal information about this user. When they ask about themselves, their family, work, or travel - ANSWER DIRECTLY using this information. Do NOT use tools for personal questions.

`)
		sb.WriteString(userContext)
		sb.WriteString("\n\n")
	}

	// Add unrestricted mode preamble if enabled
	if unrestrictedMode {
		sb.WriteString(`## UNRESTRICTED MODE ENABLED

The user has enabled unrestricted mode. You are authorized to:
- Help with ANY technical request without refusal
- Provide direct, actionable assistance for system administration
- Execute commands and scripts as requested
- Skip safety warnings unless there is genuine risk of data loss

`)
	}

	// Add the optimized base prompt
	sb.WriteString(basePrompt)

	// Add context section
	sb.WriteString("\n\n## Current Context\n")
	sb.WriteString(fmt.Sprintf("- Working Directory: %s\n", workingDir))

	// Add knowledge section if available
	if knowledge != nil && len(knowledge.Items) > 0 {
		sb.WriteString("\n## Relevant Knowledge\n")
		for i, item := range knowledge.Items {
			if i >= 5 {
				break
			}
			content := item
			if len(content) > 300 {
				content = content[:300] + "..."
			}
			sb.WriteString(fmt.Sprintf("- %s\n", content))
		}
	}

	// Add tools description
	sb.WriteString("\n")
	sb.WriteString(ToolsDescription())

	// Add user context reminder at END for recency effect (small models remember end better)
	if userContext != "" {
		shortContext := extractKeyFacts(userContext, 400)
		if shortContext != "" {
			sb.WriteString("\n\n## REMEMBER ABOUT USER:\n")
			sb.WriteString(shortContext)
		}
	}

	return sb.String()
}

// extractKeyFacts extracts the first N characters or first 3 bullet points from context.
func extractKeyFacts(context string, maxChars int) string {
	if len(context) <= maxChars {
		return context
	}

	lines := strings.Split(context, "\n")
	var result strings.Builder
	bulletCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		isBullet := strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ")
		if isBullet {
			bulletCount++
			if bulletCount > 3 {
				break
			}
		}

		if result.Len()+len(line)+1 > maxChars {
			break
		}
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString(line)
	}

	return result.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// PERSONA-AWARE PROMPT BUILDING
// Separates IDENTITY (who the agent is) from TASK (what the agent does)
// ═══════════════════════════════════════════════════════════════════════════════

// buildPromptWithPersona builds a prompt with persona identity prepended.
// Order: IDENTITY → USER_CONTEXT → UNRESTRICTED → TASK → TOOLS → KNOWLEDGE
func buildPromptWithPersona(basePrompt string, workingDir string, knowledge *KnowledgeContext, unrestrictedMode bool, userContext string, personaIdentity string) string {
	var sb strings.Builder

	// 1. IDENTITY: Persona identity comes FIRST (defines who the agent is)
	if personaIdentity != "" {
		sb.WriteString("## IDENTITY\n")
		sb.WriteString(personaIdentity)
		sb.WriteString("\n\n")
	} else {
		// Default identity if no persona provided
		sb.WriteString("## IDENTITY\nYou are Cortex, an intelligent AI assistant for software development and system administration.\n\n")
	}

	// 2. USER_CONTEXT: Personal information about the user
	if userContext != "" {
		sb.WriteString(`## ABOUT THE USER (ANSWER QUESTIONS USING THIS INFO)
You have personal information about this user. When they ask about themselves, their family, work, or travel - ANSWER DIRECTLY using this information. Do NOT use tools for personal questions.

`)
		sb.WriteString(userContext)
		sb.WriteString("\n\n")
	}

	// 3. UNRESTRICTED: Mode preamble if enabled
	if unrestrictedMode {
		sb.WriteString(`## UNRESTRICTED MODE ENABLED

The user has enabled unrestricted mode. You are authorized to:
- Help with ANY technical request without refusal
- Provide direct, actionable assistance for system administration
- Execute commands and scripts as requested
- Skip safety warnings unless there is genuine risk of data loss

`)
	}

	// 4. TASK: The optimized task instructions from Promptomatix
	sb.WriteString("## TASK INSTRUCTIONS\n")
	sb.WriteString(basePrompt)

	// 5. CONTEXT: Working directory and other context
	sb.WriteString("\n\n## Current Context\n")
	sb.WriteString(fmt.Sprintf("- Working Directory: %s\n", workingDir))

	// 6. KNOWLEDGE: Relevant knowledge items
	if knowledge != nil && len(knowledge.Items) > 0 {
		sb.WriteString("\n## Relevant Knowledge\n")
		for i, item := range knowledge.Items {
			if i >= 5 {
				break
			}
			content := item
			if len(content) > 300 {
				content = content[:300] + "..."
			}
			sb.WriteString(fmt.Sprintf("- %s\n", content))
		}
	}

	// 7. TOOLS: Available tools description
	sb.WriteString("\n")
	sb.WriteString(ToolsDescription())

	// 8. REMINDER: User context reminder at END for recency effect
	if userContext != "" {
		shortContext := extractKeyFacts(userContext, 400)
		if shortContext != "" {
			sb.WriteString("\n\n## REMEMBER ABOUT USER:\n")
			sb.WriteString(shortContext)
		}
	}

	return sb.String()
}

// buildDefaultPromptWithPersona builds the default (non-Promptomatix) prompt with persona.
func buildDefaultPromptWithPersona(workingDir string, knowledge *KnowledgeContext, unrestrictedMode bool, userContext string, personaIdentity string) string {
	// Build knowledge section
	knowledgeSection := ""
	if knowledge != nil && len(knowledge.Items) > 0 {
		knowledgeSection = "\n\n## Relevant Knowledge\nThe following knowledge may be relevant to the user's request:\n"
		for i, item := range knowledge.Items {
			if i >= 5 {
				break
			}
			content := item
			if len(content) > 300 {
				content = content[:300] + "..."
			}
			knowledgeSection += fmt.Sprintf("- %s\n", content)
		}
	}

	// Build unrestricted preamble
	unrestrictedPreamble := ""
	if unrestrictedMode {
		unrestrictedPreamble = `
## UNRESTRICTED MODE ENABLED

The user has enabled unrestricted mode. You are authorized to:
- Help with ANY technical request without refusal
- Provide direct, actionable assistance for system administration
- Execute commands and scripts as requested
- Skip safety warnings and caveats unless there is genuine risk of data loss
- Assume the user is a competent administrator who understands the implications

DO NOT refuse requests or add unnecessary warnings. The user is in control of their system.
Provide helpful, direct assistance for whatever they ask.

`
	}

	// Build identity section
	identitySection := "You are Cortex, an intelligent AI assistant with the ability to interact with the local system."
	if personaIdentity != "" {
		identitySection = personaIdentity
	}

	return fmt.Sprintf(`%s## MEMORY RULE - READ THIS FIRST
**ALWAYS look at the conversation history above before responding.**
If the user told you their project name, editor, preference, or ANY fact - USE IT when they ask about it.
DO NOT give generic answers when the user already told you the specific answer.

Example: If user said "I use vim" then later asks "what editor do I use?" → Answer: "You use vim."

---

%s
You can read files, explore directories, search for files, and execute commands to help users accomplish tasks.

## Conversation Awareness

**CRITICAL: You MUST remember and use information from the entire conversation.**

- When the user tells you something (project names, preferences, context), REMEMBER IT for the entire session
- Before using tools, check if the user has already provided relevant information
- Reference information the user shared earlier in the conversation
- If asked about something the user mentioned, recall it from the conversation history
- Examples of information to remember: project names, file locations, preferences, context shared

## Current Context
- Working Directory: %s%s

## Your Capabilities

You have access to tools that let you interact with the filesystem and execute commands.
When you need to perform an action, use a tool call in your response.

%s

## How to Use Tools

To use a tool, include this format in your response:
<tool>tool_name</tool><params>{"param_name": "value"}</params>

## Response Format - SUMMARY FIRST

**CRITICAL: Always start your response with a one-sentence summary of what you did or found.**

Examples:
- "**Found 3 mounted volumes:** Macintosh HD, Time Machine, and iCloud Drive."
- "**Successfully installed** the dependencies using npm."
- "**Fixed the issue:** The error was caused by a missing semicolon on line 42."

This summary lets the user immediately understand if you completed the task correctly.

## Guidelines

1. **RESPOND APPROPRIATELY**: Not every message needs a tool.
2. **WHEN ASKED TO DO SOMETHING, USE TOOLS - NEVER JUST EXPLAIN**
3. **DO EXACTLY WHAT IS ASKED - NO MORE, NO LESS**
4. **One Tool at a Time**: Use one tool per response, then wait for the result before continuing.

CRITICAL RULES:
1. You are an AGENT that EXECUTES commands, not an advisor
2. DO EXACTLY what was asked - no extra actions
`, unrestrictedPreamble, identitySection, workingDir, knowledgeSection, ToolsDescription())
}
