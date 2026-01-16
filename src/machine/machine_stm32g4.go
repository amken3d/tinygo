//go:build stm32g4

package machine

// Peripheral abstraction layer for the stm32g4

import (
	"device/stm32"
	"runtime/interrupt"
	"runtime/volatile"
	"unsafe"
)

// Internal use: configured speed of the APB1 and APB2 bus frequencies
const (
	APB1_TIM_FREQ = 170_000_000 // 170MHz
	APB2_TIM_FREQ = 170_000_000 // 170MHz
)

func CPUFrequency() uint32 {
	return 170_000_000 // 170MHz
}

// Unique device ID (96 bits)
var deviceIDAddr = []uintptr{0x1FFF7590, 0x1FFF7594, 0x1FFF7598}

// Alternate function numbers - these are the raw AF values from the datasheet
// Pin-specific mappings are in machine_stm32g4_altfunc.go
const (
	AF_TIM2  = 1 // TIM2 on PA0-PA3, PA5, PA15, PB3, PB10, PB11
	AF_TIM3  = 2 // TIM3 on PA6, PA7, PB0, PB1, PB4, PB5
	AF_TIM4  = 2 // TIM4 on PB6-PB9
	AF_TIM1  = 6 // TIM1 on PA8-PA11
	AF_TIM15 = 9 // TIM15 on PA2, PA3
	AF_TIM16 = 1 // TIM16 on PA6, PB8
)

// IRQ constants for timers
const (
	irq_TIM1_BRK_TIM15 = stm32.IRQ_TIM1_BRK_TIM15
	irq_TIM1_UP_TIM16  = stm32.IRQ_TIM1_UP_TIM16
	irq_TIM1_CC        = stm32.IRQ_TIM1_CC
	irq_TIM2           = stm32.IRQ_TIM2
	irq_TIM3           = stm32.IRQ_TIM3
	irq_TIM4           = stm32.IRQ_TIM4
	irq_TIM6           = stm32.IRQ_TIM6_DACUNDER
	irq_TIM7           = stm32.IRQ_TIM7
	irq_TIM8_BRK       = stm32.IRQ_TIM8_BRK
	irq_TIM8_UP        = stm32.IRQ_TIM8_UP
	irq_TIM8_CC        = stm32.IRQ_TIM8_CC
)

func (p Pin) getPort() *stm32.GPIO_Type {
	switch p / 16 {
	case 0:
		return stm32.GPIOA
	case 1:
		return stm32.GPIOB
	case 2:
		return stm32.GPIOC
	case 3:
		return stm32.GPIOD
	case 5:
		return stm32.GPIOF
	case 6:
		return stm32.GPIOG
	default:
		panic("machine: unknown port")
	}
}

// enableClock enables the clock for this desired GPIO port.
func (p Pin) enableClock() {
	switch p / 16 {
	case 0:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIOAEN)
	case 1:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIOBEN)
	case 2:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIOCEN)
	case 3:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIODEN)
	case 5:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIOFEN)
	case 6:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIOGEN)
	default:
		panic("machine: unknown port")
	}
}

