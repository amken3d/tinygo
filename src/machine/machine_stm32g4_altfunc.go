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

// =============================================================================
// Function Interfaces - Each function has an interface only valid pins implement
// =============================================================================

// COMP1_OUT_Pin is implemented by pins that can serve as COMP1_OUT
type COMP1_OUT_Pin interface {
	ConfigureCOMP1_OUT()
	pin() Pin
}

// COMP2_OUT_Pin is implemented by pins that can serve as COMP2_OUT
type COMP2_OUT_Pin interface {
	ConfigureCOMP2_OUT()
	pin() Pin
}

// COMP3_OUT_Pin is implemented by pins that can serve as COMP3_OUT
type COMP3_OUT_Pin interface {
	ConfigureCOMP3_OUT()
	pin() Pin
}

// COMP4_OUT_Pin is implemented by pins that can serve as COMP4_OUT
type COMP4_OUT_Pin interface {
	ConfigureCOMP4_OUT()
	pin() Pin
}

// FDCAN1_RX_Pin is implemented by pins that can serve as FDCAN1_RX
type FDCAN1_RX_Pin interface {
	ConfigureFDCAN1_RX()
	pin() Pin
}

// FDCAN1_TX_Pin is implemented by pins that can serve as FDCAN1_TX
type FDCAN1_TX_Pin interface {
	ConfigureFDCAN1_TX()
	pin() Pin
}

// I2C1_SCL_Pin is implemented by pins that can serve as I2C1_SCL
type I2C1_SCL_Pin interface {
	ConfigureI2C1_SCL()
	pin() Pin
}

// I2C1_SDA_Pin is implemented by pins that can serve as I2C1_SDA
type I2C1_SDA_Pin interface {
	ConfigureI2C1_SDA()
	pin() Pin
}

// I2C1_SMBA_Pin is implemented by pins that can serve as I2C1_SMBA
type I2C1_SMBA_Pin interface {
	ConfigureI2C1_SMBA()
	pin() Pin
}

// I2C2_SCL_Pin is implemented by pins that can serve as I2C2_SCL
type I2C2_SCL_Pin interface {
	ConfigureI2C2_SCL()
	pin() Pin
}

// I2C2_SDA_Pin is implemented by pins that can serve as I2C2_SDA
type I2C2_SDA_Pin interface {
	ConfigureI2C2_SDA()
	pin() Pin
}

// I2C2_SMBA_Pin is implemented by pins that can serve as I2C2_SMBA
type I2C2_SMBA_Pin interface {
	ConfigureI2C2_SMBA()
	pin() Pin
}

// I2C3_SCL_Pin is implemented by pins that can serve as I2C3_SCL
type I2C3_SCL_Pin interface {
	ConfigureI2C3_SCL()
	pin() Pin
}

// I2C3_SDA_Pin is implemented by pins that can serve as I2C3_SDA
type I2C3_SDA_Pin interface {
	ConfigureI2C3_SDA()
	pin() Pin
}

// I2C3_SMBA_Pin is implemented by pins that can serve as I2C3_SMBA
type I2C3_SMBA_Pin interface {
	ConfigureI2C3_SMBA()
	pin() Pin
}

// I2S2_CK_Pin is implemented by pins that can serve as I2S2_CK
type I2S2_CK_Pin interface {
	ConfigureI2S2_CK()
	pin() Pin
}

// I2S2_MCK_Pin is implemented by pins that can serve as I2S2_MCK
type I2S2_MCK_Pin interface {
	ConfigureI2S2_MCK()
	pin() Pin
}

// I2S2_SD_Pin is implemented by pins that can serve as I2S2_SD
type I2S2_SD_Pin interface {
	ConfigureI2S2_SD()
	pin() Pin
}

// I2S2_WS_Pin is implemented by pins that can serve as I2S2_WS
type I2S2_WS_Pin interface {
	ConfigureI2S2_WS()
	pin() Pin
}

// I2S3_CK_Pin is implemented by pins that can serve as I2S3_CK
type I2S3_CK_Pin interface {
	ConfigureI2S3_CK()
	pin() Pin
}

// I2S3_MCK_Pin is implemented by pins that can serve as I2S3_MCK
type I2S3_MCK_Pin interface {
	ConfigureI2S3_MCK()
	pin() Pin
}

// I2S3_SD_Pin is implemented by pins that can serve as I2S3_SD
type I2S3_SD_Pin interface {
	ConfigureI2S3_SD()
	pin() Pin
}

// I2S3_WS_Pin is implemented by pins that can serve as I2S3_WS
type I2S3_WS_Pin interface {
	ConfigureI2S3_WS()
	pin() Pin
}

// I2SCKIN_Pin is implemented by pins that can serve as I2SCKIN
type I2SCKIN_Pin interface {
	ConfigureI2SCKIN()
	pin() Pin
}

// IR_OUT_Pin is implemented by pins that can serve as IR_OUT
type IR_OUT_Pin interface {
	ConfigureIR_OUT()
	pin() Pin
}

// JTCK_Pin is implemented by pins that can serve as JTCK
type JTCK_Pin interface {
	ConfigureJTCK()
	pin() Pin
}

// JTDI_Pin is implemented by pins that can serve as JTDI
type JTDI_Pin interface {
	ConfigureJTDI()
	pin() Pin
}

// JTDO_Pin is implemented by pins that can serve as JTDO
type JTDO_Pin interface {
	ConfigureJTDO()
	pin() Pin
}

// JTMS_Pin is implemented by pins that can serve as JTMS
type JTMS_Pin interface {
	ConfigureJTMS()
	pin() Pin
}

// JTRST_Pin is implemented by pins that can serve as JTRST
type JTRST_Pin interface {
	ConfigureJTRST()
	pin() Pin
}

// LPTIM1_ETR_Pin is implemented by pins that can serve as LPTIM1_ETR
type LPTIM1_ETR_Pin interface {
	ConfigureLPTIM1_ETR()
	pin() Pin
}

// LPTIM1_IN1_Pin is implemented by pins that can serve as LPTIM1_IN1
type LPTIM1_IN1_Pin interface {
	ConfigureLPTIM1_IN1()
	pin() Pin
}

// LPTIM1_IN2_Pin is implemented by pins that can serve as LPTIM1_IN2
type LPTIM1_IN2_Pin interface {
	ConfigureLPTIM1_IN2()
	pin() Pin
}

// LPTIM1_OUT_Pin is implemented by pins that can serve as LPTIM1_OUT
type LPTIM1_OUT_Pin interface {
	ConfigureLPTIM1_OUT()
	pin() Pin
}

// LPUART1_CTS_Pin is implemented by pins that can serve as LPUART1_CTS
type LPUART1_CTS_Pin interface {
	ConfigureLPUART1_CTS()
	pin() Pin
}

// LPUART1_RTS_DE_Pin is implemented by pins that can serve as LPUART1_RTS_DE
type LPUART1_RTS_DE_Pin interface {
	ConfigureLPUART1_RTS_DE()
	pin() Pin
}

// LPUART1_RX_Pin is implemented by pins that can serve as LPUART1_RX
type LPUART1_RX_Pin interface {
	ConfigureLPUART1_RX()
	pin() Pin
}

// LPUART1_TX_Pin is implemented by pins that can serve as LPUART1_TX
type LPUART1_TX_Pin interface {
	ConfigureLPUART1_TX()
	pin() Pin
}

// MCO_Pin is implemented by pins that can serve as MCO
type MCO_Pin interface {
	ConfigureMCO()
	pin() Pin
}

// RTC_OUT2_Pin is implemented by pins that can serve as RTC_OUT2
type RTC_OUT2_Pin interface {
	ConfigureRTC_OUT2()
	pin() Pin
}

// RTC_REFIN_Pin is implemented by pins that can serve as RTC_REFIN
type RTC_REFIN_Pin interface {
	ConfigureRTC_REFIN()
	pin() Pin
}

// SAI1_CK1_Pin is implemented by pins that can serve as SAI1_CK1
type SAI1_CK1_Pin interface {
	ConfigureSAI1_CK1()
	pin() Pin
}

// SAI1_CK2_Pin is implemented by pins that can serve as SAI1_CK2
type SAI1_CK2_Pin interface {
	ConfigureSAI1_CK2()
	pin() Pin
}

// SAI1_D1_Pin is implemented by pins that can serve as SAI1_D1
type SAI1_D1_Pin interface {
	ConfigureSAI1_D1()
	pin() Pin
}

// SAI1_D2_Pin is implemented by pins that can serve as SAI1_D2
type SAI1_D2_Pin interface {
	ConfigureSAI1_D2()
	pin() Pin
}

// SAI1_D3_Pin is implemented by pins that can serve as SAI1_D3
type SAI1_D3_Pin interface {
	ConfigureSAI1_D3()
	pin() Pin
}

// SPI1_MISO_Pin is implemented by pins that can serve as SPI1_MISO
type SPI1_MISO_Pin interface {
	ConfigureSPI1_MISO()
	pin() Pin
}

// SPI1_MOSI_Pin is implemented by pins that can serve as SPI1_MOSI
type SPI1_MOSI_Pin interface {
	ConfigureSPI1_MOSI()
	pin() Pin
}

// SPI1_NSS_Pin is implemented by pins that can serve as SPI1_NSS
type SPI1_NSS_Pin interface {
	ConfigureSPI1_NSS()
	pin() Pin
}

// SPI1_SCK_Pin is implemented by pins that can serve as SPI1_SCK
type SPI1_SCK_Pin interface {
	ConfigureSPI1_SCK()
	pin() Pin
}

// SPI2_MISO_Pin is implemented by pins that can serve as SPI2_MISO
type SPI2_MISO_Pin interface {
	ConfigureSPI2_MISO()
	pin() Pin
}

// SPI2_MOSI_Pin is implemented by pins that can serve as SPI2_MOSI
type SPI2_MOSI_Pin interface {
	ConfigureSPI2_MOSI()
	pin() Pin
}

// SPI2_NSS_Pin is implemented by pins that can serve as SPI2_NSS
type SPI2_NSS_Pin interface {
	ConfigureSPI2_NSS()
	pin() Pin
}

// SPI2_SCK_Pin is implemented by pins that can serve as SPI2_SCK
type SPI2_SCK_Pin interface {
	ConfigureSPI2_SCK()
	pin() Pin
}

// SPI3_MISO_Pin is implemented by pins that can serve as SPI3_MISO
type SPI3_MISO_Pin interface {
	ConfigureSPI3_MISO()
	pin() Pin
}

// SPI3_MOSI_Pin is implemented by pins that can serve as SPI3_MOSI
type SPI3_MOSI_Pin interface {
	ConfigureSPI3_MOSI()
	pin() Pin
}

// SPI3_NSS_Pin is implemented by pins that can serve as SPI3_NSS
type SPI3_NSS_Pin interface {
	ConfigureSPI3_NSS()
	pin() Pin
}

// SPI3_SCK_Pin is implemented by pins that can serve as SPI3_SCK
type SPI3_SCK_Pin interface {
	ConfigureSPI3_SCK()
	pin() Pin
}

// SWCLK_Pin is implemented by pins that can serve as SWCLK
type SWCLK_Pin interface {
	ConfigureSWCLK()
	pin() Pin
}

// SWDIO_Pin is implemented by pins that can serve as SWDIO
type SWDIO_Pin interface {
	ConfigureSWDIO()
	pin() Pin
}

// TIM15_BKIN_Pin is implemented by pins that can serve as TIM15_BKIN
type TIM15_BKIN_Pin interface {
	ConfigureTIM15_BKIN()
	pin() Pin
}

// TIM15_CH1_Pin is implemented by pins that can serve as TIM15_CH1
type TIM15_CH1_Pin interface {
	ConfigureTIM15_CH1()
	pin() Pin
}

// TIM15_CH1N_Pin is implemented by pins that can serve as TIM15_CH1N
type TIM15_CH1N_Pin interface {
	ConfigureTIM15_CH1N()
	pin() Pin
}

