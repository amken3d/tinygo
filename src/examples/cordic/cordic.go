package main

// This example demonstrates the CORDIC coprocessor available on STM32G4.
//
// The CORDIC provides hardware-accelerated computation of:
// - Trigonometric functions: sin, cos, atan, atan2
// - Hyperbolic functions: sinh, cosh, atanh
// - Other: natural logarithm, square root, magnitude
//
// These functions are useful for motor control (FOC), signal processing,
// and other applications requiring fast math operations.

import (
	"machine"
	"time"
)

func main() {
	println("CORDIC Coprocessor Example")
	println("==========================")
	time.Sleep(time.Second)

	// Enable the CORDIC peripheral
	machine.Cordic.Enable()
	println("CORDIC enabled")
	println("")

	// Example 1: Trigonometric functions
	println("1. Trigonometric Functions")
	println("--------------------------")
	testTrigFunctions()

	// Example 2: Vector operations (useful for motor control)
	println("")
	println("2. Vector Operations")
	println("--------------------")
	testVectorOperations()

	// Example 3: Performance comparison
	println("")
	println("3. Performance Test")
	println("-------------------")
	testPerformance()

	// Example 4: Low-level Q31 operations
	println("")
	println("4. Low-Level Q31 Operations")
	println("---------------------------")
	testQ31Operations()

	println("")
	println("Example complete!")

	for {
		time.Sleep(time.Second)
	}
}

// testTrigFunctions demonstrates basic trigonometric calculations
func testTrigFunctions() {
	// Test angles: 0, 30, 45, 60, 90 degrees
	angles := []float32{0, 0.523599, 0.785398, 1.047198, 1.570796}
	names := []string{"0", "30", "45", "60", "90"}

	for i, angle := range angles {
		sin := machine.Cordic.Sin(angle)
		cos := machine.Cordic.Cos(angle)
		println("  ", names[i], "deg: sin=", formatFloat(sin), " cos=", formatFloat(cos))
	}

	// SinCos is more efficient when you need both
	println("")
	println("  Using SinCos (more efficient):")
	sin, cos := machine.Cordic.SinCos(0.785398) // 45 degrees
	println("  45 deg: sin=", formatFloat(sin), " cos=", formatFloat(cos))
}

// testVectorOperations demonstrates vector math useful for motor control
func testVectorOperations() {
	// Test vectors
	vectors := [][2]float32{
		{1.0, 0.0},     // 0 degrees
		{0.707, 0.707}, // 45 degrees
		{0.0, 1.0},     // 90 degrees
		{-0.5, 0.866},  // 120 degrees
	}

	for _, v := range vectors {
		x, y := v[0], v[1]
		mag := machine.Cordic.Magnitude(x, y)
		phase := machine.Cordic.Atan2(y, x)
		println("  Vector (", formatFloat(x), ",", formatFloat(y), "): mag=", formatFloat(mag), " phase=", formatFloat(phase), "rad")
	}

	// Arctangent
	println("")
	println("  Arctangent:")
	atan := machine.Cordic.Atan(0.5)
	println("  atan(0.5) =", formatFloat(atan), "rad")
}

// testPerformance measures CORDIC calculation time
func testPerformance() {
	const iterations = 1000

	// Pre-configure for sine calculation
	machine.Cordic.Configure(machine.CordicConfig{
		Function:  machine.CordicFuncSine,
		Precision: machine.CordicPrecision20Iter,
		Scale:     0,
	})

	// Time multiple calculations
	start := time.Now()
	var result int32
	for i := 0; i < iterations; i++ {
		// Use low-level function for maximum speed
		result = machine.Cordic.Calculate(int32(i * 1000))
	}
	elapsed := time.Since(start)

	// Prevent optimization from removing the loop
	_ = result

	println("  ", iterations, "sine calculations in", int64(elapsed.Microseconds()), "us")
	println("  Average:", int64(elapsed.Nanoseconds())/iterations, "ns per calculation")
}

// testQ31Operations demonstrates low-level fixed-point operations
func testQ31Operations() {
	// Q31 format: [-1, 1) maps to [0x80000000, 0x7FFFFFFF]
	// For angles: divide by pi, so [-pi, pi] maps to [-1, 1]

	// 45 degrees = pi/4 radians = 0.25 (in angle/pi units)
	// 0.25 in Q31 = 0.25 * 0x80000000 = 0x20000000
	angle45 := int32(0x20000000)

	sin, cos := machine.Cordic.SinCosQ31(angle45)
	println("  45 deg in Q31 format:")
	println("    sin(Q31) = 0x", formatHex(uint32(sin)))
	println("    cos(Q31) = 0x", formatHex(uint32(cos)))

	// Convert back to float for verification
	sinF := machine.Q31ToFloat(sin)
	cosF := machine.Q31ToFloat(cos)
	println("    sin(float) =", formatFloat(sinF), " (expected ~0.707)")
	println("    cos(float) =", formatFloat(cosF), " (expected ~0.707)")
}

// formatFloat formats a float32 with 3 decimal places
func formatFloat(f float32) string {
	// Simple formatting without fmt package
	negative := f < 0
	if negative {
		f = -f
	}
	whole := int(f)
	frac := int((f - float32(whole)) * 1000)
	if frac < 0 {
		frac = -frac
	}

	result := ""
	if negative {
		result = "-"
	}
	result += intToString(whole) + "."
	if frac < 10 {
		result += "00"
	} else if frac < 100 {
		result += "0"
	}
	result += intToString(frac)
	return result
}

// intToString converts an integer to string
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

// formatHex formats a uint32 as hex string
func formatHex(n uint32) string {
	const hexChars = "0123456789ABCDEF"
	result := ""
	for i := 0; i < 8; i++ {
		result = string(hexChars[n&0xF]) + result
		n >>= 4
	}
	return result
}
