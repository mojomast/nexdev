# Implementation Plan: Security and Robustness Improvements

## Overview

This implementation plan breaks down the security and robustness improvements into discrete, incremental tasks. Each task builds on previous work and includes testing to validate correctness. The plan follows a logical progression: security infrastructure first, then integration into existing components, followed by configuration and observability improvements.

## Tasks

- [x] 1. Create security infrastructure
  - Create the `internal/security` package with path sanitization, input validation, and audit logging
  - Implement core security primitives that will be used throughout the system
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 7.1, 7.2, 7.3, 7.4, 7.5, 10.1, 10.2, 10.3, 10.4, 10.5_

  - [x] 1.1 Implement PathSanitizer
    - Create `internal/security/path_sanitizer.go`
    - Implement `NewPathSanitizer(projectRoot string) (*PathSanitizer, error)`
    - Implement `ValidatePath(path string) (string, error)` with full validation logic
    - Implement `IsPathSafe(path string) bool` for boolean checks
    - Handle cross-platform path separators (Unix and Windows)
    - _Requirements: 1.1, 1.2, 1.3, 1.5_

  - [ ]* 1.2 Write property tests for PathSanitizer
    - **Property 1: Path containment invariant**
    - **Validates: Requirements 1.1, 1.3**

  - [ ]* 1.3 Write property tests for directory traversal rejection
    - **Property 2: Directory traversal rejection**
    - **Validates: Requirements 1.2**

  - [ ]* 1.4 Write property tests for path normalization
    - **Property 3: Path normalization consistency**
    - **Validates: Requirements 1.5**

  - [ ]* 1.5 Write unit tests for PathSanitizer edge cases
    - Test empty paths, ".", "..", absolute paths
    - Test Windows-style paths (C:\, \\server\share)
    - Test symbolic link handling
    - Test paths with special characters
    - _Requirements: 1.1, 1.2, 1.3, 1.5_

  - [x] 1.6 Implement InputValidator
    - Create `internal/security/input_validator.go`
    - Implement `NewInputValidator() *InputValidator`
    - Implement `ValidateProjectName(name string) error`
    - Implement `ValidateJSON(data []byte, schema interface{}) error`
    - Implement `ValidateFileContent(content string, maxSize int) error`
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_

  - [ ]* 1.7 Write property tests for InputValidator
    - **Property 13: Character whitelist enforcement**
    - **Property 14: Pattern matching correctness**
    - **Property 15: Schema validation correctness**
    - **Property 16: Validation error specificity**
    - **Property 17: Size limit enforcement**
    - **Validates: Requirements 7.1, 7.2, 7.3, 7.4, 7.5**

  - [x] 1.8 Implement AuditLogger
    - Create `internal/security/audit_logger.go`
    - Implement `NewAuditLogger(logPath string) (*AuditLogger, error)`
    - Implement `LogPathRejection(path string, reason string)`
    - Implement `LogFileOperation(operation string, path string, success bool)`
    - Implement `LogAuthFailure(provider string, error string)`
    - Implement `Close() error`
    - Use structured log format with timestamps
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5_

  - [ ]* 1.9 Write property tests for AuditLogger
    - **Property 18: Audit log completeness**
    - **Validates: Requirements 10.1, 10.2, 10.3, 10.5**

  - [ ]* 1.10 Write unit tests for AuditLogger
    - Test log file creation
    - Test log entry format
    - Test concurrent logging
    - Test log rotation (if implemented)
    - _Requirements: 10.4, 10.5_

- [x] 2. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 3. Integrate path sanitization into Task Executor
  - Modify the Task Executor to use PathSanitizer for all file operations
  - Add audit logging for file operations
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 10.3_

  - [x] 3.1 Modify TaskExecutor to use PathSanitizer
    - Update `internal/executor/task_executor.go`
    - Add `pathSanitizer *security.PathSanitizer` field
    - Add `auditLogger *security.AuditLogger` field
    - Update `NewTaskExecutor` to initialize security components
    - Modify `writeFile()` to call `writeFileSafe()`
    - _Requirements: 1.1, 1.3_

  - [x] 3.2 Implement writeFileSafe method
    - Create `writeFileSafe(file File) error` method
    - Validate path using PathSanitizer before writing
    - Log rejected paths to audit log
    - Log successful file operations to audit log
    - Return descriptive errors for validation failures
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 10.3_

  - [ ]* 3.3 Write integration tests for Task Executor path validation
    - Test file writes within project root succeed
    - Test file writes outside project root are rejected
    - Test audit log entries are created
    - Test error messages are descriptive
    - _Requirements: 1.1, 1.2, 1.4, 10.3_