// TIM15_CH2_Pin is implemented by pins that can serve as TIM15_CH2
type TIM15_CH2_Pin interface {
	ConfigureTIM15_CH2()
	pin() Pin
}

// TIM16_BKIN_Pin is implemented by pins that can serve as TIM16_BKIN
type TIM16_BKIN_Pin interface {
	ConfigureTIM16_BKIN()
	pin() Pin
}

// TIM16_CH1_Pin is implemented by pins that can serve as TIM16_CH1
type TIM16_CH1_Pin interface {
	ConfigureTIM16_CH1()
	pin() Pin
}

// TIM16_CH1N_Pin is implemented by pins that can serve as TIM16_CH1N
type TIM16_CH1N_Pin interface {
	ConfigureTIM16_CH1N()
	pin() Pin
}

// TIM17_BKIN_Pin is implemented by pins that can serve as TIM17_BKIN
type TIM17_BKIN_Pin interface {
	ConfigureTIM17_BKIN()
	pin() Pin
}

// TIM17_CH1_Pin is implemented by pins that can serve as TIM17_CH1
type TIM17_CH1_Pin interface {
	ConfigureTIM17_CH1()
	pin() Pin
}

// TIM17_CH1N_Pin is implemented by pins that can serve as TIM17_CH1N
type TIM17_CH1N_Pin interface {
	ConfigureTIM17_CH1N()
	pin() Pin
}

// TIM1_BKIN_Pin is implemented by pins that can serve as TIM1_BKIN
type TIM1_BKIN_Pin interface {
	ConfigureTIM1_BKIN()
	pin() Pin
}

// TIM1_BKIN2_Pin is implemented by pins that can serve as TIM1_BKIN2
type TIM1_BKIN2_Pin interface {
	ConfigureTIM1_BKIN2()
	pin() Pin
}

// TIM1_CH1_Pin is implemented by pins that can serve as TIM1_CH1
type TIM1_CH1_Pin interface {
	ConfigureTIM1_CH1()
	pin() Pin
}

// TIM1_CH1N_Pin is implemented by pins that can serve as TIM1_CH1N
type TIM1_CH1N_Pin interface {
	ConfigureTIM1_CH1N()
	pin() Pin
}

// TIM1_CH2_Pin is implemented by pins that can serve as TIM1_CH2
type TIM1_CH2_Pin interface {
	ConfigureTIM1_CH2()
	pin() Pin
}

// TIM1_CH2N_Pin is implemented by pins that can serve as TIM1_CH2N
type TIM1_CH2N_Pin interface {
	ConfigureTIM1_CH2N()
	pin() Pin
}

// TIM1_CH3_Pin is implemented by pins that can serve as TIM1_CH3
type TIM1_CH3_Pin interface {
	ConfigureTIM1_CH3()
	pin() Pin
}

// TIM1_CH3N_Pin is implemented by pins that can serve as TIM1_CH3N
type TIM1_CH3N_Pin interface {
	ConfigureTIM1_CH3N()
	pin() Pin
}

// TIM1_CH4_Pin is implemented by pins that can serve as TIM1_CH4
type TIM1_CH4_Pin interface {
	ConfigureTIM1_CH4()
	pin() Pin
}

// TIM1_CH4N_Pin is implemented by pins that can serve as TIM1_CH4N
type TIM1_CH4N_Pin interface {
	ConfigureTIM1_CH4N()
	pin() Pin
}

// TIM1_ETR_Pin is implemented by pins that can serve as TIM1_ETR
type TIM1_ETR_Pin interface {
	ConfigureTIM1_ETR()
	pin() Pin
}

// TIM2_CH1_Pin is implemented by pins that can serve as TIM2_CH1
type TIM2_CH1_Pin interface {
	ConfigureTIM2_CH1()
	pin() Pin
}

// TIM2_CH2_Pin is implemented by pins that can serve as TIM2_CH2
type TIM2_CH2_Pin interface {
	ConfigureTIM2_CH2()
	pin() Pin
}

// TIM2_CH3_Pin is implemented by pins that can serve as TIM2_CH3
type TIM2_CH3_Pin interface {
	ConfigureTIM2_CH3()
	pin() Pin
}

// TIM2_CH4_Pin is implemented by pins that can serve as TIM2_CH4
type TIM2_CH4_Pin interface {
	ConfigureTIM2_CH4()
	pin() Pin
}

// TIM2_ETR_Pin is implemented by pins that can serve as TIM2_ETR
type TIM2_ETR_Pin interface {
	ConfigureTIM2_ETR()
	pin() Pin
}

// TIM3_CH1_Pin is implemented by pins that can serve as TIM3_CH1
type TIM3_CH1_Pin interface {
	ConfigureTIM3_CH1()
	pin() Pin
}

// TIM3_CH2_Pin is implemented by pins that can serve as TIM3_CH2
type TIM3_CH2_Pin interface {
	ConfigureTIM3_CH2()
	pin() Pin
}

// TIM3_CH3_Pin is implemented by pins that can serve as TIM3_CH3
type TIM3_CH3_Pin interface {
	ConfigureTIM3_CH3()
	pin() Pin
}

// TIM3_CH4_Pin is implemented by pins that can serve as TIM3_CH4
type TIM3_CH4_Pin interface {
	ConfigureTIM3_CH4()
	pin() Pin
}

// TIM3_ETR_Pin is implemented by pins that can serve as TIM3_ETR
type TIM3_ETR_Pin interface {
	ConfigureTIM3_ETR()
	pin() Pin
}

// TIM4_CH1_Pin is implemented by pins that can serve as TIM4_CH1
type TIM4_CH1_Pin interface {
	ConfigureTIM4_CH1()
	pin() Pin
}

// TIM4_CH2_Pin is implemented by pins that can serve as TIM4_CH2
type TIM4_CH2_Pin interface {
	ConfigureTIM4_CH2()
	pin() Pin
}

// TIM4_CH3_Pin is implemented by pins that can serve as TIM4_CH3
type TIM4_CH3_Pin interface {
	ConfigureTIM4_CH3()
	pin() Pin
}

// TIM4_CH4_Pin is implemented by pins that can serve as TIM4_CH4
type TIM4_CH4_Pin interface {
	ConfigureTIM4_CH4()
	pin() Pin
}

// TIM4_ETR_Pin is implemented by pins that can serve as TIM4_ETR
type TIM4_ETR_Pin interface {
	ConfigureTIM4_ETR()
	pin() Pin
}

// TIM8_BKIN_Pin is implemented by pins that can serve as TIM8_BKIN
type TIM8_BKIN_Pin interface {
	ConfigureTIM8_BKIN()
	pin() Pin
}

// TIM8_BKIN2_Pin is implemented by pins that can serve as TIM8_BKIN2
type TIM8_BKIN2_Pin interface {
	ConfigureTIM8_BKIN2()
	pin() Pin
}

// TIM8_CH1_Pin is implemented by pins that can serve as TIM8_CH1
type TIM8_CH1_Pin interface {
	ConfigureTIM8_CH1()
	pin() Pin
}

// TIM8_CH1N_Pin is implemented by pins that can serve as TIM8_CH1N
type TIM8_CH1N_Pin interface {
	ConfigureTIM8_CH1N()
	pin() Pin
}

// TIM8_CH2_Pin is implemented by pins that can serve as TIM8_CH2
type TIM8_CH2_Pin interface {
	ConfigureTIM8_CH2()
	pin() Pin
}

// TIM8_CH2N_Pin is implemented by pins that can serve as TIM8_CH2N
type TIM8_CH2N_Pin interface {
	ConfigureTIM8_CH2N()
	pin() Pin
}

// TIM8_CH3_Pin is implemented by pins that can serve as TIM8_CH3
type TIM8_CH3_Pin interface {
	ConfigureTIM8_CH3()
	pin() Pin
}

// TIM8_CH3N_Pin is implemented by pins that can serve as TIM8_CH3N
type TIM8_CH3N_Pin interface {
	ConfigureTIM8_CH3N()
	pin() Pin
}

// TIM8_CH4_Pin is implemented by pins that can serve as TIM8_CH4
type TIM8_CH4_Pin interface {
	ConfigureTIM8_CH4()
	pin() Pin
}

// TIM8_CH4N_Pin is implemented by pins that can serve as TIM8_CH4N
type TIM8_CH4N_Pin interface {
	ConfigureTIM8_CH4N()
	pin() Pin
}

// TIM8_ETR_Pin is implemented by pins that can serve as TIM8_ETR
type TIM8_ETR_Pin interface {
	ConfigureTIM8_ETR()
	pin() Pin
}

// TRACECK_Pin is implemented by pins that can serve as TRACECK
type TRACECK_Pin interface {
	ConfigureTRACECK()
	pin() Pin
}

// TRACED0_Pin is implemented by pins that can serve as TRACED0
type TRACED0_Pin interface {
	ConfigureTRACED0()
	pin() Pin
}

// TRACED1_Pin is implemented by pins that can serve as TRACED1
type TRACED1_Pin interface {
	ConfigureTRACED1()
	pin() Pin
}

// TRACED2_Pin is implemented by pins that can serve as TRACED2
type TRACED2_Pin interface {
	ConfigureTRACED2()
	pin() Pin
}

// TRACED3_Pin is implemented by pins that can serve as TRACED3
type TRACED3_Pin interface {
	ConfigureTRACED3()
	pin() Pin
}

// TRACESWO_Pin is implemented by pins that can serve as TRACESWO
type TRACESWO_Pin interface {
	ConfigureTRACESWO()
	pin() Pin
}

// UART4_RTS_DE_Pin is implemented by pins that can serve as UART4_RTS_DE
type UART4_RTS_DE_Pin interface {
	ConfigureUART4_RTS_DE()
	pin() Pin
}

// UART4_RX_Pin is implemented by pins that can serve as UART4_RX
type UART4_RX_Pin interface {
	ConfigureUART4_RX()
	pin() Pin
}

// UART4_TX_Pin is implemented by pins that can serve as UART4_TX
type UART4_TX_Pin interface {
	ConfigureUART4_TX()
	pin() Pin
}

// USART1_CK_Pin is implemented by pins that can serve as USART1_CK
type USART1_CK_Pin interface {
	ConfigureUSART1_CK()
	pin() Pin
}

// USART1_CTS_Pin is implemented by pins that can serve as USART1_CTS
type USART1_CTS_Pin interface {
	ConfigureUSART1_CTS()
	pin() Pin
}

// USART1_RTS_DE_Pin is implemented by pins that can serve as USART1_RTS_DE
type USART1_RTS_DE_Pin interface {
	ConfigureUSART1_RTS_DE()
	pin() Pin
}

// USART1_RX_Pin is implemented by pins that can serve as USART1_RX
type USART1_RX_Pin interface {
	ConfigureUSART1_RX()
	pin() Pin
}

// USART1_TX_Pin is implemented by pins that can serve as USART1_TX
type USART1_TX_Pin interface {
	ConfigureUSART1_TX()
	pin() Pin
}

// USART2_CK_Pin is implemented by pins that can serve as USART2_CK
type USART2_CK_Pin interface {
	ConfigureUSART2_CK()
	pin() Pin
}

// USART2_CTS_Pin is implemented by pins that can serve as USART2_CTS
type USART2_CTS_Pin interface {
	ConfigureUSART2_CTS()
	pin() Pin
}

// USART2_RTS_DE_Pin is implemented by pins that can serve as USART2_RTS_DE
type USART2_RTS_DE_Pin interface {
	ConfigureUSART2_RTS_DE()
	pin() Pin
}

// USART2_RX_Pin is implemented by pins that can serve as USART2_RX
type USART2_RX_Pin interface {
	ConfigureUSART2_RX()
	pin() Pin
}

// USART2_TX_Pin is implemented by pins that can serve as USART2_TX
type USART2_TX_Pin interface {
	ConfigureUSART2_TX()
	pin() Pin
}

