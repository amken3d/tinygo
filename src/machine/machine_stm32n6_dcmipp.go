//go:build stm32n6

package machine

// DCMIPP (digital camera memory interface pixel pipeline) driver for the
// STM32N6.
//
// Scope of this v1 driver: configure the DCMIPP common section, route the
// CSI-2 input through the pipeline, and run a Pipe 0 raw memory dump
// (snapshot or continuous) into a caller-provided buffer with a
// frame-complete callback.
//
// Out of scope (intentionally — these are each substantial drivers of their
// own): Pipe 1 / Pipe 2 ISP (decimation, demosaic, gamma, CSC, scaling,
// statistics), parallel input via the legacy DCMI interface, double-buffer
// switching beyond AR1, line-event and limit-event reporting, AXI QoS
// tuning. The exported peripheral struct stm32.DCMIPP keeps the door open
// for callers that need to reach those registers directly.
//
// Memory placement note. The buffer passed to StartPipe0 is written by the
// DCMIPP AXI master directly, so it must live in memory the AXI master can
// reach (e.g. AXISRAM3..6 or PSRAM); it must be 32-bit aligned; and the
// caller is responsible for cache maintenance if the M55 D-cache covers
// that region.
//
// See RM0486 §34 (DCMIPP) for full register-level details.

import (
	"device/arm"
	"device/stm32"
	"runtime/interrupt"
	"unsafe"
)

// DCMIPPInputMode selects the DCMIPP input. CSI-2 is the only input wired
// out of the package on the NUCLEO-N657X0-Q board (parallel DCMI pins
// aren't routed to the camera connector).
type DCMIPPInputMode uint8

const (
	DCMIPPInputDCMI DCMIPPInputMode = 0 // Parallel DCMI (legacy DCMI compatibility)
	DCMIPPInputCSI  DCMIPPInputMode = 1 // CSI-2 host
)

// DCMIPPCaptureMode selects continuous vs single-shot capture.
type DCMIPPCaptureMode uint8

const (
	DCMIPPCaptureContinuous DCMIPPCaptureMode = 0
	DCMIPPCaptureSnapshot   DCMIPPCaptureMode = 1
)

// DCMIPPFlowMode controls how P0FSCR.DTMODE filters virtual-channel
// data types into Pipe 0.
type DCMIPPFlowMode uint8

const (
	// DCMIPPFlowSingleDT forwards only the data type matching DataType
	// (DTIDA). This is the right mode for sensors that emit one DT per VC.
	DCMIPPFlowSingleDT DCMIPPFlowMode = 0

	// DCMIPPFlowAllDT forwards every data type on the selected virtual
	// channel — useful for capturing embedded data + pixel data.
	DCMIPPFlowAllDT DCMIPPFlowMode = 3
)

// DCMIPPConfig configures the DCMIPP common section (input mux, virtual
// channel, data-type filter). Pipe-side configuration happens in
// StartPipe0.
type DCMIPPConfig struct {
	// InputMode selects DCMI (parallel) vs CSI-2.
	InputMode DCMIPPInputMode

	// VirtualChannel is the CSI-2 VC routed into Pipe 0 (0..3). Ignored
	// when InputMode is DCMIPPInputDCMI.
	VirtualChannel uint8

	// FlowMode selects the P0FSCR.DTMODE policy. Default
	// DCMIPPFlowSingleDT.
	FlowMode DCMIPPFlowMode

	// DataType is the 6-bit CSI-2 DT code captured by Pipe 0 when
	// FlowMode is DCMIPPFlowSingleDT (e.g. CSIDataTypeRAW10).
	DataType uint8

	// SwapRB toggles DCMIPP_CMCR.SWAPRB (RGB ↔ BGR / VYU ↔ UYV).
	SwapRB bool
}

// DCMIPP is the singleton DCMIPP instance.
var DCMIPP dcmippDevice

type dcmippDevice struct {
	bus *stm32.DCMIPP_Type

	frameCallback func()

	intr     interrupt.Interrupt
	intrInit bool
}

