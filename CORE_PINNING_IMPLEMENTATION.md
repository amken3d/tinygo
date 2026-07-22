# Core Pinning Implementation Summary

This document describes the implementation of CPU core affinity (pinning) for RP2040/RP2350 dual-core microcontrollers with TinyGo's `scheduler.cores`.

## Implementation Overview

The implementation adds the ability to pin goroutines to specific CPU cores, ensuring deterministic execution locations for time-critical workloads like motion control.

## Key Features

1. **Guaranteed Migration**: `LockCore()` yields internally until the goroutine is actually on the target core
2. **Per-Core Queues**: Dedicated run queues for each core plus a shared queue for unpinned tasks
3. **Portable API**: `runtime.LockOSThread()` works across schedulers (pins on cores, no-op elsewhere)
4. **Hardware-Specific API**: `machine.LockCore(n)` for explicit core selection on RP2040/RP2350

## Architecture

### Task Structure

Added `Affinity int8` field to `internal/task.Task`:
- `-1`: Unpinned (default)
- `0`: Pinned to core 0
- `1`: Pinned to core 1

### Run Queues

Replaced single run queue with three queues in `scheduler_cores.go`:

```go
runqueueShared task.Queue         // For unpinned tasks
runqueueCore   [numCPU]task.Queue // Per-core queues [0] and [1]
```

### Scheduler Logic

Each core's scheduler loop:
1. Check own dedicated queue (for pinned tasks)
2. Check shared queue (for unpinned tasks)
3. If no work, sleep or wait

### Migration Guarantee

`machineLockCore()` implementation:

```go
func machineLockCore(core int) {
    // Set affinity
    schedulerLock.Lock()
    t := task.Current()
    if t != nil {
        t.Affinity = int8(core)
    }
    schedulerLock.Unlock()
    
    // Yield until actually on target core
    for int(currentCPU()) != core {
        Gosched()
    }
}
```

This ensures the caller is running on the target core before returning.

## Files Modified

### Core Runtime Files

1. **src/internal/task/task.go**
   - Added `Affinity int8` field to Task struct

