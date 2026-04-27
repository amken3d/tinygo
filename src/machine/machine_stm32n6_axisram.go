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
// Two things must happen before any code (or peripheral) touches the
// extended AXISRAMs:
//
//  1. Enable the per-block clock via RCC.MEMENSR. By reset they're gated
//     off to save quiescent current, and writes to a gated AXISRAM bus-
//     fault on some bus paths (the reads silently return 0xFF).
//
//  2. Open up the corresponding RISAF instance so masters can read/write.
//     Each AXISRAM has its own RISAF: RISAF4 = AXISRAM3, RISAF5 = AXISRAM4,
//     RISAF6 = AXISRAM5, RISAF7 = AXISRAM6. Same register layout as RISAF1
//     (which is what the SVD exposes); they sit at AHB3PERIPH+0x9000,
//     0xA000, 0xB000, 0xC000.
//
// initExtendedAXISRAM is called once from runtime_stm32n657.go's init().
// It's idempotent so re-running it is harmless.

import (
	"device/stm32"
)

// InitExtendedAXISRAM brings AXISRAM3..6 out of reset and enables their
// bus clocks. After power-on the upper SRAM blocks are held in reset
// (MEMRSTR.AXISRAMxRST=1) AND their clocks are gated (MEMENR.AXISRAMxEN=0).
// Both must be set/cleared for the SRAMs to respond. With clocks enabled
// but reset held, accesses silently return 0 (read) or drop (write) —
// exactly the symptom we observed before adding the reset-clear step.
//
// Idempotent — safe to re-call.
func InitExtendedAXISRAM() {
	// 1. Bring AXISRAM3..6 out of reset. RCC.MEMRSTCR is the W1C clear
	//    register that mirrors MEMRSTR (reset-state register); writing 1
	//    deasserts reset for the corresponding block.
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
}
