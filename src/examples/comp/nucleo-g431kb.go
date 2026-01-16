//go:build nucleog431kb

package main

// Comparator is a global peripheral - no board-specific pin configuration needed.
// This file exists for consistency with other examples and documents
// the pin mappings on the Nucleo-G431KB board.
//
// Comparator Pin Mappings (Nucleo-G431KB):
//
// COMP1:
//   - INP1: PA1 (non-inverting input 1)
//   - INP2: PB1 (non-inverting input 2)
//   - INM1: PA4 (inverting input 1)
//   - INM2: PA0 (inverting input 2)
//
// COMP2:
//   - INP1: PA7 (non-inverting input 1)
//   - INP2: PA3 (non-inverting input 2)
//   - INM1: PA5 (inverting input 1)
//   - INM2: PA2 (inverting input 2)
//
// COMP3:
//   - INP1: PB0 (non-inverting input 1)
//   - INP2: PB2 (non-inverting input 2)
//   - INM1: PB1 (inverting input 1)
//   - INM2: PA0 (inverting input 2)
//
// COMP4:
//   - INP1: PB0 (non-inverting input 1)
//   - INP2: PA4 (non-inverting input 2)
//   - INM1: PB2 (inverting input 1)
//   - INM2: PA2 (inverting input 2)
//
// Internal Reference Options (INMSEL):
//   - 1/4 VREFINT: ~0.30V (at VREFINT=1.21V)
//   - 1/2 VREFINT: ~0.60V
//   - 3/4 VREFINT: ~0.91V
//   - VREFINT:     ~1.21V
//
// Output Routing:
//   - Comparator output can be read via the VALUE bit
//   - Output can also route to timer BKIN inputs for fault protection
//   - Output can route to timer OCREF_CLR for PWM shutdown