// Configure brings up the DCMIPP common section: clocks, reset pulse,
// input mux, and Pipe 0 flow-selection (which CSI virtual channel and
// data type get routed into the pipe). It does not start capture — call
// StartPipe0 with a buffer when you're ready to receive frames.
func (d *dcmippDevice) Configure(cfg DCMIPPConfig) {
	d.bus = stm32.DCMIPP

	// 0a. RIF master attributes: grant DCMIPP's AXI master access to
	//     AXISRAM. Without this, AXI writes are silently dropped at the
	//     bus fabric — the master's internal byte counter (P0DCCNTR)
	//     still increments, but pixel data never reaches memory. Mirrors
	//     ST's `HAL_RIF_RIMC_ConfigMasterAttributes(RIF_MASTER_INDEX_DCMIPP,
	//     {CID=1, SEC, PRIV})` from their NUCLEO-N657 main.c. Master
	//     index 9 → RIMC_ATTR9; encoding is CID at bits 6:4, SEC at bit 8,
	//     PRIV at bit 9.
	stm32.RCC.AHB3ENSR.Set(stm32.RCC_AHB3ENR_RIFSCEN)
	_ = stm32.RCC.AHB3ENR.Get()
	stm32.RIFSC.RIMC_ATTR9.Set((1 << 4) | (1 << 8) | (1 << 9))

	// 0a'. RISC slave-side: mark the DCMIPP peripheral itself as
	//      secure + privileged. Without this, RIMC_ATTR9.SEC doesn't
	//      actually propagate to the AXI master's transaction tags
	//      (the master inherits the slave-side security attribute).
	//      DCMIPP is in RISC_SECCFGR2 / RISC_PRIVCFGR2 at bit 29
	//      (per ST's RIF_RISC_PERIPH_INDEX_DCMIPP encoding).
	stm32.RIFSC.RISC_SECCFGR2.SetBits(1 << 29)
	stm32.RIFSC.RISC_PRIVCFGR2.SetBits(1 << 29)

	// 0b. No RISAF programming. ST's DCMIPP_ContinuousMode reference
	//     never touches RISAF — RIMC + RISC alone are sufficient when the
	//     destination buffer lives in AXISRAM3 (the region ST uses), since
	//     RISAF4 (AXISRAM3) is permissive enough by default. Caller is
	//     expected to use AXISRAM3 (0x34200000+) and to have called
	//     InitExtendedAXISRAM beforehand.

	// 0. Kernel clock: route IC17 = PLL3 / 2 = 450 MHz to the DCMIPP.
	//    Higher than ST's PLL2/3=333 MHz, but PLL3 is what we have
	//    available (PLL2 not enabled in initCLK). The ISP throughput is
	//    the bottleneck on Pipe 1 — at PLL3/3=300 MHz we got ~80 bytes
	//    of a 640-byte row through before the line cycle closed; pushing
	//    to /2 should fully cover the line. IC17SEL: 0=PLL1, 1=PLL2,
	//    2=PLL3, 3=PLL4.
	stm32.RCC.IC17CFGR.Set((2 << 28) | (1 << 16)) // SEL=PLL3, INT=1 → /2
	stm32.RCC.DIVENSR.Set(stm32.RCC_DIVENSR_IC17ENS)
	// CCIPR1.DCMIPPSEL = 0b10 → IC17 (RM 0486 §14.10: 00=pclk5,
	// 01=per_ck, 10=ic17_ck, 11=hsi_div_ck). Mask 0x300000 already
	// covers the field bits 21:20.
	stm32.RCC.CCIPR1.ReplaceBits(0x200000, stm32.RCC_CCIPR1_DCMIPPSEL_Msk, 0)

	// 1. Bus clock + reset pulse. APB5 ENSR/RSTSR/RSTCR mirror the
	//    read-only ENR/RSTR registers — see machine_stm32n6.go.
	stm32.RCC.APB5ENSR.Set(stm32.RCC_APB5ENR_DCMIPPEN)
	_ = stm32.RCC.APB5ENR.Get()

	stm32.RCC.APB5RSTSR.Set(stm32.RCC_APB5RSTR_DCMIPPRST)
	for i := 0; i < 256; i++ {
		arm.Asm("nop")
	}
	stm32.RCC.APB5RSTCR.Set(stm32.RCC_APB5RSTR_DCMIPPRST)
	_ = stm32.RCC.APB5RSTR.Get()

	// 2. Common-section configuration.
	cmcr := uint32(0)
	if cfg.InputMode == DCMIPPInputCSI {
		cmcr |= stm32.DCMIPP_CMCR_INSEL
	}
	if cfg.SwapRB {
		cmcr |= stm32.DCMIPP_CMCR_SWAPRB
	}
	// Map the common frame counter to Pipe 0 (PSFC=0 — also the reset
	// default; explicit so the value is obvious).
	d.bus.CMCR.Set(cmcr)

	// 3. Pipe 0 flow selection.
	//    DTIDA = data type, DTMODE = filter mode, VC = source virtual
	//    channel, PIPEN = pipe enable. PIPEN is set here (not in
	//    StartPipe0) because the RM requires PIPEN=1 *before* starting
	//    capture; CPTREQ in P0FCTCR is the per-frame request gate.
	dt := uint32(cfg.DataType) & 0x3F
	flow := uint32(cfg.FlowMode) & 0x3
	vc := uint32(cfg.VirtualChannel) & 0x3
	d.bus.P0FSCR.Set(
		dt |
			(flow << stm32.DCMIPP_P0FSCR_DTMODE_Pos) |
			(vc << stm32.DCMIPP_P0FSCR_VC_Pos) |
			stm32.DCMIPP_P0FSCR_PIPEN,
	)

	// 4. Default Pipe 0 packer config. SWAPYUV off, MSB-aligned for raw
	//    formats (PAD=1) which is the convention software / GPUs expect,
	//    no byte/line decimation, no header dump, no double-buffer.
	d.bus.P0PPCR.Set(stm32.DCMIPP_P0PPCR_PAD)

	// 5. Disable any leftover Pipe 0 IRQ sources and clear W1C flags.
	//    Per RM § "Pipe0 interrupt clear register" only bits 0..7 are
	//    valid; bits 31:8 are Reserved.
	d.bus.P0IER.Set(0)
	d.bus.P0FCR.Set((1 << 0) | (1 << 1) | (1 << 2) | (1 << 6) | (1 << 7))

	if !d.intrInit {
		d.intr = interrupt.New(stm32.IRQ_DCMIPP, dcmippHandleInterrupt)
		d.intrInit = true
	}
	d.intr.Enable()
}

