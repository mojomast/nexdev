# Design Document: Security and Robustness Improvements

## Overview

This design document outlines the implementation approach for critical security and robustness improvements to the geoffrussy project. The improvements address nine key areas: path sanitization, architecture parsing, provider configuration, rate-limit handling, logging, concurrency safety, input validation, error recovery, configuration validation, and audit logging.

The design follows Go best practices and leverages existing patterns in the codebase while introducing new security layers and robustness mechanisms. The implementation will be backward-compatible and will not break existing functionality.

### Key Design Decisions

1. **Security-First Approach**: All file operations will be validated before execution, with no exceptions
2. **Structured Data**: Replace text parsing with JSON-based structured data exchange
3. **Graceful Degradation**: System should continue operating when non-critical components fail
4. **Observable Operations**: All security-relevant operations will be logged for audit purposes
5. **Minimal Dependencies**: Use Go standard library where possible, add minimal external dependencies

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     CLI Layer                                │
│  - Configuration validation                                  │
│  - Non-interactive setup                                     │
│  - Environment variable support                              │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  Security Layer (NEW)                        │
│  - Path Sanitizer                                            │
│  - Input Validator                                           │
│  - Audit Logger                                              │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  Execution Layer                             │
│  - Task Executor (with path validation)                      │
│  - Architecture Generator (with JSON parsing)                │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  Provider Layer                              │
│  - Rate-limit tracking                                       │
│  - Quota monitoring                                          │
│  - Enhanced error handling                                   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  State Layer                                 │
│  - Concurrent access handling                                │
│  - Transaction retry logic                                   │
│  - Rate-limit/quota storage                                  │
└─────────────────────────────────────────────────────────────┘
```

### Security Boundaries

1. **File System Boundary**: All file writes must pass through path sanitization
2. **LLM Output Boundary**: All LLM responses must be validated before processing
3. **User Input Boundary**: All CLI inputs must be validated before use
4. **Database Boundary**: All database operations must handle concurrency safely

## Components and Interfaces

### 1. Path Sanitizer (NEW)

**Location**: `internal/security/path_sanitizer.go`

**Purpose**: Validate and sanitize file paths to prevent directory traversal attacks

**Interface**:
```go
type PathSanitizer struct {
    projectRoot string
}

func NewPathSanitizer(projectRoot string) (*PathSanitizer, error)
func (ps *PathSanitizer) ValidatePath(path string) (string, error)
func (ps *PathSanitizer) IsPathSafe(path string) bool
```

**Key Methods**:
- `ValidatePath(path string)`: Returns absolute path if safe, error otherwise
- `IsPathSafe(path string)`: Boolean check without error details

**Validation Rules**:
1. Resolve path to absolute form
2. Check if resolved path starts with project root
3. Reject paths containing `..` after resolution
4. Reject absolute paths that don't start with project root
5. Normalize path separators for cross-platform compatibility

### 2. Input Validator (NEW)

**Location**: `internal/security/input_validator.go`

**Purpose**: Validate all user inputs and LLM outputs

**Interface**:
```go
type InputValidator struct{}

func NewInputValidator() *InputValidator
func (iv *InputValidator) ValidateProjectName(name string) error
func (iv *InputValidator) ValidateJSON(data []byte, schema interface{}) error
func (iv *InputValidator) ValidateFileContent(content string, maxSize int) error
```

**Validation Rules**:
- Project names: alphanumeric, hyphens, underscores only
- JSON: must parse and match expected schema
- File content: size limits to prevent memory exhaustion

### 3. Audit Logger (NEW)

**Location**: `internal/security/audit_logger.go`

**Purpose**: Log security-relevant events for audit trail

**Interface**:
```go
type AuditLogger struct {
    logger *log.Logger
    file   *os.File
}

