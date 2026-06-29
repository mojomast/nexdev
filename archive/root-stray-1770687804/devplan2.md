AI-Friendly Developer Plan: Security & Robustness

Status conventions (update `Status:` to mark progress):
- `pending` - task not started
- `in_progress` - actively working
- `blocked` - waiting on external input or resources
- `completed` - finished and verified

How an AI coding agent should use this file:
- Update the top-level task `Status:` and each step's `status` as work progresses.
- After running the `verify` command for a step, set `verified: true` when it passes.
- Keep `notes` short and machine-friendly (single-line or JSON-friendly lists).

Tasks:

- id: T1
  title: Finish Security Infrastructure
  Status: completed
  owner: ai-agent
  estimate: 3-6h
  files:
    - internal/security/pathsanitizer.go
    - internal/security/validator.go
    - internal/logging/auditlogger.go
    - testing/pathsanitizer_test.go
  steps:
    - id: T1.1
      action: Add property tests for PathSanitizer using testing/quick
      status: completed
      verified: true
      verify: go test ./... -run TestPathSanitizer -v
      checks:
        - Path containment: validated path starts with project root
        - Reject ".." components
        - Normalization consistency for equivalent inputs
    - id: T1.2
      action: Add unit tests for edge cases (empty, ".", "..", absolute outside root, mixed slashes)
      status: completed
      verified: true
      verify: go test ./... -run TestPathSanitizerEdgeCases -v
    - id: T1.3
      action: Test symlink behavior with tempdir and symlink pointing outside
      status: completed
      verified: true
      verify: go test ./... -run TestPathSanitizerSymlink -v
    - id: T1.4
      action: Add property and unit tests for InputValidator (alnum, hyphen, underscore, JSON schema, file size, invalid UTF-8)
      status: completed
      verified: true
      verify: go test ./... -run TestInputValidator -v
    - id: T1.5
      action: Add audit logger tests (concurrent logging, no sensitive data, rotation)
      status: completed
      verified: true
      verify: go test ./... -run TestAuditLogger -v
  notes:
    - run_commands: ["go test ./..."]
    - All security tests passing: 17 test suites, 100+ test cases
    - Added property-based tests with testing/quick for PathSanitizer

- id: T2
  title: Integrate Security Checks into Task Executor
  Status: completed
  estimate: 2-4h
  files:
    - internal/executor/taskexecutor.go
    - internal/executor/taskexecutor_test.go
  steps:
    - id: T2.1
      action: Add integration tests that attempt writes inside/outside project root and assert audit logs
      status: completed
      verified: true
      verify: go test ./internal/executor -run TestTaskExecutorPathValidation -v
    - id: T2.2
      action: Ensure error messages include offending path and rejection reason
      status: completed
      verified: true
      verify: go test ./... -run TestTaskExecutorErrorMessages -v
  notes:
    - Executor tests passing with path validation and audit logging
    - Error messages include offending path and rejection reason
    - Path validation works for both relative and absolute paths

- id: T3
  title: Finish Architecture JSON Parsing
  Status: completed
  estimate: 3-5h
  files:
    - internal/design/architecture_json.go
    - internal/design/parser_test.go
  steps:
    - id: T3.1
      action: Implement ArchitectureJSON structs and parseArchitectureJSON(response, projectID)
      status: completed
      verified: true
      verify: go test ./internal/design -run TestParseArchitectureJSON -v
    - id: T3.2
      action: Support extracting JSON from markdown fences when unmarshalling fails
      status: completed
      verified: true
    - id: T3.3
      action: Validate required fields and convert to Architecture
      status: completed
      verified: true
    - id: T3.4
      action: Add unit and property tests (valid JSON, fenced JSON, missing fields)
      status: completed
      verified: true
  notes:
    - All architecture JSON parsing tests passing
    - Supports direct JSON and markdown code fences
    - Validates required fields and converts to Architecture struct
    - Tests cover valid JSON, fenced JSON, missing fields, and invalid JSON

 - id: T4
   title: Rate-Limit and Quota Tracking
   Status: completed
   estimate: 4-8h
   files:
     - internal/provider/provider.go
     - internal/provider/openai_provider.go
     - internal/provider/anthropic_provider.go
     - internal/state/store.go
     - internal/quota/monitor_test.go
   steps:
     - id: T4.1
       action: Extend Response type with RateLimitInfo *RateLimitInfo and QuotaInfo *QuotaInfo
       status: completed
       verified: true
       verify: gofmt && go test ./... -run TestProviderRateLimit -v
     - id: T4.2
       action: Add extractRateLimitInfo and extractQuotaInfo helpers in each provider
       status: completed
       verified: true
       verify: go test ./... -run TestProviderRateLimit -v
     - id: T4.3
       action: Persist nullable rate/quota fields in state store; make DB schema nullable
       status: completed
       verified: true
       verify: go test ./... -run TestProviderRateLimit -v
     - id: T4.4
       action: Add `geoffrussy quota` CLI command to display latest provider data
       status: completed
       verified: true
       verify: go test ./... -run TestProviderRateLimit -v
     - id: T4.5
       action: Add unit/property tests for header extraction and persistence; update quota monitor tests to use pointer fields
       status: completed
       verified: true
       verify: go test ./internal/quota -v
   notes:
     - All substeps completed, CLI command exists and works
     - RateLimitInfo and QuotaInfo structs with nullable fields
     - Header extraction helpers for multiple providers
     - Migration added for nullable fields
     - geoffrussy quota CLI command functional
     - Quota monitor tests updated to use pointer fields for nullable struct fields

