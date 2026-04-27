//go:build stm32n657

package machine

import (
	"device/stm32"
)

func (uart *UART) configurePins(config UARTConfig) {
	config.TX.ConfigureAltFunc(PinConfig{Mode: PinModeUARTTX}, uart.TxAltFuncSelector)
	config.RX.ConfigureAltFunc(PinConfig{Mode: PinModeUARTRX}, uart.RxAltFuncSelector)
}

// getBaudRateDivisor derives the USART BRR value from the peripheral's
// kernel clock. After initCLK runs, every USART/UART peripheral on N6 is fed
// from its APB bus clock (PCLK1 for USART2/3/UART4-8, PCLK2 for USART1/6/10
// and UART9, PCLK4 for LPUART1), which is 200 MHz in our clock plan.
// OVER8=0 is the reset default, so BRR = fck / baud.
// Ref: RM0486 pg 3298
//
// Uses PCLK1_FREQ (= actual PCLK1 on N6) rather than APB1_TIM_FREQ, which
// is calibrated to the (different, lower) TIM kernel clock rate — see the
// note above APB1_TIM_FREQ in machine_stm32n6.go.
func (uart *UART) getBaudRateDivisor(baudRate uint32) uint32 {
	return PCLK1_FREQ / baudRate
}

func (uart *UART) setRegisters() {
	uart.rxReg = &uart.Bus.RDR
	uart.txReg = &uart.Bus.TDR
	uart.statusReg = &uart.Bus.ISR
	uart.txEmptyFlag = stm32.USART_ISR_TXE
	uart.errClearReg = &uart.Bus.ICR
}

// getFreqRange returns the I2C TIMINGR register value for the requested
// SCL frequency. I2C kernel clock on N6 defaults to PCLK1 (200 MHz in our
// clock plan — see machine_stm32n6.go). Values below were calculated to
// hit ~100/400 kHz with PCLK1=200 MHz; tune via STM32CubeMX if you need
// different timing margins.
func (i2c *I2C) getFreqRange(br uint32) uint32 {
	switch br {
	case 100 * KHz:
		// PRESC=4 (div 5, t_PRESC=25ns), SCLDEL=4, SDADEL=0,
		// SCLH=0xC7 (high=200*25=5000ns), SCLL=0xC7 (low=5000ns)
		// → 10 µs period = 100 kHz.
		return 0x4040C7C7
	case 400 * KHz:
		// PRESC=1 (div 2, t_PRESC=10ns), SCLDEL=4, SDADEL=0,
		// SCLH=0x40 (high=65*10=650ns), SCLL=0xB8 (low=185*10=1850ns)
		// → 2.5 µs period = 400 kHz.
		return 0x104040B8
	default:
		return 0
	}
}
