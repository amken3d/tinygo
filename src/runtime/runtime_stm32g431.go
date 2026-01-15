//go:build stm32g431xx

package runtime

import (
	"machine"
)

func init() {
	initCLK()

	machine.InitSerial()

	initTickTimer(&machine.TIM7)
}
