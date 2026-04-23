//go:build stm32n6

package runtime

import (
	"device/arm"
	"machine"
	"unsafe"
)

//go:extern __isr_vector
var __isr_vector [0]uint32

func putchar(c byte) {
	machine.Serial.WriteByte(c)
}

func getchar() byte {
	for machine.Serial.Buffered() == 0 {
		Gosched()
	}
	v, _ := machine.Serial.ReadByte()
	return v
}

func buffered() int {
	return machine.Serial.Buffered()
}

// initCLK is the N6's pre-main chip bring-up. Called from
// runtime_stm32n657.go's init() before InitSerial / initTickTimer.
//
// Two non-obvious items compared to other STM32 families:
//
//  1. VTOR must be re-pointed at our vector table. openocd's reset-halt +
//     manual-PC-resume flow lands the CPU at our Reset_Handler in Secure
//     state with VTOR_S still at whatever the ROM bootloader left it;
//     without this write the first exception after boot vectors into ROM.
//
//  2. SEVONPEND in SCB.SCR must be 1. The scheduler's idle path calls
//     WFE, which on Cortex-M55 only exits on an explicit SEV, a higher-
//     priority exception actually taken, or — with SEVONPEND set — any
//     newly-pending IRQ. Without this bit, the scheduler sleeps through
//     tick events and time.Sleep hangs.
//
// RCC clock-enable quirk: on N6 the plain *ENR registers are write-ignored
// and clocks must be gated via the *ENSR "Set" register aliases. That's
// handled in machine_stm32n6.go (enableClock, enableAltFuncClock, TIM
// definitions), not here.
//
// Clock tree: we intentionally leave the CPU on HSI at its reset-default
// divider (nominally 64 MHz) rather than bringing up PLL1. This keeps the
// bring-up footprint tiny; turn on the PLL in a later step if higher
// performance is needed, and update CPUFrequency() in machine_stm32n6.go
// to match — SysTick's reload in initTickTimer reads that back.
func initCLK() {
	arm.SCB.VTOR.Set(uint32(uintptr(unsafe.Pointer(&__isr_vector))))
	arm.SCB.SCR.SetBits(1 << 4) // SEVONPEND
}
