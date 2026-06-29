# Performance Testing Plan - Geoffrussy AI Coding Agent

**Version**: 0.1.0  
**Date**: January 29, 2026  
**Purpose**: Validate performance characteristics under various load conditions

## Test Objectives

1. **Scalability**: Verify performance with large projects
2. **Responsiveness**: Ensure UI remains responsive under load
3. **Resource Usage**: Monitor CPU, memory, and disk I/O
4. **Database Performance**: Validate SQLite performance with large datasets
5. **Git Performance**: Measure overhead of Git operations

## Test Environment

### Baseline Configuration
- **CPU**: Intel/AMD x86_64 or Apple M1/M2
- **Memory**: 8GB RAM minimum (16GB recommended)
- **Disk**: SSD with > 500MB free
- **Network**: Stable internet connection (for API tests)
- **Go Version**: 1.21 or later
- **SQLite Version**: 3.x with WAL mode

### Test Data

#### Small Project
- Phases: 3
- Tasks per phase: 3
- Total tasks: 9
- Estimated tokens: ~10K
- Database size: ~1MB

#### Medium Project
- Phases: 7
- Tasks per phase: 5
- Total tasks: 35
- Estimated tokens: ~50K
- Database size: ~5MB

#### Large Project
- Phases: 10
- Tasks per phase: 10
- Total tasks: 100
- Estimated tokens: ~200K
- Database size: ~20MB

#### Very Large Project (Stress Test)
- Phases: 15
- Tasks per phase: 20
- Total tasks: 300
- Estimated tokens: ~1M
- Database size: ~100MB

## Test Scenarios

### 1. Startup Performance

#### Test 1.1: Cold Startup
```
time geoffrussy --version
time geoffrussy status
```
**Metrics**:
- [ ] Initialization time < 500ms
- [ ] Config loading time < 100ms
- [ ] Database connection time < 200ms
- [ ] Total startup time < 1s

**Benchmark**:
| Environment | Small Project | Medium Project | Large Project |
|-------------|---------------|-----------------|---------------|
| Local SSD  | < 200ms       | < 300ms        | < 500ms       |
| Network Disk  | < 500ms      | < 800ms        | < 1.5s       |

#### Test 1.2: Warm Startup (with cache)
```
# Run twice and measure second run
time geoffrussy status
```
**Metrics**:
- [ ] Faster than cold start
- [ ] Config cache effective
- [ ] Database connection pooling working

### 2. State Store Performance

#### Test 2.1: Project CRUD Operations

```go
// Benchmark: Create 1000 projects
func BenchmarkCreateProjects(b *testing.B) {
    for i := 0; i < b.N; i++ {
        store.CreateProject(&Project{
            ID:   fmt.Sprintf("project-%d", i),
            Name:  fmt.Sprintf("Test Project %d", i),
        })
    }
}
```

**Expected Performance**:
| Operation | Small DB (1MB) | Medium DB (5MB) | Large DB (20MB) | Very Large DB (100MB) |
|-----------|-----------------|-------------------|-------------------|----------------------|
| Create Project | < 5ms        | < 10ms            | < 20ms             | < 50ms              |
| Get Project   | < 1ms        | < 2ms             | < 5ms              | < 10ms             |
| Update Project| < 5ms        | < 10ms            | < 20ms             | < 50ms              |
| Delete Project| < 10ms       | < 20ms            | < 50ms             | < 100ms            |

#### Test 2.2: Phase CRUD Operations

```go
// Benchmark: Query phases with progress calculation
func BenchmarkListPhasesWithProgress(b *testing.B) {
    for i := 0; i < b.N; i++ {
        store.ListPhases(projectID)
        store.CalculateProgress(projectID)
    }
}
```