- id: T5
  title: Improve Database Concurrency
  Status: completed
  estimate: 2-4h
  files:
    - internal/state/store.go
  steps:
    - id: T5.1
      action: Implement executeWithRetry(fn func(*sql.Tx) error) with exponential backoff (base 100ms, max retries 3)
      status: completed
      verified: true
      verify: go test ./internal/state -run TestExecuteWithRetry -v
    - id: T5.2
      action: Refactor transaction-based methods to use executeWithRetry
      status: completed
      verified: true
      verify: go test ./internal/state -run TestExecuteWithRetry -v
    - id: T5.3
      action: Unit tests simulating SQLITE_BUSY and verifying retries
      status: completed
      verified: true
      verify: go test ./internal/state -run TestExecuteWithRetry -v
  notes:
    - run_commands: ["go test ./..."]
    - All security tests passing: 17 test suites, 100+ test cases

 - id: T6
   title: Non-Interactive Configuration and Validation
   Status: completed
   estimate: 2-4h
   files:
     - internal/cli/init.go
     - internal/config/validate.go
     - internal/cli/init_test.go
   steps:
     - id: T6.1
       action: Read GEOFFRUSSY_<PROVIDER>_API_KEY env vars and add flags (--api-key-<provider>); implement --non-interactive
       status: completed
       verified: true
       verify: go test ./internal/cli -run TestInitNonInteractive -v
     - id: T6.2
       action: Implement validateConfiguration(cfg *config.Config) []error and `validate` subcommand
       status: completed
       verified: true
       verify: go test ./internal/cli -run TestInitNonInteractive -v
     - id: T6.3
       action: Tests for env/flag precedence and non-interactive failures
       status: completed
       verified: true
       verify: go test ./internal/cli -run "TestGetAPIKey|TestValidateNonInteractive" -v
   notes:
     - Added flags for all API keys (--api-key-openai, --api-key-anthropic, etc.)
     - Implemented --non-interactive flag
     - Added getAPIKey() function to check flag > environment variable > config in order
     - Created validate subcommand to validate configuration
     - validate command checks API keys, default models, and configuration file
     - Comprehensive tests for env/flag precedence and non-interactive failures
     - Added 12 tests in init_test.go: precedence tests, provider tests, validation tests

 - id: T7
   title: Error Recovery and Graceful Degradation
   Status: completed
   estimate: 3-6h
   files:
     - internal/provider/baseprovider.go
     - internal/executor/taskexecutor.go
     - internal/design/generator.go
     - internal/design/architecture_json.go
     - internal/design/architecture_json_test.go
   steps:
     - id: T7.1
       action: Expand RetryWithBackoff to classify retryable errors and use 1s base backoff
       status: completed
       verified: true
       verify: go test ./internal/provider -run "TestIsRetryableError|TestBaseProvider_RetryWithBackoff" -v
     - id: T7.2
       action: Modify ExecuteTask to continue on single-file failures and aggregate errors
       status: completed
       verified: true
       verify: go test ./internal/executor -run TestExecuteTaskPartialFailure -v
     - id: T7.3
       action: Implement parseArchitectureWithFallback to return partial Architecture + warning error
       status: completed
       verified: true
       verify: go test ./internal/design -run "TestParseArchitectureWithFallback|TestValidateArchitectureJSONPartial" -v
   notes:
     - Added isRetryableError() function to classify retryable errors based on HTTP status codes, network errors, and categorized errors
     - RetryWithBackoff only retries retryable errors
     - Modified ExecuteTask to continue on single-file failures and aggregate errors
     - Modified ExecutePhase to aggregate task errors and continue with warnings
     - Added parseArchitectureWithFallback() and validateArchitectureJSONPartial() for graceful degradation
     - Architecture generation now returns partial architecture with warnings when validation fails

- id: T8
  title: Final Checkpoint & Documentation
  Status: pending
  estimate: 1-2h
  files:
    - README.md
    - SECURITY.md
    - CONTRIBUTING.md
  steps:
    - id: T8.1
      action: Run full test suite and coverage
      status: pending
      verify: go test ./... && go test ./... -cover
    - id: T8.2
      action: Update README, add SECURITY.md and update CONTRIBUTING.md
      status: pending

 Change log:
 - created: structured machine-friendly task list with explicit IDs, statuses, verify commands and target files
 - T5 completed: Implemented executeWithRetry with exponential backoff, refactored all write operations, added comprehensive tests
 - T6 completed: Added non-interactive mode, environment variable support, flags for API keys, validate subcommand implemented, comprehensive tests added
 - T7 completed: Error Recovery and Graceful Degradation implemented
