//go:build nucleog431kb

package main

// OPAMP is a global peripheral - no board-specific pin configuration needed.
// This file exists for consistency with other examples and documents
// the pin mappings on the Nucleo-G431KB board.
//
// OPAMP Pin Mappings (Nucleo-G431KB):
//
// OPAMP1:
//   - VINP0: PA1
//   - VINP1: PA3
//   - VINP2: PA7
//   - VINM0: PA3
//   - VINM1: PC5
//   - VOUT:  PA3 (also VINM0/VINP1)
//
// OPAMP2:
//   - VINP0: PA7
//   - VINP1: PB14
//   - VINP2: PB0
//   - VINM0: PA5
//   - VINM1: PC5
//   - VOUT:  PA6
//
// OPAMP3:
//   - VINP0: PB0
//   - VINP1: PB13
//   - VINP2: PA1
//   - VINM0: PB2
//   - VINM1: PB10
//   - VOUT:  PB1
//
// Internal ADC Connections:
//   - OPAMP1 -> ADC1_IN13
//   - OPAMP2 -> ADC2_IN16
//   - OPAMP3 -> ADC3_IN13 (if available)