**Expected Performance**:
| Operation | Small Project (3 phases) | Medium Project (7 phases) | Large Project (10 phases) | Very Large (15 phases) |
|-----------|---------------------------|---------------------------|---------------------------|----------------------|
| Create Phase| < 5ms              | < 10ms                    | < 15ms                    | < 25ms               |
| List Phases  | < 2ms              | < 5ms                     | < 10ms                     | < 15ms                |
| Calculate Progress | < 10ms       | < 25ms                    | < 50ms                     | < 100ms               |
| Update Phase Status | < 5ms       | < 10ms                    | < 15ms                     | < 25ms               |

#### Test 2.3: Task CRUD Operations

```go
// Benchmark: Bulk task operations
func BenchmarkBulkTaskOperations(b *testing.B) {
    // Create 100 tasks
    for i := 0; i < 100; i++ {
        store.SaveTask(&Task{ID: fmt.Sprintf("task-%d", i)})
    }
    
    // Query all tasks
    tasks := store.ListTasks(phaseID)
    
    // Update all tasks
    for _, task := range tasks {
        store.UpdateTaskStatus(task.ID, TaskCompleted)
    }
}
```

**Expected Performance**:
| Operation | Per Task (Small) | Per Task (Medium) | Per Task (Large) | Batch (100 tasks) |
|-----------|-------------------|-------------------|-------------------|-------------------|
| Create Task | < 1ms           | < 2ms              | < 3ms              | < 50ms           |
| Get Task    | < 0.5ms         | < 1ms              | < 2ms              | < 100ms          |
| Update Status| < 1ms           | < 2ms              | < 3ms              | < 100ms          |
| List Tasks  | < 2ms           | < 5ms              | < 10ms             | < 200ms          |

#### Test 2.4: Checkpoint Operations

```go
// Benchmark: Checkpoint creation and rollback
func BenchmarkCheckpointRoundtrip(b *testing.B) {
    for i := 0; i < b.N; i++ {
        checkpointManager.CreateCheckpoint(name)
        checkpointManager.Rollback(checkpointID)
    }
}
```

**Expected Performance**:
| Operation | Small Project | Medium Project | Large Project |
|-----------|---------------|----------------|---------------|
| Create Checkpoint | < 100ms | < 200ms | < 500ms |
| List Checkpoints | < 50ms  | < 100ms | < 200ms |
| Rollback | < 500ms | < 1s   | < 2s    |
| Validate Checkpoint | < 50ms | < 100ms | < 200ms |

#### Test 2.5: Token Usage Tracking

```go
// Benchmark: Record 1000 token usage records
func BenchmarkTokenUsageTracking(b *testing.B) {
    for i := 0; i < 1000; i++ {
        store.RecordTokenUsage(&TokenUsage{
            Provider: "openai",
            Tokens: 100,
        })
    }
    
    // Calculate statistics
    store.GetTokenStats(projectID)
    store.GetTotalCost(projectID)
}
```

**Expected Performance**:
| Operation | Small (100 records) | Medium (1K records) | Large (10K records) |
|-----------|---------------------|---------------------|---------------------|
| Record Usage | < 1ms       | < 5ms              | < 20ms             |
| Get Token Stats | < 5ms  | < 20ms              | < 100ms            |
| Get Total Cost  | < 10ms | < 30ms              | < 150ms            |
| Get Stats by Provider | < 15ms | < 50ms | < 200ms |

### 3. Pipeline Stage Performance

#### Test 3.1: Interview Stage

**Test Flow**:
```
# Time interview with 50 questions
time geoffrussy interview
```

**Metrics**:
- [ ] First question appears < 500ms after start
- [ ] Answer processing < 200ms
- [ ] Follow-up generation < 3s (with API call)
- [ ] Phase completion < 2s (after last answer)
- [ ] Total interview time < 10 minutes (without API wait)

**Performance Targets**:
| Component | Target |
|-----------|---------|
| Startup to first question | < 500ms |
| Answer validation | < 100ms |
| State save | < 50ms |
| Next question load | < 200ms |
| LLM follow-up (including API) | < 5s |
| Phase transition | < 1s |