2. **src/runtime/scheduler_cores.go**
   - Replaced `runqueue` with `runqueueShared` and `runqueueCore[numCPU]`
   - Modified `scheduleTask()` to push to appropriate queue based on affinity
   - Modified `Gosched()` to push to appropriate queue
   - Modified scheduler loop to check per-core queue first, then shared
   - Added affinity check in sleep wake-up (tasks waking on wrong core go to that core's queue)
   - Added `machineLockCore()` and `machineUnlockCore()` implementation
   - Added `lockOSThreadImpl()` and `unlockOSThreadImpl()`
   - Updated `schedulerRunQueue()` to return `&runqueueShared`

3. **src/runtime/runtime.go**
   - Implemented `LockOSThread()` and `UnlockOSThread()` with comprehensive documentation
   - Changed from stubs to functional implementations

4. **src/runtime/scheduler_cooperative.go**
   - Added `lockOSThreadImpl()` and `unlockOSThreadImpl()` stubs (no-op)

5. **src/runtime/scheduler_none.go**
   - Added `lockOSThreadImpl()` and `unlockOSThreadImpl()` stubs (no-op)

6. **src/runtime/scheduler_threads.go**
   - Added `lockOSThreadImpl()` and `unlockOSThreadImpl()` stubs (no-op)

### Machine Package Files

7. **src/machine/machine_rp2.go**
   - Added `LockCore(core int)` declaration with comprehensive godoc
   - Added `UnlockCore()` declaration with comprehensive godoc
   - Documentation covers:
     - Blocking behavior and best practices
     - Usage patterns (static, dynamic)
     - Use cases (motion control, real-time, partitioning)
     - Availability and portability notes

8. **src/machine/machine_rp2_cores.go** (NEW)
   - Implementation for builds with `scheduler.cores`
   - Validates core parameter (0-1 range check)
   - Links to runtime via `//go:linkname`

9. **src/machine/machine_rp2_nocores.go** (NEW)
   - Panic stubs for builds without `scheduler.cores`
   - Clear error messages guiding users to use `-scheduler=cores`

### Examples and Documentation

10. **examples/core-pinning/main.go** (NEW)
    - Comprehensive example demonstrating all usage patterns
    - Shows static pinning, dynamic pinning, and portable code
    - Includes best practices (periodic yielding, verification)

11. **CORE_PINNING_USAGE.md** (NEW)
    - Complete usage guide
    - API reference
    - Best practices and common pitfalls
    - Comparison with interrupt-based approaches
    - How it works (internals)

12. **CORE_PINNING_IMPLEMENTATION.md** (NEW)
    - This file
    - Implementation details for maintainers

## Build Tags

All implementation uses appropriate build tags:

- `machine_rp2_cores.go`: `//go:build (rp2040 || rp2350) && scheduler.cores`
- `machine_rp2_nocores.go`: `//go:build (rp2040 || rp2350) && !scheduler.cores`
- `scheduler_cores.go`: `//go:build scheduler.cores` (existing)

## Testing Recommendations

### Unit Tests

1. **Affinity setting**: Verify `LockCore(n)` sets `task.Affinity` to `n`
2. **Queue selection**: Verify pinned tasks go to correct queue
3. **Migration**: Verify goroutine migrates to target core

### Integration Tests

1. **Static pinning**: Spawn goroutine, pin to core 1, verify it stays on core 1
2. **Dynamic pinning**: Pin, unpin, verify migration behavior
3. **Sleep wake-up**: Sleep a pinned task, verify it wakes on correct core
4. **Interleaving**: Multiple pinned tasks on same core, verify they interleave

### Manual Testing on Hardware

Example test program:

```go
func main() {
    machine.LockCore(0)
    println("Main on core", machine.CurrentCore())
    
    done := make(chan bool)
    go func() {
        machine.LockCore(1)
        for i := 0; i < 100; i++ {
            if machine.CurrentCore() != 1 {
                panic("migrated!")
            }
            time.Sleep(10 * time.Millisecond)
        }
        done <- true
    }()
    <-done
}
```

## Performance Considerations

### Overhead

- **Per-task overhead**: +1 byte (Affinity field)
- **Scheduler overhead**: One extra queue check per scheduling decision
- **Migration overhead**: `LockCore()` loops until migration completes

### Optimization Opportunities

1. **Batch migrations**: Could collect pending migrations and do them together
2. **Affinity hints**: Could add hints for scheduler to avoid migration in first place
3. **Core-local allocations**: Could allocate from core-local heaps

Not implemented yet as current design is simple and sufficient for motion control use case.

## Comparison with Other Approaches

### vs. Interrupt Handlers

**Core Pinning Advantages:**
- No interrupt latency/jitter
- Full goroutine context (can use channels, allocations, etc.)
- Simpler concurrency model
- Better for continuous/frequent work

**Interrupt Advantages:**
- Can preempt immediately
- Better for truly sporadic events
- Lower idle overhead

### vs. Work Stealing

Work stealing (like Go's P/M model) is not suitable for hard real-time:
- Non-deterministic migration
- Load balancing conflicts with affinity
- More complex scheduler

Core pinning with fixed assignment is simpler and more predictable.

## Future Work

### Potential Enhancements

1. **More cores**: Support targets with >2 cores (ESP32, STM32H7)
2. **Affinity masks**: Allow tasks to run on subset of cores (e.g., "core 0 or 1 but not 2")
3. **Core-local allocation**: Reduce cross-core memory traffic
4. **Priority levels**: Combine with priority scheduling
5. **Metrics**: Track per-core utilization

### Other Targets

Targets that could benefit:
- **ESP32**: Dual-core Xtensa (could reuse this implementation)
- **STM32H7**: Dual-core Cortex-M7 + M4 (heterogeneous)
- **i.MX RT**: Some variants are multi-core

## References

- GitHub PR: https://github.com/tinygo-org/tinygo/pull/5092
- Motion control use case: Step generation on dedicated core
- Standard Go: `runtime.LockOSThread()` compatibility
