package main

// This example demonstrates the FMAC (Filter Math Accelerator) available on STM32G4.
//
// The FMAC provides hardware-accelerated FIR and IIR digital filters:
// - FIR: y[n] = b0*x[n] + b1*x[n-1] + ... + bN*x[n-N]
// - IIR: y[n] = b0*x[n] + ... + bN*x[n-N] - a1*y[n-1] - ... - aM*y[n-M]
//
// These filters are useful for signal processing, sensor data smoothing,
// audio processing, and motor control applications.

import (
	"machine"
	"time"
)

func main() {
	println("FMAC Filter Math Accelerator Example")
	println("=====================================")
	time.Sleep(time.Second)

	// Enable the FMAC peripheral
	machine.Fmac.Enable()
	println("FMAC enabled")
	println("")

	// Example 1: Simple 4-tap averaging filter (FIR)
	println("1. Simple 4-Tap Averaging Filter (FIR)")
	println("---------------------------------------")
	testAveragingFilter()

	// Example 2: Low-pass filter
	println("")
	println("2. 8-Tap Low-Pass Filter")
	println("-------------------------")
	testLowPassFilter()

	// Example 3: Performance test
	println("")
	println("3. Performance Test")
	println("-------------------")
	testPerformance()

	println("")
	println("Example complete!")

	for {
		time.Sleep(time.Second)
	}
}

// testAveragingFilter demonstrates a simple 4-tap moving average filter
func testAveragingFilter() {
	// Reset FMAC before configuration
	machine.Fmac.Reset()

	// Memory layout for 4-tap FIR:
	// X1 (input samples): address 0, size 4+4=8 (need P extra samples for history)
	// X2 (coefficients): address 8, size 4
	// Y (output): address 12, size 4
	machine.Fmac.ConfigureBuffers(machine.FMACBufferConfig{
		X1Base: 0,
		X1Size: 8,
		X2Base: 8,
		X2Size: 4,
		YBase:  12,
		YSize:  4,
	})

	// Load coefficients: 4-tap averaging filter = [0.25, 0.25, 0.25, 0.25]
	// In q1.15: 0.25 * 32768 = 8192 = 0x2000
	coeffs := []int16{8192, 8192, 8192, 8192}
	machine.Fmac.LoadX2Buffer(coeffs)

	// Preload X1 buffer with zeros (initial state)
	zeros := []int16{0, 0, 0, 0}
	machine.Fmac.LoadX1Buffer(zeros)

	// Start FIR filter
	machine.Fmac.StartFIR(machine.FMACFilterConfig{
		P:        4, // 4 taps
		Clipping: true,
	})

	// Process some test samples
	// Input: step function [0, 0, 0, 0, 16384, 16384, 16384, 16384]
	// Expected output: gradually rises [0, 4096, 8192, 12288, 16384, 16384, 16384, 16384]
	inputs := []int16{0, 0, 16384, 16384, 16384, 16384, 16384, 16384}

	println("  Input -> Output (q1.15 values)")
	for _, input := range inputs {
		machine.Fmac.WriteInputBlocking(input)

		// Small delay to let filter process
		for i := 0; i < 10; i++ {
		}

		output, ok := machine.Fmac.ReadOutput()
		if ok {
			println("  ", input, " -> ", output)
		}
	}

	machine.Fmac.Stop()
}

// testLowPassFilter demonstrates an 8-tap low-pass FIR filter
func testLowPassFilter() {
	machine.Fmac.Reset()

	// Memory layout for 8-tap FIR
	machine.Fmac.ConfigureBuffers(machine.FMACBufferConfig{
		X1Base: 0,
		X1Size: 16, // 8 + 8 for history
		X2Base: 16,
		X2Size: 8,
		YBase:  24,
		YSize:  8,
	})

	// Simple 8-tap low-pass filter coefficients (Hamming window)
	// These values are pre-scaled for q1.15
	// Normalized so sum = 1.0 (sum of q1.15 values = 32768)
	coeffs := []int16{
		1024, // h[0]
		2048, // h[1]
		4096, // h[2]
		6144, // h[3]
		6144, // h[4]
		4096, // h[5]
		2048, // h[6]
		1024, // h[7] - note: coefficients are symmetric
	}
	machine.Fmac.LoadX2Buffer(coeffs)

	// Preload with zeros
	zeros := make([]int16, 8)
	machine.Fmac.LoadX1Buffer(zeros)

	// Start filter
	machine.Fmac.StartFIR(machine.FMACFilterConfig{
		P:        8,
		Clipping: true,
	})

	// Generate a test signal: square wave
	println("  Processing square wave...")
	println("  Input -> Output")

	for i := 0; i < 16; i++ {
		// Square wave: alternate between +0.5 and -0.5
		var input int16
		if (i/4)%2 == 0 {
			input = 16384 // +0.5 in q1.15
		} else {
			input = -16384 // -0.5 in q1.15
		}

		machine.Fmac.WriteInputBlocking(input)

		// Wait and read output
		for j := 0; j < 10; j++ {
		}

		if output, ok := machine.Fmac.ReadOutput(); ok {
			// Convert to approximate percentage for display
			inputPct := int32(input) * 100 / 32768
			outputPct := int32(output) * 100 / 32768
			println("  ", inputPct, "% -> ", outputPct, "%")
		}
	}

	machine.Fmac.Stop()
}

// testPerformance measures FMAC filter calculation time
func testPerformance() {
	const iterations = 1000

	machine.Fmac.Reset()

	// Setup simple 4-tap filter for performance test
	machine.Fmac.ConfigureBuffers(machine.FMACBufferConfig{
		X1Base: 0,
		X1Size: 64, // Larger buffer for continuous processing
		X2Base: 64,
		X2Size: 4,
		YBase:  68,
		YSize:  64,
	})

	// Load coefficients
	coeffs := []int16{8192, 8192, 8192, 8192}
	machine.Fmac.LoadX2Buffer(coeffs)

	// Preload
	zeros := make([]int16, 4)
	machine.Fmac.LoadX1Buffer(zeros)

	// Start filter
	machine.Fmac.StartFIR(machine.FMACFilterConfig{
		P:        4,
		Clipping: true,
	})

	// Time the filter processing
	start := time.Now()
	var lastOutput int16
	for i := 0; i < iterations; i++ {
		machine.Fmac.WriteInputBlocking(int16(i * 10))
		if out, ok := machine.Fmac.ReadOutput(); ok {
			lastOutput = out
		}
	}
	elapsed := time.Since(start)

	// Prevent optimization
	_ = lastOutput

	machine.Fmac.Stop()

	println("  ", iterations, "filter samples in", int64(elapsed.Microseconds()), "us")
	println("  Average:", int64(elapsed.Nanoseconds())/iterations, "ns per sample")
}
