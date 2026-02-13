---
project: Cortex
component: Agents
phase: Design
date_created: 2026-02-04T16:57:49
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:21:13.373972
---

# Contributing to Cortex Coder Agent

Thank you for your interest in contributing to Cortex Coder Agent! This document provides guidelines for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Coding Guidelines](#coding-guidelines)
- [Testing Guidelines](#testing-guidelines)
- [Documentation](#documentation)
- [Submitting Changes](#submitting-changes)
- [Style Guide](#style-guide)

## Code of Conduct

We are committed to providing a welcoming and inclusive environment. Please:

- Be respectful and constructive
- Welcome newcomers and help them learn
- Focus on what is best for the community
- Show empathy and understanding

## Getting Started

### Prerequisites

- Go 1.24.2 or later
- Git
- Make (optional, but recommended)

### Initial Setup

```bash
# Fork the repository
# Clone your fork
git clone https://github.com/YOUR_USERNAME/cortex-coder-agent.git
cd cortex-coder-agent

# Add upstream remote
git remote add upstream https://github.com/RedClaus/cortex-coder-agent.git

# Install dependencies
go mod download

# Run tests
go test ./...
```

### Development Build

```bash
# Build for your platform
make build

# Or directly with go
go build -o coder ./cmd/coder

# Run the binary
./coder
```

## Development Workflow

### Branch Strategy

1. **Main Branch**: `main` - Production code
2. **Feature Branches**: `feature/your-feature-name`
3. **Bugfix Branches**: `bugfix/your-bugfix-name`
4. **Hotfix Branches**: `hotfix/your-hotfix-name`

### Workflow Steps

1. **Create a branch**

```bash
git checkout -b feature/your-feature-name
```

2. **Make your changes**

3. **Run tests**

```bash
go test ./...
```

4. **Lint and format**

```bash
go fmt ./...
go vet ./...
```

5. **Commit changes**

```bash
git add .
git commit -m "feat: add new feature"
```

6. **Push to your fork**

```bash
git push origin feature/your-feature-name
```

7. **Create Pull Request**

   - Describe your changes
   - Reference related issues
   - Ensure CI checks pass

### Commit Message Convention

Follow conventional commits:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Formatting
- `refactor`: Code restructuring
- `perf`: Performance improvement
- `test`: Test additions/changes
- `chore`: Maintenance tasks

**Examples:**

```
feat(editor): add syntax highlighting for Python files

fix(diff): correct line numbering in side-by-side view

docs(readme): update installation instructions
```

## Coding Guidelines

### Principles

1. **Simplicity**: Write simple, readable code
2. **Surgical Changes**: Small, focused changes
3. **Goal-Driven**: Code should solve a specific problem
4. **Test-Driven**: Write tests for new functionality
5. **Documentation**: Document public APIs

### Code Organization

```
pkg/
├── editor/          # Editor components
│   ├── buffer.go   # Buffer management
│   └── buffer_test.go
├── tui/            # TUI components
│   ├── app.go      # Main app model
│   ├── editor.go   # Editor panel
│   ├── diff.go     # Diff viewer
│   ├── changes.go  # Change management
│   ├── chat.go     # Chat panel
│   ├── browser.go  # File browser
│   ├── layout.go   # Layout manager
│   └── styles.go   # Styles/theming
└── skills/         # Skills system
```

### File Naming

- Use `snake_case` for files
- Match package structure to file organization
- Keep files focused (single responsibility)

### Go Conventions

- Use `gofmt` for formatting
- Run `go vet` for static analysis
- Use meaningful variable names
- Prefer short, focused functions
- Avoid premature optimization

### Error Handling

```go
// Good
file, err := os.Open(path)
if err != nil {
    return fmt.Errorf("failed to open file %s: %w", path, err)
}

// Bad
file, err := os.Open(path)
if err != nil {
    return err
}
```

### Context Usage

```go
// Always use context for external calls
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := client.SendRequest(ctx, req)
```

### Concurrency

```go
// Use channels for communication
done := make(chan error, 1)
go func() {
    done <- longRunningTask()
}()

// Select for timeout
select {
case err := <-done:
    // Handle result
case <-time.After(timeout):
    // Handle timeout
}
```

## Testing Guidelines

### Test Organization

```
package_name_test.go  # Package tests
# or
pkg/
├── component.go
└── component_test.go
```

### Test Structure

```go
func TestFeatureName(t *testing.T) {
    // Setup
    setup := func() {}
    teardown := func() {}
    setup()
    defer teardown()
    
    // Arrange
    input := "test input"
    expected := "expected output"
    
    // Act
    actual := SomeFunction(input)
    
    // Assert
    assert.Equal(t, expected, actual)
}
```

### Test Coverage

- Aim for 60%+ coverage
- Test public APIs
- Test error paths
- Test edge cases

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run with race detector
go test ./... -race

# Run specific package
go test ./pkg/editor

# Run specific test
go test ./pkg/editor -run TestBuffer
```

### Table-Driven Tests

```go
func TestParsing(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"valid", "input", "output", false},
        {"invalid", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := Parse(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("unexpected error")
            }
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Mocking

```go
// Use interfaces for mocking
type FileReader interface {
    Read(path string) ([]byte, error)
}

// Implement mock for testing
type MockReader struct {
    content string
    err     error
}

func (m *MockReader) Read(path string) ([]byte, error) {
    return []byte(m.content), m.err
}
```

## Documentation

### Code Comments

```go
// FunctionName does X and returns Y.
// Use this when Z.
func FunctionName(input string) (output string, err error) {
    // Implementation
}

// Complex algorithms need inline comments
// Step 1: Parse input
// Step 2: Process
// Step 3: Return result
```

### Public Documentation

- Update README.md for user-facing changes
- Update USAGE.md for new keybindings
- Add examples for new features
- Document breaking changes

### API Documentation

- Use godoc comments for exported functions
- Include usage examples
- Document error conditions
- Note performance characteristics

## Submitting Changes

### Pull Request Process

1. **Update documentation** (if needed)
2. **Add tests** for new functionality
3. **Run full test suite**
4. **Create PR** with:
   - Clear title and description
   - Reference related issues
   - Screenshots for UI changes
   - Testing instructions

### PR Checklist

- [ ] Code follows style guidelines
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] All tests passing
- [ ] No linting errors
- [ ] Commit messages follow convention

### Review Process

- Maintainers will review your PR
- Address review feedback
- Keep PRs focused and small
- Be responsive to comments

## Style Guide

### Naming Conventions

**Files:**
- `snake_case.go`
- `snake_case_test.go`

**Packages:**
- `lowercase` (no underscores)
- Single word when possible
- Descriptive of contents

**Variables/Functions:**
- `CamelCase` for exported
- `camelCase` for private
- Descriptive but concise

**Constants:**
- `UPPER_SNAKE_CASE`
- Group related constants

### Formatting

```bash
# Format code
go fmt ./...

# Format specific file
go fmt ./pkg/editor/buffer.go
```

### Linting

```bash
# Vet code
go vet ./...

# Use golangci-lint (optional)
golangci-lint run
```

### Code Structure

**Prefer composition over inheritance:**

```go
// Good
type Editor struct {
    buffer   *Buffer
    viewport *Viewport
    styles    *Styles
}

// Bad (avoid deep nesting)
type Editor struct {
    nested struct {
        deeply struct {
            nested interface{}
        }
    }
}
```

**Interface design:**

```go
// Small, focused interfaces
type Reader interface {
    Read([]byte) (int, error)
}

type Writer interface {
    Write([]byte) (int, error)
}
```

**Error handling:**

```go
// Always handle errors
file, err := os.Open(path)
if err != nil {
    return fmt.Errorf("context: %w", err)
}

defer file.Close()
```

### Performance

- Profile before optimizing
- Use benchmarks for critical paths
- Prefer simple solutions
- Cache expensive operations when appropriate

### Security

- Validate user input
- Sanitize file paths
- Use context for timeouts
- Handle errors gracefully

## Release Process

Releases follow semantic versioning: `MAJOR.MINOR.PATCH`

- **MAJOR**: Breaking changes
- **MINOR**: New features (backwards compatible)
- **PATCH**: Bug fixes (backwards compatible)

### Creating a Release

1. Update version in `version.go`
2. Update CHANGELOG.md
3. Tag release: `git tag v1.2.3`
4. Push tags: `git push --tags`
5. Create GitHub release

## Getting Help

- GitHub Issues: Bug reports and feature requests
- Discord: Community discussion
- Documentation: README.md and USAGE.md

## Recognition

Contributors will be recognized in:
- CONTRIBUTORS.md file
- Release notes
- Project README

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to Cortex Coder Agent! 🚀
