// Package permissions handles tool execution approval and permission tiers
package permissions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Tier represents a permission level
type Tier string

const (
	TierUnrestricted Tier = "unrestricted" // Execute all tools automatically
	TierSome         Tier = "some"         // Auto-approve low risk, ask for high
	TierRestricted   Tier = "restricted"   // Ask before every execution
)

// RiskLevel indicates how risky a tool execution is
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// ApprovalStatus represents the state of an approval request
type ApprovalStatus string

const (
	StatusPending  ApprovalStatus = "pending"
	StatusApproved ApprovalStatus = "approved"
	StatusDenied   ApprovalStatus = "denied"
	StatusTimeout  ApprovalStatus = "timeout"
)

// Common errors
var (
	ErrApprovalDenied  = errors.New("approval denied by user")
	ErrApprovalTimeout = errors.New("approval request timed out")
	ErrDangerousCmd    = errors.New("command matches dangerous pattern")
	ErrApprovalPending = errors.New("approval request already pending")
	ErrNotFound        = errors.New("approval request not found")
)

// Service manages permissions and approvals
type Service struct {
	mu              sync.RWMutex
	tier            Tier
	approvals       map[string]*UserApprovals // keyed by userID
	path            string                    // path to approvals file
	pendingMu       sync.RWMutex
	pending         map[string]*PendingApproval // keyed by request ID
	dangerousPatterns []string                  // regex patterns for dangerous commands
	approvalTimeout time.Duration
}

// PendingApproval tracks an in-flight approval request
type PendingApproval struct {
	Request   *ApprovalRequest
	Response  chan *ApprovalResponse
	Status    ApprovalStatus
	CreatedAt time.Time
	ExpiresAt time.Time
}

// UserApprovals stores approval rules for a user
type UserApprovals struct {
	UserID    string                    `yaml:"user_id"`
	ToolRules map[string]*ToolApproval `yaml:"tools"`
}

// ToolApproval defines approval rules for a specific tool
type ToolApproval struct {
	AlwaysAllow       bool     `yaml:"always_allow"`
	AllowedPatterns   []string `yaml:"allowed_patterns,omitempty"`
	DeniedPatterns    []string `yaml:"denied_patterns,omitempty"`
	AllowedDirs       []string `yaml:"allowed_directories,omitempty"`
	DeniedDirs        []string `yaml:"denied_directories,omitempty"`
	AllowedDomains    []string `yaml:"allowed_domains,omitempty"` // for API tool
}

// ApprovalRequest represents a pending approval
type ApprovalRequest struct {
	ID         string         `json:"id"`
	UserID     string         `json:"user_id"`
	Tool       string         `json:"tool"`
	Command    string         `json:"command"`
	Args       map[string]any `json:"args,omitempty"`
	WorkingDir string         `json:"working_dir"`
	RiskLevel  RiskLevel      `json:"risk_level"`
	Reason     string         `json:"reason"`
	CreatedAt  time.Time      `json:"created_at"`
}

// ApprovalResponse is the user's decision
type ApprovalResponse struct {
	Approved       bool   `json:"approved"`
	AlwaysAllow    bool   `json:"always_allow"`      // Remember "always allow [tool]"
	AllowDir       bool   `json:"allow_dir"`         // Allow in this directory
	AllowPattern   string `json:"allow_pattern"`     // Allow matching pattern
	Modified       string `json:"modified"`          // If user edited the command
	DenyPattern    string `json:"deny_pattern"`      // Add to denied patterns
}

