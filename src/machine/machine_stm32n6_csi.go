//go:build stm32n6

package machine

// CSI-2 host driver for the STM32N6.
//
// The N6 CSI-2 host is a Synopsys-style controller with an integrated D-PHY
// supporting up to 2 data lanes. Pixel data flows out of the host into the
// DCMIPP via a fixed internal bus; this driver only configures the host
// (clocks, lane count, virtual channel filter, PHY frequency) and
// starts/stops virtual-channel reception. Pixel-side processing happens in
// machine_stm32n6_dcmipp.go.
//
// Typical bring-up order for a CSI-2 sensor (e.g. IMX335/IMX355 on
// MB1854) is:
//
//   1. Power up + clock the sensor over I2C (sensor-specific; out of scope).
//   2. machine.CSI.Configure(machine.CSIConfig{...})
//   3. machine.DCMIPP.Configure(machine.DCMIPPConfig{InputMode: DCMIPPInputCSI, ...})
//   4. machine.DCMIPP.StartPipe0(buf, mode)
//   5. machine.CSI.Start(0)
//   6. Sensor: stream-on (sensor-specific I2C command).
//
// See RM0486 §35 (CSI-2 host) for full register-level details.

import (
	"device/arm"
	"device/stm32"
	"runtime/interrupt"
	"runtime/volatile"
	"unsafe"
)

// CSIBitsPerPixel is the per-data-type word width as encoded in
// CSI_VCxCFGR{2,3,4}.DTxFT.
type CSIBitsPerPixel uint8

const (
	CSIBitsPerPixel6  CSIBitsPerPixel = 0
	CSIBitsPerPixel7  CSIBitsPerPixel = 1
	CSIBitsPerPixel8  CSIBitsPerPixel = 2
	CSIBitsPerPixel10 CSIBitsPerPixel = 3
	CSIBitsPerPixel12 CSIBitsPerPixel = 4
	CSIBitsPerPixel14 CSIBitsPerPixel = 5
	CSIBitsPerPixel16 CSIBitsPerPixel = 6
)

// CSI-2 standard data-type codes (subset). Pass these as DataType fields.
// See MIPI CSI-2 v1.3 §11 (Data Type definitions).
const (
	CSIDataTypeYUV422_8  uint8 = 0x1E
	CSIDataTypeYUV422_10 uint8 = 0x1F
	CSIDataTypeRGB565    uint8 = 0x22
	CSIDataTypeRGB888    uint8 = 0x24
	CSIDataTypeRAW8      uint8 = 0x2A
	CSIDataTypeRAW10     uint8 = 0x2B
	CSIDataTypeRAW12     uint8 = 0x2C
	CSIDataTypeRAW14     uint8 = 0x2D
)

// CSIConfig describes a minimal single-VC, single-data-type setup. It's
// what most camera bring-ups need; for multi-VC or multi-DT the lower-level
// register accessors (stm32.CSI.*) are still exported.
type CSIConfig struct {
	// NumLanes is 1 or 2. The N6 D-PHY does not support more lanes.
	NumLanes uint8

	// VirtualChannel selects which CSI-2 VC to receive (0..3). Most
	// sensors send on VC0.
	VirtualChannel uint8

	// DataType is the 6-bit CSI-2 DT code (see CSIDataType*).
	DataType uint8

	// BitsPerPixel matches the DataType width (e.g. RAW10 → CSIBitsPerPixel10).
	BitsPerPixel CSIBitsPerPixel

	// PHYConfigClockMHz is the configuration clock fed to the D-PHY (CCFR
	// in CSI_PFCR). On N6 this is typically the HSI at ~24 MHz; the CSI
	// PFCR field stores the value as the integer MHz.
	// Default 24 if zero.
	PHYConfigClockMHz uint8

	// PHYHighSpeedRange is the HSFR field of CSI_PFCR (7-bit). It picks
	// the D-PHY high-speed receive frequency band; the mapping of HSFR
	// codes to Mbps ranges is in RM0486 Table 547. Pass 0 to let
	// Configure pick a value from PHYDataRateMbps; pass non-zero to
	// override.
	PHYHighSpeedRange uint8

	// PHYDataRateMbps is the per-lane HS data rate. Used only if
	// PHYHighSpeedRange is 0. Must be set to a non-zero value in that
	// case; the mapping is approximate (see csiHSFRForMbps).
	PHYDataRateMbps uint32

	// SwapDataLanes connects physical lane 0 to logical 1 and physical
	// lane 1 to logical 0. The board may route the sensor's lane 0 to
	// the chip's lane 1 — flip this if no frames arrive.
	SwapDataLanes bool
}

