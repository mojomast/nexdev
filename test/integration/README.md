# Integration Tests

This directory contains integration tests for geoffrussy that test the full pipeline with real AI providers.

## Prerequisites

1. **ZAI API Key**: You must have a valid ZAI API key configured in your `~/.config/geoffrussy/config.yaml`:
   ```yaml
   api_keys:
     zai: your-api-key-here
   default_models:
     develop: glm-4.7
     design: glm-4.7
     devplan: glm-4.7
     interview: glm-4.7
   ```

2. **Build Tag**: These tests require the `integration` build tag to run.

## Running Tests

### Full Pipeline Test

Tests the complete geoffrussy pipeline:
- Configuration loading
- ZAI provider setup with glm-4.7
- Project creation
- Interview data creation
- Architecture generation
- Development plan generation
- Task execution

```bash
# Using make
make test-pipeline

# Or directly
INTEGRATION_TEST=1 go test -v -tags=integration -run TestFullPipelineZAI ./test/integration/... -timeout 10m
```

### Simple DevPlan Execution Test

A faster test that uses a pre-defined simple devplan (bypasses AI generation):

```bash
# Using make
make test-pipeline-simple

# Or directly
INTEGRATION_TEST=1 go test -v -tags=integration -run TestSimpleDevPlanExecution ./test/integration/... -timeout 5m
```

### All Integration Tests

```bash
# Using make (without INTEGRATION_TEST flag, tests will skip)
make test-integration

# With real execution
INTEGRATION_TEST=1 make test-integration
```

## What to Expect

### Full Pipeline Test (`TestFullPipelineZAI`)

This test will:
1. Create a temporary directory for the test project
2. Initialize the configuration and state store
3. Set up the ZAI provider with glm-4.7 model
4. Create interview data for a "Hello World Service"
5. Generate architecture using the real AI provider (may take 30-60 seconds)
6. Generate development phases using the real AI provider (may take 30-60 seconds)
7. Store phases and tasks in the state database
8. Attempt to execute the first task (will likely fail in test environment but validates executor setup)
9. Verify state management

**Duration**: 2-5 minutes depending on API response times

**API Usage**: ~2-3 API calls to ZAI (architecture + devplan generation)

### Simple DevPlan Test (`TestSimpleDevPlanExecution`)

This test will:
1. Create a temporary directory
2. Set up the ZAI provider
3. Initialize the state store
4. Create a simple pre-defined devplan (no AI calls)
5. Execute the first task

**Duration**: 30-60 seconds

**API Usage**: 1 API call for task execution

## Environment Variables

- `INTEGRATION_TEST=1`: Required to run integration tests (prevents accidental execution)
- `GEOFFRUSSY_ZAI_API_KEY`: Alternative way to provide ZAI API key

## Troubleshooting

### Test Skips

If tests skip with message "Set INTEGRATION_TEST=1 to run", ensure you're setting the environment variable:
```bash
INTEGRATION_TEST=1 go test ...
```

### API Key Issues

If you see "ZAI API key not configured", check your config:
```bash
cat ~/.config/geoffrussy/config.yaml
```

Or set via environment:
```bash
export GEOFFRUSSY_ZAI_API_KEY="your-key-here"
```

### Timeout Issues

The tests have generous timeouts (5-10 minutes), but if you're on a slow connection:
```bash
INTEGRATION_TEST=1 go test -v -tags=integration -run TestFullPipelineZAI ./test/integration/... -timeout 20m
```

## CI/CD Integration

These tests are designed to be run manually or in CI with proper API key secrets. Do not run these tests as part of regular unit test suites to avoid:

1. Excessive API costs
2. Long build times
3. Flaky tests due to API availability

Example GitHub Actions workflow:
```yaml
- name: Run Pipeline Tests
  env:
    INTEGRATION_TEST: "1"
    GEOFFRUSSY_ZAI_API_KEY: ${{ secrets.ZAI_API_KEY }}
  run: go test -v -tags=integration ./test/integration/... -timeout 10m
```
