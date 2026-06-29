# Requirements Document: Geoffrussy AI Coding Agent

## Introduction

Geoffrussy is a next-generation AI-powered development orchestration platform that reimagines human-AI collaboration on software projects. The system prioritizes deep project understanding through a multi-stage iterative pipeline: Interview → Architecture Design → DevPlan Generation → Phase Review. Each stage includes review points where users can approve, iterate, or refine before proceeding. Built in Go as a single binary, Geoffrussy serves as a sophisticated development companion that transforms requirements into executable plans while maintaining transparency and control throughout the entire development lifecycle.

## Glossary

- **Geoffrussy**: The AI coding agent system
- **Interview_Engine**: Component that conducts guided discovery sessions with five phases
- **Design_Generator**: Component that generates system architecture from interview data
- **DevPlan**: Living documentation artifact containing executable development plans
- **DevPlan_Generator**: Component that converts architecture into 7-10 executable phases
- **Phase_Reviewer**: Component that validates and improves generated DevPlan phases
- **API_Bridge**: Component managing multi-model orchestration
- **Task_Executor**: Component executing development tasks
- **State_Store**: SQLite-based persistence layer
- **Configuration_Manager**: Component managing API keys, model preferences, and state
- **Token_Counter**: Component tracking API token usage
- **Cost_Estimator**: Component calculating and tracking costs
- **Git_Manager**: Component handling Git operations
- **Live_Monitor**: Component providing real-time execution streaming
- **Terminal_UI**: Interactive terminal interface component using Bubbletea
- **CLI**: Command-line interface component using Cobra
- **Build_System**: Component for compiling and packaging Geoffrussy
- **Distribution_System**: Component for releasing binaries
- **Checkpoint**: Saved state allowing rollback
- **Detour**: Mid-execution change to the development plan
- **Blocker**: Issue preventing task execution progress
- **Model_Provider**: External AI service (OpenAI, Anthropic, Ollama, OpenCode, Firmware.ai, Requesty.ai, Z.ai, Kimi, custom)
- **Phase**: A discrete, executable unit of the DevPlan containing 3-5 tasks
- **Reiteration**: Process of refining outputs at any pipeline stage before approval
- **Rate_Limit**: API provider restriction on number of requests per time period
- **Quota**: API provider restriction on total usage (tokens, cost, or requests)

## Requirements

### Requirement 1: System Initialization

**User Story:** As a developer, I want to initialize Geoffrussy in my project, so that I can configure the system and begin the development workflow.

#### Acceptance Criteria

1. WHEN a user runs the init command, THE Configuration_Manager SHALL create a configuration directory structure
2. WHEN initializing, THE Configuration_Manager SHALL prompt for API keys for each Model_Provider
3. WHEN API keys are provided, THE Configuration_Manager SHALL validate them against their respective Model_Provider
4. WHEN initialization completes, THE State_Store SHALL create an SQLite database for persistence
5. WHEN a project is already initialized, THE Configuration_Manager SHALL detect existing configuration and prevent re-initialization
6. THE Configuration_Manager SHALL store configuration in a standard location accessible across sessions

### Requirement 2: Interactive Interview Phase (Stage 1)

**User Story:** As a developer, I want to participate in an interactive interview with five distinct phases, so that Geoffrussy deeply understands my project requirements before generating code.

#### Acceptance Criteria

