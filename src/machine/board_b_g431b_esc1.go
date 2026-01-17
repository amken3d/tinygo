//go:build b_g431b_esc1

// B-G431B-ESC1 Discovery Kit
//
// This board is a 3-phase brushless motor controller (ESC) based on STM32G431CB.
// It includes integrated gate drivers, current sensing OPAMPs, and power MOSFETs.
//
// Datasheet: https://www.st.com/resource/en/data_brief/b-g431b-esc1.pdf
// User Manual: https://www.st.com/resource/en/user_manual/um2516-bg431besc1-discovery-kit-stmicroelectronics.pdf
// Schematic: https://www.st.com/resource/en/schematic_pack/b-g431b-esc1-sch-reva.pdf
//
// Key features:
// - STM32G431CB microcontroller (170 MHz, 128KB Flash, 32KB RAM)
// - 3-phase gate driver (L6387) with integrated bootstrap
// - 3x STL180N6F7 power MOSFETs per phase (6 total)
// - 3-shunt current sensing with internal OPAMPs
// - BEMF sensing for sensorless control
// - Hall sensor / encoder inputs
// - CAN bus interface
// - 8-48V input voltage range

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
	LED_BUILTIN = LED_GREEN
	LED_GREEN   = PC6

	// User button on daughterboard
	BUTTON   = USER_BTN
	USER_BTN = PC10
)

// =============================================================================
// Motor Control - PWM Phase Outputs (directly control gate drivers)
// =============================================================================

// 3-Phase PWM outputs for brushless motor control
// High-side and low-side MOSFETs for each phase
// Connected to TIM1 channels with complementary outputs
const (
	// Phase U (A)
	PHASE_UH = PA8  // TIM1_CH1 - U phase high-side
	PHASE_UL = PC13 // TIM1_CH1N - U phase low-side

	// Phase V (B)
	PHASE_VH = PA9  // TIM1_CH2 - V phase high-side
	PHASE_VL = PA12 // TIM1_CH2N - V phase low-side

	// Phase W (C)
	PHASE_WH = PA10 // TIM1_CH3 - W phase high-side
	PHASE_WL = PB15 // TIM1_CH3N - W phase low-side
)

// Aliases for SimpleFOC compatibility
const (
	INH_A = PHASE_UH
	INL_A = PHASE_UL
	INH_B = PHASE_VH
	INL_B = PHASE_VL
	INH_C = PHASE_WH
	INL_C = PHASE_WL
)

// =============================================================================
// Motor Control - Current Sensing (via internal OPAMPs)
// =============================================================================

// Current sensing uses the internal OPAMPs for signal conditioning.
// The shunt resistors are connected between CURR_H and CURR_L pins.
// OPAMP output goes to internal ADC channel.
const (
	// Phase U current sensing (OPAMP1)
	CURR1_H = PA1 // OPAMP1_VINP - positive shunt terminal
	CURR1_L = PA3 // OPAMP1_VINM - negative shunt terminal / OPAMP1_VOUT

	// Phase V current sensing (OPAMP2)
	CURR2_H = PA7 // OPAMP2_VINP - positive shunt terminal
	CURR2_L = PA5 // OPAMP2_VINM - negative shunt terminal

	// Phase W current sensing (OPAMP3)
	CURR3_H = PB0 // OPAMP3_VINP - positive shunt terminal
	CURR3_L = PB2 // OPAMP3_VINM - negative shunt terminal
)

// ADC channels for current sensing (directly or via OPAMP internal output)
const (
	ADC_CURR1 = PA1 // ADC1_IN2 or use OPAMP1 internal (ADC1_IN13)
	ADC_CURR2 = PA7 // ADC2_IN4 or use OPAMP2 internal (ADC2_IN16)
	ADC_CURR3 = PB0 // ADC1_IN15 or use OPAMP3 internal
)

// =============================================================================
// Motor Control - BEMF Sensing (for sensorless control)
// =============================================================================

// Back-EMF sensing pins for sensorless 6-step commutation
const (
	BEMF1 = PA4  // ADC2_IN17 - Phase U BEMF
	BEMF2 = PC4  // ADC2_IN5 - Phase V BEMF
	BEMF3 = PB11 // ADC1_IN14 - Phase W BEMF
)

// =============================================================================
// Motor Control - Hall Sensors / Encoder
// =============================================================================

// Hall sensor inputs (directly usable or via timer capture)
// Also configurable as quadrature encoder inputs
const (
	HALL1 = PB6 // TIM4_CH1 - Hall sensor 1 / Encoder A
	HALL2 = PB7 // TIM4_CH2 - Hall sensor 2 / Encoder B
	HALL3 = PB8 // TIM4_CH3 - Hall sensor 3 / Encoder Z (index)

	ENCODER_A = HALL1
	ENCODER_B = HALL2
	ENCODER_Z = HALL3
)

