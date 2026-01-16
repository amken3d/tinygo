//go:build stm32g4

package machine

// Advanced timer features for STM32G4 (TIM1, TIM8, TIM20)
// This file extends the TIM type with hardware features available on advanced timers:
// - Center-aligned PWM mode
// - Complementary outputs with dead-time insertion
// - Break input for fault protection
// - Encoder interface mode
// - Hall sensor interface (TI1 XOR function)
// - TRGO/TRGO2 output for triggering ADC or other timers
// - Repetition counter

import (
	"device/stm32"
	"errors"
)

// Advanced timer errors
var (
	ErrNotAdvancedTimer  = errors.New("timer: feature requires advanced timer (TIM1/TIM8)")
	ErrInvalidDeadTime   = errors.New("timer: dead-time value out of range")
	ErrEncoderNotSupport = errors.New("timer: encoder mode not supported on this timer")
)

// =============================================================================
// Center-Aligned Mode (CR1.CMS)
// =============================================================================

// CenterAlignedMode selects when output compare interrupts are generated
// in center-aligned (up-down counting) PWM mode
type CenterAlignedMode uint8

const (
	// CenterAlignedModeDisabled - Edge-aligned mode (up-counting only)
	CenterAlignedModeDisabled CenterAlignedMode = 0
	// CenterAlignedMode1 - OC interrupt flags set on down-counting only
	CenterAlignedMode1 CenterAlignedMode = 1
	// CenterAlignedMode2 - OC interrupt flags set on up-counting only
	CenterAlignedMode2 CenterAlignedMode = 2
	// CenterAlignedMode3 - OC interrupt flags set on both up and down counting
	CenterAlignedMode3 CenterAlignedMode = 3
)

// SetCenterAlignedMode configures the timer for center-aligned (up-down) counting.
// In center-aligned mode, the counter counts up to ARR then down to 0, creating
// symmetric PWM waveforms. This is commonly used for motor control to reduce
// current sensing noise and EMI.
//
// Note: The timer must be disabled (CEN=0) before changing center-aligned mode.
// Call this before Configure() or while the timer is stopped.
func (t *TIM) SetCenterAlignedMode(mode CenterAlignedMode) {
	// Clear CMS bits and set new mode
	cr1 := t.Device.CR1.Get()
	cr1 &^= stm32.TIM_CR1_CMS_Msk
	cr1 |= uint32(mode) << stm32.TIM_CR1_CMS_Pos
	t.Device.CR1.Set(cr1)
}

// GetCenterAlignedMode returns the current center-aligned mode setting
func (t *TIM) GetCenterAlignedMode() CenterAlignedMode {
	return CenterAlignedMode((t.Device.CR1.Get() & stm32.TIM_CR1_CMS_Msk) >> stm32.TIM_CR1_CMS_Pos)
}

// CountDirection returns the current counting direction.
// Returns true if counting up, false if counting down.
// Only meaningful in center-aligned mode.
func (t *TIM) CountDirection() bool {
	return (t.Device.CR1.Get() & stm32.TIM_CR1_DIR) == 0
}

// =============================================================================
// Complementary Outputs (CCER.CCxNE)
// =============================================================================

// EnableComplementaryOutput enables the complementary (inverted) output for a channel.
// Only available on advanced timers (TIM1, TIM8) for channels 1-3.
// The complementary output appears on the CHxN pin.
//
// For proper operation with complementary outputs:
// 1. Configure dead-time using SetDeadTime()
// 2. Enable main output using EnableMainOutput()
func (t *TIM) EnableComplementaryOutput(channel uint8) error {
	if !t.IsAdvancedTimer() {
		return ErrNotAdvancedTimer
	}
	if channel > 2 {
		return errors.New("timer: complementary output only available on channels 0-2")
	}

	// CCxNE bits are at positions 2, 6, 10 for channels 1, 2, 3
	t.Device.CCER.SetBits(stm32.TIM_CCER_CC1NE << (channel * 4))
	return nil
}

// DisableComplementaryOutput disables the complementary output for a channel
func (t *TIM) DisableComplementaryOutput(channel uint8) {
	if channel <= 2 {
		t.Device.CCER.ClearBits(stm32.TIM_CCER_CC1NE << (channel * 4))
	}
}