// DefaultDangerousPatterns are shell commands that should always be blocked or warned
var DefaultDangerousPatterns = []string{
	// Destructive file operations
	`rm\s+-rf\s+/[^.]`,         // rm -rf / (but allow relative paths)
	`rm\s+-rf\s+~`,             // rm -rf ~
	`rm\s+-rf\s+\$HOME`,        // rm -rf $HOME
	`rm\s+-rf\s+/\*`,           // rm -rf /*

	// Disk operations
	`>\s*/dev/sd[a-z]`,         // overwrite disk
	`dd\s+.*of=/dev/sd`,        // dd to disk
	`dd\s+.*of=/dev/nvme`,      // dd to NVMe
	`mkfs\.`,                   // format filesystem

	// Fork bombs and resource exhaustion
	`:\(\)\{\s*:\|:\&\s*\};:`,  // fork bomb
	`\.\s+/dev/zero`,           // read /dev/zero

	// Permission changes on root
	`chmod\s+-R\s+777\s+/`,     // chmod 777 /
	`chown\s+-R\s+.*\s+/[^.]`,  // chown /

	// Remote code execution via pipes
	`curl.*\|\s*sh`,            // curl pipe to shell
	`wget.*\|\s*sh`,            // wget pipe to shell
	`curl.*\|\s*bash`,          // curl pipe to bash
	`wget.*\|\s*bash`,          // wget pipe to bash
	`curl.*\|\s*python`,        // curl pipe to python
	`wget.*\|\s*python`,        // wget pipe to python

	// Command substitution and eval (potential injection vectors)
	`\beval\s+`,                // eval command
	`\bexec\s+`,                // exec command (replaces shell)
	`\$\(.*\)`,                 // command substitution
	"`[^`]+`",                  // backtick command substitution

	// Network exfiltration tools (when used suspiciously)
	`nc\s+-e`,                  // netcat with execute
	`nc\s+.*-l.*-e`,            // netcat listener with execute
	`ncat\s+-e`,                // ncat with execute

	// Credential theft
	`cat\s+.*\.ssh/`,           // reading SSH keys
	`cat\s+.*/etc/shadow`,      // reading shadow file
	`cat\s+.*/etc/passwd`,      // reading passwd (less critical but often part of recon)

	// History manipulation (covering tracks)
	`history\s+-c`,             // clear history
	`>\s+.*\.bash_history`,     // overwrite bash history
	`unset\s+HISTFILE`,         // disable history

	// Kernel/system manipulation
	`insmod\s+`,                // insert kernel module
	`modprobe\s+`,              // load kernel module
	`sysctl\s+-w`,              // write sysctl

	// Container escapes
	`docker\s+run.*--privileged`,       // privileged docker
	`docker\s+run.*-v\s+/:/`,           // mount root
	`nsenter\s+`,                       // namespace enter
}

// NewService creates a new permission service
func NewService(defaultTier Tier) *Service {
	home, _ := os.UserHomeDir()
	return &Service{
		tier:              defaultTier,
		approvals:         make(map[string]*UserApprovals),
		path:              filepath.Join(home, ".pinky", "approvals.yaml"),
		pending:           make(map[string]*PendingApproval),
		dangerousPatterns: DefaultDangerousPatterns,
		approvalTimeout:   5 * time.Minute,
	}
}

// NewServiceWithPath creates a new permission service with a custom path
func NewServiceWithPath(defaultTier Tier, path string) *Service {
	return &Service{
		tier:              defaultTier,
		approvals:         make(map[string]*UserApprovals),
		path:              path,
		pending:           make(map[string]*PendingApproval),
		dangerousPatterns: DefaultDangerousPatterns,
		approvalTimeout:   5 * time.Minute,
	}
}

// SetApprovalTimeout sets the timeout for approval requests
func (s *Service) SetApprovalTimeout(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvalTimeout = d
}

// AddDangerousPattern adds a pattern to the dangerous commands list
func (s *Service) AddDangerousPattern(pattern string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dangerousPatterns = append(s.dangerousPatterns, pattern)
}

// SetTier changes the permission tier
func (s *Service) SetTier(tier Tier) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tier = tier
}

// GetTier returns the current tier
func (s *Service) GetTier() Tier {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tier
}

// CheckResult represents the result of a permission check
type CheckResult struct {
	NeedsApproval bool
	AutoApproved  bool
	Blocked       bool
	BlockReason   string
	RiskLevel     RiskLevel
}

