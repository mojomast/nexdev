# Implementation Plan: Geoffrussy AI Coding Agent

## Overview

This implementation plan breaks down the Geoffrussy system into discrete, executable phases. Each phase builds on previous work and results in deployable, testable code. The plan follows a bottom-up approach: infrastructure first, then core components, then integration, and finally polish.

## Tasks

- [x] 1. Project Setup and Infrastructure
  - Initialize Go project with proper module structure
  - Set up development environment with Docker
  - Configure CI/CD pipeline with GitHub Actions
  - Create basic project documentation
  - _Requirements: 1.1, 1.4, 15.1, 17.1, 17.2, 17.3_

- [ ] 2. State Store Implementation (SQLite)
  - [x] 2.1 Create database schema and migrations
    - Define all tables (projects, interview_data, architectures, phases, tasks, checkpoints, token_usage, rate_limits, quotas, token_stats_cache, blockers, config)
    - Implement migration system
    - _Requirements: 1.4, 14.1_
  
  - [x] 2.2 Implement State Store interface
    - Create StateStore struct with SQLite connection
    - Implement project operations (Create, Get, Update)
    - Implement interview data operations (Save, Get)
    - Implement architecture operations (Save, Get)
    - Implement phase operations (Save, Get, List, UpdateStatus)
    - Implement task operations (Save, Get, UpdateStatus)
    - _Requirements: 14.1, 14.2, 14.3, 14.4, 14.5, 14.6, 14.7_
  
  - [x] 2.3 Write property test for state persistence round-trip
    - **Property 4: State Preservation Round-Trip**
    - **Validates: Requirements 14.1, 14.2, 14.3**
  
  - [x] 2.4 Write unit tests for State Store
    - Test database initialization
    - Test CRUD operations for each entity
    - Test error handling for corrupted database
    - _Requirements: 14.8_

- [ ] 3. Configuration Manager
  - [x] 3.1 Implement configuration loading from multiple sources
    - Load from config file (~/.geoffrussy/config.yaml)
    - Load from environment variables
    - Load from command-line flags
    - Apply precedence rules (flags > env > file)
    - _Requirements: 18.1, 18.2, 18.3, 18.4, 18.5_
  
  - [x] 3.2 Implement API key management
    - Store API keys securely
    - Validate API keys against providers
    - _Requirements: 1.2, 1.3_
  
  - [ ]* 3.3 Write property test for configuration precedence
    - **Property 16: Configuration Precedence**
    - **Validates: Requirements 18.5**
  
  - [ ]* 3.4 Write unit tests for configuration validation
    - Test invalid configuration detection
    - Test error messages
    - _Requirements: 18.6, 18.7_

- [x] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.


- [ ] 5. API Bridge and Provider Integration
  - [x] 5.1 Create Provider interface and base implementation
    - Define Provider interface
    - Create base provider struct with common functionality
    - Implement retry with exponential backoff
    - _Requirements: 6.11, 20.1, 20.2_
  
  - [x] 5.2 Implement OpenAI provider
    - Authenticate with API key
    - List available models
    - Make API calls with streaming support
    - Extract rate limit and quota information
    - _Requirements: 6.1, 6.15, 6.16_
  
  - [x] 5.3 Implement Anthropic provider
    - Authenticate with API key
    - List available models
    - Make API calls with streaming support
    - Extract rate limit and quota information
    - _Requirements: 6.2, 6.15, 6.16_
  
  - [x] 5.4 Implement Ollama provider
    - Connect to local Ollama instance
    - List available models
    - Make API calls with streaming support
    - _Requirements: 6.3_
  
  - [x] 5.5 Implement OpenCode provider with dynamic discovery
    - Figure out how to use opencode run to use opencode for tasks
    - Discover available models dynamically
    - ensure opencode run can be used instead of api calls when opencode provider is selected.
    - _Requirements: 6.4, 6.14_
  
  - [x] 5.6 Implement Firmware.ai provider
    - Authenticate with API key
    - List available models
    - Make API calls with streaming support
    - Extract rate limit and quota information
    - _Requirements: 6.5, 6.15, 6.16_
  
  - [x] 5.7 Implement Requesty.ai provider
    - Authenticate with API key
    - List available models
    - Make API calls with streaming support
    - Extract rate limit and quota information
    - _Requirements: 6.6, 6.15, 6.16_
  
  - [x] 5.8 Implement Z.ai provider with coding plan support
    - Authenticate with API key
    - List available models
    - Make API calls with coding plan capabilities
    - Extract rate limit and quota information
    - _Requirements: 6.7, 6.15, 6.16_
  
  - [x] 5.9 Implement Kimi provider with coding plan support
    - Authenticate with API key
    - List available models
    - Make API calls with coding plan capabilities
    - Extract rate limit and quota information
    - _Requirements: 6.8, 6.15, 6.16_
  
  - [x] 5.10 Implement API Bridge
    - Normalize responses from different providers
    - Handle provider selection
    - Validate models
    - Track rate limits and quotas
    - _Requirements: 6.10, 6.12, 6.13, 6.15, 6.16_
  
  - [ ]* 5.11 Write property test for multi-provider support
    - **Property 27: Multi-Provider Support**
    - **Validates: Requirements 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8**
  
  - [ ]* 5.12 Write property test for API retry with backoff
    - **Property 19: API Retry Exponential Backoff**
    - **Validates: Requirements 20.1, 20.2**
  
  - [ ]* 5.13 Write unit tests for each provider
    - Test authentication
    - Test model listing
    - Test API calls (with mocking)
    - Test error handling
    - _Requirements: 6.12_