1. WHEN starting an interview, THE Interview_Engine SHALL conduct five sequential interview phases: Project Essence, Technical Constraints, Integration Points, Scope Definition, and Refinement & Validation
2. WHEN conducting Project Essence phase, THE Interview_Engine SHALL ask about problem statement, target users, success metrics, and core value proposition
3. WHEN conducting Technical Constraints phase, THE Interview_Engine SHALL ask about language preferences, performance requirements, scale expectations, and compliance needs
4. WHEN conducting Integration Points phase, THE Interview_Engine SHALL ask about external APIs, database types, authentication methods, and existing codebase
5. WHEN conducting Scope Definition phase, THE Interview_Engine SHALL ask about MVP features, timeline, resource constraints, and prioritization
6. WHEN conducting Refinement & Validation phase, THE Interview_Engine SHALL summarize all gathered information and request user confirmation
7. WHEN a user provides an answer, THE Interview_Engine SHALL analyze it and ask intelligent follow-up questions based on the response
8. WHEN appropriate, THE Interview_Engine SHALL propose reasonable defaults and ask for user agreement
9. WHEN the interview completes, THE Interview_Engine SHALL generate a structured JSON file containing all gathered information including problem statement, target users, technical stack, integrations, scope, constraints, assumptions, and unknowns
10. WHEN a user requests to pause, THE Interview_Engine SHALL save progress and allow resumption later
11. WHEN resuming an interview, THE Interview_Engine SHALL load previous responses and continue from the last question
12. WHEN a user requests reiteration, THE Interview_Engine SHALL allow refinement of any previous answers or asking new questions
13. WHEN the interview is complete, THE System SHALL present a review checkpoint asking user to approve, adjust, or continue
14. WHEN the interview JSON is approved, THE Git_Manager SHALL commit it to version control

### Requirement 3: Architecture Design Generation (Stage 2)

**User Story:** As a developer, I want Geoffrussy to generate a comprehensive system architecture from interview data, so that I have a clear technical design before creating the DevPlan.

#### Acceptance Criteria

1. WHEN interview data is approved, THE Design_Generator SHALL create a complete system architecture document
2. WHEN generating architecture, THE Design_Generator SHALL include system diagrams showing all components and their relationships
3. WHEN generating architecture, THE Design_Generator SHALL include component breakdown for frontend, backend, database, cache, queue, and monitoring
4. WHEN generating architecture, THE Design_Generator SHALL include data flow diagrams for key user journeys
5. WHEN generating architecture, THE Design_Generator SHALL include technology rationale explaining each choice
6. WHEN generating architecture, THE Design_Generator SHALL include scaling strategy for handling 10x growth
7. WHEN generating architecture, THE Design_Generator SHALL include API contract with key REST endpoints and WebSocket events
8. WHEN generating architecture, THE Design_Generator SHALL include database schema outline with tables and relationships
9. WHEN generating architecture, THE Design_Generator SHALL include security approach covering authentication, authorization, encryption, and audit
10. WHEN generating architecture, THE Design_Generator SHALL include observability strategy for logging, metrics, and tracing
11. WHEN generating architecture, THE Design_Generator SHALL include deployment architecture for development, staging, and production environments
12. WHEN generating architecture, THE Design_Generator SHALL include risk assessment identifying potential issues with probability, impact, and mitigation
13. WHEN generating architecture, THE Design_Generator SHALL include assumptions and unknowns sections
14. WHEN architecture is generated, THE Git_Manager SHALL commit it to version control
15. WHEN the architecture is complete, THE System SHALL present a review checkpoint asking user to approve, adjust, or continue
16. WHEN a user requests reiteration, THE Design_Generator SHALL allow refinement of technology choices, scaling strategy, database approach, real-time mechanisms, observability depth, or deployment targets

### Requirement 4: DevPlan Generation from Architecture (Stage 3)

**User Story:** As a developer, I want Geoffrussy to generate a comprehensive DevPlan from architecture and interview data, so that I have a clear, executable development roadmap organized into phases.

#### Acceptance Criteria

