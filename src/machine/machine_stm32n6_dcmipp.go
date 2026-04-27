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

	// 0b. RISAF3 region for AXISRAM2: granting RIMC permission to DCMIPP
	//     isn't enough — AXISRAM2 itself has its own per-region access
	//     control (RISAF3). By default no region is enabled and the deny
	//     policy blocks all bus-master accesses (only the CPU works
	//     because the FSBL set up an MPU/CACHE path that bypasses RISAF
	//     for CPU loads/stores). RISAF3 lives at 0x44028000 — same
	//     register layout as the SVD-defined RISAF1, just at a different
	//     address. We open REG1 to cover the entire AXISRAM2 (0x34100000
	//     .. 0x341FFFFF) and grant CID 0 (CPU) + CID 1 (DCMIPP/etc.) full
	//     read+write access.
	risaf3 := (*stm32.RISAF_Type)(unsafe.Pointer(uintptr(0x44028000)))
	// STARTR/ENDR are offsets within AXISRAM2 (the high bits are
	// hard-wired to the region's base address), so 0..0xFFFFF covers
	// the full AXISRAM2.
	risaf3.REG1_STARTR.Set(0x00000000)
	risaf3.REG1_ENDR.Set(0x000FFFFF)
	// Allow read+write for CID 0 (CPU) and CID 1 (DCMIPP, plus other
	// AXI masters that ST configures with CID=1). Setting CIDs 2..7
	// triggered a bus fault in testing — those slots may be reserved
	// or aliased by RIF.
	risaf3.REG1_CIDCFGR.Set(
		(1 << 0) | (1 << 1) | // RDENC0 | RDENC1
			(1 << 16) | (1 << 17), // WRENC0 | WRENC1
	)
	// CFGR: BREN=1 + SEC=1. With TZEN off the M55 sends transactions
	// tagged secure (single-state == secure-equivalent), and DCMIPP
	// inherits secure from RISC_SECCFGR2. SEC=0 would deny CPU access
	// (proven empirically: hung at next stack reference).
	risaf3.REG1_CFGR.Set((1 << 0) | (1 << 8))
	// Clear any pending illegal-access flag from prior boots.
	risaf3.IACR.Set(stm32.RISAF_IASR_CAEF | stm32.RISAF_IASR_IAEF)

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
	Pipe1FormatYUYV   Pipe1Format = 6  // YUV422 1-buffer, byte order Y0 U Y1 V
	Pipe1FormatUYVY   Pipe1Format = 10 // YUV422 1-buffer, byte order U Y0 V Y1
	Pipe1FormatRGB565 Pipe1Format = 1
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

	// OutputWidth, OutputHeight are the final post-crop dimensions.
	// The downsizer takes input → max ratio (8× per axis), then a crop
	// trims to OutputWidth × OutputHeight.
	OutputWidth, OutputHeight uint16

	// BayerType matches the sensor's Bayer pattern.
	BayerType Pipe1BayerType

	// Format is the pixel-packer output format.
	Format Pipe1Format
}

