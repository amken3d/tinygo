//go:build stm32n657

package runtime

import (
	"machine"
)

func init() {
	initCLK()

	// Note: machine.InitExtendedAXISRAM() is called lazily from the
	// DCMIPP driver path now, after InitSerial — earlier calls were
	// hanging the board before any output was visible. Investigation
	// pending; the in-machine RISAF setup pattern from DCMIPP.Configure
	// is known good.

	machine.InitSerial()

	// N6's initTickTimer uses SysTick (see runtime_stm32n6_timers.go), not
	// a peripheral TIM — the shared runtime_stm32_timers.go is excluded
	// from stm32n6 via build tag. Hardware-pended peripheral IRQs don't
	// dispatch reliably to Secure handlers on this chip in its default
	// dev-boot / TZEN-off configuration, but SysTick (a CPU-integrated
	// exception, not an external IRQ) works without issue and matches
	// what ST's HAL_Init uses.
	initTickTimer()
}
