//go:build stm32g4

package machine

// CORDIC (COordinate Rotation DIgital Computer) coprocessor driver
//
// The CORDIC provides hardware acceleration for trigonometric and hyperbolic
// mathematical functions commonly used in motor control, signal processing,
// and other applications.
//
// Supported functions:
// - Trigonometric: sin, cos, phase (atan2), modulus (magnitude), atan
// - Hyperbolic: sinh, cosh, atanh
// - Other: natural logarithm, square root
//
// All values use q1.31 fixed-point format where the range [-1, 1) maps to
// [0x80000000, 0x7FFFFFFF]. For angles, values must be divided by π first
// (so [-π, π] becomes [-1, 1]).
//
// Reference: RM0440 Section 17 - CORDIC coprocessor

import (
	"device/stm32"
)

// CORDIC function codes
const (
	CordicFuncCosine  = 0 // cos(θ), also returns sin(θ) as secondary result
	CordicFuncSine    = 1 // sin(θ), also returns cos(θ) as secondary result
	CordicFuncPhase   = 2 // atan2(y,x), also returns modulus as secondary result
	CordicFuncModulus = 3 // sqrt(x²+y²), also returns phase as secondary result
	CordicFuncArctan  = 4 // atan(x)
	CordicFuncCosh    = 5 // cosh(x), also returns sinh(x) as secondary result
	CordicFuncSinh    = 6 // sinh(x), also returns cosh(x) as secondary result
	CordicFuncAtanh   = 7 // atanh(x)
	CordicFuncLn      = 8 // ln(x)
	CordicFuncSqrt    = 9 // sqrt(x)
)

// CORDIC precision levels (number of iterations / 4)
// Higher precision = more accurate but slower
const (
	CordicPrecision4Iter  = 1 // 4 iterations, 1 cycle, ~2^-3 error
	CordicPrecision8Iter  = 2 // 8 iterations, 2 cycles, ~2^-7 error
	CordicPrecision12Iter = 3 // 12 iterations, 3 cycles, ~2^-11 error
	CordicPrecision16Iter = 4 // 16 iterations, 4 cycles, ~2^-15 error (recommended for q1.15)
	CordicPrecision20Iter = 5 // 20 iterations, 5 cycles, ~2^-18 error
	CordicPrecision24Iter = 6 // 24 iterations, 6 cycles, ~2^-19 error (recommended for q1.31)
)

// CORDIC represents the CORDIC coprocessor peripheral
type CORDIC struct {
	Device *stm32.CORDIC_Type
}

// Global CORDIC instance
var Cordic = CORDIC{Device: stm32.CORDIC}

// CordicConfig holds configuration for a CORDIC operation
type CordicConfig struct {
	Function  uint8 // Function to compute (CordicFunc* constants)
	Precision uint8 // Precision level (CordicPrecision* constants), 1-15
	Scale     uint8 // Scale factor for extended range (0-7)
}

// Enable enables the CORDIC peripheral clock
func (c *CORDIC) Enable() {
	stm32.RCC.AHB1ENR.SetBits(stm32.RCC_AHB1ENR_CORDICEN)
}

// Disable disables the CORDIC peripheral clock
func (c *CORDIC) Disable() {
	stm32.RCC.AHB1ENR.ClearBits(stm32.RCC_AHB1ENR_CORDICEN)
}

// Configure sets up the CORDIC for a specific function
// The configuration remains valid until changed, so multiple calculations
// of the same function can be performed without reconfiguring
func (c *CORDIC) Configure(config CordicConfig) {
	// Ensure clock is enabled
	c.Enable()

	// Build CSR value
	// FUNC[3:0] = config.Function
	// PRECISION[7:4] = config.Precision
	// SCALE[10:8] = config.Scale
	// NARGS = 0 (single argument mode by default)
	// NRES = 0 (single result mode by default)
	// ARGSIZE = 0 (32-bit arguments)
	// RESSIZE = 0 (32-bit results)
	csr := uint32(config.Function) |
		(uint32(config.Precision) << 4) |
		(uint32(config.Scale) << 8)

	c.Device.CSR.Set(csr)
}

// ConfigureTwoArgs sets up CORDIC for functions requiring two arguments
func (c *CORDIC) ConfigureTwoArgs(config CordicConfig) {
	c.Enable()

	// Same as Configure but with NARGS=1 for two arguments
	csr := uint32(config.Function) |
		(uint32(config.Precision) << 4) |
		(uint32(config.Scale) << 8) |
		(1 << 20) // NARGS = 1

	c.Device.CSR.Set(csr)
}

