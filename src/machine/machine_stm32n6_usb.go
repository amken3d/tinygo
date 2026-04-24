//go:build stm32n6

package machine

// USB device-controller driver for the STM32N6 OTG1 core.
//
// The N6's OTG cores are standard Synopsys DWC2 IP. The programming model
// is FIFO-based (no dedicated-per-endpoint RAM, no DMA in this driver):
//
//   - One shared RX FIFO drains into device via GRXSTSP + DFIFO0 reads.
//   - Each IN endpoint has its own TX FIFO; data is pushed via DFIFO[ep].
//   - DFIFO register windows start at core+0x1000 with 0x1000-byte stride.
//
// Current state: Full-Speed on the integrated HS PHY. DCFG.DSPD=1 (not 3 —
// N6 has no separate FS-only PHY). PHY reference clock is CLKP at 20 MHz,
// routed from PLL1 / IC5 / CLKP — matches ST's CubeMX NUCLEO-N657 config.
// VDDUSB (PWR.SVMCR3.USB33SV) must be enabled before the PHY will drive
// the bus.
//
// Servicing: currently polled from the main loop via PollUSB(). The
// NVIC-dispatched path is wired up but the OTG1 hardware IRQ doesn't
// fire reliably on this chip — root cause unknown; polling on a 600 MHz
// M55 is cheap enough that this is not blocking. See PollUSB().
//
// Scope: no DMA, no host mode, no SOF handling, no HS signalling, no iso
// endpoints. CDC-ACM works; MSC / UVC / HS are future phases.

import (
	"device/arm"
	"device/stm32"
	"machine/usb"
	"runtime/interrupt"
	"runtime/volatile"
	"unsafe"
)

// NumberOfUSBEndpoints is the count of device endpoints the N6 OTG1 core
// exposes to this driver (EP0 + EP1..EP8).
const NumberOfUSBEndpoints = 9

// Synopsys DWC2 register bit definitions. These are not in the SVD as
// named flags (only as Set*/Get* helpers per-field), so I decided to spell them out
// locally — it keeps the core-bringup sequences readable. Values match
// the STM32N6 Reference Manual (RM0486) OTG peripheral chapter.
const (
	// GRSTCTL
	otgGRSTCTL_CSRST      = 1 << 0
	otgGRSTCTL_RXFFLSH    = 1 << 4
	otgGRSTCTL_TXFFLSH    = 1 << 5
	otgGRSTCTL_TXFNUM_Pos = 6
	otgGRSTCTL_TXFNUM_ALL = 0x10 << 6
	otgGRSTCTL_AHBIDL     = 1 << 31

	// GUSBCFG
	otgGUSBCFG_TRDT_Pos = 10
	otgGUSBCFG_PHYSEL   = 1 << 6
	otgGUSBCFG_TSDPS    = 1 << 22
	otgGUSBCFG_FHMOD    = 1 << 29
	otgGUSBCFG_FDMOD    = 1 << 30

	// GAHBCFG
	otgGAHBCFG_GINTMSK = 1 << 0
	otgGAHBCFG_TXFELVL = 1 << 7

	// GCCFG — charger/PHY helper bits. On N6 the layout is:
	//   bit 23 VBVALOVAL   — value of the overridden VBUS-valid input
	//   bit 24 VBVALEXTOEN — enable the VBUS-valid external override
	//   bit 25 PULLDOWNEN  — pull-down resistors (host-mode only)
	// With VBUS sensing disabled we need BOTH override bits set so the
	// PHY sees a stable "VBUS present" state; with device mode selected
	// we must also clear PULLDOWNEN so the D+ pull-up wins on attach.
	otgGCCFG_VBVALOVAL   = 1 << 23
	otgGCCFG_VBVALEXTOEN = 1 << 24
	otgGCCFG_PULLDOWNEN  = 1 << 25

	// GINTSTS / GINTMSK (same bit positions; GINTSTS is W1C)
	otgGINTSTS_MMIS     = 1 << 1
	otgGINTSTS_OTGINT   = 1 << 2
	otgGINTSTS_SOF      = 1 << 3
	otgGINTSTS_RXFLVL   = 1 << 4
	otgGINTSTS_ESUSP    = 1 << 10
	otgGINTSTS_USBSUSP  = 1 << 11
	otgGINTSTS_USBRST   = 1 << 12
	otgGINTSTS_ENUMDNE  = 1 << 13
	otgGINTSTS_IEPINT   = 1 << 18
	otgGINTSTS_OEPINT   = 1 << 19
	otgGINTSTS_IISOIXFR = 1 << 20
	otgGINTSTS_RESETDET = 1 << 23
	otgGINTSTS_WKUINT   = 1 << 31

	// GRXSTSR/P decoded fields
	otgGRXSTS_EPNUM_Msk  = 0xF
	otgGRXSTS_BCNT_Pos   = 4
	otgGRXSTS_BCNT_Msk   = 0x7FF << 4
	otgGRXSTS_DPID_Pos   = 15
	otgGRXSTS_PKTSTS_Pos = 17
	otgGRXSTS_PKTSTS_Msk = 0xF << 17
	// device-mode PKTSTS values
	otgPKTSTS_GOUTNAK    = 0x1
	otgPKTSTS_OUT_RECV   = 0x2
	otgPKTSTS_OUT_COMP   = 0x3
	otgPKTSTS_SETUP_COMP = 0x4
	otgPKTSTS_SETUP_RECV = 0x6

	// DCFG / DCTL / DSTS. DSPD encoding on the N6 HS-embedded PHY:
	//   0 = High-speed
	//   1 = Full-speed (using internal HS PHY) ← what N6 wants for FS
	//   3 = Full-speed (dedicated FS PHY, not present on N6)
	otgDCFG_DSPD_HS = 0
	otgDCFG_DSPD_FS = 1
	otgDCFG_DAD_Pos = 4
	otgDCFG_DAD_Msk = 0x7F << 4
	otgDCTL_RWUSIG  = 1 << 0
	otgDCTL_SDIS    = 1 << 1
	otgDCTL_SGINAK  = 1 << 7
	otgDCTL_CGINAK  = 1 << 8
	otgDCTL_SGONAK  = 1 << 9
	otgDCTL_CGONAK  = 1 << 10
	otgDSTS_ENUMSPD = 0x3 << 1

	// DIEPCTL / DOEPCTL shared bits
	otgEPCTL_USBAEP     = 1 << 15
	otgEPCTL_NAKSTS     = 1 << 17
	otgEPCTL_EPTYP_Pos  = 18
	otgEPCTL_STALL      = 1 << 21
	otgEPCTL_TXFNUM_Pos = 22
	otgEPCTL_CNAK       = 1 << 26
	otgEPCTL_SNAK       = 1 << 27
	otgEPCTL_SD0PID     = 1 << 28
	otgEPCTL_EPDIS      = 1 << 30
	otgEPCTL_EPENA      = 1 << 31
	// EP0 only: MPSIZ encoding 0=64, 1=32, 2=16, 3=8
	otgEP0CTL_MPSIZ_64 = 0
	otgEP0CTL_MPSIZ_32 = 1

	// DIEPINT / DOEPINT
	otgDIEPINT_XFRC   = 1 << 0
	otgDIEPINT_EPDISD = 1 << 1
	otgDIEPINT_TOC    = 1 << 3
	otgDIEPINT_ITTXFE = 1 << 4
	otgDIEPINT_INEPNE = 1 << 6
	otgDIEPINT_TXFE   = 1 << 7
	otgDOEPINT_XFRC   = 1 << 0
	otgDOEPINT_EPDISD = 1 << 1
	otgDOEPINT_STUP   = 1 << 3

	// DIEPTSIZ0 / DOEPTSIZ0 (EP0 has narrow PKTCNT field)
	otgDIEPTSIZ0_PKTCNT_Pos  = 19
	otgDIEPTSIZ_PKTCNT_Pos   = 19
	otgDIEPTSIZ_PKTCNT_Msk   = 0x3FF << 19
	otgDOEPTSIZ0_STUPCNT_Pos = 29
	otgDOEPTSIZ0_PKTCNT      = 1 << 19

	// DAINTMSK
	otgDAINTMSK_IEP_Pos = 0
	otgDAINTMSK_OEP_Pos = 16
)