- [ ] 6. Token Counter and Cost Estimator
  - [x] 6.1 Implement Token Counter
    - Count tokens for different models
    - Estimate tokens for text
    - Calculate token statistics (total, by provider, by phase, average, peak)
    - _Requirements: 8.1, 8.2, 8.9_
  
  - [x] 6.2 Implement Cost Estimator
    - Calculate costs based on provider pricing
    - Track costs by phase and provider
    - Calculate cost statistics (total, by provider, by phase, average, most expensive, trends)
    - Support budget limits with warnings
    - _Requirements: 8.3, 8.4, 8.5, 8.6, 8.7, 8.8, 8.10, 8.13, 8.14_
  
  - [x] 6.3 Integrate with State Store
    - Persist token usage records
    - Persist rate limit data
    - Persist quota data
    - Cache token statistics
    - _Requirements: 8.6, 8a.8_
  
  - [ ]* 6.4 Write property test for token cost calculation
    - **Property 11: Token Cost Calculation**
    - **Validates: Requirements 8.1, 8.2, 8.3**
  
  - [ ]* 6.5 Write property test for token statistics aggregation
    - **Property 24: Token Statistics Aggregation**
    - **Validates: Requirements 8.9, 8.12**
  
  - [ ]* 6.6 Write property test for cost statistics accuracy
    - **Property 25: Cost Statistics Accuracy**
    - **Validates: Requirements 8.10, 8.12**
  
  - [ ]* 6.7 Write unit tests for budget limit warnings
    - Test warning when approaching limit
    - Test blocking when limit exceeded
    - _Requirements: 8.7_

- [x] 7. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.


- [ ] 8. Git Manager
  - [x] 8.1 Implement Git operations
    - Initialize repository
    - Check if directory is a repository
    - Commit files with metadata
    - Stage changes
    - Create tags for checkpoints
    - Reset to tags for rollback
    - Get repository status
    - Detect conflicts
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8, 10.9_
  
  - [ ]* 8.2 Write property test for git commit integrity
    - **Property 12: Git Commit Integrity**
    - **Validates: Requirements 10.1, 10.2, 10.3, 10.5**
  
  - [ ]* 8.3 Write unit tests for Git Manager
    - Test repository initialization
    - Test commit creation
    - Test tag creation and reset
    - Test conflict detection
    - _Requirements: 10.4, 10.9_

- [ ] 9. Interview Engine
  - [x] 9.1 Implement interview question flow
    - Define five interview phases
    - Create question templates for each phase
    - Implement question sequencing
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_
  
  - [x] 9.2 Implement LLM-powered follow-up questions
    - Analyze user answers
    - Generate intelligent follow-ups
    - Propose reasonable defaults
    - _Requirements: 2.7, 2.8_
  
  - [x] 9.3 Implement interview state management
    - Save progress
    - Resume from saved state
    - Allow reiteration of answers
    - _Requirements: 2.10, 2.11, 2.12_
  
  - [x] 9.4 Implement interview summary and JSON export
    - Summarize gathered information
    - Generate structured JSON output
    - Validate completeness
    - _Requirements: 2.6, 2.9_
  
  - [ ]* 9.5 Write property test for interview data completeness
    - **Property 3: Interview Data Completeness**
    - **Validates: Requirements 2.9**
  
  - [ ]* 9.6 Write property test for interview reiteration preservation
    - **Property 5: Interview Reiteration Preservation**
    - **Validates: Requirements 2.12**
  
  - [ ]* 9.7 Write unit tests for interview flow
    - Test each phase execution
    - Test pause and resume
    - Test reiteration
    - _Requirements: 2.10, 2.11, 2.12_