// ErrCSIInvalidConfig is returned by Configure when the supplied CSIConfig
// can't be programmed (e.g. NumLanes out of range).
var ErrCSIInvalidConfig = errCSIInvalidConfig{}

type errCSIInvalidConfig struct{}

func (errCSIInvalidConfig) Error() string { return "machine: invalid CSIConfig" }

// CSI is the singleton CSI-2 host instance.
var CSI csiDevice

type csiDevice struct {
	bus *stm32.CSI_Type

	intr     interrupt.Interrupt
	intrInit bool
}

// Configure enables clocks, releases reset, runs the Synopsys D-PHY
// init sequence (which is non-trivial — see ST HAL_DCMIPP_CSI_SetConfig
// for the canonical reference), programs the lane mapping, and sets up
// a single virtual-channel + data-type filter. Does not start the
// receive engine — call Start() once the sensor is streaming.
func (c *csiDevice) Configure(cfg CSIConfig) error {
	if cfg.NumLanes < 1 || cfg.NumLanes > 2 {
		return ErrCSIInvalidConfig
	}
	if cfg.VirtualChannel > 3 {
		return ErrCSIInvalidConfig
	}
	if cfg.PHYHighSpeedRange == 0 && cfg.PHYDataRateMbps == 0 {
		return ErrCSIInvalidConfig
	}

	c.bus = stm32.CSI

	// ---- Kernel clock: IC18 = PLL1 / 80 = 20 MHz ----
	// CSI's kernel clock (which feeds the D-PHY's config-clock counter) is
	// hard-wired to IC18 — there is no CCIPR select bit. By reset, IC18 is
	// gated off (DIVENR.IC18EN=0) and its source is PLL4 (which we don't
	// bring up). Without enabling it the PHY's internal state machine
	// never advances past its config-clock wait, so SOF/EOF never fire
	// even though the registers all program cleanly off the APB5 bus
	// clock. Mirror ST's MX_DCMIPP_ClockConfig (PLL1/40=20 MHz on their
	// 800 MHz PLL1; PLL1/80=20 MHz on our 1600 MHz PLL1).
	stm32.RCC.IC18CFGR.Set((0 << 28) | (79 << 16)) // SEL=PLL1, INT=79 → /80
	stm32.RCC.DIVENSR.Set(stm32.RCC_DIVENSR_IC18ENS)

	// ---- Bus clock + peripheral reset pulse ----
	stm32.RCC.APB5ENSR.Set(stm32.RCC_APB5ENR_CSIEN)
	_ = stm32.RCC.APB5ENR.Get()
	stm32.RCC.APB5RSTSR.Set(stm32.RCC_APB5RSTR_CSIRST)
	for i := 0; i < 256; i++ {
		arm.Asm("nop")
	}
	stm32.RCC.APB5RSTCR.Set(stm32.RCC_APB5RSTR_CSIRST)
	_ = stm32.RCC.APB5RSTR.Get()

	// ---- Disable host + PHY before reprogramming ----
	c.bus.CR.ClearBits(stm32.CSI_CR_CSIEN)
	c.bus.PCR.ClearBits(stm32.CSI_PCR_CLEN | stm32.CSI_PCR_DL0EN | stm32.CSI_PCR_DL1EN)
	c.bus.PCR.SetBits(stm32.CSI_PCR_PWRDOWN)

	// ---- D-PHY init (RM 40.6.1 + ST HAL_DCMIPP_CSI_SetConfig) ----
	//
	// 1. Stop the D-PHY (PRCR.PEN=0)
	c.bus.PRCR.ClearBits(stm32.CSI_PRCR_PEN)

	// 2. Get D-PHY enabled with all lanes off
	c.bus.PCR.Set(0)

	// 3. Pulse testclk to wake the D-PHY's test interface
	c.bus.PTCR0.SetBits(stm32.CSI_PTCR0_TCKEN)
	for i := 0; i < 100_000; i++ { // ~1 ms equivalent
		arm.Asm("nop")
	}
	c.bus.PTCR0.Set(0)

	// 4. Look up Synopsys-D-PHY hsfreqrange + DLL osc-freq-target for
	//    the requested bit rate.
	snps := csiPickSnpsFreq(cfg.PHYDataRateMbps)
	if cfg.PHYHighSpeedRange != 0 {
		snps.hsfreqrange = cfg.PHYHighSpeedRange
	}

	// 5. Program PFCR with CCFR + HSFR (no DLD yet — we're still in
	//    init mode). CCFR=0x28 matches what ST HAL hard-codes; per RM
	//    the field is `RoundUp((F-17)*4)` where F is the configuration
	//    clock in MHz. ST treats it as 27 MHz → 0x28.
	const csiCCFR = 0x28
	c.bus.PFCR.Set(
		uint32(csiCCFR<<stm32.CSI_PFCR_CCFR_Pos) |
			(uint32(snps.hsfreqrange) << stm32.CSI_PFCR_HSFR_Pos),
	)

	// 6. Write Synopsys D-PHY internal registers via the test
	//    interface. Sequence per ST HAL DCMIPP_CSI_WritePHYReg().
	//    Magic register addresses come from the DesignWare D-PHY
	//    spec; we match ST's writes (which is the only working
	//    reference for this PHY on N6).
	csiWritePHYReg(c.bus, 0x00, 0x08, 0x38)                           // deskew_polarity_rw = 1
	csiWritePHYReg(c.bus, 0x00, 0xE4, 0x11)                           // counter_for_des_en (cfgclkfreqrange band)
	csiWritePHYReg(c.bus, 0x00, 0xE3, uint8(snps.oscFreqTarget>>8))   // DLL osc freq target high byte
	csiWritePHYReg(c.bus, 0x00, 0xE3, uint8(snps.oscFreqTarget&0xFF)) // DLL osc freq target low byte (ST HAL has same reg twice)

	// 7. Re-write PFCR with DLD bit set (basedir = RX).
	c.bus.PFCR.Set(
		uint32(csiCCFR<<stm32.CSI_PFCR_CCFR_Pos) |
			(uint32(snps.hsfreqrange) << stm32.CSI_PFCR_HSFR_Pos) |
			stm32.CSI_PFCR_DLD,
	)

	// 8. Enable D-PHY clock lane + data lanes (with PWRDOWN still
	//    set — released next step via PRCR.PEN).
	pcr := uint32(stm32.CSI_PCR_DL0EN | stm32.CSI_PCR_CLEN | stm32.CSI_PCR_PWRDOWN)
	if cfg.NumLanes == 2 {
		pcr |= stm32.CSI_PCR_DL1EN
	}
	c.bus.PCR.Set(pcr)

	// 9. Take PHY out of reset (PRCR.PEN=1)
	c.bus.PRCR.SetBits(stm32.CSI_PRCR_PEN)

	// 10. Clear PMCR (force-RX-mode bits) — leave it in normal RX.
	c.bus.PMCR.Set(0)

	// ---- Lane merger config (logical-to-physical lane mapping) ----
	dl0, dl1 := uint32(1), uint32(2)
	if cfg.SwapDataLanes {
		dl0, dl1 = 2, 1
	}
	c.bus.LMCFGR.Set(
		uint32(cfg.NumLanes)<<stm32.CSI_LMCFGR_LANENB_Pos |
			dl0<<stm32.CSI_LMCFGR_DL0MAP_Pos |
			dl1<<stm32.CSI_LMCFGR_DL1MAP_Pos,
	)

	// ---- Per-VC filter (DT/BPP) ----
	dt := uint32(cfg.DataType) & 0x3F
	bpp := uint32(cfg.BitsPerPixel) & 0x1F
	cfgr1Reg := c.vcConfigReg1(cfg.VirtualChannel)
	cfgr1Reg.Set(
		stm32.CSI_VC0CFGR1_DT0EN |
			(dt << stm32.CSI_VC0CFGR1_DT0_Pos) |
			(bpp << stm32.CSI_VC0CFGR1_DT0FT_Pos),
	)

	// ---- Enable host. VCxSTART is pulsed in Start(). ----
	c.bus.CR.SetBits(stm32.CSI_CR_CSIEN)

	if !c.intrInit {
		c.intr = interrupt.New(stm32.IRQ_CSI_DBG, csiHandleInterrupt)
		c.intrInit = true
	}
	return nil
}