func NewAuditLogger(logPath string) (*AuditLogger, error)
func (al *AuditLogger) LogPathRejection(path string, reason string)
func (al *AuditLogger) LogFileOperation(operation string, path string, success bool)
func (al *AuditLogger) LogAuthFailure(provider string, error string)
func (al *AuditLogger) Close() error
```

**Log Format**:
```
[TIMESTAMP] [LEVEL] [COMPONENT] [OPERATION] key=value key=value
```

### 4. Enhanced Task Executor

**Location**: `internal/executor/task_executor.go` (modified)

**Changes**:
1. Add `PathSanitizer` field
2. Modify `writeFile()` to validate paths before writing
3. Add audit logging for file operations
4. Improve error messages with security context

**New Method**:
```go
func (te *TaskExecutor) writeFileSafe(file File) error {
    // Validate path
    safePath, err := te.pathSanitizer.ValidatePath(file.Path)
    if err != nil {
        te.auditLogger.LogPathRejection(file.Path, err.Error())
        return fmt.Errorf("path validation failed: %w", err)
    }
    
    // Write file
    // ... existing logic with safePath
    
    te.auditLogger.LogFileOperation("write", safePath, true)
    return nil
}
```

### 5. Enhanced Architecture Generator

**Location**: `internal/design/generator.go` (modified)

**Changes**:
1. Modify prompt to explicitly request JSON output
2. Replace text parsing with JSON unmarshaling
3. Add retry logic for malformed responses
4. Validate parsed architecture against schema

**New Prompt Structure**:
```
You are an expert software architect. Generate a system architecture in JSON format.

OUTPUT FORMAT (REQUIRED):
Return ONLY valid JSON matching this schema:
{
  "system_overview": "string",
  "components": [
    {
      "name": "string",
      "type": "frontend|backend|database|cache|queue|monitoring",
      "purpose": "string",
      "technologies": ["string"],
      "dependencies": ["string"]
    }
  ],
  ...
}

Do not include markdown code fences or any text outside the JSON object.
```

**New Method**:
```go
func (g *Generator) parseArchitectureJSON(response string) (*Architecture, error) {
    var archData ArchitectureJSON
    
    // Try direct JSON parsing
    if err := json.Unmarshal([]byte(response), &archData); err != nil {
        // Try extracting JSON from markdown code fence
        cleaned := extractJSONFromMarkdown(response)
        if err := json.Unmarshal([]byte(cleaned), &archData); err != nil {
            return nil, fmt.Errorf("failed to parse architecture JSON: %w", err)
        }
    }
    
    // Validate required fields
    if err := validateArchitectureData(&archData); err != nil {
        return nil, fmt.Errorf("invalid architecture data: %w", err)
    }
    
    // Convert to Architecture struct
    return convertToArchitecture(&archData), nil
}
```

### 6. Enhanced Provider Interface

**Location**: `internal/provider/provider.go` (modified)

**Changes**:
1. Update `Response` struct to include rate-limit and quota data
2. Modify all provider implementations to extract header information
3. Store rate-limit/quota data in State_Store after each call

**Updated Response Struct**:
```go
type Response struct {
    Content            string
    TokensInput        int
    TokensOutput       int
    Model              string
    Provider           string
    Timestamp          time.Time
    RateLimitInfo      *RateLimitInfo  // NEW
    QuotaInfo          *QuotaInfo      // NEW
}
```

**Provider Implementation Pattern**:
```go
func (p *OpenAIProvider) Call(model string, prompt string) (*Response, error) {
    // ... existing call logic ...
    
    // Extract rate-limit info from headers
    rateLimitInfo := extractRateLimitInfo(resp.Header)
    quotaInfo := extractQuotaInfo(resp.Header)
    
    response := &Response{
        // ... existing fields ...
        RateLimitInfo: rateLimitInfo,
        QuotaInfo:     quotaInfo,
    }
    
    // Store in database
    if rateLimitInfo != nil {
        p.store.SaveRateLimit(p.Name(), rateLimitInfo)
    }
    if quotaInfo != nil {
        p.store.SaveQuota(p.Name(), quotaInfo)
    }
    
    return response, nil
}
```

### 7. Enhanced State Store

**Location**: `internal/state/store.go` (modified)

**Changes**:
1. Add transaction retry logic with exponential backoff
2. Ensure WAL mode is enabled (already done)
3. Add connection pooling configuration
4. Implement database busy error handling

**New Method**:
```go
func (s *Store) executeWithRetry(fn func(*sql.Tx) error) error {
    maxRetries := 3
    baseDelay := 100 * time.Millisecond
    
    for attempt := 0; attempt <= maxRetries; attempt++ {
        tx, err := s.db.Begin()
        if err != nil {
            if isBusyError(err) && attempt < maxRetries {
                time.Sleep(baseDelay * time.Duration(math.Pow(2, float64(attempt))))
                continue
            }
            return err
        }
        
        err = fn(tx)
        if err != nil {
            tx.Rollback()
            if isBusyError(err) && attempt < maxRetries {
                time.Sleep(baseDelay * time.Duration(math.Pow(2, float64(attempt))))
                continue
            }
            return err
        }
        
        if err := tx.Commit(); err != nil {
            if isBusyError(err) && attempt < maxRetries {
                time.Sleep(baseDelay * time.Duration(math.Pow(2, float64(attempt))))
                continue
            }
            return err
        }
        
        return nil
    }
    
    return fmt.Errorf("transaction failed after %d retries", maxRetries)
}

