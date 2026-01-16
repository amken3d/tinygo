//go:build stm32g4

package machine

// Advanced ADC features for STM32G4
//
// This file provides motor control-oriented ADC features:
// - Multiple ADC instances (ADC1, ADC2)
// - Injected channels for hardware-triggered conversions
// - Timer trigger sources for PWM-synchronized sampling
// - Dual ADC mode for simultaneous sampling
// - DMA support for automatic data transfer
//
// Reference: RM0440 Section 21 - Analog-to-Digital Converter

import (
	"device/stm32"
	"runtime/volatile"
	"unsafe"
)

// ADC Common register structure (for dual mode configuration)
// Located at ADC1 base + 0x300 for ADC12_Common
type adcCommonRegisters struct {
	CSR volatile.Register32 // 0x00 - Common status register
	_   [4]byte
	CCR volatile.Register32 // 0x08 - Common control register
	CDR volatile.Register32 // 0x0C - Common regular data register (dual mode)
}

// ADC Common register base addresses
var (
	adc12Common = (*adcCommonRegisters)(unsafe.Pointer(uintptr(0x50000300)))
)

// =============================================================================
// ADC Common Control Register (CCR) constants
// =============================================================================

const (
	// DUAL mode selection (bits 4:0)
	adcCCR_DUAL_Pos = 0
	adcCCR_DUAL_Msk = 0x1F

	// ADC clock mode selection (bits 17:16)
	adcCCR_CKMODE_Pos = 16
	adcCCR_CKMODE_Msk = 0x3 << adcCCR_CKMODE_Pos

	// ADC prescaler (bits 21:18)
	adcCCR_PRESC_Pos = 18
	adcCCR_PRESC_Msk = 0xF << adcCCR_PRESC_Pos

	// VREFINT enable (bit 22)
	adcCCR_VREFEN = 1 << 22

	// Temperature sensor enable (bit 23)
	adcCCR_TSEN = 1 << 23

	// VBAT enable (bit 24)
	adcCCR_VBATEN = 1 << 24

	// Dual mode data format (bits 15:14) - for MDMAx
	adcCCR_MDMA_Pos = 14
	adcCCR_MDMA_Msk = 0x3 << adcCCR_MDMA_Pos

	// Delay between 2 sampling phases (bits 11:8)
	adcCCR_DELAY_Pos = 8
	adcCCR_DELAY_Msk = 0xF << adcCCR_DELAY_Pos
)

// ADC Dual mode values
const (
	// ADCDualModeIndependent - ADC1 and ADC2 operate independently
	ADCDualModeIndependent = 0x00

	// ADCDualModeRegSimult - Regular simultaneous mode
	ADCDualModeRegSimult = 0x06

	// ADCDualModeInjSimult - Injected simultaneous mode
	ADCDualModeInjSimult = 0x05

	// ADCDualModeRegInterlv - Interleaved mode (regular channels only)
	ADCDualModeRegInterlv = 0x07

	// ADCDualModeInjSimultRegSimult - Combined injected + regular simultaneous
	ADCDualModeInjSimultRegSimult = 0x01

	// ADCDualModeRegSimultAltTrig - Regular simultaneous + alternate trigger
	ADCDualModeRegSimultAltTrig = 0x02

	// ADCDualModeInjSimultInterlv - Injected simultaneous + interleaved
	ADCDualModeInjSimultInterlv = 0x03
)

// ADC Clock mode values
const (
	// ADCClockModeAsync - Asynchronous clock from ADC clock source
	ADCClockModeAsync = 0x0 << adcCCR_CKMODE_Pos

	// ADCClockModeSync1 - Synchronous clock, AHB/1
	ADCClockModeSync1 = 0x1 << adcCCR_CKMODE_Pos

	// ADCClockModeSync2 - Synchronous clock, AHB/2
	ADCClockModeSync2 = 0x2 << adcCCR_CKMODE_Pos

	// ADCClockModeSync4 - Synchronous clock, AHB/4
	ADCClockModeSync4 = 0x3 << adcCCR_CKMODE_Pos
)

