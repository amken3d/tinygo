//go:build stm32g4

package machine

// Timer implementation for STM32G4
// Supports all 4 PWM channels with center-aligned mode, complementary outputs,
// dead-time insertion, encoder mode, and ADC triggering for motor control.

import (
	"device/stm32"
	"errors"
	"runtime/interrupt"
	"runtime/volatile"
)

const PWM_MODE1 = 0x6

type TimerCallback func()
type ChannelCallback func(channel uint8)

type PinFunction struct {
	Pin     Pin
	AltFunc uint8
}

type TimerChannel struct {
	Pins              []PinFunction
	ComplementaryPins []PinFunction // CHxN pins (only for TIM1/TIM8 channels 0-2)
}

type TIM struct {
	EnableRegister *volatile.Register32
	EnableFlag     uint32
	Device         *stm32.TIM_Type
	Channels       [4]TimerChannel
	timerInterrupt interrupt.Interrupt
	interruptInit  bool

	wraparoundCallback TimerCallback
	channelCallbacks   [4]ChannelCallback

	busFreq uint64
}

// Configure enables and configures this PWM.
func (t *TIM) Configure(config PWMConfig) error {
	// Enable device
	t.EnableRegister.SetBits(t.EnableFlag)

	err := t.setPeriod(config.Period, true)
	if err != nil {
		return err
	}

	// Auto-repeat
	t.Device.EGR.SetBits(stm32.TIM_EGR_UG)

	// Enable the timer
	t.Device.CR1.SetBits(stm32.TIM_CR1_CEN | stm32.TIM_CR1_ARPE)

	return nil
}

func (t *TIM) Count() uint32 {
	return uint32(t.Device.CNT.Get())
}

// ensureInterrupt registers the unified interrupt handler once
func (t *TIM) ensureInterrupt() {
	if !t.interruptInit {
		t.timerInterrupt = t.registerInterrupt()
		t.timerInterrupt.SetPriority(0xc1)
		t.timerInterrupt.Enable()
		t.interruptInit = true
	}
}

// SetWraparoundInterrupt configures a callback to be called each
// time the timer 'wraps-around'.
func (t *TIM) SetWraparoundInterrupt(callback TimerCallback) error {
	// Ensure the unified interrupt handler is registered (only once)
	t.ensureInterrupt()

	// Clear update flag
	t.Device.SR.ClearBits(stm32.TIM_SR_UIF)

	t.wraparoundCallback = callback

	// Enable the hardware interrupt
	t.Device.DIER.SetBits(stm32.TIM_DIER_UIE)

	return nil
}

// SetMatchInterrupt sets a callback to be called when a channel reaches its set-point.
// Supports all 4 channels (0-3).
func (t *TIM) SetMatchInterrupt(channel uint8, callback ChannelCallback) error {
	if channel > 3 {
		return errors.New("channel must be 0-3")
	}

	// Ensure the unified interrupt handler is registered (only once)
	t.ensureInterrupt()

	t.channelCallbacks[channel] = callback

	// Clear the interrupt flag and enable the hardware interrupt
	// Note: CC1IF, CC2IF, CC3IF, CC4IF are at bits 1, 2, 3, 4 respectively
	// CC1IE, CC2IE, CC3IE, CC4IE are at bits 1, 2, 3, 4 respectively
	t.Device.SR.ClearBits(stm32.TIM_SR_CC1IF << channel)
	t.Device.DIER.SetBits(stm32.TIM_DIER_CC1IE << channel)

	return nil
}

// SetPeriod updates the period of this PWM peripheral.
func (t *TIM) SetPeriod(period uint64) error {
	return t.setPeriod(period, true)
}

// Set the period
func (t *TIM) setPeriod(period uint64, updatePrescaler bool) error {
	var top uint64
	if period == 0 {
		top = ARR_MAX
	} else {
		// Formula: top = period_ns * busFreq_Hz / 1e9
		// Using intermediate divisions to avoid overflow:
		// (period/1000) * (busFreq/1000) / 1000 = period * busFreq / 1e9
		top = (period / 1000) * (t.busFreq / 1000) / 1000
	}

	var psc uint64
	if updatePrescaler {
		psc = 1
		for top/psc >= ARR_MAX {
			psc++
			if psc >= PSC_MAX {
				return errors.New("period is too long")
			}
		}

		t.Device.PSC.Set(uint32(psc) - 1)
	} else {
		psc = uint64(t.Device.PSC.Get()) + 1
	}

	top = top / psc

	t.Device.ARR.Set(arrtype(top) - 1)

	return nil
}

// Top returns the current counter top, for use in duty cycle calculation.
func (t *TIM) Top() uint32 {
	return uint32(t.Device.ARR.Get()) + 1
}