// USART3_CK_Pin is implemented by pins that can serve as USART3_CK
type USART3_CK_Pin interface {
	ConfigureUSART3_CK()
	pin() Pin
}

// USART3_CTS_Pin is implemented by pins that can serve as USART3_CTS
type USART3_CTS_Pin interface {
	ConfigureUSART3_CTS()
	pin() Pin
}

// USART3_RTS_DE_Pin is implemented by pins that can serve as USART3_RTS_DE
type USART3_RTS_DE_Pin interface {
	ConfigureUSART3_RTS_DE()
	pin() Pin
}

// USART3_RX_Pin is implemented by pins that can serve as USART3_RX
type USART3_RX_Pin interface {
	ConfigureUSART3_RX()
	pin() Pin
}

// USART3_TX_Pin is implemented by pins that can serve as USART3_TX
type USART3_TX_Pin interface {
	ConfigureUSART3_TX()
	pin() Pin
}

// USB_CRS_SYNC_Pin is implemented by pins that can serve as USB_CRS_SYNC
type USB_CRS_SYNC_Pin interface {
	ConfigureUSB_CRS_SYNC()
	pin() Pin
}

// =============================================================================
// Pin Types - Each physical pin is its own type for type safety
// =============================================================================

type PA0Type struct{ Pin }
type PA1Type struct{ Pin }
type PA2Type struct{ Pin }
type PA3Type struct{ Pin }
type PA4Type struct{ Pin }
type PA5Type struct{ Pin }
type PA6Type struct{ Pin }
type PA7Type struct{ Pin }
type PA8Type struct{ Pin }
type PA9Type struct{ Pin }
type PA10Type struct{ Pin }
type PA11Type struct{ Pin }
type PA12Type struct{ Pin }
type PA13Type struct{ Pin }
type PA14Type struct{ Pin }
type PA15Type struct{ Pin }
type PB0Type struct{ Pin }
type PB1Type struct{ Pin }
type PB2Type struct{ Pin }
type PB3Type struct{ Pin }
type PB4Type struct{ Pin }
type PB5Type struct{ Pin }
type PB6Type struct{ Pin }
type PB7Type struct{ Pin }
type PB8Type struct{ Pin }
type PB9Type struct{ Pin }
type PB10Type struct{ Pin }
type PB11Type struct{ Pin }
type PB12Type struct{ Pin }
type PB13Type struct{ Pin }
type PB14Type struct{ Pin }
type PB15Type struct{ Pin }
type PC0Type struct{ Pin }
type PC1Type struct{ Pin }
type PC2Type struct{ Pin }
type PC3Type struct{ Pin }
type PC4Type struct{ Pin }
type PC5Type struct{ Pin }
type PC6Type struct{ Pin }
type PC7Type struct{ Pin }
type PC8Type struct{ Pin }
type PC9Type struct{ Pin }
type PC10Type struct{ Pin }
type PC11Type struct{ Pin }
type PC12Type struct{ Pin }
type PC13Type struct{ Pin }
type PD0Type struct{ Pin }
type PD1Type struct{ Pin }
type PD2Type struct{ Pin }
type PD3Type struct{ Pin }
type PD4Type struct{ Pin }
type PD5Type struct{ Pin }
type PD6Type struct{ Pin }
type PD7Type struct{ Pin }
type PD8Type struct{ Pin }
type PD9Type struct{ Pin }
type PD10Type struct{ Pin }
type PD11Type struct{ Pin }
type PD12Type struct{ Pin }
type PD13Type struct{ Pin }
type PD14Type struct{ Pin }
type PD15Type struct{ Pin }
type PE0Type struct{ Pin }
type PE1Type struct{ Pin }
type PE2Type struct{ Pin }
type PE3Type struct{ Pin }
type PE4Type struct{ Pin }
type PE5Type struct{ Pin }
type PE6Type struct{ Pin }
type PE7Type struct{ Pin }
type PE8Type struct{ Pin }
type PE9Type struct{ Pin }
type PE10Type struct{ Pin }
type PE11Type struct{ Pin }
type PE12Type struct{ Pin }
type PE13Type struct{ Pin }
type PE14Type struct{ Pin }
type PE15Type struct{ Pin }
type PF0Type struct{ Pin }
type PF1Type struct{ Pin }
type PF2Type struct{ Pin }
type PF9Type struct{ Pin }
type PF10Type struct{ Pin }
type PG10Type struct{ Pin }

// Pin instances
var (
	PA0Pin  = PA0Type{Pin: PA0}
	PA1Pin  = PA1Type{Pin: PA1}
	PA2Pin  = PA2Type{Pin: PA2}
	PA3Pin  = PA3Type{Pin: PA3}
	PA4Pin  = PA4Type{Pin: PA4}
	PA5Pin  = PA5Type{Pin: PA5}
	PA6Pin  = PA6Type{Pin: PA6}
	PA7Pin  = PA7Type{Pin: PA7}
	PA8Pin  = PA8Type{Pin: PA8}
	PA9Pin  = PA9Type{Pin: PA9}
	PA10Pin = PA10Type{Pin: PA10}
	PA11Pin = PA11Type{Pin: PA11}
	PA12Pin = PA12Type{Pin: PA12}
	PA13Pin = PA13Type{Pin: PA13}
	PA14Pin = PA14Type{Pin: PA14}
	PA15Pin = PA15Type{Pin: PA15}
	PB0Pin  = PB0Type{Pin: PB0}
	PB1Pin  = PB1Type{Pin: PB1}
	PB2Pin  = PB2Type{Pin: PB2}
	PB3Pin  = PB3Type{Pin: PB3}
	PB4Pin  = PB4Type{Pin: PB4}
	PB5Pin  = PB5Type{Pin: PB5}
	PB6Pin  = PB6Type{Pin: PB6}
	PB7Pin  = PB7Type{Pin: PB7}
	PB8Pin  = PB8Type{Pin: PB8}
	PB9Pin  = PB9Type{Pin: PB9}
	PB10Pin = PB10Type{Pin: PB10}
	PB11Pin = PB11Type{Pin: PB11}
	PB12Pin = PB12Type{Pin: PB12}
	PB13Pin = PB13Type{Pin: PB13}
	PB14Pin = PB14Type{Pin: PB14}
	PB15Pin = PB15Type{Pin: PB15}
	PC0Pin  = PC0Type{Pin: PC0}
	PC1Pin  = PC1Type{Pin: PC1}
	PC2Pin  = PC2Type{Pin: PC2}
	PC3Pin  = PC3Type{Pin: PC3}
	PC4Pin  = PC4Type{Pin: PC4}
	PC5Pin  = PC5Type{Pin: PC5}
	PC6Pin  = PC6Type{Pin: PC6}
	PC7Pin  = PC7Type{Pin: PC7}
	PC8Pin  = PC8Type{Pin: PC8}
	PC9Pin  = PC9Type{Pin: PC9}
	PC10Pin = PC10Type{Pin: PC10}
	PC11Pin = PC11Type{Pin: PC11}
	PC12Pin = PC12Type{Pin: PC12}
	PC13Pin = PC13Type{Pin: PC13}
	PD0Pin  = PD0Type{Pin: PD0}
	PD1Pin  = PD1Type{Pin: PD1}
	PD2Pin  = PD2Type{Pin: PD2}
	PD3Pin  = PD3Type{Pin: PD3}
	PD4Pin  = PD4Type{Pin: PD4}
	PD5Pin  = PD5Type{Pin: PD5}
	PD6Pin  = PD6Type{Pin: PD6}
	PD7Pin  = PD7Type{Pin: PD7}
	PD8Pin  = PD8Type{Pin: PD8}
	PD9Pin  = PD9Type{Pin: PD9}
	PD10Pin = PD10Type{Pin: PD10}
	PD11Pin = PD11Type{Pin: PD11}
	PD12Pin = PD12Type{Pin: PD12}
	PD13Pin = PD13Type{Pin: PD13}
	PD14Pin = PD14Type{Pin: PD14}
	PD15Pin = PD15Type{Pin: PD15}
	PE0Pin  = PE0Type{Pin: PE0}
	PE1Pin  = PE1Type{Pin: PE1}
	PE2Pin  = PE2Type{Pin: PE2}
	PE3Pin  = PE3Type{Pin: PE3}
	PE4Pin  = PE4Type{Pin: PE4}
	PE5Pin  = PE5Type{Pin: PE5}
	PE6Pin  = PE6Type{Pin: PE6}
	PE7Pin  = PE7Type{Pin: PE7}
	PE8Pin  = PE8Type{Pin: PE8}
	PE9Pin  = PE9Type{Pin: PE9}
	PE10Pin = PE10Type{Pin: PE10}
	PE11Pin = PE11Type{Pin: PE11}
	PE12Pin = PE12Type{Pin: PE12}
	PE13Pin = PE13Type{Pin: PE13}
	PE14Pin = PE14Type{Pin: PE14}
	PE15Pin = PE15Type{Pin: PE15}
	PF0Pin  = PF0Type{Pin: PF0}
	PF1Pin  = PF1Type{Pin: PF1}
	PF2Pin  = PF2Type{Pin: PF2}
	PF9Pin  = PF9Type{Pin: PF9}
	PF10Pin = PF10Type{Pin: PF10}
	PG10Pin = PG10Type{Pin: PG10}
)

// pin() method for all pin types (used by interfaces)
func (p PA0Type) pin() Pin  { return p.Pin }
func (p PA1Type) pin() Pin  { return p.Pin }
func (p PA2Type) pin() Pin  { return p.Pin }
func (p PA3Type) pin() Pin  { return p.Pin }
func (p PA4Type) pin() Pin  { return p.Pin }
func (p PA5Type) pin() Pin  { return p.Pin }
func (p PA6Type) pin() Pin  { return p.Pin }
func (p PA7Type) pin() Pin  { return p.Pin }
func (p PA8Type) pin() Pin  { return p.Pin }
func (p PA9Type) pin() Pin  { return p.Pin }
func (p PA10Type) pin() Pin { return p.Pin }
func (p PA11Type) pin() Pin { return p.Pin }
func (p PA12Type) pin() Pin { return p.Pin }
func (p PA13Type) pin() Pin { return p.Pin }
func (p PA14Type) pin() Pin { return p.Pin }
func (p PA15Type) pin() Pin { return p.Pin }
func (p PB0Type) pin() Pin  { return p.Pin }
func (p PB1Type) pin() Pin  { return p.Pin }
func (p PB2Type) pin() Pin  { return p.Pin }
func (p PB3Type) pin() Pin  { return p.Pin }
func (p PB4Type) pin() Pin  { return p.Pin }
func (p PB5Type) pin() Pin  { return p.Pin }
func (p PB6Type) pin() Pin  { return p.Pin }
func (p PB7Type) pin() Pin  { return p.Pin }
func (p PB8Type) pin() Pin  { return p.Pin }
func (p PB9Type) pin() Pin  { return p.Pin }
func (p PB10Type) pin() Pin { return p.Pin }
func (p PB11Type) pin() Pin { return p.Pin }
func (p PB12Type) pin() Pin { return p.Pin }
func (p PB13Type) pin() Pin { return p.Pin }
func (p PB14Type) pin() Pin { return p.Pin }
func (p PB15Type) pin() Pin { return p.Pin }
func (p PC0Type) pin() Pin  { return p.Pin }
func (p PC1Type) pin() Pin  { return p.Pin }
func (p PC2Type) pin() Pin  { return p.Pin }
func (p PC3Type) pin() Pin  { return p.Pin }
func (p PC4Type) pin() Pin  { return p.Pin }
func (p PC5Type) pin() Pin  { return p.Pin }
func (p PC6Type) pin() Pin  { return p.Pin }
func (p PC7Type) pin() Pin  { return p.Pin }
func (p PC8Type) pin() Pin  { return p.Pin }
func (p PC9Type) pin() Pin  { return p.Pin }
func (p PC10Type) pin() Pin { return p.Pin }
func (p PC11Type) pin() Pin { return p.Pin }
func (p PC12Type) pin() Pin { return p.Pin }
func (p PC13Type) pin() Pin { return p.Pin }
func (p PD0Type) pin() Pin  { return p.Pin }
func (p PD1Type) pin() Pin  { return p.Pin }
func (p PD2Type) pin() Pin  { return p.Pin }
func (p PD3Type) pin() Pin  { return p.Pin }
func (p PD4Type) pin() Pin  { return p.Pin }
func (p PD5Type) pin() Pin  { return p.Pin }
func (p PD6Type) pin() Pin  { return p.Pin }
func (p PD7Type) pin() Pin  { return p.Pin }
func (p PD8Type) pin() Pin  { return p.Pin }
func (p PD9Type) pin() Pin  { return p.Pin }
func (p PD10Type) pin() Pin { return p.Pin }
func (p PD11Type) pin() Pin { return p.Pin }
func (p PD12Type) pin() Pin { return p.Pin }
func (p PD13Type) pin() Pin { return p.Pin }
func (p PD14Type) pin() Pin { return p.Pin }
func (p PD15Type) pin() Pin { return p.Pin }
func (p PE0Type) pin() Pin  { return p.Pin }
func (p PE1Type) pin() Pin  { return p.Pin }
func (p PE2Type) pin() Pin  { return p.Pin }
func (p PE3Type) pin() Pin  { return p.Pin }
func (p PE4Type) pin() Pin  { return p.Pin }
func (p PE5Type) pin() Pin  { return p.Pin }
func (p PE6Type) pin() Pin  { return p.Pin }
func (p PE7Type) pin() Pin  { return p.Pin }
func (p PE8Type) pin() Pin  { return p.Pin }
func (p PE9Type) pin() Pin  { return p.Pin }
func (p PE10Type) pin() Pin { return p.Pin }
func (p PE11Type) pin() Pin { return p.Pin }
func (p PE12Type) pin() Pin { return p.Pin }
func (p PE13Type) pin() Pin { return p.Pin }
func (p PE14Type) pin() Pin { return p.Pin }
func (p PE15Type) pin() Pin { return p.Pin }
func (p PF0Type) pin() Pin  { return p.Pin }
func (p PF1Type) pin() Pin  { return p.Pin }
func (p PF2Type) pin() Pin  { return p.Pin }
func (p PF9Type) pin() Pin  { return p.Pin }
func (p PF10Type) pin() Pin { return p.Pin }
func (p PG10Type) pin() Pin { return p.Pin }

