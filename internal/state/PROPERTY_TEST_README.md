# Property-Based Test for State Persistence Round-Trip

## Overview

This document describes the property-based test implementation for **Property 4: State Preservation Round-Trip** as defined in the design document.

## Property Definition

**Property 4: State Preservation Round-Trip**

*For any* project state (interview answers, architecture, DevPlan, task progress), saving the state then loading it should produce an equivalent state with no data loss.

**Validates: Requirements 14.1, 14.2, 14.3**

## Implementation

The property test is implemented in `store_property_test.go` using the [gopter](https://github.com/leanovate/gopter) library for property-based testing in Go.

### Test Configuration

- **Minimum iterations**: 100 (as specified in the design document)
- **Test framework**: gopter v0.2.9
- **Test file**: `internal/state/store_property_test.go`

### Properties Tested

The test validates round-trip persistence for all major state entities:

1. **Project**: Tests that saving and loading a Project preserves all fields
2. **InterviewData**: Tests that saving and loading interview data preserves all nested structures
3. **Architecture**: Tests that saving and loading architecture documents preserves content
4. **Phase**: Tests that saving and loading phases preserves all metadata and timestamps
5. **Task**: Tests that saving and loading tasks preserves all fields including optional timestamps

### Generators

The test includes comprehensive generators for all data structures:

- `genProject()`: Generates random Project instances
- `genInterviewData()`: Generates random InterviewData with nested structures
- `genTechStack()`: Generates random technology stack configurations
- `genTechChoice()`: Generates random technology choices
- `genIntegration()`: Generates random integration configurations
- `genScope()`: Generates random project scope definitions
- `genRefinement()`: Generates random refinement history entries
- `genArchitecture()`: Generates random architecture documents
- `genPhase()`: Generates random phase definitions
- `genTask()`: Generates random task definitions
- `genStage()`: Generates random pipeline stages
- `genPhaseStatus()`: Generates random phase statuses
- `genTaskStatus()`: Generates random task statuses
- `genOptionalTime()`: Generates optional timestamps (nil or valid time)

### Equality Comparisons

The test includes custom equality comparison functions that handle:

- **Timestamp precision**: Allows 1-second tolerance for database timestamp precision
- **Optional fields**: Properly compares nil vs non-nil pointers
- **Nested structures**: Deep comparison of all nested data structures
- **Slices**: Order-sensitive comparison of string slices and complex types

### Key Features

1. **Comprehensive Coverage**: Tests all major state entities defined in the requirements
2. **Smart Generators**: Generates valid data that respects database constraints
3. **Foreign Key Handling**: Properly creates parent entities before testing child entities
4. **Timestamp Handling**: Truncates timestamps to seconds to avoid precision issues
5. **In-Memory Testing**: Uses SQLite in-memory databases for fast test execution

## Running the Tests

### Run All Tests

```bash
make test
```

### Run Only Property Tests

```bash
go test -v ./internal/state/... -run TestProperty4
```

### Run with Coverage

```bash
go test -v -race -coverprofile=coverage.txt ./internal/state/...
```

### Run with Verbose Output

```bash
go test -v ./internal/state/... -run TestProperty4 -gopter.verbose
```

## Expected Behavior

When the test runs successfully:

1. Each property will be tested with at least 100 random inputs
2. All round-trip operations should preserve data exactly
3. No data loss should occur during save/load cycles
4. All nested structures should be preserved
5. Timestamps should match within 1-second tolerance

## Troubleshooting

### Test Failures

If the property test fails, gopter will provide:

1. **Counterexample**: The specific input that caused the failure
2. **Shrunk example**: A minimal version of the failing input
3. **Failure reason**: The specific assertion that failed

### Common Issues

1. **Timestamp precision**: If timestamps don't match, check database precision settings
2. **Foreign key violations**: Ensure parent entities are created before children
3. **JSON marshaling**: Verify complex nested structures serialize correctly
4. **Nil pointer handling**: Check optional field comparisons

## Integration with CI/CD

The property test is automatically run as part of the test suite:

```yaml
# .github/workflows/ci.yml
- name: Run tests
  run: make test
```

## Dependencies

- `github.com/leanovate/gopter v0.2.9`: Property-based testing framework
- `github.com/mattn/go-sqlite3 v1.14.18`: SQLite driver

## References

- Design Document: `.kiro/specs/geoffrey-ai-agent/design.md`
- Requirements: `.kiro/specs/geoffrey-ai-agent/requirements.md`
- Task List: `.kiro/specs/geoffrey-ai-agent/tasks.md`
- gopter Documentation: https://github.com/leanovate/gopter

## Notes

- The test uses in-memory SQLite databases (`:memory:`) for fast execution
- Each property test creates a fresh database to ensure isolation
- Timestamps are truncated to seconds to avoid precision issues with SQLite
- The test validates both simple fields and complex nested structures
- All generators produce valid data that respects database constraints