- [x] 10. Design Generator
  - [x] 10.1 Implement architecture generation
    - Generate system overview and diagrams
    - Generate component breakdown
    - Generate data flow diagrams
    - Generate technology rationale
    - Generate scaling strategy
    - Generate API contract
    - Generate database schema
    - Generate security approach
    - Generate observability strategy
    - Generate deployment architecture
    - Generate risk assessment
    - Document assumptions and unknowns
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 3.9, 3.10, 3.11, 3.12, 3.13_
  
  - [x] 10.2 Implement architecture reiteration
    - Allow refinement of technology choices
    - Allow refinement of scaling strategy
    - Allow refinement of other architectural decisions
    - _Requirements: 3.16_
  
  - [ ]* 10.3 Write property test for architecture document completeness
    - **Property 6: Architecture Document Completeness**
    - **Validates: Requirements 3.1-3.13**
  
  - [ ]* 10.4 Write unit tests for architecture generation
    - Test each section generation
    - Test reiteration
    - _Requirements: 3.16_

- [x] 11. DevPlan Generator
  - [x] 11.1 Implement phase generation
    - Generate 7-10 phases from architecture
    - Ensure phases build on each other
    - Include 3-5 tasks per phase
    - Include objective, success criteria, dependencies
    - Estimate token usage and costs
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8, 4.10, 4.11_
  
  - [x] 11.2 Implement phase manipulation
    - Merge two phases
    - Split a phase
    - Reorder phases
    - _Requirements: 4.14_
  
  - [x] 11.3 Implement DevPlan export
    - Generate phase markdown files
    - Generate master devplan.md overview
    - Commit to Git
    - _Requirements: 4.9, 4.15_
  
  - [ ]* 11.4 Write property test for DevPlan phase structure
    - **Property 7: DevPlan Phase Structure**
    - **Validates: Requirements 4.1, 4.2, 4.3, 4.4, 4.5, 4.7, 4.8**
  
  - [ ]* 11.5 Write property test for phase dependency ordering
    - **Property 8: Phase Dependency Ordering**
    - **Validates: Requirements 4.2, 4.6**
  
  - [ ]* 11.6 Write property test for phase merge preservation
    - **Property 9: Phase Merge Preservation**
    - **Validates: Requirements 4.14**
  
  - [ ]* 11.7 Write unit tests for phase manipulation
    - Test merge
    - Test split
    - Test reorder
    - _Requirements: 4.14_

- [x] 12. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.


- [x] 13. Phase Reviewer
  - [x] 13.1 Implement phase review logic
    - Analyze each phase for clarity, completeness, dependencies, scope, risks, feasibility, testing, integration
    - Categorize issues as critical, warning, or info
    - Check for cross-phase issues
    - Generate review report
    - _Requirements: 5.1, 5.2, 5.3, 5.4_
  
  - [x] 13.2 Implement improvement suggestions
    - Generate specific, actionable suggestions
    - Allow selective application of improvements
    - Update phase files with improvements
    - _Requirements: 5.5, 5.6, 5.7, 5.8, 5.9_
  
  - [ ]* 13.3 Write property test for phase review categorization
    - **Property 10: Phase Review Categorization**
    - **Validates: Requirements 5.1, 5.2, 5.4, 5.9**
  
  - [ ]* 13.4 Write unit tests for phase reviewer
    - Test issue detection
    - Test improvement generation
    - Test improvement application
    - _Requirements: 5.6, 5.7, 5.8_