// =============================================================================
// ADC Instance type for advanced operations
// =============================================================================

// ADCPeripheral represents an ADC peripheral instance with advanced features
type ADCPeripheral struct {
	instance *stm32.ADC_Type
	number   uint8 // 1 or 2
}

// Global ADC peripheral instances
var (
	ADC1Periph = ADCPeripheral{instance: stm32.ADC1, number: 1}
	ADC2Periph = ADCPeripheral{instance: stm32.ADC2, number: 2}
)

// ADCResolution defines the ADC resolution
type ADCResolution uint8

const (
	ADCResolution12Bit ADCResolution = 0
	ADCResolution10Bit ADCResolution = 1
	ADCResolution8Bit  ADCResolution = 2
	ADCResolution6Bit  ADCResolution = 3
)

// ADCSampleTime defines the sample time in ADC clock cycles
type ADCSampleTime uint8

const (
	ADCSampleTime2_5   ADCSampleTime = 0 // 2.5 cycles
	ADCSampleTime6_5   ADCSampleTime = 1 // 6.5 cycles
	ADCSampleTime12_5  ADCSampleTime = 2 // 12.5 cycles
	ADCSampleTime24_5  ADCSampleTime = 3 // 24.5 cycles
	ADCSampleTime47_5  ADCSampleTime = 4 // 47.5 cycles
	ADCSampleTime92_5  ADCSampleTime = 5 // 92.5 cycles
	ADCSampleTime247_5 ADCSampleTime = 6 // 247.5 cycles
	ADCSampleTime640_5 ADCSampleTime = 7 // 640.5 cycles
)

// ADCAdvancedConfig holds advanced configuration options
type ADCAdvancedConfig struct {
	Resolution ADCResolution
	ClockMode  uint32 // ADCClockMode* constant
}

// =============================================================================
// ADC Peripheral Methods
// =============================================================================

// Enable powers up and enables the ADC peripheral
func (a *ADCPeripheral) Enable() error {
	// Enable ADC clock
	stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_ADC12EN)

	// Exit deep power-down mode
	a.instance.CR.ClearBits(stm32.ADC_CR_DEEPPWD)

	// Enable voltage regulator
	a.instance.CR.SetBits(stm32.ADC_CR_ADVREGEN)

	// Wait for voltage regulator startup (~20us)
	for i := 0; i < 10000; i++ {
	}

	return nil
}

// Disable turns off the ADC peripheral
func (a *ADCPeripheral) Disable() {
	// Stop any ongoing conversions
	a.instance.CR.SetBits(stm32.ADC_CR_ADSTP | stm32.ADC_CR_JADSTP)
	for a.instance.CR.HasBits(stm32.ADC_CR_ADSTP | stm32.ADC_CR_JADSTP) {
	}

	// Disable ADC
	a.instance.CR.SetBits(stm32.ADC_CR_ADDIS)
	for a.instance.CR.HasBits(stm32.ADC_CR_ADEN) {
	}
}

// Calibrate performs ADC calibration
// differential: true for differential inputs, false for single-ended
func (a *ADCPeripheral) Calibrate(differential bool) {
	// Ensure ADC is disabled
	a.instance.CR.ClearBits(stm32.ADC_CR_ADEN)

	// Select calibration mode
	if differential {
		a.instance.CR.SetBits(stm32.ADC_CR_ADCALDIF)
	} else {
		a.instance.CR.ClearBits(stm32.ADC_CR_ADCALDIF)
	}

	// Start calibration
	a.instance.CR.SetBits(stm32.ADC_CR_ADCAL)

	// Wait for calibration to complete
	for a.instance.CR.HasBits(stm32.ADC_CR_ADCAL) {
	}
}