// =============================================================================
// Pin Method Implementations - Each pin implements only its valid functions
// =============================================================================

// PA0 alternate functions
func (p PA0Type) ConfigureTIM2_CH1()   { p.Pin.configureAltFunc(1) }
func (p PA0Type) ConfigureUSART2_CTS() { p.Pin.configureAltFunc(7) }
func (p PA0Type) ConfigureCOMP1_OUT()  { p.Pin.configureAltFunc(8) }
func (p PA0Type) ConfigureTIM8_BKIN()  { p.Pin.configureAltFunc(9) }
func (p PA0Type) ConfigureTIM8_ETR()   { p.Pin.configureAltFunc(10) }

// PA1 alternate functions
func (p PA1Type) ConfigureRTC_REFIN()     { p.Pin.configureAltFunc(0) }
func (p PA1Type) ConfigureTIM2_CH2()      { p.Pin.configureAltFunc(1) }
func (p PA1Type) ConfigureUSART2_RTS_DE() { p.Pin.configureAltFunc(7) }
func (p PA1Type) ConfigureTIM15_CH1N()    { p.Pin.configureAltFunc(9) }

// PA2 alternate functions
func (p PA2Type) ConfigureTIM2_CH3()  { p.Pin.configureAltFunc(1) }
func (p PA2Type) ConfigureUSART2_TX() { p.Pin.configureAltFunc(7) }
func (p PA2Type) ConfigureCOMP2_OUT() { p.Pin.configureAltFunc(8) }
func (p PA2Type) ConfigureTIM15_CH1() { p.Pin.configureAltFunc(9) }

// PA3 alternate functions
func (p PA3Type) ConfigureTIM2_CH4()  { p.Pin.configureAltFunc(1) }
func (p PA3Type) ConfigureSAI1_CK1()  { p.Pin.configureAltFunc(3) }
func (p PA3Type) ConfigureUSART2_RX() { p.Pin.configureAltFunc(7) }
func (p PA3Type) ConfigureTIM15_CH2() { p.Pin.configureAltFunc(9) }

// PA4 alternate functions
func (p PA4Type) ConfigureTIM3_CH2()  { p.Pin.configureAltFunc(2) }
func (p PA4Type) ConfigureSPI1_NSS()  { p.Pin.configureAltFunc(5) }
func (p PA4Type) ConfigureSPI3_NSS()  { p.Pin.configureAltFunc(6) }
func (p PA4Type) ConfigureI2S3_WS()   { p.Pin.configureAltFunc(6) }
func (p PA4Type) ConfigureUSART2_CK() { p.Pin.configureAltFunc(7) }

// PA5 alternate functions
func (p PA5Type) ConfigureTIM2_CH1() { p.Pin.configureAltFunc(1) }
func (p PA5Type) ConfigureTIM2_ETR() { p.Pin.configureAltFunc(2) }
func (p PA5Type) ConfigureSPI1_SCK() { p.Pin.configureAltFunc(5) }

// PA6 alternate functions
func (p PA6Type) ConfigureTIM16_CH1() { p.Pin.configureAltFunc(1) }
func (p PA6Type) ConfigureTIM3_CH1()  { p.Pin.configureAltFunc(2) }
func (p PA6Type) ConfigureTIM8_BKIN() { p.Pin.configureAltFunc(4) }
func (p PA6Type) ConfigureSPI1_MISO() { p.Pin.configureAltFunc(5) }
func (p PA6Type) ConfigureTIM1_BKIN() { p.Pin.configureAltFunc(6) }
func (p PA6Type) ConfigureCOMP1_OUT() { p.Pin.configureAltFunc(8) }

// PA7 alternate functions
func (p PA7Type) ConfigureTIM17_CH1() { p.Pin.configureAltFunc(1) }
func (p PA7Type) ConfigureTIM3_CH2()  { p.Pin.configureAltFunc(2) }
func (p PA7Type) ConfigureTIM8_CH1N() { p.Pin.configureAltFunc(4) }
func (p PA7Type) ConfigureSPI1_MOSI() { p.Pin.configureAltFunc(5) }
func (p PA7Type) ConfigureTIM1_CH1N() { p.Pin.configureAltFunc(6) }
func (p PA7Type) ConfigureCOMP2_OUT() { p.Pin.configureAltFunc(8) }

// PA8 alternate functions
func (p PA8Type) ConfigureMCO()       { p.Pin.configureAltFunc(0) }
func (p PA8Type) ConfigureI2C3_SCL()  { p.Pin.configureAltFunc(2) }
func (p PA8Type) ConfigureI2C2_SDA()  { p.Pin.configureAltFunc(4) }
func (p PA8Type) ConfigureI2S2_MCK()  { p.Pin.configureAltFunc(5) }
func (p PA8Type) ConfigureTIM1_CH1()  { p.Pin.configureAltFunc(6) }
func (p PA8Type) ConfigureUSART1_CK() { p.Pin.configureAltFunc(7) }
func (p PA8Type) ConfigureTIM4_ETR()  { p.Pin.configureAltFunc(10) }

// PA9 alternate functions
func (p PA9Type) ConfigureI2C3_SMBA()  { p.Pin.configureAltFunc(2) }
func (p PA9Type) ConfigureI2C2_SCL()   { p.Pin.configureAltFunc(4) }
func (p PA9Type) ConfigureI2S3_MCK()   { p.Pin.configureAltFunc(5) }
func (p PA9Type) ConfigureTIM1_CH2()   { p.Pin.configureAltFunc(6) }
func (p PA9Type) ConfigureUSART1_TX()  { p.Pin.configureAltFunc(7) }
func (p PA9Type) ConfigureTIM15_BKIN() { p.Pin.configureAltFunc(9) }
func (p PA9Type) ConfigureTIM2_CH3()   { p.Pin.configureAltFunc(10) }

// PA10 alternate functions
func (p PA10Type) ConfigureTIM17_BKIN()   { p.Pin.configureAltFunc(1) }
func (p PA10Type) ConfigureUSB_CRS_SYNC() { p.Pin.configureAltFunc(3) }
func (p PA10Type) ConfigureI2C2_SMBA()    { p.Pin.configureAltFunc(4) }
func (p PA10Type) ConfigureSPI2_MISO()    { p.Pin.configureAltFunc(5) }
func (p PA10Type) ConfigureTIM1_CH3()     { p.Pin.configureAltFunc(6) }
func (p PA10Type) ConfigureUSART1_RX()    { p.Pin.configureAltFunc(7) }
func (p PA10Type) ConfigureTIM2_CH4()     { p.Pin.configureAltFunc(10) }
func (p PA10Type) ConfigureTIM8_BKIN()    { p.Pin.configureAltFunc(11) }

// PA11 alternate functions
func (p PA11Type) ConfigureSPI2_MOSI()  { p.Pin.configureAltFunc(5) }
func (p PA11Type) ConfigureI2S2_SD()    { p.Pin.configureAltFunc(5) }
func (p PA11Type) ConfigureTIM1_CH1N()  { p.Pin.configureAltFunc(6) }
func (p PA11Type) ConfigureUSART1_CTS() { p.Pin.configureAltFunc(7) }
func (p PA11Type) ConfigureCOMP1_OUT()  { p.Pin.configureAltFunc(8) }
func (p PA11Type) ConfigureFDCAN1_RX()  { p.Pin.configureAltFunc(9) }
func (p PA11Type) ConfigureTIM4_CH1()   { p.Pin.configureAltFunc(10) }
func (p PA11Type) ConfigureTIM1_CH4()   { p.Pin.configureAltFunc(11) }

// PA12 alternate functions
func (p PA12Type) ConfigureTIM16_CH1()     { p.Pin.configureAltFunc(1) }
func (p PA12Type) ConfigureI2SCKIN()       { p.Pin.configureAltFunc(5) }
func (p PA12Type) ConfigureTIM1_CH2N()     { p.Pin.configureAltFunc(6) }
func (p PA12Type) ConfigureUSART1_RTS_DE() { p.Pin.configureAltFunc(7) }
func (p PA12Type) ConfigureCOMP2_OUT()     { p.Pin.configureAltFunc(8) }
func (p PA12Type) ConfigureFDCAN1_TX()     { p.Pin.configureAltFunc(9) }
func (p PA12Type) ConfigureTIM4_CH2()      { p.Pin.configureAltFunc(10) }
func (p PA12Type) ConfigureTIM1_ETR()      { p.Pin.configureAltFunc(11) }

// PA13 alternate functions
func (p PA13Type) ConfigureSWDIO()      { p.Pin.configureAltFunc(0) }
func (p PA13Type) ConfigureJTMS()       { p.Pin.configureAltFunc(0) }
func (p PA13Type) ConfigureTIM16_CH1N() { p.Pin.configureAltFunc(1) }
func (p PA13Type) ConfigureI2C1_SCL()   { p.Pin.configureAltFunc(4) }
func (p PA13Type) ConfigureIR_OUT()     { p.Pin.configureAltFunc(5) }
func (p PA13Type) ConfigureUSART3_CTS() { p.Pin.configureAltFunc(7) }
func (p PA13Type) ConfigureTIM4_CH3()   { p.Pin.configureAltFunc(10) }

// PA14 alternate functions
func (p PA14Type) ConfigureSWCLK()      { p.Pin.configureAltFunc(0) }
func (p PA14Type) ConfigureJTCK()       { p.Pin.configureAltFunc(0) }
func (p PA14Type) ConfigureLPTIM1_OUT() { p.Pin.configureAltFunc(1) }
func (p PA14Type) ConfigureI2C1_SDA()   { p.Pin.configureAltFunc(4) }
func (p PA14Type) ConfigureTIM8_CH2()   { p.Pin.configureAltFunc(5) }
func (p PA14Type) ConfigureTIM1_BKIN()  { p.Pin.configureAltFunc(6) }
func (p PA14Type) ConfigureUSART2_TX()  { p.Pin.configureAltFunc(7) }