1. WHEN architecture is approved, THE DevPlan_Generator SHALL create 7-10 executable phases
2. WHEN generating phases, THE DevPlan_Generator SHALL ensure each phase builds on previous phases
3. WHEN generating phases, THE DevPlan_Generator SHALL ensure each phase results in deployable code
4. WHEN generating phases, THE DevPlan_Generator SHALL ensure each phase is completable in 1-2 hours by an LLM agent
5. WHEN generating phases, THE DevPlan_Generator SHALL include 3-5 actionable tasks per phase
6. WHEN generating phases, THE DevPlan_Generator SHALL follow the standard order: Setup & Infrastructure, Database & Models, Core API, Authentication & Authorization, Frontend Foundation, Real-time Sync, Integrations, Testing & Validation, Performance & Observability, Deployment & Hardening
7. WHEN generating a phase, THE DevPlan_Generator SHALL include objective, success criteria, dependencies, and estimated token usage
8. WHEN generating tasks, THE DevPlan_Generator SHALL include description, acceptance criteria, implementation notes, and blocker tracking placeholders
9. WHEN generating tasks, THE DevPlan_Generator SHALL include validation review checklists
10. WHEN the DevPlan is generated, THE Git_Manager SHALL commit all phase files to version control in a devplan directory
11. THE DevPlan_Generator SHALL format phases as both human-readable markdown and machine-parseable
12. WHEN generating tasks, THE DevPlan_Generator SHALL reference specific requirements from the interview
13. WHEN the DevPlan is complete, THE System SHALL present a review checkpoint asking user to approve, adjust, or continue
14. WHEN a user requests reiteration, THE DevPlan_Generator SHALL allow merging phases, splitting phases, or reordering phases
15. THE DevPlan_Generator SHALL create a master devplan.md overview file with phase summary and estimated total tokens and costs

### Requirement 5: Phase Review and Validation (Stage 4)

**User Story:** As a developer, I want Geoffrussy to review the generated DevPlan for issues and improvements, so that I can identify and fix problems before development begins.

#### Acceptance Criteria

1. WHEN the DevPlan is generated, THE Phase_Reviewer SHALL analyze each phase for clarity, completeness, dependencies, scope, risks, feasibility, testing, and integration
2. WHEN reviewing a phase, THE Phase_Reviewer SHALL identify issues and categorize them as critical, warning, or info
3. WHEN reviewing phases, THE Phase_Reviewer SHALL check for cross-phase issues and dependencies
4. WHEN review is complete, THE Phase_Reviewer SHALL generate a structured review report with all findings including severity breakdown
5. WHEN displaying review results, THE System SHALL show issue count by severity and list each issue with description and suggestion
6. WHEN improvements are suggested, THE System SHALL allow users to apply all improvements, apply selected improvements, or skip improvements
7. WHEN improvements are applied, THE DevPlan_Generator SHALL update phase files with the suggestions
8. WHEN improvements are applied, THE Git_Manager SHALL commit the updated phase files with message indicating review improvements
9. THE Phase_Reviewer SHALL provide specific, actionable suggestions for each identified issue
10. WHEN review is complete, THE System SHALL present a final checkpoint asking if user is ready to begin development

### Requirement 6: Multi-Model Support

**User Story:** As a developer, I want to select different AI models from multiple providers, so that I can optimize for cost, capability, and performance.

#### Acceptance Criteria

1. THE API_Bridge SHALL support OpenAI API integration
2. THE API_Bridge SHALL support Anthropic API integration
3. THE API_Bridge SHALL support Ollama local model integration
4. THE API_Bridge SHALL support OpenCode with dynamic model discovery
5. THE API_Bridge SHALL support Firmware.ai integration
6. THE API_Bridge SHALL support Requesty.ai integration
7. THE API_Bridge SHALL support Z.ai with coding plan capabilities
8. THE API_Bridge SHALL support Kimi with coding plan capabilities
9. THE API_Bridge SHALL support custom API endpoints
10. WHEN a user selects a model, THE Configuration_Manager SHALL validate the model is available
11. WHEN making API calls, THE API_Bridge SHALL use the appropriate authentication for each Model_Provider
12. WHEN a Model_Provider is unavailable, THE API_Bridge SHALL return a descriptive error
13. THE API_Bridge SHALL normalize responses from different Model_Provider formats into a common structure
14. WHEN using OpenCode, THE API_Bridge SHALL discover available models dynamically
15. WHEN a provider supports rate limiting information, THE API_Bridge SHALL track and store rate limit data
16. WHEN a provider supports quota information, THE API_Bridge SHALL track and store quota data

