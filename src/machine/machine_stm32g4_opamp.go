//go:build stm32g4

package machine

// OPAMP (Operational Amplifier) peripheral driver for STM32G4
//
// The STM32G4 has up to 6 OPAMPs (STM32G431 has 3: OPAMP1, OPAMP2, OPAMP3).
// Each OPAMP can operate in three modes:
// - Standalone: External feedback network, user-defined gain
// - Follower: Unity gain buffer (voltage follower)
// - PGA: Programmable Gain Amplifier with internal gain network
//
// The OPAMP can route its output internally to an ADC channel for direct
// sampling without using a GPIO pin.
//
// Reference: RM0440 Section 23 - Operational Amplifiers

import (
	"device/stm32"
	"runtime/volatile"
	"unsafe"
)

// OPAMP CSR register bit positions and masks
const (
	// OPAEN - OPAMP enable
	opampCSR_OPAEN_Pos = 0
	opampCSR_OPAEN     = 1 << opampCSR_OPAEN_Pos

	// FORCE_VP - Force internal reference on non-inverting input
	opampCSR_FORCE_VP_Pos = 1
	opampCSR_FORCE_VP     = 1 << opampCSR_FORCE_VP_Pos

	// VP_SEL - Non-inverting input selection
	opampCSR_VP_SEL_Pos = 2
	opampCSR_VP_SEL_Msk = 0x3 << opampCSR_VP_SEL_Pos

	// USERTRIM - User trimming enable
	opampCSR_USERTRIM_Pos = 4
	opampCSR_USERTRIM     = 1 << opampCSR_USERTRIM_Pos

	// VM_SEL - Inverting input selection and mode
	opampCSR_VM_SEL_Pos = 5
	opampCSR_VM_SEL_Msk = 0x3 << opampCSR_VM_SEL_Pos

	// OPAHSM - High-speed mode enable
	opampCSR_OPAHSM_Pos = 7
	opampCSR_OPAHSM     = 1 << opampCSR_OPAHSM_Pos

	// OPAINTOEN - Internal output to ADC enable
	opampCSR_OPAINTOEN_Pos = 8
	opampCSR_OPAINTOEN     = 1 << opampCSR_OPAINTOEN_Pos

	// CALON - Calibration mode enable
	opampCSR_CALON_Pos = 11
	opampCSR_CALON     = 1 << opampCSR_CALON_Pos

	// CALSEL - Calibration selection
	opampCSR_CALSEL_Pos = 12
	opampCSR_CALSEL_Msk = 0x3 << opampCSR_CALSEL_Pos

	// PGA_GAIN - PGA gain selection
	opampCSR_PGA_GAIN_Pos = 14
	opampCSR_PGA_GAIN_Msk = 0x1F << opampCSR_PGA_GAIN_Pos

	// TRIMOFFSETP - Offset trimming value (PMOS)
	opampCSR_TRIMOFFSETP_Pos = 19
	opampCSR_TRIMOFFSETP_Msk = 0x1F << opampCSR_TRIMOFFSETP_Pos

	// TRIMOFFSETN - Offset trimming value (NMOS)
	opampCSR_TRIMOFFSETN_Pos = 24
	opampCSR_TRIMOFFSETN_Msk = 0x1F << opampCSR_TRIMOFFSETN_Pos

	// CALOUT - Calibration output (read-only)
	opampCSR_CALOUT_Pos = 30
	opampCSR_CALOUT     = 1 << opampCSR_CALOUT_Pos

	// LOCK - Lock bit (write-once)
	opampCSR_LOCK_Pos = 31
	opampCSR_LOCK     = 1 << opampCSR_LOCK_Pos
)

// VM_SEL values (inverting input / mode selection)
const (
	// OPAMPVMSelVINM0 selects VINM0 as inverting input (standalone mode)
	OPAMPVMSelVINM0 = 0x0 << opampCSR_VM_SEL_Pos
	// OPAMPVMSelVINM1 selects VINM1 as inverting input (standalone mode)
	OPAMPVMSelVINM1 = 0x1 << opampCSR_VM_SEL_Pos
	// OPAMPVMSelPGA selects PGA mode (internal feedback)
	OPAMPVMSelPGA = 0x2 << opampCSR_VM_SEL_Pos
	// OPAMPVMSelFollower selects follower mode (output connected to inverting input)
	OPAMPVMSelFollower = 0x3 << opampCSR_VM_SEL_Pos
)

