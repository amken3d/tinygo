//go:build stm32g431xx

package runtime

import (
	"machine"
)

func init() {
	initCLK()

	machine.InitSerial()

	// Use TIM3 instead of TIM7 - TIM7 is a basic timer without output compare
	// channels which are needed for fine-grained sleep functionality
	initTickTimer(&machine.TIM3)
}