// FIFO sizing (in 32-bit words). Must fit the core's dedicated RAM; for
// the N6 OTG1 FS path the RAM budget is at least 1.25 KB of words. We lay
// out:
//
//	RX shared:            128 words (=512 B) — enough for one bulk-out + EP0 setup overhead.
//	EP0 TX (non-periodic): 32 words (=128 B)
//	EP1 TX (CDC notif IN): 16 words  (= 64 B)
//	EP2 TX (CDC data IN):  64 words  (=256 B)
//	others: 0
//
// Addresses are expressed as (depth << 16) | start, same word units.
const (
	otgRXFIFO_DEPTH  = 128
	otgTXFIFO0_DEPTH = 32
	otgTXFIFO1_DEPTH = 16
	otgTXFIFO2_DEPTH = 64
)

// FIFO memory window access. On DWC2 the FIFO RAM is mapped at core+0x1000
// with a 0x1000-byte stride per endpoint. Writing to fifoWin[n] pushes one
// 32-bit word into EP n's TX FIFO; reading from fifoWin[0] pops one word
// from the shared RX FIFO.
//
// The OTG_Type struct ends at 0xE04, so these windows are outside the
// struct and we compute them from the OTG1 base address at init time.
var fifoWin [NumberOfUSBEndpoints]*volatile.Register32

func init() {
	base := uintptr(unsafe.Pointer(stm32.OTG1))
	for i := 0; i < NumberOfUSBEndpoints; i++ {
		addr := base + 0x1000 + uintptr(i)*0x1000
		fifoWin[i] = (*volatile.Register32)(unsafe.Pointer(addr))
	}
}

// endPoints is the type|direction table consumed by generic usb.go when
// it walks SET_CONFIGURATION and calls initEndpoint for each entry. The
// control slot is always present; class drivers mutate the others via
// ConfigureUSBEndpoint.
var endPoints = []uint32{
	usb.CONTROL_ENDPOINT:  usb.ENDPOINT_TYPE_CONTROL,
	usb.CDC_ENDPOINT_ACM:  (usb.ENDPOINT_TYPE_INTERRUPT | usb.EndpointIn),
	usb.CDC_ENDPOINT_OUT:  (usb.ENDPOINT_TYPE_BULK | usb.EndpointOut),
	usb.CDC_ENDPOINT_IN:   (usb.ENDPOINT_TYPE_BULK | usb.EndpointIn),
	usb.HID_ENDPOINT_IN:   usb.ENDPOINT_TYPE_DISABLE,
	usb.HID_ENDPOINT_OUT:  usb.ENDPOINT_TYPE_DISABLE,
	usb.MIDI_ENDPOINT_IN:  usb.ENDPOINT_TYPE_DISABLE,
	usb.MIDI_ENDPOINT_OUT: usb.ENDPOINT_TYPE_DISABLE,
	8:                     usb.ENDPOINT_TYPE_DISABLE,
}

// Per-endpoint OUT staging. Setup packets are 8 bytes; bulk-out is at
// most one 64-byte packet per transfer in this driver. We copy out of the
// FIFO into these buffers in the RXFLVL handler, then dispatch on XFRC.
var (
	setupPkt   [8]byte
	setupReady bool

	epRxBuffer [NumberOfUSBEndpoints][64]byte
	epRxCount  [NumberOfUSBEndpoints]uint16

	// Pending IN to complete on EP0 data stage (descriptor transfer, etc.).
	// The generic dispatcher hands us the full payload in one sendUSBPacket
	// call; for EP0 FS the MPS is 64 and the core's EP0 TSIZ.PKTCNT is only
	// 2 bits (max 3 packets per transfer), so anything longer than 192 B
	// must be chunked. Configuration + HID-report descriptors can hit this;
	// string descriptors typically do not.
	ep0InBuf    []byte
	ep0InOffset int
)

var otg = stm32.OTG1

// usbIRQ is the shared interrupt.Interrupt handle; reinitialising without
// duplicating the registration.
var usbIRQ interrupt.Interrupt

