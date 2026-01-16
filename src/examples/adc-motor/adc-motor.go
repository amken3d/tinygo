package main

// This example demonstrates advanced ADC features for motor control on STM32G4.
//
// For Field-Oriented Control (FOC) of 3-phase motors, you need:
// 1. Simultaneous sampling of two phase currents (Ia and Ib)
// 2. Synchronized to PWM center for accurate current measurement
// 3. Fast conversion with minimal CPU overhead (using DMA)
//
// Hardware setup (Nucleo-G431KB with current sensing):
// - Phase A current: PA1 (ADC1_IN2) via OPAMP1
// - Phase B current: PA7 (ADC2_IN4) via OPAMP2
// - PWM on TIM1 (CH1, CH2, CH3 for 3-phase)
//
// The FMAC and CORDIC peripherals can be used for FOC calculations.

import (
	"machine"
	"time"
)

// Current measurement buffer
var (
	currentA [4]uint16 // ADC1 injected results
	currentB [4]uint16 // ADC2 injected results
)

func main() {
	println("ADC Motor Control Example")
	println("=========================")
	time.Sleep(time.Second)

	// Example 1: Basic dual ADC setup
	println("")
	println("1. Dual ADC Simultaneous Sampling")
	println("----------------------------------")
	testDualADCSimultaneous()

	// Example 2: Timer-triggered injected conversion
	println("")
	println("2. Timer-Triggered Injected Conversion")
	println("--------------------------------------")
	testTimerTriggeredInjected()

	// Example 3: DMA-based continuous sampling
	println("")
	println("3. DMA-Based Continuous Sampling")
	println("--------------------------------")
	testDMASampling()

	// Example 4: Complete motor control ADC setup
	println("")
	println("4. Complete FOC Current Sensing Setup")
	println("-------------------------------------")
	testFOCCurrentSensing()

	println("")
	println("Example complete!")

	for {
		time.Sleep(time.Second)
	}
}

// testDualADCSimultaneous demonstrates simultaneous sampling on ADC1 and ADC2
func testDualADCSimultaneous() {
	// Initialize both ADCs
	machine.ADC1Periph.Enable()
	machine.ADC2Periph.Enable()

	// Calibrate both ADCs
	machine.ADC1Periph.Calibrate(false)
	machine.ADC2Periph.Calibrate(false)

	// Configure resolution
	machine.ADC1Periph.Configure(machine.ADCAdvancedConfig{
		Resolution: machine.ADCResolution12Bit,
	})
	machine.ADC2Periph.Configure(machine.ADCAdvancedConfig{
		Resolution: machine.ADCResolution12Bit,
	})

	// Start ADCs
	machine.ADC1Periph.Start()
	machine.ADC2Periph.Start()

	// Configure sample times
	machine.ADC1Periph.SetSampleTime(2, machine.ADCSampleTime24_5) // PA1
	machine.ADC2Periph.SetSampleTime(4, machine.ADCSampleTime24_5) // PA7

	println("  ADC1 and ADC2 configured for simultaneous sampling")
	println("  - ADC1 channel 2 (PA1)")
	println("  - ADC2 channel 4 (PA7)")
	println("")

	// Take some readings
	println("  Reading samples:")
	for i := 0; i < 5; i++ {
		adc1Val := machine.ADC1Periph.ReadChannel(2)
		adc2Val := machine.ADC2Periph.ReadChannel(4)
		println("    ADC1:", adc1Val, " ADC2:", adc2Val)
		time.Sleep(100 * time.Millisecond)
	}

	machine.ADC1Periph.Disable()
	machine.ADC2Periph.Disable()
}

// testTimerTriggeredInjected demonstrates timer-triggered injected conversions
func testTimerTriggeredInjected() {
	println("  Configuring injected channels with TIM1 trigger:")
	println("  - ADC1: Channels 2, 15 (phase currents)")
	println("  - ADC2: Channels 4, 12 (phase currents)")
	println("  - Trigger: TIM1_TRGO2 (PWM center)")
	println("")

	// Configure dual ADC for injected simultaneous mode
	machine.ConfigureInjectedSimultaneous(
		machine.ADCInjectedConfig{
			Channels:    []uint8{2, 15},               // ADC1 channels
			Trigger:     machine.ADCInjTrigTIM1_TRGO2, // TIM1 TRGO2
			TriggerEdge: machine.ADCTriggerRising,
			SampleTime:  machine.ADCSampleTime24_5,
		},
		machine.ADCInjectedConfig{
			Channels:    []uint8{4, 12},               // ADC2 channels
			Trigger:     machine.ADCInjTrigTIM1_TRGO2, // Same trigger
			TriggerEdge: machine.ADCTriggerRising,
			SampleTime:  machine.ADCSampleTime24_5,
		},
	)

	// For this demo, use software trigger instead of timer
	println("  Software triggering for demo:")
	for i := 0; i < 3; i++ {
		// Start injected conversion
		machine.ADC1Periph.StartInjected()

		// Wait for completion
		for !machine.ADC1Periph.IsInjectedComplete() {
		}
		machine.ADC1Periph.ClearInjectedComplete()

		// Read results
		adc1Results := machine.ADC1Periph.ReadInjected()
		adc2Results := machine.ADC2Periph.ReadInjected()

		println("    ADC1: [", adc1Results[0], ",", adc1Results[1], "]",
			" ADC2: [", adc2Results[0], ",", adc2Results[1], "]")

		time.Sleep(100 * time.Millisecond)
	}

	machine.ADC1Periph.Disable()
	machine.ADC2Periph.Disable()
}

