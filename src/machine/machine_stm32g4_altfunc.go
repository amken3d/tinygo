// Source: STM32G4 Datasheet DS12589 Rev 6 - Alternate Function Table

//go:build stm32g4

package machine

import "device/stm32"

// configureAltFunc sets a pin to alternate function mode with the specified AF number
func (p Pin) configureAltFunc(af uint8) {
	// Get port and pin number
	port := p / 16
	pin := uint8(p % 16)

	// Enable GPIO clock
	switch port {
	case 0:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIOAEN)
	case 1:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIOBEN)
	case 2:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIOCEN)
	case 3:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIODEN)
	case 4:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIOEEN)
	case 5:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIOFEN)
	case 6:
		stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_GPIOGEN)
	}

	// Get GPIO port registers
	var gpio *stm32.GPIO_Type
	switch port {
	case 0:
		gpio = stm32.GPIOA
	case 1:
		gpio = stm32.GPIOB
	case 2:
		gpio = stm32.GPIOC
	case 3:
		gpio = stm32.GPIOD
	case 4:
		gpio = stm32.GPIOE
	case 5:
		gpio = stm32.GPIOF
	case 6:
		gpio = stm32.GPIOG
	}

	// Set alternate function mode (MODER = 0b10)
	gpio.MODER.ReplaceBits(0x2, 0x3, pin*2)

	// Set alternate function in AFR registers
	if pin < 8 {
		gpio.AFRL.ReplaceBits(uint32(af), 0xF, pin*4)
	} else {
		gpio.AFRH.ReplaceBits(uint32(af), 0xF, (pin-8)*4)
	}
}

// =============================================================================
// Pin Constants
// =============================================================================

const (
	PA0  Pin = 0
	PA1  Pin = 1
	PA2  Pin = 2
	PA3  Pin = 3
	PA4  Pin = 4
	PA5  Pin = 5
	PA6  Pin = 6
	PA7  Pin = 7
	PA8  Pin = 8
	PA9  Pin = 9
	PA10 Pin = 10
	PA11 Pin = 11
	PA12 Pin = 12
	PA13 Pin = 13
	PA14 Pin = 14
	PA15 Pin = 15
	PB0  Pin = 16
	PB1  Pin = 17
	PB2  Pin = 18
	PB3  Pin = 19
	PB4  Pin = 20
	PB5  Pin = 21
	PB6  Pin = 22
	PB7  Pin = 23
	PB8  Pin = 24
	PB9  Pin = 25
	PB10 Pin = 26
	PB11 Pin = 27
	PB12 Pin = 28
	PB13 Pin = 29
	PB14 Pin = 30
	PB15 Pin = 31
	PC0  Pin = 32
	PC1  Pin = 33
	PC2  Pin = 34
	PC3  Pin = 35
	PC4  Pin = 36
	PC5  Pin = 37
	PC6  Pin = 38
	PC7  Pin = 39
	PC8  Pin = 40
	PC9  Pin = 41
	PC10 Pin = 42
	PC11 Pin = 43
	PC12 Pin = 44
	PC13 Pin = 45
	PD0  Pin = 48
	PD1  Pin = 49
	PD2  Pin = 50
	PD3  Pin = 51
	PD4  Pin = 52
	PD5  Pin = 53
	PD6  Pin = 54
	PD7  Pin = 55
	PD8  Pin = 56
	PD9  Pin = 57
	PD10 Pin = 58
	PD11 Pin = 59
	PD12 Pin = 60
	PD13 Pin = 61
	PD14 Pin = 62
	PD15 Pin = 63
	PE0  Pin = 64
	PE1  Pin = 65
	PE2  Pin = 66
	PE3  Pin = 67
	PE4  Pin = 68
	PE5  Pin = 69
	PE6  Pin = 70
	PE7  Pin = 71
	PE8  Pin = 72
	PE9  Pin = 73
	PE10 Pin = 74
	PE11 Pin = 75
	PE12 Pin = 76
	PE13 Pin = 77
	PE14 Pin = 78
	PE15 Pin = 79
	PF0  Pin = 80
	PF1  Pin = 81
	PF2  Pin = 82
	PF9  Pin = 89
	PF10 Pin = 90
	PG10 Pin = 106
)
