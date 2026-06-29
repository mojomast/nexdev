# Missing MCP Tools for Geoffrussy

**Document Version:** 1.0
**Date:** 2026-01-30
**Status:** Planning Document

---

## Executive Summary

Geoffrussy's MCP server currently provides **read-only monitoring tools** but lacks **action tools** needed for autonomous AI agents to build complete software projects. This document outlines the missing tools, their importance, and implementation specifications.

### Current State
✅ **Available:** 5 monitoring/checkpoint tools
❌ **Missing:** 7 core workflow tools
📊 **Coverage:** ~42% of full autonomous capability

---

## 1. Missing Tools Overview

| Tool Name | Priority | Workflow Stage | Blocking |
|-----------|----------|----------------|----------|
| `run_interview` | **CRITICAL** | Interview | Yes |
| `submit_interview_answer` | **CRITICAL** | Interview | Yes |
| `generate_design` | **CRITICAL** | Design | Yes |
| `regenerate_design` | HIGH | Design | No |
| `create_devplan` | **CRITICAL** | Planning | Yes |
| `execute_phase` | **CRITICAL** | Development | Yes |
| `execute_task` | HIGH | Development | No |
| `get_task_output` | MEDIUM | Development | No |
| `handle_blocker` | MEDIUM | Development | No |

---

## 2. Critical Missing Tools

### 2.1 `run_interview`

**Purpose:** Initiate or resume the project interview process to gather requirements.

**Why Critical:**
- **Blocker for all other stages** - Cannot proceed to design/planning without interview data
- **Foundation of project understanding** - All subsequent work depends on accurate requirements
- **Autonomous operation** - AI agents need programmatic access to start projects

**Current Workaround:** Manual CLI command `geoffrussy interview` (interactive TUI, not automatable)

**Proposed Implementation:**

```json
{
  "name": "run_interview",
  "description": "Start or resume the project interview to gather requirements through 5 phases",
  "inputSchema": {
    "type": "object",
    "properties": {
      "projectPath": {
        "type": "string",
        "description": "Absolute path to the project directory"
      },
      "model": {
        "type": "string",
        "description": "Model to use for interview (e.g., 'glm-4.7', 'gpt-4')",
        "default": "uses default from config"
      },
      "resume": {
        "type": "boolean",
        "description": "Resume existing interview if available",
        "default": false
      }
    },
    "required": ["projectPath"]
  }
}
```

**Return Format:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Interview Phase: 1 of 5\nPhase: Project Essence\nQuestion: What problem does your project solve?\n\nProvide your answer using the submit_interview_answer tool."
    }
  ],
  "metadata": {
    "phase": 1,
    "totalPhases": 5,
    "phaseTitle": "Project Essence",
    "questionId": "problem_statement",
    "isComplete": false
  }
}
```

**Behavior:**
1. Check if interview already exists (resume capability)
2. Initialize interview engine with specified model
3. Start interview at phase 1 or resume from saved position
4. Return current question to agent
5. Agent must call `submit_interview_answer` to proceed

---

### 2.2 `submit_interview_answer`

**Purpose:** Submit an answer to the current interview question and receive the next question.

**Why Critical:**
- **Enables autonomous interview completion** - Agents can complete entire interview without human intervention
- **Structured data collection** - Ensures all required information is gathered
- **Validation and feedback** - Can validate answers and request clarification

**Current Workaround:** None - interactive TUI only

**Proposed Implementation:**

```json
{
  "name": "submit_interview_answer",
  "description": "Submit an answer to the current interview question and proceed to next question",
  "inputSchema": {
    "type": "object",
    "properties": {
      "projectPath": {
        "type": "string",
        "description": "Absolute path to the project directory"
      },
      "questionId": {
        "type": "string",
        "description": "ID of the question being answered (from run_interview response)"
      },
      "answer": {
        "type": "string",
        "description": "The answer to the current question"
      }
    },
    "required": ["projectPath", "questionId", "answer"]
  }
}
```

**Return Format:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "✅ Answer recorded\n\nInterview Phase: 1 of 5\nPhase: Project Essence\nQuestion: Who are your target users?\n\nProvide your answer using submit_interview_answer."
    }
  ],
  "metadata": {
    "phase": 1,
    "totalPhases": 5,
    "phaseTitle": "Project Essence",
    "questionId": "target_users",
    "previousAnswer": "recorded",
    "isComplete": false
  }
}
```

