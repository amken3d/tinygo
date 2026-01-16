package main

// This example demonstrates advanced timer features on STM32G4:
// - Center-aligned PWM mode
// - Complementary outputs with dead-time
// - Break input for fault protection
// - TRGO output for ADC synchronization
//
// These features are commonly used for 3-phase motor control (BLDC/PMSM).
// This example shows how to configure the peripheral - the actual motor
// control algorithm (FOC, six-step, etc.) is left to the user.

import (
	"machine"
	"time"
)

func main() {
	println("Advanced Timer Example")
	println("======================")
	time.Sleep(time.Second)

	// Example 1: Center-aligned PWM with complementary outputs
	println("\n1. Configuring center-aligned PWM with dead-time...")
	configureCenterAlignedPWM()

	// Example 2: Demonstrating duty cycle changes
	println("\n2. Running PWM duty cycle sweep...")
	runDutyCycleSweep()

	// Example 3: Encoder mode (if pins are available)
	println("\n3. Configuring encoder mode...")
	configureEncoderMode()

	println("\nExample complete. PWM running continuously.")
	for {
		time.Sleep(time.Second)
	}
}

// configureCenterAlignedPWM sets up TIM1 for 3-phase PWM with:
// - Center-aligned mode (symmetric PWM)
// - Complementary outputs on CH1/CH1N, CH2/CH2N, CH3/CH3N
// - Dead-time insertion between high and low side
// - TRGO output on update event (for ADC triggering)
func configureCenterAlignedPWM() {
	// Enable timer clock and configure period
	// 20kHz PWM = 50us period = 50000ns
	err := pwmTimer.Configure(machine.PWMConfig{
		Period: 50000, // 50us = 20kHz
	})
	if err != nil {
		println("  Error configuring timer:", err.Error())
		return
	}

	// Set center-aligned mode 3 (interrupts on both up and down count)
	// This creates symmetric PWM waveforms ideal for motor control
	pwmTimer.SetCenterAlignedMode(machine.CenterAlignedMode3)
	println("  Center-aligned mode 3 enabled")

	// Configure dead-time (500ns typical for motor drivers)
	err = pwmTimer.SetDeadTimeNs(500)
	if err != nil {
		println("  Error setting dead-time:", err.Error())
	} else {
		println("  Dead-time set to 500ns")
	}

	// Configure off-state behavior for safety
	pwmTimer.SetOffStateIdle(true) // Force outputs to idle state when MOE=0
	pwmTimer.SetOffStateRun(true)  // Force inactive level when channel disabled

	// Configure TRGO to generate trigger on update event
	// This allows ADC to sample at PWM center (optimal for current sensing)
	pwmTimer.SetTRGO(machine.TRGOSourceUpdate)
	println("  TRGO configured for update event")

	// Configure PWM channels
	configureChannel(0, phaseAHigh) // Phase A high-side
	configureChannel(1, phaseBHigh) // Phase B high-side
	configureChannel(2, phaseCHigh) // Phase C high-side

	// Enable complementary outputs (low-side drivers)
	pwmTimer.EnableComplementaryOutput(0) // Phase A low-side
	pwmTimer.EnableComplementaryOutput(1) // Phase B low-side
	pwmTimer.EnableComplementaryOutput(2) // Phase C low-side
	println("  Complementary outputs enabled")

	// Configure break input for fault protection (optional)
	if brakePin != machine.NoPin {
		err = pwmTimer.ConfigureBreak(machine.BreakConfig{
			Enable:                true,
			Polarity:              machine.BreakActiveLow, // Active low fault signal
			Filter:                4,                      // Some filtering
			AutomaticOutputEnable: false,                  // Manual re-enable after fault
		})
		if err == nil {
			println("  Break input configured")
		}
	}

	// Enable main output to start PWM
	pwmTimer.EnableMainOutput()
	pwmTimer.Start()
	println("  PWM started")
}

// configureChannel sets up a single PWM channel
func configureChannel(channel uint8, pin machine.Pin) {
	if pin == machine.NoPin {
		return
	}
	_, err := pwmTimer.Channel(pin)
	if err != nil {
		println("  Error configuring channel", channel, ":", err.Error())
		return
	}
	// Set initial duty cycle to 50%
	pwmTimer.Set(channel, pwmTimer.Top()/2)
}

// runDutyCycleSweep demonstrates changing duty cycles
func runDutyCycleSweep() {
	top := pwmTimer.Top()
	println("  PWM Top value:", top)

	// Sweep from 10% to 90% duty cycle
	for duty := uint32(10); duty <= 90; duty += 10 {
		value := top * duty / 100
		pwmTimer.Set(0, value) // Phase A
		pwmTimer.Set(1, value) // Phase B
		pwmTimer.Set(2, value) // Phase C
		println("  Duty cycle:", duty, "%")
		time.Sleep(time.Millisecond * 500)
	}

	// Return to 50%
	pwmTimer.Set(0, top/2)
	pwmTimer.Set(1, top/2)
	pwmTimer.Set(2, top/2)
	println("  Returned to 50% duty")
}

// configureEncoderMode demonstrates quadrature encoder interface
func configureEncoderMode() {
	if encoderTimer == nil {
		println("  Encoder timer not configured for this board")
		return
	}

	// Configure encoder mode with x4 resolution (count on all edges)
	err := encoderTimer.ConfigureEncoder(machine.EncoderConfig{
		Mode:    machine.EncoderMode3, // x4 resolution
		Filter:  2,                    // Light filtering
		InvertA: false,
		InvertB: false,
	})
	if err != nil {
		println("  Error configuring encoder:", err.Error())
		return
	}

	// Set maximum count (e.g., for 1000 CPR encoder with x4 = 4000 counts/rev)
	encoderTimer.Device.ARR.Set(4000 - 1)

	// Reset and start counting
	encoderTimer.ResetCounter()
	encoderTimer.Start()
	println("  Encoder mode configured (x4 resolution)")

	// Read encoder position a few times
	for i := 0; i < 5; i++ {
		count := encoderTimer.Count()
		dir := "UP"
		if !encoderTimer.CountDirection() {
			dir = "DOWN"
		}
		println("  Encoder count:", count, "Direction:", dir)
		time.Sleep(time.Millisecond * 200)
	}
}
