//go:build nucleon657

package machine

import (
	"device/stm32"
	"runtime/interrupt"
)

// Pin mappings for the NUCLEO-N657X0-Q board. Matches ST's stm32n6xx_nucleo
// BSP (https://github.com/STMicroelectronics/stm32n6xx-nucleo-bsp).
const (
	LED_BLUE    = PG8
	LED_RED     = PG10
	LED_GREEN   = PG0
	LED_BUILTIN = LED_BLUE
	LED         = LED_BUILTIN
)

const (
	BUTTON      = BUTTON_USER
	BUTTON_USER = PC13
)

// UART pins. USART1 on PE5/PE6 (AF7) is wired to the ST-LINK Virtual COM Port.
const (
	UART_TX_PIN = PE5
	UART_RX_PIN = PE6
	UART_ALT_FN = 7 // GPIO_AF7_USART1
)

var (
	UART1  = &_UART1
	_UART1 = UART{
		Buffer:            NewRingBuffer(),
		Bus:               stm32.USART1,
		TxAltFuncSelector: UART_ALT_FN,
		RxAltFuncSelector: UART_ALT_FN,
	}
	DefaultUART = UART1
)

func init() {
	UART1.Interrupt = interrupt.New(stm32.IRQ_USART1, _UART1.handleInterrupt)
}
