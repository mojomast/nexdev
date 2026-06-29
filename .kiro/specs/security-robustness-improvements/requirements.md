# Requirements Document

## Introduction

This document specifies the requirements for implementing critical security and robustness improvements to the geoffrussy project. Geoffrussy is a Go-based AI-assisted development workflow tool that uses LLMs to generate architecture, development plans, and execute tasks. The system currently has several security vulnerabilities and robustness issues that need to be addressed to ensure safe and reliable operation.

## Glossary

- **Task_Executor**: The component responsible for executing development tasks using LLM-generated code
- **Architecture_Generator**: The component that generates system architecture from interview data
- **Provider**: An LLM API provider (OpenAI, Anthropic, etc.)
- **State_Store**: The SQLite-based persistence layer for project state
- **CLI**: Command-line interface for user interaction
- **Project_Root**: The base directory of the user's project being developed
- **Path_Sanitization**: The process of validating and normalizing file paths to prevent directory traversal attacks
- **Rate_Limit**: API provider restrictions on request frequency
- **Quota**: API provider restrictions on token usage or cost
- **WAL_Mode**: Write-Ahead Logging mode in SQLite for improved concurrency

## Requirements

### Requirement 1: Path Sanitization and File-Write Security

**User Story:** As a developer, I want the system to prevent malicious file writes outside the project directory, so that my system files remain protected from unauthorized modifications.

#### Acceptance Criteria

1. WHEN the Task_Executor receives a file path from LLM output, THE Task_Executor SHALL validate the path is within the Project_Root before writing
2. WHEN a file path contains directory traversal sequences (../, ..\, absolute paths), THE Task_Executor SHALL reject the path and log a security warning
3. WHEN a file path is validated, THE Task_Executor SHALL resolve it to an absolute path and verify it starts with the Project_Root
4. WHEN a file write is rejected due to path validation failure, THE Task_Executor SHALL return a descriptive error message indicating the security violation
5. THE Task_Executor SHALL normalize all file paths to prevent bypass attempts using path variations

### Requirement 2: Robust Architecture Parsing

**User Story:** As a user, I want the architecture generation to be reliable and structured, so that I can consistently process and use the generated architecture data.

#### Acceptance Criteria

1. WHEN the Architecture_Generator requests architecture from an LLM, THE Architecture_Generator SHALL specify JSON output format in the prompt
2. WHEN the LLM returns architecture data, THE Architecture_Generator SHALL parse it as structured JSON rather than text extraction
3. IF JSON parsing fails, THEN THE Architecture_Generator SHALL retry with a clarification prompt requesting valid JSON
4. WHEN architecture is successfully parsed, THE Architecture_Generator SHALL validate all required fields are present
5. THE Architecture_Generator SHALL provide clear error messages when parsing fails after retries

### Requirement 3: Dynamic Provider Discovery and Non-Interactive Configuration

**User Story:** As a user, I want to configure API providers non-interactively using environment variables or flags, so that I can automate geoffrussy setup in CI/CD pipelines.

#### Acceptance Criteria

1. WHEN the CLI initializes, THE CLI SHALL discover available providers dynamically from the provider registry
2. WHEN API keys are needed, THE CLI SHALL check environment variables before prompting interactively
3. THE CLI SHALL accept API keys via command-line flags for non-interactive configuration
4. WHEN running in non-interactive mode, THE CLI SHALL use default values or fail with clear error messages if required configuration is missing
5. THE CLI SHALL support a configuration file format for batch provider setup

### Requirement 4: Rate-Limit and Quota Information Exposure

**User Story:** As a developer, I want accurate rate-limit and quota information from API providers, so that I can avoid throttling and plan API usage effectively.

#### Acceptance Criteria

1. WHEN a Provider makes an API call, THE Provider SHALL extract rate-limit information from response headers
2. WHEN a Provider makes an API call, THE Provider SHALL extract quota information from response headers
3. THE Provider SHALL store rate-limit and quota information in the State_Store after each API call
4. WHEN rate-limit information is unavailable in headers, THE Provider SHALL return null values rather than zeros
5. THE CLI SHALL display current rate-limit and quota status when requested by the user