// ConfigureTwoResults sets up CORDIC for functions returning two results
func (c *CORDIC) ConfigureTwoResults(config CordicConfig) {
	c.Enable()

	// Same as Configure but with NRES=1 for two results
	csr := uint32(config.Function) |
		(uint32(config.Precision) << 4) |
		(uint32(config.Scale) << 8) |
		(1 << 19) // NRES = 1

	c.Device.CSR.Set(csr)
}

// ConfigureTwoArgsResults sets up CORDIC for functions with two args and two results
func (c *CORDIC) ConfigureTwoArgsResults(config CordicConfig) {
	c.Enable()

	csr := uint32(config.Function) |
		(uint32(config.Precision) << 4) |
		(uint32(config.Scale) << 8) |
		(1 << 19) | // NRES = 1
		(1 << 20) // NARGS = 1

	c.Device.CSR.Set(csr)
}

// Calculate performs a single-argument, single-result calculation
// Uses zero-overhead mode: write triggers calculation, read waits for result
func (c *CORDIC) Calculate(arg int32) int32 {
	c.Device.WDATA.Set(uint32(arg))
	return int32(c.Device.RDATA.Get())
}

// Calculate2Args performs a two-argument, single-result calculation
func (c *CORDIC) Calculate2Args(arg1, arg2 int32) int32 {
	c.Device.WDATA.Set(uint32(arg1))
	c.Device.WDATA.Set(uint32(arg2))
	return int32(c.Device.RDATA.Get())
}

// Calculate2Results performs a single-argument calculation returning two results
func (c *CORDIC) Calculate2Results(arg int32) (res1, res2 int32) {
	c.Device.WDATA.Set(uint32(arg))
	res1 = int32(c.Device.RDATA.Get())
	res2 = int32(c.Device.RDATA.Get())
	return
}

// Calculate2Args2Results performs a two-argument calculation returning two results
func (c *CORDIC) Calculate2Args2Results(arg1, arg2 int32) (res1, res2 int32) {
	c.Device.WDATA.Set(uint32(arg1))
	c.Device.WDATA.Set(uint32(arg2))
	res1 = int32(c.Device.RDATA.Get())
	res2 = int32(c.Device.RDATA.Get())
	return
}

// IsReady returns true if a result is available
func (c *CORDIC) IsReady() bool {
	return c.Device.CSR.Get()&(1<<31) != 0 // RRDY flag
}

// ============================================================================
// Fixed-Point Conversion Utilities
// ============================================================================

// Q31Max is the maximum positive value in q1.31 format (represents ~1.0)
const Q31Max = int32(0x7FFFFFFF)

// Q31Min is the minimum negative value in q1.31 format (represents -1.0)
const Q31Min = int32(-0x80000000)

// FloatToQ31 converts a float32 in range [-1, 1] to q1.31 fixed-point
// Values outside the range will be clamped
func FloatToQ31(f float32) int32 {
	if f >= 1.0 {
		return Q31Max
	}
	if f <= -1.0 {
		return Q31Min
	}
	return int32(f * float32(0x80000000))
}

// Q31ToFloat converts a q1.31 fixed-point value to float32
func Q31ToFloat(q int32) float32 {
	return float32(q) / float32(0x80000000)
}

// AngleToQ31 converts an angle in radians to q1.31 format
// The angle is divided by π, so [-π, π] maps to [-1, 1]
func AngleToQ31(radians float32) int32 {
	return FloatToQ31(radians / 3.14159265358979323846)
}

// Q31ToAngle converts a q1.31 value back to radians
// Multiplies by π to convert from [-1, 1] to [-π, π]
func Q31ToAngle(q int32) float32 {
	return Q31ToFloat(q) * 3.14159265358979323846
}

// ============================================================================
// High-Level Functions (convenience wrappers)
// ============================================================================

// Sin computes the sine of an angle in radians
// Returns sin(θ) as a float32 in range [-1, 1]
func (c *CORDIC) Sin(radians float32) float32 {
	c.Configure(CordicConfig{
		Function:  CordicFuncSine,
		Precision: CordicPrecision20Iter,
		Scale:     0,
	})
	arg := AngleToQ31(radians)
	// Write angle, then write modulus (1.0 = 0x7FFFFFFF)
	c.Device.WDATA.Set(uint32(arg))
	result := int32(c.Device.RDATA.Get())
	return Q31ToFloat(result)
}

// Cos computes the cosine of an angle in radians
// Returns cos(θ) as a float32 in range [-1, 1]
func (c *CORDIC) Cos(radians float32) float32 {
	c.Configure(CordicConfig{
		Function:  CordicFuncCosine,
		Precision: CordicPrecision20Iter,
		Scale:     0,
	})
	arg := AngleToQ31(radians)
	c.Device.WDATA.Set(uint32(arg))
	result := int32(c.Device.RDATA.Get())
	return Q31ToFloat(result)
}