- [x] 14. CLI Implementation (Cobra)
  - [x] 14.1 Set up Cobra CLI framework
    - Initialize Cobra application
    - Define global flags
    - Set up command structure
    - _Requirements: 15.1, 15.5_
  
  - [x] 14.2 Implement init command
    - Create configuration directory
    - Prompt for API keys
    - Initialize database
    - Initialize Git repository if needed
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_
  
  - [x] 14.3 Implement interview command
    - Start or resume interview
    - Handle pause and resume
    - _Requirements: 2.1-2.14_
  
  - [x] 14.4 Implement design command
    - Generate architecture
    - Handle reiteration
    - _Requirements: 3.1-3.16_
  
  - [x] 14.5 Implement plan command
    - Generate DevPlan
    - Handle reiteration
    - _Requirements: 4.1-4.15_
  
  - [x] 14.6 Implement review command
    - Run phase review
    - Display results
    - Apply improvements
    - _Requirements: 5.1-5.10_
  
  - [x] 14.7 Implement develop command
    - Execute phases
    - Handle detours and blockers
    - _Requirements: 9.1-9.8, 11.1-11.8, 12.1-12.8_
  
  - [x] 14.8 Implement status command
    - Display current stage and phase
    - Show progress
    - Show blockers
    - _Requirements: 21.1, 21.2, 21.4, 21.5, 21.6, 21.7, 21.8_
  
  - [x] 14.9 Implement stats command
    - Display token usage statistics
    - Display cost statistics
    - Show breakdowns by provider and phase
    - _Requirements: 8.11, 8.12, 8.13, 8.14_
  
  - [x] 14.10 Implement quota command
    - Check rate limits for all providers
    - Check quotas for all providers
    - Display warnings if approaching limits
    - _Requirements: 8a.3, 8a.4, 8a.5, 8a.6, 8a.7_
  
  - [x] 14.11 Implement checkpoint command
    - Create checkpoint
    - List checkpoints
    - _Requirements: 13.1, 13.2, 13.4_
  
  - [x] 14.12 Implement rollback command
    - Rollback to checkpoint
    - _Requirements: 13.5, 13.6_
  
  - [ ]* 14.13 Write property test for CLI argument validation
    - **Property 17: CLI Argument Validation**
    - **Validates: Requirements 15.3, 15.4, 15.7**
  
  - [ ]* 14.14 Write unit tests for each command
    - Test argument parsing
    - Test error handling
    - Test help text
    - _Requirements: 15.3, 15.4, 15.6, 15.7_

- [x] 15. Terminal UI (Bubbletea)
  - [x] 15.1 Implement Interview Model
    - Display questions with progress
    - Handle user input
    - Allow navigation
    - _Requirements: 16.1, 16.2, 16.8_
  
  - [x] 15.2 Implement Execution Model
    - Display real-time streaming output
    - Show task progress
    - Handle pause/resume/skip
    - _Requirements: 16.3, 16.4_
  
  - [x] 15.3 Implement Review Model
    - Display review results
    - Allow selection of improvements
    - _Requirements: 16.8_
  
  - [x] 15.4 Implement Status Model
    - Display project status dashboard
    - Show progress bars
    - _Requirements: 16.5_
  
  - [x] 15.5 Implement keyboard shortcuts and navigation
    - Define keyboard shortcuts
    - Handle scrolling and selection
    - Handle terminal resize
    - _Requirements: 16.4, 16.6, 16.7_
  
  - [ ]* 15.6 Write integration tests for Terminal UI
    - Test interview flow
    - Test execution display
    - Test review display
    - _Requirements: 16.1, 16.2, 16.3_

- [x] 16. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.


- [x] 17. Task Executor and Live Monitor
  - [x] 17.1 Implement Task Executor
    - Execute phases and tasks
    - Stream output in real-time
    - Handle pause, resume, skip
    - Mark tasks as blocked
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7, 9.8_
  
  - [x] 17.2 Implement Live Monitor
    - Display real-time output
    - Show task status
    - Handle errors
    - _Requirements: 9.2, 9.3, 9.4, 9.5_
  
  - [ ]* 17.3 Write unit tests for Task Executor
    - Test task execution
    - Test pause/resume
    - Test blocker detection
    - _Requirements: 9.6, 9.7, 9.8_

