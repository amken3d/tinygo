//go:build (rp2040 || rp2350) && scheduler.cores

package machine

const numCPU = 2

// LockCore pins the calling goroutine to the specified CPU core.
// See machine_rp2.go for full documentation.
func LockCore(core int) {
	if core < 0 || core >= numCPU {
		panic("machine: core out of range")
	}
	machineLockCore(core)
}

// UnlockCore unpins the calling goroutine.
// See machine_rp2.go for full documentation.
func UnlockCore() {
	machineUnlockCore()
}

//go:linkname machineLockCore runtime.machineLockCore
func machineLockCore(core int)

//go:linkname machineUnlockCore runtime.machineUnlockCore
func machineUnlockCore()
