//go:build stm32g431xx

package runtime

import (
	"machine"

	"device/arm"
)

func init() {
	// Enable FPU: STM32G431 has a Cortex-M4F with single-precision FPU.
	// Set CP10 and CP11 to full access (bits [23:20] = 0xF).
	// Must be done before any float operation or FPU instructions will HardFault.
	arm.SCB.CPACR.SetBits(0xF << 20)
	initCLK()

	machine.InitSerial()

	// Use TIM3 instead of TIM7 - TIM7 is a basic timer without output compare
	// channels which are needed for fine-grained sleep functionality
	initTickTimer(&machine.TIM3)
}
