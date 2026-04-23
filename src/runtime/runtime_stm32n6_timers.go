//go:build stm32n6

package runtime

import (
	"device/arm"
	"runtime/interrupt"
	"runtime/volatile"

	_ "unsafe"
)

// N6 doesn't use a peripheral TIM for the runtime clock — experiments showed
// that hardware-pended peripheral IRQs don't dispatch reliably on a
// NUCLEO-N657X0-Q in its default security / RIF configuration, even though
// software-pended IRQs (via STIR) do. SysTick is a CPU-integrated exception
// (number 15), not a peripheral IRQ, and routes independently of RIF or the
// NVIC's external-IRQ logic. It's what CubeMX's HAL_Init uses for its 1 ms
// tick baseline on N6, so we follow the same pattern.
//
// The exported symbols (ticks, sleepTicks, ticksToNanoseconds,
// nanosecondsToTicks) match the contract of runtime_stm32_timers.go; the
// shared file excludes stm32n6 via build tag and this one takes its place.

const (
	// A single timeUnit represents 16 ns. Preserved across STM32 targets so
	// that conversions with time.Duration stay consistent.
	NS_PER_TICK = 16

	// For very short sleeps a busy loop avoids races with scheduler wake.
	MAX_BUSY_LOOP_NS = 10e3 // 10 µs

	// SysTick fires every 1 ms.
	TICK_INTR_PERIOD_NS = 1e6
	TICK_PER_INTR       = TICK_INTR_PERIOD_NS / NS_PER_TICK
)

var (
	// Ticks accumulated from SysTick overflow events. Each increment covers
	// TICK_PER_INTR (= 1 ms / 16 ns = 62500) time units.
	tickCount volatile.Register64

	// Cached reload value so ticks() can convert the SysTick down-counter
	// to a time-units offset inside the current 1 ms period.
	systReload uint32
)

func ticksToNanoseconds(t timeUnit) int64 {
	return int64(t) * NS_PER_TICK
}

func nanosecondsToTicks(ns int64) timeUnit {
	return timeUnit(ns / NS_PER_TICK)
}

//go:linkname ticks runtime.ticks
func ticks() timeUnit {
	// SysTick is a 24-bit down-counter that reloads at each underflow, so
	// the elapsed-in-current-ms count is (reload - current). Snapshot both
	// tickCount and the counter with interrupts disabled so the SysTick
	// ISR can't fire between the two reads.
	mask := interrupt.Disable()
	overflows := uint64(tickCount.Get())
	cvr := arm.SYST.SYST_CVR.Get() & 0x00FFFFFF
	interrupt.Restore(mask)

	// Protect against an overflow that happened right after our reads: if
	// COUNTFLAG got latched (SysTick just wrapped), treat us as if the ISR
	// already ran for that wrap and add one to the overflow count. We do
	// NOT clear COUNTFLAG (reading CSR clears it); the real ISR will do so
	// on the next dispatch.
	elapsed := systReload - cvr
	return timeUnit(overflows*TICK_PER_INTR + uint64(elapsed)*TICK_PER_INTR/uint64(systReload+1))
}

//go:linkname sleepTicks runtime.sleepTicks
func sleepTicks(d timeUnit) {
	if ticksToNanoseconds(d) < MAX_BUSY_LOOP_NS {
		// Short sleep: busy-spin, avoids scheduler wake overhead.
		end := ticks() + d
		for ticks() < end {
		}
		return
	}

	end := ticks() + d
	for ticks() < end {
		// WFE wakes on any event including the SysTick exception entry/exit
		// (with SEVONPEND in SCR, pending exceptions set the Event register).
		arm.Asm("wfe")
	}
}

// handleSysTick is installed in the vector table (via //export) as the
// strong override of the weak SysTick_Handler alias in stm32n657.s. It just
// bumps the 1-ms tick counter; keeping it minimal also minimises its
// ISR latency so the cadence stays accurate.
//
//export SysTick_Handler
func handleSysTick() {
	tickCount.Set(tickCount.Get() + 1)
}

// initTickTimer is called from runtime_stm32n657.go's init() and replaces
// the TIM-based initTickTimer from the shared runtime_stm32_timers.go.
// Argument is unused — kept for signature compatibility with the init site.
func initTickTimer() {
	// Reload value for 1 ms at the current CPU frequency. CPUFrequency() is
	// defined in machine_stm32n6.go.
	cycles := machineCPUFrequency() / 1000
	systReload = cycles - 1
	// arm.SetupSystemTimer configures RVR + CVR + enables ENABLE|TICKINT|
	// CLKSOURCE(processor), and sets the SysTick exception to fire on each
	// reload.
	_ = arm.SetupSystemTimer(systReload)
}

// machineCPUFrequency returns the current CPU frequency in Hz. It forwards
// to machine.CPUFrequency but is declared here via //go:linkname so the
// runtime package does not take a cycle import on machine.
//
//go:linkname machineCPUFrequency machine.CPUFrequency
func machineCPUFrequency() uint32
