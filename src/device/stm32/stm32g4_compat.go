//go:build stm32g4

package stm32

import (
	"runtime/volatile"
	"unsafe"
)

// This file provides compatibility constants and accessor methods for
// STM32G4 timer registers that are missing from the SVD-generated device file.
// These are needed for advanced timer features like center-aligned PWM,
// complementary outputs, encoder mode, and ADC triggering.

// =============================================================================
// CR1 Center-Aligned Mode and Direction Constants (MISSING from device file)
// =============================================================================

const (
	// TIM_CR1_DIR - Direction bit (bit 4)
	TIM_CR1_DIR_Pos = 4
	TIM_CR1_DIR_Msk = 0x10
	TIM_CR1_DIR     = 0x10

	// TIM_CR1_CMS - Center-aligned mode selection (bits 5-6)
	TIM_CR1_CMS_Pos = 5
	TIM_CR1_CMS_Msk = 0x60

	// Center-aligned mode values
	TIM_CR1_CMS_EdgeAligned    = 0x0 << TIM_CR1_CMS_Pos // Edge-aligned (up counting)
	TIM_CR1_CMS_CenterAligned1 = 0x1 << TIM_CR1_CMS_Pos // Center-aligned mode 1
	TIM_CR1_CMS_CenterAligned2 = 0x2 << TIM_CR1_CMS_Pos // Center-aligned mode 2
	TIM_CR1_CMS_CenterAligned3 = 0x3 << TIM_CR1_CMS_Pos // Center-aligned mode 3
)

// =============================================================================
// CCMR2_Output Register Access (offset 0x1C) - MISSING from TIM_Type structure
// =============================================================================

const (
	// OC3M - Output Compare 3 mode (bits 4-6)
	TIM_CCMR2_Output_OC3M_Pos = 4
	TIM_CCMR2_Output_OC3M_Msk = 0x70

	// OC3PE - Output Compare 3 preload enable (bit 3)
	TIM_CCMR2_Output_OC3PE_Pos = 3
	TIM_CCMR2_Output_OC3PE     = 0x8

	// CC3S - Capture/Compare 3 selection (bits 0-1)
	TIM_CCMR2_Output_CC3S_Pos = 0
	TIM_CCMR2_Output_CC3S_Msk = 0x3

	// OC4M - Output Compare 4 mode (bits 12-14)
	TIM_CCMR2_Output_OC4M_Pos = 12
	TIM_CCMR2_Output_OC4M_Msk = 0x7000

	// OC4PE - Output Compare 4 preload enable (bit 11)
	TIM_CCMR2_Output_OC4PE_Pos = 11
	TIM_CCMR2_Output_OC4PE     = 0x800

	// CC4S - Capture/Compare 4 selection (bits 8-9)
	TIM_CCMR2_Output_CC4S_Pos = 8
	TIM_CCMR2_Output_CC4S_Msk = 0x300
)

// CCMR2_Output_Reg returns a pointer to the CCMR2_Output register
// The register is at offset 0x1C from the timer base address
func (o *TIM_Type) CCMR2_Output_Reg() *volatile.Register32 {
	baseAddr := uintptr(unsafe.Pointer(o))
	return (*volatile.Register32)(unsafe.Pointer(baseAddr + 0x1C))
}

// =============================================================================
// CCR3 and CCR4 Register Access (offsets 0x3C and 0x40) - MISSING from TIM_Type
// =============================================================================

// CCR3_Reg returns a pointer to the CCR3 register (offset 0x3C)
func (o *TIM_Type) CCR3_Reg() *volatile.Register32 {
	baseAddr := uintptr(unsafe.Pointer(o))
	return (*volatile.Register32)(unsafe.Pointer(baseAddr + 0x3C))
}

// CCR4_Reg returns a pointer to the CCR4 register (offset 0x40)
func (o *TIM_Type) CCR4_Reg() *volatile.Register32 {
	baseAddr := uintptr(unsafe.Pointer(o))
	return (*volatile.Register32)(unsafe.Pointer(baseAddr + 0x40))
}

// =============================================================================
// CCER Constants for Channels 2, 3 and 4 complementary outputs (MISSING)
// =============================================================================