// Check performs a comprehensive permission check
func (s *Service) Check(userID, tool, command, workingDir string, riskLevel RiskLevel) *CheckResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := &CheckResult{RiskLevel: riskLevel}

	// First check for dangerous commands (always block)
	if s.isDangerousCommand(command) {
		result.Blocked = true
		result.BlockReason = "command matches dangerous pattern"
		return result
	}

	// Check user-specific denied patterns first
	if userApprovals, ok := s.approvals[userID]; ok {
		if toolApproval, ok := userApprovals.ToolRules[tool]; ok {
			// Check denied patterns
			if s.matchesPatternLocked(command, toolApproval.DeniedPatterns) {
				result.Blocked = true
				result.BlockReason = "command matches denied pattern"
				return result
			}
			// Check denied directories
			if s.matchesDirLocked(workingDir, toolApproval.DeniedDirs) {
				result.Blocked = true
				result.BlockReason = "execution in denied directory"
				return result
			}
		}
	}

	// Check tier
	switch s.tier {
	case TierUnrestricted:
		result.AutoApproved = true
		return result
	case TierRestricted:
		result.NeedsApproval = true
		return result
	case TierSome:
		// Check if user has always-allow for this tool
		if userApprovals, ok := s.approvals[userID]; ok {
			if toolApproval, ok := userApprovals.ToolRules[tool]; ok {
				if toolApproval.AlwaysAllow {
					result.AutoApproved = true
					return result
				}
				// Check allowed patterns
				if s.matchesPatternLocked(command, toolApproval.AllowedPatterns) {
					result.AutoApproved = true
					return result
				}
				// Check allowed directories
				if s.matchesDirLocked(workingDir, toolApproval.AllowedDirs) {
					result.AutoApproved = true
					return result
				}
			}
		}
		// For "some" tier, auto-approve low risk
		if riskLevel == RiskLow {
			result.AutoApproved = true
			return result
		}
		result.NeedsApproval = true
		return result
	}

	result.NeedsApproval = true
	return result
}

// NeedsApproval checks if a tool execution needs user approval (legacy compatibility)
func (s *Service) NeedsApproval(userID, tool, command string, riskLevel string) bool {
	risk := RiskLevel(riskLevel)
	result := s.Check(userID, tool, command, "", risk)
	return result.NeedsApproval || result.Blocked
}

// isDangerousCommand checks if a command matches dangerous patterns
func (s *Service) isDangerousCommand(command string) bool {
	for _, pattern := range s.dangerousPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		if re.MatchString(command) {
			return true
		}
	}
	return false
}

// IsDangerous checks if a command is dangerous (exported for UI warnings)
func (s *Service) IsDangerous(command string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isDangerousCommand(command)
}

// matchesPattern checks if command matches any pattern (acquires lock)
func (s *Service) matchesPattern(command string, patterns []string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.matchesPatternLocked(command, patterns)
}

// matchesPatternLocked checks if command matches any pattern (caller holds lock)
func (s *Service) matchesPatternLocked(command string, patterns []string) bool {
	for _, pattern := range patterns {
		// Try glob pattern first
		if matched, _ := filepath.Match(pattern, command); matched {
			return true
		}
		// Try as regex if it looks like one
		if strings.ContainsAny(pattern, "^$()[]{}+?\\") {
			if re, err := regexp.Compile(pattern); err == nil {
				if re.MatchString(command) {
					return true
				}
			}
		}
		// Try prefix match for simple patterns like "git *"
		if strings.HasSuffix(pattern, " *") {
			prefix := strings.TrimSuffix(pattern, " *")
			if strings.HasPrefix(command, prefix+" ") || command == prefix {
				return true
			}
		}
	}
	return false
}