// PA15 alternate functions
func (p PA15Type) ConfigureJTDI()         { p.Pin.configureAltFunc(0) }
func (p PA15Type) ConfigureTIM2_CH1()     { p.Pin.configureAltFunc(1) }
func (p PA15Type) ConfigureTIM8_CH1()     { p.Pin.configureAltFunc(2) }
func (p PA15Type) ConfigureI2C1_SCL()     { p.Pin.configureAltFunc(4) }
func (p PA15Type) ConfigureSPI1_NSS()     { p.Pin.configureAltFunc(5) }
func (p PA15Type) ConfigureSPI3_NSS()     { p.Pin.configureAltFunc(6) }
func (p PA15Type) ConfigureI2S3_WS()      { p.Pin.configureAltFunc(6) }
func (p PA15Type) ConfigureUSART2_RX()    { p.Pin.configureAltFunc(7) }
func (p PA15Type) ConfigureUART4_RTS_DE() { p.Pin.configureAltFunc(8) }
func (p PA15Type) ConfigureTIM1_BKIN()    { p.Pin.configureAltFunc(9) }

// PB0 alternate functions
func (p PB0Type) ConfigureTIM3_CH3()  { p.Pin.configureAltFunc(2) }
func (p PB0Type) ConfigureTIM8_CH2N() { p.Pin.configureAltFunc(4) }
func (p PB0Type) ConfigureTIM1_CH2N() { p.Pin.configureAltFunc(6) }

// PB1 alternate functions
func (p PB1Type) ConfigureTIM3_CH4()  { p.Pin.configureAltFunc(2) }
func (p PB1Type) ConfigureTIM8_CH3N() { p.Pin.configureAltFunc(4) }
func (p PB1Type) ConfigureTIM1_CH3N() { p.Pin.configureAltFunc(6) }
func (p PB1Type) ConfigureCOMP4_OUT() { p.Pin.configureAltFunc(8) }

// PB2 alternate functions
func (p PB2Type) ConfigureRTC_OUT2()   { p.Pin.configureAltFunc(0) }
func (p PB2Type) ConfigureLPTIM1_OUT() { p.Pin.configureAltFunc(1) }
func (p PB2Type) ConfigureI2C3_SMBA()  { p.Pin.configureAltFunc(4) }

// PB3 alternate functions
func (p PB3Type) ConfigureJTDO()         { p.Pin.configureAltFunc(0) }
func (p PB3Type) ConfigureTRACESWO()     { p.Pin.configureAltFunc(0) }
func (p PB3Type) ConfigureTIM2_CH2()     { p.Pin.configureAltFunc(1) }
func (p PB3Type) ConfigureTIM4_ETR()     { p.Pin.configureAltFunc(2) }
func (p PB3Type) ConfigureUSB_CRS_SYNC() { p.Pin.configureAltFunc(3) }
func (p PB3Type) ConfigureTIM8_CH1N()    { p.Pin.configureAltFunc(4) }
func (p PB3Type) ConfigureSPI1_SCK()     { p.Pin.configureAltFunc(5) }
func (p PB3Type) ConfigureSPI3_SCK()     { p.Pin.configureAltFunc(6) }
func (p PB3Type) ConfigureI2S3_CK()      { p.Pin.configureAltFunc(6) }
func (p PB3Type) ConfigureUSART2_TX()    { p.Pin.configureAltFunc(7) }
func (p PB3Type) ConfigureTIM3_ETR()     { p.Pin.configureAltFunc(10) }

// PB4 alternate functions
func (p PB4Type) ConfigureJTRST()      { p.Pin.configureAltFunc(0) }
func (p PB4Type) ConfigureTIM16_CH1()  { p.Pin.configureAltFunc(1) }
func (p PB4Type) ConfigureTIM3_CH1()   { p.Pin.configureAltFunc(2) }
func (p PB4Type) ConfigureTIM8_CH2N()  { p.Pin.configureAltFunc(4) }
func (p PB4Type) ConfigureSPI1_MISO()  { p.Pin.configureAltFunc(5) }
func (p PB4Type) ConfigureSPI3_MISO()  { p.Pin.configureAltFunc(6) }
func (p PB4Type) ConfigureUSART2_RX()  { p.Pin.configureAltFunc(7) }
func (p PB4Type) ConfigureTIM17_BKIN() { p.Pin.configureAltFunc(10) }

// PB5 alternate functions
func (p PB5Type) ConfigureTIM16_BKIN() { p.Pin.configureAltFunc(1) }
func (p PB5Type) ConfigureTIM3_CH2()   { p.Pin.configureAltFunc(2) }
func (p PB5Type) ConfigureTIM8_CH3N()  { p.Pin.configureAltFunc(3) }
func (p PB5Type) ConfigureI2C1_SMBA()  { p.Pin.configureAltFunc(4) }
func (p PB5Type) ConfigureSPI1_MOSI()  { p.Pin.configureAltFunc(5) }
func (p PB5Type) ConfigureSPI3_MOSI()  { p.Pin.configureAltFunc(6) }
func (p PB5Type) ConfigureI2S3_SD()    { p.Pin.configureAltFunc(6) }
func (p PB5Type) ConfigureUSART2_CK()  { p.Pin.configureAltFunc(7) }
func (p PB5Type) ConfigureI2C3_SDA()   { p.Pin.configureAltFunc(8) }
func (p PB5Type) ConfigureTIM17_CH1()  { p.Pin.configureAltFunc(10) }
func (p PB5Type) ConfigureLPTIM1_IN1() { p.Pin.configureAltFunc(11) }

// PB6 alternate functions
func (p PB6Type) ConfigureTIM16_CH1N() { p.Pin.configureAltFunc(1) }
func (p PB6Type) ConfigureTIM4_CH1()   { p.Pin.configureAltFunc(2) }
func (p PB6Type) ConfigureTIM8_CH1()   { p.Pin.configureAltFunc(5) }
func (p PB6Type) ConfigureTIM8_ETR()   { p.Pin.configureAltFunc(6) }
func (p PB6Type) ConfigureUSART1_TX()  { p.Pin.configureAltFunc(7) }
func (p PB6Type) ConfigureCOMP4_OUT()  { p.Pin.configureAltFunc(8) }
func (p PB6Type) ConfigureTIM8_BKIN2() { p.Pin.configureAltFunc(10) }
func (p PB6Type) ConfigureLPTIM1_ETR() { p.Pin.configureAltFunc(11) }

// PB7 alternate functions
func (p PB7Type) ConfigureTIM17_CH1N() { p.Pin.configureAltFunc(1) }
func (p PB7Type) ConfigureTIM4_CH2()   { p.Pin.configureAltFunc(2) }
func (p PB7Type) ConfigureI2C1_SDA()   { p.Pin.configureAltFunc(4) }
func (p PB7Type) ConfigureTIM8_BKIN()  { p.Pin.configureAltFunc(5) }
func (p PB7Type) ConfigureUSART1_RX()  { p.Pin.configureAltFunc(7) }
func (p PB7Type) ConfigureCOMP3_OUT()  { p.Pin.configureAltFunc(8) }
func (p PB7Type) ConfigureTIM3_CH4()   { p.Pin.configureAltFunc(10) }
func (p PB7Type) ConfigureLPTIM1_IN2() { p.Pin.configureAltFunc(11) }

// PB8 alternate functions
func (p PB8Type) ConfigureTIM16_CH1() { p.Pin.configureAltFunc(1) }
func (p PB8Type) ConfigureTIM4_CH3()  { p.Pin.configureAltFunc(2) }
func (p PB8Type) ConfigureSAI1_CK1()  { p.Pin.configureAltFunc(3) }
func (p PB8Type) ConfigureI2C1_SCL()  { p.Pin.configureAltFunc(4) }
func (p PB8Type) ConfigureUSART3_RX() { p.Pin.configureAltFunc(7) }
func (p PB8Type) ConfigureCOMP1_OUT() { p.Pin.configureAltFunc(8) }
func (p PB8Type) ConfigureFDCAN1_RX() { p.Pin.configureAltFunc(9) }
func (p PB8Type) ConfigureTIM8_CH2()  { p.Pin.configureAltFunc(10) }

// PB9 alternate functions
func (p PB9Type) ConfigureTIM17_CH1() { p.Pin.configureAltFunc(1) }
func (p PB9Type) ConfigureTIM4_CH4()  { p.Pin.configureAltFunc(2) }
func (p PB9Type) ConfigureSAI1_D2()   { p.Pin.configureAltFunc(3) }
func (p PB9Type) ConfigureI2C1_SDA()  { p.Pin.configureAltFunc(4) }
func (p PB9Type) ConfigureIR_OUT()    { p.Pin.configureAltFunc(6) }
func (p PB9Type) ConfigureUSART3_TX() { p.Pin.configureAltFunc(7) }
func (p PB9Type) ConfigureCOMP2_OUT() { p.Pin.configureAltFunc(8) }
func (p PB9Type) ConfigureFDCAN1_TX() { p.Pin.configureAltFunc(9) }
func (p PB9Type) ConfigureTIM8_CH3()  { p.Pin.configureAltFunc(10) }

// PB10 alternate functions
func (p PB10Type) ConfigureTIM2_CH3()   { p.Pin.configureAltFunc(1) }
func (p PB10Type) ConfigureUSART3_TX()  { p.Pin.configureAltFunc(7) }
func (p PB10Type) ConfigureLPUART1_RX() { p.Pin.configureAltFunc(8) }

// PB11 alternate functions
func (p PB11Type) ConfigureTIM2_CH4()   { p.Pin.configureAltFunc(1) }
func (p PB11Type) ConfigureUSART3_RX()  { p.Pin.configureAltFunc(7) }
func (p PB11Type) ConfigureLPUART1_TX() { p.Pin.configureAltFunc(8) }

// PB12 alternate functions
func (p PB12Type) ConfigureI2C2_SMBA()      { p.Pin.configureAltFunc(4) }
func (p PB12Type) ConfigureSPI2_NSS()       { p.Pin.configureAltFunc(5) }
func (p PB12Type) ConfigureI2S2_WS()        { p.Pin.configureAltFunc(5) }
func (p PB12Type) ConfigureTIM1_BKIN()      { p.Pin.configureAltFunc(6) }
func (p PB12Type) ConfigureUSART3_CK()      { p.Pin.configureAltFunc(7) }
func (p PB12Type) ConfigureLPUART1_RTS_DE() { p.Pin.configureAltFunc(8) }

// PB13 alternate functions
func (p PB13Type) ConfigureSPI2_SCK()    { p.Pin.configureAltFunc(5) }
func (p PB13Type) ConfigureI2S2_CK()     { p.Pin.configureAltFunc(5) }
func (p PB13Type) ConfigureTIM1_CH1N()   { p.Pin.configureAltFunc(6) }
func (p PB13Type) ConfigureUSART3_CTS()  { p.Pin.configureAltFunc(7) }
func (p PB13Type) ConfigureLPUART1_CTS() { p.Pin.configureAltFunc(8) }

// PB14 alternate functions
func (p PB14Type) ConfigureTIM15_CH1()     { p.Pin.configureAltFunc(1) }
func (p PB14Type) ConfigureSPI2_MISO()     { p.Pin.configureAltFunc(5) }
func (p PB14Type) ConfigureTIM1_CH2N()     { p.Pin.configureAltFunc(6) }
func (p PB14Type) ConfigureUSART3_RTS_DE() { p.Pin.configureAltFunc(7) }
func (p PB14Type) ConfigureCOMP4_OUT()     { p.Pin.configureAltFunc(8) }