// Start enables the ADC and waits for it to be ready
func (a *ADCPeripheral) Start() {
	a.instance.CR.SetBits(stm32.ADC_CR_ADEN)
	for !a.instance.ISR.HasBits(stm32.ADC_ISR_ADRDY) {
	}
}

// Configure sets up the ADC with advanced options
func (a *ADCPeripheral) Configure(config ADCAdvancedConfig) {
	// Set resolution
	cfgr := a.instance.CFGR.Get()
	cfgr &^= stm32.ADC_CFGR_RES_Msk
	cfgr |= uint32(config.Resolution) << stm32.ADC_CFGR_RES_Pos
	a.instance.CFGR.Set(cfgr)

	// Configure all channels as single-ended by default
	a.instance.DIFSEL.Set(0)
}

// SetSampleTime sets the sample time for a channel
func (a *ADCPeripheral) SetSampleTime(channel uint8, sampleTime ADCSampleTime) {
	if channel <= 9 {
		pos := uint32(channel) * 3
		a.instance.SMPR1.ReplaceBits(uint32(sampleTime)<<pos, 0x7<<pos, 0)
	} else {
		pos := uint32(channel-10) * 3
		a.instance.SMPR2.ReplaceBits(uint32(sampleTime)<<pos, 0x7<<pos, 0)
	}
}

// SetDifferential configures a channel pair for differential input
// channel: the positive input channel (even number)
func (a *ADCPeripheral) SetDifferential(channel uint8, differential bool) {
	if differential {
		a.instance.DIFSEL.SetBits(1 << channel)
	} else {
		a.instance.DIFSEL.ClearBits(1 << channel)
	}
}

// ReadChannel performs a single conversion on the specified channel
func (a *ADCPeripheral) ReadChannel(channel uint8) uint16 {
	// Configure single conversion
	a.instance.SQR1.Set(uint32(channel) << stm32.ADC_SQR1_SQ1_Pos)

	// Start conversion
	a.instance.CR.SetBits(stm32.ADC_CR_ADSTART)

	// Wait for completion
	for !a.instance.ISR.HasBits(stm32.ADC_ISR_EOC) {
	}

	return uint16(a.instance.DR.Get())
}

// =============================================================================
// Injected Channel Support
// =============================================================================

// ADCInjectedTrigger defines the trigger source for injected conversions
type ADCInjectedTrigger uint8

const (
	ADCInjTrigTIM1_TRGO  ADCInjectedTrigger = 0  // TIM1 TRGO
	ADCInjTrigTIM1_CC4   ADCInjectedTrigger = 1  // TIM1 CC4 event
	ADCInjTrigTIM2_TRGO  ADCInjectedTrigger = 2  // TIM2 TRGO
	ADCInjTrigTIM2_CC1   ADCInjectedTrigger = 3  // TIM2 CC1 event
	ADCInjTrigTIM3_CC4   ADCInjectedTrigger = 4  // TIM3 CC4 event
	ADCInjTrigEXTI15     ADCInjectedTrigger = 6  // EXTI line 15
	ADCInjTrigTIM1_TRGO2 ADCInjectedTrigger = 8  // TIM1 TRGO2
	ADCInjTrigTIM3_CC3   ADCInjectedTrigger = 9  // TIM3 CC3 event
	ADCInjTrigTIM3_TRGO  ADCInjectedTrigger = 10 // TIM3 TRGO
	ADCInjTrigTIM3_CC1   ADCInjectedTrigger = 11 // TIM3 CC1 event
	ADCInjTrigTIM6_TRGO  ADCInjectedTrigger = 12 // TIM6 TRGO
	ADCInjTrigTIM15_TRGO ADCInjectedTrigger = 14 // TIM15 TRGO
)

// ADCTriggerEdge defines the trigger edge
type ADCTriggerEdge uint8

const (
	ADCTriggerDisabled ADCTriggerEdge = 0
	ADCTriggerRising   ADCTriggerEdge = 1
	ADCTriggerFalling  ADCTriggerEdge = 2
	ADCTriggerBoth     ADCTriggerEdge = 3
)