func isBusyError(err error) bool {
    return strings.Contains(err.Error(), "database is locked") ||
           strings.Contains(err.Error(), "SQLITE_BUSY")
}
```

### 8. Enhanced CLI Configuration

**Location**: `internal/cli/init.go` (modified)

**Changes**:
1. Add environment variable support for API keys
2. Add command-line flags for non-interactive mode
3. Implement configuration validation
4. Support dynamic provider discovery

**Environment Variables**:
```
GEOFFRUSSY_OPENAI_API_KEY
GEOFFRUSSY_ANTHROPIC_API_KEY
GEOFFRUSSY_FIRMWARE_API_KEY
... (one per provider)
```

**New Flags**:
```
--non-interactive: Skip all prompts, use env vars or fail
--api-key-<provider>=<key>: Set API key via flag
--validate-only: Validate configuration without making changes
```

**Configuration Validation**:
```go
func validateConfiguration(cfg *config.Config) []error {
    var errors []error
    
    // Check API keys
    if len(cfg.APIKeys) == 0 {
        errors = append(errors, fmt.Errorf("no API keys configured"))
    }
    
    for provider, key := range cfg.APIKeys {
        if key == "" {
            errors = append(errors, fmt.Errorf("empty API key for provider: %s", provider))
        }
    }
    
    // Check project root
    if _, err := os.Stat(cfg.ProjectRoot); os.IsNotExist(err) {
        errors = append(errors, fmt.Errorf("project root does not exist: %s", cfg.ProjectRoot))
    }
    
    // Check database
    if cfg.DBPath != "" {
        store, err := state.NewStore(cfg.DBPath)
        if err != nil {
            errors = append(errors, fmt.Errorf("database error: %w", err))
        } else {
            if err := store.HealthCheck(); err != nil {
                errors = append(errors, fmt.Errorf("database health check failed: %w", err))
            }
            store.Close()
        }
    }
    
    return errors
}
```

### 9. Structured Logging

**Location**: `internal/logging/logger.go` (NEW)

**Purpose**: Provide structured logging throughout the application

**Implementation**: Use `log/slog` from Go 1.21+ standard library

**Interface**:
```go
type Logger struct {
    slog *slog.Logger
}

func NewLogger(level slog.Level, output io.Writer) *Logger
func (l *Logger) Debug(msg string, args ...any)
func (l *Logger) Info(msg string, args ...any)
func (l *Logger) Warn(msg string, args ...any)
func (l *Logger) Error(msg string, args ...any)
func (l *Logger) With(args ...any) *Logger
```

**Usage Pattern**:
```go
logger := logging.NewLogger(slog.LevelInfo, os.Stdout)

logger.Info("API call completed",
    "provider", "openai",
    "model", "gpt-4",
    "tokens_input", 1500,
    "tokens_output", 800,
    "duration_ms", 2340,
)
```

## Data Models

### ArchitectureJSON (NEW)

```go
type ArchitectureJSON struct {
    SystemOverview   string              `json:"system_overview"`
    Components       []ComponentJSON     `json:"components"`
    DataFlows        []DataFlowJSON      `json:"data_flows"`
    TechRationale    map[string]string   `json:"tech_rationale"`
    ScalingStrategy  ScalingPlanJSON     `json:"scaling_strategy"`
    APIContract      APISpecJSON         `json:"api_contract"`
    DatabaseSchema   SchemaJSON          `json:"database_schema"`
    SecurityApproach SecurityPlanJSON    `json:"security_approach"`
    Observability    ObservabilityJSON   `json:"observability"`
    Deployment       DeploymentPlanJSON  `json:"deployment"`
    Risks            []RiskJSON          `json:"risks"`
    Assumptions      []string            `json:"assumptions"`
    Unknowns         []string            `json:"unknowns"`
}

