//go:build tinyfoc_gan

// TinyFOC-GAN - Compact 3-Phase GaN Motor Controller
//
// This board is a compact 3-phase GaN (Gallium Nitride) motor controller
// designed for Field-Oriented Control (FOC), based on STM32G431CBT6 (LQFP48).
//
// Key features:
// - STM32G431CB microcontroller (170 MHz, 128KB Flash, 32KB RAM)
// - 3-phase GaN half-bridge gate drivers
// - 3-shunt current sensing (external ADC, single-ended)
// - BEMF sensing for sensorless control
// - FDCAN bus interface
// - UART telemetry output

package machine

import (
	"device/stm32"
	"runtime/interrupt"
)

// =============================================================================
// LED and Button
// =============================================================================

const (
	LED         = LED_BUILTIN
	LED_BUILTIN = LED_STATUS
	LED_STATUS  = PB12 // Status LED

	// No dedicated button on this board; PA0 used as placeholder
	BUTTON   = USER_BTN
	USER_BTN = PA0
)

// =============================================================================
// Motor Control - PWM Phase Outputs (TIM1 CH1-3 with complementary outputs)
// =============================================================================

// 3-Phase PWM outputs for brushless motor control
// High-side and low-side GaN FETs for each phase
// Connected to TIM1 channels with complementary outputs
const (
	// Phase A (U)
	PHASE_AH = PA8  // TIM1_CH1 - A phase high-side
	PHASE_AL = PC13 // TIM1_CH1N - A phase low-side

	// Phase B (V)
	PHASE_BH = PA9  // TIM1_CH2 - B phase high-side
	PHASE_BL = PA12 // TIM1_CH2N - B phase low-side

	// Phase C (W)
	PHASE_CH = PA10 // TIM1_CH3 - C phase high-side
	PHASE_CL = PB15 // TIM1_CH3N - C phase low-side
)

// U/V/W aliases (industry standard naming)
const (
	PHASE_UH = PHASE_AH
	PHASE_UL = PHASE_AL
	PHASE_VH = PHASE_BH
	PHASE_VL = PHASE_BL
	PHASE_WH = PHASE_CH
	PHASE_WL = PHASE_CL
)

// SimpleFOC INH/INL aliases
const (
	INH_A = PHASE_AH
	INL_A = PHASE_AL
	INH_B = PHASE_BH
	INL_B = PHASE_BL
	INH_C = PHASE_CH
	INL_C = PHASE_CL
)

// =============================================================================
// Motor Control - Current Sensing (3-shunt, single-ended ADC)
// =============================================================================

const (
	CURRENT_A = PA1 // ADC1_IN2 - Phase A current (A_OUTADC)
	CURRENT_B = PA2 // ADC1_IN3 - Phase B current (B_OUTADC)
	CURRENT_C = PA3 // ADC1_IN4 - Phase C current (C_OUTADC)
)

// =============================================================================
// Motor Control - VBUS Sensing
// =============================================================================

const (
	VBUS = PA0 // ADC1_IN1 - DC bus voltage (scaled)
)

// =============================================================================
// Motor Control - BEMF Sensing (for sensorless control)
// =============================================================================

const (
	BEMF_A    = PB1  // ADC1_IN12 - Phase A BEMF
	BEMF_B    = PB0  // ADC1_IN15 - Phase B BEMF
	BEMF_C    = PB11 // ADC1_IN14 - Phase C BEMF
	BEMF_GPIO = PB5  // BEMF voltage divider enable (GPIO output)
)

// =============================================================================
// Gate Driver Alerts (active-low GPIO inputs)
// =============================================================================

const (
	ALERT_A = PA4 // Gate driver A fault
	ALERT_B = PA5 // Gate driver B fault
	ALERT_C = PA6 // Gate driver C fault
)

// =============================================================================
// Communication Interfaces
// =============================================================================

const (
	// UART pins (telemetry / signal input)
	UART_TX_PIN = PB3 // USART2_TX (TLM - telemetry output)
	UART_RX_PIN = PB4 // USART2_RX (SI - signal input)

	// CAN bus pins
	CAN_RX_PIN = PA11 // FDCAN1_RX
	CAN_TX_PIN = PB9  // FDCAN1_TX

	// I2C pins (not exposed on this board)
	I2C0_SCL_PIN = NoPin
	I2C0_SDA_PIN = NoPin

	// SPI pins (not exposed on this board)
	SPI0_SCK_PIN = NoPin
	SPI0_SDI_PIN = NoPin
	SPI0_SDO_PIN = NoPin
)

// =============================================================================
// Hardware Peripherals
// =============================================================================

var (
	// UART via USART2 (telemetry / signal)
	UART1  = &_UART1
	_UART1 = UART{
		Buffer:            NewRingBuffer(),
		Bus:               stm32.USART2,
		TxAltFuncSelector: AF7_USART1_USART2_USART3,
		RxAltFuncSelector: AF7_USART1_USART2_USART3,
	}
	DefaultUART = UART1
)

func init() {
	UART1.Interrupt = interrupt.New(stm32.IRQ_USART2, _UART1.handleInterrupt)
}

// =============================================================================
// Motor Control Helper Constants
// =============================================================================

// Timer assignments for motor control
const (
	// TIM1 is used for 3-phase PWM generation (center-aligned mode)
	MOTOR_PWM_TIMER = 1

	// TIM6/TIM7 can be used for ADC triggering
	ADC_TRIGGER_TIMER = 6
)

// Recommended PWM frequencies for GaN motor control
// GaN FETs support higher switching frequencies than silicon MOSFETs
const (
	PWM_FREQUENCY_20KHZ  = 20000
	PWM_FREQUENCY_30KHZ  = 30000
	PWM_FREQUENCY_40KHZ  = 40000
	PWM_FREQUENCY_80KHZ  = 80000
	PWM_FREQUENCY_100KHZ = 100000
)