// dcmippHandleInterrupt is the free-function dispatcher for the DCMIPP
// IRQ (TinyGo's interrupt.New rejects bound-method closures). Forwards
// to the singleton's method.
func dcmippHandleInterrupt(intr interrupt.Interrupt) {
	DCMIPP.handleInterrupt(intr)
}

// StartPipe0 arms a Pipe 0 dump capture into buf.
//
// Snapshot vs continuous:
//   - Snapshot: hardware writes one frame's worth of pixel data and goes
//     idle (CPTACT clears). If the buffer is smaller than one frame, the
//     dump-limiter (P0DCLMTR) trips first and fires LIMITF in P0SR; the
//     caller must call StopPipe0 to clear CPTREQ in that case.
//   - Continuous: the same dump runs every frame, overwriting buf. The
//     caller calls StopPipe0 to end streaming.
//
// Pipe0Active() reports CPTACT; WaitPipe0Snapshot() bundles the right
// logic for snapshot capture (poll for natural EOF, fall through to
// StopPipe0 on LIMITF). Both are bounded so a stuck pipe is observable.
//
// buf must be 16-byte aligned (P0PPM0AR1 silently masks off the low 4
// bits) and AXI-reachable (AXISRAM2..6 or PSRAM with the matching RISAF
// region opened). The DCMIPP AXI master writes 32-bit words.
func (d *dcmippDevice) StartPipe0(buf []byte, mode DCMIPPCaptureMode) {
	if len(buf) < 4 {
		return
	}
	addr := uintptr(unsafe.Pointer(&buf[0]))

	// Memory0 primary address. The pipe always uses M0AR1 unless
	// double-buffer mode is enabled (which we don't enable here).
	d.bus.P0PPM0AR1.Set(uint32(addr))

	// Disarm before reconfiguring. CPTREQ=0 so the next-VSync sample
	// won't latch a stale request mid-write of P0PPM0AR1/P0DCLMTR.
	d.bus.P0FCTCR.Set(0)

	// Clear all W1C flag bits in P0FCR. Per RM § "Pipe0 interrupt
	// clear register": bits 31:8 are Reserved/RES0; only bits 0..7
	// are W1C (CLINEF=0, CFRAMEF=1, CVSYNCF=2, CLIMITF=6, COVRF=7).
	const fcrAll = (1 << 0) | (1 << 1) | (1 << 2) | (1 << 6) | (1 << 7) // 0xC7
	d.bus.P0FCR.Set(fcrAll)

	// P0DCCNTR is read-only per RM ("At FrameEnd, the counter ... is
	// reset to start the count of the next frame"). No software action
	// needed; the hardware self-resets per-frame.

	// Limiter: ENABLE=1 with LIMIT=len(buf)/4 for both modes. Per RM §
	// "DCMIPP Pipe0 dump limit register" the counter self-resets at
	// FrameEnd, and when CNT reaches LIMIT the pipe deletes the rest of
	// the frame's data until the next VSync. So with the limiter on and
	// LIMIT==buffer size in 32-bit words:
	//   - Snapshot: pipe writes up to len(buf), discards rest of frame,
	//     CPTACT clears at natural EOF.
	//   - Continuous: same per frame; next frame starts fresh, writes
	//     up to len(buf) again at P0PPM0AR1, etc.
	// Disabling the limiter in continuous mode lets the AXI master run
	// off the end of buf — empirically that hangs the CPU (likely from
	// stack/code corruption when DCMIPP writes a 10 MB frame into a
	// 64 KB buffer slot).
	limitWords := uint32(len(buf) / 4)
	if limitWords >= 1<<24 {
		limitWords = (1 << 24) - 1
	}
	d.bus.P0DCLMTR.Set(limitWords | stm32.DCMIPP_P0DCLMTR_ENABLE)

	// Pipe 0 IRQ sources: frame complete + overrun. We deliberately do
	// NOT enable LIMITIE here — the IRQ handler clears LIMITF on entry,
	// which would race WaitPipe0Snapshot's polling. Polling sees the
	// flag in P0SR directly.
	d.bus.P0IER.Set(stm32.DCMIPP_P0IER_FRAMEIE | stm32.DCMIPP_P0IER_OVRIE)

	// Capture mode + request. CPTREQ is the per-frame "go" bit; in
	// snapshot mode the hardware auto-clears it after the frame, in
	// continuous mode we keep it set until StopPipe0 clears it.
	fctcr := uint32(0)
	if mode == DCMIPPCaptureSnapshot {
		fctcr |= stm32.DCMIPP_P0FCTCR_CPTMODE
	}
	fctcr |= stm32.DCMIPP_P0FCTCR_CPTREQ
	d.bus.P0FCTCR.Set(fctcr)
}

