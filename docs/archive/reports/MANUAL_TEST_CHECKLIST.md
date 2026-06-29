# Manual Testing Checklist - Geoffrussy AI Coding Agent

**Version**: 0.1.0  
**Date**: January 29, 2026  
**Purpose**: Comprehensive manual testing before public release

This checklist covers all critical user-facing functionality. Tests are organized by priority and complexity.

## Test Environment Setup

### Pre-Test Checklist
- [ ] Clean test directory created
- [ ] No existing `.geoffrussy` directory
- [ ] Git not initialized in test directory
- [ ] Test API keys configured (or mock keys for offline testing)
- [ ] Network connectivity (for provider tests)
- [ ] Sufficient disk space (> 100MB)

### Platform Testing
- [ ] **Linux** (AMD64)
- [ ] **Linux** (ARM64)
- [ ] **macOS** (Intel)
- [ ] **macOS** (Apple Silicon)
- [ ] **Windows** (AMD64)
- [ ] **Windows** (ARM64)

## Priority 1: Core Functionality Tests

### 1.1 Initialization Tests
**Objective**: Verify project initialization works correctly

#### Test 1.1.1: Fresh Initialization
```
mkdir test-project
cd test-project
geoffrussy init
```
**Expected**:
- [ ] Config directory created (`~/.geoffrussy/`)
- [ ] Database initialized successfully
- [ ] API key prompts appear (if keys not configured)
- [ ] Git repository initialized
- [ ] Success message displayed

#### Test 1.1.2: Re-initialization Prevention
```
geoffrussy init
```
**Expected**:
- [ ] Error message about existing configuration
- [ ] No data loss or corruption
- [ ] Suggestion to use `resume` command

#### Test 1.1.3: Configuration File Creation
```
ls -la ~/.geoffrussy/config.yaml
```
**Expected**:
- [ ] File exists
- [ ] File permissions are 0600 (owner only)
- [ ] Contains valid YAML structure
- [ ] API keys are present (if provided)