// Configure the USB peripheral. Matches the UART-compatible signature used
// by every other chip's USB driver. The register sequence here mirrors
// ST's HAL (stm32n6xx_ll_usb.c / stm32n6xx_hal_pcd.c) step-for-step —
// the HAL is the only working reference for the N6 PCD, and divergence
// from it reliably breaks enumeration.
func (dev *USBDevice) Configure(config UARTConfig) {
	if dev.initcomplete {
		return
	}

	// --- VDDUSB 3.3 V supply switch ------------------------------------
	//
	// The N6 has an independent USB 3.3 V domain gated off at reset.
	// Without this the PHY analog front-end never powers up and the host
	// sees nothing on attach. Mirror HAL_PWREx_EnableVddUSB: set
	// PWR.SVMCR3.USB33SV, wait for USB33RDY (bounded — if the rail isn't
	// present on the board the poll would otherwise hang forever).
	stm32.PWR.SVMCR3.SetBits(stm32.PWR_SVMCR3_USB33SV)
	for i := 0; i < 1_000_000; i++ {
		if stm32.PWR.SVMCR3.Get()&stm32.PWR_SVMCR3_USB33RDY != 0 {
			break
		}
	}

	// --- PHY reference clock: CLKP = IC5 = PLL1 / 80 = 20 MHz ----------
	//
	// This mirrors the NUCLEO-N657 CubeMX clock tree. HSE/2 = 24 MHz is
	// a standard DWC2 PHY ref freq on paper, but on this silicon the
	// PHY apparently only locks on the ST-validated 20 MHz path — an
	// earlier revision of this driver tried HSE/2 and the host never saw
	// enumeration traffic.
	//
	// Assumes initCLK already brought PLL1 up at 1600 MHz VCO
	// (HSI×25 → 1600). If that changes, adjust IC5INT accordingly.
	stm32.RCC.IC5CFGR.Set(
		(0 << 28) | // IC5SEL = 0 (PLL1)
			(79 << 16), // IC5INT = divider-1; div 80 → 1600/80 = 20 MHz
	)
	stm32.RCC.DIVENSR.Set(stm32.RCC_DIVENR_IC5EN)

	// CKPER source = IC5 (PERSEL=4); then enable CKPER (MISCENSR.PERENS).
	stm32.RCC.CCIPR7.ReplaceBits(4, stm32.RCC_CCIPR7_PERSEL_Msk, 0)
	stm32.RCC.MISCENSR.Set(stm32.RCC_MISCENSR_PERENS)

	// OTGPHY1SEL = 0b01 (CLKP), OTGPHY1CKREFSEL = 0 (use same source).
	// 0b01 at position 12 → value 0x1000.
	stm32.RCC.CCIPR6.ReplaceBits(
		0x1000,
		stm32.RCC_CCIPR6_OTGPHY1SEL_Msk|stm32.RCC_CCIPR6_OTGPHY1CKREFSEL_Msk,
		0,
	)

	// --- OTG + PHY peripheral clocks -----------------------------------
	stm32.RCC.AHB5ENSR.Set(stm32.RCC_AHB5ENR_OTGPHY1EN | stm32.RCC_AHB5ENR_OTG1EN)
	_ = stm32.RCC.AHB5ENR.Get()

	// Pulse the peripheral reset (RSTSR = set-latches, RSTCR = clear).
	stm32.RCC.AHB5RSTSR.Set(stm32.RCC_AHB5RSTR_OTG1RST | stm32.RCC_AHB5RSTR_OTGPHY1RST)
	for i := 0; i < 256; i++ {
		arm.Asm("nop")
	}
	stm32.RCC.AHB5RSTCR.Set(stm32.RCC_AHB5RSTR_OTG1RST | stm32.RCC_AHB5RSTR_OTGPHY1RST)
	_ = stm32.RCC.AHB5RSTR.Get()

	// --- USB_CoreInit (HAL path for HS embedded PHY) -------------------
	//
	// 1. Clear TSDPS (data-line pulsing via utmi_txvalid is what HS PHY
	//    expects — TSDPS selects an alternate pulsing scheme).
	// 2. Soft-reset the core and wait for AHB master idle.
	otg.GUSBCFG.ClearBits(otgGUSBCFG_TSDPS)
	for i := 0; i < 1_000_000; i++ {
		if otg.GRSTCTL.Get()&otgGRSTCTL_AHBIDL != 0 {
			break
		}
	}
	otg.GRSTCTL.Set(otgGRSTCTL_CSRST)
	for i := 0; i < 1_000_000; i++ {
		if otg.GRSTCTL.Get()&otgGRSTCTL_CSRST == 0 {
			break
		}
	}
	for i := 0; i < 1_000_000; i++ {
		if otg.GRSTCTL.Get()&otgGRSTCTL_AHBIDL != 0 {
			break
		}
	}

	// --- USB_SetCurrentMode(DEVICE) ------------------------------------
	// Clear FHMOD+FDMOD, set FDMOD, then spin until GINTSTS.CMOD reflects
	// device mode. Mode-switch settle can take up to ~25 ms so we budget
	// a 50 ms equivalent busy-loop (at 600 MHz CPU, ~30M iterations).
	gusbcfg := otg.GUSBCFG.Get()
	gusbcfg &^= otgGUSBCFG_FHMOD | otgGUSBCFG_FDMOD | (0xF << otgGUSBCFG_TRDT_Pos)
	gusbcfg |= otgGUSBCFG_FDMOD | (9 << otgGUSBCFG_TRDT_Pos)
	otg.GUSBCFG.Set(gusbcfg)
	for i := 0; i < 30_000_000; i++ {
		if otg.GINTSTS.Get()&1 == 0 { // CMOD == 0 → device
			break
		}
	}

	// --- USB_DevInit ---------------------------------------------------
	//
	// Zero all per-EP TX-FIFO config; we'll program the ones we need
	// after FIFO layout below.
	otg.DIEPTXF1.Set(0)
	otg.DIEPTXF2.Set(0)

	// VBUS sensing is disabled on NUCLEO-N657 (no dedicated VBUS pad).
	// When VBUS sense is off the PHY needs the EXT-override to stay
	// permanently "valid"; also clear PULLDOWNEN so device-mode pull-ups
	// on D+ win on attach.
	otg.GCCFG.ClearBits(otgGCCFG_PULLDOWNEN)
	otg.GCCFG.SetBits(otgGCCFG_VBVALEXTOEN | otgGCCFG_VBVALOVAL)

	// Soft-disconnect while we finish configuring.
	otg.DCTL.SetBits(otgDCTL_SDIS)

	// Restart the PHY clock (HAL comment: "USBx_PCGCCTL = 0"). On N6 this
	// clears STPPCLK + GATEHCLK if left set by a prior suspend.
	otg.PCGCCTL.Set(0)

	// Set device speed. For the HS-embedded PHY, "FS" is encoded as
	// USB_OTG_SPEED_HIGH_IN_FULL (DSPD=1). HAL: `USBx_DEVICE->DCFG |= speed;`
	dcfg := otg.DCFG.Get()
	dcfg &^= 0x3
	dcfg |= otgDCFG_DSPD_FS
	otg.DCFG.Set(dcfg)

	// --- FIFO layout ---------------------------------------------------
	// RXFSIZ = shared-OUT depth; HNPTXFSIZ is the EP0 TX FIFO in device
	// mode (low 16: start word, high 16: depth). DIEPTXFn: per-IN-EP.
	otg.GRXFSIZ.Set(otgRXFIFO_DEPTH)
	otg.HNPTXFSIZ.Set((otgTXFIFO0_DEPTH << 16) | otgRXFIFO_DEPTH)
	otg.DIEPTXF1.Set((otgTXFIFO1_DEPTH << 16) | (otgRXFIFO_DEPTH + otgTXFIFO0_DEPTH))
	otg.DIEPTXF2.Set((otgTXFIFO2_DEPTH << 16) | (otgRXFIFO_DEPTH + otgTXFIFO0_DEPTH + otgTXFIFO1_DEPTH))

	// Flush TX FIFOs (all) then RX FIFO.
	otg.GRSTCTL.Set(otgGRSTCTL_TXFFLSH | otgGRSTCTL_TXFNUM_ALL)
	for otg.GRSTCTL.Get()&otgGRSTCTL_TXFFLSH != 0 {
	}
	otg.GRSTCTL.Set(otgGRSTCTL_RXFFLSH)
	for otg.GRSTCTL.Get()&otgGRSTCTL_RXFFLSH != 0 {
	}

	// Disable all endpoint / address masks + clear pending per-EP IRQs.
	otg.DIEPMSK.Set(0)
	otg.DOEPMSK.Set(0)
	otg.DAINTMSK.Set(0)
	otg.DIEPEMPMSK.Set(0)
	for ep := uint32(0); ep < NumberOfUSBEndpoints; ep++ {
		diepctl := diepctlReg(ep)
		if diepctl.Get()&otgEPCTL_EPENA != 0 {
			if ep == 0 {
				diepctl.Set(otgEPCTL_SNAK)
			} else {
				diepctl.Set(otgEPCTL_EPDIS | otgEPCTL_SNAK)
			}
		} else {
			diepctl.Set(0)
		}
		dieptsizReg(ep).Set(0)
		diepintReg(ep).Set(0xFB7F)

		doepctl := doepctlReg(ep)
		if doepctl.Get()&otgEPCTL_EPENA != 0 {
			if ep == 0 {
				doepctl.Set(otgEPCTL_SNAK)
			} else {
				doepctl.Set(otgEPCTL_EPDIS | otgEPCTL_SNAK)
			}
		} else {
			doepctl.Set(0)
		}
		doeptsizReg(ep).Set(0)
		doepintReg(ep).Set(0xFB7F)
	}

	// Clear GINTSTS (HAL uses 0xBFFFFFFF — bit 30 SRQINT stays pending
	// because it's not a simple W1C) then enable our interrupts.
	otg.GINTSTS.Set(0xBFFFFFFF)
	otg.GINTMSK.Set(
		otgGINTSTS_RXFLVL |
			otgGINTSTS_USBSUSP |
			otgGINTSTS_USBRST |
			otgGINTSTS_ENUMDNE |
			otgGINTSTS_IEPINT |
			otgGINTSTS_OEPINT |
			otgGINTSTS_IISOIXFR |
			otgGINTSTS_RESETDET |
			otgGINTSTS_WKUINT,
	)

	// NVIC: register + enable OTG1 vector.
	usbIRQ = interrupt.New(stm32.IRQ_OTG1, handleUSBIRQ)
	usbIRQ.SetPriority(0x40)
	usbIRQ.Enable()

	// Enable the core's global interrupt output (AHB side).
	otg.GAHBCFG.SetBits(otgGAHBCFG_GINTMSK)

	// Release soft-disconnect — host now sees pull-up on D+ and starts
	// enumeration.
	otg.DCTL.ClearBits(otgDCTL_SDIS)

	dev.initcomplete = true
}