// ADCInjectedConfig holds configuration for injected channels
type ADCInjectedConfig struct {
	// Channels to convert (1-4 channels)
	Channels []uint8

	// Trigger source
	Trigger ADCInjectedTrigger

	// Trigger edge
	TriggerEdge ADCTriggerEdge

	// SampleTime for all injected channels
	SampleTime ADCSampleTime
}

// ConfigureInjected configures the injected channel sequence
func (a *ADCPeripheral) ConfigureInjected(config ADCInjectedConfig) {
	// Set sample time for each channel
	for _, ch := range config.Channels {
		a.SetSampleTime(ch, config.SampleTime)
	}

	// Build JSQR register value
	// JL[1:0]: Injected sequence length (0=1 conv, 1=2 conv, etc.)
	// JEXTSEL[6:2]: External trigger selection
	// JEXTEN[8:7]: External trigger enable
	// JSQ1-4: Channel numbers

	numChannels := len(config.Channels)
	if numChannels > 4 {
		numChannels = 4
	}
	if numChannels == 0 {
		return
	}

	jsqr := uint32(numChannels-1) << stm32.ADC_JSQR_JL_Pos
	jsqr |= uint32(config.Trigger) << stm32.ADC_JSQR_JEXTSEL_Pos
	jsqr |= uint32(config.TriggerEdge) << stm32.ADC_JSQR_JEXTEN_Pos

	// Set channel numbers in sequence
	if numChannels >= 1 {
		jsqr |= uint32(config.Channels[0]) << stm32.ADC_JSQR_JSQ1_Pos
	}
	if numChannels >= 2 {
		jsqr |= uint32(config.Channels[1]) << stm32.ADC_JSQR_JSQ2_Pos
	}
	if numChannels >= 3 {
		jsqr |= uint32(config.Channels[2]) << stm32.ADC_JSQR_JSQ3_Pos
	}
	if numChannels >= 4 {
		jsqr |= uint32(config.Channels[3]) << stm32.ADC_JSQR_JSQ4_Pos
	}

	a.instance.JSQR.Set(jsqr)
}

// StartInjected starts injected conversion (software trigger)
func (a *ADCPeripheral) StartInjected() {
	a.instance.CR.SetBits(stm32.ADC_CR_JADSTART)
}

// IsInjectedComplete returns true if injected sequence is complete
func (a *ADCPeripheral) IsInjectedComplete() bool {
	return a.instance.ISR.HasBits(stm32.ADC_ISR_JEOS)
}

// ClearInjectedComplete clears the injected end-of-sequence flag
func (a *ADCPeripheral) ClearInjectedComplete() {
	a.instance.ISR.SetBits(stm32.ADC_ISR_JEOS)
}

// ReadInjected reads the injected data registers
// Returns up to 4 values depending on configured sequence length
func (a *ADCPeripheral) ReadInjected() [4]uint16 {
	return [4]uint16{
		uint16(a.instance.JDR1.Get()),
		uint16(a.instance.JDR2.Get()),
		uint16(a.instance.JDR3.Get()),
		uint16(a.instance.JDR4.Get()),
	}
}

// ReadInjectedChannel reads a specific injected data register (1-4)
func (a *ADCPeripheral) ReadInjectedChannel(index uint8) uint16 {
	switch index {
	case 1:
		return uint16(a.instance.JDR1.Get())
	case 2:
		return uint16(a.instance.JDR2.Get())
	case 3:
		return uint16(a.instance.JDR3.Get())
	case 4:
		return uint16(a.instance.JDR4.Get())
	}
	return 0
}

// EnableInjectedInterrupt enables the end-of-injected-sequence interrupt
func (a *ADCPeripheral) EnableInjectedInterrupt() {
	a.instance.IER.SetBits(stm32.ADC_IER_JEOSIE)
}