// Enable peripheral clock
func enableAltFuncClock(bus unsafe.Pointer) {
	switch bus {
	case unsafe.Pointer(stm32.PWR): // Power interface clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_PWREN)
	case unsafe.Pointer(stm32.I2C1): // I2C1 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_I2C1EN)
	case unsafe.Pointer(stm32.I2C2): // I2C2 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_I2C2EN)
	case unsafe.Pointer(stm32.I2C3): // I2C3 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_I2C3EN)
	case unsafe.Pointer(stm32.USART1): // USART1 clock enable
		stm32.RCC.APB2ENR.SetBits(stm32.RCC_APB2ENR_USART1EN)
	case unsafe.Pointer(stm32.USART2): // USART2 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_USART2EN)
	case unsafe.Pointer(stm32.USART3): // USART3 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_USART3EN)
	case unsafe.Pointer(stm32.UART4): // UART4 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_UART4EN)
	case unsafe.Pointer(stm32.LPUART1): // LPUART1 clock enable
		stm32.RCC.APB1ENR2.SetBits(stm32.RCC_APB1ENR2_LPUART1EN)
	case unsafe.Pointer(stm32.SPI1): // SPI1 clock enable
		stm32.RCC.APB2ENR.SetBits(stm32.RCC_APB2ENR_SPI1EN)
	case unsafe.Pointer(stm32.SPI2): // SPI2 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_SPI2EN)
	case unsafe.Pointer(stm32.SPI3): // SPI3 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_SPI3EN)
	case unsafe.Pointer(stm32.TIM1): // TIM1 clock enable
		stm32.RCC.APB2ENR.SetBits(stm32.RCC_APB2ENR_TIM1EN)
	case unsafe.Pointer(stm32.TIM2): // TIM2 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_TIM2EN)
	case unsafe.Pointer(stm32.TIM3): // TIM3 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_TIM3EN)
	case unsafe.Pointer(stm32.TIM4): // TIM4 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_TIM4EN)
	case unsafe.Pointer(stm32.TIM6): // TIM6 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_TIM6EN)
	case unsafe.Pointer(stm32.TIM7): // TIM7 clock enable
		stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_TIM7EN)
	case unsafe.Pointer(stm32.TIM8): // TIM8 clock enable
		stm32.RCC.APB2ENR.SetBits(stm32.RCC_APB2ENR_TIM8EN)
	case unsafe.Pointer(stm32.TIM15): // TIM15 clock enable
		stm32.RCC.APB2ENR.SetBits(stm32.RCC_APB2ENR_TIM15EN)
	case unsafe.Pointer(stm32.TIM16): // TIM16 clock enable
		stm32.RCC.APB2ENR.SetBits(stm32.RCC_APB2ENR_TIM16EN)
	case unsafe.Pointer(stm32.SYSCFG): // SYSCFG clock enable
		stm32.RCC.APB2ENR.SetBits(stm32.RCC_APB2ENR_SYSCFGEN)
	}
}

func (p Pin) registerInterrupt() interrupt.Interrupt {
	pin := uint8(p) % 16

	switch pin {
	case 0:
		return interrupt.New(stm32.IRQ_EXTI0, func(interrupt.Interrupt) { handlePinInterrupt(0) })
	case 1:
		return interrupt.New(stm32.IRQ_EXTI1, func(interrupt.Interrupt) { handlePinInterrupt(1) })
	case 2:
		return interrupt.New(stm32.IRQ_EXTI2, func(interrupt.Interrupt) { handlePinInterrupt(2) })
	case 3:
		return interrupt.New(stm32.IRQ_EXTI3, func(interrupt.Interrupt) { handlePinInterrupt(3) })
	case 4:
		return interrupt.New(stm32.IRQ_EXTI4, func(interrupt.Interrupt) { handlePinInterrupt(4) })
	case 5:
		return interrupt.New(stm32.IRQ_EXTI9_5, func(interrupt.Interrupt) { handlePinInterrupt(5) })
	case 6:
		return interrupt.New(stm32.IRQ_EXTI9_5, func(interrupt.Interrupt) { handlePinInterrupt(6) })
	case 7:
		return interrupt.New(stm32.IRQ_EXTI9_5, func(interrupt.Interrupt) { handlePinInterrupt(7) })
	case 8:
		return interrupt.New(stm32.IRQ_EXTI9_5, func(interrupt.Interrupt) { handlePinInterrupt(8) })
	case 9:
		return interrupt.New(stm32.IRQ_EXTI9_5, func(interrupt.Interrupt) { handlePinInterrupt(9) })
	case 10:
		return interrupt.New(stm32.IRQ_EXTI15_10, func(interrupt.Interrupt) { handlePinInterrupt(10) })
	case 11:
		return interrupt.New(stm32.IRQ_EXTI15_10, func(interrupt.Interrupt) { handlePinInterrupt(11) })
	case 12:
		return interrupt.New(stm32.IRQ_EXTI15_10, func(interrupt.Interrupt) { handlePinInterrupt(12) })
	case 13:
		return interrupt.New(stm32.IRQ_EXTI15_10, func(interrupt.Interrupt) { handlePinInterrupt(13) })
	case 14:
		return interrupt.New(stm32.IRQ_EXTI15_10, func(interrupt.Interrupt) { handlePinInterrupt(14) })
	case 15:
		return interrupt.New(stm32.IRQ_EXTI15_10, func(interrupt.Interrupt) { handlePinInterrupt(15) })
	}

	return interrupt.Interrupt{}
}