// SetStallEPIn stalls the IN endpoint `ep`. Called by the generic stack
// when it has no handler for a SETUP request.
func (dev *USBDevice) SetStallEPIn(ep uint32) {
	diepctl := diepctlReg(ep)
	diepctl.SetBits(otgEPCTL_STALL)
}

// IRQ handler.

func handleUSBIRQ(intr interrupt.Interrupt) {
	sts := otg.GINTSTS.Get() & otg.GINTMSK.Get()

	if sts&otgGINTSTS_USBRST != 0 {
		handleUSBReset()
		otg.GINTSTS.Set(otgGINTSTS_USBRST)
	}

	if sts&otgGINTSTS_ENUMDNE != 0 {
		handleEnumDone()
		otg.GINTSTS.Set(otgGINTSTS_ENUMDNE)
	}

	if sts&otgGINTSTS_RXFLVL != 0 {
		// RXFLVL auto-clears when the FIFO drains; we loop inside.
		drainRxFifo()
	}

	if sts&otgGINTSTS_IEPINT != 0 {
		handleIEPInt()
	}

	if sts&otgGINTSTS_OEPINT != 0 {
		handleOEPInt()
	}

	if sts&(otgGINTSTS_USBSUSP|otgGINTSTS_WKUINT|otgGINTSTS_RESETDET) != 0 {
		otg.GINTSTS.Set(otgGINTSTS_USBSUSP | otgGINTSTS_WKUINT | otgGINTSTS_RESETDET)
	}
}