// SinCos computes both sine and cosine of an angle in radians
// More efficient than calling Sin and Cos separately
func (c *CORDIC) SinCos(radians float32) (sin, cos float32) {
	c.ConfigureTwoResults(CordicConfig{
		Function:  CordicFuncSine,
		Precision: CordicPrecision20Iter,
		Scale:     0,
	})
	arg := AngleToQ31(radians)
	c.Device.WDATA.Set(uint32(arg))
	sinQ := int32(c.Device.RDATA.Get())
	cosQ := int32(c.Device.RDATA.Get())
	return Q31ToFloat(sinQ), Q31ToFloat(cosQ)
}

// Atan2 computes the arctangent of y/x, returning the angle in radians
// Handles all four quadrants correctly
// x, y should be in range [-1, 1]
func (c *CORDIC) Atan2(y, x float32) float32 {
	c.ConfigureTwoArgs(CordicConfig{
		Function:  CordicFuncPhase,
		Precision: CordicPrecision20Iter,
		Scale:     0,
	})
	xQ := FloatToQ31(x)
	yQ := FloatToQ31(y)
	c.Device.WDATA.Set(uint32(xQ))
	c.Device.WDATA.Set(uint32(yQ))
	result := int32(c.Device.RDATA.Get())
	return Q31ToAngle(result)
}

// Magnitude computes the magnitude (modulus) of a vector: sqrt(x² + y²)
// x, y should be in range [-1, 1]
// Result saturates to 1.0 if magnitude exceeds 1
func (c *CORDIC) Magnitude(x, y float32) float32 {
	c.ConfigureTwoArgs(CordicConfig{
		Function:  CordicFuncModulus,
		Precision: CordicPrecision20Iter,
		Scale:     0,
	})
	xQ := FloatToQ31(x)
	yQ := FloatToQ31(y)
	c.Device.WDATA.Set(uint32(xQ))
	c.Device.WDATA.Set(uint32(yQ))
	result := int32(c.Device.RDATA.Get())
	return Q31ToFloat(result)
}

// Atan computes the arctangent of x, returning the angle in radians
// x should be in range [-1, 1] (use scale parameter for larger values)
func (c *CORDIC) Atan(x float32) float32 {
	c.Configure(CordicConfig{
		Function:  CordicFuncArctan,
		Precision: CordicPrecision20Iter,
		Scale:     0,
	})
	xQ := FloatToQ31(x)
	c.Device.WDATA.Set(uint32(xQ))
	result := int32(c.Device.RDATA.Get())
	return Q31ToAngle(result)
}

// Sqrt computes the square root of x
// x should be in range [0.027, 0.75] for scale=0
// Use scale parameter for larger values (up to 2.34 with scale=2)
func (c *CORDIC) Sqrt(x float32) float32 {
	// Determine appropriate scale factor
	var scale uint8 = 0
	scaledX := x
	if x >= 1.75 {
		scale = 2
		scaledX = x / 4.0
	} else if x >= 0.75 {
		scale = 1
		scaledX = x / 2.0
	}

	c.Configure(CordicConfig{
		Function:  CordicFuncSqrt,
		Precision: CordicPrecision12Iter, // Sqrt converges faster
		Scale:     scale,
	})
	xQ := FloatToQ31(scaledX)
	c.Device.WDATA.Set(uint32(xQ))
	result := int32(c.Device.RDATA.Get())

	// Scale result back
	r := Q31ToFloat(result)
	for i := uint8(0); i < scale; i++ {
		r *= 2.0
	}
	return r
}

// Sinh computes the hyperbolic sine of x
// x should be in range [-1.118, 1.118]
func (c *CORDIC) Sinh(x float32) float32 {
	// Scale = 1 required for hyperbolic functions
	scaledX := x / 2.0
	c.Configure(CordicConfig{
		Function:  CordicFuncSinh,
		Precision: CordicPrecision20Iter,
		Scale:     1,
	})
	xQ := FloatToQ31(scaledX)
	c.Device.WDATA.Set(uint32(xQ))
	result := int32(c.Device.RDATA.Get())
	return Q31ToFloat(result) * 2.0 // Undo scaling
}

// Cosh computes the hyperbolic cosine of x
// x should be in range [-1.118, 1.118]
func (c *CORDIC) Cosh(x float32) float32 {
	scaledX := x / 2.0
	c.Configure(CordicConfig{
		Function:  CordicFuncCosh,
		Precision: CordicPrecision20Iter,
		Scale:     1,
	})
	xQ := FloatToQ31(scaledX)
	c.Device.WDATA.Set(uint32(xQ))
	result := int32(c.Device.RDATA.Get())
	return Q31ToFloat(result) * 2.0
}