### Requirement 7: Model Selection Per Phase

**User Story:** As a developer, I want to choose which model to use for each development phase, so that I can balance cost and capability appropriately.

#### Acceptance Criteria

1. WHEN starting a phase, THE Configuration_Manager SHALL prompt the user to select a model
2. WHEN displaying model options, THE Configuration_Manager SHALL show available models with their capabilities and pricing
3. WHEN a model is selected, THE Configuration_Manager SHALL store the selection for that phase
4. WHEN executing a phase, THE Task_Executor SHALL use the model selected for that phase
5. THE Configuration_Manager SHALL allow users to set default models for each phase type
6. WHEN a previously selected model is unavailable, THE Configuration_Manager SHALL prompt for an alternative

### Requirement 8: Token Counting and Cost Estimation

**User Story:** As a developer, I want to track token usage and costs with detailed statistics, so that I can manage my API budget effectively and understand usage patterns.

#### Acceptance Criteria

1. WHEN an API call is made, THE Token_Counter SHALL count input tokens
2. WHEN an API response is received, THE Token_Counter SHALL count output tokens
3. WHEN tokens are counted, THE Cost_Estimator SHALL calculate cost based on the Model_Provider pricing
4. WHEN a phase completes, THE Cost_Estimator SHALL display total cost for that phase
5. THE Cost_Estimator SHALL maintain a running total of all costs across the project
6. WHEN costs are calculated, THE State_Store SHALL persist cost data
7. THE Cost_Estimator SHALL support setting budget limits with warnings when approaching limits
8. WHEN generating the DevPlan, THE Cost_Estimator SHALL provide estimated total cost for all phases
9. THE Token_Counter SHALL provide statistics including total tokens by provider, by phase, average per call, and peak usage
10. THE Cost_Estimator SHALL provide statistics including total cost by provider, by phase, average cost per call, most expensive call, and cost trends over time
11. THE System SHALL provide a stats command to display token and cost statistics
12. WHEN displaying statistics, THE System SHALL show breakdowns by provider and by phase
13. THE System SHALL track and display the most expensive API calls
14. THE System SHALL show cost trends over time with timestamps

### Requirement 8a: Rate Limiting and Quota Monitoring

**User Story:** As a developer, I want to monitor rate limits and quotas for all API providers, so that I can avoid hitting limits and plan my usage accordingly.

#### Acceptance Criteria

1. WHEN an API call returns rate limit information, THE API_Bridge SHALL extract and store the rate limit data
2. WHEN an API call returns quota information, THE API_Bridge SHALL extract and store the quota data
3. THE System SHALL provide a quota command to check rate limits and quotas for all configured providers
4. WHEN displaying quota information, THE System SHALL show requests remaining, requests limit, and reset time for each provider
5. WHEN displaying quota information, THE System SHALL show tokens remaining, tokens limit, cost remaining, cost limit, and reset time where available
6. WHEN approaching a rate limit, THE System SHALL warn the user before making additional calls
7. WHEN approaching a quota limit, THE System SHALL warn the user before making additional calls
8. THE State_Store SHALL persist rate limit and quota data with timestamps
9. WHEN rate limit or quota data is stale, THE System SHALL refresh it from the provider
10. THE System SHALL respect rate limits by delaying requests when necessary

### Requirement 9: Task Execution with Live Monitoring

**User Story:** As a developer, I want to see real-time progress during task execution, so that I understand what Geoffrussy is doing and can intervene if needed.

#### Acceptance Criteria