// DisableInjectedInterrupt disables the end-of-injected-sequence interrupt
func (a *ADCPeripheral) DisableInjectedInterrupt() {
	a.instance.IER.ClearBits(stm32.ADC_IER_JEOSIE)
}

// =============================================================================
// Dual ADC Mode Support
// =============================================================================

// ADCDualModeConfig holds configuration for dual ADC mode
type ADCDualModeConfig struct {
	// Mode selects the dual mode operation
	Mode uint8

	// ClockMode selects the ADC clock source
	ClockMode uint32

	// Delay between sampling phases (0-15, in ADC clock cycles)
	Delay uint8
}

// ConfigureDualMode configures ADC1+ADC2 for simultaneous operation
func ConfigureDualMode(config ADCDualModeConfig) {
	// Both ADCs must be disabled for dual mode configuration
	ADC1Periph.Disable()
	ADC2Periph.Disable()

	// Configure common control register
	ccr := uint32(config.Mode) << adcCCR_DUAL_Pos
	ccr |= config.ClockMode & adcCCR_CKMODE_Msk
	ccr |= uint32(config.Delay) << adcCCR_DELAY_Pos

	adc12Common.CCR.Set(ccr)
}

// ConfigureInjectedSimultaneous sets up both ADCs for simultaneous injected sampling
// This is the most common mode for motor control current sensing
func ConfigureInjectedSimultaneous(adc1Config, adc2Config ADCInjectedConfig) {
	// Configure dual mode for injected simultaneous
	ConfigureDualMode(ADCDualModeConfig{
		Mode:      ADCDualModeInjSimult,
		ClockMode: ADCClockModeSync2, // AHB/2 for reliable operation
		Delay:     0,
	})

	// Enable both ADCs
	ADC1Periph.Enable()
	ADC2Periph.Enable()

	// Calibrate both ADCs
	ADC1Periph.Calibrate(false)
	ADC2Periph.Calibrate(false)

	// Configure injected sequences
	ADC1Periph.ConfigureInjected(adc1Config)
	ADC2Periph.ConfigureInjected(adc2Config)

	// Start both ADCs
	ADC1Periph.Start()
	ADC2Periph.Start()
}

// ReadDualInjected reads injected results from both ADCs
// Returns ADC1 results in first array, ADC2 results in second
func ReadDualInjected() (adc1 [4]uint16, adc2 [4]uint16) {
	adc1 = ADC1Periph.ReadInjected()
	adc2 = ADC2Periph.ReadInjected()
	return
}

// =============================================================================
// Offset Compensation
// =============================================================================

// SetOffset configures offset compensation for a channel
// offset: signed 12-bit value to subtract from conversion result
// channel: ADC channel number
// index: offset register index (1-4)
func (a *ADCPeripheral) SetOffset(index uint8, channel uint8, offset int16) {
	// OFRx register format:
	// OFFSET[11:0]: Offset value (signed)
	// OFFSET_CH[30:26]: Channel selection
	// SSATE[31]: Signed saturation enable

	ofr := uint32(offset&0xFFF) | (uint32(channel) << 26) | (1 << 31)

	switch index {
	case 1:
		a.instance.OFR1.Set(ofr)
	case 2:
		a.instance.OFR2.Set(ofr)
	case 3:
		a.instance.OFR3.Set(ofr)
	case 4:
		a.instance.OFR4.Set(ofr)
	}
}

// =============================================================================
// ADC Channel Mapping for STM32G431
// =============================================================================

// GetADC1Channel returns the ADC1 channel number for a pin, or 0 if not available
func GetADC1Channel(pin Pin) uint8 {
	switch pin {
	case PA0:
		return 1
	case PA1:
		return 2
	case PA2:
		return 3
	case PA3:
		return 4
	case PB14:
		return 5
	case PC0:
		return 6
	case PC1:
		return 7
	case PC2:
		return 8
	case PC3:
		return 9
	case PF0:
		return 10
	case PB12:
		return 11
	case PB1:
		return 12
	case PB11:
		return 14
	case PB0:
		return 15
	}
	return 0
}