// ConfigurePipe1 programs all the ISP and packer registers for Pipe 1.
// The pipe is left enabled (PIPEN=1) and ready to receive a StartPipe1
// arm. Call after Configure() (which sets up the common section + clocks).
func (d *dcmippDevice) ConfigurePipe1(cfg Pipe1Config) {
	// IP-Plug: lock, write Client 2 traffic shaping (OTR=8 to allow
	// pipelined AXI bursts) and the global memory page size, then
	// release PSTART. Smaller page size (64B vs default 256B) is more
	// forgiving of buffers that aren't page-aligned, which ours isn't.
	d.bus.IPGR2.SetBits(stm32.DCMIPP_IPGR2_PSTART)
	for i := 0; i < 100_000; i++ {
		if d.bus.IPGR3.HasBits(stm32.DCMIPP_IPGR3_IDLE) {
			break
		}
	}
	d.bus.IPGR1.Set(0) // 64-byte memory page
	d.bus.IPC2R1.Set(
		(4 << stm32.DCMIPP_IPC2R1_TRAFFIC_Pos) | // 128-byte burst
			(7 << stm32.DCMIPP_IPC2R1_OTR_Pos), // 8 outstanding
	)
	d.bus.IPGR2.ClearBits(stm32.DCMIPP_IPGR2_PSTART)

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

	// Black-level calibration: enable with zero offsets. The ISP requires
	// BLC to be on (per ST's BSP) for the demosaic block to behave well
	// even when the offsets are zero.
	d.bus.P1BLCCR.Set(stm32.DCMIPP_P1BLCCR_ENABLE)

	// Demosaic / Bayer→RGB. Mid-strength filter (STRENGTH_8 = 4) for all
	// four detectors gives reasonable edges without heavy ringing.
	const algStrength = 4
	d.bus.P1DMCR.Set(
		stm32.DCMIPP_P1DMCR_ENABLE |
			(uint32(cfg.BayerType) << stm32.DCMIPP_P1DMCR_TYPE_Pos) |
			(algStrength << stm32.DCMIPP_P1DMCR_PEAK_Pos) |
			(algStrength << stm32.DCMIPP_P1DMCR_LINEV_Pos) |
			(algStrength << stm32.DCMIPP_P1DMCR_LINEH_Pos) |
			(algStrength << stm32.DCMIPP_P1DMCR_EDGE_Pos),
	)

	// Decimation disabled (broke writes outright when on; dropped to
	// 0 bytes touched). Re-enable later once we understand why.
	d.bus.P1DECR.Set(0)
	effInputW := uint32(cfg.InputWidth)
	effInputH := uint32(cfg.InputHeight)

	// Downsize from (decimated) input → output. With decimation 2× the
	// effective input is 1296×972 for IMX335; downsize to 320×240 needs
	// HRatio = 8192*1296/320 ≈ 33177 — well under the 65535 ceiling,
	// so no crop step needed (downsize lands exactly on 320×240).
	const ratioMax = 65535
	hRatio := uint32(8192) * effInputW / uint32(cfg.OutputWidth)
	if hRatio > ratioMax {
		hRatio = ratioMax
	}
	vRatio := uint32(8192) * effInputH / uint32(cfg.OutputHeight)
	if vRatio > ratioMax {
		vRatio = ratioMax
	}
	// Per ST's CMW_UTILS_get_down_config: HDivFactor = (1024*8192-1) / HRatio.
	hDiv := uint32(1024*8192-1) / hRatio
	vDiv := uint32(1024*8192-1) / vRatio
	// Intermediate size — what the downsize would produce given input
	// and ratio — used only to decide whether crop is needed.
	interW := effInputW * 8192 / hRatio
	interH := effInputH * 8192 / vRatio

	// Order matters here: ST's HAL_DCMIPP_PIPE_SetDownsizeConfig writes
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
	// Now enable downsize as a separate write.
	d.bus.P1DSCR.SetBits(stm32.DCMIPP_P1DSCR_ENABLE)

	// Crop: only enable if the downsize couldn't reach OutputWidth/Height
	// exactly (intermediate was bigger than target). Explicitly clear
	// P1CRSTR so HSTART/VSTART = 0 (top-left origin).
	d.bus.P1CRSTR.Set(0)
	if interW > uint32(cfg.OutputWidth) || interH > uint32(cfg.OutputHeight) {
		d.bus.P1CRSZR.Set(
			uint32(cfg.OutputWidth) |
				(uint32(cfg.OutputHeight) << stm32.DCMIPP_P1CRSZR_VSIZE_Pos) |
				stm32.DCMIPP_P1CRSZR_ENABLE,
		)
	} else {
		d.bus.P1CRSZR.Set(0)
	}

	// Pixel packer: pure FORMAT field; nothing else needed for YUV422
	// 1-buffer. Pitch = OutputWidth × 2 bytes (each pixel is one byte
	// of luma + half a byte of chroma, packed per YUYV).
	d.bus.P1PPCR.Set(uint32(cfg.Format) << stm32.DCMIPP_P1PPCR_FORMAT_Pos)
	d.bus.P1PPM0PR.Set(uint32(cfg.OutputWidth) * 2)

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
// 16-byte aligned. The pipe writes via the same AXI master as Pipe 0, so
// the same RIF / RISAF setup applies.
func (d *dcmippDevice) StartPipe1(buf []byte, mode DCMIPPCaptureMode) {
	if len(buf) < 4 {
		return
	}
	addr := uintptr(unsafe.Pointer(&buf[0]))

	// Disarm to avoid mid-flight half-config.
	d.bus.P1FCTCR.Set(0)
	d.bus.P1FSCR.ClearBits(stm32.DCMIPP_P1FSCR_PIPEN)

	// Set capture mode bit (CPTMODE for snapshot) and destination addr.
	if mode == DCMIPPCaptureSnapshot {
		d.bus.P1FCTCR.Set(stm32.DCMIPP_P1FCTCR_CPTMODE)
	}
	d.bus.P1PPM0AR1.Set(uint32(addr))

	// Activate pipe + start capture (PIPEN and CPTREQ together, matching
	// HAL_DCMIPP_PIPE_Start's DCMIPP_EnableCapture).
	d.bus.P1FSCR.SetBits(stm32.DCMIPP_P1FSCR_PIPEN)
	d.bus.P1FCTCR.SetBits(stm32.DCMIPP_P1FCTCR_CPTREQ)
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