### Requirement 5: Structured Logging and Error Handling

**User Story:** As a maintainer, I want structured logging throughout the codebase, so that I can debug issues efficiently and monitor system behavior.

#### Acceptance Criteria

1. THE system SHALL use a structured logging library (such as zerolog or zap) for all log output
2. WHEN an error occurs, THE system SHALL log it with contextual information including component name, operation, and relevant identifiers
3. THE system SHALL support configurable log levels (debug, info, warn, error)
4. WHEN logging API calls, THE system SHALL include provider, model, token counts, and timing information
5. THE system SHALL sanitize sensitive information (API keys, user data) from log output

### Requirement 6: SQLite Concurrency Safety

**User Story:** As a user, I want the system to handle concurrent operations safely, so that my project state remains consistent even with parallel task execution.

#### Acceptance Criteria

1. THE State_Store SHALL use WAL_Mode for all database connections
2. WHEN multiple operations access the database concurrently, THE State_Store SHALL use appropriate transaction isolation
3. THE State_Store SHALL implement retry logic with exponential backoff for database busy errors
4. WHEN a transaction fails due to concurrency conflicts, THE State_Store SHALL retry up to 3 times before returning an error
5. THE State_Store SHALL use connection pooling to manage concurrent access efficiently

### Requirement 7: Input Validation and Sanitization

**User Story:** As a developer, I want all user inputs and LLM outputs to be validated, so that the system behaves predictably and securely.

#### Acceptance Criteria

1. WHEN the system receives file paths from any source, THE system SHALL validate them against a whitelist of allowed characters
2. WHEN the system receives project names or identifiers, THE system SHALL validate they match expected patterns (alphanumeric, hyphens, underscores)
3. WHEN the system receives JSON from LLM output, THE system SHALL validate it against expected schemas before processing
4. WHEN validation fails, THE system SHALL return specific error messages indicating which validation rule was violated
5. THE system SHALL limit the size of LLM responses to prevent memory exhaustion

### Requirement 8: Error Recovery and Graceful Degradation

**User Story:** As a user, I want the system to recover gracefully from errors, so that temporary failures don't require manual intervention or data loss.

#### Acceptance Criteria

1. WHEN an API call fails with a retryable error (5xx, timeout), THE Provider SHALL retry with exponential backoff
2. WHEN the State_Store encounters a database lock, THE State_Store SHALL wait and retry rather than failing immediately
3. WHEN architecture parsing fails, THE Architecture_Generator SHALL attempt to extract partial data rather than failing completely
4. WHEN a file write fails, THE Task_Executor SHALL log the error and continue with remaining files rather than aborting
5. THE system SHALL maintain operation logs that can be used to resume interrupted operations

### Requirement 9: Configuration Validation

**User Story:** As a user, I want the system to validate my configuration at startup, so that I discover configuration errors early rather than during execution.

#### Acceptance Criteria

1. WHEN the system starts, THE CLI SHALL validate all configured API keys are non-empty
2. WHEN the system starts, THE CLI SHALL verify the Project_Root exists and is writable
3. WHEN the system starts, THE CLI SHALL check that the State_Store database is accessible and not corrupted
4. WHEN configuration validation fails, THE CLI SHALL display specific error messages for each validation failure
5. THE CLI SHALL provide a dedicated command to validate configuration without executing operations

### Requirement 10: Audit Logging for Security Events

**User Story:** As a security-conscious developer, I want security-relevant events to be logged, so that I can detect and investigate potential security issues.

#### Acceptance Criteria

1. WHEN a path validation fails, THE Task_Executor SHALL log the rejected path and the reason for rejection
2. WHEN an API call fails with authentication errors, THE Provider SHALL log the provider and error details
3. WHEN file operations are performed, THE Task_Executor SHALL log the file path and operation type
4. THE system SHALL maintain an audit log separate from general application logs
5. WHEN audit logs are written, THE system SHALL include timestamps, user context, and operation outcomes

