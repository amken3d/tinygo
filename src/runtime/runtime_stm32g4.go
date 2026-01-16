//go:build stm32g4

package runtime

import (
	"device/stm32"
	"machine"
)

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

/*
clock settings for STM32G4

	+-------------+-----------+
	| HSI16       | 16mhz     |
	| SYSCLK      | 170mhz    |
	| HCLK        | 170mhz    |
	| APB1(PCLK1) | 170mhz    |
	| APB2(PCLK2) | 170mhz    |
	+-------------+-----------+

PLL configuration: HSI16 (16MHz) / PLLM(4) * PLLN(85) / PLLR(2) = 170MHz
VCO = 16MHz / 4 * 85 = 340MHz (valid range is 96-344MHz)
*/
func initCLK() {
	// Enable PWR clock

	stm32.RCC.APB1ENR1.SetBits(stm32.RCC_APB1ENR1_PWREN)
	// Read back to ensure the write is complete (memory barrier)
	_ = stm32.RCC.APB1ENR1.Get()

	// Set Power Regulator to Range 1 (required for 170MHz)
	// VOS = 01 for Range 1
	stm32.PWR.CR1.ReplaceBits(1<<stm32.PWR_CR1_VOS_Pos, stm32.PWR_CR1_VOS_Msk, 0)
	// Wait for voltage scaling to be ready (VOSF = 0 means ready)
	for stm32.PWR.SR2.HasBits(stm32.PWR_SR2_VOSF) {
	}

	// Enable Range 1 Boost mode for 170MHz (R1MODE = 0)
	stm32.PWR.CR5.ClearBits(stm32.PWR_CR5_R1MODE)

	// Enable HSI16
	stm32.RCC.CR.SetBits(stm32.RCC_CR_HSION)
	for !stm32.RCC.CR.HasBits(stm32.RCC_CR_HSIRDY) {
	}

	// Disable PLL before configuration
	stm32.RCC.CR.ClearBits(stm32.RCC_CR_PLLON)
	for stm32.RCC.CR.HasBits(stm32.RCC_CR_PLLRDY) {
	}

	// Configure PLL: HSI16 / 4 * 85 / 2 = 170 MHz
	// PLLSRC = HSI16 (2)
	// PLLM = 3 (divide by 4, encoded as M-1)
	// PLLN = 85 (multiply by 85)
	// PLLR = 0 (divide by 2)
	// PLLREN = 1 (enable R output for SYSCLK)
	const (
		PLLSRC_HSI16 = 2  // HSI16 as PLL source
		PLLM_DIV4    = 3  // /4 (encoded as M-1)
		PLLN_MUL85   = 85 // *85
		PLLR_DIV2    = 0  // /2 (0 = divide by 2)
	)
	stm32.RCC.PLLCFGR.Set(
		(PLLSRC_HSI16 << stm32.RCC_PLLCFGR_PLLSRC_Pos) |
			(PLLM_DIV4 << stm32.RCC_PLLCFGR_PLLM_Pos) |
			(PLLN_MUL85 << stm32.RCC_PLLCFGR_PLLN_Pos) |
			(PLLR_DIV2 << stm32.RCC_PLLCFGR_PLLR_Pos) |
			stm32.RCC_PLLCFGR_PLLREN) // Enable PLLR output

	// Enable PLL
	stm32.RCC.CR.SetBits(stm32.RCC_CR_PLLON)
	for !stm32.RCC.CR.HasBits(stm32.RCC_CR_PLLRDY) {
	}

	// Set flash latency to 4 wait states (required for 170MHz in Range 1 Boost)
	// Must be set BEFORE switching to higher frequency clock
	const FLASH_LATENCY_4 = 4
	stm32.FLASH.ACR.ReplaceBits(FLASH_LATENCY_4, stm32.Flash_ACR_LATENCY_Msk, 0)
	for (stm32.FLASH.ACR.Get() & stm32.Flash_ACR_LATENCY_Msk) != FLASH_LATENCY_4 {
	}

	// Enable prefetch buffer, instruction cache and data cache
	stm32.FLASH.ACR.SetBits(stm32.Flash_ACR_PRFTEN | stm32.Flash_ACR_ICEN | stm32.Flash_ACR_DCEN)

	// Set AHB prescaler to 1 (no division)
	stm32.RCC.CFGR.ReplaceBits(0, stm32.RCC_CFGR_HPRE_Msk, 0)

	// Set APB1 and APB2 prescalers to 1 (no division)
	stm32.RCC.CFGR.ReplaceBits(0, stm32.RCC_CFGR_PPRE1_Msk, 0)
	stm32.RCC.CFGR.ReplaceBits(0, stm32.RCC_CFGR_PPRE2_Msk, 0)

	// Switch system clock to PLL (SW = 11)
	const RCC_CFGR_SW_PLL = 3
	stm32.RCC.CFGR.ReplaceBits(RCC_CFGR_SW_PLL, stm32.RCC_CFGR_SW_Msk, 0)
	// Wait for PLL to be used as system clock (SWS = 11)
	for (stm32.RCC.CFGR.Get() & stm32.RCC_CFGR_SWS_Msk) != (RCC_CFGR_SW_PLL << stm32.RCC_CFGR_SWS_Pos) {
	}
}