- [x] 4. Implement structured logging
  - Create a structured logging package using Go's log/slog
  - Replace existing log calls with structured logging
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

  - [x] 4.1 Create logging package
    - Create `internal/logging/logger.go`
    - Implement `NewLogger(level slog.Level, output io.Writer) *Logger`
    - Implement wrapper methods: `Debug`, `Info`, `Warn`, `Error`
    - Implement `With(args ...any) *Logger` for contextual logging
    - Implement `SanitizeSensitive(value string) string` for API key redaction
    - _Requirements: 5.1, 5.2, 5.3, 5.5_

  - [ ]* 4.2 Write property tests for logging
    - **Property 9: Error log completeness**
    - **Property 10: API call log completeness**
    - **Property 11: Sensitive data sanitization**
    - **Validates: Requirements 5.2, 5.4, 5.5**

  - [x] 4.3 Integrate structured logging into Task Executor
    - Update `internal/executor/task_executor.go`
    - Replace existing log calls with structured logging
    - Add contextual fields (task_id, phase_id, project_id)
    - _Requirements: 5.2, 5.4_

  - [x] 4.4 Integrate structured logging into providers
    - Update `internal/provider/openai.go` and other providers
    - Log API calls with provider, model, tokens, duration
    - Sanitize API keys in log output
    - _Requirements: 5.2, 5.4, 5.5_

- [x] 5. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Improve architecture parsing with JSON
  - Modify Architecture Generator to use JSON parsing instead of text extraction
  - Add retry logic for malformed responses
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_

  - [x] 6.1 Define JSON schema types
    - Create `internal/design/architecture_json.go`
    - Define `ArchitectureJSON` struct with json tags
    - Define `ComponentJSON`, `DataFlowJSON`, etc.
    - Add validation functions for required fields
    - _Requirements: 2.2, 2.4_

  - [x] 6.2 Update architecture generation prompt
    - Modify `buildArchitecturePrompt()` in `internal/design/generator.go`
    - Add explicit JSON format instructions
    - Include JSON schema in prompt
    - Specify "no markdown code fences" requirement
    - _Requirements: 2.1_

  - [x] 6.3 Implement JSON parsing with fallback
    - Create `parseArchitectureJSON(response string) (*Architecture, error)`
    - Try direct JSON parsing first
    - If failed, try extracting JSON from markdown code fences
    - Validate all required fields are present
    - Convert JSON structs to Architecture struct
    - _Requirements: 2.2, 2.4_

  - [x] 6.4 Implement retry logic for malformed responses
    - Modify `GenerateArchitecture()` to retry on parse failures
    - Create clarification prompt requesting valid JSON
    - Limit retries to 2 attempts
    - Return clear error message after retries exhausted
    - _Requirements: 2.3, 2.5_

  - [ ]* 6.5 Write property tests for architecture parsing
    - **Property 4: JSON parsing correctness**
    - **Validates: Requirements 2.2, 2.4**

  - [ ]* 6.6 Write unit tests for architecture parsing
    - Test valid JSON architecture
    - Test JSON with missing fields
    - Test JSON in markdown code fences
    - Test retry logic with malformed JSON
    - Test error messages
    - _Requirements: 2.2, 2.3, 2.4, 2.5_