// GetADC2Channel returns the ADC2 channel number for a pin, or 0 if not available
func GetADC2Channel(pin Pin) uint8 {
	switch pin {
	case PA0:
		return 1
	case PA1:
		return 2
	case PA6:
		return 3
	case PA7:
		return 4
	case PC4:
		return 5
	case PC0:
		return 6
	case PC1:
		return 7
	case PC2:
		return 8
	case PC3:
		return 9
	case PF1:
		return 10
	case PC5:
		return 11
	case PB2:
		return 12
	case PA5:
		return 13
	case PB11:
		return 14
	case PB15:
		return 15
	case PA4:
		return 17 // OPAMP1_VOUT internal
	}
	return 0
}

// Internal ADC channels
const (
	ADC1_VOPAMP1 = 13 // OPAMP1 output (internal)
	ADC1_VTEMP   = 16 // Temperature sensor
	ADC1_VBAT    = 17 // VBAT/3
	ADC1_VREFINT = 18 // Internal voltage reference

	ADC2_OPAMP2    = 16 // OPAMP2 output (internal)
	ADC2_OPAMP3_VM = 18 // OPAMP3 inverting input (internal)
)

// =============================================================================
// DMA Support for ADC
// =============================================================================

// DMA request IDs for DMAMUX (from RM0440 Table 91)
const (
	dmamuxReqADC1 = 5  // ADC1 DMA request
	dmamuxReqADC2 = 36 // ADC2 DMA request
)

// DMA channel registers structure (for direct access)
type dmaChannel struct {
	CCR   *volatile.Register32
	CNDTR *volatile.Register32
	CPAR  *volatile.Register32
	CMAR  *volatile.Register32
}

// getDMAChannel returns a dmaChannel struct for the specified channel (1-8)
func getDMAChannel(dma *stm32.DMA_Type, ch uint8) dmaChannel {
	baseAddr := uintptr(unsafe.Pointer(dma))
	// Each channel has 4 registers (CCR, CNDTR, CPAR, CMAR) = 16 bytes + 4 byte gap = 20 bytes
	// Channel 1 starts at offset 0x08
	offset := uintptr(0x08 + (int(ch)-1)*0x14)

	return dmaChannel{
		CCR:   (*volatile.Register32)(unsafe.Pointer(baseAddr + offset)),
		CNDTR: (*volatile.Register32)(unsafe.Pointer(baseAddr + offset + 0x04)),
		CPAR:  (*volatile.Register32)(unsafe.Pointer(baseAddr + offset + 0x08)),
		CMAR:  (*volatile.Register32)(unsafe.Pointer(baseAddr + offset + 0x0C)),
	}
}

// getDMAMUXChannel returns a pointer to the DMAMUX channel control register
func getDMAMUXChannel(ch uint8) *volatile.Register32 {
	baseAddr := uintptr(unsafe.Pointer(stm32.DMAMUX))
	// Each channel is 4 bytes apart
	offset := uintptr(int(ch) * 4)
	return (*volatile.Register32)(unsafe.Pointer(baseAddr + offset))
}

// DMA CCR register bits
const (
	dmaCCR_EN       = 1 << 0  // Channel enable
	dmaCCR_TCIE     = 1 << 1  // Transfer complete interrupt enable
	dmaCCR_HTIE     = 1 << 2  // Half transfer interrupt enable
	dmaCCR_TEIE     = 1 << 3  // Transfer error interrupt enable
	dmaCCR_DIR      = 1 << 4  // Data transfer direction (0=periph to mem)
	dmaCCR_CIRC     = 1 << 5  // Circular mode
	dmaCCR_PINC     = 1 << 6  // Peripheral increment mode
	dmaCCR_MINC     = 1 << 7  // Memory increment mode
	dmaCCR_PSIZE_8  = 0 << 8  // Peripheral size 8-bit
	dmaCCR_PSIZE_16 = 1 << 8  // Peripheral size 16-bit
	dmaCCR_PSIZE_32 = 2 << 8  // Peripheral size 32-bit
	dmaCCR_MSIZE_8  = 0 << 10 // Memory size 8-bit
	dmaCCR_MSIZE_16 = 1 << 10 // Memory size 16-bit
	dmaCCR_MSIZE_32 = 2 << 10 // Memory size 32-bit
	dmaCCR_PL_Low   = 0 << 12 // Priority low
	dmaCCR_PL_Med   = 1 << 12 // Priority medium
	dmaCCR_PL_High  = 2 << 12 // Priority high
	dmaCCR_PL_VHigh = 3 << 12 // Priority very high
)

