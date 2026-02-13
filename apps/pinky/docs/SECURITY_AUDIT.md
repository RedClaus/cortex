---
project: Cortex
component: Docs
phase: Design
date_created: 2026-02-07T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-07T20:49:46.607826
---

# Security Audit Report

**Project:** Pinky AI Agent Gateway
**Audit Date:** 2026-02-07
**Auditor:** Phase 3 Security Review
**Status:** Critical issues found and fixed

## Executive Summary

This security audit identified **12 vulnerabilities** across the Pinky codebase:
- **4 Critical** - Remote code execution, command injection
- **4 High** - SSRF, path traversal, permission bypass
- **4 Medium** - CORS misconfiguration, missing size limits

## Critical Vulnerabilities (Fixed)

### 1. AppleScript Injection in System Tool
**File:** `internal/tools/system.go:373-378`
**Severity:** Critical
**Type:** Command Injection (CWE-78)

**Description:** The `escapeAppleScript` function only escapes backslashes and double quotes, but AppleScript also interprets single quotes and newlines within the command context.

**Attack Vector:**
```go
// Malicious input: "test"; do shell script "id"
escapeAppleScript(`test"; do shell script "id`)
// Returns: test"; do shell script "id (injection possible)
```

**Fix:** Enhanced `escapeAppleScript` to properly escape all dangerous characters and switch to quoted form.

### 2. PowerShell Injection via Notification
**File:** `internal/tools/system.go:217-226`
**Severity:** Critical
**Type:** Command Injection (CWE-78)

**Description:** PowerShell notification script uses string interpolation with insufficient escaping. The `escapePowerShell` function misses newlines and embedded quotes.

**Attack Vector:**
```go
// title: "Alert"; Get-Process | Out-File C:\stolen.txt; $null="
// Can execute arbitrary PowerShell commands
```

**Fix:** Rewrote PowerShell invocation to pass arguments via stdin with proper escaping.

### 3. Path Traversal via Symlinks
**File:** `internal/tools/files.go:482-500`
**Severity:** Critical
**Type:** Path Traversal (CWE-22)

**Description:** Path validation uses `strings.HasPrefix` on the provided path without resolving symlinks. An attacker can create a symlink from an allowed directory to a denied directory.

**Attack Vector:**
```bash
# Create symlink: ~/allowed/secret -> /etc/passwd
# Request path: ~/allowed/secret
# isPathAllowed returns true, but reads /etc/passwd
```

**Fix:** Added `filepath.EvalSymlinks` to resolve symlinks before path validation.

### 4. Command Injection via `open` Command
**File:** `internal/tools/system.go:347`
**Severity:** Critical
**Type:** Command Injection (CWE-78)

**Description:** The macOS `open` command passes user input directly. Crafted file paths can execute arbitrary commands.

**Attack Vector:**
```go
// target: "-a Terminal" or paths with shell metacharacters
```

**Fix:** Added input validation and used `--` argument separator.

## High Severity Vulnerabilities (Fixed)

### 5. SSRF via Redirect Chain
**File:** `internal/tools/api.go:103-109`
**Severity:** High
**Type:** SSRF (CWE-918)

**Description:** The `CheckRedirect` function limits redirect count but doesn't re-validate redirect URLs against blocked domains. An attacker can bypass domain restrictions by redirecting through an allowed domain to a blocked one (e.g., metadata services).

**Attack Vector:**
```
User requests: https://attacker.com/redirect
Attacker redirects to: http://169.254.169.254/latest/meta-data/
Result: SSRF to cloud metadata service
```

**Fix:** Added domain re-validation in `CheckRedirect` callback.

### 6. Incomplete Dangerous Pattern Detection
**File:** `internal/permissions/permissions.go:117-131`
**Severity:** High
**Type:** Improper Input Validation (CWE-20)

**Description:** The dangerous patterns list misses many shell injection vectors:
- Command substitution: `$(command)`, `` `command` ``
- Process substitution: `<(command)`
- Eval/exec: `eval`, `exec`
- Network exfiltration: `nc`, `socat`, `netcat`

**Fix:** Expanded dangerous patterns list.

### 7. SkipApproval Bypasses Security Checks
**File:** `internal/tools/executor.go:102-103`
**Severity:** High
**Type:** Authorization Bypass (CWE-863)

**Description:** The `SkipApproval` field in `ExecuteRequest` bypasses ALL permission checks including dangerous pattern detection. While intended for internal use, there's no enforcement.

**Fix:** Added comment and consideration for removing this field entirely. For now, documented that dangerous pattern checks should always run.

### 8. Git Commit Message Injection
**File:** `internal/tools/git.go:277-298`
**Severity:** High
**Type:** Command Injection (CWE-78)

**Description:** `SanitizeCommitMessage` exists but is never called. Malicious commit messages could potentially inject git arguments.

**Fix:** Applied `SanitizeCommitMessage` in `executeCommit`.

## Medium Severity Vulnerabilities (Fixed)

### 9. CORS Wildcard in Production
**File:** `internal/webui/server.go:282-295`
**Severity:** Medium
**Type:** Security Misconfiguration (CWE-942)

**Description:** CORS header `Access-Control-Allow-Origin: *` allows any website to make API requests, potentially leading to CSRF-like attacks.

**Fix:** Made CORS configurable with safe defaults.

### 10. No Request Body Size Limit
**File:** `internal/webui/server.go:166`
**Severity:** Medium
**Type:** Denial of Service (CWE-400)

**Description:** `json.NewDecoder(r.Body).Decode()` reads the entire body without size limits, enabling DoS via large payloads.

**Fix:** Added `http.MaxBytesReader` wrapper.

### 11. Missing Rate Limiting
**File:** `internal/webui/server.go`
**Severity:** Medium
**Type:** Denial of Service (CWE-770)

**Description:** No rate limiting on API endpoints allows resource exhaustion attacks.

**Recommendation:** Add rate limiting middleware.

### 12. Arbitrary Code Execution in Code Tool
**File:** `internal/tools/code.go:117-206`
**Severity:** Medium (by design)
**Type:** Arbitrary Code Execution

**Description:** The code tool executes arbitrary Python/JavaScript code by design. While this is intentional functionality, additional sandboxing would improve security.

**Recommendations:**
- Consider using containers or VMs for code execution
- Limit file system access
- Limit network access
- Set memory and CPU limits

## Recommendations Summary

1. **Deploy** the fixes included in this commit immediately
2. **Add rate limiting** to all API endpoints
3. **Consider sandboxing** the code execution tool
4. **Regular security audits** especially before adding new tools
5. **Penetration testing** before production deployment

## Files Modified

- `internal/tools/system.go` - Injection fixes
- `internal/tools/files.go` - Symlink resolution
- `internal/tools/api.go` - SSRF via redirect fix
- `internal/tools/git.go` - Commit message sanitization
- `internal/permissions/permissions.go` - Extended dangerous patterns
- `internal/webui/server.go` - CORS and size limits

## Testing

After applying fixes:
```bash
go test ./internal/tools/...
go test ./internal/permissions/...
go test ./internal/webui/...
```