// PB15 alternate functions
func (p PB15Type) ConfigureRTC_REFIN()  { p.Pin.configureAltFunc(0) }
func (p PB15Type) ConfigureTIM15_CH2()  { p.Pin.configureAltFunc(1) }
func (p PB15Type) ConfigureTIM15_CH1N() { p.Pin.configureAltFunc(2) }
func (p PB15Type) ConfigureCOMP3_OUT()  { p.Pin.configureAltFunc(3) }
func (p PB15Type) ConfigureTIM1_CH3N()  { p.Pin.configureAltFunc(4) }
func (p PB15Type) ConfigureSPI2_MOSI()  { p.Pin.configureAltFunc(5) }
func (p PB15Type) ConfigureI2S2_SD()    { p.Pin.configureAltFunc(5) }

// PC0 alternate functions
func (p PC0Type) ConfigureLPTIM1_IN1() { p.Pin.configureAltFunc(1) }
func (p PC0Type) ConfigureTIM1_CH1()   { p.Pin.configureAltFunc(2) }
func (p PC0Type) ConfigureLPUART1_RX() { p.Pin.configureAltFunc(8) }

// PC1 alternate functions
func (p PC1Type) ConfigureLPTIM1_OUT() { p.Pin.configureAltFunc(1) }
func (p PC1Type) ConfigureTIM1_CH2()   { p.Pin.configureAltFunc(2) }
func (p PC1Type) ConfigureLPUART1_TX() { p.Pin.configureAltFunc(8) }

// PC2 alternate functions
func (p PC2Type) ConfigureLPTIM1_IN2() { p.Pin.configureAltFunc(1) }
func (p PC2Type) ConfigureTIM1_CH3()   { p.Pin.configureAltFunc(2) }
func (p PC2Type) ConfigureCOMP3_OUT()  { p.Pin.configureAltFunc(3) }

// PC3 alternate functions
func (p PC3Type) ConfigureLPTIM1_ETR() { p.Pin.configureAltFunc(1) }
func (p PC3Type) ConfigureTIM1_CH4()   { p.Pin.configureAltFunc(2) }
func (p PC3Type) ConfigureSAI1_D1()    { p.Pin.configureAltFunc(3) }
func (p PC3Type) ConfigureTIM1_BKIN2() { p.Pin.configureAltFunc(6) }

// PC4 alternate functions
func (p PC4Type) ConfigureTIM1_ETR()  { p.Pin.configureAltFunc(2) }
func (p PC4Type) ConfigureI2C2_SCL()  { p.Pin.configureAltFunc(4) }
func (p PC4Type) ConfigureUSART1_TX() { p.Pin.configureAltFunc(7) }

// PC5 alternate functions
func (p PC5Type) ConfigureTIM15_BKIN() { p.Pin.configureAltFunc(2) }
func (p PC5Type) ConfigureSAI1_D3()    { p.Pin.configureAltFunc(3) }
func (p PC5Type) ConfigureTIM1_CH4N()  { p.Pin.configureAltFunc(6) }
func (p PC5Type) ConfigureUSART1_RX()  { p.Pin.configureAltFunc(7) }

// PC6 alternate functions
func (p PC6Type) ConfigureTIM3_CH1() { p.Pin.configureAltFunc(2) }
func (p PC6Type) ConfigureTIM8_CH1() { p.Pin.configureAltFunc(4) }
func (p PC6Type) ConfigureI2S2_MCK() { p.Pin.configureAltFunc(6) }

// PC7 alternate functions
func (p PC7Type) ConfigureTIM3_CH2() { p.Pin.configureAltFunc(2) }
func (p PC7Type) ConfigureTIM8_CH2() { p.Pin.configureAltFunc(4) }
func (p PC7Type) ConfigureI2S3_MCK() { p.Pin.configureAltFunc(6) }

// PC8 alternate functions
func (p PC8Type) ConfigureTIM3_CH3() { p.Pin.configureAltFunc(2) }
func (p PC8Type) ConfigureTIM8_CH3() { p.Pin.configureAltFunc(4) }
func (p PC8Type) ConfigureI2C3_SCL() { p.Pin.configureAltFunc(8) }

// PC9 alternate functions
func (p PC9Type) ConfigureTIM3_CH4()   { p.Pin.configureAltFunc(2) }
func (p PC9Type) ConfigureTIM8_CH4()   { p.Pin.configureAltFunc(4) }
func (p PC9Type) ConfigureI2SCKIN()    { p.Pin.configureAltFunc(5) }
func (p PC9Type) ConfigureTIM8_BKIN2() { p.Pin.configureAltFunc(6) }
func (p PC9Type) ConfigureI2C3_SDA()   { p.Pin.configureAltFunc(8) }

// PC10 alternate functions
func (p PC10Type) ConfigureTIM8_CH1N() { p.Pin.configureAltFunc(4) }
func (p PC10Type) ConfigureUART4_TX()  { p.Pin.configureAltFunc(5) }
func (p PC10Type) ConfigureSPI3_SCK()  { p.Pin.configureAltFunc(6) }
func (p PC10Type) ConfigureI2S3_CK()   { p.Pin.configureAltFunc(6) }
func (p PC10Type) ConfigureUSART3_TX() { p.Pin.configureAltFunc(7) }

// PC11 alternate functions
func (p PC11Type) ConfigureTIM8_CH2N() { p.Pin.configureAltFunc(4) }
func (p PC11Type) ConfigureUART4_RX()  { p.Pin.configureAltFunc(5) }
func (p PC11Type) ConfigureSPI3_MISO() { p.Pin.configureAltFunc(6) }
func (p PC11Type) ConfigureUSART3_RX() { p.Pin.configureAltFunc(7) }
func (p PC11Type) ConfigureI2C3_SDA()  { p.Pin.configureAltFunc(8) }

// PC12 alternate functions
func (p PC12Type) ConfigureTIM8_CH3N() { p.Pin.configureAltFunc(4) }
func (p PC12Type) ConfigureSPI3_MOSI() { p.Pin.configureAltFunc(6) }
func (p PC12Type) ConfigureI2S3_SD()   { p.Pin.configureAltFunc(6) }
func (p PC12Type) ConfigureUSART3_CK() { p.Pin.configureAltFunc(7) }

// PC13 alternate functions
func (p PC13Type) ConfigureTIM1_BKIN() { p.Pin.configureAltFunc(2) }
func (p PC13Type) ConfigureTIM1_CH1N() { p.Pin.configureAltFunc(4) }
func (p PC13Type) ConfigureTIM8_CH4N() { p.Pin.configureAltFunc(6) }

// PD0 alternate functions
func (p PD0Type) ConfigureTIM8_CH4N() { p.Pin.configureAltFunc(6) }
func (p PD0Type) ConfigureFDCAN1_RX() { p.Pin.configureAltFunc(9) }

// PD1 alternate functions
func (p PD1Type) ConfigureTIM8_CH4()   { p.Pin.configureAltFunc(4) }
func (p PD1Type) ConfigureTIM8_BKIN2() { p.Pin.configureAltFunc(6) }
func (p PD1Type) ConfigureFDCAN1_TX()  { p.Pin.configureAltFunc(9) }

// PD2 alternate functions
func (p PD2Type) ConfigureTIM3_ETR()  { p.Pin.configureAltFunc(2) }
func (p PD2Type) ConfigureTIM8_BKIN() { p.Pin.configureAltFunc(4) }

// PD3 alternate functions
func (p PD3Type) ConfigureTIM2_CH1()   { p.Pin.configureAltFunc(2) }
func (p PD3Type) ConfigureTIM2_ETR()   { p.Pin.configureAltFunc(2) }
func (p PD3Type) ConfigureUSART2_CTS() { p.Pin.configureAltFunc(7) }

// PD4 alternate functions
func (p PD4Type) ConfigureTIM2_CH2()      { p.Pin.configureAltFunc(2) }
func (p PD4Type) ConfigureUSART2_RTS_DE() { p.Pin.configureAltFunc(7) }

// PD5 alternate functions
func (p PD5Type) ConfigureUSART2_TX() { p.Pin.configureAltFunc(7) }

// PD6 alternate functions
func (p PD6Type) ConfigureTIM2_CH4()  { p.Pin.configureAltFunc(2) }
func (p PD6Type) ConfigureSAI1_D1()   { p.Pin.configureAltFunc(3) }
func (p PD6Type) ConfigureUSART2_RX() { p.Pin.configureAltFunc(7) }

// PD7 alternate functions
func (p PD7Type) ConfigureTIM2_CH3()  { p.Pin.configureAltFunc(2) }
func (p PD7Type) ConfigureUSART2_CK() { p.Pin.configureAltFunc(7) }

// PD8 alternate functions
func (p PD8Type) ConfigureUSART3_TX() { p.Pin.configureAltFunc(7) }

// PD9 alternate functions
func (p PD9Type) ConfigureUSART3_RX() { p.Pin.configureAltFunc(7) }

// PD10 alternate functions
func (p PD10Type) ConfigureUSART3_CK() { p.Pin.configureAltFunc(7) }

// PD11 alternate functions
func (p PD11Type) ConfigureUSART3_CTS() { p.Pin.configureAltFunc(7) }

// PD12 alternate functions
func (p PD12Type) ConfigureTIM4_CH1()      { p.Pin.configureAltFunc(2) }
func (p PD12Type) ConfigureUSART3_RTS_DE() { p.Pin.configureAltFunc(7) }

// PD13 alternate functions
func (p PD13Type) ConfigureTIM4_CH2() { p.Pin.configureAltFunc(2) }

// PD14 alternate functions
func (p PD14Type) ConfigureTIM4_CH3() { p.Pin.configureAltFunc(2) }

// PD15 alternate functions
func (p PD15Type) ConfigureTIM4_CH4() { p.Pin.configureAltFunc(2) }
func (p PD15Type) ConfigureSPI2_NSS() { p.Pin.configureAltFunc(6) }

// PE0 alternate functions
func (p PE0Type) ConfigureTIM4_ETR()  { p.Pin.configureAltFunc(2) }
func (p PE0Type) ConfigureTIM16_CH1() { p.Pin.configureAltFunc(4) }
func (p PE0Type) ConfigureUSART1_TX() { p.Pin.configureAltFunc(7) }

// PE1 alternate functions
func (p PE1Type) ConfigureTIM17_CH1() { p.Pin.configureAltFunc(4) }
func (p PE1Type) ConfigureUSART1_RX() { p.Pin.configureAltFunc(7) }

// PE2 alternate functions
func (p PE2Type) ConfigureTRACECK()  { p.Pin.configureAltFunc(0) }
func (p PE2Type) ConfigureTIM3_CH1() { p.Pin.configureAltFunc(2) }
func (p PE2Type) ConfigureSAI1_CK1() { p.Pin.configureAltFunc(3) }

// PE3 alternate functions
func (p PE3Type) ConfigureTRACED0()  { p.Pin.configureAltFunc(0) }
func (p PE3Type) ConfigureTIM3_CH2() { p.Pin.configureAltFunc(2) }

// PE4 alternate functions
func (p PE4Type) ConfigureTRACED1()  { p.Pin.configureAltFunc(0) }
func (p PE4Type) ConfigureTIM3_CH3() { p.Pin.configureAltFunc(2) }
func (p PE4Type) ConfigureSAI1_D2()  { p.Pin.configureAltFunc(3) }

// PE5 alternate functions
func (p PE5Type) ConfigureTRACED2()  { p.Pin.configureAltFunc(0) }
func (p PE5Type) ConfigureTIM3_CH4() { p.Pin.configureAltFunc(2) }
func (p PE5Type) ConfigureSAI1_CK2() { p.Pin.configureAltFunc(3) }

// PE6 alternate functions
func (p PE6Type) ConfigureTRACED3() { p.Pin.configureAltFunc(0) }
func (p PE6Type) ConfigureSAI1_D1() { p.Pin.configureAltFunc(3) }

// PE7 alternate functions
func (p PE7Type) ConfigureTIM1_ETR() { p.Pin.configureAltFunc(2) }

