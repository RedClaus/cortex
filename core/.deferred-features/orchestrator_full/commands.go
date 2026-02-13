package orchestrator

// ═══════════════════════════════════════════════════════════════════════════════
// SHELL COMMAND LISTS (SSOT)
// ═══════════════════════════════════════════════════════════════════════════════

// simpleShellCommands is a curated subset of commands that skip cognitive routing.
// These are safe, common commands that don't need AI interpretation.
var simpleShellCommands = []string{
	// Navigation & listing
	"ls", "cd", "pwd", "tree",
	// File viewing
	"cat", "head", "tail", "less", "more", "wc",
	// File operations
	"mkdir", "rmdir", "touch", "cp", "mv", "rm", "ln",
	// Permissions
	"chmod", "chown", "chgrp",
	// Text processing (simple invocations)
	"grep", "find", "locate", "which", "whereis",
	// System info
	"whoami", "hostname", "uname", "uptime", "date", "cal",
	"id", "groups", "w", "who", "last",
	// Environment
	"echo", "env", "printenv", "export",
	// Process & system
	"ps", "top", "htop", "df", "du", "free",
	// Network basics
	"ping", "curl", "wget", "ifconfig", "ip",
	// Development
	"git", "make",
	// Utilities
	"clear", "history", "exit", "quit", "true", "false",
}

// shellCommands is the comprehensive list of recognized shell commands.
// Used for detecting shell command input patterns.
var shellCommands = []string{
	// Navigation & files
	"ls", "pwd", "cat", "head", "tail", "less", "more", "wc",
	"find", "locate", "tree", "file", "stat", "readlink",
	// File operations
	"mkdir", "rmdir", "touch", "cp", "mv", "rm", "ln",
	"chmod", "chown", "chgrp",
	// Archives
	"tar", "gzip", "gunzip", "zip", "unzip", "bzip2", "xz",
	// Text processing
	"grep", "egrep", "fgrep", "sed", "awk", "cut", "sort", "uniq",
	"tr", "diff", "patch", "tee", "xargs",
	// System info
	"ps", "top", "htop", "df", "du", "free", "uname", "uptime",
	"whoami", "hostname", "id", "groups", "w", "who", "last",
	"env", "printenv", "set",
	// Network
	"curl", "wget", "ping", "traceroute", "netstat", "ss", "ip",
	"ifconfig", "dig", "nslookup", "host", "nc", "telnet",
	"ssh", "scp", "rsync", "ftp", "sftp",
	// Development
	"git", "svn", "hg",
	"docker", "docker-compose", "podman",
	"kubectl", "helm", "terraform",
	"npm", "yarn", "pnpm", "npx", "node", "deno", "bun",
	"go", "gofmt", "golint",
	"python", "python3", "pip", "pip3", "pipenv", "poetry",
	"ruby", "gem", "bundle", "rails",
	"cargo", "rustc", "rustup",
	"java", "javac", "mvn", "gradle",
	"make", "cmake", "gcc", "g++", "clang",
	// Package managers
	"apt", "apt-get", "dpkg", "yum", "dnf", "pacman", "brew", "port",
	// Utilities
	"echo", "printf", "date", "cal", "bc", "expr",
	"which", "whereis", "type", "command",
	"man", "info", "help", "apropos",
	"clear", "reset", "history",
	"sleep", "watch", "time", "timeout",
	"yes", "true", "false", "test",
	// Process management
	"kill", "killall", "pkill", "pgrep", "nohup", "jobs", "bg", "fg",
	// Disk & mount
	"mount", "umount", "fdisk", "lsblk", "blkid",
	// Users & permissions
	"sudo", "su", "passwd", "useradd", "usermod", "userdel",
	// Misc
	"alias", "unalias", "source", "export", "eval",
	"xdg-open", "open", "pbcopy", "pbpaste",
}
