//go:build nucleog431kb

package main

import "machine"

// PWM Timer - TIM1 (Advanced timer with complementary outputs)
var pwmTimer = &machine.TIM1

// 3-Phase PWM pins (directly from TIM1 channels)
// These pins are directly driven by the timer hardware
var (
	// High-side outputs (directly from timer)
	phaseAHigh = machine.PA8  // TIM1_CH1
	phaseBHigh = machine.PA9  // TIM1_CH2
	phaseCHigh = machine.PA10 // TIM1_CH3

	// Low-side outputs (directly from timer, accent on actual Nucleo-G431KB pinout)
	// Note: Check your specific board schematic for available CHxN pins
	phaseALow = machine.PA7 // TIM1_CH1N (directly from timer)
	phaseBLow = machine.PB0 // TIM1_CH2N (directly from timer)
	phaseCLow = machine.PB1 // TIM1_CH3N (directly from timer)

	// Brake input (directly from timer for hardware fault protection)
	brakePin = machine.PA6 // TIM1_BKIN (directly handled by timer hardware)
)

// Encoder Timer - TIM2 (32-bit counter, good for high-resolution encoders)
var encoderTimer = &machine.TIM2

// Encoder pins (directly from timer in encoder mode)
var (
	encoderA = machine.PA0 // TIM2_CH1
	encoderB = machine.PA1 // TIM2_CH2
)