const (
	// Channel 2 complementary (CC2NE is missing from device file)
	TIM_CCER_CC2NE_Pos = 6
	TIM_CCER_CC2NE_Msk = 0x40
	TIM_CCER_CC2NE     = 0x40 // Capture/Compare 2 complementary output enable

	// Channel 3
	TIM_CCER_CC3E_Pos  = 8
	TIM_CCER_CC3E_Msk  = 0x100
	TIM_CCER_CC3E      = 0x100 // Capture/Compare 3 output enable
	TIM_CCER_CC3P_Pos  = 9
	TIM_CCER_CC3P_Msk  = 0x200
	TIM_CCER_CC3P      = 0x200 // Capture/Compare 3 output polarity
	TIM_CCER_CC3NE_Pos = 10
	TIM_CCER_CC3NE_Msk = 0x400
	TIM_CCER_CC3NE     = 0x400 // Capture/Compare 3 complementary output enable
	TIM_CCER_CC3NP_Pos = 11
	TIM_CCER_CC3NP_Msk = 0x800
	TIM_CCER_CC3NP     = 0x800 // Capture/Compare 3 complementary output polarity

	// Channel 4
	TIM_CCER_CC4E_Pos  = 12
	TIM_CCER_CC4E_Msk  = 0x1000
	TIM_CCER_CC4E      = 0x1000 // Capture/Compare 4 output enable
	TIM_CCER_CC4P_Pos  = 13
	TIM_CCER_CC4P_Msk  = 0x2000
	TIM_CCER_CC4P      = 0x2000 // Capture/Compare 4 output polarity
	TIM_CCER_CC4NP_Pos = 15
	TIM_CCER_CC4NP_Msk = 0x8000
	TIM_CCER_CC4NP     = 0x8000 // Capture/Compare 4 complementary output polarity
)

// =============================================================================
// SR (Status Register) Constants for Channels 3 and 4 (MISSING)
// =============================================================================

const (
	TIM_SR_CC3IF_Pos = 3
	TIM_SR_CC3IF_Msk = 0x8
	TIM_SR_CC3IF     = 0x8 // Capture/Compare 3 interrupt flag

	TIM_SR_CC4IF_Pos = 4
	TIM_SR_CC4IF_Msk = 0x10
	TIM_SR_CC4IF     = 0x10 // Capture/Compare 4 interrupt flag

	TIM_SR_CC3OF_Pos = 11
	TIM_SR_CC3OF_Msk = 0x800
	TIM_SR_CC3OF     = 0x800 // Capture/Compare 3 overcapture flag

	TIM_SR_CC4OF_Pos = 12
	TIM_SR_CC4OF_Msk = 0x1000
	TIM_SR_CC4OF     = 0x1000 // Capture/Compare 4 overcapture flag
)

// =============================================================================
// DIER Constants for Channels 3 and 4 (MISSING)
// =============================================================================

const (
	TIM_DIER_CC3IE_Pos = 3
	TIM_DIER_CC3IE_Msk = 0x8
	TIM_DIER_CC3IE     = 0x8 // Capture/Compare 3 interrupt enable

	TIM_DIER_CC4IE_Pos = 4
	TIM_DIER_CC4IE_Msk = 0x10
	TIM_DIER_CC4IE     = 0x10 // Capture/Compare 4 interrupt enable

	TIM_DIER_CC3DE_Pos = 11
	TIM_DIER_CC3DE_Msk = 0x800
	TIM_DIER_CC3DE     = 0x800 // Capture/Compare 3 DMA request enable

	TIM_DIER_CC4DE_Pos = 12
	TIM_DIER_CC4DE_Msk = 0x1000
	TIM_DIER_CC4DE     = 0x1000 // Capture/Compare 4 DMA request enable
)

// =============================================================================
// SMCR Encoder Mode Constants (Named values MISSING)
// =============================================================================

const (
	// SMS - Slave mode selection encoder modes
	TIM_SMCR_SMS_EncoderMode1 = 0x1 // Counter counts on TI1 edges
	TIM_SMCR_SMS_EncoderMode2 = 0x2 // Counter counts on TI2 edges
	TIM_SMCR_SMS_EncoderMode3 = 0x3 // Counter counts on both TI1 and TI2 edges
)

// =============================================================================
// CCMR1_Input Named Constants (values MISSING, pos/mask exist)
// =============================================================================

