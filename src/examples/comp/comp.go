package main

// This example demonstrates the Comparator (COMP) peripheral on STM32G4.
//
// Comparators compare two analog voltages and produce a digital output.
// Features:
// - Multiple input sources (GPIO, DAC, VREFINT with divider)
// - Programmable hysteresis (0-70mV)
// - Output blanking for PWM noise immunity
// - Window comparator mode using two comparators
//
// Hardware connections (Nucleo-G431KB):
// - COMP1 INP1 (PA1): Connect analog input signal
// - Optional: LED on output pin to visualize comparator state

import (
	"machine"
	"time"
)

func main() {
	println("Comparator (COMP) Peripheral Example")
	println("=====================================")
	time.Sleep(time.Second)

	// Example 1: Simple threshold detection
	println("")
	println("1. Simple Threshold Detection")
	println("------------------------------")
	testSimpleThreshold()

	// Example 2: Hysteresis comparison
	println("")
	println("2. Hysteresis Comparison")
	println("------------------------")
	testHysteresis()

	// Example 3: Window comparator
	println("")
	println("3. Window Comparator")
	println("--------------------")
	testWindowComparator()

	// Example 4: Output blanking for PWM
	println("")
	println("4. Output Blanking")
	println("------------------")
	testBlanking()

	// Example 5: Reading comparator output
	println("")
	println("5. Continuous Reading")
	println("---------------------")
	testContinuousReading()

	println("")
	println("Example complete!")

	for {
		time.Sleep(time.Second)
	}
}

// testSimpleThreshold demonstrates basic threshold detection
func testSimpleThreshold() {
	// Compare input against 1/2 VREFINT (~0.6V at VREFINT=1.21V)
	machine.COMP1.ConfigureWithThreshold(
		machine.COMPInpSelINP1,       // Input on INP1 pin
		machine.COMPInmSel1_2VREFINT, // Compare against 1/2 VREFINT
		machine.COMPHystNone,         // No hysteresis
	)

	println("  COMP1 configured for threshold detection:")
	println("  - Input: INP1 (PA1)")
	println("  - Threshold: 1/2 VREFINT (~0.6V)")
	println("  - Hysteresis: None")
	println("")

	// Read output for a few samples
	println("  Reading output state:")
	for i := 0; i < 5; i++ {
		if machine.COMP1.Output() {
			println("    Input > Threshold (HIGH)")
		} else {
			println("    Input <= Threshold (LOW)")
		}
		time.Sleep(200 * time.Millisecond)
	}

	machine.COMP1.Disable()
}

// testHysteresis demonstrates the hysteresis feature
func testHysteresis() {
	// Configure with 30mV hysteresis to prevent oscillation at threshold
	machine.COMP2.Configure(machine.COMPConfig{
		InvertingInput:      machine.COMPInmSel1_2VREFINT,
		NonInvertingInput:   machine.COMPInpSelINP1,
		Hysteresis:          machine.COMPHyst30mV,
		EnableVREFINTScaler: true,
		EnableVREFINTBridge: true,
	})

	println("  COMP2 configured with hysteresis:")
	println("  - Hysteresis: 30mV")
	println("  - This prevents output oscillation when input")
	println("    is near the threshold voltage.")
	println("")
	println("  Available hysteresis levels:")
	println("    COMPHystNone  - 0mV")
	println("    COMPHyst10mV  - 10mV")
	println("    COMPHyst20mV  - 20mV")
	println("    COMPHyst30mV  - 30mV")
	println("    COMPHyst40mV  - 40mV")
	println("    COMPHyst50mV  - 50mV")
	println("    COMPHyst60mV  - 60mV")
	println("    COMPHyst70mV  - 70mV")

	time.Sleep(100 * time.Millisecond)

	machine.COMP2.Disable()
}

// testWindowComparator demonstrates using two comparators as a window detector
func testWindowComparator() {
	// Set up window: 1/4 VREFINT to 3/4 VREFINT
	// Output is HIGH when input is within this window

	println("  Configuring window comparator:")
	println("  - Low threshold: 1/4 VREFINT (~0.3V)")
	println("  - High threshold: 3/4 VREFINT (~0.9V)")
	println("")

	// Configure window using COMP1 (low) and COMP2 (high)
	machine.COMP1.ConfigureWindow(
		&machine.COMP2,
		machine.COMPInpSelINP1,       // Input pin
		machine.COMPInmSel1_4VREFINT, // Low threshold
		machine.COMPInmSel3_4VREFINT, // High threshold
	)

	println("  Reading window state:")
	for i := 0; i < 5; i++ {
		inWindow := machine.COMP1.WindowOutput(&machine.COMP2)
		lowState := machine.COMP1.Output()
		highState := machine.COMP2.Output()

		if inWindow {
			println("    Input IN window (above low, below high)")
		} else if !lowState {
			println("    Input BELOW window")
		} else if highState {
			println("    Input ABOVE window")
		}

		time.Sleep(200 * time.Millisecond)
	}

	machine.COMP1.Disable()
	machine.COMP2.Disable()
}

// testBlanking demonstrates output blanking for PWM applications
func testBlanking() {
	println("  Output blanking masks comparator output during")
	println("  specific timer events to prevent false triggers")
	println("  from PWM switching noise.")
	println("")
	println("  Available blanking sources:")
	println("    COMPBlankNone     - No blanking")
	println("    COMPBlankTIM1OC5  - TIM1 OC5")
	println("    COMPBlankTIM2OC3  - TIM2 OC3")
	println("    COMPBlankTIM3OC3  - TIM3 OC3")
	println("    COMPBlankTIM8OC5  - TIM8 OC5")
	println("    COMPBlankTIM15OC1 - TIM15 OC1")
	println("")

	// Configure comparator with blanking
	machine.COMP3.Configure(machine.COMPConfig{
		InvertingInput:      machine.COMPInmSel1_2VREFINT,
		NonInvertingInput:   machine.COMPInpSelINP1,
		Hysteresis:          machine.COMPHyst10mV,
		Blanking:            machine.COMPBlankTIM1OC5, // Blank during TIM1 OC5
		EnableVREFINTScaler: true,
		EnableVREFINTBridge: true,
	})

	println("  COMP3 configured with TIM1 OC5 blanking")
	println("  (Timer must be configured separately)")

	time.Sleep(100 * time.Millisecond)

	machine.COMP3.Disable()
}

// testContinuousReading demonstrates continuous comparator monitoring
func testContinuousReading() {
	// Configure COMP4 for simple threshold
	machine.COMP4.ConfigureSimple(
		machine.COMPInmSel1_2VREFINT,
		machine.COMPInpSelINP1,
	)

	println("  COMP4 monitoring input for 3 seconds...")
	println("  (Apply varying voltage to input pin)")
	println("")

	prevState := machine.COMP4.Output()
	transitions := 0

	start := time.Now()
	for time.Since(start) < 3*time.Second {
		state := machine.COMP4.Output()
		if state != prevState {
			transitions++
			if state {
				println("    Transition: LOW -> HIGH")
			} else {
				println("    Transition: HIGH -> LOW")
			}
			prevState = state
		}
		time.Sleep(10 * time.Millisecond)
	}

	println("")
	println("  Total transitions detected:", transitions)

	machine.COMP4.Disable()
}
