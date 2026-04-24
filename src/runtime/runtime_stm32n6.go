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
// Clock tree: we force the CPU and bus clocks back onto HSI with the N6
// reset-default dividers (HPRE=/2, PPREx=/1), regardless of how we got
// here. The openocd SRAM path already hands off in this shape, but the
// XSPI FSBL path leaves PLL1 running with CPU ≈ 600 MHz and PCLK ≈ 150
// MHz — ~10× what machine.CPUFrequency() / machine.APB1_TIM_FREQ assume,
// which makes SysTick-based Sleep run 10× fast and puts UART baud off by
// the same factor. Resetting to HSI gets us to a single known state so
// the same binary runs correctly on both boot paths. When we eventually
// bring up PLL1 for higher performance, this block goes away and
// CPUFrequency() / APB*_TIM_FREQ have to update in lockstep.
func initCLK() {
	arm.SCB.VTOR.Set(uint32(uintptr(unsafe.Pointer(&__isr_vector))))
	arm.SCB.SCR.SetBits(1 << 4) // SEVONPEND

	// Make sure HSI is running; ROM leaves it on, but be defensive.
	stm32.RCC.CR.SetBits(stm32.RCC_CR_HSION)
	for stm32.RCC.SR.Get()&stm32.RCC_SR_HSIRDY == 0 {
	}

	// Switch CPU and SYSCLK muxes back to HSI (CPUSW=0, SYSSW=0). IC1/IC2
	// stay wherever ROM put them — once we're off them the clock tree
	// can't see them anymore, so we don't bother powering them down.
	cfgr1 := stm32.RCC.CFGR1.Get()
	cfgr1 &^= stm32.RCC_CFGR1_CPUSW_Msk | stm32.RCC_CFGR1_SYSSW_Msk
	stm32.RCC.CFGR1.Set(cfgr1)
	for stm32.RCC.CFGR1.Get()&(stm32.RCC_CFGR1_CPUSWS_Msk|stm32.RCC_CFGR1_SYSSWS_Msk) != 0 {
	}

	// Restore AHB/APB dividers to their reset defaults: HPRE=/2 (encoded
	// as 1), PPRE1..5 = /1 (encoded 0), TIMPRE=0. After this HCLK = 32
	// MHz, PCLKx = 32 MHz, matching APB1_TIM_FREQ / APB2_TIM_FREQ in
	// machine_stm32n6.go.
	cfgr2 := stm32.RCC.CFGR2.Get()
	cfgr2 &^= stm32.RCC_CFGR2_HPRE_Msk |
		stm32.RCC_CFGR2_PPRE1_Msk |
		stm32.RCC_CFGR2_PPRE2_Msk |
		stm32.RCC_CFGR2_PPRE4_Msk |
		stm32.RCC_CFGR2_PPRE5_Msk |
		stm32.RCC_CFGR2_TIMPRE_Msk
	cfgr2 |= 1 << stm32.RCC_CFGR2_HPRE_Pos
	stm32.RCC.CFGR2.Set(cfgr2)
}
