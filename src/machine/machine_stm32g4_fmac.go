//go:build stm32g4

package machine

// FMAC (Filter Math Accelerator) peripheral driver
//
// The FMAC provides hardware acceleration for digital filter algorithms:
// - FIR filters (Finite Impulse Response) up to 127 taps
// - IIR filters (Infinite Impulse Response) with configurable feedback
//
// All values use q1.15 fixed-point format where the range [-1, 1) maps to
// [-32768, 32767] (int16).
//
// The FMAC has 256 words of internal memory that can be partitioned between:
// - X1 buffer: Input data samples
// - X2 buffer: Filter coefficients
// - Y buffer: Output data
//
// Reference: RM0440 Section 18 - Filter Math Accelerator

import (
	"device/stm32"
)

// FMAC function codes
const (
	FMACFuncLoadX1 = 1 // Load X1 buffer with data
	FMACFuncLoadX2 = 2 // Load X2 buffer with coefficients
	FMACFuncLoadY  = 3 // Load Y buffer (for IIR initial values)
	FMACFuncFIR    = 8 // FIR filter: y[n] = sum(b[k] * x[n-k])
	FMACFuncIIR    = 9 // IIR filter: y[n] = sum(b[k]*x[n-k]) - sum(a[k]*y[n-k])
)

// FMAC watermark values for interrupt/DMA triggering
const (
	FMACWatermark1 = 0 // Threshold is 1
	FMACWatermark2 = 1 // Threshold is 2
	FMACWatermark4 = 2 // Threshold is 4
	FMACWatermark8 = 3 // Threshold is 8
)

// FMAC represents the FMAC peripheral
type FMAC struct {
	Device *stm32.FMAC_Type
}

// Global FMAC instance
var Fmac = FMAC{Device: stm32.FMAC}

// FMACBufferConfig holds configuration for X1, X2, and Y buffer partitioning
type FMACBufferConfig struct {
	// X1 buffer configuration (input samples)
	X1Base uint8 // Start address in internal memory (0-255)
	X1Size uint8 // Buffer size (1-256, actual stored value is size-1)

	// X2 buffer configuration (filter coefficients)
	X2Base uint8 // Start address in internal memory
	X2Size uint8 // Buffer size (1-256)

	// Y buffer configuration (output)
	YBase uint8 // Start address in internal memory
	YSize uint8 // Buffer size (1-256)
}

// FMACFilterConfig holds configuration for filter operations
type FMACFilterConfig struct {
	P        uint8 // Number of feed-forward taps (FIR coefficients, 2-127)
	Q        uint8 // Number of feedback taps (IIR coefficients, 1-127, use 1 for FIR)
	R        uint8 // Gain factor (output is shifted right by R bits, 0-7)
	Clipping bool  // Enable output clipping/saturation to q1.15 range
	FullWM   uint8 // X1 buffer full watermark (FMACWatermark*)
	EmptyWM  uint8 // Y buffer empty watermark (FMACWatermark*)
}

// Enable enables the FMAC peripheral clock
func (f *FMAC) Enable() {
	stm32.RCC.AHB1ENR.SetBits(stm32.RCC_AHB1ENR_FMACEN)
}

// Disable disables the FMAC peripheral clock
func (f *FMAC) Disable() {
	stm32.RCC.AHB1ENR.ClearBits(stm32.RCC_AHB1ENR_FMACEN)
}

// Reset performs a software reset of the FMAC
// This clears all buffers and registers, and must be called before reconfiguration
func (f *FMAC) Reset() {
	f.Device.CR.SetBits(stm32.FMAC_CR_RESET)
	// Wait for reset to complete (RESET bit auto-clears)
	for f.Device.CR.HasBits(stm32.FMAC_CR_RESET) {
	}
}

// ConfigureBuffers sets up the memory partitioning for X1, X2, and Y buffers
// Total memory is 256 words - ensure buffers don't overlap
func (f *FMAC) ConfigureBuffers(config FMACBufferConfig) {
	// Ensure clock is enabled
	f.Enable()

	// Configure X1 buffer (input samples)
	// X1_BASE[7:0], X1_BUF_SIZE[15:8]
	x1cfg := uint32(config.X1Base) | (uint32(config.X1Size) << 8)
	f.Device.X1BUFCFG.Set(x1cfg)

	// Configure X2 buffer (coefficients)
	// X2_BASE[7:0], X2_BUF_SIZE[15:8]
	x2cfg := uint32(config.X2Base) | (uint32(config.X2Size) << 8)
	f.Device.X2BUFCFG.Set(x2cfg)

	// Configure Y buffer (output)
	// Y_BASE[7:0], Y_BUF_SIZE[15:8]
	ycfg := uint32(config.YBase) | (uint32(config.YSize) << 8)
	f.Device.YBUFCFG.Set(ycfg)
}

