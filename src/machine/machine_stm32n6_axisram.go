//go:build stm32n6

package machine

// Extended AXI SRAM bring-up.
//
// The N6 has six AXISRAM blocks beyond the 256 KB AXISRAM2 region the
// linker uses for code/data/stack. AXISRAM3..6 each provide 448 KB at
// addresses 0x34200000, 0x34270000, 0x342E0000, 0x34350000 — together
// 1.79 MB of additional, AXI-master-reachable scratch space ideal for
// camera frame buffers, NPU activations, etc.
//
// Three things must happen before any code (or peripheral) can read or
// write the extended AXISRAMs — missing any one of the three results
// in writes silently dropped and reads returning 0xFF (or 0x00):
//
//  1. Bring the block out of reset via RCC.MEMRSTCR (W1C clear).
//
//  2. Enable the per-block bus clock via RCC.MEMENSR (W1S set).
//
//  3. Clear the SRAMSD ("shutdown") bit in the corresponding RAMCFG
//     instance's CR register. Reset value is SRAMSD=1 — the SRAM is
//     electrically powered down to save quiescent current. Steps 1+2
//     alone are NOT sufficient; this is the step our previous attempt
//     was missing. ST's HAL_DCMIPP_MspInit clears it via
//     HAL_RAMCFG_EnableAXISRAM(RAMCFG_SRAM3_AXI), which writes
//     CLEAR_BIT(CR, RAMCFG_CR_SRAMSD).
//
// InitExtendedAXISRAM is idempotent so re-running it is harmless.

import (
	"device/stm32"
)

// InitExtendedAXISRAM brings AXISRAM3..6 out of reset, enables their
// bus clocks, and clears their RAMCFG shutdown bit. After this call the
// 1.79 MB at 0x34200000..0x343BFFFF is read/write-able by the CPU and
// AXI bus masters (subject to RIF/RISAF policy).
func InitExtendedAXISRAM() {
	// 1. Clear reset for AXISRAM3..6. MEMRSTCR is the W1C clear register
	//    that mirrors MEMRSTR (reset-state register).
	stm32.RCC.MEMRSTCR.Set(
		stm32.RCC_MEMRSTCR_AXISRAM3RSTC |
			stm32.RCC_MEMRSTCR_AXISRAM4RSTC |
			stm32.RCC_MEMRSTCR_AXISRAM5RSTC |
			stm32.RCC_MEMRSTCR_AXISRAM6RSTC,
	)
	_ = stm32.RCC.MEMRSTR.Get()

	// 2. Enable bus clocks via MEMENSR (W1S enable-set register).
	stm32.RCC.MEMENSR.Set(
		stm32.RCC_MEMENSR_AXISRAM3ENS |
			stm32.RCC_MEMENSR_AXISRAM4ENS |
			stm32.RCC_MEMENSR_AXISRAM5ENS |
			stm32.RCC_MEMENSR_AXISRAM6ENS,
	)
	_ = stm32.RCC.MEMENR.Get()

	// 3. Clear the per-block RAMCFG SRAMSD bit. Without this the SRAM is
	//    in shutdown — the bus clock and reset are only half the story.
	//    RAMCFG itself is on AHB2 and needs its own peripheral clock
	//    enabled before its registers are reachable.
	stm32.RCC.AHB2ENSR.Set(stm32.RCC_AHB2ENR_RAMCFGEN)
	_ = stm32.RCC.AHB2ENR.Get()
	stm32.RAMCFG.AXISRAM3CR.ClearBits(stm32.RAMCFG_AXISRAM3CR_SRAMSD)
	stm32.RAMCFG.AXISRAM4CR.ClearBits(stm32.RAMCFG_AXISRAM4CR_SRAMSD)
	stm32.RAMCFG.AXISRAM5CR.ClearBits(stm32.RAMCFG_AXISRAM5CR_SRAMSD)
	stm32.RAMCFG.AXISRAM6CR.ClearBits(stm32.RAMCFG_AXISRAM6CR_SRAMSD)
}