// csiHandleInterrupt is the free-function dispatcher for the CSI IRQ
// (TinyGo's interrupt.New rejects bound-method closures). Just forwards
// to the singleton's method.
func csiHandleInterrupt(intr interrupt.Interrupt) {
	CSI.handleInterrupt(intr)
}

// Start pulses the VCxSTART bit for the given virtual channel. The sensor
// must be streaming before this is called (or queued shortly after, since
// the host ignores idle lanes).
func (c *csiDevice) Start(virtualChannel uint8) {
	c.bus.CR.SetBits(csiStartBit(virtualChannel))
}

// Stop pulses the VCxSTOP bit. Pending lines may still arrive — drain via
// the DCMIPP frame callback.
func (c *csiDevice) Stop(virtualChannel uint8) {
	c.bus.CR.SetBits(csiStopBit(virtualChannel))
}

// Disable powers down the PHY and disables the host. Configure() must be
// called again to bring the host back up.
func (c *csiDevice) Disable() {
	c.bus.CR.ClearBits(stm32.CSI_CR_CSIEN)
	c.bus.PCR.ClearBits(stm32.CSI_PCR_CLEN | stm32.CSI_PCR_DL0EN | stm32.CSI_PCR_DL1EN)
	c.bus.PCR.SetBits(stm32.CSI_PCR_PWRDOWN)
}