#### Test 1.1.4: API Key Input Validation
**Test Case**: Enter invalid API key
```
sk-invalid-key-format-123
```
**Expected**:
- [ ] Key accepted (we don't validate format for all providers)
- [ ] Provider-specific validation on first API call
- [ ] Helpful error message if invalid

### 1.2 Interview Tests

#### Test 1.2.1: Start New Interview
```
geoffrussy interview
```
**Expected**:
- [ ] Interview starts at Phase 1: Project Essence
- [ ] First question displayed clearly
- [ ] Progress indicator shows 1/5 phases
- [ ] Answer input prompt visible

#### Test 1.2.2: Answer Questions
**Test**: Provide answers for all questions
**Expected**:
- [ ] Each answer accepted
- [ ] Progress updates after each answer
- [ ] LLM-powered follow-up questions appear (if enabled)
- [ ] Default answers suggested

#### Test 1.2.3: Pause Interview
**Test**: Use keyboard shortcut to pause (Ctrl+C)
**Expected**:
- [ ] Graceful pause (not crash)
- [ ] State saved
- [ ] Resume option offered

#### Test 1.2.4: Resume Interview
```
geoffrussy interview --resume
```
**Expected**:
- [ ] Previous answers displayed
- [ ] Continues from last answered question
- [ ] No data loss

#### Test 1.2.5: Reiterate Answer
**Test**: Go back and change an answer
**Expected**:
- [ ] Can navigate to previous questions
- [ ] Can edit answers
- [ ] Follow-up questions regenerate
- [ ] New state saved

#### Test 1.2.6: Complete Interview
**Test**: Answer all 5 phases completely
**Expected**:
- [ ] All 5 phases completed
- [ ] Summary displayed
- [ ] JSON export offered
- [ ] Project advances to Design stage

### 1.3 Design Tests

#### Test 1.3.1: Generate Architecture
```
geoffrussy design
```
**Expected**:
- [ ] Architecture generation starts
- [ ] Streaming output visible
- [ ] Progress indicators
- [ ] Model selection shown
- [ ] Success message on completion

#### Test 1.3.2: Architecture Document Content
**Test**: Review generated `architecture.md`
**Expected**:
- [ ] File created in project directory
- [ ] Contains all required sections:
  - [ ] System overview
  - [ ] Component breakdown
  - [ ] Data flow diagrams
  - [ ] Technology rationale
  - [ ] Scaling strategy
  - [ ] API contracts
  - [ ] Database schema
  - [ ] Security approach
  - [ ] Observability strategy
  - [ ] Deployment architecture
  - [ ] Risk assessment

#### Test 1.3.3: Reiterate Architecture
```
geoffrussy design --refine
```
**Expected**:
- [ ] Previous architecture loaded
- [ ] Prompt for refinement input
- [ ] LLM generates improvements
- [ ] Architecture updated

#### Test 1.3.4: Generate JSON
```
geoffrussy design --format json
```
**Expected**:
- [ ] JSON file created
- [ ] Valid JSON structure
- [ ] Contains all architecture fields

### 1.4 DevPlan Tests

#### Test 1.4.1: Generate DevPlan
```
geoffrussy plan
```
**Expected**:
- [ ] DevPlan generation starts
- [ ] 7-10 phases generated
- [ ] Each phase has 3-5 tasks
- [ ] Phase files created (`phases/001-*.md`, etc.)
- [ ] Master plan created (`devplan.md`)
- [ ] Token usage estimates shown

#### Test 1.4.2: Phase Structure Validation
**Test**: Inspect phase files
**Expected**:
- [ ] Each phase has objective
- [ ] Success criteria defined
- [ ] Dependencies listed
- [ ] Tasks numbered and ordered
- [ ] Token cost estimates included

#### Test 1.4.3: Merge Phases
```
geoffrussy plan --merge 2 3
```
**Expected**:
- [ ] Phases merged successfully
- [ ] Tasks combined
- [ ] Dependencies updated
- [ ] Remaining phases renumbered

#### Test 1.4.4: Split Phase
```
geoffrussy plan --split 2
```
**Expected**:
- [ ] Phase split at appropriate point
- [ ] Tasks distributed
- [ ] Dependencies preserved
- [ ] New phase created

#### Test 1.4.5: Reorder Phases
```
geoffrussy plan --reorder 2 5 4
```
**Expected**:
- [ ] Phases reordered
- [ ] Dependencies validated
- [ ] Conflicting dependencies caught

### 1.5 Review Tests

#### Test 1.5.1: Run Phase Review
```
geoffrussy review
```
**Expected**:
- [ ] All phases analyzed
- [ ] Issues categorized (critical, warning, info)
- [ ] Review report generated
- [ ] Improvements suggested

#### Test 1.5.2: Review Issue Categories
**Expected**:
- [ ] Clarity issues identified
- [ ] Completeness gaps found
- [ ] Dependency errors detected
- [ ] Scope issues highlighted
- [ ] Risk factors noted
- [ ] Testing gaps reported

#### Test 1.5.3: Apply Improvements
**Test**: Select and apply improvements
**Expected**:
- [ ] Can review improvements
- [ ] Can select specific improvements
- [ ] Phase files updated
- [ ] Changes saved

#### Test 1.5.4: Cross-Phase Issues
**Test**: Check for issues between phases
**Expected**:
- [ ] Dependency conflicts detected
- [ ] Missing dependencies identified
- [ ] Circular dependencies caught

### 1.6 Development Tests

#### Test 1.6.1: Execute Phase
```
geoffrussy develop --phase 001
```
**Expected**:
- [ ] Phase starts execution
- [ ] Tasks executed in order
- [ ] Streaming output visible
- [ ] Progress tracked per task

#### Test 1.6.2: Pause Execution
**Test**: Use keyboard shortcut (Ctrl+C) during execution
**Expected**:
- [ ] Execution pauses gracefully
- [ ] Current task state saved
- [ ] Resume option displayed

#### Test 1.6.3: Resume Execution
```
geoffrussy develop --resume
```
**Expected**:
- [ ] Resumes from paused task
- [ ] No task duplication
- [ ] Continues to next task

#### Test 1.6.4: Skip Task
**Test**: Skip a task during execution
**Expected**:
- [ ] Task marked as skipped
- [ ] Next task starts
- [ ] Progress updated

#### Test 1.6.5: Task Completion
**Expected**:
- [ ] Task marked as completed
- [ ] Token usage recorded
- [ ] Cost calculated
- [ ] Git commit created

## Priority 2: Error Handling Tests

### 2.1 Invalid API Keys

#### Test 2.1.1: Empty API Key
```
# Provide empty API key
```
**Expected**:
- [ ] Error message: "API key cannot be empty"
- [ ] Suggestion: "Enter a valid API key"

#### Test 2.1.2: Invalid API Key
```
# Use invalid key like "sk-invalid"
```
**Expected**:
- [ ] Provider returns authentication error
- [ ] Error categorized as API error
- [ ] Retry offered (if applicable)
- [ ] No key exposed in error message

#### Test 2.1.3: No API Key Configured
**Expected**:
- [ ] Error: "API key not found for provider: X"
- [ ] Prompt to configure API key
- [ ] Link to configuration docs

### 2.2 Network Failures

#### Test 2.2.1: No Network Connection
```
# Disconnect network before starting
geoffrussy interview
```
**Expected**:
- [ ] Error categorized as Network error
- [ ] Helpful error message
- [ ] Retry suggested
- [ ] No crash or hang

#### Test 2.2.2: Network Timeout
**Expected**:
- [ ] Timeout detected
- [ ] Retry with exponential backoff
- [ ] Max retries attempted
- [ ] Clear error after retries exhausted

#### Test 2.2.3: Network Restoration
**Test**: Restore network during retry
**Expected**:
- [ ] Operation continues automatically
- [ ] No user intervention needed
- [ ] Progress resumes

### 2.3 Git Conflicts

#### Test 2.3.1: Detect Conflicts
```
# Create conflicting changes outside Geoffrussy
geoffrussy design
```
**Expected**:
- [ ] Git conflict detected
- [ ] Error message explains conflict
- [ ] User intervention requested
- [ ] No data loss

#### Test 2.3.2: Merge Conflict Resolution
**Expected**:
- [ ] Clear instructions provided
- [ ] Can resolve manually
- [ ] Geoffrussy can continue after resolution

#### Test 2.3.3: Uncommitted Changes
**Expected**:
- [ ] Detected before operation
- [ ] Warning displayed
- [ ] Commit requested
- [ ] Operation continues after commit

## Priority 3: Advanced Features Tests

### 3.1 Checkpoint System

#### Test 3.1.1: Create Checkpoint
```
geoffrussy checkpoint create --name "After Phase 1"
```
**Expected**:
- [ ] Checkpoint created
- [ ] Git tag created
- [ ] Metadata saved
- [ ] Checkpoint listed

#### Test 3.1.2: List Checkpoints
```
geoffrussy checkpoint list
```
**Expected**:
- [ ] All checkpoints displayed
- [ ] Show name, timestamp, git tag
- [ ] Show progress at checkpoint

#### Test 3.1.3: Rollback to Checkpoint
```
geoffrussy rollback checkpoint-id
```
**Expected**:
- [ ] Git reset to tag
- [ ] Database state restored
- [ ] Project stage updated
- [ ] Rollback confirmed

#### Test 3.1.4: Rollback Validation
**Test**: Try to rollback to non-existent checkpoint
**Expected**:
- [ ] Error: checkpoint not found
- [ ] List of available checkpoints
- [ ] No data corruption

### 3.2 Detour Support

#### Test 3.2.1: Request Detour
**Test**: Use keyboard shortcut to request detour during execution
**Expected**:
- [ ] Execution paused
- [ ] Detour information requested
- [ ] New tasks generated
- [ ] DevPlan updated

#### Test 3.2.2: Detour Tasks Integration
**Expected**:
- [ ] Tasks added to DevPlan
- [ ] Dependencies preserved
- [ ] Original tasks not lost
- [ ] Integration points marked

#### Test 3.2.3: Detour Tracking
```
ls detours/
```
**Expected**:
- [ ] Detour directory created
- [ ] Detour tasks tracked separately
- [ ] Can reference original tasks

### 3.3 Blocker Detection

#### Test 3.3.1: Simulate Blocker
**Test**: Cause a task to fail (e.g., invalid command)
**Expected**:
- [ ] Task failure detected
- [ ] Retry attempted (max 3)
- [ ] After N failures, marked as blocked
- [ ] User notified with context

#### Test 3.3.2: Blocker Resolution
```
geoffrussy develop --resolve-blocker blocker-id
```
**Expected**:
- [ ] Resolution strategies suggested
- [ ] User can try auto-resolution
- [ ] Or manual intervention
- [ ] Blocker resolved
- [ ] Task re-executed

#### Test 3.3.3: Active Blockers Display
```
geoffrussy status
```
**Expected**:
- [ ] Blocked tasks shown
- [ ] Blocker reasons displayed
- [ ] Resolution actions suggested

### 3.4 Pipeline Navigation

#### Test 3.4.1: Go Back to Previous Stage
```
geoffrussy navigate --to interview
```
**Expected**:
- [ ] Can navigate back
- [ ] Current work preserved
- [ ] Dependent artifacts noted for regeneration
- [ ] Git commit created

#### Test 3.4.2: Skip Stage Prevention
**Test**: Try to skip ahead
```
geoffrussy navigate --to develop
# From interview stage
```
**Expected**:
- [ ] Error: cannot skip stages
- [ ] Missing prerequisite identified
- [ ] Suggestion: complete current stage first

#### Test 3.4.3: Same Stage Prevention
```
geoffrussy navigate --to design
# Already at design stage
```
**Expected**:
- [ ] Error: already at design stage
- [ ] No changes made
- [ ] No corruption

### 3.5 Resume Capability

#### Test 3.5.1: Detect Incomplete Work
```
# Start interview but don't complete
# Exit and run
geoffrussy
```
**Expected**:
- [ ] Detects incomplete work
- [ ] Offers to resume
- [ ] Shows current stage
- [ ] Displays progress

#### Test 3.5.2: Resume from Last Checkpoint
**Expected**:
- [ ] Rolls back to latest checkpoint
- [ ] Restores all state
- [ ] Continues execution
- [ ] No data loss

#### Test 3.5.3: Resume from Specific Stage
```
geoffrussy resume --stage design
```
**Expected**:
- [ ] Moves to design stage
- [ ] Loads architecture data
- [ ] Ready to continue

#### Test 3.5.4: Resume with Restart
```
geoffrussy resume --restart-stage
```
**Expected**:
- [ ] Resets stage state
- [ ] In-progress tasks reset
- [ ] Clear for new execution

## Priority 4: Provider-Specific Tests

### 4.1 OpenAI Provider

#### Test 4.1.1: Model Selection
```
geoffrussy config set default_model interview gpt-4
```
**Expected**:
- [ ] Model set for interview stage
- [ ] Configuration saved
- [ ] Used in next API call

#### Test 4.1.2: Streaming Response
**Expected**:
- [ ] Real-time streaming visible
- [ ] Text appears progressively
- [ ] No buffering issues
- [ ] Complete response received

#### Test 4.1.3: Rate Limit Handling
**Test**: Hit rate limit (may need rapid requests)
**Expected**:
- [ ] Rate limit error detected
- [ ] Wait time calculated
- [ ] Automatic retry
- [ ] No duplicate requests during wait

### 4.2 Anthropic Provider

#### Test 4.2.1: Claude Models
```
geoffrussy config set default_model design claude-3-5-sonnet
```
**Expected**:
- [ ] Model configured
- [ ] Successfully calls Anthropic API
- [ ] Responses received

#### Test 4.2.2: Header Format
**Expected**:
- [ ] x-api-key header used (not Authorization)
- [ ] Version header included
- [ ] Request formatted correctly

### 4.3 Ollama Provider

#### Test 4.3.1: Local Model Discovery
```
# Start Ollama server
geoffrussy config set default_model develop llama2
```
**Expected**:
- [ ] Connects to localhost:11434
- [ ] Lists available models
- [ ] Uses local model
- [ ] No API key required

#### Test 4.3.2: Offline Capabilities
```
# Stop network
geoffrussy status
```
**Expected**:
- [ ] Status works offline
- [ ] No API calls needed
- [ ] Displays cached data

### 4.4 Other Providers

#### Test 4.4.1: Firmware.ai
**Expected**:
- [ ] Authentication works
- [ ] Models listed
- [ ] API calls succeed
- [ ] Rate limits tracked

#### Test 4.4.2: Requesty.ai
**Expected**:
- [ ] Authentication works
- [ ] Models listed
- [ ] API calls succeed
- [ ] Rate limits tracked

#### Test 4.4.3: Z.ai
**Expected**:
- [ ] Authentication works
- [ ] Models listed
- [ ] Coding plan support works
- [ ] Rate limits tracked

#### Test 4.4.4: Kimi
**Expected**:
- [ ] Authentication works
- [ ] Models listed
- [ ] Coding plan support works
- [ ] Rate limits tracked

#### Test 4.4.5: OpenCode
**Expected**:
- [ ] Dynamic model discovery works
- [ ] Uses opencode run for tasks
- [ ] Model list populates
- [ ] No API key needed

## Priority 5: Cost and Statistics Tests

### 5.1 Token Tracking

#### Test 5.1.1: Token Usage Display
```
geoffrussy stats
```
**Expected**:
- [ ] Total tokens shown
- [ ] Breakdown by provider
- [ ] Breakdown by phase
- [ ] Input/output split

#### Test 5.1.2: Cost Calculation
**Expected**:
- [ ] Total cost displayed
- [ ] By provider cost breakdown
- [ ] By phase cost breakdown
- [ ] Most expensive operations identified

#### Test 5.1.3: Budget Warnings
**Test**: Set low budget (e.g., $1.00)
```
geoffrussy config set budget_limit 1.00
```
**Expected**:
- [ ] Warning at 80% ($0.80)
- [ ] Critical at 95% ($0.95)
- [ ] Block at 100% ($1.00)
- [ ] Clear messages

### 5.2 Quota Monitoring

#### Test 5.2.1: Check Quotas
```
geoffrussy quota
```
**Expected**:
- [ ] Rate limits for all providers
- [ ] Tokens remaining for all providers
- [ ] Warnings if approaching limits
- [ ] Clear formatting

#### Test 5.2.2: Automatic Delaying
**Test**: Approach rate limit
**Expected**:
- [ ] Request delayed automatically
- [ ] Wait time displayed
- [ ] Request retries after wait
- [ ] No errors from rate limit

## Priority 6: Integration Tests

### 6.1 Complete Pipeline

#### Test 6.1.1: End-to-End Workflow
```
geoffrussy init
geoffrussy interview
geoffrussy design
geoffrussy plan
geoffrussy review
geoffrussy develop
```
**Expected**:
- [ ] All stages complete successfully
- [ ] Artifacts created at each stage
- [ ] Git commits throughout
- [ ] Project marked as complete
- [ ] Final stats available

#### Test 6.1.2: Pause/Resume Throughout
**Expected**:
- [ ] Can pause at any stage
- [ ] Can resume from any stage
- [ ] State preserved across sessions
- [ ] No data loss

#### Test 6.1.3: Reiteration at Each Stage
**Expected**:
- [ ] Can reiterate interview answers
- [ ] Can refine architecture
- [ ] Can modify DevPlan
- [ ] Can apply review improvements
- [ ] All changes tracked

### 6.2 Error Recovery Integration

#### Test 6.2.1: Network Recovery During Pipeline
**Test**: Cause network failure, then restore
**Expected**:
- [ ] Error caught and categorized
- [ ] State preserved
- [ ] Resume after network restore
- [ ] Pipeline continues

#### Test 6.2.2: API Key Rotation
```
geoffrussy config set api_key.anthropic new-key
geoffrussy interview --resume
```
**Expected**:
- [ ] New key used
- [ ] No authentication errors
- [ ] Continues seamlessly

## Priority 7: User Experience Tests

### 7.1 Terminal UI

#### Test 7.1.1: Interview UI
**Expected**:
- [ ] Clear question display
- [ ] Answer input visible
- [ ] Progress indicator
- [ ] Keyboard shortcuts work
- [ ] Responsive to input

#### Test 7.1.2: Execution UI
**Expected**:
- [ ] Streaming output visible
- [ ] Task progress bars
- [ ] Current task highlighted
- [ ] Pause/resume controls
- [ ] Status updates

#### Test 7.1.3: Review UI
**Expected**:
- [ ] Issues list displayed
- [ ] Can select improvements
- [ ] Clear categorization
- [ ] Apply button works
- [ ] Feedback on changes

#### Test 7.1.4: Status Dashboard
**Expected**:
- [ ] Current stage shown
- [ ] Phase progress bars
- [ ] Active blockers displayed
- [ ] Token/cost summary
- [ ] Time elapsed shown

### 7.2 Command Help

#### Test 7.2.1: Main Help
```
geoffrussy --help
```
**Expected**:
- [ ] All commands listed
- [ ] Brief descriptions
- [ ] Global flags shown
- [ ] Clear formatting

#### Test 7.2.2: Command-Specific Help
```
geoffrussy interview --help
geoffrussy design --help
```
**Expected**:
- [ ] All flags listed
- [ ] Descriptions provided
- [ ] Examples shown
- [ ] Usage format clear

### 7.3 Error Messages

#### Test 7.3.1: User Error Messages
**Test**: Provide invalid input
**Expected**:
- [ ] Clear error category (User Error)
- [ ] Specific problem identified
- [ ] Helpful suggestion
- [ ] No technical jargon

#### Test 7.3.2: System Error Messages
**Test**: Cause system error (disk full)
**Expected**:
- [ ] Clear error category (System Error)
- [ ] Problem explained
- [ ] Actionable suggestion
- [ ] Severity indicated

## Test Results Recording

### Pass/Fail Summary

| Test Category | Total | Passed | Failed | Blocked |
|-------------|-------|--------|--------|----------|
| Initialization | 4 | | | |
| Interview | 6 | | | |
| Design | 4 | | | |
| DevPlan | 5 | | | |
| Review | 4 | | | |
| Development | 5 | | | |
| Error Handling | 9 | | | |
| Advanced Features | 12 | | | |
| Provider-Specific | 8 | | | |
| Cost & Stats | 3 | | | |
| Integration | 3 | | | |
| UX | 4 | | | |
| **TOTAL** | **67** | | | |

### Critical Path Tests

These tests must pass for release:

- [ ] 1.1.1: Fresh initialization
- [ ] 1.2.6: Complete interview
- [ ] 1.3.2: Architecture content
- [ ] 1.4.2: Phase structure
- [ ] 1.5.1: Run phase review
- [ ] 1.6.5: Task completion
- [ ] 2.1.1: Empty API key handling
- [ ] 2.2.1: No network connection
- [ ] 3.1.1: Create checkpoint
- [ ] 3.1.3: Rollback to checkpoint
- [ ] 3.3.1: Blocker detection
- [ ] 3.4.1: Go back to previous stage
- [ ] 3.5.1: Detect incomplete work
- [ ] 5.1.1: Token usage display
- [ ] 5.2.1: Check quotas
- [ ] 6.1.1: End-to-end workflow
- [ ] 7.1.1: Interview UI
- [ ] 7.3.1: User error messages

**Critical Path Status**: ___ / 20 Passed

### Known Issues

Document any issues found during testing:

| Issue ID | Description | Severity | Status |
|----------|-------------|-----------|--------|
| | | | |
| | | | |
| | | | |

### Tester Notes

Provide additional observations, UX feedback, or suggestions:

- 
- 
- 

## Sign-off

**Tester**: _________________  
**Date**: _________________  
**Platform**: _________________  
**Go Version**: _________________  

**Release Decision**:
- [ ] ✅ APPROVED - All critical tests passed
- [ ] ⚠️  CONDITIONAL - Minor issues but acceptable for release
- [ ] ❌ NOT APPROVED - Critical issues found

**Blocking Issues**: _______________________________

**Recommended Actions**: _______________________________

---

## Appendix A: Test Environment Setup

### Automated Test Scripts

```bash
#!/bin/bash
# test-environment.sh

echo "Setting up test environment..."

# Create test directory
TEST_DIR=$(mktemp -d)
cd $TEST_DIR

# Clean any existing config
rm -rf ~/.geoffrussy

# Set up test API keys (use mock or test keys)
export GEOFFRUSSY_OPENAI_API_KEY="sk-test-key"
export GEOFFRUSSY_ANTHROPIC_API_KEY="sk-ant-test-key"

echo "Test environment ready: $TEST_DIR"
echo "Config directory will be: ~/.geoffrussy"
echo ""
echo "Run 'geoffrussy init' to begin testing"
```

### Quick Smoke Test

```bash
#!/bin/bash
# smoke-test.sh

echo "Running smoke tests..."

# Test 1: Help command
geoffrussy --help || exit 1

# Test 2: Version command
geoffrussy version || exit 1

# Test 3: Status (should work without project)
geoffrussy status || exit 1

echo "✅ Smoke tests passed"
```

## Appendix B: Mock Provider Setup

For testing without real API keys:

```go
// mock-provider.go (for internal testing)
type MockProvider struct {
    *BaseProvider
}

func (m *MockProvider) Call(req *Request) (*Response, error) {
    return &Response{
        Content: "Mock response",
        TokensInput: 100,
        TokensOutput: 50,
    }, nil
}

func (m *MockProvider) Stream(req *Request, callback func(string)) error {
    callback("Mock streaming response")
    return nil
}
```

## Appendix C: Test Coverage Matrix

| Feature | Unit Tests | Integration Tests | Manual Tests |
|----------|------------|-------------------|--------------|
| Interview | ✅ | ⚠️ | ✅ |
| Design | ✅ | ⚠️ | ✅ |
| DevPlan | ✅ | ⚠️ | ✅ |
| Review | ✅ | ⚠️ | ✅ |
| Develop | ✅ | ⚠️ | ✅ |
| State Management | ✅ | ⚠️ | ✅ |
| Git Integration | ✅ | ⚠️ | ✅ |
| Error Handling | ✅ | ⚠️ | ✅ |
| Providers | ✅ | ⚠️ | ✅ |
| CLI | ✅ | ⚠️ | ✅ |
| TUI | ⚠️ | ⚠️ | ✅ |

Legend:
- ✅ Tested
- ⚠️ Partially tested
- ❌ Not tested

---

**This checklist should be completed before v0.1.0 public release.**
