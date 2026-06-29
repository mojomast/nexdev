# Contributing to Geoffrussy

Thank you for your interest in contributing to Geoffrussy! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct:

- Be respectful and inclusive
- Welcome newcomers and help them get started
- Focus on what is best for the community
- Show empathy towards other community members

## Getting Started

### Prerequisites

- Go 1.24 or later
- GCC (for SQLite compilation via go-sqlite3)
- Git
- Make
- Docker (optional, for containerized development)

### Setting Up Development Environment

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/geoffrussy.git
   cd geoffrussy
   ```

3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/yourusername/geoffrussy.git
   ```

4. Install dependencies:
   ```bash
   go mod download
   ```

5. Build the project:
   ```bash
   make build
   ```

6. Run tests:
   ```bash
   make test
   ```

### Using Docker for Development

```bash
# Start development container
docker-compose up -d geoffrussy-dev

# Enter the container
docker-compose exec geoffrussy-dev sh

# Inside container
make build
make test
```

## Development Workflow

### 1. Create a Branch

Always create a new branch for your work:

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `test/` - Test additions or modifications

### 2. Make Your Changes

- Write clear, concise commit messages
- Follow Go best practices and idioms
- Add tests for new functionality
- Update documentation as needed
- Keep commits focused and atomic

### 3. Test Your Changes

Before submitting, ensure all tests pass:

```bash
# Run all tests
make test

# Run specific test types
make test-unit
make test-property
make test-integration

# Check code formatting
make fmt

# Run linters
make lint

# Run go vet
make vet
```

### 4. Commit Your Changes

Write clear commit messages following this format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Test additions or modifications
- `chore`: Build process or auxiliary tool changes

Example:
```
feat(interview): add follow-up question generation

Implement LLM-powered follow-up question generation based on user
answers. The system now analyzes responses and generates intelligent
follow-up questions to gather more context.

Closes #123
```

### 5. Push and Create Pull Request

```bash
# Push to your fork
git push origin feature/your-feature-name

# Create pull request on GitHub
```

## Pull Request Guidelines

### Before Submitting

- [ ] All tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] Linters pass (`make lint`)
- [ ] Documentation is updated
- [ ] Commit messages are clear
- [ ] Branch is up to date with main

### Pull Request Template

When creating a PR, include:

1. **Description**: What does this PR do?
2. **Motivation**: Why is this change needed?
3. **Changes**: List of changes made
4. **Testing**: How was this tested?
5. **Screenshots**: If applicable
6. **Related Issues**: Link to related issues

Example:
```markdown
## Description
Implements the Interview Engine component with five-phase question flow.

## Motivation
Requirement 2.1-2.5 specify that Geoffrussy should conduct a five-phase
interview to gather project requirements.

## Changes
- Added InterviewEngine struct and interface
- Implemented five interview phases
- Added question templates
- Implemented state management for pause/resume

## Testing
- Added unit tests for each phase
- Added property test for interview data completeness
- Manually tested pause/resume functionality

## Related Issues
Closes #45
```

### Review Process

1. Maintainers will review your PR
2. Address any feedback or requested changes
3. Once approved, a maintainer will merge your PR

## Coding Standards

### Go Style Guide

