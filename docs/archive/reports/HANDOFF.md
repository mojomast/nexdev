# Geoffrussy - Handoff Guide

## Overview

Geoffrussy is an AI-powered software development orchestration tool that helps you:
1. Interview project requirements
2. Generate architecture designs
3. Create development plans
4. Execute tasks with AI agent assistance

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/mojomast/geoffrussy.git
cd geoffrussy
make build
make install  # or just copy bin/geoffrussy to your PATH
```

### Configuration

Set up your AI providers:

```bash
# Configure Requesty (recommended)
geoffrussy config --set-key requesty YOUR_REQUESTY_API_KEY

# Configure Z.ai
geoffrussy config --set-key zai YOUR_ZAI_API_KEY

# View available providers and models
geoffrussy config --list-providers
```

## Workflow

### 1. Interview Phase

Gather requirements through guided questions:

```bash
geoffrussy interview
```

- Answer questions about your project
- Provide technical constraints
- Define success metrics
- Specify target users

### 2. Design Phase

Generate system architecture:

```bash
geoffrussy design --model openai/gpt-5-nano
```

- Creates comprehensive architecture document
- Defines system components
- Specifies technology stack
- Includes security and scaling strategies

### 3. Plan Phase

Break down into executable phases:

```bash
geoffrussy plan --model openai/gpt-5-nano
```

- Generates 7-10 development phases
- Each phase has 3-5 tasks
- Includes acceptance criteria
- Estimates token usage and cost

### 4. Develop Phase

Execute tasks with real-time monitoring:

```bash
geoffrussy develop --model openai/gpt-5-nano
```

- Executes tasks in order
- Generates actual code using LLM
- Writes files to disk
- Shows progress in TUI

**Keyboard controls:**
- `P` - Pause execution
- `R` - Resume execution
- `S` - Skip current task
- `Q` or `Ctrl+C` - Quit

## Project Structure

```
your-project/
├── .geoffrussy/
│   ├── state.db          # SQLite database
│   ├── architecture.json   # Generated architecture
│   └── config.yaml       # Project-specific config
├── backend/              # Generated code
├── frontend/             # Generated code
└── ...
```

## Configuration File (`~/.config/geoffrussy/config.yaml`)

```yaml
api_keys:
  requesty: "your-api-key"
  zai: "your-api-key"
  openai: "your-api-key"
  # Add more providers as needed

default_models:
  design: "openai/gpt-5-nano"
  devplan: "openai/gpt-5-nano"
  interview: "openai/gpt-5-nano"

favorite_models:
  - "openai/gpt-5-nano"
  - "glm-4.7"
  - "gpt-4"

budget_limit: 0  # Set to 0 for unlimited
verbose_logging: false
```

## Available Commands

### `geoffrussy init`
Initialize a new project in the current directory.

### `geoffrussy interview`
Run guided interview to gather requirements.

**Flags:**
- `--model` - Override default model

### `geoffrussy design`
Generate system architecture from interview data.

**Flags:**
- `--model` - Override default model
- `--refine` - Refine specific section

### `geoffrussy plan`
Generate development plan from architecture.

**Flags:**
- `--model` - Override default model
- `--merge` - Merge two phases (e.g., `1,2`)
- `--split` - Split a phase (e.g., `1:3`)
- `--reorder` - Reorder phases interactively

### `geoffrussy develop`
Execute tasks with AI agent assistance.

**Flags:**
- `--model` - Override default model
- `--phase` - Specific phase to execute

**TUI Controls:**
- `P` - Pause execution
- `R` - Resume execution
- `S` - Skip current task
- `Q` - Quit

### `geoffrussy status`
View project progress and statistics.

**Flags:**
- `--phase` - Show specific phase
- `--status` - Filter by status
- `--verbose` - Show detailed stats

### `geoffrussy config`
Manage configuration.

**Flags:**
- `--set-key` - Set API key for a provider
- `--list-providers` - List all available providers and models
- `--set-default` - Set default model for a stage

### `geoffrussy review`
Review completed work and generate summaries.

## Development Dashboard

When running `geoffrussy develop`, you'll see:

```
┌────────────────────────────────────────────────────────────┐
│  🚀 Geoffrussy Execution Monitor                  │
│                                                    │
│  Phase: Phase 2 - Core API                     │
│  Task: Implement API server skeleton                │
│                                                    │
│  Elapsed: 5m 23s                                │
│                                                    │
│  ████████████████████████████░░░░░░░░░░░  │
│                                                    │
│  22:35:06 ▶ Starting task...                    │
│  22:35:07 ✓ Completed task: API skeleton          │
│  22:35:07 ▶ Starting task: Auth middleware       │
│  22:35:07 ⋯ Executing task...                    │
│  22:35:08 ✓ Completed task: Auth middleware          │
└────────────────────────────────────────────────────┘
```

**Task Updates in TUI Viewport:**
- Task start notifications
- LLM call progress
- Token usage stats
- File creation confirmations
- Content previews (truncated)

## Troubleshooting

### "Unknown Model" Error

If you see this error, check:
1. Model name is supported by your provider
2. Config file has correct API key
3. Run `geoffrussy config --list-providers` to see available models

**Example fix:**
```bash
# List available models
geoffrussy config --list-providers