1. WHEN executing a task, THE Task_Executor SHALL stream output in real-time
2. WHEN streaming output, THE Live_Monitor SHALL display it in the terminal interface
3. WHEN a task starts, THE Live_Monitor SHALL display the task name and description
4. WHEN a task completes, THE Live_Monitor SHALL display completion status and duration
5. WHEN an error occurs, THE Live_Monitor SHALL display the error with context
6. THE Live_Monitor SHALL allow users to pause execution at any time
7. THE Live_Monitor SHALL allow users to skip the current task
8. WHEN execution is paused, THE Task_Executor SHALL save state and allow resumption

### Requirement 10: Git Integration

**User Story:** As a developer, I want Geoffrussy to integrate with Git, so that all changes are tracked and the DevPlan evolves as living documentation.

#### Acceptance Criteria

1. WHEN the interview JSON is created, THE Git_Manager SHALL commit it to the repository
2. WHEN the architecture document is created, THE Git_Manager SHALL commit it to the repository
3. WHEN the DevPlan is created, THE Git_Manager SHALL commit all phase files to the repository
4. WHEN the DevPlan is updated, THE Git_Manager SHALL commit the changes with descriptive messages
5. WHEN code is generated, THE Git_Manager SHALL stage changes for review
6. THE Git_Manager SHALL detect if the project is not a Git repository and offer to initialize one
7. WHEN creating commits, THE Git_Manager SHALL include metadata about which stage or task generated the changes
8. THE Git_Manager SHALL allow users to review changes before committing
9. WHEN conflicts are detected, THE Git_Manager SHALL notify the user and pause execution

### Requirement 11: Detour Support

**User Story:** As a developer, I want to request detours during execution, so that I can adapt the plan when requirements change or issues arise.

#### Acceptance Criteria

1. WHEN a user requests a detour, THE Task_Executor SHALL pause current execution
2. WHEN a detour is requested, THE Interview_Engine SHALL gather information about the requested change
3. WHEN detour information is gathered, THE DevPlan_Generator SHALL update the DevPlan with new tasks
4. WHEN the DevPlan is updated, THE Git_Manager SHALL commit the changes to the detours directory
5. WHEN a detour is complete, THE Task_Executor SHALL resume execution with the updated plan
6. THE DevPlan_Generator SHALL maintain task dependencies when inserting detour tasks
7. WHEN a detour conflicts with planned tasks, THE DevPlan_Generator SHALL notify the user and request resolution
8. THE DevPlan_Generator SHALL track all detours in a separate detours directory within the devplan

### Requirement 12: Blocker Detection and Resolution

**User Story:** As a developer, I want Geoffrussy to detect and help resolve blockers, so that execution can continue smoothly.

#### Acceptance Criteria

1. WHEN a task fails repeatedly, THE Task_Executor SHALL mark it as blocked
2. WHEN a blocker is detected, THE Task_Executor SHALL notify the user with context
3. WHEN a blocker is reported, THE Interview_Engine SHALL gather information about the issue
4. WHEN blocker information is gathered, THE Task_Executor SHALL attempt resolution strategies
5. WHEN a blocker cannot be auto-resolved, THE Task_Executor SHALL request user intervention
6. WHEN a user resolves a blocker, THE Task_Executor SHALL resume from the blocked task
7. THE State_Store SHALL persist blocker information for analysis
8. WHEN a blocker is encountered, THE Task_Executor SHALL record it in the phase file's blocker tracking section

### Requirement 13: Checkpoint System

**User Story:** As a developer, I want to create checkpoints and rollback if needed, so that I can safely experiment and recover from mistakes.

#### Acceptance Criteria

1. WHEN a phase completes, THE State_Store SHALL automatically create a checkpoint
2. WHEN a user requests a checkpoint, THE State_Store SHALL save the current state
3. WHEN creating a checkpoint, THE Git_Manager SHALL create a Git tag
4. WHEN listing checkpoints, THE State_Store SHALL display all available checkpoints with metadata
5. WHEN a user requests rollback, THE State_Store SHALL restore state to the selected checkpoint
6. WHEN rolling back, THE Git_Manager SHALL reset the repository to the checkpoint tag
7. THE State_Store SHALL preserve checkpoint history even after rollback

