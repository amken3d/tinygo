//go:build nucleog431kb

package main

// ADC Motor Control Pin Mappings for Nucleo-G431KB
//
// Current Sensing with OPAMPs:
// ===========================
//
// OPAMP1 (for Phase A current):
//   - VINP0: PA1 (connect to current shunt)
//   - VOUT:  PA3 (external) or ADC1_IN13 (internal)
//
// OPAMP2 (for Phase B current):
//   - VINP0: PA7 (connect to current shunt)
//   - VOUT:  PA6 (external) or ADC2_IN16 (internal)
//
// OPAMP3 (for Phase C current, if needed):
//   - VINP0: PB0 (connect to current shunt)
//   - VOUT:  PB1 (external) or internal
//
// ADC Channel Assignments:
// ========================
//
// ADC1:
//   - IN1:  PA0
//   - IN2:  PA1 (OPAMP1 input)
//   - IN3:  PA2
//   - IN4:  PA3 (OPAMP1 output)
//   - IN13: OPAMP1 internal output
//   - IN15: PB0
//
// ADC2:
//   - IN1:  PA0
//   - IN2:  PA1
//   - IN3:  PA6 (OPAMP2 output)
//   - IN4:  PA7 (OPAMP2 input)
//   - IN16: OPAMP2 internal output
//   - IN15: PB15
//
// Timer Trigger Sources:
// ======================
//
// For FOC, use TIM1_TRGO2 with update event as trigger source.
// This triggers ADC sampling at the center of the PWM period
// where the current is most stable.
//
// TIM1 PWM Outputs:
//   - CH1:  PA8  (Phase A high-side)
//   - CH1N: PA7  (Phase A low-side) - conflicts with ADC2_IN4!
//   - CH2:  PA9  (Phase B high-side)
//   - CH2N: PB0  (Phase B low-side)
//   - CH3:  PA10 (Phase C high-side)
//   - CH3N: PB1  (Phase C low-side)
//
// Recommended FOC Configuration:
// ==============================
//
// 1. Use internal OPAMP outputs (not external pins) to avoid conflicts
// 2. Sample at TIM1_TRGO2 (update event = PWM center)
// 3. Use injected simultaneous mode for Ia and Ib
// 4. Calculate Ic = -(Ia + Ib) in software