type ComponentJSON struct {
    Name         string   `json:"name"`
    Type         string   `json:"type"`
    Purpose      string   `json:"purpose"`
    Technologies []string `json:"technologies"`
    Dependencies []string `json:"dependencies"`
}

// ... similar JSON structs for other types
```

### AuditLogEntry (NEW)

```go
type AuditLogEntry struct {
    Timestamp time.Time
    Level     string
    Component string
    Operation string
    Details   map[string]string
    Success   bool
}
```

### Enhanced RateLimitInfo

```go
type RateLimitInfo struct {
    Provider          string
    RequestsRemaining *int       // Pointer to distinguish between 0 and unknown
    RequestsLimit     *int       // Pointer to distinguish between 0 and unknown
    ResetAt           *time.Time // Pointer for optional field
    RetryAfter        *time.Duration
    CheckedAt         time.Time
}
```

### Enhanced QuotaInfo

```go
type QuotaInfo struct {
    Provider        string
    TokensRemaining *int       // Pointer to distinguish between 0 and unknown
    TokensLimit     *int       // Pointer to distinguish between 0 and unknown
    CostRemaining   *float64   // Pointer to distinguish between 0 and unknown
    CostLimit       *float64   // Pointer to distinguish between 0 and unknown
    ResetAt         *time.Time
    CheckedAt       time.Time
}
```



## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Path Sanitization Properties

**Property 1: Path containment invariant**
*For any* file path provided to the Task_Executor, if the path passes validation, then its resolved absolute path must start with the Project_Root directory.
**Validates: Requirements 1.1, 1.3**

**Property 2: Directory traversal rejection**
*For any* file path containing directory traversal sequences (../, ..\) or absolute paths outside Project_Root, the PathSanitizer must reject the path and return an error.
**Validates: Requirements 1.2**

**Property 3: Path normalization consistency**
*For any* two equivalent file paths (e.g., "a/b" and "a//b" or "a/./b"), normalization must produce identical results.
**Validates: Requirements 1.5**

### Architecture Parsing Properties

**Property 4: JSON parsing correctness**
*For any* valid JSON architecture response from an LLM, parsing must succeed and produce a structured Architecture object with all required fields populated.
**Validates: Requirements 2.2, 2.4**

### Provider Configuration Properties

**Property 5: Provider discovery completeness**
*For any* provider registered in the provider registry, dynamic discovery must include that provider in the returned list.
**Validates: Requirements 3.1**

### Rate-Limit and Quota Properties

**Property 6: Header extraction accuracy**
*For any* HTTP response containing rate-limit or quota headers, the Provider must correctly extract and parse all available information.
**Validates: Requirements 4.1, 4.2**

**Property 7: Rate-limit persistence**
*For any* API call that returns rate-limit information, querying the State_Store immediately after should return the same rate-limit values.
**Validates: Requirements 4.3**

**Property 8: Null vs zero distinction**
*For any* HTTP response missing rate-limit or quota headers, the Provider must return nil pointers rather than zero values.
**Validates: Requirements 4.4**

### Logging Properties

**Property 9: Error log completeness**
*For any* error that occurs in the system, the log entry must contain component name, operation name, and relevant identifiers.
**Validates: Requirements 5.2**

**Property 10: API call log completeness**
*For any* API call made to a Provider, the log entry must include provider name, model name, token counts, and duration.
**Validates: Requirements 5.4**

**Property 11: Sensitive data sanitization**
*For any* log entry, sensitive information (API keys, passwords) must be redacted or masked before output.
**Validates: Requirements 5.5**

### Concurrency Properties

**Property 12: Concurrent operation safety**
*For any* set of concurrent database operations, the final state must be equivalent to some sequential execution of those operations (serializability).
**Validates: Requirements 6.2**

### Input Validation Properties

**Property 13: Character whitelist enforcement**
*For any* file path containing characters outside the allowed set, validation must reject the path with a specific error.
**Validates: Requirements 7.1**

**Property 14: Pattern matching correctness**
*For any* project name or identifier, validation must correctly determine whether it matches the expected pattern (alphanumeric, hyphens, underscores only).
**Validates: Requirements 7.2**

**Property 15: Schema validation correctness**
*For any* JSON input, schema validation must correctly identify whether the JSON conforms to the expected schema.
**Validates: Requirements 7.3**

**Property 16: Validation error specificity**
*For any* validation failure, the error message must identify the specific validation rule that was violated.
**Validates: Requirements 1.4, 7.4**

**Property 17: Size limit enforcement**
*For any* LLM response exceeding the configured size limit, the system must reject the response before processing.
**Validates: Requirements 7.5**

### Audit Logging Properties

**Property 18: Audit log completeness**
*For any* security-relevant operation (path rejection, auth failure, file operation), an audit log entry must be created containing timestamp, operation type, details, and outcome.
**Validates: Requirements 10.1, 10.2, 10.3, 10.5**

## Error Handling

### Error Categories

1. **Security Errors**: Path validation failures, authentication failures
   - Action: Reject operation, log to audit log, return descriptive error
   - No retry

2. **Transient Errors**: Network timeouts, database locks, rate limits
   - Action: Retry with exponential backoff
   - Max retries: 3
   - Base delay: 100ms

3. **Validation Errors**: Invalid input, malformed JSON, schema violations
   - Action: Return specific error message, no retry
   - Log at WARN level

4. **System Errors**: Database corruption, disk full, out of memory
   - Action: Log at ERROR level, fail fast
   - No retry

### Error Recovery Strategies

**API Call Failures**:
```go
func (p *Provider) callWithRetry(model, prompt string) (*Response, error) {
    var lastErr error
    
    for attempt := 0; attempt < 3; attempt++ {
        resp, err := p.call(model, prompt)
        if err == nil {
            return resp, nil
        }
        
        lastErr = err
        
        // Check if error is retryable
        if !isRetryable(err) {
            return nil, err
        }
        
        // Exponential backoff
        delay := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
        time.Sleep(delay)
    }
    
    return nil, fmt.Errorf("failed after 3 retries: %w", lastErr)
}