// PE8 alternate functions
func (p PE8Type) ConfigureTIM1_CH1N() { p.Pin.configureAltFunc(2) }

// PE9 alternate functions
func (p PE9Type) ConfigureTIM1_CH1() { p.Pin.configureAltFunc(2) }

// PE10 alternate functions
func (p PE10Type) ConfigureTIM1_CH2N() { p.Pin.configureAltFunc(2) }

// PE11 alternate functions
func (p PE11Type) ConfigureTIM1_CH2() { p.Pin.configureAltFunc(2) }

// PE12 alternate functions
func (p PE12Type) ConfigureTIM1_CH3N() { p.Pin.configureAltFunc(2) }

// PE13 alternate functions
func (p PE13Type) ConfigureTIM1_CH3() { p.Pin.configureAltFunc(2) }

// PE14 alternate functions
func (p PE14Type) ConfigureTIM1_CH4()   { p.Pin.configureAltFunc(2) }
func (p PE14Type) ConfigureTIM1_BKIN2() { p.Pin.configureAltFunc(6) }

// PE15 alternate functions
func (p PE15Type) ConfigureTIM1_BKIN() { p.Pin.configureAltFunc(2) }
func (p PE15Type) ConfigureTIM1_CH4N() { p.Pin.configureAltFunc(6) }
func (p PE15Type) ConfigureUSART3_RX() { p.Pin.configureAltFunc(7) }

// PF0 alternate functions
func (p PF0Type) ConfigureI2C2_SDA()  { p.Pin.configureAltFunc(4) }
func (p PF0Type) ConfigureSPI2_NSS()  { p.Pin.configureAltFunc(5) }
func (p PF0Type) ConfigureI2S2_WS()   { p.Pin.configureAltFunc(5) }
func (p PF0Type) ConfigureTIM1_CH3N() { p.Pin.configureAltFunc(6) }

// PF1 alternate functions
func (p PF1Type) ConfigureSPI2_SCK() { p.Pin.configureAltFunc(5) }
func (p PF1Type) ConfigureI2S2_CK()  { p.Pin.configureAltFunc(5) }

// PF2 alternate functions
func (p PF2Type) ConfigureI2C2_SMBA() { p.Pin.configureAltFunc(4) }

// PF9 alternate functions
func (p PF9Type) ConfigureTIM15_CH1() { p.Pin.configureAltFunc(3) }
func (p PF9Type) ConfigureSPI2_SCK()  { p.Pin.configureAltFunc(5) }

// PF10 alternate functions
func (p PF10Type) ConfigureTIM15_CH2() { p.Pin.configureAltFunc(3) }
func (p PF10Type) ConfigureSPI2_SCK()  { p.Pin.configureAltFunc(5) }

// PG10 alternate functions
func (p PG10Type) ConfigureMCO() { p.Pin.configureAltFunc(0) }

// =============================================================================
// Runtime Lookup (for flexibility when compile-time safety isn't needed)
// =============================================================================

// AFLookup returns the AF number for a pin/function combination, or -1 if invalid
func AFLookup(pin Pin, function string) int {
	key := pinFuncKey{pin: pin, function: function}
	if af, ok := afMap[key]; ok {
		return int(af)
	}
	return -1
}

type pinFuncKey struct {
	pin      Pin
	function string
}