- [x] 18. Detour Support
  - [x] 18.1 Implement detour workflow
    - Pause execution
    - Gather detour information
    - Update DevPlan with new tasks
    - Resume execution
    - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5_
  
  - [x] 18.2 Implement detour tracking
    - Track detours in separate directory
    - Maintain task dependencies
    - Handle conflicts
    - _Requirements: 11.6, 11.7, 11.8_
  
  - [ ]* 18.3 Write property test for detour task integration
    - **Property 13: Detour Task Integration**
    - **Validates: Requirements 11.1, 11.3, 11.6, 11.8**
  
  - [ ]* 18.4 Write unit tests for detour support
    - Test detour creation
    - Test task insertion
    - Test dependency preservation
    - _Requirements: 11.6, 11.7_

- [x] 19. Blocker Detection and Resolution
  - [x] 19.1 Implement blocker detection
    - Track task failures
    - Mark as blocked after N failures
    - Notify user with context
    - _Requirements: 12.1, 12.2, 12.7, 12.8_
  
  - [x] 19.2 Implement blocker resolution
    - Gather information about blocker
    - Attempt resolution strategies
    - Request user intervention if needed
    - Resume from blocked task
    - _Requirements: 12.3, 12.4, 12.5, 12.6_
  
  - [ ]* 19.3 Write property test for blocker detection threshold
    - **Property 14: Blocker Detection Threshold**
    - **Validates: Requirements 12.1, 12.2, 12.7, 12.8**
  
  - [ ]* 19.4 Write unit tests for blocker resolution
    - Test resolution strategies
    - Test user intervention
    - Test resume after resolution
    - _Requirements: 12.5, 12.6_

- [x] 20. Checkpoint and Rollback System
  - [x] 20.1 Implement checkpoint creation
    - Save current state
    - Create Git tag
    - Store metadata
    - _Requirements: 13.1, 13.2, 13.3_
  
  - [x] 20.2 Implement checkpoint listing
    - Display all checkpoints with metadata
    - _Requirements: 13.4_
  
  - [x] 20.3 Implement rollback
    - Restore state from checkpoint
    - Reset Git repository to tag
    - Preserve checkpoint history
    - _Requirements: 13.5, 13.6, 13.7_
  
  - [ ]* 20.4 Write property test for checkpoint rollback round-trip
    - **Property 15: Checkpoint Rollback Round-Trip**
    - **Validates: Requirements 13.1, 13.2, 13.3, 13.5, 13.6**
  
  - [ ]* 20.5 Write unit tests for checkpoint system
    - Test checkpoint creation
    - Test listing
    - Test rollback
    - Test history preservation
    - _Requirements: 13.7_

- [x] 21. DevPlan Evolution and Tracking
  - [x] 21.1 Implement task completion tracking
    - Update task status
    - Update DevPlan files
    - Commit changes to Git
    - _Requirements: 19.1, 19.3_

  - [x] 21.2 Implement changelog maintenance
    - Record all modifications
    - Include timestamps
    - Track decisions
    - _Requirements: 19.4, 19.7, 19.8_

  - [x] 21.3 Implement DevPlan visualization
    - Highlight completed, in-progress, pending tasks
    - _Requirements: 19.6_
  
  - [ ]* 21.4 Write property test for DevPlan evolution tracking
    - **Property 18: DevPlan Evolution Tracking**
    - **Validates: Requirements 19.1, 19.3, 19.4, 19.7**
  
  - [ ]* 21.5 Write unit tests for changelog
    - Test modification recording
    - Test timestamp tracking
    - _Requirements: 19.7_

- [x] 22. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.


- [x] 23. Progress Tracking and Status
  - [x] 23.1 Implement progress calculation
    - Calculate completion percentage
    - Track time elapsed and estimated remaining
    - _Requirements: 21.4, 21.5_

  - [x] 23.2 Implement status display
    - Show current stage and phase
    - Show completed, in-progress, pending tasks
    - Show token usage and costs
    - Show active blockers
    - _Requirements: 21.1, 21.2, 21.3, 21.6, 21.8_

  - [x] 23.3 Implement status filtering
    - Filter by phase
    - Filter by component
    - _Requirements: 21.7_
  
  - [ ]* 23.4 Write property test for progress calculation accuracy
    - **Property 20: Progress Calculation Accuracy**
    - **Validates: Requirements 21.5**
  
  - [ ]* 23.5 Write unit tests for status display
    - Test status calculation
    - Test filtering
    - _Requirements: 21.7_