func isRetryable(err error) bool {
    // 5xx errors, timeouts, rate limits
    return strings.Contains(err.Error(), "timeout") ||
           strings.Contains(err.Error(), "5") ||
           strings.Contains(err.Error(), "rate limit")
}
```

**Database Lock Handling**:
```go
func (s *Store) executeWithRetry(fn func(*sql.Tx) error) error {
    for attempt := 0; attempt < 3; attempt++ {
        tx, err := s.db.Begin()
        if err != nil {
            if isDatabaseBusy(err) && attempt < 2 {
                time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
                continue
            }
            return err
        }
        
        err = fn(tx)
        if err != nil {
            tx.Rollback()
            if isDatabaseBusy(err) && attempt < 2 {
                time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
                continue
            }
            return err
        }
        
        if err := tx.Commit(); err != nil {
            if isDatabaseBusy(err) && attempt < 2 {
                time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
                continue
            }
            return err
        }
        
        return nil
    }
    
    return fmt.Errorf("transaction failed after 3 retries")
}
```

**Partial Architecture Parsing**:
```go
func (g *Generator) parseArchitectureWithFallback(response string) (*Architecture, error) {
    // Try full JSON parsing
    arch, err := g.parseArchitectureJSON(response)
    if err == nil {
        return arch, nil
    }
    
    // Try partial extraction
    arch = &Architecture{}
    
    // Extract what we can from text
    if overview := extractSection(response, "SYSTEM OVERVIEW"); overview != "" {
        arch.SystemOverview = overview
    }
    
    if len(arch.SystemOverview) > 0 {
        // Return partial architecture with warning
        return arch, fmt.Errorf("partial architecture extracted: %w", err)
    }
    
    // Complete failure
    return nil, err
}
```

### Error Message Guidelines

1. **Be Specific**: Include what failed, why it failed, and what was expected
2. **Include Context**: Add relevant identifiers (file path, provider name, etc.)
3. **Suggest Actions**: When possible, suggest how to fix the issue
4. **No Sensitive Data**: Never include API keys or user data in error messages

**Examples**:
```
❌ Bad:  "validation failed"
✅ Good: "path validation failed: '/etc/passwd' is outside project root '/home/user/project'"