// matchesDirLocked checks if a directory matches any allowed/denied directory (caller holds lock)
func (s *Service) matchesDirLocked(dir string, dirs []string) bool {
	if dir == "" {
		return false
	}
	// Resolve to absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	for _, d := range dirs {
		absD, err := filepath.Abs(d)
		if err != nil {
			absD = d
		}
		// Check if dir is under d (prefix match)
		if absDir == absD || strings.HasPrefix(absDir, absD+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// RecordApproval saves an approval decision
func (s *Service) RecordApproval(userID, tool string, response *ApprovalResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.approvals[userID]; !ok {
		s.approvals[userID] = &UserApprovals{
			UserID:    userID,
			ToolRules: make(map[string]*ToolApproval),
		}
	}

	if _, ok := s.approvals[userID].ToolRules[tool]; !ok {
		s.approvals[userID].ToolRules[tool] = &ToolApproval{}
	}

	if response.AlwaysAllow {
		s.approvals[userID].ToolRules[tool].AlwaysAllow = true
	}

	// Persist to disk
	s.save()
}

// Load reads approvals from disk
func (s *Service) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No file yet, that's OK
		}
		return err
	}

	return yaml.Unmarshal(data, &s.approvals)
}

// save persists approvals to disk
func (s *Service) save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(s.approvals)
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}

// generateRequestID creates a unique ID for approval requests
func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateApprovalRequest creates a new approval request and returns it
func (s *Service) CreateApprovalRequest(userID, tool, command, workingDir, reason string, riskLevel RiskLevel, args map[string]any) *ApprovalRequest {
	return &ApprovalRequest{
		ID:         generateRequestID(),
		UserID:     userID,
		Tool:       tool,
		Command:    command,
		Args:       args,
		WorkingDir: workingDir,
		RiskLevel:  riskLevel,
		Reason:     reason,
		CreatedAt:  time.Now(),
	}
}

// RequestApproval submits an approval request and waits for response
// This is the main entry point for the approval workflow
func (s *Service) RequestApproval(ctx context.Context, req *ApprovalRequest) (*ApprovalResponse, error) {
	// First check if this would be auto-approved or blocked
	result := s.Check(req.UserID, req.Tool, req.Command, req.WorkingDir, req.RiskLevel)

	if result.Blocked {
		return nil, ErrDangerousCmd
	}

	if result.AutoApproved {
		return &ApprovalResponse{Approved: true}, nil
	}

	// Create pending approval
	pending := &PendingApproval{
		Request:   req,
		Response:  make(chan *ApprovalResponse, 1),
		Status:    StatusPending,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.approvalTimeout),
	}

	s.pendingMu.Lock()
	s.pending[req.ID] = pending
	s.pendingMu.Unlock()

	// Clean up when done
	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, req.ID)
		s.pendingMu.Unlock()
	}()

	// Wait for response or timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(s.approvalTimeout):
		return nil, ErrApprovalTimeout
	case resp := <-pending.Response:
		if resp == nil {
			return nil, ErrApprovalDenied
		}
		// Record the approval preferences
		if resp.Approved {
			s.recordApprovalPreferences(req, resp)
		}
		if !resp.Approved {
			return nil, ErrApprovalDenied
		}
		return resp, nil
	}
}

// RespondToApproval processes a user's response to an approval request
func (s *Service) RespondToApproval(requestID string, response *ApprovalResponse) error {
	s.pendingMu.Lock()
	pending, ok := s.pending[requestID]
	if !ok {
		s.pendingMu.Unlock()
		return ErrNotFound
	}

	if pending.Status != StatusPending {
		s.pendingMu.Unlock()
		return ErrNotFound
	}

	if response.Approved {
		pending.Status = StatusApproved
	} else {
		pending.Status = StatusDenied
	}
	s.pendingMu.Unlock()

	// Send response (non-blocking since channel is buffered)
	select {
	case pending.Response <- response:
	default:
		// Channel full, already responded
	}

	return nil
}

// GetPendingApproval retrieves a pending approval request by ID
func (s *Service) GetPendingApproval(requestID string) (*ApprovalRequest, bool) {
	s.pendingMu.RLock()
	defer s.pendingMu.RUnlock()

	pending, ok := s.pending[requestID]
	if !ok || pending.Status != StatusPending {
		return nil, false
	}
	return pending.Request, true
}

