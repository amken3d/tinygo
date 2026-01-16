package main

// This example demonstrates the quadrature encoder interface mode
// available on STM32G4 timers (TIM1, TIM2, TIM3, TIM4, TIM8).
//
// Connect a quadrature encoder to the timer's CH1 and CH2 pins.
// The timer hardware automatically counts encoder pulses and
// tracks direction - no software polling or interrupts needed.

import (
	"machine"
	"time"
)

func main() {
	println("Quadrature Encoder Example")
	println("==========================")
	time.Sleep(time.Second)

	// Configure the timer for encoder mode
	// EncoderMode3 counts on all edges of both channels (x4 resolution)
	// For a 1000 CPR encoder, this gives 4000 counts per revolution
	err := encTimer.ConfigureEncoder(machine.EncoderConfig{
		Mode:    machine.EncoderMode3, // x4 resolution (both edges, both channels)
		Filter:  2,                    // Light input filtering for noise rejection
		InvertA: false,                // Set true to reverse count direction
		InvertB: false,
	})
	if err != nil {
		println("Error configuring encoder:", err.Error())
		return
	}
	println("Encoder mode configured")

	// Set the auto-reload value (maximum count before wrap)
	// For position tracking, set to (CPR * 4 - 1) for one full revolution
	// Or use maximum value for continuous counting
	encTimer.Device.ARR.Set(0xFFFFFFFF) // Maximum for TIM2 (32-bit)
	println("ARR set to maximum (32-bit counter)")

	// Reset counter to zero
	encTimer.ResetCounter()

	// Start the timer
	encTimer.Start()
	println("Encoder counting started")
	println("")

	// Configure encoder pins (must be done after encoder mode config)
	// The pins are automatically configured for alternate function
	// when ConfigureEncoder is called, but we show explicit config here
	encPinA.ConfigureAltFunc(machine.PinConfig{Mode: machine.PinInputPullup}, encAltFunc)
	encPinB.ConfigureAltFunc(machine.PinConfig{Mode: machine.PinInputPullup}, encAltFunc)

	println("Rotate the encoder to see position changes...")
	println("")

	var lastCount uint32 = 0
	for {
		// Read current position
		count := encTimer.Count()

		// Only print if changed
		if count != lastCount {
			// Get direction (true = counting up, false = counting down)
			direction := "CW "
			if !encTimer.CountDirection() {
				direction = "CCW"
			}

			// Calculate position in degrees (assuming 1000 CPR encoder)
			// With x4 mode: 4000 counts = 360 degrees
			degrees := (count % 4000) * 360 / 4000

			println("Count:", count, " Direction:", direction, " Angle:", degrees, "deg")
			lastCount = count
		}

		time.Sleep(time.Millisecond * 50)
	}
}