// StopPipe0 clears CPTREQ. In continuous mode the current frame finishes
// and the pipe goes idle; in snapshot mode this is a no-op when the
// hardware has already cleared CPTREQ at end of frame, but is required
// when the dump-limiter tripped before EOF (CPTACT stays set otherwise).
func (d *dcmippDevice) StopPipe0() {
	d.bus.P0FCTCR.ClearBits(stm32.DCMIPP_P0FCTCR_CPTREQ)
	d.bus.P0IER.Set(0)
}

// WaitPipe0Snapshot waits for a snapshot capture to complete. Returns the
// completion path:
//
//	"frame" — natural end-of-frame; CPTACT cleared on its own, full
//	         frame in buf.
//	"limit" — P0DCLMTR tripped before EOF; caller's buffer was too small
//	         for one frame. WaitPipe0Snapshot called StopPipe0 so the
//	         pipe is idle on return; buf contains the prefix that fit.
//	"stuck" — neither happened within timeoutMs; pipe is in an
//	         unexpected state. The caller may want to log diagnostics
//	         then call StopPipe0 and reset.
//
// timeoutMs is the wall-clock budget. For a snapshot at typical frame
// rates 1000 ms is generous; bump it if the sensor runs at < 5 fps.
func (d *dcmippDevice) WaitPipe0Snapshot(timeoutMs int) string {
	const pollIntervalUs = 200
	maxIters := (timeoutMs * 1000) / pollIntervalUs
	for i := 0; i < maxIters; i++ {
		sr := d.bus.P0SR.Get()
		if sr&stm32.DCMIPP_P0SR_LIMITF != 0 {
			d.StopPipe0()
			// Settle: wait for CPTACT to drop, bounded.
			for j := 0; j < 1000 && d.bus.P0SR.HasBits(stm32.DCMIPP_P0SR_CPTACT); j++ {
				for k := 0; k < 1000; k++ {
					arm.Asm("nop")
				}
			}
			return "limit"
		}
		if sr&stm32.DCMIPP_P0SR_CPTACT == 0 {
			return "frame"
		}
		for k := 0; k < 1000; k++ {
			arm.Asm("nop")
		}
	}
	return "stuck"
}

// OnPipe0Frame registers a callback that runs from the DCMIPP IRQ context
// after each frame is fully written to memory. Pass nil to disarm. The
// callback runs with interrupts enabled at the global level but the
// DCMIPP IRQ pending — keep it short.
func (d *dcmippDevice) OnPipe0Frame(cb func()) {
	d.frameCallback = cb
}

// FrameCount returns the common frame counter. It increments on the pipe
// selected by CMCR.PSFC (Pipe 0 here) and wraps at 2^32.
func (d *dcmippDevice) FrameCount() uint32 {
	return d.bus.CMFRCR.Get()
}

// Pipe0Active reports whether Pipe 0 currently has a capture in flight.
// Useful for polling snapshot completion without enabling the IRQ.
func (d *dcmippDevice) Pipe0Active() bool {
	return d.bus.P0SR.HasBits(stm32.DCMIPP_P0SR_CPTACT)
}

// handleInterrupt dispatches Pipe 0 and Pipe 1 events. The DCMIPP has a
// single IRQ vector (#48 on N6); each pipe has its own status register
// with the same flag-bit layout, and we clear via per-pipe W1C FCR.
//
// Critical: OVRF in particular MUST be cleared per pipe — once set, the
// AXI master for that pipe halts until the flag is acknowledged. For
// Pipe 1 (ISP path) overrun is much more common than Pipe 0 (raw dump)
// since the ISP generates output continuously and the FIFO is shared.
func (d *dcmippDevice) handleInterrupt(interrupt.Interrupt) {
	const flagMask = stm32.DCMIPP_P0SR_LINEF |
		stm32.DCMIPP_P0SR_FRAMEF |
		stm32.DCMIPP_P0SR_VSYNCF |
		stm32.DCMIPP_P0SR_LIMITF |
		stm32.DCMIPP_P0SR_OVRF

	// Pipe 0
	sr0 := d.bus.P0SR.Get()
	d.bus.P0FCR.Set(sr0 & flagMask)
	if sr0&stm32.DCMIPP_P0SR_FRAMEF != 0 {
		if cb := d.frameCallback; cb != nil {
			cb()
		}
	}

	// Pipe 1 — same flag layout in P1SR/P1FCR.
	sr1 := d.bus.P1SR.Get()
	d.bus.P1FCR.Set(sr1 & flagMask)
}