- [x] 24. Resume Capability
  - [x] 24.1 Implement resume detection
    - Detect incomplete work on startup
    - Offer to resume from last checkpoint
    - _Requirements: 22.1, 22.2_
  
  - [x] 24.2 Implement resume workflow
    - Restore all state including model selections
    - Display summary of completed work
    - Continue from next pending task
    - _Requirements: 22.3, 22.4, 22.7_
  
  - [x] 24.3 Implement resume from any checkpoint
    - Allow selection of checkpoint
    - Resume from selected checkpoint
    - _Requirements: 22.5_
  
  - [x] 24.4 Implement interview resume
    - Show previous answers
    - Continue from last question
    - _Requirements: 22.6_
  
  - [x] 24.5 Implement stage-specific resume
    - Resume from any pipeline stage
    - Allow restarting a stage
    - _Requirements: 22.8_
  
  - [ ]* 24.6 Write property test for state preservation round-trip (already covered in task 2.3)
    - **Property 4: State Preservation Round-Trip**
    - **Validates: Requirements 22.1, 22.2, 22.3, 22.4**
  
  - [ ]* 24.7 Write unit tests for resume capability
    - Test resume detection
    - Test resume from checkpoint
    - Test stage-specific resume
    - _Requirements: 22.5, 22.8_

- [x] 25. Pipeline Stage Navigation
  - [x] 25.1 Implement stage navigation
    - Allow going back to previous stage
    - Preserve current work
    - Regenerate dependent artifacts
    - _Requirements: 23.1, 23.2, 23.3_
  
  - [x] 25.2 Implement pipeline history tracking
    - Track complete pipeline history
    - Track all iterations
    - Commit with stage markers
    - _Requirements: 23.4, 23.5_
  
  - [x] 25.3 Implement stage prerequisite checking
    - Prevent skipping stages
    - Validate prerequisites
    - _Requirements: 23.6_
  
  - [ ]* 25.4 Write property test for stage navigation preservation
    - **Property 21: Stage Navigation Preservation**
    - **Validates: Requirements 23.1, 23.2, 23.3**
  
  - [ ]* 25.5 Write unit tests for stage navigation
    - Test going back
    - Test artifact regeneration
    - Test prerequisite checking
    - _Requirements: 23.6_

- [ ] 26. Rate Limiting and Quota Monitoring
  - [x] 26.1 Implement rate limit tracking
    - Extract rate limit info from API responses
    - Store rate limit data
    - Check rate limits before calls
    - Delay requests when necessary
    - _Requirements: 8a.1, 8a.2, 8a.8, 8a.10_
  
  - [x] 26.2 Implement quota tracking
    - Extract quota info from API responses
    - Store quota data
    - Check quotas before calls
    - _Requirements: 8a.2, 8a.8_
  
  - [x] 26.3 Implement quota display
    - Show rate limits for all providers
    - Show quotas for all providers
    - Warn when approaching limits
    - _Requirements: 8a.3, 8a.4, 8a.5, 8a.6, 8a.7_
  
  - [x] 26.4 Implement data refresh
    - Refresh stale rate limit data
    - Refresh stale quota data
    - _Requirements: 8a.9_
  
  - [ ]* 26.5 Write property test for rate limit tracking accuracy
    - **Property 23: Rate Limit Tracking Accuracy**
    - **Validates: Requirements 8a.1, 8a.2**
  
  - [ ]* 26.6 Write property test for quota monitoring
    - **Property 26: Quota Monitoring**
    - **Validates: Requirements 8a.2, 8a.6, 8a.7**
  
  - [ ]* 26.7 Write unit tests for rate limiting
    - Test rate limit extraction
    - Test request delaying
    - Test warnings
    - _Requirements: 8a.6, 8a.7, 8a.10_

- [x] 27. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.


- [x] 28. Error Handling and Recovery
  - [x] 28.1 Implement error categorization
    - Distinguish user errors, API errors, system errors, Git errors
    - Provide appropriate responses for each category
    - _Requirements: 20.6_
  
  - [x] 28.2 Implement state preservation on critical errors
    - Save state before exiting
    - Log error with full context
    - _Requirements: 20.3, 20.4_
  
  - [x] 28.3 Implement retry after error
    - Offer to retry failed operations
    - _Requirements: 20.5_
  
  - [x] 28.4 Implement offline-capable operations
    - Identify operations that can work offline
    - Provide offline fallbacks
    - _Requirements: 20.7_
  
  - [ ]* 28.5 Write unit tests for error handling
    - Test each error category
    - Test state preservation
    - Test retry logic
    - _Requirements: 20.3, 20.4, 20.5_

