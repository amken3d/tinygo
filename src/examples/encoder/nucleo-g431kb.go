//go:build nucleog431kb

package main

import "machine"

// Encoder Timer - TIM2 (32-bit counter, ideal for high-resolution encoders)
// TIM2 provides a 32-bit counter that won't overflow during normal operation
var encTimer = &machine.TIM2

// Encoder pins
// TIM2_CH1 and TIM2_CH2 are used for quadrature encoder input
var (
	encPinA    = machine.PA0 // TIM2_CH1 - Encoder channel A
	encPinB    = machine.PA1 // TIM2_CH2 - Encoder channel B
	encAltFunc = uint8(machine.AF1_TIM1_TIM2_TIM5_TIM8_LPTIM1)
)

// Alternative: Use TIM3 (16-bit) if TIM2 is needed for something else
// var encTimer = &machine.TIM3
// var encPinA = machine.PA6 // TIM3_CH1
// var encPinB = machine.PA7 // TIM3_CH2
// var encAltFunc = uint8(machine.AF2_TIM1_TIM2_TIM3_TIM4_TIM5_TIM8_TIM15)