// handleUSBReset: host has issued a bus reset. Re-arm EP0 control and
// clear all IN/OUT endpoint state so the next SETUP starts clean.
func handleUSBReset() {
	// Clear our stale device state.
	usbConfiguration = 0
	setupReady = false
	ep0InBuf = nil
	ep0InOffset = 0
	for i := range epRxCount {
		epRxCount[i] = 0
	}

	// Disable all OUT endpoints (except EP0), clear pending IRQs.
	for ep := uint32(1); ep < NumberOfUSBEndpoints; ep++ {
		doepctl := doepctlReg(ep)
		if doepctl.Get()&otgEPCTL_EPENA != 0 {
			doepctl.SetBits(otgEPCTL_EPDIS | otgEPCTL_SNAK)
		}
		doepintReg(ep).Set(0xFFFFFFFF)
		diepctl := diepctlReg(ep)
		if diepctl.Get()&otgEPCTL_EPENA != 0 {
			diepctl.SetBits(otgEPCTL_EPDIS | otgEPCTL_SNAK)
		}
		diepintReg(ep).Set(0xFFFFFFFF)
	}

	// Reset device address.
	dcfg := otg.DCFG.Get()
	dcfg &^= otgDCFG_DAD_Msk
	otg.DCFG.Set(dcfg)

	// Enable EP0 IN/OUT master interrupts + standard event masks.
	otg.DAINTMSK.Set((1 << 0) | (1 << 16)) // IN0 + OUT0
	otg.DOEPMSK.Set(otgDOEPINT_XFRC | otgDOEPINT_STUP | otgDOEPINT_EPDISD)
	otg.DIEPMSK.Set(otgDIEPINT_XFRC | otgDIEPINT_TOC | otgDIEPINT_EPDISD)

	// Arm EP0 to receive the first SETUP.
	initEndpoint(0, usb.ENDPOINT_TYPE_CONTROL)
}

// handleEnumDone: speed negotiated. We programmed FS so just keep EP0
// at 64-byte MPS. If we ever flip to HS we'd clamp/adjust here.
func handleEnumDone() {
	diepctl := diepctlReg(0)
	v := diepctl.Get()
	v &^= 0x3 // MPSIZ field on EP0
	v |= otgEP0CTL_MPSIZ_64
	diepctl.Set(v)

	// Clear global IN/OUT NAK (core reset leaves them set).
	otg.DCTL.SetBits(otgDCTL_CGINAK | otgDCTL_CGONAK)
}

// drainRxFifo pops every packet currently in the shared RX FIFO and
// dispatches it. Runs in IRQ context. Called from the master ISR and also
// re-entered from ReceiveUSBControlPacket's synchronous poll loop.
func drainRxFifo() {
	for otg.GINTSTS.Get()&otgGINTSTS_RXFLVL != 0 {
		// Mask RXFLVL while we pop one packet — the core re-asserts it
		// when another is waiting.
		otg.GINTMSK.ClearBits(otgGINTSTS_RXFLVL)

		status := otg.GRXSTSP.Get()
		ep := status & otgGRXSTS_EPNUM_Msk
		bcnt := (status & otgGRXSTS_BCNT_Msk) >> otgGRXSTS_BCNT_Pos
		pktsts := (status & otgGRXSTS_PKTSTS_Msk) >> otgGRXSTS_PKTSTS_Pos

		// Guard against unexpected EP numbers (4-bit field, we only size
		// tables for 9 EPs). Drain the FIFO payload if any, then skip.
		if ep >= NumberOfUSBEndpoints {
			leftover := int(bcnt)
			for leftover > 0 {
				_ = fifoWin[0].Get()
				leftover -= 4
			}
			otg.GINTMSK.SetBits(otgGINTSTS_RXFLVL)
			continue
		}

		switch pktsts {
		case otgPKTSTS_SETUP_RECV:
			// 8-byte setup; always into setupPkt[]. The actual dispatch
			// happens when OEPINT.STUP later fires for EP0.
			fifoReadInto(setupPkt[:8])

		case otgPKTSTS_SETUP_COMP:
			setupReady = true

		case otgPKTSTS_OUT_RECV:
			n := int(bcnt)
			if n > len(epRxBuffer[ep]) {
				n = len(epRxBuffer[ep])
			}
			fifoReadInto(epRxBuffer[ep][:n])
			epRxCount[ep] = uint16(n)
			leftover := int(bcnt) - n
			for leftover > 0 {
				_ = fifoWin[0].Get()
				leftover -= 4
			}

		case otgPKTSTS_OUT_COMP, otgPKTSTS_GOUTNAK:
			// Transfer-level completion / NAK — dispatched via OEPINT.
		}

		otg.GINTMSK.SetBits(otgGINTSTS_RXFLVL)
	}
}

// handleIEPInt services IN-endpoint interrupts (XFRC etc.). The IEPINT
// bit in GINTSTS is read-only; it clears when all per-EP DIEPINT bits are
// cleared.
func handleIEPInt() {
	daint := otg.DAINT.Get() & otg.DAINTMSK.Get()
	for ep := uint32(0); ep < NumberOfUSBEndpoints; ep++ {
		if daint&(1<<ep) == 0 {
			continue
		}
		intReg := diepintReg(ep)
		flags := intReg.Get()

		if flags&otgDIEPINT_XFRC != 0 {
			intReg.Set(otgDIEPINT_XFRC)

			if ep == 0 {
				// Either continuing a chunked descriptor send, or we
				// just finished the last packet.
				if ep0InOffset < len(ep0InBuf) {
					ep0SendChunk()
				} else {
					ep0InBuf = nil
					ep0InOffset = 0
					// After EP0 IN completes, host will send OUT status
					// stage (ZLP). Re-arm OUT for it.
					armEP0Out()
				}
			} else if usbTxHandler[ep] != nil {
				usbTxHandler[ep]()
			}
		}

		// Clear any leftover maskable flags.
		intReg.Set(flags & (otgDIEPINT_EPDISD | otgDIEPINT_TOC | otgDIEPINT_ITTXFE | otgDIEPINT_INEPNE))
	}
}