// =============================================================================
// Motor Control - Bus Voltage and Temperature
// =============================================================================

const (
	// DC bus voltage sensing (voltage divider to ADC)
	VBUS = PA0 // ADC1_IN1 - DC bus voltage (scaled)

	// Temperature sensing
	TEMPERATURE = PB14 // ADC2_IN11 - NTC thermistor (if populated)
)

// =============================================================================
// Communication Interfaces
// =============================================================================

const (
	// UART pins (directly exposed on connector)
	UART_TX_PIN = PB3 // USART2_TX
	UART_RX_PIN = PB4 // USART2_RX

	// I2C pins (directly exposed on connector)
	// Note: Directly overlaps with Hall sensor pins!
	I2C0_SCL_PIN = PB8 // I2C1_SCL (shared with HALL3)
	I2C0_SDA_PIN = PB7 // I2C1_SDA (shared with HALL2)

	// CAN bus pins
	CAN_RX_PIN = PA11 // FDCAN1_RX
	CAN_TX_PIN = PB9  // FDCAN1_TX
	CAN_SHDN   = PC11 // CAN transceiver shutdown (directly active low)
	CAN_TERM   = PC14

	// SPI is not directly exposed but pins are available
	SPI0_SCK_PIN = PB3 // SPI1_SCK (shared with UART_TX)
	SPI0_SDI_PIN = PB4 // SPI1_MISO (shared with UART_RX)
	SPI0_SDO_PIN = PB5 // SPI1_MOSI
)

// =============================================================================
// Analog Pins
// =============================================================================

const (
	VBUS     = PA0 // VBUS voltage sensing
	U_OPAMPH = PA1 // Current sense 1 (OPAMP1+)
	U_OPAMPL = PA3 // Current sense 1 out
	V_OPAML  = PA5 // Current sense 2-
	V_OPAMPH = PA7 // Current sense 2+
	W_OPAMPH = PB0 // Current sense 3+
	W_OPAML  = PB2 // Current sense 3-
)

// =============================================================================
// PWM Input
// =============================================================================

const (
	// External PWM input for ESC control
	PWM_INPUT = PA15 // TIM2_CH1 - PWM input capture
)

// =============================================================================
// Potentiometer (on daughterboard)
// =============================================================================

const (
	// Potentiometer for manual speed control (directly on daughterboard)
	POTENTIOMETER = PB12 // ADC1_IN3
)

// =============================================================================
// Hardware Peripherals
// =============================================================================

var (
	// UART2 is exposed on the connector
	UART1  = &_UART1
	_UART1 = UART{
		Buffer:            NewRingBuffer(),
		Bus:               stm32.USART2,
		TxAltFuncSelector: AF7_USART1_USART2_USART3,
		RxAltFuncSelector: AF7_USART1_USART2_USART3,
	}
	DefaultUART = UART1

	// I2C1 (shared with Hall sensor pins - choose one or the other)
	I2C1 = &I2C{
		Bus:             stm32.I2C1,
		AltFuncSelector: AF4_I2C1_I2C2_I2C3_I2C4,
	}
	I2C0 = I2C1

	// SPI1 (limited availability due to pin sharing with UART)
	SPI1 = &SPI{
		Bus:             stm32.SPI1,
		AltFuncSelector: AF5_SPI1_SPI2_I2S2_I2S3,
	}
	SPI0 = SPI1
)

func init() {
	UART1.Interrupt = interrupt.New(stm32.IRQ_USART2, _UART1.handleInterrupt)
}

// =============================================================================
// Motor Control Helper Constants
// =============================================================================

// Timer assignments for motor control
const (
	// TIM1 is used for 3-phase PWM generation (directly center-aligned mode)
	MOTOR_PWM_TIMER = 1

	// TIM4 can be used for Hall sensor decoding or encoder
	HALL_ENCODER_TIMER = 4

	// TIM2 can be used for PWM input capture
	PWM_INPUT_TIMER = 2

	// TIM6/TIM7 can be used for ADC triggering
	ADC_TRIGGER_TIMER = 6
)

// Recommended PWM frequency for BLDC motor control
const (
	// 20 kHz is typical for brushless motor control
	// Higher frequencies reduce audible noise but increase switching losses
	PWM_FREQUENCY_20KHZ = 20000
	PWM_FREQUENCY_30KHZ = 30000
	PWM_FREQUENCY_40KHZ = 40000
)
