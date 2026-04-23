//go:build stm32 && !stm32n6

package machine

import "device/stm32"

const uartRXNotEmptyIE = stm32.USART_CR1_RXNEIE