// handleOEPInt services OUT-endpoint interrupts (SETUP/XFRC).
func handleOEPInt() {
	daint := (otg.DAINT.Get() & otg.DAINTMSK.Get()) >> 16
	for ep := uint32(0); ep < NumberOfUSBEndpoints; ep++ {
		if daint&(1<<ep) == 0 {
			continue
		}
		intReg := doepintReg(ep)
		flags := intReg.Get()

		if flags&otgDOEPINT_STUP != 0 {
			intReg.Set(otgDOEPINT_STUP)
			setupReady = false
			dispatchSetup()
		} else if flags&otgDOEPINT_XFRC != 0 {
			intReg.Set(otgDOEPINT_XFRC)
			if ep == 0 {
				// End of an EP0 OUT data / status stage. Re-arm for the
				// next SETUP.
				armEP0Out()
			} else {
				dispatchRx(ep)
			}
		}

		intReg.Set(flags & otgDOEPINT_EPDISD)
	}
}

// dispatchSetup parses the 8-byte SETUP packet we captured in drainRxFifo
// and hands it off to the standard-request handler or class-setup handler.
func dispatchSetup() {
	setup := usb.NewSetup(setupPkt[:])

	// The generic stack sometimes queues data on EP0 as part of standard
	// handling. That's OK — our sendUSBPacket will chunk through.
	ep0InBuf = nil
	ep0InOffset = 0

	ok := false
	if setup.BmRequestType&usb.REQUEST_TYPE == usb.REQUEST_STANDARD {
		ok = handleStandardSetup(setup)
	} else if setup.WIndex < uint16(len(usbSetupHandler)) && usbSetupHandler[setup.WIndex] != nil {
		ok = usbSetupHandler[setup.WIndex](setup)
	}

	if !ok {
		USBDev.SetStallEPIn(0)
	}

	// If the request had no data stage and no handler scheduled a send,
	// status-stage ZLP arms via armEP0Out in handleOEPInt when the host
	// sends the OUT status.
	armEP0Out()
}

// dispatchRx delivers a completed OUT transfer to its handler and re-arms
// the endpoint.
func dispatchRx(ep uint32) {
	n := epRxCount[ep]
	buf := epRxBuffer[ep][:n]
	epRxCount[ep] = 0
	if usbRxHandler[ep] == nil || usbRxHandler[ep](buf) {
		AckUsbOutTransfer(ep)
	}
}

// initEndpoint is called by generic usb.go during SET_CONFIGURATION (and
// directly after bus reset for EP0). The `config` value is the same
// type|direction bitfield we stored in endPoints[].
func initEndpoint(ep, config uint32) {
	switch config {
	case usb.ENDPOINT_TYPE_CONTROL:
		// EP0 IN: MPSIZ=64 (encoded as 0), USBAEP is always set on EP0.
		diepctlReg(0).Set(otgEP0CTL_MPSIZ_64)
		// EP0 OUT: MPSIZ=64 ditto.
		doepctlReg(0).Set(otgEP0CTL_MPSIZ_64)
		// Arm EP0 OUT to receive SETUP.
		armEP0Out()

	case usb.ENDPOINT_TYPE_BULK | usb.EndpointIn:
		configureInEP(ep, 2, 64) // EPTYP=2 (bulk), MPS=64

	case usb.ENDPOINT_TYPE_BULK | usb.EndpointOut:
		configureOutEP(ep, 2, 64)

	case usb.ENDPOINT_TYPE_INTERRUPT | usb.EndpointIn:
		configureInEP(ep, 3, 64)

	case usb.ENDPOINT_TYPE_INTERRUPT | usb.EndpointOut:
		configureOutEP(ep, 3, 64)

	case usb.ENDPOINT_TYPE_ISOCHRONOUS | usb.EndpointIn:
		configureInEP(ep, 1, 64)

	case usb.ENDPOINT_TYPE_ISOCHRONOUS | usb.EndpointOut:
		configureOutEP(ep, 1, 64)

	case usb.ENDPOINT_TYPE_DISABLE:
		// leave the EP off.
	}
}

// configureInEP opens a non-control IN endpoint.
func configureInEP(ep uint32, eptyp, mps uint32) {
	// Pick a TX FIFO number for this EP. For the simple CDC layout we
	// hard-map: EP1 -> TXFIFO1 (notification), EP3 -> TXFIFO2 (bulk).
	var txfnum uint32
	switch ep {
	case usb.CDC_ENDPOINT_ACM:
		txfnum = 1
	case usb.CDC_ENDPOINT_IN:
		txfnum = 2
	default:
		txfnum = ep
	}

	diepctlReg(ep).Set(
		mps |
			(eptyp << otgEPCTL_EPTYP_Pos) |
			otgEPCTL_USBAEP |
			(txfnum << otgEPCTL_TXFNUM_Pos) |
			otgEPCTL_SD0PID |
			otgEPCTL_SNAK,
	)
	otg.DAINTMSK.SetBits(1 << ep)
}

// configureOutEP opens a non-control OUT endpoint and arms it for one
// MPS-sized packet.
func configureOutEP(ep uint32, eptyp, mps uint32) {
	doepctlReg(ep).Set(
		mps |
			(eptyp << otgEPCTL_EPTYP_Pos) |
			otgEPCTL_USBAEP |
			otgEPCTL_SD0PID |
			otgEPCTL_CNAK |
			otgEPCTL_EPENA,
	)
	doeptsizReg(ep).Set((1 << otgDIEPTSIZ_PKTCNT_Pos) | mps)
	otg.DAINTMSK.SetBits(1 << (ep + 16))
}