// ADCDMAConfig holds configuration for ADC DMA transfers
type ADCDMAConfig struct {
	// DMAChannel: DMA channel number (1-8 for DMA1)
	DMAChannel uint8

	// Buffer: pointer to destination buffer
	Buffer *uint16

	// BufferLen: number of samples in buffer
	BufferLen uint16

	// Circular: enable circular buffer mode
	Circular bool

	// Priority: DMA priority (dmaCCR_PL_*)
	Priority uint32
}

// ConfigureDMA sets up DMA for regular channel conversions
// This transfers ADC results directly to memory without CPU intervention
func (a *ADCPeripheral) ConfigureDMA(config ADCDMAConfig) {
	// Enable DMA1 clock
	stm32.RCC.AHB1ENR.SetBits(stm32.RCC_AHB1ENR_DMA1EN)
	stm32.RCC.AHB1ENR.SetBits(stm32.RCC_AHB1ENR_DMAMUXEN)

	// Get DMA channel registers
	dma := getDMAChannel(stm32.DMA1, config.DMAChannel)

	// Disable channel before configuration
	dma.CCR.ClearBits(dmaCCR_EN)

	// Configure DMAMUX to route ADC request to this DMA channel
	// DMAMUX channel index = DMA channel - 1 (0-based)
	dmamux := getDMAMUXChannel(config.DMAChannel - 1)
	if a.number == 1 {
		dmamux.Set(dmamuxReqADC1)
	} else {
		dmamux.Set(dmamuxReqADC2)
	}

	// Set peripheral address (ADC data register)
	dma.CPAR.Set(uint32(uintptr(unsafe.Pointer(&a.instance.DR))))

	// Set memory address
	dma.CMAR.Set(uint32(uintptr(unsafe.Pointer(config.Buffer))))

	// Set number of data items
	dma.CNDTR.Set(uint32(config.BufferLen))

	// Configure channel control register
	ccr := uint32(dmaCCR_MINC)      // Memory increment
	ccr |= dmaCCR_PSIZE_16          // Peripheral size 16-bit
	ccr |= dmaCCR_MSIZE_16          // Memory size 16-bit
	ccr |= config.Priority & 0x3000 // Priority

	if config.Circular {
		ccr |= dmaCCR_CIRC
	}

	dma.CCR.Set(ccr)

	// Enable DMA in ADC
	a.instance.CFGR.SetBits(stm32.ADC_CFGR_DMAEN)

	if config.Circular {
		a.instance.CFGR.SetBits(stm32.ADC_CFGR_DMACFG) // Circular DMA mode
	}
}

// EnableDMA enables DMA transfers for this ADC
func (a *ADCPeripheral) EnableDMA(dmaChannel uint8) {
	dma := getDMAChannel(stm32.DMA1, dmaChannel)
	dma.CCR.SetBits(dmaCCR_EN)
}

// DisableDMA disables DMA transfers for this ADC
func (a *ADCPeripheral) DisableDMA(dmaChannel uint8) {
	dma := getDMAChannel(stm32.DMA1, dmaChannel)
	dma.CCR.ClearBits(dmaCCR_EN)
}