# Set correct model
geoffrussy config --set-default devplan glm-4.7
```

### Task Not Creating Files

If tasks complete but no code is generated:
1. Check TUI viewport for error messages
2. Review LLM response for JSON parsing issues
3. Try with `-v` flag for verbose output
4. Check provider API is accessible

### Dashboard Not Updating

If the TUI doesn't show progress:
1. Check terminal size (needs min 80x20)
2. Look for error messages in stdout
3. Try with `--verbose` flag
4. Ensure no other process is capturing stdout

## Best Practices

1. **Start from interview** - Let Geoffrussy understand your requirements first
2. **Review architecture** - Check generated architecture matches your vision
3. **Adjust plan** - Modify phases before development if needed
4. **Use specific models** - Different tasks may need different models
5. **Monitor execution** - Watch TUI for progress and catch errors early
6. **Review generated code** - Geoffrussy writes files, but review quality

## Model Recommendations

### For Interview & Planning
- `openai/gpt-5-nano` - Fast, good for structured responses
- `gpt-4o-mini` - Good balance of speed and quality
- `glm-4.7` - Latest Z.ai model, excellent for coding

### For Code Generation
- `glm-4.7` - Excellent coding capabilities
- `openai/gpt-4` - Well-tested code generation
- `claude-sonnet-4-5` - High-quality, careful code

### For Architecture Design
- `gpt-4` - Good for system design
- `claude-sonnet-4` - Thinks step-by-step

## Project Phases (Typical Flow)

1. **Phase 0: Setup & Infrastructure**
   - Project structure
   - Dependencies
   - Docker setup

2. **Phase 1: Database & Models**
   - Schema design
   - ORM models
   - Migrations

3. **Phase 2: Core API**
   - REST endpoints
   - WebSocket support
   - Message handling

4. **Phase 3: Authentication & Authorization**
   - JWT auth
   - User management
   - RBAC

5. **Phase 4: Frontend Foundation**
   - UI framework
   - API integration
   - Basic components

6. **Phase 5: Real-time Sync**
   - WebSocket implementation
   - Event handling
   - State management

7. **Phase 6: Integrations**
   - AI provider adapters
   - Tool integration
   - External APIs

8. **Phase 7: Testing & Validation**
   - Unit tests
   - Integration tests
   - E2E tests

9. **Phase 8: Performance & Observability**
   - Logging
   - Metrics
   - Monitoring

10. **Phase 9: Deployment & Hardening**
   - CI/CD setup
   - Security hardening
   - Production deployment

## Getting Help

```bash
# Get command help
geoffrussy [command] --help

# General help
geoffrussy --help
```

## Resources

- **GitHub Repository**: https://github.com/mojomast/geoffrussy
- **Requesty Models**: https://router.requesty.ai/v1/models
- **Z.ai Documentation**: https://docs.z.ai/guides/overview/quick-start
- **OpenAI API**: https://platform.openai.com/docs

## Support

For issues or questions:
1. Check existing GitHub issues
2. Review error messages carefully (they include context)
3. Enable verbose logging with `--verbose`
4. Check configuration in `~/.config/geoffrussy/config.yaml`

---

**Last Updated:** January 29, 2026
**Version:** 0.1.0
