//go:build stm32n657

package machine

import (
	"device/stm32"
)

// ---------- UART ----------

func (uart *UART) configurePins(config UARTConfig) {
	config.TX.ConfigureAltFunc(PinConfig{Mode: PinModeUARTTX}, uart.TxAltFuncSelector)
	config.RX.ConfigureAltFunc(PinConfig{Mode: PinModeUARTRX}, uart.RxAltFuncSelector)
}

// getBaudRateDivisor derives the USART BRR value from the peripheral's
// kernel clock. After initCLK runs, every USART/UART peripheral on N6 is fed
// from its APB bus clock (PCLK1 for USART2/3/UART4-8, PCLK2 for USART1/6/10
// and UART9, PCLK4 for LPUART1) which in our clock plan is PCLK1=PCLK2=150
// MHz. OVER8=0 is the reset default, so BRR = fck / baud.
func (uart *UART) getBaudRateDivisor(baudRate uint32) uint32 {
	return APB1_TIM_FREQ / baudRate
}

func (uart *UART) setRegisters() {
	uart.rxReg = &uart.Bus.RDR
	uart.txReg = &uart.Bus.TDR
	uart.statusReg = &uart.Bus.ISR
	uart.txEmptyFlag = stm32.USART_ISR_TXE
	uart.errClearReg = &uart.Bus.ICR
}
