package main

// This example demonstrates the OPAMP (Operational Amplifier) peripheral on STM32G4.
//
// The OPAMP can operate in three modes:
// - Standalone: Uses external feedback network for custom gain
// - Follower: Unity-gain buffer (voltage follower)
// - PGA: Programmable Gain Amplifier with internal gain network
//
// Hardware connections (Nucleo-G431KB):
// - OPAMP1 VINP (PA1): Connect analog input signal
// - OPAMP1 VOUT (PA3): Amplified output (or use internal ADC connection)
//
// Note: Input signals should be within the OPAMP input range (VSS to VDD).

import (
	"machine"
	"time"
)

func main() {
	println("OPAMP Operational Amplifier Example")
	println("====================================")
	time.Sleep(time.Second)

	// Example 1: PGA (Programmable Gain Amplifier) mode
	println("")
	println("1. PGA Mode - Gain x4")
	println("---------------------")
	testPGAMode()

	// Example 2: Follower (Buffer) mode
	println("")
	println("2. Follower Mode - Unity Gain Buffer")
	println("------------------------------------")
	testFollowerMode()

	// Example 3: Standalone mode with external feedback
	println("")
	println("3. Standalone Mode Info")
	println("-----------------------")
	testStandaloneInfo()

	// Example 4: Using multiple OPAMPs
	println("")
	println("4. Multiple OPAMPs")
	println("------------------")
	testMultipleOpamps()

	// Example 5: Calibration
	println("")
	println("5. OPAMP Calibration")
	println("--------------------")
	testCalibration()

	println("")
	println("Example complete!")

	for {
		time.Sleep(time.Second)
	}
}

// testPGAMode demonstrates the Programmable Gain Amplifier mode
func testPGAMode() {
	// Configure OPAMP1 as PGA with gain of 4
	// Input on VINP0, output routed internally to ADC
	machine.OPAMP1.ConfigurePGA(
		machine.OPAMPVPSelVINP0, // Use VINP0 input
		machine.OPAMPPGAGain4,   // Gain = 4
		true,                    // Enable internal ADC connection
	)

	println("  OPAMP1 configured as PGA:")
	println("  - Input: VINP0 (PA1)")
	println("  - Gain: x4")
	println("  - Output: Internal to ADC")

	// Check if enabled
	if machine.OPAMP1.IsEnabled() {
		println("  - Status: Enabled")
	}

	// Demonstrate gain switching
	println("")
	println("  Switching gains:")

	gains := []struct {
		val  uint32
		name string
	}{
		{machine.OPAMPPGAGain2, "x2"},
		{machine.OPAMPPGAGain4, "x4"},
		{machine.OPAMPPGAGain8, "x8"},
		{machine.OPAMPPGAGain16, "x16"},
		{machine.OPAMPPGAGain32, "x32"},
		{machine.OPAMPPGAGain64, "x64"},
	}

	for _, g := range gains {
		machine.OPAMP1.SetPGAGain(g.val)
		println("  - Gain set to", g.name)
		time.Sleep(100 * time.Millisecond)
	}

	machine.OPAMP1.Disable()
}

// testFollowerMode demonstrates the voltage follower (buffer) mode
func testFollowerMode() {
	// Configure OPAMP2 as voltage follower
	// Output follows input exactly (unity gain)
	machine.OPAMP2.ConfigureFollower(
		machine.OPAMPVPSelVINP0, // Use VINP0 input
		false,                   // Output to external pin only
	)

	println("  OPAMP2 configured as Follower:")
	println("  - Input: VINP0")
	println("  - Gain: x1 (unity)")
	println("  - Output: External pin")

	// Enable high-speed mode for better bandwidth
	machine.OPAMP2.SetHighSpeed(true)
	println("  - High-speed mode: Enabled")

	time.Sleep(100 * time.Millisecond)

	machine.OPAMP2.Disable()
}

// testStandaloneInfo shows how standalone mode would be configured
func testStandaloneInfo() {
	println("  Standalone mode allows external feedback networks")
	println("  for custom gain configurations.")
	println("")
	println("  Example: Non-inverting amplifier with external resistors")
	println("  - Connect input signal to VINP0")
	println("  - Connect feedback network to VINM0")
	println("  - Gain = 1 + (Rf/Rin)")
	println("")
	println("  Code example:")
	println("    machine.OPAMP1.ConfigureStandalone(")
	println("        machine.OPAMPVPSelVINP0,  // Non-inverting input")
	println("        machine.OPAMPVMSelVINM0,  // Inverting input (feedback)")
	println("        true,                     // Internal ADC output")
	println("    )")
}

// testMultipleOpamps demonstrates using multiple OPAMPs together
func testMultipleOpamps() {
	// Configure OPAMP1 with gain of 2
	machine.OPAMP1.ConfigurePGA(
		machine.OPAMPVPSelVINP0,
		machine.OPAMPPGAGain2,
		true,
	)
	println("  OPAMP1: PGA gain x2")

	// Configure OPAMP2 with gain of 4
	machine.OPAMP2.ConfigurePGA(
		machine.OPAMPVPSelVINP0,
		machine.OPAMPPGAGain4,
		true,
	)
	println("  OPAMP2: PGA gain x4")

	// Configure OPAMP3 with gain of 8
	machine.OPAMP3.ConfigurePGA(
		machine.OPAMPVPSelVINP0,
		machine.OPAMPPGAGain8,
		true,
	)
	println("  OPAMP3: PGA gain x8")

	println("")
	println("  All three OPAMPs running simultaneously")
	println("  for parallel signal conditioning")

	time.Sleep(100 * time.Millisecond)

	// Disable all
	machine.OPAMP1.Disable()
	machine.OPAMP2.Disable()
	machine.OPAMP3.Disable()
}

// testCalibration demonstrates the calibration feature
func testCalibration() {
	println("  Performing OPAMP1 offset calibration...")
	println("  This optimizes offset voltage for best accuracy.")
	println("")

	// Configure as follower for calibration
	machine.OPAMP1.ConfigureFollower(
		machine.OPAMPVPSelVINP0,
		false,
	)

	// Perform calibration
	trimP, trimN := machine.OPAMP1.Calibrate()

	println("  Calibration complete:")
	println("  - PMOS trim value:", int(trimP))
	println("  - NMOS trim value:", int(trimN))

	// Re-enable with calibrated values
	machine.OPAMP1.ConfigureFollower(
		machine.OPAMPVPSelVINP0,
		true,
	)

	println("  OPAMP1 now running with calibrated offsets")

	time.Sleep(100 * time.Millisecond)

	machine.OPAMP1.Disable()
}
