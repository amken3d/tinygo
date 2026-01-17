//go:build stm32g4

package machine

import (
	"device/stm32"
	"runtime/volatile"
	"unsafe"
)

// ADC Common Control Register (CCR) - located at ADC1 base + 0x300
// This register controls clock mode for ADC1 and ADC2
const (
	adcCommonCCRAddr = 0x50000308 // ADC12_COMMON->CCR address

	// CKMODE bits (16:17) - ADC clock mode selection
	adcBasicCKMODE_Pos  = 16
	adcBasicCKMODE_Msk  = 0x3 << adcBasicCKMODE_Pos
	adcBasicCKMODE_DIV4 = 0x3 << adcBasicCKMODE_Pos // Synchronous clock mode (HCLK/4)
)

// InitADC initializes the registers needed for ADC1.
func InitADC() {
	// Enable ADC clock (ADC12 on AHB2)
	//stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_ADC12EN)
	stm32.RCC.SetAHB2ENR_ADC12EN(1)

	// Configure ADC clock source in ADC Common CCR register
	// Use synchronous clock mode (HCLK/4) which bypasses the need
	// for configuring RCC_CCIPR.ADC12SEL
	// At 170MHz HCLK, HCLK/4 = 42.5MHz ADC clock (max is 60MHz per RM0440)
	ccr := (*volatile.Register32)(unsafe.Pointer(uintptr(adcCommonCCRAddr)))
	ccrVal := ccr.Get()
	ccrVal &^= adcBasicCKMODE_Msk // Clear CKMODE bits
	ccrVal |= adcBasicCKMODE_DIV4 // Set HCLK/4
	ccr.Set(ccrVal)

	initADC1()
	initADC2()
	// Exit deep power-down mode and enable voltage regulator
	stm32.ADC1.CR.ClearBits(stm32.ADC_CR_DEEPPWD)
	stm32.ADC1.CR.SetBits(stm32.ADC_CR_ADVREGEN)

	// Wait for voltage regulator startup (20us typical at 170MHz ~= 3400 cycles)
	for i := 0; i < 10000; i++ {
		// Simple delay loop
	}
}
func initADC1() {
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
func initADC2() {
	// Exit deep power-down mode
	//stm32.ADC2.CR.ClearBits(stm32.ADC_CR_DEEPPWD)
	stm32.ADC2.SetCR_DEEPPWD(0)

	// Enable voltage regulator
	//stm32.ADC2.CR.SetBits(stm32.ADC_CR_ADVREGEN)
	stm32.ADC2.SetCR_ADVREGEN(1)

	// Wait for regulator startup (~20us)
	for i := 0; i < 50000; i++ {
	}
	// Ensure ADC is fully disabled before writing DIFSEL
	// Per RM0440: DIFSEL can only be written when ADCAL=0, JADSTART=0,
	// JADSTP=0, ADSTART=0, ADSTP=0, ADDIS=0 and ADEN=0
	for stm32.ADC2.CR.HasBits(stm32.ADC_CR_ADCAL | stm32.ADC_CR_JADSTART |
		stm32.ADC_CR_JADSTP | stm32.ADC_CR_ADSTART | stm32.ADC_CR_ADSTP |
		stm32.ADC_CR_ADDIS | stm32.ADC_CR_ADEN) {
		// Wait until ADC is fully disabled
	}
	// Configure single-ended mode
	stm32.ADC2.DIFSEL.Set(0)

	// Set resolution to 12 bits
	stm32.ADC2.CFGR.ClearBits(stm32.ADC_CFGR_RES_Msk)
	stm32.ADC2.CFGR.ClearBits(stm32.ADC_CFGR_CONT)

	// Calibrate (single-ended)
	stm32.ADC2.CR.ClearBits(stm32.ADC_CR_ADEN)
	stm32.ADC2.CR.ClearBits(stm32.ADC_CR_ADCALDIF)
	stm32.ADC2.CR.SetBits(stm32.ADC_CR_ADCAL)
	for stm32.ADC2.CR.HasBits(stm32.ADC_CR_ADCAL) {
	}

	// Enable ADC2
	stm32.ADC2.CR.SetBits(stm32.ADC_CR_ADEN)
	for !stm32.ADC2.ISR.HasBits(stm32.ADC_ISR_ADRDY) {
	}
}

// adcInitialized tracks whether the ADC has been initialized
var adcInitialized bool
var adc2Initialized bool

// Configure configures an ADC pin to be able to read analog data.
func (a ADC) Configure(ADCConfig) {
	// Ensure ADC1 is initialized
	if !adcInitialized {
		InitADC()
		adcInitialized = true
	}

	// Configure pin as analog input
	a.Pin.ConfigureAltFunc(PinConfig{Mode: PinInputAnalog}, 0)

	// Check if this pin needs ADC2 and initialize it
	ch1 := getADC1Channel(a.Pin)
	ch2 := getADC2Channel(a.Pin)

	if ch1 == 0 && ch2 != 0 {
		// Pin only available on ADC2 - ensure it's initialized

		if !adc2Initialized {
			initADC2()
			adc2Initialized = true
		}
	}
}

// readADC1Channel performs a conversion on ADC1 for the specified channel
func readADC1Channel(channel uint8) uint16 {
	// Set sample time (247.5 cycles for accuracy)
	if channel <= 9 {
		pos := uint32(channel) * 3
		stm32.ADC1.SMPR1.ReplaceBits(6<<pos, 0x7<<pos, 0)
	} else {
		pos := uint32(channel-10) * 3
		stm32.ADC1.SMPR2.ReplaceBits(6<<pos, 0x7<<pos, 0)
	}

	// Configure regular sequence - single conversion
	stm32.ADC1.SQR1.Set(uint32(channel) << stm32.ADC_SQR1_SQ1_Pos)

	// Start conversion
	stm32.ADC1.CR.SetBits(stm32.ADC_CR_ADSTART)

	// Wait for conversion to complete
	for !stm32.ADC1.ISR.HasBits(stm32.ADC_ISR_EOC) {
	}

	// Read 12-bit result and scale to 16-bit
	return uint16(stm32.ADC1.DR.Get()) << 4
}

// readADC2Channel performs a conversion on ADC2 for the specified channel
func readADC2Channel(channel uint8) uint16 {
	// Ensure ADC2 is initialized
	if !adc2Initialized {
		initADC2()
		adc2Initialized = true
	}

	// Set sample time (247.5 cycles for accuracy)
	if channel <= 9 {
		pos := uint32(channel) * 3
		stm32.ADC2.SMPR1.ReplaceBits(6<<pos, 0x7<<pos, 0)
	} else {
		pos := uint32(channel-10) * 3
		stm32.ADC2.SMPR2.ReplaceBits(6<<pos, 0x7<<pos, 0)
	}

	// Configure regular sequence - single conversion
	stm32.ADC2.SQR1.Set(uint32(channel) << stm32.ADC_SQR1_SQ1_Pos)

	// Start conversion
	stm32.ADC2.CR.SetBits(stm32.ADC_CR_ADSTART)

	// Wait for conversion to complete
	for !stm32.ADC2.ISR.HasBits(stm32.ADC_ISR_EOC) {
	}

	// Read 12-bit result and scale to 16-bit
	return uint16(stm32.ADC2.DR.Get()) << 4
}

// Get returns the current value of an ADC pin in the range 0..0xffff.
// Automatically selects the correct ADC (ADC1 or ADC2) based on pin mapping.
func (a ADC) Get() uint16 {
	// Try ADC1 first
	ch1 := getADC1Channel(a.Pin)
	if ch1 != 0 {
		return readADC1Channel(ch1)
	}

	// Fall back to ADC2
	ch2 := getADC2Channel(a.Pin)
	if ch2 != 0 {
		return readADC2Channel(ch2)
	}

	// Pin not available on any ADC
	return 0
}

// getChannel returns the ADC1 channel for backward compatibility
// For pins only available on ADC2, returns 0 (caller should use getADC2Channel)
func (a ADC) getChannel() uint8 {
	return getADC1Channel(a.Pin)
}

// GetADCChannel returns the channel number and which ADC(s) support this pin
// Returns: channel number, supportsADC1, supportsADC2
func GetADCChannel(pin Pin) (channel uint8, adc1 bool, adc2 bool) {
	ch1 := getADC1Channel(pin)
	ch2 := getADC2Channel(pin)

	if ch1 != 0 && ch2 != 0 {
		// Shared channel (ADC12_INx) - channel numbers are the same
		return ch1, true, true
	} else if ch1 != 0 {
		return ch1, true, false
	} else if ch2 != 0 {
		return ch2, false, true
	}
	return 0, false, false
}

// getADC1Channel returns the ADC1 channel number for a pin, or 0 if not available on ADC1
func getADC1Channel(pin Pin) uint8 {
	switch pin {
	// ADC12 shared channels (available on both ADC1 and ADC2)
	case PA0:
		return 1 // ADC12_IN1
	case PA1:
		return 2 // ADC12_IN2
	case PC0:
		return 6 // ADC12_IN6
	case PC1:
		return 7 // ADC12_IN7
	case PC2:
		return 8 // ADC12_IN8
	case PC3:
		return 9 // ADC12_IN9
	case PB11:
		return 14 // ADC12_IN14

	// ADC1-only channels
	case PA2:
		return 3 // ADC1_IN3
	case PA3:
		return 4 // ADC1_IN4
	case PB14:
		return 5 // ADC1_IN5
	case PF0:
		return 10 // ADC1_IN10 (OSC_IN - usually not usable)
	case PB12:
		return 11 // ADC1_IN11
	case PB1:
		return 12 // ADC1_IN12
	case PB0:
		return 15 // ADC1_IN15
	}
	return 0
}

// getADC2Channel returns the ADC2 channel number for a pin, or 0 if not available on ADC2
func getADC2Channel(pin Pin) uint8 {
	switch pin {
	// ADC12 shared channels (available on both ADC1 and ADC2)
	case PA0:
		return 1 // ADC12_IN1
	case PA1:
		return 2 // ADC12_IN2
	case PC0:
		return 6 // ADC12_IN6
	case PC1:
		return 7 // ADC12_IN7
	case PC2:
		return 8 // ADC12_IN8
	case PC3:
		return 9 // ADC12_IN9
	case PB11:
		return 14 // ADC12_IN14

	// ADC2-only channels
	case PA6:
		return 3 // ADC2_IN3
	case PA7:
		return 4 // ADC2_IN4
	case PC4:
		return 5 // ADC2_IN5
	case PF1:
		return 10 // ADC2_IN10 (OSC_OUT - usually not usable)
	case PC5:
		return 11 // ADC2_IN11
	case PB2:
		return 12 // ADC2_IN12
	case PA5:
		return 13 // ADC2_IN13
	case PB15:
		return 15 // ADC2_IN15
	case PA4:
		return 17 // ADC2_IN17
	}
	return 0
}