// VP_SEL values (non-inverting input selection)
const (
	// OPAMPVPSelVINP0 selects VINP0 as non-inverting input
	OPAMPVPSelVINP0 = 0x0 << opampCSR_VP_SEL_Pos
	// OPAMPVPSelVINP1 selects VINP1 as non-inverting input
	OPAMPVPSelVINP1 = 0x1 << opampCSR_VP_SEL_Pos
	// OPAMPVPSelVINP2 selects VINP2 as non-inverting input
	OPAMPVPSelVINP2 = 0x2 << opampCSR_VP_SEL_Pos
	// OPAMPVPSelDAC selects internal DAC output as non-inverting input
	OPAMPVPSelDAC = 0x3 << opampCSR_VP_SEL_Pos
)

// PGA gain values for non-inverting mode
const (
	// OPAMPPGAGain2 sets gain = 2
	OPAMPPGAGain2 = 0x0 << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGain4 sets gain = 4
	OPAMPPGAGain4 = 0x1 << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGain8 sets gain = 8
	OPAMPPGAGain8 = 0x2 << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGain16 sets gain = 16
	OPAMPPGAGain16 = 0x3 << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGain32 sets gain = 32
	OPAMPPGAGain32 = 0x4 << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGain64 sets gain = 64
	OPAMPPGAGain64 = 0x5 << opampCSR_PGA_GAIN_Pos
)

// PGA gain values for inverting mode (VINM0 connected to external signal)
const (
	// OPAMPPGAGainInvMinus1 sets gain = -1 (with VINM0 input)
	OPAMPPGAGainInvMinus1 = 0x8 << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGainInvMinus3 sets gain = -3 (with VINM0 input)
	OPAMPPGAGainInvMinus3 = 0x9 << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGainInvMinus7 sets gain = -7 (with VINM0 input)
	OPAMPPGAGainInvMinus7 = 0xA << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGainInvMinus15 sets gain = -15 (with VINM0 input)
	OPAMPPGAGainInvMinus15 = 0xB << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGainInvMinus31 sets gain = -31 (with VINM0 input)
	OPAMPPGAGainInvMinus31 = 0xC << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGainInvMinus63 sets gain = -63 (with VINM0 input)
	OPAMPPGAGainInvMinus63 = 0xD << opampCSR_PGA_GAIN_Pos
)

// PGA gain values for inverting mode with VINM1 connected
const (
	// OPAMPPGAGainInvMinus1VINM1 sets gain = -1 (with VINM1 input)
	OPAMPPGAGainInvMinus1VINM1 = 0x10 << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGainInvMinus3VINM1 sets gain = -3 (with VINM1 input)
	OPAMPPGAGainInvMinus3VINM1 = 0x11 << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGainInvMinus7VINM1 sets gain = -7 (with VINM1 input)
	OPAMPPGAGainInvMinus7VINM1 = 0x12 << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGainInvMinus15VINM1 sets gain = -15 (with VINM1 input)
	OPAMPPGAGainInvMinus15VINM1 = 0x13 << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGainInvMinus31VINM1 sets gain = -31 (with VINM1 input)
	OPAMPPGAGainInvMinus31VINM1 = 0x14 << opampCSR_PGA_GAIN_Pos
	// OPAMPPGAGainInvMinus63VINM1 sets gain = -63 (with VINM1 input)
	OPAMPPGAGainInvMinus63VINM1 = 0x15 << opampCSR_PGA_GAIN_Pos
)

// Calibration selection values
const (
	// OPAMPCalSel3P3 selects 3.3% VDDA calibration point
	OPAMPCalSel3P3 = 0x0 << opampCSR_CALSEL_Pos
	// OPAMPCalSel10 selects 10% VDDA calibration point
	OPAMPCalSel10 = 0x1 << opampCSR_CALSEL_Pos
	// OPAMPCalSel50 selects 50% VDDA calibration point
	OPAMPCalSel50 = 0x2 << opampCSR_CALSEL_Pos
	// OPAMPCalSel90 selects 90% VDDA calibration point
	OPAMPCalSel90 = 0x3 << opampCSR_CALSEL_Pos
)

// OPAMP represents a single OPAMP peripheral instance
type OPAMP struct {
	csr *volatile.Register32
}

// Global OPAMP instances
var (
	OPAMP1 = OPAMP{csr: &stm32.OPAMP.OPAMP1_CSR}
	OPAMP2 = OPAMP{csr: &stm32.OPAMP.OPAMP2_CSR}
	OPAMP3 = OPAMP{csr: &stm32.OPAMP.OPAMP3_CSR}
)