// ==========================================================================
// Pipe 1 — pixel pipe with full ISP (BLC, demosaic, downsize, crop, packer)
// ==========================================================================
//
// Where Pipe 0 dumps the raw CSI byte stream as-is, Pipe 1 walks the data
// through the on-chip ISP and emits a UVC-ready YUYV (or RGB / mono) frame.
// This driver targets the minimum-viable ISP path:
//
//	RAW10 in →  black-level subtract  →  Bayer→RGB demosaic
//	         →  downsize (max 8×)     →  optional crop
//	         →  YUV422 packer         →  AXI write
//
// Color conversion matrix, gamma, and statistics blocks stay disabled
// (identity / off). Output colors will be uncalibrated but the data is
// structurally correct YUYV.
//
// Memory budget: a 320×240 YUYV frame is 153,600 bytes — fits comfortably
// in the 256 KB AXISRAM2 RAM region. 640×480 (614 KB) does not fit and
// would need PSRAM. The downsizer caps at 8× per-axis from a single stage,
// so 2592×1944 → 320×240 needs downsize-to-324×243 followed by a small
// crop, both done in hardware here.

// Pipe1BayerType selects the input Bayer pattern. IMX335 default is RGGB.
type Pipe1BayerType uint8

const (
	Pipe1BayerRGGB Pipe1BayerType = 0
	Pipe1BayerGRBG Pipe1BayerType = 1
	Pipe1BayerGBRG Pipe1BayerType = 2
	Pipe1BayerBGGR Pipe1BayerType = 3
)

// Pipe1Format selects the pixel-packer output format.
type Pipe1Format uint8

const (
	Pipe1FormatRGB888 Pipe1Format = 0  // 24 bpp R8 G8 B8 (or YUV444 if YUVConv enabled)
	Pipe1FormatRGB565 Pipe1Format = 1  // 16 bpp R5 G6 B5
	Pipe1FormatARGB   Pipe1Format = 2  // 32 bpp A=0xFF + R8 G8 B8
	Pipe1FormatRGBA   Pipe1Format = 3  // 32 bpp R8 G8 B8 + A=0xFF
	Pipe1FormatMonoY8 Pipe1Format = 4  // 8 bpp grayscale (G8 from demosaic, or Y if YUVConv)
	Pipe1FormatYUV444 Pipe1Format = 5  // 32 bpp YUV444 1-buffer
	Pipe1FormatYUYV   Pipe1Format = 6  // 16 bpp YUV422 1-buffer, Y0 U Y1 V byte order
	Pipe1FormatUYVY   Pipe1Format = 10 // 16 bpp YUV422 1-buffer, U Y0 V Y1 byte order
)

// Pipe1Config configures the Pipe 1 ISP path end-to-end.
type Pipe1Config struct {
	// VirtualChannel is the CSI-2 VC routed into Pipe 1 (0..3).
	VirtualChannel uint8

	// FlowMode selects DTMODE (DCMIPPFlowSingleDT for typical sensors).
	FlowMode DCMIPPFlowMode

	// DataType is the 6-bit CSI-2 DT code (e.g. CSIDataTypeRAW10).
	DataType uint8

	// InputWidth, InputHeight describe the sensor's frame size.
	InputWidth, InputHeight uint16

	// OutputWidth, OutputHeight are the post-downsize dimensions written
	// to memory.
	//
	// The DCMIPP downsize block has a hard 16-bit field for HRATIO/VRATIO
	// where ratio = 8192 × source / dest. The field saturates at 65535
	// (≈ 8.0×), and *programming the field at exactly 65535 makes the
	// downsize block degenerate to ~1/64 of the requested output*. The
	// caller must ensure source/dest is strictly below 8.0× per axis —
	// either by choosing a larger output, or by setting SourceCrop* to
	// trim the sensor frame to dimensions that scale cleanly.
	OutputWidth, OutputHeight uint16

	// SourceCropX, SourceCropY, SourceCropW, SourceCropH define a crop
	// applied *before* downsize. Zero W or H means "no crop" — the
	// full InputWidth × InputHeight is passed through.
	//
	// Use this when output dimensions would otherwise hit the downsize
	// clamp. Example: IMX335 2592×1944 → 320×240 needs ratio 8.1× which
	// clamps. Setting SourceCrop to (96, 72, 2400, 1800) feeds the
	// downsize a 2400×1800 sub-rectangle and produces clean 320×240
	// output (ratio 7.5×, no clamp).
	SourceCropX, SourceCropY uint16
	SourceCropW, SourceCropH uint16

	// BayerType matches the sensor's Bayer pattern.
	BayerType Pipe1BayerType

	// Format is the pixel-packer output format.
	Format Pipe1Format
}