//---------- UART related code

// Configure the UART.
func (uart *UART) configurePins(config UARTConfig) {
	config.TX.ConfigureAltFunc(PinConfig{Mode: PinModeUARTTX}, uart.TxAltFuncSelector)
	config.RX.ConfigureAltFunc(PinConfig{Mode: PinModeUARTRX}, uart.RxAltFuncSelector)
}

// UART baudrate calc based on the bus and target baud rate
func (uart *UART) getBaudRateDivisor(baudRate uint32) uint32 {
	return CPUFrequency() / baudRate
}

// Register names for STM32G4 family
func (uart *UART) setRegisters() {
	uart.rxReg = &uart.Bus.RDR
	uart.txReg = &uart.Bus.TDR
	uart.statusReg = &uart.Bus.ISR
	uart.txEmptyFlag = stm32.USART_ISR_TXE
}

//---------- SPI related types and code

// SPI on the STM32G4 using MODER / alternate function pins
type SPI struct {
	Bus             *stm32.SPI_Type
	AltFuncSelector uint8
}

func (spi *SPI) config8Bits() {
	// Set rx threshold to 8-bits, so RXNE flag is set for 1 byte
	spi.Bus.CR2.SetBits(stm32.SPI_CR2_FRXTH)
}

// Set baud rate for SPI
func (spi *SPI) getBaudRate(config SPIConfig) uint32 {
	var conf uint32

	localFrequency := config.Frequency
	if localFrequency == 0 {
		localFrequency = 4e6 // 4MHz default
	}

	// SPI1 is on APB2 (170MHz), SPI2/SPI3 on APB1 (170MHz)
	// Both are same frequency with no prescaler
	// Baud rate = peripheral clock / (2^(BR+1))
	// BR=0 -> /2, BR=1 -> /4, ..., BR=7 -> /256

	// 170MHz based frequency thresholds
	// BR values: 0=/2, 1=/4, 2=/8, 3=/16, 4=/32, 5=/64, 6=/128, 7=/256
	switch {
	case localFrequency < 664063: // 170M/256
		conf = 7
	case localFrequency < 1328125: // 170M/128
		conf = 6
	case localFrequency < 2656250: // 170M/64
		conf = 5
	case localFrequency < 5312500: // 170M/32
		conf = 4
	case localFrequency < 10625000: // 170M/16
		conf = 3
	case localFrequency < 21250000: // 170M/8
		conf = 2
	case localFrequency < 42500000: // 170M/4
		conf = 1
	case localFrequency < 85000000: // 170M/2
		conf = 0
	default:
		conf = 7
	}

	return conf << stm32.SPI_CR1_BR_Pos
}

// Configure SPI pins for input output and clock
func (spi *SPI) configurePins(config SPIConfig) {
	config.SCK.ConfigureAltFunc(PinConfig{Mode: PinModeSPICLK}, spi.AltFuncSelector)
	config.SDO.ConfigureAltFunc(PinConfig{Mode: PinModeSPISDO}, spi.AltFuncSelector)
	config.SDI.ConfigureAltFunc(PinConfig{Mode: PinModeSPISDI}, spi.AltFuncSelector)
}

//---------- I2C related code

// I2C timing values for 170MHz PCLK1
// Calculated using STM32CubeMX
func (i2c *I2C) getFreqRange(br uint32) uint32 {
	switch br {
	case 10 * KHz:
		return 0xF010F3FE // 170MHz, 10kHz I2C
	case 100 * KHz:
		return 0x30A0A7FB // 170MHz, 100kHz I2C (Standard mode)
	case 400 * KHz:
		return 0x10802D9B // 170MHz, 400kHz I2C (Fast mode)
	case 500 * KHz:
		return 0x00802172 // 170MHz, 500kHz I2C (Fast mode plus)
	default:
		return 0
	}
}

//---------- Timer related code

