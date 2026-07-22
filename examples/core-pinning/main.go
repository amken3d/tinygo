//go:build (rp2040 || rp2350) && scheduler.cores

package main

import (
	"machine"
	"runtime"
	"time"
)

func main() {
	// Example 1: Static core pinning (recommended pattern)
	// Pin main goroutine to core 0 for I/O and coordination
	machine.LockCore(0)
	println("Main goroutine running on core", machine.CurrentCore())

	// Example 2: Pin a worker goroutine to core 1 for time-critical work
	// This pattern is ideal for motion control, real-time processing, etc.
	done := make(chan bool)
	go timeCriticalWorker(done)

	// Example 3: Using runtime.LockOSThread for portable code
	// This works the same as machine.LockCore on RP2040/RP2350 with scheduler.cores,
	// but is portable to other platforms
	go portableWorker()

	// Main loop: demonstrate that we're actually on core 0
	for i := 0; i < 5; i++ {
		println("Main loop iteration", i, "on core", machine.CurrentCore())
		time.Sleep(500 * time.Millisecond)
	}

	<-done
	println("Done!")
}

// timeCriticalWorker demonstrates pinning a goroutine to core 1 for
// time-critical operations like step generation in motion control.
func timeCriticalWorker(done chan bool) {
	// Pin this goroutine to core 1
	machine.LockCore(1)
	println("Worker goroutine pinned to core", machine.CurrentCore())

	// Simulate time-critical work (e.g., step pulse generation)
	for i := 0; i < 10; i++ {
		// Time-critical operation here
		// In a real motion controller, this might be generating step pulses
		// with precise timing

		// Verify we're still on core 1
		if machine.CurrentCore() != 1 {
			panic("Worker migrated away from core 1!")
		}

		// Yield periodically to allow other goroutines on this core to run
		// This is a best practice to avoid monopolizing the core
		if i%3 == 0 {
			runtime.Gosched()
		}

		time.Sleep(200 * time.Millisecond)
	}

	println("Worker finished on core", machine.CurrentCore())
	done <- true
}

// portableWorker demonstrates using runtime.LockOSThread for code
// that needs to work across different TinyGo schedulers and platforms.
func portableWorker() {
	// This pins to the current core on RP2040/RP2350 with scheduler.cores,
	// and is a no-op on other schedulers
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initialCore := machine.CurrentCore()
	println("Portable worker started on core", initialCore)

	for i := 0; i < 3; i++ {
		println("Portable worker iteration", i, "on core", machine.CurrentCore())
		time.Sleep(300 * time.Millisecond)
	}
}