// armEP0Out arms EP0 OUT to accept up to 3 back-to-back SETUP packets or
// one data/status OUT. STUPCNT=3 + PKTCNT=1 + XFRSIZ=64 is the canonical
// EP0-idle programming.
func armEP0Out() {
	otg.DOEPTSIZ0.Set(
		(3 << otgDOEPTSIZ0_STUPCNT_Pos) |
			otgDOEPTSIZ0_PKTCNT |
			64,
	)
	doepctlReg(0).SetBits(otgEPCTL_EPENA | otgEPCTL_CNAK)
}

// sendUSBPacket enqueues one TX packet. Called by generic usb.go for EP0
// descriptor responses and by class drivers for their own IN endpoints.
// For EP0, anything longer than 3*64 bytes is chunked across multiple
// transfers by the XFRC handler (ep0SendChunk).
//
//go:noinline
func sendUSBPacket(ep uint32, data []byte) {
	if ep == 0 {
		ep0InBuf = data
		ep0InOffset = 0
		ep0SendChunk()
		return
	}

	// Non-EP0: single shot — caller is responsible for chunking at class
	// level if needed (CDC sends one MPS-sized chunk per kickTx).
	n := uint32(len(data))
	dieptsizReg(ep).Set((1 << otgDIEPTSIZ_PKTCNT_Pos) | n)
	diepctlReg(ep).SetBits(otgEPCTL_EPENA | otgEPCTL_CNAK)
	fifoWrite(ep, data)
}

// ep0SendChunk sends the next chunk of ep0InBuf (up to 3 packets of 64 B
// each). Called from sendUSBPacket for the first chunk and from the XFRC
// IRQ for subsequent ones.
func ep0SendChunk() {
	remaining := len(ep0InBuf) - ep0InOffset
	const maxEP0Chunk = 3 * 64
	if remaining > maxEP0Chunk {
		remaining = maxEP0Chunk
	}

	if remaining == 0 {
		// Final status-stage ZLP.
		otg.DIEPTSIZ0.Set((1 << otgDIEPTSIZ0_PKTCNT_Pos))
		diepctlReg(0).SetBits(otgEPCTL_EPENA | otgEPCTL_CNAK)
		ep0InBuf = nil
		return
	}

	pktcnt := uint32(remaining+63) / 64
	if pktcnt == 0 {
		pktcnt = 1
	}
	otg.DIEPTSIZ0.Set((pktcnt << otgDIEPTSIZ0_PKTCNT_Pos) | uint32(remaining))
	diepctlReg(0).SetBits(otgEPCTL_EPENA | otgEPCTL_CNAK)
	fifoWrite(0, ep0InBuf[ep0InOffset:ep0InOffset+remaining])
	ep0InOffset += remaining
}

// SendZlp sends a zero-length packet on EP0 (used by generic stack as
// status-stage / stall-avoidance).
func SendZlp() {
	otg.DIEPTSIZ0.Set(1 << otgDIEPTSIZ0_PKTCNT_Pos)
	diepctlReg(0).SetBits(otgEPCTL_EPENA | otgEPCTL_CNAK)
}

// SendUSBInPacket is the public entry class drivers use for data IN. It
// wraps sendUSBPacket; SAMD51 etc. use this same two-level API.
func SendUSBInPacket(ep uint32, data []byte) bool {
	sendUSBPacket(ep, data)
	return true
}

// handleUSBSetAddress is invoked by the generic SET_ADDRESS handler.
// DWC2 has the unusual requirement that the device address be programmed
// *before* sending the status-stage ZLP.
func handleUSBSetAddress(setup usb.Setup) bool {
	dcfg := otg.DCFG.Get()
	dcfg &^= otgDCFG_DAD_Msk
	dcfg |= (uint32(setup.WValueL) << otgDCFG_DAD_Pos) & otgDCFG_DAD_Msk
	otg.DCFG.Set(dcfg)
	SendZlp()
	return true
}

// handleEndpointRx returns the bytes most recently captured for OUT EP
// `ep`. The generic dispatcher calls this after learning the EP has data
// available (via XFRC), which drives our dispatchRx.
func handleEndpointRx(ep uint32) []byte {
	return epRxBuffer[ep][:epRxCount[ep]]
}

// AckUsbOutTransfer re-arms a non-control OUT endpoint for the next MPS-
// sized packet. Generic stack calls this after the RX handler consumes
// what handleEndpointRx returned.
func AckUsbOutTransfer(ep uint32) {
	if ep == 0 {
		armEP0Out()
		return
	}
	epRxCount[ep] = 0
	doeptsizReg(ep).Set((1 << otgDIEPTSIZ_PKTCNT_Pos) | 64)
	doepctlReg(ep).SetBits(otgEPCTL_EPENA | otgEPCTL_CNAK)
}

// ReceiveUSBControlPacket blocks until the host delivers a CDC control-
// OUT data phase (e.g., the 7-byte SET_LINE_CODING payload). Runs in IRQ
// context (cdcSetup is called from our ISR) so we hand-drain RXFLVL
// rather than returning-and-reentering.
func ReceiveUSBControlPacket() ([cdcLineInfoSize]byte, error) {
	var out [cdcLineInfoSize]byte

	// Arm EP0 OUT for a 7-byte data-OUT packet (plus SETUP room).
	otg.DOEPTSIZ0.Set(
		(3 << otgDOEPTSIZ0_STUPCNT_Pos) |
			otgDOEPTSIZ0_PKTCNT |
			cdcLineInfoSize,
	)
	doepctlReg(0).SetBits(otgEPCTL_EPENA | otgEPCTL_CNAK)

	// Spin waiting for an OUT packet on EP0. We service RXFLVL inline.
	timeout := 300_000
	for epRxCount[0] < cdcLineInfoSize {
		if otg.GINTSTS.Get()&otgGINTSTS_RXFLVL != 0 {
			drainRxFifo()
		}
		timeout--
		if timeout == 0 {
			return out, ErrUSBReadTimeout
		}
	}

	copy(out[:], epRxBuffer[0][:cdcLineInfoSize])
	epRxCount[0] = 0
	return out, nil
}