// vcConfigReg1 returns CSI_VCxCFGR1 for the given virtual channel. The
// four VC blocks are at +0x10, +0x20, +0x30, +0x40, each starting with
// CFGR1.
func (c *csiDevice) vcConfigReg1(vc uint8) *volatile.Register32 {
	base := uintptr(unsafe.Pointer(&c.bus.VC0CFGR1)) + uintptr(vc)*0x10
	return (*volatile.Register32)(unsafe.Pointer(base))
}

func csiStartBit(vc uint8) uint32 {
	switch vc {
	case 0:
		return stm32.CSI_CR_VC0START
	case 1:
		return stm32.CSI_CR_VC1START
	case 2:
		return stm32.CSI_CR_VC2START
	case 3:
		return stm32.CSI_CR_VC3START
	}
	return 0
}

func csiStopBit(vc uint8) uint32 {
	switch vc {
	case 0:
		return stm32.CSI_CR_VC0STOP
	case 1:
		return stm32.CSI_CR_VC1STOP
	case 2:
		return stm32.CSI_CR_VC2STOP
	case 3:
		return stm32.CSI_CR_VC3STOP
	}
	return 0
}

// csiSnpsFreq is one row of the Synopsys D-PHY frequency table. Per ST
// HAL_DCMIPP_CSI_SetConfig, each supported per-lane bit rate has both a
// hsfreqrange code and a DLL oscillator-target value.
type csiSnpsFreq struct {
	bitrateMbps   uint32
	hsfreqrange   uint8
	oscFreqTarget uint16
}