// SinhCosh computes both hyperbolic sine and cosine
// More efficient than calling Sinh and Cosh separately
func (c *CORDIC) SinhCosh(x float32) (sinh, cosh float32) {
	scaledX := x / 2.0
	c.ConfigureTwoResults(CordicConfig{
		Function:  CordicFuncSinh,
		Precision: CordicPrecision20Iter,
		Scale:     1,
	})
	xQ := FloatToQ31(scaledX)
	c.Device.WDATA.Set(uint32(xQ))
	sinhQ := int32(c.Device.RDATA.Get())
	coshQ := int32(c.Device.RDATA.Get())
	return Q31ToFloat(sinhQ) * 2.0, Q31ToFloat(coshQ) * 2.0
}

// Ln computes the natural logarithm of x
// x should be in range [0.107, 9.35]
func (c *CORDIC) Ln(x float32) float32 {
	// Determine appropriate scale factor based on input range
	var scale uint8
	var scaledX float32

	switch {
	case x < 1.0:
		scale = 1
		scaledX = x / 2.0
	case x < 3.0:
		scale = 2
		scaledX = x / 4.0
	case x < 7.0:
		scale = 3
		scaledX = x / 8.0
	default:
		scale = 4
		scaledX = x / 16.0
	}

	c.Configure(CordicConfig{
		Function:  CordicFuncLn,
		Precision: CordicPrecision20Iter,
		Scale:     scale,
	})
	xQ := FloatToQ31(scaledX)
	c.Device.WDATA.Set(uint32(xQ))
	result := int32(c.Device.RDATA.Get())

	// Result needs to be multiplied by 2^(n+1)
	r := Q31ToFloat(result)
	for i := uint8(0); i <= scale; i++ {
		r *= 2.0
	}
	return r
}

// Atanh computes the hyperbolic arctangent of x
// x should be in range [-0.806, 0.806]
func (c *CORDIC) Atanh(x float32) float32 {
	scaledX := x / 2.0
	c.Configure(CordicConfig{
		Function:  CordicFuncAtanh,
		Precision: CordicPrecision20Iter,
		Scale:     1,
	})
	xQ := FloatToQ31(scaledX)
	c.Device.WDATA.Set(uint32(xQ))
	result := int32(c.Device.RDATA.Get())
	return Q31ToFloat(result) * 2.0
}

// ============================================================================
// Low-Level Q31 Functions (for performance-critical code)
// ============================================================================

// SinQ31 computes sine using q1.31 fixed-point directly
// Input angle should already be divided by π (range [-1, 1] = [-π, π])
// Returns sin(θ*π) in q1.31 format
func (c *CORDIC) SinQ31(angleQ31 int32) int32 {
	c.ConfigureTwoArgs(CordicConfig{
		Function:  CordicFuncSine,
		Precision: CordicPrecision20Iter,
		Scale:     0,
	})
	// Write angle and modulus (1.0 = Q31Max)
	return c.Calculate2Args(angleQ31, Q31Max)
}

// CosQ31 computes cosine using q1.31 fixed-point directly
func (c *CORDIC) CosQ31(angleQ31 int32) int32 {
	c.ConfigureTwoArgs(CordicConfig{
		Function:  CordicFuncCosine,
		Precision: CordicPrecision20Iter,
		Scale:     0,
	})
	// Write angle and modulus (1.0 = Q31Max)
	return c.Calculate2Args(angleQ31, Q31Max)
}

// SinCosQ31 computes both sine and cosine in q1.31 format
func (c *CORDIC) SinCosQ31(angleQ31 int32) (sinQ31, cosQ31 int32) {
	c.ConfigureTwoArgsResults(CordicConfig{
		Function:  CordicFuncSine,
		Precision: CordicPrecision20Iter,
		Scale:     0,
	})
	// Write angle and modulus (1.0 = Q31Max)
	return c.Calculate2Args2Results(angleQ31, Q31Max)
}

// Atan2Q31 computes atan2(y,x) in q1.31 format
// Returns angle/π in q1.31 format
func (c *CORDIC) Atan2Q31(xQ31, yQ31 int32) int32 {
	c.ConfigureTwoArgs(CordicConfig{
		Function:  CordicFuncPhase,
		Precision: CordicPrecision20Iter,
		Scale:     0,
	})
	return c.Calculate2Args(xQ31, yQ31)
}

// MagnitudeQ31 computes sqrt(x²+y²) in q1.31 format
func (c *CORDIC) MagnitudeQ31(xQ31, yQ31 int32) int32 {
	c.ConfigureTwoArgs(CordicConfig{
		Function:  CordicFuncModulus,
		Precision: CordicPrecision20Iter,
		Scale:     0,
	})
	return c.Calculate2Args(xQ31, yQ31)
}