**When interview is complete:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "✅ Interview Complete!\n\nAll 5 phases completed. Requirements have been saved.\n\nNext step: Run generate_design to create system architecture."
    }
  ],
  "metadata": {
    "isComplete": true,
    "phasesCompleted": 5,
    "totalQuestions": 23,
    "answersRecorded": 23
  }
}
```

**Behavior:**
1. Validate answer is for current question
2. Save answer to database
3. Advance to next question in current phase or next phase
4. Return next question or completion status
5. If complete, save interview requirements to database

---

### 2.3 `generate_design`

**Purpose:** Generate system architecture from completed interview requirements.

**Why Critical:**
- **Blueprint for development** - Creates the technical architecture that guides all coding
- **Technology decisions** - Selects frameworks, databases, patterns
- **Autonomous planning** - Agents need architecture before creating devplan

**Current Workaround:** Manual CLI command `geoffrussy design` (not automatable)

**Proposed Implementation:**

```json
{
  "name": "generate_design",
  "description": "Generate system architecture from interview requirements",
  "inputSchema": {
    "type": "object",
    "properties": {
      "projectPath": {
        "type": "string",
        "description": "Absolute path to the project directory"
      },
      "model": {
        "type": "string",
        "description": "Model to use for architecture generation",
        "default": "uses default from config"
      },
      "regenerate": {
        "type": "boolean",
        "description": "Regenerate architecture if one already exists",
        "default": false
      }
    },
    "required": ["projectPath"]
  }
}
```

**Return Format:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "🏗️ Architecture Generation Complete\n\nGenerated comprehensive system architecture including:\n- System Overview\n- 12 Components\n- 8 Data Flows\n- Technology Rationale\n- Scaling Strategy\n- API Contract (10 endpoints)\n- Database Schema (15 tables)\n- Security Approach\n- Observability Strategy\n- Deployment Architecture\n- Risk Assessment\n\nArchitecture saved to: .geoffrussy/architecture.json\nView with: project://architecture resource\n\nNext step: Run create_devplan to generate development phases."
    }
  ],
  "metadata": {
    "architectureGenerated": true,
    "componentsCount": 12,
    "endpointsCount": 10,
    "tablesCount": 15,
    "tokensUsed": 45000,
    "cost": 0.23
  }
}
```

**Behavior:**
1. Check that interview is complete
2. Load interview requirements from database
3. Call AI model to generate architecture (using design prompts)
4. Save architecture to `.geoffrussy/architecture.json`
5. Update project stage to "design_complete"
6. Return summary with token usage

---

### 2.4 `create_devplan`

**Purpose:** Generate executable development plan with phases and tasks.

**Why Critical:**
- **Execution roadmap** - Breaks architecture into actionable phases
- **Task breakdown** - Creates specific tasks with dependencies
- **Progress tracking** - Enables monitoring and checkpoint creation

**Current Workaround:** Manual CLI command `geoffrussy plan` (not automatable)

**Proposed Implementation:**

```json
{
  "name": "create_devplan",
  "description": "Generate development plan with 7-10 phases and 3-5 tasks each",
  "inputSchema": {
    "type": "object",
    "properties": {
      "projectPath": {
        "type": "string",
        "description": "Absolute path to the project directory"
      },
      "model": {
        "type": "string",
        "description": "Model to use for devplan generation",
        "default": "uses default from config"
      }
    },
    "required": ["projectPath"]
  }
}
```