// SetComplementaryPolarity sets the polarity of the complementary output.
// If inverted is true, CCxN is active low. If false, CCxN is active high.
func (t *TIM) SetComplementaryPolarity(channel uint8, inverted bool) {
	if channel > 2 {
		return
	}
	// CCxNP bits are at positions 3, 7, 11 for channels 1, 2, 3
	if inverted {
		t.Device.CCER.SetBits(stm32.TIM_CCER_CC1NP << (channel * 4))
	} else {
		t.Device.CCER.ClearBits(stm32.TIM_CCER_CC1NP << (channel * 4))
	}
}

// =============================================================================
// Dead-Time Insertion (BDTR.DTG)
// =============================================================================

// SetDeadTimeRaw sets the dead-time generator value directly (DTG register, 0-255).
// The actual dead-time depends on the timer clock and DTG encoding:
//   - DTG[7:5]=0xx: DT = DTG[6:0] × tDTS
//   - DTG[7:5]=10x: DT = (64 + DTG[5:0]) × 2 × tDTS
//   - DTG[7:5]=110: DT = (32 + DTG[4:0]) × 8 × tDTS
//   - DTG[7:5]=111: DT = (32 + DTG[4:0]) × 16 × tDTS
//
// Only available on advanced timers (TIM1, TIM8).
func (t *TIM) SetDeadTimeRaw(dtg uint8) error {
	if !t.IsAdvancedTimer() {
		return ErrNotAdvancedTimer
	}
	t.Device.BDTR.ReplaceBits(uint32(dtg), stm32.TIM_BDTR_DTG_Msk, 0)
	return nil
}

// SetDeadTimeNs calculates and sets the dead-time in nanoseconds.
// The function automatically selects the appropriate DTG encoding.
// Returns error if the requested dead-time cannot be achieved.
//
// At 170MHz bus clock:
//   - Minimum: ~6ns (DTG=1)
//   - Maximum: ~5930ns (DTG=0xFF)
func (t *TIM) SetDeadTimeNs(nanoseconds uint16) error {
	if !t.IsAdvancedTimer() {
		return ErrNotAdvancedTimer
	}
	if nanoseconds == 0 {
		t.Device.BDTR.ReplaceBits(0, stm32.TIM_BDTR_DTG_Msk, 0)
		return nil
	}

	// tDTS = 1 / busFreq (when CKD=00)
	tDTS_ns := float64(1e9) / float64(t.busFreq)
	dt := float64(nanoseconds)

	var dtg uint8

	// Try DTG[7:5]=0xx range: 0-127 × tDTS
	if dt <= 127*tDTS_ns {
		dtg = uint8(dt / tDTS_ns)
	} else if dt <= 127*2*tDTS_ns {
		// DTG[7:5]=10x range: (64+0 to 64+63) × 2 × tDTS
		dtg = uint8((dt/(2*tDTS_ns))-64) | 0x80
	} else if dt <= 63*8*tDTS_ns {
		// DTG[7:5]=110 range: (32+0 to 32+31) × 8 × tDTS
		dtg = uint8((dt/(8*tDTS_ns))-32) | 0xC0
	} else if dt <= 63*16*tDTS_ns {
		// DTG[7:5]=111 range: (32+0 to 32+31) × 16 × tDTS
		dtg = uint8((dt/(16*tDTS_ns))-32) | 0xE0
	} else {
		return ErrInvalidDeadTime
	}

	t.Device.BDTR.ReplaceBits(uint32(dtg), stm32.TIM_BDTR_DTG_Msk, 0)
	return nil
}

// GetDeadTimeRaw returns the current DTG register value
func (t *TIM) GetDeadTimeRaw() uint8 {
	return uint8(t.Device.BDTR.Get() & stm32.TIM_BDTR_DTG_Msk)
}

// =============================================================================
// Break Input (BDTR.BKE, BKP, etc.)
// =============================================================================

// BreakPolarity defines the active level of the break input
type BreakPolarity uint8

const (
	BreakActiveLow  BreakPolarity = 0 // Break input active low
	BreakActiveHigh BreakPolarity = 1 // Break input active high
)

// BreakConfig configures the break input for fault protection
type BreakConfig struct {
	// Enable activates the break input
	Enable bool
	// Polarity sets whether break is active high or low
	Polarity BreakPolarity
	// Filter sets the digital filter (0-15, higher = more filtering)
	Filter uint8
	// AutomaticOutputEnable: if true, MOE is set automatically after break clears
	AutomaticOutputEnable bool
}