// ConfigurePipe1 programs all the ISP and packer registers for Pipe 1.
// The pipe is left enabled (PIPEN=1) and ready to receive a StartPipe1
// arm. Call after Configure() (which sets up the common section + clocks).
func (d *dcmippDevice) ConfigurePipe1(cfg Pipe1Config) {
	// IP-Plug left at reset defaults — matches ST's DCMIPP_ContinuousMode
	// reference, which never calls HAL_DCMIPP_SetIPPlugConfig. Earlier we
	// wrote IPGR1=0 (64B page) plus IPC2R1.TRAFFIC=4 (128B burst); burst >
	// page is invalid on the IP-Plug AXI master and silently truncated
	// most of the writes — Pipe 1 only filled the first ~1/64 of the
	// destination buffer.

	// Disarm and disable the pipe before reconfiguring.
	d.bus.P1FCTCR.Set(0)
	d.bus.P1FSCR.Set(0)

	// Flow selection (without PIPEN — we set that last so the pipe sees
	// a consistent config when it activates).
	dt := uint32(cfg.DataType) & 0x3F
	flow := uint32(cfg.FlowMode) & 0x3
	vc := uint32(cfg.VirtualChannel) & 0x3
	d.bus.P1FSCR.Set(
		dt |
			(flow << stm32.DCMIPP_P1FSCR_DTMODE_Pos) |
			(vc << stm32.DCMIPP_P1FSCR_VC_Pos),
	)

	// Black-level calibration. IMX335 raw output has a black-level
	// pedestal of ~12 LSB per channel; subtracting it gives true black
	// at zero. Values match ST's IQTune-generated isp_param_conf.h
	// (BLCR = BLCG = BLCB = 12).
	const blcOffset = 12
	d.bus.P1BLCCR.Set(
		stm32.DCMIPP_P1BLCCR_ENABLE |
			(blcOffset << stm32.DCMIPP_P1BLCCR_BLCR_Pos) |
			(blcOffset << stm32.DCMIPP_P1BLCCR_BLCG_Pos) |
			(blcOffset << stm32.DCMIPP_P1BLCCR_BLCB_Pos),
	)

	// Demosaic / Bayer→RGB. Per-detector strengths from ST's IQTune
	// (peak=2, lineV=4, lineH=4, edge=6).
	d.bus.P1DMCR.Set(
		stm32.DCMIPP_P1DMCR_ENABLE |
			(uint32(cfg.BayerType) << stm32.DCMIPP_P1DMCR_TYPE_Pos) |
			(2 << stm32.DCMIPP_P1DMCR_PEAK_Pos) |
			(4 << stm32.DCMIPP_P1DMCR_LINEV_Pos) |
			(4 << stm32.DCMIPP_P1DMCR_LINEH_Pos) |
			(6 << stm32.DCMIPP_P1DMCR_EDGE_Pos),
	)

	// Gamma correction. IQTune sets enable=1 for IMX335. Maps the
	// linear sensor response into perceptually-flat luminance.
	d.bus.P1GMCR.Set(stm32.DCMIPP_P1GMCR_ENABLE)

	// Decimation disabled. Decimation drops every Nth pixel/line which
	// breaks the Bayer pattern that the demosaic block expects. Use
	// SourceCrop instead when input pre-shrink is needed.
	d.bus.P1DECR.Set(0)

	// Source crop sits BEFORE the downsize block in the pipeline. When
	// SourceCropW/H are non-zero we feed the downsize a sub-rectangle of
	// the sensor frame; otherwise the full input passes through.
	cropW := uint32(cfg.SourceCropW)
	cropH := uint32(cfg.SourceCropH)
	cropX := uint32(cfg.SourceCropX)
	cropY := uint32(cfg.SourceCropY)
	useCrop := cropW != 0 && cropH != 0
	if useCrop {
		d.bus.P1CRSTR.Set(cropX | (cropY << stm32.DCMIPP_P1CRSTR_VSTART_Pos))
		d.bus.P1CRSZR.Set(
			cropW |
				(cropH << stm32.DCMIPP_P1CRSZR_VSIZE_Pos) |
				stm32.DCMIPP_P1CRSZR_ENABLE,
		)
	} else {
		d.bus.P1CRSTR.Set(0)
		d.bus.P1CRSZR.Set(0)
		cropW = uint32(cfg.InputWidth)
		cropH = uint32(cfg.InputHeight)
	}

	// Downsize ratio = 8192 × Source / Dest, capped at 65535 by the
	// register's 16-bit field. Empirical fact: programming HRATIO or
	// VRATIO at exactly 65535 (the field max, ≈ 8.0× downsize) makes
	// the downsize block degenerate to producing only the top-left
	// ~1/64 of the requested output. Stay strictly below the clamp.
	//
	// For ratios that would otherwise exceed 65535 the caller must
	// pre-shrink with SourceCrop. We don't attempt any silent fix here —
	// the wrong workaround produces a corrupt frame.
	const ratioMax = 65520 // one below 65535 to be safe; never clamp
	hRatio := uint32(8192) * cropW / uint32(cfg.OutputWidth)
	vRatio := uint32(8192) * cropH / uint32(cfg.OutputHeight)
	if hRatio > ratioMax || vRatio > ratioMax {
		// Refuse to program a known-bad config. Leave the pipe disabled
		// so the caller's StartPipe1 won't produce a corrupted frame.
		d.bus.P1FSCR.ClearBits(stm32.DCMIPP_P1FSCR_PIPEN)
		d.bus.P1DSCR.Set(0)
		return
	}
	// Per ST's CMW_UTILS_get_down_config: HDivFactor = (1024*8192-1) / HRatio.
	hDiv := uint32(1024*8192-1) / hRatio
	vDiv := uint32(1024*8192-1) / vRatio

	// Order matters: ST's HAL_DCMIPP_PIPE_SetDownsizeConfig writes
	// HDIV/VDIV first (without ENABLE), then HRATIO/VRATIO, then HSIZE/
	// VSIZE — and only afterwards SET-bits ENABLE. Doing it in one
	// monolithic write with ENABLE included can latch the wrong internal
	// state on HSIZE/VSIZE write.
	d.bus.P1DSCR.Set(hDiv | (vDiv << stm32.DCMIPP_P1DSCR_VDIV_Pos))
	d.bus.P1DSRTIOR.Set(hRatio | (vRatio << stm32.DCMIPP_P1DSRTIOR_VRATIO_Pos))
	d.bus.P1DSSZR.Set(
		uint32(cfg.OutputWidth) |
			(uint32(cfg.OutputHeight) << stm32.DCMIPP_P1DSSZR_VSIZE_Pos),
	)
	d.bus.P1DSCR.SetBits(stm32.DCMIPP_P1DSCR_ENABLE)

	// Pixel packer: pure FORMAT field; nothing else needed for YUV422
	// 1-buffer. Pitch = OutputWidth × bytes-per-pixel (depends on format).
	d.bus.P1PPCR.Set(uint32(cfg.Format) << stm32.DCMIPP_P1PPCR_FORMAT_Pos)
	bpp := uint32(2) // default for YUYV/UYVY/RGB565
	switch cfg.Format {
	case Pipe1FormatMonoY8:
		bpp = 1
	case Pipe1FormatRGB888:
		bpp = 3
	case Pipe1FormatARGB, Pipe1FormatRGBA, Pipe1FormatYUV444:
		bpp = 4
	}
	d.bus.P1PPM0PR.Set(uint32(cfg.OutputWidth) * bpp)

	// IRQ sources for Pipe 1: FRAMEF (frame complete) and OVRF (overrun).
	// Without OVRIE the shared DCMIPP IRQ handler never gets a chance to
	// W1C the OVRF flag in P1FCR, and the AXI master halts after the
	// first overrun.
	d.bus.P1IER.Set(stm32.DCMIPP_P1IER_FRAMEIE | stm32.DCMIPP_P1IER_OVRIE)
	// Clear any latched flags from prior runs (W1C bits 0,1,2,6,7).
	d.bus.P1FCR.Set((1 << 0) | (1 << 1) | (1 << 2) | (1 << 6) | (1 << 7))

	// Don't enable PIPEN here; StartPipe1 sets it together with CPTREQ
	// to match ST's HAL_DCMIPP_PIPE_Start sequencing.
}