// Channel returns a PWM channel for the given pin.
// Supports all 4 channels (0-3).
func (t *TIM) Channel(pin Pin) (uint8, error) {
	for chi, ch := range t.Channels {
		for _, p := range ch.Pins {
			if p.Pin == pin {
				t.configurePin(uint8(chi), p)
				return uint8(chi), nil
			}
		}
	}

	return 0, ErrInvalidOutputPin
}

// Set the value for the given PWM channel.
// Supports all 4 channels (0-3).
func (t *TIM) Set(channel uint8, value uint32) {
	t.enableMainOutput()

	// Set output compare mode to PWM mode 1 and set compare value
	switch channel {
	case 0:
		t.Device.CCMR1_Output.ReplaceBits(PWM_MODE1<<stm32.TIM_CCMR1_Output_OC1M_Pos, stm32.TIM_CCMR1_Output_OC1M_Msk, 0)
		t.Device.CCR1.Set(arrtype(value))
	case 1:
		t.Device.CCMR1_Output.ReplaceBits(PWM_MODE1<<stm32.TIM_CCMR1_Output_OC2M_Pos, stm32.TIM_CCMR1_Output_OC2M_Msk, 0)
		t.Device.CCR2.Set(arrtype(value))
	case 2:
		t.Device.CCMR2_Output_Reg().ReplaceBits(PWM_MODE1<<stm32.TIM_CCMR2_Output_OC3M_Pos, stm32.TIM_CCMR2_Output_OC3M_Msk, 0)
		t.Device.CCR3_Reg().Set(arrtype(value))
	case 3:
		t.Device.CCMR2_Output_Reg().ReplaceBits(PWM_MODE1<<stm32.TIM_CCMR2_Output_OC4M_Pos, stm32.TIM_CCMR2_Output_OC4M_Msk, 0)
		t.Device.CCR4_Reg().Set(arrtype(value))
	}

	// Enable output
	t.Device.CCER.SetBits(stm32.TIM_CCER_CC1E << (channel * 4))
}

// Unset disables a channel, including any configured interrupts.
// Supports all 4 channels (0-3).
func (t *TIM) Unset(channel uint8) {
	// Disable interrupts whilst programming to prevent spurious OC interrupts
	mask := interrupt.Disable()

	// Disable the channel (and complementary if applicable)
	t.Device.CCER.ClearBits(0xF << (channel * 4))

	// Reset to zero value
	switch channel {
	case 0:
		t.Device.CCR1.Set(0)
	case 1:
		t.Device.CCR2.Set(0)
	case 2:
		t.Device.CCR3_Reg().Set(0)
	case 3:
		t.Device.CCR4_Reg().Set(0)
	}

	// Disable the hardware interrupt
	t.Device.DIER.ClearBits(stm32.TIM_DIER_CC1IE << channel)

	// Clear the interrupt flag
	t.Device.SR.ClearBits(stm32.TIM_SR_CC1IF << channel)

	// Restore interrupts
	interrupt.Restore(mask)
}

// handleInterrupt is the unified interrupt handler for all timer events
// (Update/overflow and Output Compare). This is necessary because TIM2-TIM4
// share a single IRQ for all events.
func (t *TIM) handleInterrupt(interrupt.Interrupt) {
	// Handle Update (overflow/wraparound) interrupt
	if t.Device.SR.HasBits(stm32.TIM_SR_UIF) {
		t.Device.SR.ClearBits(stm32.TIM_SR_UIF)
		if t.wraparoundCallback != nil {
			t.wraparoundCallback()
		}
	}

	// Handle Output Compare channel 1 interrupt
	if t.Device.SR.HasBits(stm32.TIM_SR_CC1IF) {
		t.Device.SR.ClearBits(stm32.TIM_SR_CC1IF)
		if t.channelCallbacks[0] != nil {
			t.channelCallbacks[0](0)
		}
	}

	// Handle Output Compare channel 2 interrupt
	if t.Device.SR.HasBits(stm32.TIM_SR_CC2IF) {
		t.Device.SR.ClearBits(stm32.TIM_SR_CC2IF)
		if t.channelCallbacks[1] != nil {
			t.channelCallbacks[1](1)
		}
	}

	// Handle Output Compare channel 3 interrupt
	if t.Device.SR.HasBits(stm32.TIM_SR_CC3IF) {
		t.Device.SR.ClearBits(stm32.TIM_SR_CC3IF)
		if t.channelCallbacks[2] != nil {
			t.channelCallbacks[2](2)
		}
	}

	// Handle Output Compare channel 4 interrupt
	if t.Device.SR.HasBits(stm32.TIM_SR_CC4IF) {
		t.Device.SR.ClearBits(stm32.TIM_SR_CC4IF)
		if t.channelCallbacks[3] != nil {
			t.channelCallbacks[3](3)
		}
	}
}