// ConfigureBreak sets up the break input for fault protection.
// When the break input is active, all outputs are forced to their off-state.
// Only available on advanced timers (TIM1, TIM8).
func (t *TIM) ConfigureBreak(config BreakConfig) error {
	if !t.IsAdvancedTimer() {
		return ErrNotAdvancedTimer
	}

	bdtr := t.Device.BDTR.Get()

	// Clear break-related bits
	bdtr &^= stm32.TIM_BDTR_BKE | stm32.TIM_BDTR_BKP | stm32.TIM_BDTR_BKF_Msk | stm32.TIM_BDTR_AOE

	if config.Enable {
		bdtr |= stm32.TIM_BDTR_BKE
		if config.Polarity == BreakActiveHigh {
			bdtr |= stm32.TIM_BDTR_BKP
		}
		if config.Filter > 0 && config.Filter <= 15 {
			bdtr |= uint32(config.Filter) << stm32.TIM_BDTR_BKF_Pos
		}
	}

	if config.AutomaticOutputEnable {
		bdtr |= stm32.TIM_BDTR_AOE
	}

	t.Device.BDTR.Set(bdtr)
	return nil
}

// TriggerBreakEvent manually triggers a break event (software fault)
func (t *TIM) TriggerBreakEvent() {
	t.Device.EGR.SetBits(stm32.TIM_EGR_BG)
}

// =============================================================================
// Main Output Enable (BDTR.MOE)
// =============================================================================

// EnableMainOutput enables the main output (MOE bit).
// On advanced timers, outputs are only active when MOE=1.
// This is automatically cleared by a break event.
func (t *TIM) EnableMainOutput() {
	if t.IsAdvancedTimer() {
		t.Device.BDTR.SetBits(stm32.TIM_BDTR_MOE)
	}
}

// DisableMainOutput disables the main output (clears MOE bit).
// All outputs go to their off-state defined by OSSI/OSSR.
func (t *TIM) DisableMainOutput() {
	if t.IsAdvancedTimer() {
		t.Device.BDTR.ClearBits(stm32.TIM_BDTR_MOE)
	}
}

// IsMainOutputEnabled returns true if the main output enable is set
func (t *TIM) IsMainOutputEnabled() bool {
	return t.Device.BDTR.HasBits(stm32.TIM_BDTR_MOE)
}

// SetOffStateIdle configures the off-state for idle mode (OSSI).
// If enabled, outputs are forced to their idle level when MOE=0.
func (t *TIM) SetOffStateIdle(enabled bool) {
	if enabled {
		t.Device.BDTR.SetBits(stm32.TIM_BDTR_OSSI)
	} else {
		t.Device.BDTR.ClearBits(stm32.TIM_BDTR_OSSI)
	}
}

// SetOffStateRun configures the off-state for run mode (OSSR).
// If enabled, outputs are forced to their inactive level when the channel is disabled.
func (t *TIM) SetOffStateRun(enabled bool) {
	if enabled {
		t.Device.BDTR.SetBits(stm32.TIM_BDTR_OSSR)
	} else {
		t.Device.BDTR.ClearBits(stm32.TIM_BDTR_OSSR)
	}
}

// =============================================================================
// Encoder Interface Mode (SMCR.SMS)
// =============================================================================

// EncoderMode selects which encoder edges to count
type EncoderMode uint8

const (
	// EncoderModeDisabled - Normal timer operation (no encoder)
	EncoderModeDisabled EncoderMode = 0
	// EncoderMode1 - Counter counts on TI1 edges only (x1 or x2 resolution)
	EncoderMode1 EncoderMode = 1
	// EncoderMode2 - Counter counts on TI2 edges only (x1 or x2 resolution)
	EncoderMode2 EncoderMode = 2
	// EncoderMode3 - Counter counts on both TI1 and TI2 edges (x4 resolution)
	EncoderMode3 EncoderMode = 3
)

// EncoderConfig configures the quadrature encoder interface
type EncoderConfig struct {
	// Mode selects which edges to count (1, 2, or 3 for x4 resolution)
	Mode EncoderMode
	// Filter sets the input filter (0-15, higher = more filtering)
	Filter uint8
	// InvertA inverts the TI1 input polarity (reverses count direction)
	InvertA bool
	// InvertB inverts the TI2 input polarity
	InvertB bool
}