// GetPendingApprovalsForUser returns all pending approvals for a user
func (s *Service) GetPendingApprovalsForUser(userID string) []*ApprovalRequest {
	s.pendingMu.RLock()
	defer s.pendingMu.RUnlock()

	var requests []*ApprovalRequest
	for _, pending := range s.pending {
		if pending.Request.UserID == userID && pending.Status == StatusPending {
			requests = append(requests, pending.Request)
		}
	}
	return requests
}

// recordApprovalPreferences updates the stored approval rules based on user response
func (s *Service) recordApprovalPreferences(req *ApprovalRequest, resp *ApprovalResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure user and tool entries exist
	if _, ok := s.approvals[req.UserID]; !ok {
		s.approvals[req.UserID] = &UserApprovals{
			UserID:    req.UserID,
			ToolRules: make(map[string]*ToolApproval),
		}
	}
	if _, ok := s.approvals[req.UserID].ToolRules[req.Tool]; !ok {
		s.approvals[req.UserID].ToolRules[req.Tool] = &ToolApproval{}
	}

	toolApproval := s.approvals[req.UserID].ToolRules[req.Tool]

	// Record "always allow" preference
	if resp.AlwaysAllow {
		toolApproval.AlwaysAllow = true
	}

	// Record directory preference
	if resp.AllowDir && req.WorkingDir != "" {
		if !contains(toolApproval.AllowedDirs, req.WorkingDir) {
			toolApproval.AllowedDirs = append(toolApproval.AllowedDirs, req.WorkingDir)
		}
	}

	// Record pattern preference
	if resp.AllowPattern != "" {
		if !contains(toolApproval.AllowedPatterns, resp.AllowPattern) {
			toolApproval.AllowedPatterns = append(toolApproval.AllowedPatterns, resp.AllowPattern)
		}
	}

	// Record denial pattern
	if resp.DenyPattern != "" {
		if !contains(toolApproval.DeniedPatterns, resp.DenyPattern) {
			toolApproval.DeniedPatterns = append(toolApproval.DeniedPatterns, resp.DenyPattern)
		}
	}

	// Persist to disk
	s.save()
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// CancelApproval cancels a pending approval request
func (s *Service) CancelApproval(requestID string) error {
	s.pendingMu.Lock()
	pending, ok := s.pending[requestID]
	if !ok {
		s.pendingMu.Unlock()
		return ErrNotFound
	}
	pending.Status = StatusDenied
	delete(s.pending, requestID)
	s.pendingMu.Unlock()

	// Close the response channel to unblock any waiters
	close(pending.Response)
	return nil
}

// CleanupExpired removes expired pending approvals
func (s *Service) CleanupExpired() {
	now := time.Now()
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	for id, pending := range s.pending {
		if now.After(pending.ExpiresAt) {
			pending.Status = StatusTimeout
			close(pending.Response)
			delete(s.pending, id)
		}
	}
}

// GetUserApprovals returns the approval rules for a user
func (s *Service) GetUserApprovals(userID string) *UserApprovals {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if approvals, ok := s.approvals[userID]; ok {
		return approvals
	}
	return nil
}

// SetToolApproval sets approval rules for a specific tool and user
func (s *Service) SetToolApproval(userID, tool string, approval *ToolApproval) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.approvals[userID]; !ok {
		s.approvals[userID] = &UserApprovals{
			UserID:    userID,
			ToolRules: make(map[string]*ToolApproval),
		}
	}
	s.approvals[userID].ToolRules[tool] = approval
	s.save()
}

// RevokeToolApproval removes all stored approvals for a tool
func (s *Service) RevokeToolApproval(userID, tool string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userApprovals, ok := s.approvals[userID]; ok {
		delete(userApprovals.ToolRules, tool)
		s.save()
	}
}

// RevokeAllApprovals clears all stored approvals for a user
func (s *Service) RevokeAllApprovals(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.approvals, userID)
	s.save()
}