// OPAMPConfig holds configuration for the OPAMP
type OPAMPConfig struct {
	// Mode selects the operating mode via VM_SEL
	// Use OPAMPVMSelVINM0, OPAMPVMSelVINM1, OPAMPVMSelPGA, or OPAMPVMSelFollower
	Mode uint32

	// NonInvertingInput selects the non-inverting input via VP_SEL
	// Use OPAMPVPSelVINP0, OPAMPVPSelVINP1, OPAMPVPSelVINP2, or OPAMPVPSelDAC
	NonInvertingInput uint32

	// PGAGain sets the gain when Mode is OPAMPVMSelPGA
	// Use OPAMPPGAGain2, OPAMPPGAGain4, etc. for non-inverting
	// or OPAMPPGAGainInvMinus1, etc. for inverting configurations
	PGAGain uint32

	// HighSpeed enables high-speed mode for better bandwidth at higher power
	HighSpeed bool

	// InternalOutput enables routing the output to the internal ADC channel
	InternalOutput bool
}

// enableSYSCFGClock enables the SYSCFG clock which is required for OPAMP
func enableSYSCFGClock() {
	stm32.RCC.APB2ENR.SetBits(stm32.RCC_APB2ENR_SYSCFGEN)
}

// Configure configures the OPAMP with the given settings
// The OPAMP is enabled after configuration
func (o *OPAMP) Configure(config OPAMPConfig) {
	// Enable SYSCFG clock (shared with COMP)
	enableSYSCFGClock()

	// Build CSR value
	csr := uint32(0)

	// Set mode (VM_SEL)
	csr |= config.Mode & opampCSR_VM_SEL_Msk

	// Set non-inverting input (VP_SEL)
	csr |= config.NonInvertingInput & opampCSR_VP_SEL_Msk

	// Set PGA gain if in PGA mode
	if config.Mode == OPAMPVMSelPGA {
		csr |= config.PGAGain & opampCSR_PGA_GAIN_Msk
	}

	// High-speed mode
	if config.HighSpeed {
		csr |= opampCSR_OPAHSM
	}

	// Internal output to ADC
	if config.InternalOutput {
		csr |= opampCSR_OPAINTOEN
	}

	// Enable the OPAMP
	csr |= opampCSR_OPAEN

	o.csr.Set(csr)
}

// ConfigureFollower configures the OPAMP as a unity-gain voltage follower
func (o *OPAMP) ConfigureFollower(input uint32, internalOutput bool) {
	enableSYSCFGClock()

	csr := uint32(OPAMPVMSelFollower)
	csr |= input & opampCSR_VP_SEL_Msk

	if internalOutput {
		csr |= opampCSR_OPAINTOEN
	}

	csr |= opampCSR_OPAEN

	o.csr.Set(csr)
}

// ConfigurePGA configures the OPAMP as a Programmable Gain Amplifier
func (o *OPAMP) ConfigurePGA(input uint32, gain uint32, internalOutput bool) {
	enableSYSCFGClock()

	csr := uint32(OPAMPVMSelPGA)
	csr |= input & opampCSR_VP_SEL_Msk
	csr |= gain & opampCSR_PGA_GAIN_Msk

	if internalOutput {
		csr |= opampCSR_OPAINTOEN
	}

	csr |= opampCSR_OPAEN

	o.csr.Set(csr)
}

// ConfigureStandalone configures the OPAMP for standalone operation with external feedback
func (o *OPAMP) ConfigureStandalone(vpInput, vmInput uint32, internalOutput bool) {
	enableSYSCFGClock()

	csr := vmInput & opampCSR_VM_SEL_Msk // VINM0 or VINM1
	csr |= vpInput & opampCSR_VP_SEL_Msk

	if internalOutput {
		csr |= opampCSR_OPAINTOEN
	}

	csr |= opampCSR_OPAEN

	o.csr.Set(csr)
}

// Enable enables the OPAMP
func (o *OPAMP) Enable() {
	enableSYSCFGClock()
	o.csr.SetBits(opampCSR_OPAEN)
}

// Disable disables the OPAMP
func (o *OPAMP) Disable() {
	o.csr.ClearBits(opampCSR_OPAEN)
}

// IsEnabled returns true if the OPAMP is enabled
func (o *OPAMP) IsEnabled() bool {
	return o.csr.HasBits(opampCSR_OPAEN)
}