// StartPipe1 arms a Pipe 1 capture into buf. Snapshot/continuous semantics
// match StartPipe0: snapshot grabs one frame and goes idle, continuous
// rewrites buf each frame until StopPipe1.
//
// Per ST's HAL_DCMIPP_PIPE_Start: program CPTMODE + destination address
// first, then enable PIPEN and CPTREQ together.
//
// buf must be at least OutputWidth × OutputHeight × 2 bytes (for YUYV) and
// 16-byte aligned.
func (d *dcmippDevice) StartPipe1(buf []byte, mode DCMIPPCaptureMode) {
	if len(buf) < 4 {
		return
	}
	addr := uintptr(unsafe.Pointer(&buf[0]))

	// Disarm to avoid mid-flight half-config.
	d.bus.P1FCTCR.Set(0)
	d.bus.P1FSCR.ClearBits(stm32.DCMIPP_P1FSCR_PIPEN)
	d.bus.P1PPCR.ClearBits(stm32.DCMIPP_P1PPCR_DBM) // single-buffer

	if mode == DCMIPPCaptureSnapshot {
		d.bus.P1FCTCR.Set(stm32.DCMIPP_P1FCTCR_CPTMODE)
	}
	d.bus.P1PPM0AR1.Set(uint32(addr))

	// Activate pipe + start capture (PIPEN and CPTREQ together).
	d.bus.P1FSCR.SetBits(stm32.DCMIPP_P1FSCR_PIPEN)
	d.bus.P1FCTCR.SetBits(stm32.DCMIPP_P1FCTCR_CPTREQ)
}

