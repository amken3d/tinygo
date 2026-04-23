//go:build stm32n6

package machine

import "device/stm32"

// N6 USARTs surface the RX-not-empty interrupt enable as RXFNEIE (bit 5 of
// CR1) because their peripheral variant uses the FIFO-mode CR1 layout. The
// bit position and meaning match RXNEIE on the non-FIFO variant.
const uartRXNotEmptyIE = stm32.USART_CR1_RXFNEIE
