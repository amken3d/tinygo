//go:build stm32g4

package machine

// COMP (Comparator) peripheral driver for STM32G4
//
// The STM32G4 has up to 7 comparators (STM32G431 has 4: COMP1-4).
// Each comparator compares two analog inputs and produces a digital output.
//
// Features:
// - Configurable plus and minus inputs (GPIO, DAC, VREFINT with divider)
// - Programmable hysteresis (0-70mV in 10mV steps)
// - Output blanking for PWM noise immunity
// - Output polarity selection
// - Output routing to GPIO or timer inputs (BKIN, OCREF_CLR)
//
// Reference: RM0440 Section 24 - Comparators

import (
	"device/stm32"
	"runtime/volatile"
)

// COMP CSR register bit positions and masks
const (
	// EN - Comparator enable
	compCSR_EN_Pos = 0
	compCSR_EN     = 1 << compCSR_EN_Pos

	// INMSEL - Inverting input selection
	compCSR_INMSEL_Pos = 4
	compCSR_INMSEL_Msk = 0x7 << compCSR_INMSEL_Pos

	// INPSEL - Non-inverting input selection
	compCSR_INPSEL_Pos = 8
	compCSR_INPSEL     = 1 << compCSR_INPSEL_Pos

	// POL - Output polarity
	compCSR_POL_Pos = 15
	compCSR_POL     = 1 << compCSR_POL_Pos

	// HYST - Hysteresis selection
	compCSR_HYST_Pos = 16
	compCSR_HYST_Msk = 0x7 << compCSR_HYST_Pos

	// BLANKSEL - Blanking source selection
	compCSR_BLANKSEL_Pos = 19
	compCSR_BLANKSEL_Msk = 0x7 << compCSR_BLANKSEL_Pos

	// BRGEN - VREFINT resistor bridge enable
	compCSR_BRGEN_Pos = 22
	compCSR_BRGEN     = 1 << compCSR_BRGEN_Pos

	// SCALEN - VREFINT scaler enable
	compCSR_SCALEN_Pos = 23
	compCSR_SCALEN     = 1 << compCSR_SCALEN_Pos

	// VALUE - Comparator output status (read-only)
	compCSR_VALUE_Pos = 30
	compCSR_VALUE     = 1 << compCSR_VALUE_Pos

	// LOCK - Lock bit (write-once)
	compCSR_LOCK_Pos = 31
	compCSR_LOCK     = 1 << compCSR_LOCK_Pos
)

// Inverting input selection values (INMSEL)
const (
	// COMPInmSel1_4VREFINT selects 1/4 VREFINT as inverting input
	COMPInmSel1_4VREFINT = 0x0 << compCSR_INMSEL_Pos
	// COMPInmSel1_2VREFINT selects 1/2 VREFINT as inverting input
	COMPInmSel1_2VREFINT = 0x1 << compCSR_INMSEL_Pos
	// COMPInmSel3_4VREFINT selects 3/4 VREFINT as inverting input
	COMPInmSel3_4VREFINT = 0x2 << compCSR_INMSEL_Pos
	// COMPInmSelVREFINT selects VREFINT as inverting input
	COMPInmSelVREFINT = 0x3 << compCSR_INMSEL_Pos
	// COMPInmSelDAC1 selects DAC1 output as inverting input
	COMPInmSelDAC1 = 0x4 << compCSR_INMSEL_Pos
	// COMPInmSelDAC3 selects DAC3 output as inverting input (COMP1/2/3)
	COMPInmSelDAC3 = 0x5 << compCSR_INMSEL_Pos
	// COMPInmSelINM1 selects INM1 pin as inverting input
	COMPInmSelINM1 = 0x6 << compCSR_INMSEL_Pos
	// COMPInmSelINM2 selects INM2 pin as inverting input
	COMPInmSelINM2 = 0x7 << compCSR_INMSEL_Pos
)

// Non-inverting input selection values (INPSEL)
const (
	// COMPInpSelINP1 selects INP1 pin as non-inverting input
	COMPInpSelINP1 = 0 << compCSR_INPSEL_Pos
	// COMPInpSelINP2 selects INP2 pin as non-inverting input
	COMPInpSelINP2 = 1 << compCSR_INPSEL_Pos
)

