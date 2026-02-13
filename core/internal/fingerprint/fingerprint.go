// Package fingerprint provides platform fingerprinting functionality including
// environment detection, trust scoring, and pre-flight security checks.
package fingerprint

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Platform identifies the operating environment.
type Platform string

const (
	PlatformLinux   Platform = "linux"
	PlatformDarwin  Platform = "darwin"   // macOS
	PlatformWindows Platform = "windows"
	PlatformUnknown Platform = "unknown"
)

// Environment identifies the development/runtime context.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
	EnvCI          Environment = "ci"
	EnvDocker      Environment = "docker"
	EnvSSH         Environment = "ssh"
	EnvUnknown     Environment = "unknown"
)

// ProjectType identifies the kind of project.
type ProjectType string

const (
	ProjectGo       ProjectType = "go"
	ProjectNode     ProjectType = "node"
	ProjectPython   ProjectType = "python"
	ProjectRust     ProjectType = "rust"
	ProjectRuby     ProjectType = "ruby"
	ProjectJava     ProjectType = "java"
	ProjectUnknown  ProjectType = "unknown"
	ProjectMultiple ProjectType = "multiple" // Monorepo
)

// Fingerprint contains the detected platform information.
type Fingerprint struct {
	// System information
	Platform    Platform    `json:"platform"`
	Arch        string      `json:"arch"`
	OS          string      `json:"os"`
	Hostname    string      `json:"hostname"`
	User        string      `json:"user"`
	HomeDir     string      `json:"home_dir"`
	Environment Environment `json:"environment"`

	// Shell information
	Shell        string `json:"shell"`
	ShellVersion string `json:"shell_version,omitempty"`

	// Development environment
	ProjectType   ProjectType       `json:"project_type,omitempty"`
	ProjectRoot   string            `json:"project_root,omitempty"`
	GitBranch     string            `json:"git_branch,omitempty"`
	GitRemote     string            `json:"git_remote,omitempty"`
	GitDirty      bool              `json:"git_dirty,omitempty"`
	PackageFile   string            `json:"package_file,omitempty"`
	RuntimeVersions map[string]string `json:"runtime_versions,omitempty"`

	// Network environment (for infrastructure context)
	NetworkDevices []NetworkDevice `json:"network_devices,omitempty"`
	SSHConnected   bool            `json:"ssh_connected,omitempty"`
	DockerAvailable bool           `json:"docker_available,omitempty"`

	// Timestamps
	FingerprintedAt time.Time `json:"fingerprinted_at"`
}

// NetworkDevice represents a network interface.
type NetworkDevice struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // "cisco", "juniper", "linux", etc.
	Address string `json:"address,omitempty"`
}

// Fingerprinter detects platform information.
type Fingerprinter struct {
	// Timeout for detection commands
	timeout time.Duration
}

// NewFingerprinter creates a new fingerprinter.
func NewFingerprinter() *Fingerprinter {
	return &Fingerprinter{
		timeout: 5 * time.Second,
	}
}

// Detect gathers platform information.
func (f *Fingerprinter) Detect(ctx context.Context) (*Fingerprint, error) {
	fp := &Fingerprint{
		Platform:        f.detectPlatform(),
		Arch:            runtime.GOARCH,
		OS:              runtime.GOOS,
		FingerprintedAt: time.Now(),
		RuntimeVersions: make(map[string]string),
	}

	// Hostname
	if hostname, err := os.Hostname(); err == nil {
		fp.Hostname = hostname
	}

	// User info
	fp.User = os.Getenv("USER")
	if fp.User == "" {
		fp.User = os.Getenv("USERNAME")
	}

	// Home directory
	if home, err := os.UserHomeDir(); err == nil {
		fp.HomeDir = home
	}

	// Shell
	fp.Shell = f.detectShell()
	fp.ShellVersion = f.detectShellVersion(ctx, fp.Shell)

	// Environment
	fp.Environment = f.detectEnvironment()

	// Docker
	fp.DockerAvailable = f.detectDocker(ctx)

	// SSH
	fp.SSHConnected = os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != ""

	// Runtime versions (parallel detection)
	fp.RuntimeVersions = f.detectRuntimes(ctx)

	return fp, nil
}