// testDMASampling demonstrates DMA-based continuous sampling
func testDMASampling() {
	// Buffer for DMA transfers
	var adcBuffer [8]uint16

	// Enable and configure ADC1
	machine.ADC1Periph.Enable()
	machine.ADC1Periph.Calibrate(false)
	machine.ADC1Periph.Start()

	// Configure multi-channel sequence with DMA
	machine.ADC1Periph.ConfigureRegularSequence(machine.ADCRegularSequenceConfig{
		Channels:   []uint8{1, 2, 6, 7}, // 4 channels
		SampleTime: machine.ADCSampleTime47_5,
		Continuous: true,
		DMA: &machine.ADCDMAConfig{
			DMAChannel: 1,
			Buffer:     &adcBuffer[0],
			BufferLen:  4,
			Circular:   true,
			Priority:   DmaCCR_PL_High,
		},
	})

	println("  DMA configured for 4-channel continuous sampling")
	println("  - Channels: 1, 2, 6, 7")
	println("  - DMA channel: 1")
	println("  - Mode: Circular")
	println("")

	// Enable DMA and start conversion
	machine.ADC1Periph.EnableDMA(1)
	machine.ADC1Periph.StartRegular()

	// Read buffer multiple times
	println("  Reading DMA buffer:")
	for i := 0; i < 3; i++ {
		time.Sleep(100 * time.Millisecond)
		println("    [", adcBuffer[0], ",", adcBuffer[1], ",",
			adcBuffer[2], ",", adcBuffer[3], "]")
	}

	// Stop
	machine.ADC1Periph.StopRegular()
	machine.ADC1Periph.DisableDMA(1)
	machine.ADC1Periph.Disable()
}

// testFOCCurrentSensing shows a complete FOC current sensing setup
func testFOCCurrentSensing() {
	println("  Complete FOC Current Sensing Configuration")
	println("")
	println("  Typical 3-phase motor control setup:")
	println("  =====================================")
	println("")
	println("  1. Hardware Configuration:")
	println("     - OPAMP1 (PA1 input, PA3 output) -> ADC1_IN2 or internal")
	println("     - OPAMP2 (PA7 input, PA6 output) -> ADC2_IN4 or internal")
	println("     - OPAMP3 (PB0 input, PB1 output) -> ADC3 or internal")
	println("")
	println("  2. Timer Configuration (TIM1):")
	println("     - Center-aligned PWM mode")
	println("     - TRGO2 = Update event (triggers ADC at PWM center)")
	println("     - Dead-time for complementary outputs")
	println("")
	println("  3. ADC Configuration:")
	println("     - Dual ADC mode: Injected Simultaneous")
	println("     - ADC1: Phase A current (via OPAMP1 internal output)")
	println("     - ADC2: Phase B current (via OPAMP2 internal output)")
	println("     - Trigger: TIM1_TRGO2 (rising edge)")
	println("")
	println("  4. Sampling Sequence (every PWM period):")
	println("     a. TIM1 counter reaches center (update event)")
	println("     b. TRGO2 triggers ADC1 and ADC2 simultaneously")
	println("     c. Both ADCs sample Ia and Ib at the same instant")
	println("     d. End-of-injected-sequence interrupt fires")
	println("     e. FOC algorithm reads currents and calculates new duty cycles")
	println("")

	// Demonstrate the configuration
	println("  Demo: Configuring system...")

	// This would be the actual configuration for motor control:
	/*
		// 1. Configure OPAMPs as PGA for current sensing
		machine.OPAMP1.ConfigurePGA(
			machine.OPAMPVPSelVINP0,
			machine.OPAMPPGAGain8,
			true, // Internal output to ADC
		)
		machine.OPAMP2.ConfigurePGA(
			machine.OPAMPVPSelVINP0,
			machine.OPAMPPGAGain8,
			true,
		)

		// 2. Configure TIM1 for center-aligned PWM with TRGO2
		// (See pwm-advanced example)

		// 3. Configure ADCs for injected simultaneous mode
		machine.ConfigureInjectedSimultaneous(
			machine.ADCInjectedConfig{
				Channels:    []uint8{machine.ADC1_VOPAMP1}, // OPAMP1 internal
				Trigger:     machine.ADCInjTrigTIM1_TRGO2,
				TriggerEdge: machine.ADCTriggerRising,
				SampleTime:  machine.ADCSampleTime12_5,     // Fast sampling
			},
			machine.ADCInjectedConfig{
				Channels:    []uint8{machine.ADC2_OPAMP2},  // OPAMP2 internal
				Trigger:     machine.ADCInjTrigTIM1_TRGO2,
				TriggerEdge: machine.ADCTriggerRising,
				SampleTime:  machine.ADCSampleTime12_5,
			},
		)

		// 4. Enable injected interrupt
		machine.ADC1Periph.EnableInjectedInterrupt()
	*/

	println("  Configuration complete!")
	println("")
	println("  Note: Full motor control requires:")
	println("  - Timer PWM configuration (see pwm-advanced example)")
	println("  - CORDIC for sin/cos in Park transform")
	println("  - Proper interrupt handling for FOC loop")
}

// DMA priority constant alias for example usage
const DmaCCR_PL_High = 2 << 12