// csiSnpsFreqs is the table from ST HAL stm32n6xx_hal_dcmipp.c
// (SNPS_Freqs[63]). Sorted ascending by bitrate. csiPickSnpsFreq picks
// the first row whose bitrate >= request. Below 1550 Mbps the
// oscFreqTarget is fixed at 460; above that it scales with the band.
var csiSnpsFreqs = [...]csiSnpsFreq{
	{80, 0x00, 460}, {90, 0x10, 460}, {100, 0x20, 460}, {110, 0x30, 460},
	{120, 0x01, 460}, {130, 0x11, 460}, {140, 0x21, 460}, {150, 0x31, 460},
	{160, 0x02, 460}, {170, 0x12, 460}, {180, 0x22, 460}, {190, 0x32, 460},
	{205, 0x03, 460}, {220, 0x13, 460}, {235, 0x23, 460}, {250, 0x33, 460},
	{275, 0x04, 460}, {300, 0x14, 460}, {325, 0x25, 460}, {350, 0x35, 460},
	{400, 0x05, 460}, {450, 0x16, 460}, {500, 0x26, 460}, {550, 0x37, 460},
	{600, 0x07, 460}, {650, 0x18, 460}, {700, 0x28, 460}, {750, 0x39, 460},
	{800, 0x09, 460}, {850, 0x19, 460}, {900, 0x29, 460}, {950, 0x3A, 460},
	{1000, 0x0A, 460}, {1050, 0x1A, 460}, {1100, 0x2A, 460}, {1150, 0x3B, 460},
	{1200, 0x0B, 460}, {1250, 0x1B, 460}, {1300, 0x2B, 460}, {1350, 0x3C, 460},
	{1400, 0x0C, 460}, {1450, 0x1C, 460}, {1500, 0x2C, 460}, {1550, 0x3D, 285},
	{1600, 0x0D, 295}, {1650, 0x1D, 304}, {1700, 0x2E, 313}, {1750, 0x3E, 322},
	{1800, 0x0E, 331}, {1850, 0x1E, 341}, {1900, 0x2F, 350}, {1950, 0x3F, 359},
	{2000, 0x0F, 368}, {2050, 0x40, 377}, {2100, 0x41, 387}, {2150, 0x42, 396},
	{2200, 0x43, 405}, {2250, 0x44, 414}, {2300, 0x45, 423}, {2350, 0x46, 432},
	{2400, 0x47, 442}, {2450, 0x48, 451}, {2500, 0x49, 460},
}

// csiPickSnpsFreq returns the lowest band whose bitrate is >= mbps.
// Falls back to the highest band for out-of-range requests.
func csiPickSnpsFreq(mbps uint32) csiSnpsFreq {
	for _, e := range csiSnpsFreqs {
		if mbps <= e.bitrateMbps {
			return e
		}
	}
	return csiSnpsFreqs[len(csiSnpsFreqs)-1]
}

// csiWritePHYReg writes one Synopsys D-PHY internal register via the
// CSI test interface (PTCR0/PTCR1). The four writes for testcode-MSB,
// testcode-LSB, then data follow the DesignWare D-PHY documented
// sequence; we match ST HAL DCMIPP_CSI_WritePHYReg() exactly because
// the magic numbers (test register addresses) come from the PHY
// vendor's docs and aren't in the public RM.
func csiWritePHYReg(bus *stm32.CSI_Type, msb, lsb, val uint8) {
	// Write 4-bit testcode MSBs.
	bus.PTCR1.SetBits(stm32.CSI_PTCR1_TWM)
	bus.PTCR0.SetBits(stm32.CSI_PTCR0_TCKEN)
	bus.PTCR1.SetBits(stm32.CSI_PTCR1_TWM)
	bus.PTCR0.Set(0)
	bus.PTCR1.Set(0)

	bus.PTCR1.SetBits(uint32(msb))
	bus.PTCR0.SetBits(stm32.CSI_PTCR0_TCKEN)

	// Write 8-bit testcode LSBs.
	bus.PTCR0.Set(0)
	bus.PTCR1.SetBits(stm32.CSI_PTCR1_TWM)
	bus.PTCR0.SetBits(stm32.CSI_PTCR0_TCKEN)
	bus.PTCR1.SetBits(stm32.CSI_PTCR1_TWM | uint32(lsb))
	bus.PTCR0.Set(0)
	bus.PTCR1.Set(0)

	// Write data byte.
	bus.PTCR1.SetBits(uint32(val))
	bus.PTCR0.SetBits(stm32.CSI_PTCR0_TCKEN)
	bus.PTCR0.Set(0)
}

// handleInterrupt is wired to IRQ_CSI_DBG. CSI host errors (lane errors,
// ECC errors, sync timeouts) are reported in CSI_SR1; we just clear all
// latched flags so we don't tight-loop on a wedged IRQ.
func (c *csiDevice) handleInterrupt(interrupt.Interrupt) {
	c.bus.FCR1.Set(0xFFFFFFFF)
	c.bus.FCR0.Set(0xFFFFFFFF)
}