// StartPipe1DoubleBuffer arms continuous Pipe 1 capture in double-buffer
// mode: DCMIPP ping-pongs between buf0 and buf1 each frame, with the
// alternation reflected in P1SR.DBSEL. Use Pipe1ActiveBuffer to find
// which buffer DCMIPP is currently writing — the OTHER one is safe to
// read from.
//
// Both buffers must be the same size (OutputWidth × OutputHeight × bpp)
// and AXI-reachable.
func (d *dcmippDevice) StartPipe1DoubleBuffer(buf0, buf1 []byte) {
	if len(buf0) < 4 || len(buf1) < 4 {
		return
	}
	a0 := uintptr(unsafe.Pointer(&buf0[0]))
	a1 := uintptr(unsafe.Pointer(&buf1[0]))

	// Disarm.
	d.bus.P1FCTCR.Set(0)
	d.bus.P1FSCR.ClearBits(stm32.DCMIPP_P1FSCR_PIPEN)

	// Program both addresses, enable DBM in P1PPCR.
	d.bus.P1PPM0AR1.Set(uint32(a0))
	d.bus.P1PPM0AR2.Set(uint32(a1))
	d.bus.P1PPCR.SetBits(stm32.DCMIPP_P1PPCR_DBM)

	// Continuous-mode arm: PIPEN + CPTREQ.
	d.bus.P1FSCR.SetBits(stm32.DCMIPP_P1FSCR_PIPEN)
	d.bus.P1FCTCR.SetBits(stm32.DCMIPP_P1FCTCR_CPTREQ)
}

// Pipe1LastBuffer returns 0 or 1 depending on which buffer DCMIPP just
// finished writing (in double-buffer mode). That buffer is safe to read
// — DCMIPP is now writing to the OTHER one. Reads P1SR.LSTFRM.
//
// Caller should latch this once per UVC frame (at frame-offset 0) and
// hold the choice for the entire UVC frame to avoid mid-stream tearing.
func (d *dcmippDevice) Pipe1LastBuffer() int {
	if d.bus.P1SR.HasBits(stm32.DCMIPP_P1SR_LSTFRM) {
		return 1
	}
	return 0
}

// StopPipe1 clears CPTREQ. In continuous mode the in-flight frame finishes
// and the pipe goes idle.
func (d *dcmippDevice) StopPipe1() {
	d.bus.P1FCTCR.ClearBits(stm32.DCMIPP_P1FCTCR_CPTREQ)
}

// Pipe1Active reports whether Pipe 1 currently has a capture in flight.
func (d *dcmippDevice) Pipe1Active() bool {
	return d.bus.P1SR.HasBits(stm32.DCMIPP_P1SR_CPTACT)
}

// Pipe1RegDump returns a snapshot of Pipe 1's key registers, for debug.
func (d *dcmippDevice) Pipe1RegDump() Pipe1Regs {
	return Pipe1Regs{
		IPGR1:     stm32.DCMIPP.IPGR1.Get(),
		IPC2R1:    stm32.DCMIPP.IPC2R1.Get(),
		IPC2R3:    stm32.DCMIPP.IPC2R3.Get(),
		CMCR:      d.bus.CMCR.Get(),
		P1FSCR:    d.bus.P1FSCR.Get(),
		P1FCTCR:   d.bus.P1FCTCR.Get(),
		P1SR:      d.bus.P1SR.Get(),
		P1FCR:     d.bus.P1FCR.Get(),
		P1BLCCR:   d.bus.P1BLCCR.Get(),
		P1DMCR:    d.bus.P1DMCR.Get(),
		P1DECR:    d.bus.P1DECR.Get(),
		P1DCCR:    d.bus.P1DCCR.Get(),
		P1DSCR:    d.bus.P1DSCR.Get(),
		P1DSRTIOR: d.bus.P1DSRTIOR.Get(),
		P1DSSZR:   d.bus.P1DSSZR.Get(),
		P1CRSTR:   d.bus.P1CRSTR.Get(),
		P1CRSZR:   d.bus.P1CRSZR.Get(),
		P1GMCR:    d.bus.P1GMCR.Get(),
		P1YUVCR:   d.bus.P1YUVCR.Get(),
		P1PPCR:    d.bus.P1PPCR.Get(),
		P1PPM0PR:  d.bus.P1PPM0PR.Get(),
		P1PPM0AR1: d.bus.P1PPM0AR1.Get(),
	}
}

// Pipe1Regs is a snapshot of Pipe 1's key registers.
type Pipe1Regs struct {
	IPGR1, IPC2R1, IPC2R3 uint32
	CMCR                  uint32
	P1FSCR, P1FCTCR       uint32
	P1SR, P1FCR           uint32
	P1BLCCR, P1DMCR       uint32
	P1DECR, P1DCCR        uint32
	P1DSCR, P1DSRTIOR     uint32
	P1DSSZR               uint32
	P1CRSTR, P1CRSZR      uint32
	P1GMCR                uint32
	P1YUVCR               uint32
	P1PPCR                uint32
	P1PPM0PR              uint32
	P1PPM0AR1             uint32
}