#### Test 3.2: Design Stage

**Test Flow**:
```
# Generate architecture for medium project
time geoffrussy design
```

**Metrics**:
- [ ] Architecture generation starts < 1s
- [ ] Streaming response latency < 100ms
- [ ] Complete generation < 30s (API dependent)
- [ ] Markdown export < 100ms
- [ ] JSON export < 100ms

**Performance Targets**:
| Component | Target |
|-----------|---------|
| Architecture generation (local) | < 5s |
| Architecture generation (API) | < 30s |
| Streaming latency | < 100ms |
| Markdown export | < 200ms |
| JSON export | < 200ms |

#### Test 3.3: DevPlan Stage

**Test Flow**:
```
# Generate DevPlan for medium project
time geoffrussy plan
```

**Metrics**:
- [ ] Phase generation starts < 1s
- [ ] All 7-10 phases generated < 60s
- [ ] Phase file creation < 2s total
- [ ] Master plan generation < 1s
- [ ] Git commit < 1s

**Performance Targets**:
| Component | Target |
|-----------|---------|
| Phase 1-3 generation | < 15s |
| Phase 4-7 generation | < 30s |
| Phase 8-10 generation | < 15s |
| All phase file creation | < 2s |
| Master plan generation | < 1s |

**Expected by Size**:
| Project Size | Total Generation Time | File Creation |
|-------------|---------------------|---------------|
| Small (3 phases) | < 10s | < 500ms |
| Medium (7 phases) | < 60s | < 2s |
| Large (10 phases) | < 90s | < 3s |

#### Test 3.4: Review Stage

**Test Flow**:
```
# Review all phases
time geoffrussy review
```

**Metrics**:
- [ ] Review starts < 1s
- [ ] Phase analysis < 2s per phase
- [ ] Total review time < 30s (7 phases)
- [ ] Improvement generation < 10s per phase
- [ ] Report export < 1s

**Performance Targets**:
| Component | Target |
|-----------|---------|
| Single phase review | < 3s |
| Cross-phase analysis | < 5s |
| Improvement generation | < 10s |
| Complete review (7 phases) | < 30s |

#### Test 3.5: Development Execution

**Test Flow**:
```
# Execute all phases
time geoffrussy develop
```

**Metrics**:
- [ ] Phase 1 start < 500ms
- [ ] Task execution streaming < 100ms latency
- [ ] Phase completion < 2s (excluding API time)
- [ ] All 7 phases < 10 minutes (excluding API)
- [ ] Git commit per task < 500ms

**Performance Targets**:
| Component | Target |
|-----------|---------|
| Phase startup | < 500ms |
| Task streaming latency | < 100ms |
| Phase completion | < 2s |
| Git commit | < 500ms |
| Checkpoint auto-save | < 200ms |

**Expected by Size**:
| Project Size | Total Execution (excl. API) | API Time (estimated) |
|-------------|-------------------------------|---------------------|
| Small (9 tasks) | < 1 min | 2-5 min |
| Medium (35 tasks) | < 5 min | 10-20 min |
| Large (100 tasks) | < 15 min | 30-60 min |

### 4. Resource Usage Monitoring

#### Test 4.1: Memory Usage

**Method**: Monitor with `go tool pprof` or system tools

```bash
# Monitor memory during development
geoffrussy develop &
PID=$!
while true; do
    ps -p $PID -o rss,vsz
    sleep 1
done
```

**Acceptable Ranges**:
| Operation | Idle | Small Project | Medium Project | Large Project |
|-----------|-------|---------------|----------------|---------------|
| Startup | < 50MB | - | - | - |
| Interview | < 100MB | < 150MB | < 200MB | < 300MB |
| Design | < 150MB | < 200MB | < 300MB | < 500MB |
| Develop | < 200MB | < 300MB | < 500MB | < 1GB |
| Status Command | < 50MB | < 100MB | < 150MB | < 200MB |