// ConfigureEncoder sets up the timer in quadrature encoder interface mode.
// In encoder mode, the timer counts based on edges from two quadrature signals
// (CH1/TI1 and CH2/TI2). The direction is determined by the phase relationship.
//
// Supported timers: TIM2, TIM3, TIM4 (general-purpose with encoder mode)
// TIM1 and TIM8 also support encoder mode.
func (t *TIM) ConfigureEncoder(config EncoderConfig) error {
	// Check if timer supports encoder mode (needs SMCR register)
	dev := t.Device
	if dev != stm32.TIM1 && dev != stm32.TIM2 && dev != stm32.TIM3 &&
		dev != stm32.TIM4 && dev != stm32.TIM8 {
		return ErrEncoderNotSupport
	}

	// Disable timer during configuration
	t.Device.CR1.ClearBits(stm32.TIM_CR1_CEN)

	// Set encoder mode in SMCR (Slave Mode Control Register)
	var sms uint32
	switch config.Mode {
	case EncoderMode1:
		sms = stm32.TIM_SMCR_SMS_EncoderMode1
	case EncoderMode2:
		sms = stm32.TIM_SMCR_SMS_EncoderMode2
	case EncoderMode3:
		sms = stm32.TIM_SMCR_SMS_EncoderMode3
	default:
		// Disable encoder mode
		t.Device.SMCR.ClearBits(stm32.TIM_SMCR_SMS_Msk)
		return nil
	}
	t.Device.SMCR.ReplaceBits(sms, stm32.TIM_SMCR_SMS_Msk, 0)

	// Configure input capture for encoder mode
	// CCMR1: CC1S=01 (IC1 mapped to TI1), CC2S=01 (IC2 mapped to TI2)
	ccmr1 := uint32(stm32.TIM_CCMR1_Input_CC1S_TI1) | uint32(stm32.TIM_CCMR1_Input_CC2S_TI2)

	// Apply input filter (same for both channels)
	if config.Filter > 0 && config.Filter <= 15 {
		ccmr1 |= uint32(config.Filter) << stm32.TIM_CCMR1_Input_IC1F_Pos
		ccmr1 |= uint32(config.Filter) << stm32.TIM_CCMR1_Input_IC2F_Pos
	}
	t.Device.CCMR1_Output.Set(ccmr1) // Same register address in input mode

	// Configure polarity (CCER register)
	// CC1E and CC2E must be set for encoder mode
	var ccer uint32 = stm32.TIM_CCER_CC1E | stm32.TIM_CCER_CC2E
	if config.InvertA {
		ccer |= stm32.TIM_CCER_CC1P // Invert TI1 polarity
	}
	if config.InvertB {
		ccer |= stm32.TIM_CCER_CC2P // Invert TI2 polarity
	}
	t.Device.CCER.Set(ccer)

	return nil
}

// ResetCounter resets the counter to zero
func (t *TIM) ResetCounter() {
	t.Device.CNT.Set(0)
}

// SetCounter sets the counter value
func (t *TIM) SetCounter(value uint32) {
	t.Device.CNT.Set(value)
}

// =============================================================================
// Hall Sensor Interface (CR2.TI1S)
// =============================================================================

// EnableHallSensorMode enables the Hall sensor interface.
// When enabled, TI1 is the XOR of CH1, CH2, and CH3 inputs.
// This allows detecting any Hall sensor state change on a single input.
// Typically used with slave mode reset to measure time between commutations.
func (t *TIM) EnableHallSensorMode() {
	t.Device.CR2.SetBits(stm32.TIM_CR2_TI1S)
}

// DisableHallSensorMode disables the Hall sensor interface
func (t *TIM) DisableHallSensorMode() {
	t.Device.CR2.ClearBits(stm32.TIM_CR2_TI1S)
}

// IsHallSensorModeEnabled returns true if Hall sensor mode is enabled
func (t *TIM) IsHallSensorModeEnabled() bool {
	return t.Device.CR2.HasBits(stm32.TIM_CR2_TI1S)
}

// =============================================================================
// TRGO/TRGO2 Output (CR2.MMS/MMS2)
// =============================================================================

// TRGOSource selects what generates the TRGO (trigger output) signal
type TRGOSource uint8

