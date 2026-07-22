# Core Pinning Usage Guide

This document describes how to use CPU core pinning on RP2040 and RP2350 dual-core microcontrollers with TinyGo's `scheduler.cores`.

## Overview

Core pinning allows you to assign specific goroutines to specific CPU cores, ensuring they execute only on the designated core. This is useful for:

- **Motion control**: Pin step generation to core 1 for deterministic timing
- **Real-time processing**: Dedicate one core to time-critical tasks
- **Resource partitioning**: Separate I/O (core 0) from computation (core 1)

## APIs

### Machine Package (Hardware-Specific)

```go
machine.LockCore(core int)  // Pin to specific core (0 or 1)
machine.UnlockCore()         // Unpin, allow any core
```

Use these when you need explicit control over which core a goroutine runs on.

### Runtime Package (Portable)

```go
runtime.LockOSThread()   // Pin to current core
runtime.UnlockOSThread() // Unpin
```

Use these for code that needs to work across different TinyGo targets and schedulers.

## Requirements

- **Target**: RP2040 or RP2350
- **Scheduler**: Build with `-scheduler=cores`
- **Build command**: `tinygo build -target=pico -scheduler=cores ...`

## Usage Patterns

### Pattern 1: Static Pinning (Recommended)

Pin goroutines during initialization for predictable behavior:

```go
func main() {
    // Pin main to core 0 for I/O and coordination
    machine.LockCore(0)
    
    // Spawn worker pinned to core 1
    go func() {
        machine.LockCore(1)
        timeCriticalWork()
    }()
    
    // Main continues on core 0...
}
```

### Pattern 2: Motion Control

Real-world example for CNC/3D printer step generation:

```go
func main() {
    machine.LockCore(0) // Comms on core 0
    
    go func() {
        machine.LockCore(1) // Steps on core 1
        for {
            generateStepPulses()
            runtime.Gosched() // Yield periodically
        }
    }()
    
    // Handle G-code, USB, etc. on core 0
    handleCommunications()
}
```

### Pattern 3: Dynamic Pinning (Advanced)

Pin temporarily for specific operations:

```go
go func() {
    // Do unpinned work
    prepareData()
    
    // Pin for time-critical section
    machine.LockCore(1)
    processCriticalData()
    machine.UnlockCore()
    
    // Continue unpinned
    cleanupData()
}()
```

### Pattern 4: Portable Code

Use `runtime.LockOSThread` for code that should work on multiple targets:

```go
func worker() {
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()
    
    // This goroutine won't migrate on RP2040/RP2350
    // and is a no-op on other targets
    doWork()
}
```

## Best Practices

### 1. Pin Early

Pin goroutines as early as possible, ideally during initialization:

```go
// Good
func main() {
    machine.LockCore(0)
    go worker1() // spawned from pinned goroutine
}

// Less ideal
func main() {
    go worker1() // starts unpinned
}
func worker1() {
    machine.LockCore(1) // may block if core 1 busy
}
```

### 2. Yield Periodically

Pinned goroutines should yield to avoid monopolizing their core:

```go
func stepGenerator() {
    machine.LockCore(1)
    for {
        generateSteps()
        
        // Yield every N iterations
        if iteration % 100 == 0 {
            runtime.Gosched()
        }
    }
}
```

### 3. Avoid Indefinite Blocking

Don't let one goroutine occupy a core forever:

```go
// Bad: monopolizes core 1
func bad() {
    machine.LockCore(1)
    for {
        // tight loop, never yields
    }
}

// Good: yields at scheduling points
func good() {
    machine.LockCore(1)
    for {
        select {
        case work := <-workChan:
            process(work)
        case <-time.After(10 * time.Millisecond):
            // periodic timeout yields
        }
    }
}
```

## How It Works

### Migration Guarantee

`LockCore()` guarantees the goroutine is on the target core before returning:

```go
machine.LockCore(1)
// At this point, we are definitely running on core 1
currentCore := machine.CurrentCore() // returns 1
```

Internally, `LockCore` yields repeatedly until the migration completes:

```go
for int(currentCPU()) != targetCore {
    runtime.Gosched()
}
```

### Scheduling Behavior

The scheduler maintains three run queues:

1. **Core 0 queue**: Only core 0 checks this (for tasks pinned to core 0)
2. **Core 1 queue**: Only core 1 checks this (for tasks pinned to core 1)
3. **Shared queue**: Both cores check this (for unpinned tasks)

Each core checks its dedicated queue first, then the shared queue:

```
Core 0:  [Core 0 queue] -> [Shared queue]
Core 1:  [Core 1 queue] -> [Shared queue]
```

### Affinity Field

Each goroutine has an `Affinity` field in its task structure:
- `-1`: Unpinned (can run on any core)
- `0`: Pinned to core 0
- `1`: Pinned to core 1

When a task is scheduled, it's pushed to the appropriate queue based on affinity.

## Common Pitfalls

### 1. Building Without scheduler.cores

```bash
# Wrong: uses default scheduler
tinygo build -target=pico main.go

# Right: uses cores scheduler
tinygo build -target=pico -scheduler=cores main.go
```

Without `-scheduler=cores`, `machine.LockCore()` will panic at runtime.

### 2. Assuming Immediate Migration

```go
// Wrong assumption
go func() {
    machine.LockCore(1)
    // might briefly run on core 0 first!
}()
```

The spawned goroutine might start on core 0, then migrate to core 1 when `LockCore` is called. This is fine because `LockCore` guarantees the core before returning.

### 3. Racing with CurrentCore()

```go
// Wrong: racy
core := machine.CurrentCore()
machine.LockCore(core) // might have migrated already!

// Right: LockOSThread pins to current core
runtime.LockOSThread()
```

Use `runtime.LockOSThread()` if you want to pin to "whichever core I'm currently on."

## Comparison with Interrupts

Some argue that interrupt handlers are better than core pinning for time-critical work. Here's when each is appropriate:

### Use Core Pinning When:
- Work runs continuously or frequently (e.g., step generation)
- You need low latency (microseconds matter)
- Work involves tight timing loops
- You want simpler concurrency (no ISR context restrictions)

### Use Interrupts When:
- Work is truly sporadic (e.g., button press)
- You need to preempt other work immediately
- The ISR can finish quickly (<100 instructions)
- You're okay with ISR context limitations

For motion control step generation, core pinning is typically superior because:
1. No interrupt latency/jitter
2. No ISR context restrictions
3. Can run full goroutine code (channels, allocations, etc.)
4. Simpler to reason about than interrupt priorities

## Examples

See `examples/core-pinning/main.go` for a complete working example demonstrating all patterns.

## Availability on Other Targets

Currently, core pinning is only available on RP2040 and RP2350 with `scheduler.cores`.

For other targets:
- `runtime.LockOSThread/UnlockOSThread`: No-op (portable code works but does nothing)
- `machine.LockCore/UnlockCore`: Panic if called

Future targets that could support core pinning:
- ESP32 (dual-core variants)
- STM32H7 (dual-core Cortex-M7 + M4)
- i.MX RT (multi-core variants)
