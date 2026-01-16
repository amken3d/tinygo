//go:build b_g431b_esc1

package main

import "machine"

// PWM Timer - TIM1 (Advanced timer with complementary outputs)
var pwmTimer = &machine.TIM1

// 3-Phase PWM pins for B-G431B-ESC1 motor controller
// These match the actual gate driver connections on the board
var (
	// High-side outputs (directly from timer)
	phaseAHigh = machine.PHASE_UH // PA8 - TIM1_CH1
	phaseBHigh = machine.PHASE_VH // PA9 - TIM1_CH2
	phaseCHigh = machine.PHASE_WH // PA10 - TIM1_CH3

	// Low-side outputs (directly from timer complementary channels)
	phaseALow = machine.PHASE_UL // PC13 - TIM1_CH1N
	phaseBLow = machine.PHASE_VL // PA12 - TIM1_CH2N
	phaseCLow = machine.PHASE_WL // PB15 - TIM1_CH3N

	// No dedicated brake pin on B-G431B-ESC1, use comparator output instead
	// For this example, we'll use PC10 (user button) as a manual brake trigger
	brakePin = machine.PC10 // User button as brake input
)

// Encoder Timer - TIM4 (used for Hall sensors / encoder on this board)
var encoderTimer = &machine.TIM4

// Encoder/Hall sensor pins (directly from timer in encoder mode)
var (
	encoderA = machine.HALL1 // PB6 - TIM4_CH1
	encoderB = machine.HALL2 // PB7 - TIM4_CH2
)