**Memory Leaks Check**:
```go
// Test: Run 1000 iterations and check memory growth
func TestMemoryNoLeaks(t *testing.T) {
    var m1, m2 runtime.MemStats
    runtime.ReadMemStats(&m1)
    
    for i := 0; i < 1000; i++ {
        store.GetProject("test-project")
    }
    
    runtime.ReadMemStats(&m2)
    memGrowth := m2.Alloc - m1.Alloc
    
    if memGrowth > 10*1024*1024 { // > 10MB growth
        t.Errorf("Potential memory leak: %d", memGrowth)
    }
}
```

#### Test 4.2: CPU Usage

**Expected CPU Utilization**:
| Operation | CPU % (Idle) | CPU % (Active) |
|-----------|---------------|-----------------|
| Status command | < 1% | < 5% |
| Interview | < 5% | < 10% |
| Design (waiting for API) | < 2% | < 5% |
| Design (processing) | < 20% | < 30% |
| Develop (idle) | < 2% | < 5% |
| Develop (streaming) | < 10% | < 20% |

**CPU Time Breakdown**:
| Component | Target % |
|-----------|-----------|
| Database operations | < 10% |
| Git operations | < 5% |
| UI rendering | < 15% |
| API communication | < 20% |
| Business logic | < 30% |

#### Test 4.3: Disk I/O

**Expected I/O Patterns**:
| Operation | Reads | Writes |
|-----------|-------|--------|
| Startup | 10-50KB | 0KB |
| Interview | 100-500KB | 1-5KB |
| Design | 10-50KB | 10-100KB |
| DevPlan | 50-200KB | 50-500KB |
| Develop | 100-1MB | 100-1MB |
| Checkpoint | 1-5MB | 5-10MB |

**Disk Space Requirements**:
| Project Size | Database | Git Repo | Total |
|-------------|----------|----------|-------|
| Small | ~1MB | ~5MB | ~6MB |
| Medium | ~5MB | ~50MB | ~55MB |
| Large | ~20MB | ~200MB | ~220MB |
| Very Large | ~100MB | ~1GB | ~1.1GB |

#### Test 4.4: Network Usage

**Expected Network Traffic**:
| Operation | Upload | Download | API Calls |
|-----------|---------|-----------|-----------|
| Interview (50 questions) | ~5KB | ~50KB | ~25-50 |
| Design | ~10KB | ~50KB | 1-2 |
| DevPlan | ~10KB | ~100KB | 3-5 |
| Review | ~5KB | ~50KB | 2-4 |
| Develop (per task) | ~10KB | ~50KB | 1-2 |

**Bandwidth Requirements**:
- Minimum: 1 Mbps (for API streaming)
- Recommended: 10 Mbps (for responsive streaming)
- Optimal: 100+ Mbps (for large projects)

### 5. Concurrent Operations

#### Test 5.1: Concurrent Database Access

```go
// Test: 100 concurrent readers
func TestConcurrentReads(t *testing.T) {
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            store.GetProject("test-project")
        }()
    }
    wg.Wait()
    // No deadlocks or panics
}
```

**Expected**:
- [ ] No deadlocks
- [ ] No race conditions
- [ ] < 1s average response time
- [ ] < 5s max response time

#### Test 5.2: Concurrent Checkpoints

```go
// Test: 10 concurrent checkpoints
func TestConcurrentCheckpoints(t *testing.T) {
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            checkpointManager.CreateCheckpoint(fmt.Sprintf("cp-%d", id))
        }(i)
    }
    wg.Wait()
    // All checkpoints created successfully
}
```

**Expected**:
- [ ] All checkpoints created
- [ ] No database corruption
- [ ] < 5s total time

### 6. Git Performance

#### Test 6.1: Commit Operations

```bash
# Measure commit time
time geoffrussy develop --task task-001
# Watch for commit
```

