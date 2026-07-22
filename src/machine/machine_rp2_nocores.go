//go:build (rp2040 || rp2350) && !scheduler.cores

package machine

// LockCore is not available without scheduler.cores.
// Use -scheduler=cores when building for RP2040/RP2350 to enable core pinning.
func LockCore(core int) {
	panic("machine.LockCore: not available without scheduler.cores")
}

// UnlockCore is not available without scheduler.cores.
// Use -scheduler=cores when building for RP2040/RP2350 to enable core pinning.
func UnlockCore() {
	panic("machine.UnlockCore: not available without scheduler.cores")
}