- [ ] 29. Configuration Initialization Idempotence
  - [ ]* 29.1 Write property test for initialization idempotence
    - **Property 1: Configuration Initialization Idempotence**
    - **Validates: Requirements 1.5**
  
  - [ ]* 29.2 Write unit tests for init command
    - Test first initialization
    - Test re-initialization prevention
    - _Requirements: 1.5_

- [ ] 30. OpenCode Model Discovery
  - [ ]* 30.1 Write property test for OpenCode model discovery
    - **Property 22: OpenCode Model Discovery**
    - **Validates: Requirements 6.4, 6.14**
  
  - [ ]* 30.2 Write unit tests for OpenCode provider
    - Test model discovery
    - Test API calls with discovered models
    - _Requirements: 6.14_

- [ ] 31. Integration Testing
  - [ ]* 31.1 Write end-to-end pipeline test
    - Test complete workflow: Init → Interview → Design → DevPlan → Review
    - _Requirements: All pipeline stages_
  
  - [ ]* 31.2 Write resume integration tests
    - Test resume from each stage
    - Test reiteration at each stage
    - _Requirements: 22.1-22.8, 23.1-23.6_
  
  - [ ]* 31.3 Write detour integration test
    - Test detour during execution
    - _Requirements: 11.1-11.8_
  
  - [ ]* 31.4 Write blocker integration test
    - Test blocker detection and resolution
    - _Requirements: 12.1-12.8_
  
  - [ ]* 31.5 Write checkpoint integration test
    - Test checkpoint creation and rollback
    - _Requirements: 13.1-13.7_
  
  - [ ]* 31.6 Write Git integration tests
    - Test commit creation for each artifact
    - Test conflict detection
    - Test rollback
    - _Requirements: 10.1-10.9_

- [x] 32. Cross-Platform Build and Distribution
  - [x] 32.1 Set up cross-platform build
    - Configure build for Linux (AMD64, ARM64)
    - Configure build for macOS (AMD64, ARM64)
    - Configure build for Windows (AMD64, ARM64)
    - Embed SQLite as static library
    - Include version information
    - _Requirements: 17.1, 17.2, 17.3, 17.4, 17.5, 17.7_
  
  - [x] 32.2 Set up GitHub releases
    - Configure automated releases
    - Upload binaries for all platforms
    - _Requirements: 17.6_
  
  - [ ]* 32.3 Test binary on each platform
    - Test on Linux
    - Test on macOS
    - Test on Windows
    - Verify no external dependencies required
    - _Requirements: 17.4_

- [x] 33. Documentation
  - [x] 33.1 Write README
    - Project overview
    - Installation instructions
    - Quick start guide
    - Command reference
    - _Requirements: All_
  
  - [x] 33.2 Write user guide
    - Complete workflow walkthrough
    - Advanced features
    - Troubleshooting
    - _Requirements: All_
  
  - [x] 33.3 Write developer guide
    - Architecture overview
    - Contributing guidelines
    - Testing guide
    - _Requirements: All_
  
  - [x] 33.4 Write API documentation
    - Document all public interfaces
    - Include examples
    - _Requirements: All components_

- [x] 34. Final Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 35. Release Preparation
  - [x] 35.1 Run full test suite
    - Unit tests
    - Property tests
    - Integration tests
    - _Requirements: All_
  
  - [ ] 35.2 Manual testing checklist
    - Test init on fresh directory
    - Test complete pipeline
    - Test pause/resume at each stage
    - Test reiteration at each stage
    - Test with each supported model provider
    - Test checkpoint creation and rollback
    - Test detour during execution
    - Test blocker detection
    - Test on Linux, macOS, and Windows
    - Test with invalid API keys
    - Test with network failures
    - Test with Git conflicts
    - _Requirements: All_
  
  - [ ] 35.3 Performance testing
    - Test with large projects
    - Test with many phases
    - Test with high token usage
    - _Requirements: All_
  
  - [x] 35.4 Security audit
    - Review API key storage
    - Review error messages for sensitive data
    - Review logging for PII
    - _Requirements: 18.1-18.7_
  
  - [x] 35.5 Create release notes
    - Document features
    - Document known issues
    - Document breaking changes
    - _Requirements: All_

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
- Integration tests validate end-to-end workflows