**Expected Performance**:
| Files Changed | Commit Time | Tag Time | Total |
|--------------|-------------|----------|-------|
| 1 file | < 100ms | < 50ms | < 150ms |
| 10 files | < 500ms | < 100ms | < 600ms |
| 50 files | < 2s | < 500ms | < 2.5s |

#### Test 6.2: Git History Size

**Expected Performance**:
| Project Size | Git Objects | Repository Size | History Query Time |
|-------------|--------------|-----------------|-------------------|
| Small (100 commits) | ~5,000 | ~20MB | < 100ms |
| Medium (500 commits) | ~25,000 | ~100MB | < 200ms |
| Large (1000 commits) | ~50,000 | ~200MB | < 500ms |

#### Test 6.3: Rollback Performance

```bash
# Measure rollback time
time geoffrussy rollback checkpoint-id
```

**Expected Performance**:
| Project Size | Rollback Time |
|-------------|--------------|
| Small | < 1s |
| Medium | < 3s |
| Large | < 10s |

### 7. Provider Performance

#### Test 7.1: API Call Latency

**Test**: Measure API response times
```
# Monitor with verbose logging
GEOFFRUSSY_VERBOSE=true geoffrussy interview
```

**Expected Latency**:
| Provider | Model | First Token (p50) | First Token (p95) | Total Time |
|----------|--------|-------------------|-------------------|------------|
| OpenAI | GPT-4 | < 500ms | < 2s | < 30s |
| Anthropic | Claude 3.5 | < 500ms | < 2s | < 25s |
| Ollama | Llama2 | < 100ms | < 500ms | < 20s |

#### Test 7.2: Rate Limit Handling

**Test**: Rapid sequential requests
```bash
for i in {1..20}; do
    geoffrussy status &
done
```

**Expected**:
- [ ] Rate limits detected
- [ ] Automatic retry with exponential backoff
- [ ] Max 5 retries per operation
- [ ] No indefinite hangs

**Backoff Timing**:
| Attempt | Delay (min) | Delay (max) |
|---------|-------------|-------------|
| 1 | 1s | 5s |
| 2 | 2s | 10s |
| 3 | 4s | 20s |
| 4 | 8s | 40s |
| 5 | 16s | 60s |

### 8. Stress Testing

#### Test 8.1: Large Dataset

**Test**: Very large project (1000 tasks)
```
# Generate large DevPlan
time geoffrussy plan
```

**Metrics**:
- [ ] Generation completes without crash
- [ ] Memory usage < 2GB
- [ ] Total time < 5 minutes
- [ ] UI remains responsive

#### Test 8.2: Long-Running Execution

**Test**: Run for extended period
```
# Start development
geoffrussy develop

# Let run for 1 hour
# Monitor for memory leaks, CPU creep
```

**Metrics**:
- [ ] No memory leaks (stable after initial ramp-up)
- [ ] CPU usage stable (no creep)
- [ ] No database locks
- [ ] UI remains responsive

#### Test 8.3: High Token Usage

**Test**: Generate massive token usage (1M tokens)
```
# Simulate high usage
for i in {1..1000}; do
    geoffrussy design
done
```

**Metrics**:
- [ ] All token records saved
- [ ] Statistics calculation < 1s
- [ ] Cost tracking accurate
- [ ] No performance degradation over time

## Performance Benchmarks

### Baseline Performance

| Scenario | Duration | Memory | CPU | Notes |
|----------|---------|---------|-----|--------|
| Small project init | < 2s | < 50MB | < 5% | Clean start |
| Medium project interview | < 10m | < 200MB | < 10% | API dependent |
| Medium project design | < 30s | < 300MB | < 15% | API dependent |
| Medium project planning | < 1m | < 200MB | < 10% | Local heavy |
| Medium project review | < 30s | < 150MB | < 10% | Mixed |
| Medium project develop | < 15m | < 500MB | < 20% | API dependent |
| Large project develop | < 60m | < 1GB | < 25% | API dependent |

