//go:build nucleog431kb

// Schematic: https://www.st.com/resource/en/schematic_pack/mb1430-g431kb-c01_schematic.pdf
// Datasheet: https://www.st.com/resource/en/datasheet/stm32g431kb.pdf
// Nucleo-32 User Manual: https://www.st.com/resource/en/user_manual/um2397-stm32g4-nucleo32-board-mb1430-stmicroelectronics.pdf

package machine

import (
	"device/stm32"
	"runtime/interrupt"
)

const (
	// Arduino Nano-compatible pins for Nucleo-32 form factor
	D0  = PA10 // USART1_RX
	D1  = PA9  // USART1_TX
	D2  = PA12
	D3  = PB0 // TIM3_CH3
	D4  = PB7
	D5  = PB6 // TIM4_CH1
	D6  = PB1 // TIM3_CH4
	D7  = PF0
	D8  = PF1
	D9  = PA8  // TIM1_CH1
	D10 = PA11 // SPI1_CS / TIM1_CH4
	D11 = PB5  // SPI1_MOSI / TIM3_CH2
	D12 = PB4  // SPI1_MISO
	D13 = PB3  // SPI1_SCK / LED_GREEN

	// Analog pins
	A0 = PA0 // ADC1_IN1
	A1 = PA1 // ADC1_IN2
	A2 = PA3 // ADC1_IN4
	A3 = PA4 // ADC2_IN17
	A4 = PA5 // ADC2_IN13
	A5 = PA6 // ADC2_IN3
	A6 = PA7 // ADC2_IN4
	A7 = PA2 // ADC1_IN3
)

// User LD2: the green LED is connected to PB8 on Nucleo-32 boards
const (
	LED         = LED_BUILTIN
	LED_BUILTIN = LED_GREEN
	LED_GREEN   = PB8
)

const (
	// This board does not have a user button
	// Use first GPIO pin by default
	BUTTON = PA0
)

const (
	// UART pins
	// PA2 and PA3 are connected to the ST-Link Virtual Com Port (VCP)
	UART_TX_PIN = PA2
	UART_RX_PIN = PA3

	// I2C pins
	// PB6 / Arduino D5 is SCL
	// PB7 / Arduino D4 is SDA
	I2C0_SCL_PIN = PB6
	I2C0_SDA_PIN = PB7

	// SPI pins
	SPI1_SCK_PIN = PB3
	SPI1_SDI_PIN = PB4
	SPI1_SDO_PIN = PB5
	SPI0_SCK_PIN = SPI1_SCK_PIN
	SPI0_SDI_PIN = SPI1_SDI_PIN
	SPI0_SDO_PIN = SPI1_SDO_PIN
)

var (
	// USART2 is the hardware serial port connected to the onboard ST-LINK
	// debugger to be exposed as virtual COM port over USB on Nucleo boards.
	UART1  = &_UART1
	_UART1 = UART{
		Buffer:            NewRingBuffer(),
		Bus:               stm32.USART2,
		TxAltFuncSelector: AF7_USART1_USART2_USART3,
		RxAltFuncSelector: AF7_USART1_USART2_USART3,
	}
	DefaultUART = UART1

	// I2C1 is documented, alias to I2C0 as well
	I2C1 = &I2C{
		Bus:             stm32.I2C1,
		AltFuncSelector: AF4_I2C1_I2C2_I2C3_I2C4,
	}
	I2C0 = I2C1

	// SPI1 is documented, alias to SPI0 as well
	SPI1 = &SPI{
		Bus:             stm32.SPI1,
		AltFuncSelector: AF5_SPI1_SPI2_I2S2_I2S3,
	}
	SPI0 = SPI1
)

func init() {
	UART1.Interrupt = interrupt.New(stm32.IRQ_USART2, _UART1.handleInterrupt)
}
