//go:build stm32n6

package runtime

import (
	"device/arm"
	"device/stm32"
	"machine"
	"unsafe"
)

//go:extern __isr_vector
var __isr_vector [0]uint32

func putchar(c byte) {
	machine.Serial.WriteByte(c)
}

func getchar() byte {
	for machine.Serial.Buffered() == 0 {
		Gosched()
	}
	v, _ := machine.Serial.ReadByte()
	return v
}

func buffered() int {
	return machine.Serial.Buffered()
}

// initCLK is the N6's pre-main chip bring-up. Called from
// runtime_stm32n657.go's init() before InitSerial / initTickTimer.
//
// Two non-obvious items compared to other STM32 families:
//
//  1. VTOR must be re-pointed at our vector table. openocd's reset-halt +
//     manual-PC-resume flow lands the CPU at our Reset_Handler in Secure
//     state with VTOR_S still at whatever the ROM bootloader left it;
//     without this write the first exception after boot vectors into ROM.
//
//  2. SEVONPEND in SCB.SCR must be 1. The scheduler's idle path calls
//     WFE, which on Cortex-M55 only exits on an explicit SEV, a higher-
//     priority exception actually taken, or — with SEVONPEND set — any
//     newly-pending IRQ. Without this bit, the scheduler sleeps through
//     tick events and time.Sleep hangs.
//
// RCC clock-enable quirk: on N6 the plain *ENR registers are write-ignored
// and clocks must be gated via the *ENSR "Set" register aliases. That's
// handled in machine_stm32n6.go (enableClock, enableAltFuncClock, TIM
// definitions), not here.
//
// Clock tree: we bring up PLL1 (HSI×25 = 1600 MHz VCO) and PLL3
// (HSE×25 = 1200 MHz VCO), routed through Intermediate Clocks to feed:
//
//	IC1 = PLL3 / 2 = 600 MHz  -> CPUCLK
//	IC2 = PLL1 / 4 = 400 MHz  -> SYSCLK
//	HCLK  = SYSCLK / HPRE=2 = 200 MHz
//	PCLKx = HCLK / PPREx=1  = 200 MHz
//
// These values are mirrored in machine_stm32n6.go's CPUFrequency() and
// APB1_TIM_FREQ / APB2_TIM_FREQ — change them together. The configuration
// is what CubeMX picks for NUCLEO-N657 with HSE+LSE enabled and 600 MHz CPU
// target (ports SystemClock_Config from the reference FSBL project).
//
// Voltage scaling stays at SCALE1 (VOS=0 on N6), supply mode is
// "external SMPS source" (PWR.CR1.SDEN=0, the NUCLEO board's regulator
// configuration). PLL2 / PLL4 / IC3..IC10 are left at reset; the
// peripherals that need them (XSPI clocks, audio, graphics) bring them
// up themselves.
//
// The XSPI FSBL boot path drops us in with PLL1 already running on a
// different config; we always switch CPU/SYS muxes back to HSI before
// reconfiguring the PLLs so this code is idempotent across both boot
// paths (openocd SRAM and ROM XSPI).
func initCLK() {
	arm.SCB.VTOR.Set(uint32(uintptr(unsafe.Pointer(&__isr_vector))))
	arm.SCB.SCR.SetBits(1 << 4) // SEVONPEND

	// --- Park CPU/SYS on HSI so we can reconfigure PLLs safely -----------
	stm32.RCC.CR.SetBits(stm32.RCC_CR_HSION)
	for stm32.RCC.SR.Get()&stm32.RCC_SR_HSIRDY == 0 {
	}
	cfgr1 := stm32.RCC.CFGR1.Get()
	cfgr1 &^= stm32.RCC_CFGR1_CPUSW_Msk | stm32.RCC_CFGR1_SYSSW_Msk
	stm32.RCC.CFGR1.Set(cfgr1)
	for stm32.RCC.CFGR1.Get()&(stm32.RCC_CFGR1_CPUSWS_Msk|stm32.RCC_CFGR1_SYSSWS_Msk) != 0 {
	}

	// --- PWR: external SMPS source, VOS = SCALE1 (VOS bit = 0) -----------
	stm32.PWR.CR1.ClearBits(stm32.PWR_CR1_SDEN)
	for stm32.PWR.VOSCR.Get()&stm32.PWR_VOSCR_ACTVOSRDY == 0 {
	}
	stm32.PWR.VOSCR.ClearBits(stm32.PWR_VOSCR_VOS)
	for stm32.PWR.VOSCR.Get()&stm32.PWR_VOSCR_VOSRDY == 0 {
	}

	// --- HSE on (NUCLEO-N657 X3 = 48 MHz crystal) ------------------------
	stm32.RCC.CR.SetBits(stm32.RCC_CR_HSEON)
	for stm32.RCC.SR.Get()&stm32.RCC_SR_HSERDY == 0 {
	}

	// --- PLL1: HSI(64) × 25 / (1 × 1) = 1600 MHz -------------------------
	stm32.RCC.CCR.Set(stm32.RCC_CCR_PLL1ONC) // disable then wait
	for stm32.RCC.SR.Get()&stm32.RCC_SR_PLL1RDY != 0 {
	}
	// PLL1SEL = 0 (HSI), DIVM = 1, DIVN = 25, BYP = 0.
	stm32.RCC.PLL1CFGR1.Set(
		(0 << stm32.RCC_PLL1CFGR1_PLL1SEL_Pos) |
			(1 << stm32.RCC_PLL1CFGR1_PLL1DIVM_Pos) |
			(25 << stm32.RCC_PLL1CFGR1_PLL1DIVN_Pos),
	)
	stm32.RCC.PLL1CFGR2.Set(0) // FRACN = 0
	// PDIV1 = 1, PDIV2 = 1, PDIVEN = 1, MODSSDIS = 1, MODSSRST = 1
	// (no spread spectrum, P-divider chain enabled and reset).
	stm32.RCC.PLL1CFGR3.Set(
		(1 << stm32.RCC_PLL1CFGR3_PLL1PDIV1_Pos) |
			(1 << stm32.RCC_PLL1CFGR3_PLL1PDIV2_Pos) |
			stm32.RCC_PLL1CFGR3_PLL1PDIVEN |
			stm32.RCC_PLL1CFGR3_PLL1MODSSDIS |
			stm32.RCC_PLL1CFGR3_PLL1MODSSRST,
	)
	stm32.RCC.CSR.Set(stm32.RCC_CSR_PLL1ONS)
	for stm32.RCC.SR.Get()&stm32.RCC_SR_PLL1RDY == 0 {
	}

	// --- PLL3: HSE(48) × 25 / (1 × 1) = 1200 MHz -------------------------
	stm32.RCC.CCR.Set(stm32.RCC_CCR_PLL3ONC)
	for stm32.RCC.SR.Get()&stm32.RCC_SR_PLL3RDY != 0 {
	}
	stm32.RCC.PLL3CFGR1.Set(
		(2 << stm32.RCC_PLL3CFGR1_PLL3SEL_Pos) | // 2 = HSE
			(1 << stm32.RCC_PLL3CFGR1_PLL3DIVM_Pos) |
			(25 << stm32.RCC_PLL3CFGR1_PLL3DIVN_Pos),
	)
	stm32.RCC.PLL3CFGR2.Set(0)
	stm32.RCC.PLL3CFGR3.Set(
		(1 << stm32.RCC_PLL3CFGR3_PLL3PDIV1_Pos) |
			(1 << stm32.RCC_PLL3CFGR3_PLL3PDIV2_Pos) |
			stm32.RCC_PLL3CFGR3_PLL3PDIVEN |
			stm32.RCC_PLL3CFGR3_PLL3MODSSDIS |
			stm32.RCC_PLL3CFGR3_PLL3MODSSRST,
	)
	stm32.RCC.CSR.Set(stm32.RCC_CSR_PLL3ONS)
	for stm32.RCC.SR.Get()&stm32.RCC_SR_PLL3RDY == 0 {
	}

	// --- IC1 = PLL3 / 2 (CPU 600 MHz), IC2 = PLL1 / 4 (SYS 400 MHz) ------
	// SEL: 0=PLL1, 1=PLL2, 2=PLL3, 3=PLL4. INT = divider − 1.
	stm32.RCC.IC1CFGR.Set(
		(2 << stm32.RCC_IC1CFGR_IC1SEL_Pos) |
			(1 << stm32.RCC_IC1CFGR_IC1INT_Pos),
	)
	stm32.RCC.IC2CFGR.Set(
		(0 << stm32.RCC_IC2CFGR_IC2SEL_Pos) |
			(3 << stm32.RCC_IC2CFGR_IC2INT_Pos),
	)
	stm32.RCC.DIVENSR.Set(stm32.RCC_DIVENR_IC1EN | stm32.RCC_DIVENR_IC2EN)

	// --- Bus prescalers: HPRE = /2 (encoded 1), PPREx = /1 (encoded 0) ---
	stm32.RCC.CFGR2.Set(1 << stm32.RCC_CFGR2_HPRE_Pos)

	// --- Switch CPU mux to IC1, SYS mux to IC2_IC6_IC11 (both encoded 3) -
	cfgr1 = stm32.RCC.CFGR1.Get()
	cfgr1 &^= stm32.RCC_CFGR1_CPUSW_Msk | stm32.RCC_CFGR1_SYSSW_Msk
	cfgr1 |= (3 << stm32.RCC_CFGR1_CPUSW_Pos) | (3 << stm32.RCC_CFGR1_SYSSW_Pos)
	stm32.RCC.CFGR1.Set(cfgr1)
	for {
		v := stm32.RCC.CFGR1.Get()
		cpuOK := v&stm32.RCC_CFGR1_CPUSWS_Msk == 3<<stm32.RCC_CFGR1_CPUSWS_Pos
		sysOK := v&stm32.RCC_CFGR1_SYSSWS_Msk == 3<<stm32.RCC_CFGR1_SYSSWS_Pos
		if cpuOK && sysOK {
			break
		}
	}
}