// DetectProject analyzes the current directory for project type.
func (f *Fingerprinter) DetectProject(ctx context.Context, dir string) (*Fingerprint, error) {
	fp, err := f.Detect(ctx)
	if err != nil {
		return nil, err
	}

	// Find project root
	fp.ProjectRoot = f.findProjectRoot(dir)
	if fp.ProjectRoot == "" {
		fp.ProjectRoot = dir
	}

	// Detect project type
	fp.ProjectType = f.detectProjectType(fp.ProjectRoot)

	// Detect package file
	fp.PackageFile = f.findPackageFile(fp.ProjectRoot, fp.ProjectType)

	// Git information
	f.detectGit(ctx, fp, fp.ProjectRoot)

	return fp, nil
}

func (f *Fingerprinter) detectPlatform() Platform {
	switch runtime.GOOS {
	case "linux":
		return PlatformLinux
	case "darwin":
		return PlatformDarwin
	case "windows":
		return PlatformWindows
	default:
		return PlatformUnknown
	}
}

func (f *Fingerprinter) detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		// Windows fallback
		if runtime.GOOS == "windows" {
			return "cmd"
		}
		return "/bin/sh"
	}
	return shell
}

func (f *Fingerprinter) detectShellVersion(ctx context.Context, shell string) string {
	if shell == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Extract first line
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

func (f *Fingerprinter) detectEnvironment() Environment {
	// CI detection
	ciVars := []string{"CI", "CONTINUOUS_INTEGRATION", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "TRAVIS"}
	for _, v := range ciVars {
		if os.Getenv(v) != "" {
			return EnvCI
		}
	}

	// Docker detection
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return EnvDocker
	}

	// SSH detection
	if os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != "" {
		return EnvSSH
	}

	// Development vs production heuristics
	nodeEnv := os.Getenv("NODE_ENV")
	goEnv := os.Getenv("GO_ENV")
	railsEnv := os.Getenv("RAILS_ENV")

	if nodeEnv == "production" || goEnv == "production" || railsEnv == "production" {
		return EnvProduction
	}

	if nodeEnv == "development" || goEnv == "development" || railsEnv == "development" {
		return EnvDevelopment
	}

	return EnvUnknown
}

func (f *Fingerprinter) detectDocker(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "version")
	return cmd.Run() == nil
}

func (f *Fingerprinter) detectRuntimes(ctx context.Context) map[string]string {
	versions := make(map[string]string)
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	runtimes := map[string][]string{
		"go":     {"go", "version"},
		"node":   {"node", "--version"},
		"python": {"python3", "--version"},
		"rust":   {"rustc", "--version"},
		"ruby":   {"ruby", "--version"},
		"java":   {"java", "-version"},
	}

	for name, args := range runtimes {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		output, err := cmd.Output()
		if err == nil {
			// Extract version number
			version := strings.TrimSpace(string(output))
			// Get first line for multi-line output
			if idx := strings.Index(version, "\n"); idx != -1 {
				version = version[:idx]
			}
			versions[name] = version
		}
	}

	return versions
}