**Return Format:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "📋 DevPlan Generation Complete\n\nGenerated development plan with:\n- 8 phases\n- 32 tasks total\n- Average 4 tasks per phase\n- Estimated duration: 2-3 weeks\n\nPhases:\n  Phase 0: Setup & Infrastructure (5 tasks)\n  Phase 1: Core Game Engine (4 tasks)\n  Phase 2: Game Implementations (5 tasks)\n  Phase 3: UI/UX & Navigation (4 tasks)\n  Phase 4: State Management (3 tasks)\n  Phase 5: Polish & Effects (4 tasks)\n  Phase 6: Testing & Validation (4 tasks)\n  Phase 7: Deployment (3 tasks)\n\nNext step: Run execute_phase to start development."
    }
  ],
  "metadata": {
    "devplanGenerated": true,
    "phasesCount": 8,
    "tasksCount": 32,
    "tokensUsed": 38000,
    "cost": 0.19
  }
}
```

**Behavior:**
1. Check that architecture exists
2. Load architecture from database
3. Call AI model to generate devplan (7-10 phases with tasks)
4. Save phases and tasks to database
5. Update project stage to "plan_complete"
6. Return summary

---

### 2.5 `execute_phase`

**Purpose:** Execute all tasks in a specific development phase.

**Why Critical:**
- **Core development automation** - Actually writes the code
- **Autonomous operation** - Agents can build entire projects
- **Progress monitoring** - Real-time status updates

**Current Workaround:** Manual CLI command `geoffrussy develop --phase <id>` (not automatable)

**Proposed Implementation:**

```json
{
  "name": "execute_phase",
  "description": "Execute all tasks in a development phase, writing code and creating files",
  "inputSchema": {
    "type": "object",
    "properties": {
      "projectPath": {
        "type": "string",
        "description": "Absolute path to the project directory"
      },
      "phaseId": {
        "type": "string",
        "description": "ID of the phase to execute (e.g., 'phase-0', 'phase-1')"
      },
      "model": {
        "type": "string",
        "description": "Model to use for task execution",
        "default": "uses default from config"
      },
      "stopAfterPhase": {
        "type": "boolean",
        "description": "Stop after completing this phase (don't auto-continue)",
        "default": true
      },
      "streamOutput": {
        "type": "boolean",
        "description": "Stream task output in real-time (requires streaming support)",
        "default": false
      }
    },
    "required": ["projectPath", "phaseId"]
  }
}
```

**Return Format (Synchronous):**
```json
{
  "content": [
    {
      "type": "text",
      "text": "✅ Phase 0: Setup & Infrastructure - COMPLETED\n\nTasks executed:\n  ✅ Task 0.1: Initialize project structure (2m 34s)\n  ✅ Task 0.2: Setup package.json and dependencies (1m 12s)\n  ✅ Task 0.3: Create HTML template (45s)\n  ✅ Task 0.4: Setup CSS framework (1m 8s)\n  ✅ Task 0.5: Create build configuration (58s)\n\nPhase Summary:\n- Total duration: 6m 37s\n- Files created: 12\n- Files modified: 3\n- Tokens used: 15,420\n- Cost: $0.08\n\nNext phase: Phase 1: Core Game Engine"
    }
  ],
  "metadata": {
    "phaseCompleted": true,
    "phaseId": "phase-0",
    "tasksExecuted": 5,
    "tasksSucceeded": 5,
    "tasksFailed": 0,
    "filesCreated": 12,
    "filesModified": 3,
    "duration": 397,
    "tokensUsed": 15420,
    "cost": 0.08,
    "nextPhase": "phase-1"
  }
}
```

**Behavior:**
1. Load phase and tasks from database
2. Check phase dependencies are met
3. Execute each task in sequence using executor
4. Update task status in database (in_progress → completed/failed)
5. Create checkpoint after phase completion
6. Return summary with metrics
7. If stopAfterPhase=false, automatically continue to next phase

**Error Handling:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "⚠️ Phase 1: Core Game Engine - PARTIALLY COMPLETED\n\nTasks executed:\n  ✅ Task 1.1: Create game base class (2m 15s)\n  ✅ Task 1.2: Implement canvas renderer (3m 42s)\n  ❌ Task 1.3: Add input handling - FAILED\n     Error: Missing dependency 'event-handler.js'\n  ⏸️ Task 1.4: Create animation loop - BLOCKED\n\nPhase blocked. Use handle_blocker tool to resolve issues."
    }
  ],
  "metadata": {
    "phaseCompleted": false,
    "phaseBlocked": true,
    "tasksSucceeded": 2,
    "tasksFailed": 1,
    "tasksBlocked": 1,
    "blockerReason": "Missing dependency 'event-handler.js'"
  }
}
```

---

## 3. High Priority Tools

### 3.1 `regenerate_design`