// ConfigureBuffersWithWatermarks sets up buffer configuration including watermarks
func (f *FMAC) ConfigureBuffersWithWatermarks(config FMACBufferConfig, fullWM, emptyWM uint8) {
	f.Enable()

	// X1 buffer with FULL_WM
	x1cfg := uint32(config.X1Base) | (uint32(config.X1Size) << 8) | (uint32(fullWM) << 24)
	f.Device.X1BUFCFG.Set(x1cfg)

	// X2 buffer (no watermark)
	x2cfg := uint32(config.X2Base) | (uint32(config.X2Size) << 8)
	f.Device.X2BUFCFG.Set(x2cfg)

	// Y buffer with EMPTY_WM
	ycfg := uint32(config.YBase) | (uint32(config.YSize) << 8) | (uint32(emptyWM) << 24)
	f.Device.YBUFCFG.Set(ycfg)
}

// LoadX1Buffer loads input sample data into the X1 buffer
// Call this before starting filter operation to preload initial samples
// data: array of q1.15 values
func (f *FMAC) LoadX1Buffer(data []int16) {
	f.startLoad(FMACFuncLoadX1, uint8(len(data)), 0)
	for _, v := range data {
		f.writeData(v)
	}
}

// LoadX2Buffer loads filter coefficients into the X2 buffer
// coeffs: array of q1.15 coefficient values
func (f *FMAC) LoadX2Buffer(coeffs []int16) {
	f.startLoad(FMACFuncLoadX2, uint8(len(coeffs)), 0)
	for _, v := range coeffs {
		f.writeData(v)
	}
}

// LoadYBuffer loads initial values into the Y buffer (for IIR filters)
// data: array of q1.15 initial output values
func (f *FMAC) LoadYBuffer(data []int16) {
	f.startLoad(FMACFuncLoadY, uint8(len(data)), 0)
	for _, v := range data {
		f.writeData(v)
	}
}

// startLoad initiates a buffer load operation
func (f *FMAC) startLoad(function, p, q uint8) {
	// Build PARAM register value
	// FUNC[30:24], R[23:16], Q[15:8], P[7:0], START[31]
	param := uint32(function)<<24 | uint32(q)<<8 | uint32(p) | stm32.FMAC_PARAM_START
	f.Device.PARAM.Set(param)
}

// writeData writes a single q1.15 value to WDATA and waits if buffer full
func (f *FMAC) writeData(value int16) {
	// Wait if X1 buffer is full
	for f.Device.SR.HasBits(stm32.FMAC_SR_X1FULL) {
	}
	f.Device.WDATA.Set(uint32(uint16(value)))
}

// StartFIR starts a FIR filter operation
// P: number of filter taps (coefficients)
// The filter continuously processes data written to X1 and outputs to Y
func (f *FMAC) StartFIR(config FMACFilterConfig) {
	// Configure clipping if enabled
	if config.Clipping {
		f.Device.CR.SetBits(stm32.FMAC_CR_CLIPEN)
	} else {
		f.Device.CR.ClearBits(stm32.FMAC_CR_CLIPEN)
	}

	// Build PARAM value for FIR: FUNC=8, Q=1 (required for FIR)
	// FUNC[30:24], R[23:16], Q[15:8], P[7:0], START[31]
	param := uint32(FMACFuncFIR)<<24 |
		uint32(config.R)<<16 |
		uint32(1)<<8 | // Q must be 1 for FIR
		uint32(config.P) |
		stm32.FMAC_PARAM_START

	f.Device.PARAM.Set(param)
}

// StartIIR starts an IIR filter operation
// P: number of feed-forward taps (b coefficients)
// Q: number of feedback taps (a coefficients)
func (f *FMAC) StartIIR(config FMACFilterConfig) {
	// Configure clipping if enabled
	if config.Clipping {
		f.Device.CR.SetBits(stm32.FMAC_CR_CLIPEN)
	} else {
		f.Device.CR.ClearBits(stm32.FMAC_CR_CLIPEN)
	}

	// Build PARAM value for IIR
	param := uint32(FMACFuncIIR)<<24 |
		uint32(config.R)<<16 |
		uint32(config.Q)<<8 |
		uint32(config.P) |
		stm32.FMAC_PARAM_START

	f.Device.PARAM.Set(param)
}