- [ ] 7. Implement rate-limit and quota tracking
  - Enhance providers to extract and store rate-limit information
  - Update State Store to persist rate-limit data
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

  - [ ] 7.1 Update Response struct with rate-limit fields
    - Modify `internal/provider/provider.go`
    - Add `RateLimitInfo *RateLimitInfo` field to Response
    - Add `QuotaInfo *QuotaInfo` field to Response
    - Update RateLimitInfo and QuotaInfo to use pointers for nullable fields
    - _Requirements: 4.1, 4.2, 4.4_

  - [ ] 7.2 Implement header extraction for OpenAI provider
    - Modify `internal/provider/openai.go`
    - Create `extractRateLimitInfo(headers http.Header) *RateLimitInfo`
    - Create `extractQuotaInfo(headers http.Header) *QuotaInfo`
    - Extract from headers: X-RateLimit-*, X-Quota-*
    - Return nil when headers are missing (not zero values)
    - Update `Call()` method to extract and include in Response
    - _Requirements: 4.1, 4.2, 4.4_

  - [ ] 7.3 Implement header extraction for Anthropic provider
    - Modify `internal/provider/anthropic.go`
    - Implement similar extraction logic for Anthropic headers
    - Handle Anthropic-specific header names
    - _Requirements: 4.1, 4.2, 4.4_

  - [ ] 7.4 Update State Store to persist rate-limit data
    - Verify `SaveRateLimit()` and `SaveQuota()` methods exist in `internal/state/store.go`
    - If missing, implement them
    - Ensure they handle nullable fields correctly
    - _Requirements: 4.3_

  - [ ] 7.5 Integrate rate-limit storage into providers
    - Update all provider `Call()` methods
    - After successful API call, save rate-limit info to State Store
    - Save quota info to State Store
    - Handle storage errors gracefully (log but don't fail)
    - _Requirements: 4.3_

  - [ ]* 7.6 Write property tests for rate-limit extraction
    - **Property 6: Header extraction accuracy**
    - **Property 7: Rate-limit persistence**
    - **Property 8: Null vs zero distinction**
    - **Validates: Requirements 4.1, 4.2, 4.3, 4.4**

  - [ ]* 7.7 Write unit tests for rate-limit tracking
    - Test header extraction with various formats
    - Test missing headers return nil
    - Test database persistence
    - Test retrieval from database
    - _Requirements: 4.1, 4.2, 4.3, 4.4_

  - [ ] 7.8 Add CLI command to display rate-limit status
    - Create or update `internal/cli/quota.go`
    - Add command to display current rate-limits for all providers
    - Format output in readable table
    - Show "unknown" for nil values
    - _Requirements: 4.5_

- [ ] 8. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 9. Enhance database concurrency handling
  - Add transaction retry logic to State Store
  - Ensure WAL mode is enabled
  - _Requirements: 6.1, 6.2, 6.3, 6.4_

  - [ ] 9.1 Verify WAL mode is enabled
    - Check `internal/state/store.go` open() method
    - Ensure `PRAGMA journal_mode = WAL` is executed
    - Add test to verify WAL mode is active
    - _Requirements: 6.1_

  - [ ] 9.2 Implement transaction retry helper
    - Create `executeWithRetry(fn func(*sql.Tx) error) error` method
    - Implement exponential backoff (100ms base delay)
    - Retry up to 3 times on database busy errors
    - Detect busy errors: "database is locked", "SQLITE_BUSY"
    - _Requirements: 6.3, 6.4_

  - [ ] 9.3 Refactor existing transaction code to use retry helper
    - Update `ResetProjectProgress()` to use `executeWithRetry`
    - Update `DeletePhase()` to use `executeWithRetry`
    - Update other transaction-based methods as needed
    - _Requirements: 6.3, 6.4_

  - [ ]* 9.4 Write property tests for concurrent operations
    - **Property 12: Concurrent operation safety**
    - **Validates: Requirements 6.2**

  - [ ]* 9.5 Write unit tests for retry logic
    - Test successful transaction
    - Test retry on busy error
    - Test failure after max retries
    - Test exponential backoff timing
    - _Requirements: 6.3, 6.4_

- [ ] 10. Implement non-interactive configuration
  - Add environment variable support for API keys
  - Add command-line flags for configuration
  - Implement configuration validation
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 9.1, 9.2, 9.3, 9.4, 9.5_

  - [ ] 10.1 Add environment variable support
    - Modify `internal/cli/init.go`
    - Check environment variables before prompting: `GEOFFRUSSY_<PROVIDER>_API_KEY`
    - Update `promptForAPIKeys()` to check env vars first
    - Skip prompt if env var is set
    - _Requirements: 3.2_

  - [ ] 10.2 Add command-line flags for API keys
    - Add flags: `--api-key-<provider>=<key>`
    - Add flag: `--non-interactive` to skip all prompts
    - Parse flags in `runInit()`
    - Apply flag values before checking env vars
    - _Requirements: 3.3, 3.4_

  - [ ] 10.3 Implement dynamic provider discovery
    - Use `provider.GetProviderNames()` to list available providers
    - Display all registered providers during init
    - Generate env var names dynamically
    - Generate flag names dynamically
    - _Requirements: 3.1_

  - [ ]* 10.4 Write property tests for provider discovery
    - **Property 5: Provider discovery completeness**
    - **Validates: Requirements 3.1**

  - [ ] 10.5 Implement configuration validation
    - Create `validateConfiguration(cfg *config.Config) []error` function
    - Check all API keys are non-empty
    - Check project root exists and is writable
    - Check database is accessible and not corrupted
    - Return all validation errors (not just first)
    - _Requirements: 9.1, 9.2, 9.3, 9.4_

  - [ ] 10.6 Add validate command
    - Create `internal/cli/validate.go`
    - Add `validateCmd` to root command
    - Run configuration validation
    - Display all errors with specific messages
    - Exit with non-zero code if validation fails
    - _Requirements: 9.5_

  - [ ]* 10.7 Write unit tests for configuration
    - Test environment variable precedence
    - Test flag parsing
    - Test non-interactive mode
    - Test validation logic
    - Test validate command
    - _Requirements: 3.2, 3.3, 3.4, 9.1, 9.2, 9.3, 9.4, 9.5_

- [ ] 11. Implement error recovery and graceful degradation
  - Add retry logic to providers
  - Improve error handling in Task Executor
  - Add partial architecture extraction
  - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

  - [ ] 11.1 Implement provider retry logic
    - Verify `RetryWithBackoff()` exists in `internal/provider/provider.go`
    - Ensure it's used in all provider `Call()` methods
    - Add `isRetryable(err error) bool` helper
    - Retry on 5xx errors, timeouts, rate limits
    - Use exponential backoff (1s base delay)
    - _Requirements: 8.1_

  - [ ] 11.2 Improve Task Executor error handling
    - Modify `ExecuteTask()` in `internal/executor/task_executor.go`
    - Continue processing remaining files if one file write fails
    - Log each file error but don't abort
    - Return aggregate error at end if any files failed
    - _Requirements: 8.4_

  - [ ] 11.3 Implement partial architecture extraction
    - Create `parseArchitectureWithFallback()` in `internal/design/generator.go`
    - Try full JSON parsing first
    - On failure, extract what sections are available
    - Return partial Architecture with warning error
    - Only fail completely if no sections can be extracted
    - _Requirements: 8.3_

  - [ ]* 11.4 Write unit tests for error recovery
    - Test provider retry on 5xx errors
    - Test Task Executor continues after file error
    - Test partial architecture extraction
    - Test database retry on lock errors
    - _Requirements: 8.1, 8.2, 8.3, 8.4_

- [ ] 12. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 13. Update documentation
  - Update README with new configuration options
  - Document security features
  - Add examples for non-interactive setup
  - _Requirements: All_

  - [ ] 13.1 Update README.md
    - Add section on security features
    - Document environment variables
    - Document command-line flags
    - Add examples for CI/CD setup
    - Document validate command

  - [ ] 13.2 Create SECURITY.md
    - Document path sanitization
    - Document audit logging
    - Document security best practices
    - Document how to report security issues

  - [ ] 13.3 Update CONTRIBUTING.md
    - Add guidelines for security-sensitive code
    - Document testing requirements
    - Document property-based testing approach

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
- Integration tests verify components work together correctly
- All security-critical code must have 100% test coverage
- The implementation follows Go best practices and idiomatic patterns