// Hysteresis selection values
const (
	// COMPHystNone disables hysteresis (0mV)
	COMPHystNone = 0x0 << compCSR_HYST_Pos
	// COMPHyst10mV sets hysteresis to 10mV
	COMPHyst10mV = 0x1 << compCSR_HYST_Pos
	// COMPHyst20mV sets hysteresis to 20mV
	COMPHyst20mV = 0x2 << compCSR_HYST_Pos
	// COMPHyst30mV sets hysteresis to 30mV
	COMPHyst30mV = 0x3 << compCSR_HYST_Pos
	// COMPHyst40mV sets hysteresis to 40mV
	COMPHyst40mV = 0x4 << compCSR_HYST_Pos
	// COMPHyst50mV sets hysteresis to 50mV
	COMPHyst50mV = 0x5 << compCSR_HYST_Pos
	// COMPHyst60mV sets hysteresis to 60mV
	COMPHyst60mV = 0x6 << compCSR_HYST_Pos
	// COMPHyst70mV sets hysteresis to 70mV
	COMPHyst70mV = 0x7 << compCSR_HYST_Pos
)

// Blanking source selection values
const (
	// COMPBlankNone disables blanking
	COMPBlankNone = 0x0 << compCSR_BLANKSEL_Pos
	// COMPBlankTIM1OC5 uses TIM1 OC5 as blanking source
	COMPBlankTIM1OC5 = 0x1 << compCSR_BLANKSEL_Pos
	// COMPBlankTIM2OC3 uses TIM2 OC3 as blanking source
	COMPBlankTIM2OC3 = 0x2 << compCSR_BLANKSEL_Pos
	// COMPBlankTIM3OC3 uses TIM3 OC3 as blanking source
	COMPBlankTIM3OC3 = 0x3 << compCSR_BLANKSEL_Pos
	// COMPBlankTIM8OC5 uses TIM8 OC5 as blanking source
	COMPBlankTIM8OC5 = 0x4 << compCSR_BLANKSEL_Pos
	// COMPBlankTIM15OC1 uses TIM15 OC1 as blanking source
	COMPBlankTIM15OC1 = 0x5 << compCSR_BLANKSEL_Pos
)

// COMP represents a single Comparator peripheral instance
type COMP struct {
	csr *volatile.Register32
}

// Global COMP instances
var (
	COMP1 = COMP{csr: &stm32.COMP.C1CSR}
	COMP2 = COMP{csr: &stm32.COMP.C2CSR}
	COMP3 = COMP{csr: &stm32.COMP.C3CSR}
	COMP4 = COMP{csr: &stm32.COMP.C4CSR}
)

// COMPConfig holds configuration for the Comparator
type COMPConfig struct {
	// InvertingInput selects the inverting (-) input
	// Use COMPInmSel* constants
	InvertingInput uint32

	// NonInvertingInput selects the non-inverting (+) input
	// Use COMPInpSelINP1 or COMPInpSelINP2
	NonInvertingInput uint32

	// Hysteresis sets the hysteresis level
	// Use COMPHyst* constants
	Hysteresis uint32

	// Blanking sets the blanking source for PWM noise immunity
	// Use COMPBlank* constants
	Blanking uint32

	// InvertOutput inverts the comparator output polarity
	InvertOutput bool

	// EnableVREFINTScaler enables the VREFINT voltage scaler
	// Required when using VREFINT-based inverting inputs
	EnableVREFINTScaler bool

	// EnableVREFINTBridge enables the VREFINT resistor divider bridge
	// Required when using fractional VREFINT (1/4, 1/2, 3/4) as input
	EnableVREFINTBridge bool
}

// Configure configures the comparator with the given settings
// The comparator is enabled after configuration
func (c *COMP) Configure(config COMPConfig) {
	// Enable SYSCFG clock (shared with OPAMP)
	enableSYSCFGClock()

	// Build CSR value
	csr := uint32(0)

	// Set inverting input
	csr |= config.InvertingInput & compCSR_INMSEL_Msk

	// Set non-inverting input
	csr |= config.NonInvertingInput & compCSR_INPSEL

	// Set hysteresis
	csr |= config.Hysteresis & compCSR_HYST_Msk

	// Set blanking source
	csr |= config.Blanking & compCSR_BLANKSEL_Msk

	// Output polarity
	if config.InvertOutput {
		csr |= compCSR_POL
	}

	// VREFINT scaler
	if config.EnableVREFINTScaler {
		csr |= compCSR_SCALEN
	}

	// VREFINT bridge (for fractional VREFINT)
	if config.EnableVREFINTBridge {
		csr |= compCSR_BRGEN
	}

	// Enable the comparator
	csr |= compCSR_EN

	c.csr.Set(csr)
}