// Stop halts the current filter operation by performing a reset
func (f *FMAC) Stop() {
	f.Reset()
}

// WriteInput writes a single input sample to the X1 buffer
// Returns false if buffer is full (call again later)
func (f *FMAC) WriteInput(value int16) bool {
	if f.Device.SR.HasBits(stm32.FMAC_SR_X1FULL) {
		return false
	}
	f.Device.WDATA.Set(uint32(uint16(value)))
	return true
}

// WriteInputBlocking writes a single input sample, waiting if buffer full
func (f *FMAC) WriteInputBlocking(value int16) {
	for f.Device.SR.HasBits(stm32.FMAC_SR_X1FULL) {
	}
	f.Device.WDATA.Set(uint32(uint16(value)))
}

// ReadOutput reads a single output sample from the Y buffer
// Returns the value and true if data was available, or 0 and false if empty
func (f *FMAC) ReadOutput() (int16, bool) {
	if f.Device.SR.HasBits(stm32.FMAC_SR_YEMPTY) {
		return 0, false
	}
	return int16(f.Device.RDATA.Get()), true
}

// ReadOutputBlocking reads a single output sample, waiting if buffer empty
func (f *FMAC) ReadOutputBlocking() int16 {
	for f.Device.SR.HasBits(stm32.FMAC_SR_YEMPTY) {
	}
	return int16(f.Device.RDATA.Get())
}

// IsX1Full returns true if the X1 (input) buffer is full
func (f *FMAC) IsX1Full() bool {
	return f.Device.SR.HasBits(stm32.FMAC_SR_X1FULL)
}

// IsYEmpty returns true if the Y (output) buffer is empty
func (f *FMAC) IsYEmpty() bool {
	return f.Device.SR.HasBits(stm32.FMAC_SR_YEMPTY)
}

// HasOverflow returns true if an overflow occurred in the accumulator
func (f *FMAC) HasOverflow() bool {
	return f.Device.SR.HasBits(stm32.FMAC_SR_OVFL)
}

// HasUnderflow returns true if an underflow occurred
func (f *FMAC) HasUnderflow() bool {
	return f.Device.SR.HasBits(stm32.FMAC_SR_UNFL)
}

// HasSaturation returns true if saturation occurred (output was clipped)
func (f *FMAC) HasSaturation() bool {
	return f.Device.SR.HasBits(stm32.FMAC_SR_SAT)
}

// EnableReadInterrupt enables the Read interrupt (Y buffer not empty)
func (f *FMAC) EnableReadInterrupt() {
	f.Device.CR.SetBits(stm32.FMAC_CR_RIEN)
}

// DisableReadInterrupt disables the Read interrupt
func (f *FMAC) DisableReadInterrupt() {
	f.Device.CR.ClearBits(stm32.FMAC_CR_RIEN)
}

// EnableWriteInterrupt enables the Write interrupt (X1 buffer not full)
func (f *FMAC) EnableWriteInterrupt() {
	f.Device.CR.SetBits(stm32.FMAC_CR_WIEN)
}

// DisableWriteInterrupt disables the Write interrupt
func (f *FMAC) DisableWriteInterrupt() {
	f.Device.CR.ClearBits(stm32.FMAC_CR_WIEN)
}

// ============================================================================
// Fixed-Point Conversion Utilities for q1.15 Format
// ============================================================================

// Q15Max is the maximum positive value in q1.15 format (represents ~1.0)
const Q15Max = int16(0x7FFF)

// Q15Min is the minimum negative value in q1.15 format (represents -1.0)
const Q15Min = int16(-0x8000)

// FloatToQ15 converts a float32 in range [-1, 1] to q1.15 fixed-point
// Values outside the range will be clamped
func FloatToQ15(f float32) int16 {
	if f >= 1.0 {
		return Q15Max
	}
	if f <= -1.0 {
		return Q15Min
	}
	return int16(f * 32768.0)
}

// Q15ToFloat converts a q1.15 fixed-point value to float32
func Q15ToFloat(q int16) float32 {
	return float32(q) / 32768.0
}

// FloatCoeffsToQ15 converts a slice of float32 coefficients to q1.15 format
func FloatCoeffsToQ15(coeffs []float32) []int16 {
	result := make([]int16, len(coeffs))
	for i, c := range coeffs {
		result[i] = FloatToQ15(c)
	}
	return result
}