// SetHighSpeed enables or disables high-speed mode
// High-speed mode provides better bandwidth at higher power consumption
func (o *OPAMP) SetHighSpeed(enable bool) {
	if enable {
		o.csr.SetBits(opampCSR_OPAHSM)
	} else {
		o.csr.ClearBits(opampCSR_OPAHSM)
	}
}

// SetInternalOutput enables or disables routing output to internal ADC
func (o *OPAMP) SetInternalOutput(enable bool) {
	if enable {
		o.csr.SetBits(opampCSR_OPAINTOEN)
	} else {
		o.csr.ClearBits(opampCSR_OPAINTOEN)
	}
}

// SetPGAGain sets the PGA gain (only effective in PGA mode)
func (o *OPAMP) SetPGAGain(gain uint32) {
	csr := o.csr.Get()
	csr &^= opampCSR_PGA_GAIN_Msk
	csr |= gain & opampCSR_PGA_GAIN_Msk
	o.csr.Set(csr)
}

// Lock locks the OPAMP configuration registers
// Once locked, the configuration cannot be changed until the next reset
func (o *OPAMP) Lock() {
	o.csr.SetBits(opampCSR_LOCK)
}

// IsLocked returns true if the OPAMP configuration is locked
func (o *OPAMP) IsLocked() bool {
	return o.csr.HasBits(opampCSR_LOCK)
}

// =============================================================================
// Calibration Support
// =============================================================================

// StartCalibration starts the calibration process
// calSel: calibration point selection (OPAMPCalSel3P3, OPAMPCalSel10, etc.)
func (o *OPAMP) StartCalibration(calSel uint32) {
	enableSYSCFGClock()

	csr := o.csr.Get()
	csr |= opampCSR_USERTRIM // Enable user trimming
	csr |= opampCSR_CALON    // Enable calibration mode
	csr &^= opampCSR_CALSEL_Msk
	csr |= calSel & opampCSR_CALSEL_Msk
	o.csr.Set(csr)
}

// StopCalibration stops the calibration process
func (o *OPAMP) StopCalibration() {
	o.csr.ClearBits(opampCSR_CALON)
}

// CalibrationOutput returns the current calibration comparator output
func (o *OPAMP) CalibrationOutput() bool {
	return o.csr.HasBits(opampCSR_CALOUT)
}

// SetTrimOffsetP sets the PMOS trimming value (0-31)
func (o *OPAMP) SetTrimOffsetP(value uint8) {
	csr := o.csr.Get()
	csr &^= opampCSR_TRIMOFFSETP_Msk
	csr |= uint32(value&0x1F) << opampCSR_TRIMOFFSETP_Pos
	o.csr.Set(csr)
}

// SetTrimOffsetN sets the NMOS trimming value (0-31)
func (o *OPAMP) SetTrimOffsetN(value uint8) {
	csr := o.csr.Get()
	csr &^= opampCSR_TRIMOFFSETN_Msk
	csr |= uint32(value&0x1F) << opampCSR_TRIMOFFSETN_Pos
	o.csr.Set(csr)
}

// GetTrimOffsetP returns the PMOS trimming value
func (o *OPAMP) GetTrimOffsetP() uint8 {
	return uint8((o.csr.Get() & opampCSR_TRIMOFFSETP_Msk) >> opampCSR_TRIMOFFSETP_Pos)
}

// GetTrimOffsetN returns the NMOS trimming value
func (o *OPAMP) GetTrimOffsetN() uint8 {
	return uint8((o.csr.Get() & opampCSR_TRIMOFFSETN_Msk) >> opampCSR_TRIMOFFSETN_Pos)
}

// Calibrate performs automatic calibration of the OPAMP
// This function performs a binary search to find optimal trim values
// Returns the final PMOS and NMOS trim values
func (o *OPAMP) Calibrate() (trimP, trimN uint8) {
	enableSYSCFGClock()

	// Save current configuration
	savedCSR := o.csr.Get()

	// Calibrate NMOS transistors (90% VDDA reference)
	o.StartCalibration(OPAMPCalSel90)
	o.csr.SetBits(opampCSR_OPAEN)

	trimN = o.binarySearchTrim(true)

	// Calibrate PMOS transistors (10% VDDA reference)
	csr := o.csr.Get()
	csr &^= opampCSR_CALSEL_Msk
	csr |= OPAMPCalSel10
	o.csr.Set(csr)

	trimP = o.binarySearchTrim(false)

	// Restore configuration and stop calibration
	o.StopCalibration()
	o.csr.Set(savedCSR)

	// Apply calibrated values
	o.SetTrimOffsetP(trimP)
	o.SetTrimOffsetN(trimN)

	return trimP, trimN
}