var afMap = map[pinFuncKey]uint8{
	{pin: 0, function: "TIM2_CH1"}:        1,
	{pin: 0, function: "USART2_CTS"}:      7,
	{pin: 0, function: "COMP1_OUT"}:       8,
	{pin: 0, function: "TIM8_BKIN"}:       9,
	{pin: 0, function: "TIM8_ETR"}:        10,
	{pin: 1, function: "RTC_REFIN"}:       0,
	{pin: 1, function: "TIM2_CH2"}:        1,
	{pin: 1, function: "USART2_RTS_DE"}:   7,
	{pin: 1, function: "TIM15_CH1N"}:      9,
	{pin: 2, function: "TIM2_CH3"}:        1,
	{pin: 2, function: "USART2_TX"}:       7,
	{pin: 2, function: "COMP2_OUT"}:       8,
	{pin: 2, function: "TIM15_CH1"}:       9,
	{pin: 3, function: "TIM2_CH4"}:        1,
	{pin: 3, function: "SAI1_CK1"}:        3,
	{pin: 3, function: "USART2_RX"}:       7,
	{pin: 3, function: "TIM15_CH2"}:       9,
	{pin: 4, function: "TIM3_CH2"}:        2,
	{pin: 4, function: "SPI1_NSS"}:        5,
	{pin: 4, function: "SPI3_NSS"}:        6,
	{pin: 4, function: "I2S3_WS"}:         6,
	{pin: 4, function: "USART2_CK"}:       7,
	{pin: 5, function: "TIM2_CH1"}:        1,
	{pin: 5, function: "TIM2_ETR"}:        2,
	{pin: 5, function: "SPI1_SCK"}:        5,
	{pin: 6, function: "TIM16_CH1"}:       1,
	{pin: 6, function: "TIM3_CH1"}:        2,
	{pin: 6, function: "TIM8_BKIN"}:       4,
	{pin: 6, function: "SPI1_MISO"}:       5,
	{pin: 6, function: "TIM1_BKIN"}:       6,
	{pin: 6, function: "COMP1_OUT"}:       8,
	{pin: 7, function: "TIM17_CH1"}:       1,
	{pin: 7, function: "TIM3_CH2"}:        2,
	{pin: 7, function: "TIM8_CH1N"}:       4,
	{pin: 7, function: "SPI1_MOSI"}:       5,
	{pin: 7, function: "TIM1_CH1N"}:       6,
	{pin: 7, function: "COMP2_OUT"}:       8,
	{pin: 8, function: "MCO"}:             0,
	{pin: 8, function: "I2C3_SCL"}:        2,
	{pin: 8, function: "I2C2_SDA"}:        4,
	{pin: 8, function: "I2S2_MCK"}:        5,
	{pin: 8, function: "TIM1_CH1"}:        6,
	{pin: 8, function: "USART1_CK"}:       7,
	{pin: 8, function: "TIM4_ETR"}:        10,
	{pin: 9, function: "I2C3_SMBA"}:       2,
	{pin: 9, function: "I2C2_SCL"}:        4,
	{pin: 9, function: "I2S3_MCK"}:        5,
	{pin: 9, function: "TIM1_CH2"}:        6,
	{pin: 9, function: "USART1_TX"}:       7,
	{pin: 9, function: "TIM15_BKIN"}:      9,
	{pin: 9, function: "TIM2_CH3"}:        10,
	{pin: 10, function: "TIM17_BKIN"}:     1,
	{pin: 10, function: "USB_CRS_SYNC"}:   3,
	{pin: 10, function: "I2C2_SMBA"}:      4,
	{pin: 10, function: "SPI2_MISO"}:      5,
	{pin: 10, function: "TIM1_CH3"}:       6,
	{pin: 10, function: "USART1_RX"}:      7,
	{pin: 10, function: "TIM2_CH4"}:       10,
	{pin: 10, function: "TIM8_BKIN"}:      11,
	{pin: 11, function: "SPI2_MOSI"}:      5,
	{pin: 11, function: "I2S2_SD"}:        5,
	{pin: 11, function: "TIM1_CH1N"}:      6,
	{pin: 11, function: "USART1_CTS"}:     7,
	{pin: 11, function: "COMP1_OUT"}:      8,
	{pin: 11, function: "FDCAN1_RX"}:      9,
	{pin: 11, function: "TIM4_CH1"}:       10,
	{pin: 11, function: "TIM1_CH4"}:       11,
	{pin: 12, function: "TIM16_CH1"}:      1,
	{pin: 12, function: "I2SCKIN"}:        5,
	{pin: 12, function: "TIM1_CH2N"}:      6,
	{pin: 12, function: "USART1_RTS_DE"}:  7,
	{pin: 12, function: "COMP2_OUT"}:      8,
	{pin: 12, function: "FDCAN1_TX"}:      9,
	{pin: 12, function: "TIM4_CH2"}:       10,
	{pin: 12, function: "TIM1_ETR"}:       11,
	{pin: 13, function: "SWDIO"}:          0,
	{pin: 13, function: "JTMS"}:           0,
	{pin: 13, function: "TIM16_CH1N"}:     1,
	{pin: 13, function: "I2C1_SCL"}:       4,
	{pin: 13, function: "IR_OUT"}:         5,
	{pin: 13, function: "USART3_CTS"}:     7,
	{pin: 13, function: "TIM4_CH3"}:       10,
	{pin: 14, function: "SWCLK"}:          0,
	{pin: 14, function: "JTCK"}:           0,
	{pin: 14, function: "LPTIM1_OUT"}:     1,
	{pin: 14, function: "I2C1_SDA"}:       4,
	{pin: 14, function: "TIM8_CH2"}:       5,
	{pin: 14, function: "TIM1_BKIN"}:      6,
	{pin: 14, function: "USART2_TX"}:      7,
	{pin: 15, function: "JTDI"}:           0,
	{pin: 15, function: "TIM2_CH1"}:       1,
	{pin: 15, function: "TIM8_CH1"}:       2,
	{pin: 15, function: "I2C1_SCL"}:       4,
	{pin: 15, function: "SPI1_NSS"}:       5,
	{pin: 15, function: "SPI3_NSS"}:       6,
	{pin: 15, function: "I2S3_WS"}:        6,
	{pin: 15, function: "USART2_RX"}:      7,
	{pin: 15, function: "UART4_RTS_DE"}:   8,
	{pin: 15, function: "TIM1_BKIN"}:      9,
	{pin: 16, function: "TIM3_CH3"}:       2,
	{pin: 16, function: "TIM8_CH2N"}:      4,
	{pin: 16, function: "TIM1_CH2N"}:      6,
	{pin: 17, function: "TIM3_CH4"}:       2,
	{pin: 17, function: "TIM8_CH3N"}:      4,
	{pin: 17, function: "TIM1_CH3N"}:      6,
	{pin: 17, function: "COMP4_OUT"}:      8,
	{pin: 18, function: "RTC_OUT2"}:       0,
	{pin: 18, function: "LPTIM1_OUT"}:     1,
	{pin: 18, function: "I2C3_SMBA"}:      4,
	{pin: 19, function: "JTDO"}:           0,
	{pin: 19, function: "TRACESWO"}:       0,
	{pin: 19, function: "TIM2_CH2"}:       1,
	{pin: 19, function: "TIM4_ETR"}:       2,
	{pin: 19, function: "USB_CRS_SYNC"}:   3,
	{pin: 19, function: "TIM8_CH1N"}:      4,
	{pin: 19, function: "SPI1_SCK"}:       5,
	{pin: 19, function: "SPI3_SCK"}:       6,
	{pin: 19, function: "I2S3_CK"}:        6,
	{pin: 19, function: "USART2_TX"}:      7,
	{pin: 19, function: "TIM3_ETR"}:       10,
	{pin: 20, function: "JTRST"}:          0,
	{pin: 20, function: "TIM16_CH1"}:      1,
	{pin: 20, function: "TIM3_CH1"}:       2,
	{pin: 20, function: "TIM8_CH2N"}:      4,
	{pin: 20, function: "SPI1_MISO"}:      5,
	{pin: 20, function: "SPI3_MISO"}:      6,
	{pin: 20, function: "USART2_RX"}:      7,
	{pin: 20, function: "TIM17_BKIN"}:     10,
	{pin: 21, function: "TIM16_BKIN"}:     1,
	{pin: 21, function: "TIM3_CH2"}:       2,
	{pin: 21, function: "TIM8_CH3N"}:      3,
	{pin: 21, function: "I2C1_SMBA"}:      4,
	{pin: 21, function: "SPI1_MOSI"}:      5,
	{pin: 21, function: "SPI3_MOSI"}:      6,
	{pin: 21, function: "I2S3_SD"}:        6,
	{pin: 21, function: "USART2_CK"}:      7,
	{pin: 21, function: "I2C3_SDA"}:       8,
	{pin: 21, function: "TIM17_CH1"}:      10,
	{pin: 21, function: "LPTIM1_IN1"}:     11,
	{pin: 22, function: "TIM16_CH1N"}:     1,
	{pin: 22, function: "TIM4_CH1"}:       2,
	{pin: 22, function: "TIM8_CH1"}:       5,
	{pin: 22, function: "TIM8_ETR"}:       6,
	{pin: 22, function: "USART1_TX"}:      7,
	{pin: 22, function: "COMP4_OUT"}:      8,
	{pin: 22, function: "TIM8_BKIN2"}:     10,
	{pin: 22, function: "LPTIM1_ETR"}:     11,
	{pin: 23, function: "TIM17_CH1N"}:     1,
	{pin: 23, function: "TIM4_CH2"}:       2,
	{pin: 23, function: "I2C1_SDA"}:       4,
	{pin: 23, function: "TIM8_BKIN"}:      5,
	{pin: 23, function: "USART1_RX"}:      7,
	{pin: 23, function: "COMP3_OUT"}:      8,
	{pin: 23, function: "TIM3_CH4"}:       10,
	{pin: 23, function: "LPTIM1_IN2"}:     11,
	{pin: 24, function: "TIM16_CH1"}:      1,
	{pin: 24, function: "TIM4_CH3"}:       2,
	{pin: 24, function: "SAI1_CK1"}:       3,
	{pin: 24, function: "I2C1_SCL"}:       4,
	{pin: 24, function: "USART3_RX"}:      7,
	{pin: 24, function: "COMP1_OUT"}:      8,
	{pin: 24, function: "FDCAN1_RX"}:      9,
	{pin: 24, function: "TIM8_CH2"}:       10,
	{pin: 25, function: "TIM17_CH1"}:      1,
	{pin: 25, function: "TIM4_CH4"}:       2,
	{pin: 25, function: "SAI1_D2"}:        3,
	{pin: 25, function: "I2C1_SDA"}:       4,
	{pin: 25, function: "IR_OUT"}:         6,
	{pin: 25, function: "USART3_TX"}:      7,
	{pin: 25, function: "COMP2_OUT"}:      8,
	{pin: 25, function: "FDCAN1_TX"}:      9,
	{pin: 25, function: "TIM8_CH3"}:       10,
	{pin: 26, function: "TIM2_CH3"}:       1,
	{pin: 26, function: "USART3_TX"}:      7,
	{pin: 26, function: "LPUART1_RX"}:     8,
	{pin: 27, function: "TIM2_CH4"}:       1,
	{pin: 27, function: "USART3_RX"}:      7,
	{pin: 27, function: "LPUART1_TX"}:     8,
	{pin: 28, function: "I2C2_SMBA"}:      4,
	{pin: 28, function: "SPI2_NSS"}:       5,
	{pin: 28, function: "I2S2_WS"}:        5,
	{pin: 28, function: "TIM1_BKIN"}:      6,
	{pin: 28, function: "USART3_CK"}:      7,
	{pin: 28, function: "LPUART1_RTS_DE"}: 8,
	{pin: 29, function: "SPI2_SCK"}:       5,
	{pin: 29, function: "I2S2_CK"}:        5,
	{pin: 29, function: "TIM1_CH1N"}:      6,
	{pin: 29, function: "USART3_CTS"}:     7,
	{pin: 29, function: "LPUART1_CTS"}:    8,
	{pin: 30, function: "TIM15_CH1"}:      1,
	{pin: 30, function: "SPI2_MISO"}:      5,
	{pin: 30, function: "TIM1_CH2N"}:      6,
	{pin: 30, function: "USART3_RTS_DE"}:  7,
	{pin: 30, function: "COMP4_OUT"}:      8,
	{pin: 31, function: "RTC_REFIN"}:      0,
	{pin: 31, function: "TIM15_CH2"}:      1,
	{pin: 31, function: "TIM15_CH1N"}:     2,
	{pin: 31, function: "COMP3_OUT"}:      3,
	{pin: 31, function: "TIM1_CH3N"}:      4,
	{pin: 31, function: "SPI2_MOSI"}:      5,
	{pin: 31, function: "I2S2_SD"}:        5,
	{pin: 32, function: "LPTIM1_IN1"}:     1,
	{pin: 32, function: "TIM1_CH1"}:       2,
	{pin: 32, function: "LPUART1_RX"}:     8,
	{pin: 33, function: "LPTIM1_OUT"}:     1,
	{pin: 33, function: "TIM1_CH2"}:       2,
	{pin: 33, function: "LPUART1_TX"}:     8,
	{pin: 34, function: "LPTIM1_IN2"}:     1,
	{pin: 34, function: "TIM1_CH3"}:       2,
	{pin: 34, function: "COMP3_OUT"}:      3,
	{pin: 35, function: "LPTIM1_ETR"}:     1,
	{pin: 35, function: "TIM1_CH4"}:       2,
	{pin: 35, function: "SAI1_D1"}:        3,
	{pin: 35, function: "TIM1_BKIN2"}:     6,
	{pin: 36, function: "TIM1_ETR"}:       2,
	{pin: 36, function: "I2C2_SCL"}:       4,
	{pin: 36, function: "USART1_TX"}:      7,
	{pin: 37, function: "TIM15_BKIN"}:     2,
	{pin: 37, function: "SAI1_D3"}:        3,
	{pin: 37, function: "TIM1_CH4N"}:      6,
	{pin: 37, function: "USART1_RX"}:      7,
	{pin: 38, function: "TIM3_CH1"}:       2,
	{pin: 38, function: "TIM8_CH1"}:       4,
	{pin: 38, function: "I2S2_MCK"}:       6,
	{pin: 39, function: "TIM3_CH2"}:       2,
	{pin: 39, function: "TIM8_CH2"}:       4,
	{pin: 39, function: "I2S3_MCK"}:       6,
	{pin: 40, function: "TIM3_CH3"}:       2,
	{pin: 40, function: "TIM8_CH3"}:       4,
	{pin: 40, function: "I2C3_SCL"}:       8,
	{pin: 41, function: "TIM3_CH4"}:       2,
	{pin: 41, function: "TIM8_CH4"}:       4,
	{pin: 41, function: "I2SCKIN"}:        5,
	{pin: 41, function: "TIM8_BKIN2"}:     6,
	{pin: 41, function: "I2C3_SDA"}:       8,
	{pin: 42, function: "TIM8_CH1N"}:      4,
	{pin: 42, function: "UART4_TX"}:       5,
	{pin: 42, function: "SPI3_SCK"}:       6,
	{pin: 42, function: "I2S3_CK"}:        6,
	{pin: 42, function: "USART3_TX"}:      7,
	{pin: 43, function: "TIM8_CH2N"}:      4,
	{pin: 43, function: "UART4_RX"}:       5,
	{pin: 43, function: "SPI3_MISO"}:      6,
	{pin: 43, function: "USART3_RX"}:      7,
	{pin: 43, function: "I2C3_SDA"}:       8,
	{pin: 44, function: "TIM8_CH3N"}:      4,
	{pin: 44, function: "SPI3_MOSI"}:      6,
	{pin: 44, function: "I2S3_SD"}:        6,
	{pin: 44, function: "USART3_CK"}:      7,
	{pin: 45, function: "TIM1_BKIN"}:      2,
	{pin: 45, function: "TIM1_CH1N"}:      4,
	{pin: 45, function: "TIM8_CH4N"}:      6,
	{pin: 48, function: "TIM8_CH4N"}:      6,
	{pin: 48, function: "FDCAN1_RX"}:      9,
	{pin: 49, function: "TIM8_CH4"}:       4,
	{pin: 49, function: "TIM8_BKIN2"}:     6,
	{pin: 49, function: "FDCAN1_TX"}:      9,
	{pin: 50, function: "TIM3_ETR"}:       2,
	{pin: 50, function: "TIM8_BKIN"}:      4,
	{pin: 51, function: "TIM2_CH1"}:       2,
	{pin: 51, function: "TIM2_ETR"}:       2,
	{pin: 51, function: "USART2_CTS"}:     7,
	{pin: 52, function: "TIM2_CH2"}:       2,
	{pin: 52, function: "USART2_RTS_DE"}:  7,
	{pin: 53, function: "USART2_TX"}:      7,
	{pin: 54, function: "TIM2_CH4"}:       2,
	{pin: 54, function: "SAI1_D1"}:        3,
	{pin: 54, function: "USART2_RX"}:      7,
	{pin: 55, function: "TIM2_CH3"}:       2,
	{pin: 55, function: "USART2_CK"}:      7,
	{pin: 56, function: "USART3_TX"}:      7,
	{pin: 57, function: "USART3_RX"}:      7,
	{pin: 58, function: "USART3_CK"}:      7,
	{pin: 59, function: "USART3_CTS"}:     7,
	{pin: 60, function: "TIM4_CH1"}:       2,
	{pin: 60, function: "USART3_RTS_DE"}:  7,
	{pin: 61, function: "TIM4_CH2"}:       2,
	{pin: 62, function: "TIM4_CH3"}:       2,
	{pin: 63, function: "TIM4_CH4"}:       2,
	{pin: 63, function: "SPI2_NSS"}:       6,
	{pin: 64, function: "TIM4_ETR"}:       2,
	{pin: 64, function: "TIM16_CH1"}:      4,
	{pin: 64, function: "USART1_TX"}:      7,
	{pin: 65, function: "TIM17_CH1"}:      4,
	{pin: 65, function: "USART1_RX"}:      7,
	{pin: 66, function: "TRACECK"}:        0,
	{pin: 66, function: "TIM3_CH1"}:       2,
	{pin: 66, function: "SAI1_CK1"}:       3,
	{pin: 67, function: "TRACED0"}:        0,
	{pin: 67, function: "TIM3_CH2"}:       2,
	{pin: 68, function: "TRACED1"}:        0,
	{pin: 68, function: "TIM3_CH3"}:       2,
	{pin: 68, function: "SAI1_D2"}:        3,
	{pin: 69, function: "TRACED2"}:        0,
	{pin: 69, function: "TIM3_CH4"}:       2,
	{pin: 69, function: "SAI1_CK2"}:       3,
	{pin: 70, function: "TRACED3"}:        0,
	{pin: 70, function: "SAI1_D1"}:        3,
	{pin: 71, function: "TIM1_ETR"}:       2,
	{pin: 72, function: "TIM1_CH1N"}:      2,
	{pin: 73, function: "TIM1_CH1"}:       2,
	{pin: 74, function: "TIM1_CH2N"}:      2,
	{pin: 75, function: "TIM1_CH2"}:       2,
	{pin: 76, function: "TIM1_CH3N"}:      2,
	{pin: 77, function: "TIM1_CH3"}:       2,
	{pin: 78, function: "TIM1_CH4"}:       2,
	{pin: 78, function: "TIM1_BKIN2"}:     6,
	{pin: 79, function: "TIM1_BKIN"}:      2,
	{pin: 79, function: "TIM1_CH4N"}:      6,
	{pin: 79, function: "USART3_RX"}:      7,
	{pin: 80, function: "I2C2_SDA"}:       4,
	{pin: 80, function: "SPI2_NSS"}:       5,
	{pin: 80, function: "I2S2_WS"}:        5,
	{pin: 80, function: "TIM1_CH3N"}:      6,
	{pin: 81, function: "SPI2_SCK"}:       5,
	{pin: 81, function: "I2S2_CK"}:        5,
	{pin: 82, function: "I2C2_SMBA"}:      4,
	{pin: 89, function: "TIM15_CH1"}:      3,
	{pin: 89, function: "SPI2_SCK"}:       5,
	{pin: 90, function: "TIM15_CH2"}:      3,
	{pin: 90, function: "SPI2_SCK"}:       5,
	{pin: 106, function: "MCO"}:           0,
}