### Performance Limits

**Warning Thresholds** (investigate if exceeded):
| Metric | Warning | Critical |
|--------|---------|----------|
| Startup time | > 2s | > 5s |
| Database query | > 100ms | > 500ms |
| Git commit | > 1s | > 5s |
| Memory usage (small) | > 300MB | > 500MB |
| Memory usage (large) | > 1.5GB | > 2GB |
| CPU usage (idle) | > 10% | > 20% |
| CPU usage (active) | > 40% | > 60% |

## Optimization Opportunities

### Identified Areas for Improvement

1. **Database Indexing**
   - Add indexes on frequently queried columns
   - Target: `project_id`, `status`, `created_at`

2. **Caching Layer**
   - Cache frequently accessed data (progress stats, token usage)
   - Target: Reduce database queries by 50%

3. **Batch Operations**
   - Batch insertions where possible
   - Target: Reduce transaction overhead

4. **Connection Pooling**
   - Optimize SQLite connection pool size
   - Target: Better concurrency handling

5. **UI Rendering**
   - Optimize TUI rendering loop
   - Target: < 30ms frame time

6. **Git Operations**
   - Batch Git operations
   - Target: Reduce commit overhead by 50%

## Performance Testing Tools

### Built-in Tools

```bash
# CPU profiling
go test -cpuprofile=cpu.prof ./...

# Memory profiling
go test -memprofile=mem.prof ./...

# Benchmark tests
go test -bench=. -benchmem ./...

# pprof visualization
go tool pprof cpu.prof
go tool pprof mem.prof
```

### External Tools

- **Linux**: `perf`, `strace`, `valgrind`
- **macOS**: Instruments.app, `dtrace`
- **Windows**: Windows Performance Analyzer
- **Cross-platform**: `htop`, `iotop`

## Test Results Summary

### Pass/Fail Criteria

| Test Category | Benchmarks Met | Target % | Status |
|-------------|-----------------|------------|--------|
| Startup Performance | | 95% | |
| Database Performance | | 90% | |
| Pipeline Performance | | 90% | |
| Resource Usage | | 85% | |
| Concurrent Operations | | 90% | |
| Git Performance | | 85% | |
| Provider Performance | | 80% | |

### Performance Rating

- ⭐⭐⭐⭐⭐ (5/5): Excellent - All targets met
- ⭐⭐⭐⭐ (4/5): Good - Minor targets missed
- ⭐⭐⭐ (3/5): Acceptable - Some targets missed
- ⭐⭐ (2/5): Needs Improvement - Many targets missed
- ⭐ (1/5): Poor - Most targets missed

**Current Rating**: TBD (requires actual testing)

## Regression Testing

### Performance Regression Detection

```bash
# Establish baseline
./run-perf-tests.sh > baseline.txt

# After changes
./run-perf-tests.sh > current.txt

# Compare
./compare-perf.sh baseline.txt current.txt
```

**Acceptable Degradation**:
- < 5%: ✅ Negligible
- 5-10%: ⚠️ Monitor closely
- 10-20%: ❌ Investigate
- > 20%: ❌ Block release

## Sign-off

**Performance Tester**: _________________  
**Date**: _________________  
**Environment**: _________________  

**Results**:
- [ ] All performance targets met
- [ ] No critical performance issues
- [ ] Acceptable for release
- [ ] Known performance limitations documented

**Performance Rating**: _______ / 5 stars

**Blocking Performance Issues**: _________________

**Release Recommendation**:
- [ ] ✅ APPROVED - Performance is acceptable
- [ ] ⚠️ CONDITIONAL - Minor issues, acceptable for v0.1.0
- [ ] ❌ NOT APPROVED - Performance issues must be addressed

---

**This performance testing plan should be executed and documented before v0.1.0 public release.**