var (
	TIM1 = TIM{
		EnableRegister: &stm32.RCC.APB2ENR,
		EnableFlag:     stm32.RCC_APB2ENR_TIM1EN,
		Device:         stm32.TIM1,
		Channels: [4]TimerChannel{
			TimerChannel{Pins: []PinFunction{
				{PA8, AF_TIM1}, // TIM1_CH1
			}},
			TimerChannel{Pins: []PinFunction{
				{PA9, AF_TIM1}, // TIM1_CH2
			}},
			TimerChannel{Pins: []PinFunction{
				{PA10, AF_TIM1}, // TIM1_CH3
			}},
			TimerChannel{Pins: []PinFunction{
				{PA11, AF_TIM1}, // TIM1_CH4
			}},
		},
		busFreq: APB2_TIM_FREQ,
	}

	TIM2 = TIM{
		EnableRegister: &stm32.RCC.APB1ENR1,
		EnableFlag:     stm32.RCC_APB1ENR1_TIM2EN,
		Device:         stm32.TIM2,
		Channels: [4]TimerChannel{
			TimerChannel{Pins: []PinFunction{
				{PA0, AF_TIM2},
				{PA5, AF_TIM2},
				{PA15, AF_TIM2},
			}},
			TimerChannel{Pins: []PinFunction{
				{PA1, AF_TIM2},
				{PB3, AF_TIM2},
			}},
			TimerChannel{Pins: []PinFunction{
				{PA2, AF_TIM2},
				{PB10, AF_TIM2},
			}},
			TimerChannel{Pins: []PinFunction{
				{PA3, AF_TIM2},
				{PB11, AF_TIM2},
			}},
		},
		busFreq: APB1_TIM_FREQ,
	}

	TIM3 = TIM{
		EnableRegister: &stm32.RCC.APB1ENR1,
		EnableFlag:     stm32.RCC_APB1ENR1_TIM3EN,
		Device:         stm32.TIM3,
		Channels: [4]TimerChannel{
			TimerChannel{Pins: []PinFunction{
				{PA6, AF_TIM3},
				{PB4, AF_TIM3},
			}},
			TimerChannel{Pins: []PinFunction{
				{PA7, AF_TIM3},
				{PB5, AF_TIM3},
			}},
			TimerChannel{Pins: []PinFunction{
				{PB0, AF_TIM3},
			}},
			TimerChannel{Pins: []PinFunction{
				{PB1, AF_TIM3},
			}},
		},
		busFreq: APB1_TIM_FREQ,
	}

	TIM4 = TIM{
		EnableRegister: &stm32.RCC.APB1ENR1,
		EnableFlag:     stm32.RCC_APB1ENR1_TIM4EN,
		Device:         stm32.TIM4,
		Channels: [4]TimerChannel{
			TimerChannel{Pins: []PinFunction{
				{PB6, AF_TIM4},
			}},
			TimerChannel{Pins: []PinFunction{
				{PB7, AF_TIM4},
			}},
			TimerChannel{Pins: []PinFunction{
				{PB8, AF_TIM4},
			}},
			TimerChannel{Pins: []PinFunction{
				{PB9, AF_TIM4},
			}},
		},
		busFreq: APB1_TIM_FREQ,
	}

	TIM6 = TIM{
		EnableRegister: &stm32.RCC.APB1ENR1,
		EnableFlag:     stm32.RCC_APB1ENR1_TIM6EN,
		Device:         stm32.TIM6,
		Channels: [4]TimerChannel{
			TimerChannel{Pins: []PinFunction{}},
			TimerChannel{Pins: []PinFunction{}},
			TimerChannel{Pins: []PinFunction{}},
			TimerChannel{Pins: []PinFunction{}},
		},
		busFreq: APB1_TIM_FREQ,
	}

	TIM7 = TIM{
		EnableRegister: &stm32.RCC.APB1ENR1,
		EnableFlag:     stm32.RCC_APB1ENR1_TIM7EN,
		Device:         stm32.TIM7,
		Channels: [4]TimerChannel{
			TimerChannel{Pins: []PinFunction{}},
			TimerChannel{Pins: []PinFunction{}},
			TimerChannel{Pins: []PinFunction{}},
			TimerChannel{Pins: []PinFunction{}},
		},
		busFreq: APB1_TIM_FREQ,
	}

	TIM8 = TIM{
		EnableRegister: &stm32.RCC.APB2ENR,
		EnableFlag:     stm32.RCC_APB2ENR_TIM8EN,
		Device:         stm32.TIM8,
		Channels: [4]TimerChannel{
			TimerChannel{Pins: []PinFunction{}},
			TimerChannel{Pins: []PinFunction{}},
			TimerChannel{Pins: []PinFunction{}},
			TimerChannel{Pins: []PinFunction{}},
		},
		busFreq: APB2_TIM_FREQ,
	}

	TIM15 = TIM{
		EnableRegister: &stm32.RCC.APB2ENR,
		EnableFlag:     stm32.RCC_APB2ENR_TIM15EN,
		Device:         stm32.TIM15,
		Channels: [4]TimerChannel{
			TimerChannel{Pins: []PinFunction{
				{PA2, AF_TIM15}, // TIM15_CH1
			}},
			TimerChannel{Pins: []PinFunction{
				{PA3, AF_TIM15}, // TIM15_CH2
			}},
			TimerChannel{Pins: []PinFunction{}},
			TimerChannel{Pins: []PinFunction{}},
		},
		busFreq: APB2_TIM_FREQ,
	}

	TIM16 = TIM{
		EnableRegister: &stm32.RCC.APB2ENR,
		EnableFlag:     stm32.RCC_APB2ENR_TIM16EN,
		Device:         stm32.TIM16,
		Channels: [4]TimerChannel{
			TimerChannel{Pins: []PinFunction{
				{PA6, AF_TIM16}, // TIM16_CH1
				{PB8, AF_TIM16}, // TIM16_CH1
			}},
			TimerChannel{Pins: []PinFunction{}},
			TimerChannel{Pins: []PinFunction{}},
			TimerChannel{Pins: []PinFunction{}},
		},
		busFreq: APB2_TIM_FREQ,
	}
)