Follow the [Effective Go](https://golang.org/doc/effective_go.html) guidelines and:

- Use `gofmt` for formatting
- Use meaningful variable and function names
- Write comments for exported functions and types
- Keep functions small and focused
- Avoid global state when possible

### Code Organization

```go
// Package documentation
package mypackage

import (
    // Standard library imports
    "fmt"
    "os"
    
    // Third-party imports
    "github.com/spf13/cobra"
    
    // Local imports
    "github.com/yourusername/geoffrussy/internal/state"
)

// Exported types and constants
type MyType struct {
    Field1 string
    Field2 int
}

// Exported functions
func NewMyType() *MyType {
    return &MyType{}
}

// Unexported helper functions
func helperFunction() {
    // ...
}
```

### Error Handling

Always handle errors explicitly:

```go
// Good
result, err := doSomething()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Bad
result, _ := doSomething()
```

### Testing Standards

#### Unit Tests

- Test file naming: `*_test.go`
- Test function naming: `TestFunctionName`
- Use table-driven tests when appropriate
- Test both success and error cases

Example:
```go
func TestInterviewEngine_AskQuestion(t *testing.T) {
    tests := []struct {
        name    string
        phase   Phase
        want    *Question
        wantErr bool
    }{
        {
            name:    "project essence phase",
            phase:   PhaseProjectEssence,
            want:    &Question{Text: "What problem are you solving?"},
            wantErr: false,
        },
        // More test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            engine := NewInterviewEngine()
            got, err := engine.AskQuestion(tt.phase)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("AskQuestion() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("AskQuestion() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

#### Property-Based Tests

- Use [gopter](https://github.com/leanovate/gopter)
- Minimum 100 iterations
- Reference design document property
- Tag format: `// Feature: geoffrey-ai-agent, Property N: <property text>`

#### Symlink-Aware Path Tests

Path sanitizer tests create real temp directories with symlinks. When writing tests
that involve file paths, use `filepath.EvalSymlinks(t.TempDir())` to canonicalize
the temp directory (important on macOS where `/tmp` -> `/private/tmp`).

### Adding a New Provider

Providers are registered dynamically via `internal/provider/registry.go`. To add a new provider:

1. Create `internal/provider/<name>.go` implementing the `Provider` interface.
2. Add a factory entry to the `Registry` map in `internal/provider/registry.go`.
3. Optionally add a display name entry in `providerDisplayName()` in `internal/cli/init.go`.
4. The CLI will automatically generate `--api-key-<name>` flags and `GEOFFRUSSY_<NAME>_API_KEY` env var support.
5. If the provider returns rate-limit or quota headers, override `GetRateLimitInfo()` / `GetQuotaInfo()` and call `SetStore()` to enable persistent storage.
6. Add tests in `internal/provider/<name>_test.go`.

Example:
```go
func TestProperty4_StatePreservationRoundTrip(t *testing.T) {
    // Feature: geoffrey-ai-agent, Property 4: State Preservation Round-Trip
    properties := gopter.NewProperties(nil)
    
    properties.Property("saving then loading state preserves data", prop.ForAll(
        func(state *ProjectState) bool {
            store := NewStateStore(":memory:")
            err := store.SaveState(state)
            if err != nil {
                return false
            }
            
            loaded, err := store.LoadState(state.ProjectID)
            if err != nil {
                return false
            }
            
            return reflect.DeepEqual(state, loaded)
        },
        genProjectState(),
    ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}
```

## Documentation

### Code Documentation

- Document all exported types, functions, and constants
- Use complete sentences
- Include examples for complex functionality

```go
// InterviewEngine conducts a five-phase interview to gather project requirements.
// It manages question flow, state persistence, and follow-up generation.
//
// Example:
//   engine := NewInterviewEngine()
//   result, err := engine.StartInterview()
//   if err != nil {
//       log.Fatal(err)
//   }
type InterviewEngine struct {
    // ...
}
```

### User Documentation

Update relevant documentation in `docs/` when:
- Adding new features
- Changing existing behavior
- Adding new commands or flags
- Modifying configuration options

## Issue Reporting

### Bug Reports

Include:
- Geoffrussy version (`geoffrussy version`)
- Operating system and version
- Go version (if building from source)
- Steps to reproduce
- Expected behavior
- Actual behavior
- Error messages or logs

### Feature Requests

Include:
- Clear description of the feature
- Use cases and motivation
- Proposed implementation (if any)
- Alternatives considered

## Questions?

- Open a [Discussion](https://github.com/yourusername/geoffrussy/discussions)
- Join our community chat (link TBD)
- Email the maintainers (email TBD)

## License

By contributing to Geoffrussy, you agree that your contributions will be licensed under the MIT License.