### Requirement 14: State Persistence

**User Story:** As a developer, I want all project state to be persisted, so that I can resume work across sessions without losing progress.

#### Acceptance Criteria

1. THE State_Store SHALL use SQLite for embedded persistence
2. WHEN state changes occur, THE State_Store SHALL persist them immediately
3. WHEN Geoffrussy starts, THE State_Store SHALL load existing state if available
4. THE State_Store SHALL persist interview responses
5. THE State_Store SHALL persist architecture document state
6. THE State_Store SHALL persist DevPlan state and task completion status
7. THE State_Store SHALL persist model selections and configuration
8. THE State_Store SHALL persist token usage and cost data
9. WHEN the database is corrupted, THE State_Store SHALL detect it and offer recovery options

### Requirement 15: CLI Interface

**User Story:** As a developer, I want a clean command-line interface, so that I can easily interact with Geoffrussy using familiar patterns.

#### Acceptance Criteria

1. THE CLI SHALL use the Cobra framework for command structure
2. THE CLI SHALL provide commands for init, interview, design, plan, review, execute, status, stats, quota, checkpoint, and rollback
3. WHEN a command is invoked, THE CLI SHALL validate required arguments
4. WHEN arguments are invalid, THE CLI SHALL display helpful error messages
5. THE CLI SHALL support global flags for verbosity and configuration paths
6. THE CLI SHALL provide help text for all commands and flags
7. WHEN a command completes, THE CLI SHALL exit with appropriate status codes
8. THE stats command SHALL display token usage and cost statistics
9. THE quota command SHALL display rate limits and quotas for all configured providers

### Requirement 16: Terminal UI

**User Story:** As a developer, I want an interactive terminal interface, so that I can navigate and control Geoffrussy efficiently.

#### Acceptance Criteria

1. THE Terminal_UI SHALL use the Bubbletea framework for interactive components
2. WHEN displaying the interview, THE Terminal_UI SHALL show progress and allow navigation
3. WHEN displaying execution, THE Terminal_UI SHALL show real-time streaming output
4. THE Terminal_UI SHALL support keyboard shortcuts for common actions
5. THE Terminal_UI SHALL display status information in a consistent header or footer
6. WHEN displaying lists, THE Terminal_UI SHALL support scrolling and selection
7. THE Terminal_UI SHALL handle terminal resize events gracefully
8. WHEN displaying review checkpoints, THE Terminal_UI SHALL present clear options for approve, adjust, or continue

### Requirement 17: Cross-Platform Distribution

**User Story:** As a developer, I want to install Geoffrussy as a single binary, so that I can use it on any platform without complex setup.

#### Acceptance Criteria

1. THE Build_System SHALL compile Geoffrussy to a single static binary
2. THE Build_System SHALL support compilation for Linux, macOS, and Windows
3. THE Build_System SHALL support both AMD64 and ARM64 architectures
4. WHEN the binary is executed, THE System SHALL not require external dependencies
5. THE Build_System SHALL embed SQLite as a static library
6. THE Distribution_System SHALL provide binaries through GitHub releases
7. THE Build_System SHALL include version information in the binary

### Requirement 18: Configuration Management

**User Story:** As a developer, I want to manage configuration easily, so that I can customize Geoffrussy's behavior for my workflow.

#### Acceptance Criteria

1. THE Configuration_Manager SHALL store configuration in a standard location per operating system
2. THE Configuration_Manager SHALL support configuration via config file
3. THE Configuration_Manager SHALL support configuration via environment variables
4. THE Configuration_Manager SHALL support configuration via command-line flags
5. WHEN multiple configuration sources exist, THE Configuration_Manager SHALL apply precedence rules (flags > env > file)
6. THE Configuration_Manager SHALL validate configuration values on load
7. WHEN configuration is invalid, THE Configuration_Manager SHALL display specific error messages

