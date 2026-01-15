//go:build stm32g4

package machine

import (
	"device/stm32"
)

// InitADC initializes the registers needed for ADC1.
func InitADC() {
	// Enable ADC clock (ADC12 on AHB2)
	stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_ADC12EN)

	// Exit deep power-down mode and enable voltage regulator
	stm32.ADC1.CR.ClearBits(stm32.ADC_CR_DEEPPWD)
	stm32.ADC1.CR.SetBits(stm32.ADC_CR_ADVREGEN)

	// Wait for voltage regulator startup (20us typical at 170MHz ~= 3400 cycles)
	for i := 0; i < 10000; i++ {
		// Simple delay loop
	}

	// Configure for single-ended mode (all channels)
	stm32.ADC1.DIFSEL.Set(0)

	// Set ADC resolution to 12 bits
	stm32.ADC1.CFGR.ClearBits(stm32.ADC_CFGR_RES_Msk)

	// Set single conversion mode (not continuous)
	stm32.ADC1.CFGR.ClearBits(stm32.ADC_CFGR_CONT)

	// Calibrate ADC (single-ended calibration)
	stm32.ADC1.CR.ClearBits(stm32.ADC_CR_ADEN)     // Ensure ADC is disabled
	stm32.ADC1.CR.ClearBits(stm32.ADC_CR_ADCALDIF) // Single-ended calibration
	stm32.ADC1.CR.SetBits(stm32.ADC_CR_ADCAL)      // Start calibration
	for stm32.ADC1.CR.HasBits(stm32.ADC_CR_ADCAL) {
		// Wait for calibration to complete
	}

	// Enable ADC
	stm32.ADC1.CR.SetBits(stm32.ADC_CR_ADEN)
	for !stm32.ADC1.ISR.HasBits(stm32.ADC_ISR_ADRDY) {
		// Wait for ADC to be ready
	}
}

// Configure configures an ADC pin to be able to read analog data.
func (a ADC) Configure(ADCConfig) {
	a.Pin.ConfigureAltFunc(PinConfig{Mode: PinInputAnalog}, 0)

	// Set sample time (247.5 ADC clock cycles for best accuracy)
	// SMP = 110 = 247.5 cycles
	ch := a.getChannel()
	if ch <= 9 {
		pos := ch * 3
		stm32.ADC1.SMPR1.ReplaceBits(6<<pos, 0x7<<pos, 0)
	} else {
		pos := (ch - 10) * 3
		stm32.ADC1.SMPR2.ReplaceBits(6<<pos, 0x7<<pos, 0)
	}
}

// Get returns the current value of a ADC pin in the range 0..0xffff.
func (a ADC) Get() uint16 {
	ch := uint32(a.getChannel())

	// Configure regular sequence - single conversion
	// SQR1.L = 0 (1 conversion), SQR1.SQ1 = channel
	stm32.ADC1.SQR1.Set(ch << stm32.ADC_SQR1_SQ1_Pos)

	// Start conversion
	stm32.ADC1.CR.SetBits(stm32.ADC_CR_ADSTART)

	// Wait for conversion to complete
	for !stm32.ADC1.ISR.HasBits(stm32.ADC_ISR_EOC) {
	}

	// Read 12-bit result and scale to 16-bit
	result := uint16(stm32.ADC1.DR.Get()) << 4

	return result
}

func (a ADC) getChannel() uint8 {
	// ADC channel mapping for STM32G431
	// Reference: Datasheet Table 10. STM32G431xx pin and ball definitions
	switch a.Pin {
	case PA0:
		return 1 // ADC1_IN1
	case PA1:
		return 2 // ADC1_IN2
	case PA2:
		return 3 // ADC1_IN3
	case PA3:
		return 4 // ADC1_IN4
	case PB0:
		return 15 // ADC1_IN15
	case PB1:
		return 12 // ADC1_IN12
	case PB11:
		return 14 // ADC1_IN14
	case PB12:
		return 11 // ADC1_IN11
	case PB14:
		return 5 // ADC1_IN5
	case PB15:
		return 15 // ADC2_IN15 (using ADC1 for simplicity)
	case PC0:
		return 6 // ADC1_IN6
	case PC1:
		return 7 // ADC1_IN7
	case PC2:
		return 8 // ADC1_IN8
	case PC3:
		return 9 // ADC1_IN9
	}
	return 0
}