// binarySearchTrim performs binary search to find optimal trim value
func (o *OPAMP) binarySearchTrim(isNMOS bool) uint8 {
	var low, high, mid uint8 = 0, 31, 15

	for low <= high {
		mid = (low + high) / 2

		if isNMOS {
			o.SetTrimOffsetN(mid)
		} else {
			o.SetTrimOffsetP(mid)
		}

		// Small delay for settling
		for i := 0; i < 100; i++ {
		}

		if o.CalibrationOutput() {
			low = mid + 1
		} else {
			if mid == 0 {
				break
			}
			high = mid - 1
		}
	}

	return mid
}

// =============================================================================
// Timer-Controlled Mux Support (TCMR)
// =============================================================================

// TCMR register bit positions
const (
	opampTCMR_VMSSEL_Pos  = 0
	opampTCMR_VMSSEL      = 1 << opampTCMR_VMSSEL_Pos
	opampTCMR_VPSSEL_Pos  = 1
	opampTCMR_VPSSEL_Msk  = 0x3 << opampTCMR_VPSSEL_Pos
	opampTCMR_T1CMEN_Pos  = 4
	opampTCMR_T1CMEN      = 1 << opampTCMR_T1CMEN_Pos
	opampTCMR_T8CMEN_Pos  = 5
	opampTCMR_T8CMEN      = 1 << opampTCMR_T8CMEN_Pos
	opampTCMR_T20CMEN_Pos = 6
	opampTCMR_T20CMEN     = 1 << opampTCMR_T20CMEN_Pos
	opampTCMR_LOCK_Pos    = 31
	opampTCMR_LOCK        = 1 << opampTCMR_LOCK_Pos
)

// OPAMPTimerMux represents the timer-controlled multiplexer configuration
type OPAMPTimerMux struct {
	// VMSSel: 0 = VINM0 selected, 1 = VINM1 selected when timer triggers
	VMSSel bool
	// VPSSel: secondary non-inverting input selection for timer control
	// 0 = VINP0, 1 = VINP1, 2 = VINP2
	VPSSel uint8
	// EnableTIM1: enable TIM1 CC6 to control input mux
	EnableTIM1 bool
	// EnableTIM8: enable TIM8 CC6 to control input mux
	EnableTIM8 bool
	// EnableTIM20: enable TIM20 CC6 to control input mux
	EnableTIM20 bool
}

// tcmrReg returns a pointer to the TCMR register for this OPAMP
func (o *OPAMP) tcmrReg() *volatile.Register32 {
	// Calculate offset from CSR to TCMR
	// TCMR registers are at offset 0x18, 0x1C, 0x20 for OPAMP1, 2, 3
	// CSR registers are at offset 0x00, 0x04, 0x08
	csrAddr := uintptr(unsafe.Pointer(o.csr))
	tcmrAddr := csrAddr + 0x18 // TCMR offset from CSR
	return (*volatile.Register32)(unsafe.Pointer(tcmrAddr))
}

// ConfigureTimerMux configures the timer-controlled input multiplexer
// This allows automatic switching between input channels synchronized to timer events
func (o *OPAMP) ConfigureTimerMux(config OPAMPTimerMux) {
	tcmr := uint32(0)

	if config.VMSSel {
		tcmr |= opampTCMR_VMSSEL
	}

	tcmr |= uint32(config.VPSSel&0x3) << opampTCMR_VPSSEL_Pos

	if config.EnableTIM1 {
		tcmr |= opampTCMR_T1CMEN
	}
	if config.EnableTIM8 {
		tcmr |= opampTCMR_T8CMEN
	}
	if config.EnableTIM20 {
		tcmr |= opampTCMR_T20CMEN
	}

	o.tcmrReg().Set(tcmr)
}

// LockTimerMux locks the timer mux configuration
func (o *OPAMP) LockTimerMux() {
	o.tcmrReg().SetBits(opampTCMR_LOCK)
}

// IsTimerMuxLocked returns true if the timer mux configuration is locked
func (o *OPAMP) IsTimerMuxLocked() bool {
	return o.tcmrReg().HasBits(opampTCMR_LOCK)
}
