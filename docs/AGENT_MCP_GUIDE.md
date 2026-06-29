# Geoffrussy MCP Server - Agent Guide

This guide is written for AI agents to help them effectively use Geoffrussy's MCP server to manage software development projects.

## Quick Reference

**Server Command:** `geoffrussy mcp-server --project-path /path/to/project`

**Available Tools:**
- **Monitoring:** `get_status`, `get_stats`, `list_phases`
- **Checkpointing:** `create_checkpoint`, `list_checkpoints`
- **Interview:** `run_interview`, `submit_interview_answer`
- **Design:** `generate_design`, `regenerate_design`
- **Planning:** `create_devplan`
- **Execution:** `execute_phase`, `execute_task`, `get_task_output`, `handle_blocker`

**Available Resources:**
- `project://status` - Project status (JSON)
- `project://current_question` - Current interview question (JSON)
- `project://interview` - Interview requirements (JSON)
- `project://architecture` - Architecture documentation (Markdown)
- `project://devplan` - Development plan (JSON)
- `project://phases` - All phases (JSON)
- `project://task_details` - Detailed task status and logs (JSON)
- `project://checkpoints` - All checkpoints (JSON)
- `project://stats` - Token usage statistics (JSON)

## Tool Usage Guide

### 1. Interview Tools

#### `run_interview`
Start or resume the project interview to gather requirements.
- **When to use:** Start of project or when requirements need update.
- **Parameters:** `projectPath` (required), `model` (optional), `resume` (optional bool).
- **Returns:** Current question and phase info.

#### `submit_interview_answer`
Submit answer to current interview question.
- **When to use:** After receiving a question from `run_interview` or `project://current_question`.
- **Parameters:** `projectPath`, `questionId`, `answer`.
- **Returns:** Confirmation and next question (or completion status).

---

### 2. Design Tools

#### `generate_design`
Generate system architecture from interview requirements.
- **When to use:** After interview is complete.
- **Parameters:** `projectPath`, `model` (optional), `regenerate` (optional bool).
- **Returns:** Summary of generated architecture.

#### `regenerate_design`
Regenerate architecture with guidance.
- **When to use:** To iterate on design or fix issues.
- **Parameters:** `projectPath`, `guidance` (optional).
- **Returns:** Updated architecture summary.

---

### 3. Planning Tools

#### `create_devplan`
Generate development plan with phases and tasks.
- **When to use:** After architecture is generated.
- **Parameters:** `projectPath`, `model` (optional).
- **Returns:** Summary of phases and tasks created.

---

### 4. Execution Tools

#### `execute_phase`
Execute all tasks in a development phase.
- **When to use:** To build the project phase-by-phase.
- **Parameters:** `projectPath`, `phaseId`, `model` (optional), `stopAfterPhase` (default true).
- **Returns:** Execution summary with success/fail counts.

#### `execute_task`
Execute a single task.
- **When to use:** To retry a failed task or run specific task.
- **Parameters:** `projectPath`, `taskId`, `model` (optional).
- **Returns:** Execution result and output snippet.

#### `get_task_output`
Get detailed output/logs for a task.
- **When to use:** To debug failed tasks.
- **Parameters:** `projectPath`, `taskId`.
- **Returns:** Full log content.

#### `handle_blocker`
Resolve blocked tasks.
- **When to use:** When a task is marked as blocked or failed.
- **Parameters:** `projectPath`, `taskId`, `action` ("retry", "skip", "modify", "analyze"), `modification` (optional).
- **Returns:** Result of resolution action.

---

### 5. Monitoring & Checkpoint Tools (Existing)

- `get_status`: Get project overview.
- `get_stats`: Get token/cost usage.
- `list_phases`: List phases status.
- `create_checkpoint`: Save state.
- `list_checkpoints`: List saved states.

## Resource Usage Guide

### `project://current_question`
Get currently active interview question.
- **Format:** JSON
- **Use:** To check if interview is pending input without triggering tool.

### `project://task_details`
Get detailed list of tasks including status, timestamps, and log snippets.
- **Format:** JSON
- **Use:** Deep dive into execution status.

*(Other resources `project://status`, `project://architecture`, etc. remain as documented previously)*

## Recommended Workflows

### 1. New Project (End-to-End)
1. `run_interview` -> Loop `submit_interview_answer` until complete.
2. `generate_design` -> Verify with `project://architecture`.
3. `create_devplan` -> Verify with `project://devplan`.
4. Loop: `execute_phase` for each phase ID.
   - Monitor with `project://status` or `list_phases`.
   - If error: `get_task_output` -> `handle_blocker` or `execute_task`.

### 2. Resume Development
1. `get_status` -> Check current phase.
2. `list_phases` -> Find next incomplete phase.
3. `execute_phase`.

### 3. Debugging Failure
1. `project://task_details` -> Identify failed task.
2. `get_task_output` -> Read error logs.
3. `handle_blocker` (action="analyze") -> Get AI analysis.
4. `execute_task` -> Retry specific task.

---

**Version:** 1.1 (Updated with Action Tools)
**Date:** 2026-01-30
