# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Geoffrussy, please report it responsibly:

1. **Do not** open a public GitHub issue.
2. Email the maintainers at the address listed in the repository.
3. Include a clear description, steps to reproduce, and potential impact.

We will acknowledge your report within 48 hours and provide a timeline for a fix.

## Security Features

### Path Sanitization

All file paths processed by Geoffrussy are validated against the project root directory via `internal/security/PathSanitizer`. This prevents directory traversal attacks (`../` sequences, absolute paths pointing outside the root, etc.).

**Symlink protection:** `ValidatePath()` resolves symlinks using `filepath.EvalSymlinks` before checking containment. Symlinks (including chained symlinks) that resolve to locations outside the project root are rejected. For paths that do not yet exist on disk, the deepest existing ancestor is resolved, and the remaining tail components are appended and re-checked.

**Windows UNC paths:** On Windows, UNC paths (`\\server\share`, `\\?\...`, `\\.\...`) are rejected outright to prevent network path injection.

### API Key Storage

API keys are stored in the OS keyring when available (via `go-keyring`). Fallback metadata is stored in `~/.geoffrussy/config.yaml` with restricted file permissions. Keys are never logged or written to state databases.

### Input Validation

- Project names are validated against a strict alphanumeric + hyphen/underscore pattern.
- JSON input is validated for syntax and optionally against a schema before processing.
- File content is checked for valid UTF-8 encoding and size limits.

### Audit Logging

Security-relevant events (path rejections, authentication failures, file operations) are logged via `internal/security/AuditLogger` with structured JSON output.

## Dependency Policy

- Dependencies are pinned via `go.sum`.
- The project uses `go-sqlite3` (CGO) for state persistence; ensure your C toolchain is trusted.
- Review `go.mod` periodically for known vulnerabilities using `govulncheck`.

## Full Audit Report

See `docs/archive/reports/SECURITY_AUDIT.md` for the comprehensive security audit.