**Purpose:** Regenerate architecture with modifications or improvements.

**Use Cases:**
- Initial architecture needs refinement
- Technology stack changes
- Scaling requirements changed

**Implementation:**
```json
{
  "name": "regenerate_design",
  "description": "Regenerate architecture with optional guidance for modifications",
  "inputSchema": {
    "type": "object",
    "properties": {
      "projectPath": {"type": "string"},
      "guidance": {
        "type": "string",
        "description": "Specific changes or improvements to make"
      },
      "preserveComponents": {
        "type": "boolean",
        "description": "Preserve existing components where possible",
        "default": true
      }
    },
    "required": ["projectPath"]
  }
}
```

---

### 3.2 `execute_task`

**Purpose:** Execute a single task independently (without full phase execution).

**Use Cases:**
- Retry failed task
- Skip to specific task
- Manual intervention during development

**Implementation:**
```json
{
  "name": "execute_task",
  "description": "Execute a single task by ID",
  "inputSchema": {
    "type": "object",
    "properties": {
      "projectPath": {"type": "string"},
      "taskId": {
        "type": "string",
        "description": "ID of task to execute (e.g., 'task-1.3')"
      },
      "model": {"type": "string"}
    },
    "required": ["projectPath", "taskId"]
  }
}
```

---

### 3.3 `get_task_output`

**Purpose:** Retrieve detailed output from a completed or running task.

**Use Cases:**
- Debug failed tasks
- Review what was created
- Validate task completion

**Implementation:**
```json
{
  "name": "get_task_output",
  "description": "Get detailed execution output for a task",
  "inputSchema": {
    "type": "object",
    "properties": {
      "projectPath": {"type": "string"},
      "taskId": {"type": "string"}
    },
    "required": ["projectPath", "taskId"]
  }
}
```

**Return Format:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Task 0.3: Create HTML template\nStatus: completed\nDuration: 45s\n\nFiles Created:\n- index.html\n- 404.html\n\nOutput:\nCreated index.html with responsive layout...\nAdded meta tags for SEO...\nImplemented game container div...\n\nTokens: 2,340 | Cost: $0.01"
    }
  ]
}
```

---

## 4. Medium Priority Tools

### 4.1 `handle_blocker`

**Purpose:** Provide resolution guidance for blocked tasks.

**Implementation:**
```json
{
  "name": "handle_blocker",
  "description": "Attempt to resolve a blocker or get guidance on resolution",
  "inputSchema": {
    "type": "object",
    "properties": {
      "projectPath": {"type": "string"},
      "taskId": {"type": "string"},
      "action": {
        "type": "string",
        "enum": ["retry", "skip", "modify", "analyze"],
        "description": "Action to take with blocker"
      },
      "modification": {
        "type": "string",
        "description": "Modified task description if action is 'modify'"
      }
    },
    "required": ["projectPath", "taskId", "action"]
  }
}
```

---

## 5. New Resources to Add

### 5.1 `project://requirements`

**Current:** Available as `project://interview`
**Status:** ✅ Already exists

---

### 5.2 `project://current_question`

**Purpose:** Get the current interview question without starting interview.

**Schema:**
```json
{
  "uri": "project://current_question",
  "mimeType": "application/json",
  "description": "Current interview question if interview is in progress"
}
```

**Returns:**
```json
{
  "phase": 2,
  "phaseTitle": "Technical Constraints",
  "questionId": "language_preference",
  "question": "What programming language(s) do you want to use?",
  "isComplete": false
}
```

---

### 5.3 `project://task_details`

**Purpose:** Get detailed information about all tasks with their outputs.

**Schema:**
```json
{
  "uri": "project://task_details",
  "mimeType": "application/json",
  "description": "Detailed task information including outputs and status"
}
```

---

## 6. Implementation Priorities

### Phase 1: Interview Tools (Week 1)
- [ ] `run_interview`
- [ ] `submit_interview_answer`
- [ ] `project://current_question` resource

**Deliverable:** Agents can complete full interview autonomously

---

### Phase 2: Design Tools (Week 2)
- [ ] `generate_design`
- [ ] `regenerate_design`

**Deliverable:** Agents can generate and refine architecture

---