// fifoWrite pushes `data` into EP ep's TX FIFO as 32-bit words. Padding
// bytes in the final partial word are don't-care; the core uses the
// corresponding DIEPTSIZ.XFRSIZ field to know the real byte count.
func fifoWrite(ep uint32, data []byte) {
	win := fifoWin[ep]
	n := len(data)
	i := 0
	for ; i+4 <= n; i += 4 {
		win.Set(uint32(data[i]) |
			uint32(data[i+1])<<8 |
			uint32(data[i+2])<<16 |
			uint32(data[i+3])<<24)
	}
	if i < n {
		var w uint32
		for j := 0; i+j < n; j++ {
			w |= uint32(data[i+j]) << (8 * j)
		}
		win.Set(w)
	}
}

// fifoReadInto reads `len(buf)` bytes from the shared RX FIFO into `buf`.
// The RX FIFO is accessed via fifoWin[0] regardless of which endpoint the
// packet targeted; the addressing is consumed in the GRXSTSP word that
// the caller has already popped.
func fifoReadInto(buf []byte) {
	win := fifoWin[0]
	n := len(buf)
	i := 0
	for ; i+4 <= n; i += 4 {
		w := win.Get()
		buf[i] = byte(w)
		buf[i+1] = byte(w >> 8)
		buf[i+2] = byte(w >> 16)
		buf[i+3] = byte(w >> 24)
	}
	if i < n {
		w := win.Get()
		for j := 0; i+j < n; j++ {
			buf[i+j] = byte(w >> (8 * j))
		}
	}
}

// Register accessors. The SVD generates one Register32 per DIEPCTLn, not
// an array — these helpers pick the right member by endpoint index.

func diepctlReg(ep uint32) *volatile.Register32 {
	switch ep {
	case 0:
		return &otg.DIEPCTL0
	case 1:
		return &otg.DIEPCTL1
	case 2:
		return &otg.DIEPCTL2
	case 3:
		return &otg.DIEPCTL3
	case 4:
		return &otg.DIEPCTL4
	case 5:
		return &otg.DIEPCTL5
	case 6:
		return &otg.DIEPCTL6
	case 7:
		return &otg.DIEPCTL7
	case 8:
		return &otg.DIEPCTL8
	}
	return &otg.DIEPCTL0
}

func diepintReg(ep uint32) *volatile.Register32 {
	switch ep {
	case 0:
		return &otg.DIEPINT0
	case 1:
		return &otg.DIEPINT1
	case 2:
		return &otg.DIEPINT2
	case 3:
		return &otg.DIEPINT3
	case 4:
		return &otg.DIEPINT4
	case 5:
		return &otg.DIEPINT5
	case 6:
		return &otg.DIEPINT6
	case 7:
		return &otg.DIEPINT7
	case 8:
		return &otg.DIEPINT8
	}
	return &otg.DIEPINT0
}

func dieptsizReg(ep uint32) *volatile.Register32 {
	switch ep {
	case 0:
		return &otg.DIEPTSIZ0
	case 1:
		return &otg.DIEPTSIZ1
	case 2:
		return &otg.DIEPTSIZ2
	case 3:
		return &otg.DIEPTSIZ3
	case 4:
		return &otg.DIEPTSIZ4
	case 5:
		return &otg.DIEPTSIZ5
	case 6:
		return &otg.DIEPTSIZ6
	case 7:
		return &otg.DIEPTSIZ7
	case 8:
		return &otg.DIEPTSIZ8
	}
	return &otg.DIEPTSIZ0
}

func doepctlReg(ep uint32) *volatile.Register32 {
	switch ep {
	case 0:
		return &otg.DOEPCTL0
	case 1:
		return &otg.DOEPCTL1
	case 2:
		return &otg.DOEPCTL2
	case 3:
		return &otg.DOEPCTL3
	case 4:
		return &otg.DOEPCTL4
	case 5:
		return &otg.DOEPCTL5
	case 6:
		return &otg.DOEPCTL6
	case 7:
		return &otg.DOEPCTL7
	case 8:
		return &otg.DOEPCTL8
	}
	return &otg.DOEPCTL0
}

func doepintReg(ep uint32) *volatile.Register32 {
	switch ep {
	case 0:
		return &otg.DOEPINT0
	case 1:
		return &otg.DOEPINT1
	case 2:
		return &otg.DOEPINT2
	case 3:
		return &otg.DOEPINT3
	case 4:
		return &otg.DOEPINT4
	case 5:
		return &otg.DOEPINT5
	case 6:
		return &otg.DOEPINT6
	case 7:
		return &otg.DOEPINT7
	case 8:
		return &otg.DOEPINT8
	}
	return &otg.DOEPINT0
}

func doeptsizReg(ep uint32) *volatile.Register32 {
	switch ep {
	case 0:
		return &otg.DOEPTSIZ0
	case 1:
		return &otg.DOEPTSIZ1
	case 2:
		return &otg.DOEPTSIZ2
	case 3:
		return &otg.DOEPTSIZ3
	case 4:
		return &otg.DOEPTSIZ4
	case 5:
		return &otg.DOEPTSIZ5
	case 6:
		return &otg.DOEPTSIZ6
	case 7:
		return &otg.DOEPTSIZ7
	case 8:
		return &otg.DOEPTSIZ8
	}
	return &otg.DOEPTSIZ0
}

// PollUSB drives the USB device stack by running one pass of the event
// dispatcher. Callers are expected to invoke this repeatedly from the
// main loop (or any hot context that can preempt USB transactions within
// a few hundred microseconds) until the IRQ-dispatched path is working.
//
// On STM32N6 the NVIC-delivered OTG1 interrupt was observed to not fire
// reliably for peripheral-driven events despite the core holding its IRQ
// line high — ISER/ISPR/IABR diagnostics showed ISPR=0 with unmasked
// GINTSTS bits present, and a software-pended IRQ dispatched our handler
// without issue. Root cause is not yet identified; until it is, PollUSB
// is the supported service path.
func PollUSB() {
	handleUSBIRQ(interrupt.Interrupt{})
}

// EnterBootloader is a no-op-with-reset on N6: there is no vendor serial
// bootloader for the application-class boot ROM, so we just
// request a system reset. Called when a host sends the magic 1200-baud +
// DTR-deassert sequence via CDC-ACM. Future enhancement to create a Tinygo based FSBL Bootloader
func EnterBootloader() {
	arm.SystemReset()
}
