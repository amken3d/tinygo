//go:build rp2040 || rp2350

package machine

import (
	"device/rp"
	"runtime/interrupt"
	"runtime/volatile"
	"unsafe"
)

const deviceName = rp.Device

const (
	// Number of spin locks available
	// Note: On RP2350, most spinlocks are unusable due to Errata 2
	_NUMSPINLOCKS         = 32
	_PICO_SPINLOCK_ID_IRQ = 9
	// is48Pin notes whether the chip is RP2040 with 32 pins or RP2350 with 48 pins.
	is48Pin = _NUMBANK0_GPIOS == 48
)

// UART on the RP2040
var (
	UART0  = &_UART0
	_UART0 = UART{
		Buffer: NewRingBuffer(),
		Bus:    rp.UART0,
	}

	UART1  = &_UART1
	_UART1 = UART{
		Buffer: NewRingBuffer(),
		Bus:    rp.UART1,
	}
)

func init() {
	UART0.Interrupt = interrupt.New(rp.IRQ_UART0_IRQ, _UART0.handleInterrupt)
	UART1.Interrupt = interrupt.New(rp.IRQ_UART1_IRQ, _UART1.handleInterrupt)
}

//go:linkname machineInit runtime.machineInit
func machineInit() {
	// Reset all peripherals to put system into a known state,
	// except for QSPI pads and the XIP IO bank, as this is fatal if running from flash
	// and the PLLs, as this is fatal if clock muxing has not been reset on this boot
	// and USB, syscfg, as this disturbs USB-to-SWD on core 1
	bits := ^uint32(initDontReset)
	resetBlock(bits)

	// Remove reset from peripherals which are clocked only by clkSys and
	// clkRef. Other peripherals stay in reset until we've configured clocks.
	bits = ^uint32(initUnreset)
	unresetBlockWait(bits)

	clocks.init()

	// Peripheral clocks should now all be running
	unresetBlockWait(RESETS_RESET_Msk)

	// DBGPAUSE pauses the timer when a debugger is connected. This prevents
	// sleep functions from ever returning, so disable it.
	timer.setDbgPause(false)
}

//go:linkname ticks runtime.machineTicks
func ticks() uint64 {
	return timer.timeElapsed()
}

//go:linkname lightSleep runtime.machineLightSleep
func lightSleep(ticks uint64) {
	timer.lightSleep(ticks)
}

// CurrentCore returns the core number the call was made from.
func CurrentCore() int {
	return int(rp.SIO.CPUID.Get())
}

// NumCores returns number of cores available on the device.
func NumCores() int { return 2 }

// ChipVersion returns the version of the chip. 1 is returned for B0 and B1
// chip.
func ChipVersion() uint8 {
	const (
		SYSINFO_BASE                  = 0x40000000
		SYSINFO_CHIP_ID_OFFSET        = 0x00000000
		SYSINFO_CHIP_ID_REVISION_BITS = 0xf0000000
		SYSINFO_CHIP_ID_REVISION_LSB  = 28
	)

	// First register of sysinfo is chip id
	chipID := *(*uint32)(unsafe.Pointer(uintptr(SYSINFO_BASE + SYSINFO_CHIP_ID_OFFSET)))
	// Version 1 == B0/B1
	version := (chipID & SYSINFO_CHIP_ID_REVISION_BITS) >> SYSINFO_CHIP_ID_REVISION_LSB
	return uint8(version)
}

// Single DMA channel. See rp.DMA_Type.
type dmaChannel struct {
	READ_ADDR   volatile.Register32
	WRITE_ADDR  volatile.Register32
	TRANS_COUNT volatile.Register32
	CTRL_TRIG   volatile.Register32
	_           [12]volatile.Register32 // aliases
}

// Static assignment of DMA channels to peripherals.
// Allocating them statically is good enough for now. If lots of peripherals use
// DMA, these might need to be assigned at runtime.
const (
	spi0DMAChannel = iota
	spi1DMAChannel
)

// DMA channels usable on the RP2040.
var dmaChannels = (*[12 + 4*rp2350ExtraReg]dmaChannel)(unsafe.Pointer(rp.DMA))

//go:inline
func boolToBit(a bool) uint32 {
	if a {
		return 1
	}
	return 0
}

//go:inline
func u32max(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

//go:inline
func isReservedI2CAddr(addr uint8) bool {
	return (addr&0x78) == 0 || (addr&0x78) == 0x78
}

// LockCore pins the calling goroutine to the specified CPU core (0 or 1).
//
// This function guarantees that when it returns, the goroutine is actually
// executing on the requested core. It yields internally until the migration
// completes, which means it may block if the target core is busy.
//
// # Blocking Behavior
//
// LockCore will loop and yield until the goroutine successfully migrates to
// the target core. If the target core is occupied by long-running work, this
// may take significant time. For best results:
//
//   - Pin goroutines early, ideally during initialization
//   - Ensure pinned goroutines yield periodically (e.g., via channel operations,
//     time.Sleep, or runtime.Gosched)
//   - Avoid having one goroutine monopolize a core indefinitely
//
// # Usage Patterns
//
// Static pinning (recommended):
//
//	func main() {
//	    machine.LockCore(0)  // Main runs on core 0
//
//	    go func() {
//	        machine.LockCore(1)  // Worker runs on core 1
//	        stepGenerationLoop()
//	    }()
//	}
//
// Dynamic pinning (advanced):
//
//	go func() {
//	    machine.LockCore(1)  // Pin to core 1 for time-critical work
//	    doTimeCriticalWork()
//	    machine.UnlockCore() // Release core
//	}()
//
// # Use Cases
//
//   - Motion control: Pin step generation to core 1 for deterministic timing
//   - Real-time processing: Dedicate one core to time-critical tasks
//   - Resource partitioning: Separate concerns across cores (e.g., I/O on core 0,
//     computation on core 1)
//
// # Availability
//
// This function is only available when building with -scheduler=cores for
// RP2040 or RP2350 targets. On other schedulers or targets, calling LockCore
// will panic.
//
// For portable code that works across schedulers, use runtime.LockOSThread
// instead, which pins to the current core on RP2040/RP2350 with scheduler.cores,
// and is a no-op elsewhere.
func LockCore(core int)

// UnlockCore unpins the calling goroutine, allowing it to be scheduled on any
// available CPU core.
//
// If the goroutine was not previously pinned via LockCore or runtime.LockOSThread,
// this is a no-op.
//
// After calling UnlockCore, the goroutine may migrate to a different core at
// the next scheduling point (channel operation, time.Sleep, runtime.Gosched, etc.).
//
// # Availability
//
// This function is only available when building with -scheduler=cores for
// RP2040 or RP2350 targets. On other schedulers or targets, calling UnlockCore
// will panic.
func UnlockCore()