### Requirement 19: DevPlan Evolution

**User Story:** As a developer, I want the DevPlan to evolve as a living document, so that it reflects the actual development journey including detours and changes.

#### Acceptance Criteria

1. WHEN tasks are completed, THE DevPlan_Generator SHALL update task status in the DevPlan
2. WHEN detours are added, THE DevPlan_Generator SHALL insert new tasks in the detours directory with clear markers
3. WHEN the DevPlan changes, THE Git_Manager SHALL commit the updated version
4. THE DevPlan_Generator SHALL maintain a changelog section in the master devplan.md documenting all modifications
5. THE DevPlan_Generator SHALL preserve original plan structure while showing evolution
6. WHEN viewing the DevPlan, THE System SHALL highlight completed, in-progress, and pending tasks
7. THE DevPlan_Generator SHALL include timestamps for all modifications
8. THE DevPlan_Generator SHALL maintain an immutable decisions.md file tracking all architectural and design decisions

### Requirement 20: Error Handling and Recovery

**User Story:** As a developer, I want robust error handling, so that Geoffrussy can recover gracefully from failures.

#### Acceptance Criteria

1. WHEN an API call fails, THE API_Bridge SHALL retry with exponential backoff
2. WHEN retries are exhausted, THE API_Bridge SHALL return a descriptive error
3. WHEN a task fails, THE Task_Executor SHALL log the error with full context
4. WHEN a critical error occurs, THE System SHALL save state before exiting
5. WHEN resuming after an error, THE System SHALL offer to retry the failed operation
6. THE System SHALL distinguish between recoverable and non-recoverable errors
7. WHEN network errors occur, THE System SHALL provide offline-capable operations where possible

### Requirement 21: Progress Tracking and Status

**User Story:** As a developer, I want to check project status at any time, so that I understand progress and what remains.

#### Acceptance Criteria

1. WHEN a user requests status, THE System SHALL display current stage and phase
2. WHEN displaying status, THE System SHALL show completed, in-progress, and pending phases and tasks
3. WHEN displaying status, THE System SHALL show total token usage and costs
4. WHEN displaying status, THE System SHALL show time elapsed and estimated time remaining
5. THE System SHALL calculate completion percentage based on phase and task progress
6. WHEN displaying status, THE System SHALL show any active blockers
7. THE System SHALL allow filtering status by phase or component
8. WHEN displaying status, THE System SHALL indicate which pipeline stage is active (Interview, Design, DevPlan, Review, or Development)

### Requirement 22: Resume Capability

**User Story:** As a developer, I want to resume work from any point, so that I can stop and start without losing progress.

#### Acceptance Criteria

1. WHEN Geoffrussy starts, THE System SHALL detect incomplete work
2. WHEN incomplete work exists, THE System SHALL offer to resume from the last checkpoint
3. WHEN resuming, THE System SHALL restore all state including model selections and pipeline stage
4. WHEN resuming, THE System SHALL display a summary of what was completed
5. THE System SHALL allow resuming from any saved checkpoint, not just the latest
6. WHEN resuming an interview, THE System SHALL show previous answers
7. WHEN resuming execution, THE System SHALL continue from the next pending task
8. WHEN resuming from a pipeline stage, THE System SHALL allow continuing from that stage or restarting it

### Requirement 23: Pipeline Stage Navigation

**User Story:** As a developer, I want to navigate between pipeline stages, so that I can go back and refine earlier decisions if needed.

#### Acceptance Criteria

1. WHEN at any review checkpoint, THE System SHALL allow the user to go back to the previous stage
2. WHEN going back to a previous stage, THE System SHALL preserve the current work
3. WHEN returning from a previous stage, THE System SHALL regenerate dependent artifacts
4. THE System SHALL track the complete pipeline history including all iterations
5. WHEN navigating stages, THE Git_Manager SHALL commit all changes with clear stage markers
6. THE System SHALL prevent skipping stages unless all prerequisites are met