// IsDMAComplete returns true if DMA transfer is complete
func IsDMAComplete(dmaChannel uint8) bool {
	// Check transfer complete flag in DMA1 ISR
	// TC1 = bit 1, TC2 = bit 5, TC3 = bit 9, etc. (every 4 bits)
	tcBit := uint32(1 << ((dmaChannel-1)*4 + 1))
	return stm32.DMA1.ISR.HasBits(tcBit)
}

// ClearDMAComplete clears the DMA transfer complete flag
func ClearDMAComplete(dmaChannel uint8) {
	// Clear in IFCR register
	tcBit := uint32(1 << ((dmaChannel-1)*4 + 1))
	stm32.DMA1.IFCR.Set(tcBit)
}

// EnableDMAInterrupt enables the DMA transfer complete interrupt
func EnableDMAInterrupt(dmaChannel uint8) {
	dma := getDMAChannel(stm32.DMA1, dmaChannel)
	dma.CCR.SetBits(dmaCCR_TCIE)
}

// =============================================================================
// Regular Sequence with DMA (for continuous multi-channel sampling)
// =============================================================================

// ADCRegularSequenceConfig holds configuration for regular sequence
type ADCRegularSequenceConfig struct {
	// Channels: list of channels to convert (up to 16)
	Channels []uint8

	// SampleTime: sample time for all channels
	SampleTime ADCSampleTime

	// Continuous: run conversions continuously
	Continuous bool

	// DMA: DMA configuration (nil to disable DMA)
	DMA *ADCDMAConfig
}

// ConfigureRegularSequence sets up a multi-channel regular sequence
func (a *ADCPeripheral) ConfigureRegularSequence(config ADCRegularSequenceConfig) {
	numChannels := len(config.Channels)
	if numChannels > 16 {
		numChannels = 16
	}
	if numChannels == 0 {
		return
	}

	// Set sample time for each channel
	for _, ch := range config.Channels {
		a.SetSampleTime(ch, config.SampleTime)
	}

	// Configure sequence registers SQR1-4
	// SQR1: L[3:0] = length-1, SQ1[10:6], SQ2[16:12], SQ3[22:18], SQ4[28:24]
	// SQR2: SQ5-SQ9
	// SQR3: SQ10-SQ14
	// SQR4: SQ15-SQ16

	sqr1 := uint32(numChannels-1) << stm32.ADC_SQR1_L_Pos
	sqr2 := uint32(0)
	sqr3 := uint32(0)
	sqr4 := uint32(0)

	for i, ch := range config.Channels {
		pos := (i % 4) + 1 // SQ1, SQ2, etc.
		shift := uint32(pos * 6)

		switch {
		case i < 4:
			sqr1 |= uint32(ch) << shift
		case i < 9:
			sqr2 |= uint32(ch) << ((i - 4) * 6)
		case i < 14:
			sqr3 |= uint32(ch) << ((i - 9) * 6)
		default:
			sqr4 |= uint32(ch) << ((i - 14) * 6)
		}
	}

	a.instance.SQR1.Set(sqr1)
	a.instance.SQR2.Set(sqr2)
	a.instance.SQR3.Set(sqr3)
	a.instance.SQR4.Set(sqr4)

	// Configure continuous mode
	if config.Continuous {
		a.instance.CFGR.SetBits(stm32.ADC_CFGR_CONT)
	} else {
		a.instance.CFGR.ClearBits(stm32.ADC_CFGR_CONT)
	}

	// Configure DMA if specified
	if config.DMA != nil {
		a.ConfigureDMA(*config.DMA)
	}
}

// StartRegular starts regular sequence conversion
func (a *ADCPeripheral) StartRegular() {
	a.instance.CR.SetBits(stm32.ADC_CR_ADSTART)
}

// StopRegular stops regular sequence conversion
func (a *ADCPeripheral) StopRegular() {
	a.instance.CR.SetBits(stm32.ADC_CR_ADSTP)
	for a.instance.CR.HasBits(stm32.ADC_CR_ADSTP) {
	}
}
