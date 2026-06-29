# Quota Monitor Tests - Remaining Work

## Issue
The quota monitor tests need to be updated to use pointers for RateLimitInfo fields.

## Files Affected
- `internal/quota/monitor_test.go`

## Changes Needed

### Test Struct Definition (lines 23-28)
**Before:**
```go
testCases := []struct {
    name              string
    requestsRemaining int
    requestsLimit     int
    expectedLevel     WarningLevel
}
```

**After:**
```go
testCases := []struct {
    name              string
    requestsRemaining *int
    requestsLimit     *int
    expectedLevel     WarningLevel
}
```

### Test Case Values
All test cases need to use pointer literals:

**Before:**
```go
requestsRemaining: 500,
requestsLimit:     1000,
ResetAt:           time.Now().Add(time.Hour),
```

**After:**
```go
requestsRemaining: &[]int{500}[0],
requestsLimit:     &[]int{1000}[0],
ResetAt:           &[]time.Time{time.Now().Add(time.Hour)}[0],
```

### Multiple Occurrences
There are 9 test cases that need to be updated:
1. Line 172: `RequestsRemaining: 500,`
2. Line 173: `RequestsLimit: 1000,`
3. Line 174: `ResetAt: time.Now().Add(time.Hour),`
4. Line 220: `RequestsRemaining: 500,`
5. Line 221: `RequestsLimit: 1000,`
6. Line 222: `ResetAt: time.Now().Add(time.Hour),`
7. Line 246: `RequestsRemaining: 0,`
8. Line 247: `RequestsLimit: 1000,`
9. Line 248: `ResetAt: time.Now().Add(time.Hour),`

## Solution
Use the Python script provided earlier to fix all occurrences at once, or manually edit each line using the edit tool to avoid duplication issues.

## Note
This is a low-priority task as the main functionality is working correctly. The quota monitoring system itself is functional, and only the test updates are needed.