// ConfigureSimple configures the comparator with typical settings
// Uses specified inputs with no hysteresis or blanking
func (c *COMP) ConfigureSimple(invertingInput, nonInvertingInput uint32) {
	enableSYSCFGClock()

	csr := uint32(0)
	csr |= invertingInput & compCSR_INMSEL_Msk
	csr |= nonInvertingInput & compCSR_INPSEL

	// Enable VREFINT features if using VREFINT-based input
	if invertingInput <= COMPInmSelVREFINT {
		csr |= compCSR_SCALEN
		if invertingInput < COMPInmSelVREFINT {
			csr |= compCSR_BRGEN // Need bridge for fractional VREFINT
		}
	}

	csr |= compCSR_EN

	c.csr.Set(csr)
}

// ConfigureWithThreshold configures the comparator to compare an input against
// a fraction of VREFINT (1/4, 1/2, 3/4, or full VREFINT)
func (c *COMP) ConfigureWithThreshold(input uint32, threshold uint32, hysteresis uint32) {
	enableSYSCFGClock()

	csr := uint32(0)
	csr |= threshold & compCSR_INMSEL_Msk
	csr |= input & compCSR_INPSEL
	csr |= hysteresis & compCSR_HYST_Msk

	// Enable VREFINT scaler
	csr |= compCSR_SCALEN

	// Enable bridge for fractional VREFINT
	if threshold < COMPInmSelVREFINT {
		csr |= compCSR_BRGEN
	}

	csr |= compCSR_EN

	c.csr.Set(csr)
}

// Enable enables the comparator
func (c *COMP) Enable() {
	enableSYSCFGClock()
	c.csr.SetBits(compCSR_EN)
}

// Disable disables the comparator
func (c *COMP) Disable() {
	c.csr.ClearBits(compCSR_EN)
}

// IsEnabled returns true if the comparator is enabled
func (c *COMP) IsEnabled() bool {
	return c.csr.HasBits(compCSR_EN)
}

// Output returns the current comparator output state
// Returns true if the non-inverting input is greater than the inverting input
// (or the opposite if output polarity is inverted)
func (c *COMP) Output() bool {
	return c.csr.HasBits(compCSR_VALUE)
}

// Read is an alias for Output() for consistency with other peripherals
func (c *COMP) Read() bool {
	return c.Output()
}

// SetHysteresis sets the hysteresis level
func (c *COMP) SetHysteresis(hysteresis uint32) {
	csr := c.csr.Get()
	csr &^= compCSR_HYST_Msk
	csr |= hysteresis & compCSR_HYST_Msk
	c.csr.Set(csr)
}

// SetBlanking sets the blanking source
func (c *COMP) SetBlanking(blanking uint32) {
	csr := c.csr.Get()
	csr &^= compCSR_BLANKSEL_Msk
	csr |= blanking & compCSR_BLANKSEL_Msk
	c.csr.Set(csr)
}

// SetOutputPolarity sets whether the output is inverted
func (c *COMP) SetOutputPolarity(inverted bool) {
	if inverted {
		c.csr.SetBits(compCSR_POL)
	} else {
		c.csr.ClearBits(compCSR_POL)
	}
}

// Lock locks the comparator configuration registers
// Once locked, the configuration cannot be changed until the next reset
func (c *COMP) Lock() {
	c.csr.SetBits(compCSR_LOCK)
}

// IsLocked returns true if the comparator configuration is locked
func (c *COMP) IsLocked() bool {
	return c.csr.HasBits(compCSR_LOCK)
}

// =============================================================================
// Window Comparator Support
// =============================================================================

// ConfigureWindow configures two comparators as a window comparator
// The window comparator detects when an input is between two thresholds
// compLow: comparator for lower threshold (output high when input > threshold)
// compHigh: comparator for upper threshold (output high when input > threshold)
// Returns: true when input is within window (above low, below high)
//
// Example usage:
//
//	COMP1.ConfigureWindow(&COMP2, COMPInpSelINP1, COMPInmSel1_4VREFINT, COMPInmSel3_4VREFINT)
//	inWindow := COMP1.Output() && !COMP2.Output()
func (c *COMP) ConfigureWindow(compHigh *COMP, input uint32, lowThreshold uint32, highThreshold uint32) {
	// Configure low threshold comparator
	c.ConfigureWithThreshold(input, lowThreshold, COMPHystNone)

	// Configure high threshold comparator
	compHigh.ConfigureWithThreshold(input, highThreshold, COMPHystNone)
}

// WindowOutput returns true if the input is within the window
// (above the low threshold AND below the high threshold)
// compHigh must be the upper threshold comparator
func (c *COMP) WindowOutput(compHigh *COMP) bool {
	return c.Output() && !compHigh.Output()
}