### Phase 3: Planning Tools (Week 3)
- [ ] `create_devplan`

**Deliverable:** Agents can create executable development plans

---

### Phase 4: Execution Tools (Week 4-5)
- [ ] `execute_phase`
- [ ] `execute_task`
- [ ] `get_task_output`
- [ ] `handle_blocker`
- [ ] `project://task_details` resource

**Deliverable:** Agents can build complete projects autonomously

---

## 7. Technical Implementation Notes

### Database Schema Extensions

**New table: `interview_state`**
```sql
CREATE TABLE interview_state (
  project_id TEXT PRIMARY KEY,
  current_phase INTEGER,
  current_question_id TEXT,
  questions_answered INTEGER,
  is_complete BOOLEAN,
  updated_at TIMESTAMP
);
```

**New table: `interview_answers`**
```sql
CREATE TABLE interview_answers (
  id TEXT PRIMARY KEY,
  project_id TEXT,
  phase INTEGER,
  question_id TEXT,
  answer TEXT,
  created_at TIMESTAMP,
  FOREIGN KEY (project_id) REFERENCES projects(id)
);
```

---

### MCP Handler Structure

Each tool should follow this pattern:

```go
// internal/mcp/interview_handlers.go
func (h *InterviewHandlers) handleRunInterview(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
    projectPath, err := ValidateAndGetString(args, "projectPath", true)
    if err != nil {
        return ErrorResult(err.Error()), nil
    }

    // Open state store
    store, err := openStateStore(projectPath)
    if err != nil {
        return ErrorResult(err.Error()), nil
    }
    defer store.Close()

    // Initialize interview engine
    engine := interview.NewEngine(store, h.configManager)

    // Start or resume interview
    question, err := engine.GetCurrentQuestion(projectID)
    if err != nil {
        return ErrorResult(err.Error()), nil
    }

    // Format response
    return formatInterviewQuestion(question), nil
}
```

---

### Progress Streaming (Future Enhancement)

For long-running operations like `execute_phase`, consider adding streaming support:

```json
{
  "name": "execute_phase",
  "supportsStreaming": true
}
```

**Streaming format:**
```json
{"type": "progress", "task": "task-0.1", "status": "in_progress"}
{"type": "progress", "task": "task-0.1", "status": "completed"}
{"type": "progress", "task": "task-0.2", "status": "in_progress"}
...
{"type": "result", "phaseCompleted": true, ...}
```

---

## 8. Success Metrics

Once all tools are implemented, measure:

- ✅ **Full autonomy:** Agent can go from empty directory to working project with zero human intervention
- ✅ **Error recovery:** Agent can handle blockers and retry failed tasks
- ✅ **Observability:** Complete visibility into interview, design, planning, and execution
- ✅ **Cost tracking:** Token usage and costs tracked at every step
- ✅ **Quality:** Generated projects meet architecture specifications

---

## 9. Example Autonomous Workflow

Once all tools are implemented, an AI agent can:

```
1. Call: run_interview(projectPath="/path/to/game-arcade")
   → Receives first question

2. Loop: submit_interview_answer(questionId, answer)
   → Completes all 5 phases (23 questions)

3. Call: generate_design(projectPath)
   → Architecture created

4. Call: create_devplan(projectPath)
   → 8 phases with 32 tasks created

5. Loop: execute_phase(phaseId="phase-0" through "phase-7")
   → Project fully built

6. Monitor: get_status(), list_phases(), get_stats()
   → Track progress and costs

7. Checkpoint: create_checkpoint(name="v1.0-complete")
   → Save final state

Total: ZERO human intervention required
```

---

## 10. Conclusion

Adding these 7-9 missing tools will transform Geoffrussy's MCP server from a **monitoring interface** into a **full autonomous development platform**.

**Current capability:** ~42% (5/12 tools)
**With missing tools:** 100% (12/12 tools)
**Impact:** AI agents can build complete, production-ready projects autonomously

**Estimated implementation time:** 4-5 weeks
**Complexity:** Medium-High (requires integration with existing CLI commands)
**ROI:** Extremely high - enables true autonomous software development

---

**Document prepared for:** Geoffrussy MCP Enhancement
**Prepared by:** Claude Sonnet 4.5
**Date:** 2026-01-30