func (f *Fingerprinter) findProjectRoot(dir string) string {
	rootMarkers := []string{
		".git",
		"go.mod",
		"package.json",
		"Cargo.toml",
		"requirements.txt",
		"Gemfile",
		"pom.xml",
		"build.gradle",
	}

	current := dir
	for {
		for _, marker := range rootMarkers {
			if _, err := os.Stat(filepath.Join(current, marker)); err == nil {
				return current
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break // Reached filesystem root
		}
		current = parent
	}

	return ""
}

func (f *Fingerprinter) detectProjectType(dir string) ProjectType {
	projectFiles := map[string]ProjectType{
		"go.mod":           ProjectGo,
		"package.json":     ProjectNode,
		"requirements.txt": ProjectPython,
		"setup.py":         ProjectPython,
		"pyproject.toml":   ProjectPython,
		"Cargo.toml":       ProjectRust,
		"Gemfile":          ProjectRuby,
		"pom.xml":          ProjectJava,
		"build.gradle":     ProjectJava,
	}

	detected := make([]ProjectType, 0)
	for file, ptype := range projectFiles {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			detected = append(detected, ptype)
		}
	}

	switch len(detected) {
	case 0:
		return ProjectUnknown
	case 1:
		return detected[0]
	default:
		return ProjectMultiple
	}
}

func (f *Fingerprinter) findPackageFile(dir string, ptype ProjectType) string {
	packageFiles := map[ProjectType][]string{
		ProjectGo:     {"go.mod"},
		ProjectNode:   {"package.json"},
		ProjectPython: {"pyproject.toml", "requirements.txt", "setup.py"},
		ProjectRust:   {"Cargo.toml"},
		ProjectRuby:   {"Gemfile"},
		ProjectJava:   {"pom.xml", "build.gradle"},
	}

	files, ok := packageFiles[ptype]
	if !ok {
		return ""
	}

	for _, file := range files {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			return file
		}
	}

	return ""
}

func (f *Fingerprinter) detectGit(ctx context.Context, fp *Fingerprint, dir string) {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	// Check if git repo
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return
	}

	// Get branch
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	if output, err := cmd.Output(); err == nil {
		fp.GitBranch = strings.TrimSpace(string(output))
	}

	// Get remote
	cmd = exec.CommandContext(ctx, "git", "-C", dir, "config", "--get", "remote.origin.url")
	if output, err := cmd.Output(); err == nil {
		fp.GitRemote = strings.TrimSpace(string(output))
	}

	// Check dirty status
	cmd = exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain")
	if output, err := cmd.Output(); err == nil {
		fp.GitDirty = len(strings.TrimSpace(string(output))) > 0
	}
}

// JSON returns the fingerprint as JSON.
func (fp *Fingerprint) JSON() ([]byte, error) {
	return json.MarshalIndent(fp, "", "  ")
}

// Summary returns a brief text summary.
func (fp *Fingerprint) Summary() string {
	var sb strings.Builder
	sb.WriteString("Platform: " + string(fp.Platform) + "/" + fp.Arch + "\n")
	sb.WriteString("Shell: " + fp.Shell + "\n")
	sb.WriteString("Environment: " + string(fp.Environment) + "\n")
	if fp.ProjectType != "" && fp.ProjectType != ProjectUnknown {
		sb.WriteString("Project: " + string(fp.ProjectType) + "\n")
		if fp.GitBranch != "" {
			dirty := ""
			if fp.GitDirty {
				dirty = " (dirty)"
			}
			sb.WriteString("Git: " + fp.GitBranch + dirty + "\n")
		}
	}
	return sb.String()
}

// IsNetworkDevice checks if a platform indicator suggests network equipment.
func IsNetworkDevice(prompt string) (bool, string) {
	patterns := map[string]*regexp.Regexp{
		"cisco":    regexp.MustCompile(`(?i)(cisco|ios|nxos|asa)`),
		"juniper":  regexp.MustCompile(`(?i)(juniper|junos)`),
		"arista":   regexp.MustCompile(`(?i)(arista|eos)`),
		"paloalto": regexp.MustCompile(`(?i)(palo ?alto|pan-os)`),
		"fortinet": regexp.MustCompile(`(?i)(fortinet|fortigate|fortios)`),
		"mikrotik": regexp.MustCompile(`(?i)(mikrotik|routeros)`),
	}

	for vendor, pattern := range patterns {
		if pattern.MatchString(prompt) {
			return true, vendor
		}
	}

	return false, ""
}