// registerInterrupt registers a unified interrupt handler for the timer.
// This handler processes all timer events (Update, Output Compare, etc.)
// since most STM32G4 timers use a single IRQ for all events.
func (t *TIM) registerInterrupt() interrupt.Interrupt {
	switch t {
	case &TIM1:
		// TIM1 has separate UP and CC IRQs, but we use UP for the unified handler
		return interrupt.New(irq_TIM1_UP_TIM16, TIM1.handleInterrupt)
	case &TIM2:
		return interrupt.New(irq_TIM2, TIM2.handleInterrupt)
	case &TIM3:
		return interrupt.New(irq_TIM3, TIM3.handleInterrupt)
	case &TIM4:
		return interrupt.New(irq_TIM4, TIM4.handleInterrupt)
	case &TIM6:
		return interrupt.New(irq_TIM6, TIM6.handleInterrupt)
	case &TIM7:
		return interrupt.New(irq_TIM7, TIM7.handleInterrupt)
	case &TIM8:
		// TIM8 has separate UP and CC IRQs, but we use UP for the unified handler
		return interrupt.New(irq_TIM8_UP, TIM8.handleInterrupt)
	case &TIM15:
		return interrupt.New(irq_TIM1_BRK_TIM15, TIM15.handleInterrupt)
	case &TIM16:
		return interrupt.New(irq_TIM1_UP_TIM16, TIM16.handleInterrupt)
	}

	return interrupt.Interrupt{}
}

func (t *TIM) enableMainOutput() {
	t.Device.BDTR.SetBits(stm32.TIM_BDTR_MOE)
}

type arrtype = uint32
type arrRegType = volatile.Register32

const (
	ARR_MAX = 0x10000
	PSC_MAX = 0x10000
)

func initRNG() {
	// Enable HSI48 for RNG
	stm32.RCC.CRRCR.SetBits(stm32.RCC_CRRCR_HSI48ON)
	for !stm32.RCC.CRRCR.HasBits(stm32.RCC_CRRCR_HSI48RDY) {
	}

	stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_RNGEN)
	stm32.RNG.CR.SetBits(stm32.RNG_CR_RNGEN)
}