const (
	// CC1S named values
	TIM_CCMR1_Input_CC1S_TI1 = 0x1 // CC1 mapped to TI1
	TIM_CCMR1_Input_CC1S_TI2 = 0x2 // CC1 mapped to TI2
	TIM_CCMR1_Input_CC1S_TRC = 0x3 // CC1 mapped to TRC

	// CC2S named values
	TIM_CCMR1_Input_CC2S_TI2 = 0x1 << 8 // CC2 mapped to TI2
	TIM_CCMR1_Input_CC2S_TI1 = 0x2 << 8 // CC2 mapped to TI1
	TIM_CCMR1_Input_CC2S_TRC = 0x3 << 8 // CC2 mapped to TRC
)

// =============================================================================
// CR2 Master Mode Selection Named Values (MISSING, pos/mask exist)
// =============================================================================

const (
	// MMS named values (using existing TIM_CR2_MMS_Pos = 4)
	TIM_CR2_MMS_Reset        = 0x0 << 4 // Reset - UG bit as TRGO
	TIM_CR2_MMS_Enable       = 0x1 << 4 // Enable - Counter enable as TRGO
	TIM_CR2_MMS_Update       = 0x2 << 4 // Update - Update event as TRGO
	TIM_CR2_MMS_ComparePulse = 0x3 << 4 // Compare Pulse - CC1IF as TRGO
	TIM_CR2_MMS_OC1REF       = 0x4 << 4 // OC1REF as TRGO
	TIM_CR2_MMS_OC2REF       = 0x5 << 4 // OC2REF as TRGO
	TIM_CR2_MMS_OC3REF       = 0x6 << 4 // OC3REF as TRGO
	TIM_CR2_MMS_OC4REF       = 0x7 << 4 // OC4REF as TRGO

	// MMS2 - Master Mode Selection 2 (bits 20-23, for TRGO2, advanced timers only)
	TIM_CR2_MMS2_Pos    = 20
	TIM_CR2_MMS2_Msk    = 0xF00000
	TIM_CR2_MMS2_Reset  = 0x0 << 20
	TIM_CR2_MMS2_Enable = 0x1 << 20
	TIM_CR2_MMS2_Update = 0x2 << 20
	TIM_CR2_MMS2_OC1REF = 0x4 << 20
	TIM_CR2_MMS2_OC2REF = 0x5 << 20
	TIM_CR2_MMS2_OC3REF = 0x6 << 20
	TIM_CR2_MMS2_OC4REF = 0x7 << 20
)

// =============================================================================
// EGR Additional Constants (CC3G, CC4G, B2G MISSING)
// =============================================================================

const (
	// CC3G - Capture/Compare 3 generation
	TIM_EGR_CC3G_Pos = 3
	TIM_EGR_CC3G_Msk = 0x8
	TIM_EGR_CC3G     = 0x8

	// CC4G - Capture/Compare 4 generation
	TIM_EGR_CC4G_Pos = 4
	TIM_EGR_CC4G_Msk = 0x10
	TIM_EGR_CC4G     = 0x10

	// B2G - Break 2 generation
	TIM_EGR_B2G_Pos = 8
	TIM_EGR_B2G_Msk = 0x100
	TIM_EGR_B2G     = 0x100
)

// =============================================================================
// BDTR Lock Level Named Constants (MISSING)
// =============================================================================

const (
	TIM_BDTR_LOCK_OFF  = 0x0 << 8 // No lock
	TIM_BDTR_LOCK_LVL1 = 0x1 << 8 // Lock level 1
	TIM_BDTR_LOCK_LVL2 = 0x2 << 8 // Lock level 2
	TIM_BDTR_LOCK_LVL3 = 0x3 << 8 // Lock level 3

	// BK2 constants (MISSING)
	TIM_BDTR_BK2F_Pos = 20
	TIM_BDTR_BK2F_Msk = 0xF00000

	TIM_BDTR_BK2E_Pos = 24
	TIM_BDTR_BK2E_Msk = 0x1000000
	TIM_BDTR_BK2E     = 0x1000000

	TIM_BDTR_BK2P_Pos = 25
	TIM_BDTR_BK2P_Msk = 0x2000000
	TIM_BDTR_BK2P     = 0x2000000

	TIM_BDTR_BK2DSRM_Pos = 27
	TIM_BDTR_BK2DSRM_Msk = 0x8000000
	TIM_BDTR_BK2DSRM     = 0x8000000

	TIM_BDTR_BK2BID_Pos = 29
	TIM_BDTR_BK2BID_Msk = 0x20000000
	TIM_BDTR_BK2BID     = 0x20000000
)

// Note: RCC_AHB1ENR_CORDICEN and RCC_AHB1ENR_FMACEN are now defined in stm32g431xx.go