const (
	TRGOSourceReset        TRGOSource = 0 // UG bit generates trigger
	TRGOSourceEnable       TRGOSource = 1 // Counter enable generates trigger
	TRGOSourceUpdate       TRGOSource = 2 // Update event generates trigger
	TRGOSourceComparePulse TRGOSource = 3 // CC1IF generates trigger
	TRGOSourceOC1REF       TRGOSource = 4 // OC1REF generates trigger
	TRGOSourceOC2REF       TRGOSource = 5 // OC2REF generates trigger
	TRGOSourceOC3REF       TRGOSource = 6 // OC3REF generates trigger
	TRGOSourceOC4REF       TRGOSource = 7 // OC4REF generates trigger
)

// SetTRGO configures the TRGO (trigger output) source.
// TRGO can be used to trigger ADC conversions or synchronize other timers.
// For motor control ADC triggering, TRGOSourceUpdate is commonly used
// to sample at the PWM center (in center-aligned mode).
func (t *TIM) SetTRGO(source TRGOSource) {
	t.Device.CR2.ReplaceBits(uint32(source)<<stm32.TIM_CR2_MMS_Pos, stm32.TIM_CR2_MMS_Msk, 0)
}

// GetTRGO returns the current TRGO source
func (t *TIM) GetTRGO() TRGOSource {
	return TRGOSource((t.Device.CR2.Get() & stm32.TIM_CR2_MMS_Msk) >> stm32.TIM_CR2_MMS_Pos)
}

// TRGO2Source selects what generates the TRGO2 signal (advanced timers only)
type TRGO2Source uint8

const (
	TRGO2SourceReset  TRGO2Source = 0
	TRGO2SourceEnable TRGO2Source = 1
	TRGO2SourceUpdate TRGO2Source = 2
	TRGO2SourceOC1REF TRGO2Source = 4
	TRGO2SourceOC2REF TRGO2Source = 5
	TRGO2SourceOC3REF TRGO2Source = 6
	TRGO2SourceOC4REF TRGO2Source = 7
	TRGO2SourceOC5REF TRGO2Source = 8
	TRGO2SourceOC6REF TRGO2Source = 9
)

// SetTRGO2 configures the TRGO2 source (advanced timers only).
// TRGO2 provides a second trigger output, typically used for ADC triggering.
func (t *TIM) SetTRGO2(source TRGO2Source) error {
	if !t.IsAdvancedTimer() {
		return ErrNotAdvancedTimer
	}
	t.Device.CR2.ReplaceBits(uint32(source)<<stm32.TIM_CR2_MMS2_Pos, stm32.TIM_CR2_MMS2_Msk, 0)
	return nil
}

// =============================================================================
// Repetition Counter (RCR)
// =============================================================================

// SetRepetitionCounter sets the repetition counter value.
// Update events are only generated after (RCR+1) counter overflows/underflows.
// This is useful for reducing update interrupt frequency in motor control.
// Only available on advanced timers (TIM1, TIM8).
func (t *TIM) SetRepetitionCounter(value uint8) error {
	if !t.IsAdvancedTimer() {
		return ErrNotAdvancedTimer
	}
	t.Device.RCR.Set(uint32(value))
	return nil
}

// GetRepetitionCounter returns the current repetition counter value
func (t *TIM) GetRepetitionCounter() uint8 {
	return uint8(t.Device.RCR.Get())
}

// =============================================================================
// Utility Functions
// =============================================================================

// IsAdvancedTimer returns true if this is an advanced-control timer (TIM1, TIM8)
func (t *TIM) IsAdvancedTimer() bool {
	return t.Device == stm32.TIM1 || t.Device == stm32.TIM8
}

// Start enables the timer counter
func (t *TIM) Start() {
	t.Device.CR1.SetBits(stm32.TIM_CR1_CEN)
}

// Stop disables the timer counter
func (t *TIM) Stop() {
	t.Device.CR1.ClearBits(stm32.TIM_CR1_CEN)
}

// IsRunning returns true if the timer counter is enabled
func (t *TIM) IsRunning() bool {
	return t.Device.CR1.HasBits(stm32.TIM_CR1_CEN)
}

// GenerateUpdateEvent forces an update event (reloads shadow registers)
func (t *TIM) GenerateUpdateEvent() {
	t.Device.EGR.SetBits(stm32.TIM_EGR_UG)
}

// GetBusFrequency returns the timer's input clock frequency
func (t *TIM) GetBusFrequency() uint64 {
	return t.busFreq
}