❌ Bad:  "API error"
✅ Good: "OpenAI API call failed: rate limit exceeded (429), retry after 60s"

❌ Bad:  "invalid input"
✅ Good: "project name 'my project!' is invalid: only alphanumeric, hyphens, and underscores allowed"
```

## Testing Strategy

### Dual Testing Approach

This feature requires both unit tests and property-based tests to ensure comprehensive coverage:

- **Unit tests**: Verify specific examples, edge cases, and error conditions
- **Property tests**: Verify universal properties across all inputs

Both testing approaches are complementary and necessary. Unit tests catch concrete bugs in specific scenarios, while property tests verify general correctness across a wide range of inputs.

### Property-Based Testing

We will use the `testing/quick` package from Go's standard library for property-based testing, supplemented with custom generators for domain-specific types.

**Configuration**:
- Minimum 100 iterations per property test
- Each test must reference its design document property
- Tag format: `// Feature: security-robustness-improvements, Property N: [property text]`

**Example Property Test**:
```go
// Feature: security-robustness-improvements, Property 1: Path containment invariant
func TestProperty_PathContainmentInvariant(t *testing.T) {
    projectRoot := "/home/user/project"
    sanitizer, err := security.NewPathSanitizer(projectRoot)
    require.NoError(t, err)
    
    config := &quick.Config{MaxCount: 100}
    
    property := func(relativePath string) bool {
        // Generate valid relative paths
        if !isValidRelativePath(relativePath) {
            return true // Skip invalid inputs
        }
        
        safePath, err := sanitizer.ValidatePath(relativePath)
        if err != nil {
            // Validation can fail, that's ok
            return true
        }
        
        // Property: validated path must start with project root
        return strings.HasPrefix(safePath, projectRoot)
    }
    
    if err := quick.Check(property, config); err != nil {
        t.Error(err)
    }
}
```

### Unit Testing Focus Areas

1. **Path Sanitization**:
   - Test specific traversal attempts: `../../../etc/passwd`
   - Test absolute paths: `/etc/passwd`
   - Test Windows paths: `C:\Windows\System32`
   - Test edge cases: empty string, `.`, `..`

2. **Architecture Parsing**:
   - Test valid JSON architecture
   - Test JSON with missing fields
   - Test JSON wrapped in markdown code fences
   - Test completely invalid JSON

3. **Provider Configuration**:
   - Test environment variable precedence
   - Test flag parsing
   - Test interactive vs non-interactive modes
   - Test configuration file loading

4. **Rate-Limit Extraction**:
   - Test various header formats (OpenAI, Anthropic, etc.)
   - Test missing headers
   - Test malformed header values

5. **Database Concurrency**:
   - Test concurrent reads
   - Test concurrent writes
   - Test transaction conflicts
   - Test retry logic

6. **Input Validation**:
   - Test valid inputs
   - Test boundary cases
   - Test invalid characters
   - Test size limits

7. **Audit Logging**:
   - Test log entry format
   - Test sensitive data redaction
   - Test log file creation and rotation

### Integration Testing

Integration tests will verify that components work together correctly:

1. **End-to-End Path Validation**: LLM output → Task Executor → File System
2. **Architecture Generation Flow**: Interview Data → LLM → JSON Parsing → Storage
3. **Provider Rate-Limit Flow**: API Call → Header Extraction → Database Storage → CLI Display
4. **Configuration Flow**: Environment Variables → CLI → Provider Setup → Validation

### Test Coverage Goals

- **Line Coverage**: Minimum 80% for new code
- **Branch Coverage**: Minimum 75% for new code
- **Property Tests**: All 18 correctness properties must have corresponding tests
- **Security Tests**: 100% coverage of security-critical paths (path validation, input sanitization)

### Testing Tools

- **Unit Testing**: Go's built-in `testing` package
- **Property Testing**: `testing/quick` package
- **Assertions**: `github.com/stretchr/testify` for readable assertions
- **Mocking**: `github.com/stretchr/testify/mock` for provider mocks
- **Coverage**: `go test -cover` and `go tool cover`

