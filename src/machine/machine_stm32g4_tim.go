//go:build stm32g4

package machine

// Timer implementation for STM32G4
// This is a simplified version that supports basic timer functionality
// needed for tick timers. Advanced PWM features on channels 3/4 are not
// supported due to device file limitations.

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
	Pins []PinFunction
}

type TIM struct {
	EnableRegister *volatile.Register32
	EnableFlag     uint32
	Device         *stm32.TIM_Type
	Channels       [4]TimerChannel
	UpInterrupt    interrupt.Interrupt
	OCInterrupt    interrupt.Interrupt

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

// SetWraparoundInterrupt configures a callback to be called each
// time the timer 'wraps-around'.
func (t *TIM) SetWraparoundInterrupt(callback TimerCallback) error {
	// Ensure the interrupt handler for Update events is registered
	t.UpInterrupt = t.registerUPInterrupt()

	// Clear update flag
	t.Device.SR.ClearBits(stm32.TIM_SR_UIF)

	t.wraparoundCallback = callback
	t.UpInterrupt.SetPriority(0xc1)
	t.UpInterrupt.Enable()

	// Enable the hardware interrupt
	t.Device.DIER.SetBits(stm32.TIM_DIER_UIE)

	return nil
}

// Sets a callback to be called when a channel reaches it's set-point.
// Note: Only channels 0 and 1 are supported on STM32G4
func (t *TIM) SetMatchInterrupt(channel uint8, callback ChannelCallback) error {
	if channel > 1 {
		return errors.New("channel not supported on STM32G4")
	}

	t.channelCallbacks[channel] = callback

	// Ensure the interrupt handler for Output Compare events is registered
	t.OCInterrupt = t.registerOCInterrupt()

	// Clear the interrupt flag
	t.Device.SR.ClearBits(stm32.TIM_SR_CC1IF << channel)

	// Enable the interrupt
	t.OCInterrupt.SetPriority(0xc1)
	t.OCInterrupt.Enable()

	// Enable the hardware interrupt
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
		top = (period / 1000) * (t.busFreq / 1000) / (1000 * 1000)
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
// Note: Only channels 0 and 1 are supported on STM32G4
func (t *TIM) Channel(pin Pin) (uint8, error) {
	for chi, ch := range t.Channels {
		if chi > 1 {
			continue // Skip unsupported channels
		}
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
func (t *TIM) Set(channel uint8, value uint32) {
	t.enableMainOutput()

	// Set output compare mode to PWM mode 1 and enable the channel
	switch channel {
	case 0:
		t.Device.CCMR1_Output.ReplaceBits(PWM_MODE1<<stm32.TIM_CCMR1_Output_OC1M_Pos, stm32.TIM_CCMR1_Output_OC1M_Msk, 0)
		t.Device.CCR1.Set(arrtype(value))
	case 1:
		t.Device.CCMR1_Output.ReplaceBits(PWM_MODE1<<stm32.TIM_CCMR1_Output_OC2M_Pos, stm32.TIM_CCMR1_Output_OC2M_Msk, 0)
		t.Device.CCR2.Set(arrtype(value))
	}

	// Enable output
	t.Device.CCER.SetBits(stm32.TIM_CCER_CC1E << (channel * 4))
}

// Unset disables a channel, including any configured interrupts.
func (t *TIM) Unset(channel uint8) {
	// Disable interrupts whilst programming to prevent spurious OC interrupts
	mask := interrupt.Disable()

	// Disable the channel
	t.Device.CCER.ReplaceBits(0, 0xD, channel*4)

	// Reset to zero value
	switch channel {
	case 0:
		t.Device.CCR1.Set(0)
	case 1:
		t.Device.CCR2.Set(0)
	}

	// Disable the hardware interrupt
	t.Device.DIER.ClearBits(stm32.TIM_DIER_CC1IE << channel)

	// Clear the interrupt flag
	t.Device.SR.ClearBits(stm32.TIM_SR_CC1IF << channel)

	// Restore interrupts
	interrupt.Restore(mask)
}

// handleUPInterrupt is called when the Update (wraparound) interrupt fires
func (t *TIM) handleUPInterrupt(interrupt.Interrupt) {
	if t.Device.SR.HasBits(stm32.TIM_SR_UIF) {
		t.Device.SR.ClearBits(stm32.TIM_SR_UIF)
		if t.wraparoundCallback != nil {
			t.wraparoundCallback()
		}
	}
}

// handleOCInterrupt is called when an Output Compare interrupt fires
func (t *TIM) handleOCInterrupt(interrupt.Interrupt) {
	if t.Device.SR.HasBits(stm32.TIM_SR_CC1IF) {
		t.Device.SR.ClearBits(stm32.TIM_SR_CC1IF)
		if t.channelCallbacks[0] != nil {
			t.channelCallbacks[0](0)
		}
	}
	if t.Device.SR.HasBits(stm32.TIM_SR_CC2IF) {
		t.Device.SR.ClearBits(stm32.TIM_SR_CC2IF)
		if t.channelCallbacks[1] != nil {
			t.channelCallbacks[1](1)
		}
	}
}
