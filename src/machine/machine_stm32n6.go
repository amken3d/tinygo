//go:build stm32n6

package machine

// Peripheral abstraction layer for the stm32n6 (Cortex-M55) family.

import (
	"device/stm32"
	"runtime/interrupt"
	"runtime/volatile"
	"unsafe"
)

// 96-bit factory unique device ID, in OTP. Per STM32N6 reference manual §32.
var deviceIDAddr = []uintptr{0x0BFA0500, 0x0BFA0504, 0x0BFA0508}

// Alternate function numbers are peripheral-specific on STM32. For N6 the
// numbering broadly follows the U5/L5 conventions; callers should consult the
// datasheet for the exact AF for each pin. We only name the AFs we currently
// use; the rest are spelled out as plain integers.
const (
	AF7_USARTx = 7
)

const (
	PA0  = portA + 0
	PA1  = portA + 1
	PA2  = portA + 2
	PA3  = portA + 3
	PA4  = portA + 4
	PA5  = portA + 5
	PA6  = portA + 6
	PA7  = portA + 7
	PA8  = portA + 8
	PA9  = portA + 9
	PA10 = portA + 10
	PA11 = portA + 11
	PA12 = portA + 12
	PA13 = portA + 13
	PA14 = portA + 14
	PA15 = portA + 15

	PB0  = portB + 0
	PB1  = portB + 1
	PB2  = portB + 2
	PB3  = portB + 3
	PB4  = portB + 4
	PB5  = portB + 5
	PB6  = portB + 6
	PB7  = portB + 7
	PB8  = portB + 8
	PB9  = portB + 9
	PB10 = portB + 10
	PB11 = portB + 11
	PB12 = portB + 12
	PB13 = portB + 13
	PB14 = portB + 14
	PB15 = portB + 15

	PC0  = portC + 0
	PC1  = portC + 1
	PC2  = portC + 2
	PC3  = portC + 3
	PC4  = portC + 4
	PC5  = portC + 5
	PC6  = portC + 6
	PC7  = portC + 7
	PC8  = portC + 8
	PC9  = portC + 9
	PC10 = portC + 10
	PC11 = portC + 11
	PC12 = portC + 12
	PC13 = portC + 13
	PC14 = portC + 14
	PC15 = portC + 15

	PD0  = portD + 0
	PD1  = portD + 1
	PD2  = portD + 2
	PD3  = portD + 3
	PD4  = portD + 4
	PD5  = portD + 5
	PD6  = portD + 6
	PD7  = portD + 7
	PD8  = portD + 8
	PD9  = portD + 9
	PD10 = portD + 10
	PD11 = portD + 11
	PD12 = portD + 12
	PD13 = portD + 13
	PD14 = portD + 14
	PD15 = portD + 15

	PE0  = portE + 0
	PE1  = portE + 1
	PE2  = portE + 2
	PE3  = portE + 3
	PE4  = portE + 4
	PE5  = portE + 5
	PE6  = portE + 6
	PE7  = portE + 7
	PE8  = portE + 8
	PE9  = portE + 9
	PE10 = portE + 10
	PE11 = portE + 11
	PE12 = portE + 12
	PE13 = portE + 13
	PE14 = portE + 14
	PE15 = portE + 15

	PF0  = portF + 0
	PF1  = portF + 1
	PF2  = portF + 2
	PF3  = portF + 3
	PF4  = portF + 4
	PF5  = portF + 5
	PF6  = portF + 6
	PF7  = portF + 7
	PF8  = portF + 8
	PF9  = portF + 9
	PF10 = portF + 10
	PF11 = portF + 11
	PF12 = portF + 12
	PF13 = portF + 13
	PF14 = portF + 14
	PF15 = portF + 15

	PG0  = portG + 0
	PG1  = portG + 1
	PG2  = portG + 2
	PG3  = portG + 3
	PG4  = portG + 4
	PG5  = portG + 5
	PG6  = portG + 6
	PG7  = portG + 7
	PG8  = portG + 8
	PG9  = portG + 9
	PG10 = portG + 10
	PG11 = portG + 11
	PG12 = portG + 12
	PG13 = portG + 13
	PG14 = portG + 14
	PG15 = portG + 15

	PH0  = portH + 0
	PH1  = portH + 1
	PH2  = portH + 2
	PH3  = portH + 3
	PH4  = portH + 4
	PH5  = portH + 5
	PH6  = portH + 6
	PH7  = portH + 7
	PH8  = portH + 8
	PH9  = portH + 9
	PH10 = portH + 10
	PH11 = portH + 11
	PH12 = portH + 12
	PH13 = portH + 13
	PH14 = portH + 14
	PH15 = portH + 15
)

func (p Pin) getPort() *stm32.GPIO_Type {
	switch p / 16 {
	case 0:
		return stm32.GPIOA
	case 1:
		return stm32.GPIOB
	case 2:
		return stm32.GPIOC
	case 3:
		return stm32.GPIOD
	case 4:
		return stm32.GPIOE
	case 5:
		return stm32.GPIOF
	case 6:
		return stm32.GPIOG
	case 7:
		return stm32.GPIOH
	case 8:
		return stm32.GPION

	default:
		panic("machine: unknown port")
	}
}

// enableClock enables the AHB4 clock for the GPIO port owning this pin.
//
// On STM32N6 the plain AHB4ENR register is not directly writable by the
// application — it reflects the active clock-gate state but does not accept
// RMW writes. Instead, there is an Enable Set Register (AHB4ENSR) where
// writing a 1 to a bit atomically sets the corresponding bit in AHB4ENR,
// and a mirror Clear register (AHB4ENCR). The bit layout of ENSR matches
// ENR, so the ENR-named constants are reused as write values for ENSR.
func (p Pin) enableClock() {
	switch p / 16 {
	case 0:
		stm32.RCC.AHB4ENSR.Set(stm32.RCC_AHB4ENR_GPIOAEN)
	case 1:
		stm32.RCC.AHB4ENSR.Set(stm32.RCC_AHB4ENR_GPIOBEN)
	case 2:
		stm32.RCC.AHB4ENSR.Set(stm32.RCC_AHB4ENR_GPIOCEN)
	case 3:
		stm32.RCC.AHB4ENSR.Set(stm32.RCC_AHB4ENR_GPIODEN)
	case 4:
		stm32.RCC.AHB4ENSR.Set(stm32.RCC_AHB4ENR_GPIOEEN)
	case 5:
		stm32.RCC.AHB4ENSR.Set(stm32.RCC_AHB4ENR_GPIOFEN)
	case 6:
		stm32.RCC.AHB4ENSR.Set(stm32.RCC_AHB4ENR_GPIOGEN)
	case 7:
		stm32.RCC.AHB4ENSR.Set(stm32.RCC_AHB4ENR_GPIOHEN)
	case 8:
		stm32.RCC.AHB4ENSR.Set(stm32.RCC_AHB4ENR_GPIONEN)

	default:
		panic("machine: unknown port")
	}
	_ = stm32.RCC.AHB4ENR.Get() // readback on ENR to settle the clock-gate
}

// enableAltFuncClock enables the peripheral clock for a given peripheral so
// its GPIO alternate-function routing is active. Uses the ENSR aliases for
// the same reason enableClock does — plain ENR registers are not writable.
func enableAltFuncClock(bus unsafe.Pointer) {
	switch bus {
	case unsafe.Pointer(stm32.USART1):
		stm32.RCC.APB2ENSR.Set(stm32.RCC_APB2ENR_USART1EN)
	case unsafe.Pointer(stm32.USART2):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_USART2EN)
	case unsafe.Pointer(stm32.USART3):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_USART3EN)
	case unsafe.Pointer(stm32.USART6):
		stm32.RCC.APB2ENSR.Set(stm32.RCC_APB2ENR_USART6EN)
	case unsafe.Pointer(stm32.UART4):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_UART4EN)
	case unsafe.Pointer(stm32.UART5):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_UART5EN)
	case unsafe.Pointer(stm32.UART7):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_UART7EN)
	case unsafe.Pointer(stm32.UART8):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_UART8EN)
	case unsafe.Pointer(stm32.LPUART1):
		stm32.RCC.APB4LENSR.Set(stm32.RCC_APB4LENR_LPUART1EN)
	case unsafe.Pointer(stm32.I2C1):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_I2C1EN)
	case unsafe.Pointer(stm32.I2C2):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_I2C2EN)
	case unsafe.Pointer(stm32.I2C3):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_I2C3EN)
	case unsafe.Pointer(stm32.I2C4):
		stm32.RCC.APB4LENSR.Set(stm32.RCC_APB4LENR_I2C4EN)
	case unsafe.Pointer(stm32.TIM1):
		stm32.RCC.APB2ENSR.Set(stm32.RCC_APB2ENR_TIM1EN)
	case unsafe.Pointer(stm32.TIM2):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_TIM2EN)
	case unsafe.Pointer(stm32.TIM3):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_TIM3EN)
	case unsafe.Pointer(stm32.TIM4):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_TIM4EN)
	case unsafe.Pointer(stm32.TIM5):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_TIM5EN)
	case unsafe.Pointer(stm32.TIM6):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_TIM6EN)
	case unsafe.Pointer(stm32.TIM7):
		stm32.RCC.APB1LENSR.Set(stm32.RCC_APB1LENR_TIM7EN)
	case unsafe.Pointer(stm32.TIM8):
		stm32.RCC.APB2ENSR.Set(stm32.RCC_APB2ENR_TIM8EN)
	case unsafe.Pointer(stm32.TIM15):
		stm32.RCC.APB2ENSR.Set(stm32.RCC_APB2ENR_TIM15EN)
	case unsafe.Pointer(stm32.TIM16):
		stm32.RCC.APB2ENSR.Set(stm32.RCC_APB2ENR_TIM16EN)
	case unsafe.Pointer(stm32.TIM17):
		stm32.RCC.APB2ENSR.Set(stm32.RCC_APB2ENR_TIM17EN)
	case unsafe.Pointer(stm32.SYSCFG):
		stm32.RCC.APB4HENSR.Set(stm32.RCC_APB4HENR_SYSCFGEN)
	}
}

// EXTI on N6 has one IRQ per external line (0..15), mirroring the U5/L5 model.
func (p Pin) registerInterrupt() interrupt.Interrupt {
	pin := uint8(p) % 16
	switch pin {
	case 0:
		return interrupt.New(stm32.IRQ_EXTI0, func(interrupt.Interrupt) { handlePinInterrupt(0) })
	case 1:
		return interrupt.New(stm32.IRQ_EXTI1, func(interrupt.Interrupt) { handlePinInterrupt(1) })
	case 2:
		return interrupt.New(stm32.IRQ_EXTI2, func(interrupt.Interrupt) { handlePinInterrupt(2) })
	case 3:
		return interrupt.New(stm32.IRQ_EXTI3, func(interrupt.Interrupt) { handlePinInterrupt(3) })
	case 4:
		return interrupt.New(stm32.IRQ_EXTI4, func(interrupt.Interrupt) { handlePinInterrupt(4) })
	case 5:
		return interrupt.New(stm32.IRQ_EXTI5, func(interrupt.Interrupt) { handlePinInterrupt(5) })
	case 6:
		return interrupt.New(stm32.IRQ_EXTI6, func(interrupt.Interrupt) { handlePinInterrupt(6) })
	case 7:
		return interrupt.New(stm32.IRQ_EXTI7, func(interrupt.Interrupt) { handlePinInterrupt(7) })
	case 8:
		return interrupt.New(stm32.IRQ_EXTI8, func(interrupt.Interrupt) { handlePinInterrupt(8) })
	case 9:
		return interrupt.New(stm32.IRQ_EXTI9, func(interrupt.Interrupt) { handlePinInterrupt(9) })
	case 10:
		return interrupt.New(stm32.IRQ_EXTI10, func(interrupt.Interrupt) { handlePinInterrupt(10) })
	case 11:
		return interrupt.New(stm32.IRQ_EXTI11, func(interrupt.Interrupt) { handlePinInterrupt(11) })
	case 12:
		return interrupt.New(stm32.IRQ_EXTI12, func(interrupt.Interrupt) { handlePinInterrupt(12) })
	case 13:
		return interrupt.New(stm32.IRQ_EXTI13, func(interrupt.Interrupt) { handlePinInterrupt(13) })
	case 14:
		return interrupt.New(stm32.IRQ_EXTI14, func(interrupt.Interrupt) { handlePinInterrupt(14) })
	case 15:
		return interrupt.New(stm32.IRQ_EXTI15, func(interrupt.Interrupt) { handlePinInterrupt(15) })
	}
	return interrupt.Interrupt{}
}

func handlePinInterrupt(pin uint8) {
	if stm32.EXTI.RPR1.HasBits(1<<pin) || stm32.EXTI.FPR1.HasBits(1<<pin) {
		stm32.EXTI.RPR1.Set(1 << pin)
		stm32.EXTI.FPR1.Set(1 << pin)

		callback := pinCallbacks[pin]
		if callback != nil {
			callback(interruptPins[pin])
		}
	}
}

// ---------- Timer layer ----------

// Bus and timer clocks match the chip's reset state. HSI (64 MHz) drives
// both CPUCLK and SYSCLK directly; RCC.CFGR2 resets with HPRE = /2 and
// PPREx = /1, so after reset:
//
//	CPUCLK   64 MHz   (HSI, feeds the M55 core and SysTick)
//	SYSCLK   64 MHz   (HSI)
//	HCLK     32 MHz   (SYSCLK / HPRE=2)
//	PCLKx    32 MHz   (HCLK / PPREx=1)
//	TIMxCLK  32 MHz   (APBdiv=1 ⇒ equal to PCLK)
//
// initCLK in runtime_stm32n6.go leaves these at their reset values. If a
// future change programs PLL1 / IC1 / the HPRE field, update these constants
// and CPUFrequency() in lockstep.
const (
	APB1_TIM_FREQ = 32e6 // 32 MHz
	APB2_TIM_FREQ = 32e6 // 32 MHz
)

func CPUFrequency() uint32 {
	return 64_000_000
}

// Point TIM EnableRegister at the ENSR aliases so the shared
// machine_stm32_tim.go code (which does EnableRegister.SetBits(EnableFlag))
// actually enables the peripheral clock on N6.
var (
	TIM1 = TIM{
		EnableRegister: &stm32.RCC.APB2ENSR,
		EnableFlag:     stm32.RCC_APB2ENR_TIM1EN,
		Device:         stm32.TIM1,
		busFreq:        APB2_TIM_FREQ,
	}

	TIM2 = TIM{
		EnableRegister: &stm32.RCC.APB1LENSR,
		EnableFlag:     stm32.RCC_APB1LENR_TIM2EN,
		Device:         stm32.TIM2,
		busFreq:        APB1_TIM_FREQ,
	}

	TIM3 = TIM{
		EnableRegister: &stm32.RCC.APB1LENSR,
		EnableFlag:     stm32.RCC_APB1LENR_TIM3EN,
		Device:         stm32.TIM3,
		busFreq:        APB1_TIM_FREQ,
	}

	TIM4 = TIM{
		EnableRegister: &stm32.RCC.APB1LENSR,
		EnableFlag:     stm32.RCC_APB1LENR_TIM4EN,
		Device:         stm32.TIM4,
		busFreq:        APB1_TIM_FREQ,
	}

	TIM5 = TIM{
		EnableRegister: &stm32.RCC.APB1LENSR,
		EnableFlag:     stm32.RCC_APB1LENR_TIM5EN,
		Device:         stm32.TIM5,
		busFreq:        APB1_TIM_FREQ,
	}

	TIM6 = TIM{
		EnableRegister: &stm32.RCC.APB1LENSR,
		EnableFlag:     stm32.RCC_APB1LENR_TIM6EN,
		Device:         stm32.TIM6,
		busFreq:        APB1_TIM_FREQ,
	}

	TIM7 = TIM{
		EnableRegister: &stm32.RCC.APB1LENSR,
		EnableFlag:     stm32.RCC_APB1LENR_TIM7EN,
		Device:         stm32.TIM7,
		busFreq:        APB1_TIM_FREQ,
	}

	TIM8 = TIM{
		EnableRegister: &stm32.RCC.APB2ENSR,
		EnableFlag:     stm32.RCC_APB2ENR_TIM8EN,
		Device:         stm32.TIM8,
		busFreq:        APB2_TIM_FREQ,
	}

	TIM15 = TIM{
		EnableRegister: &stm32.RCC.APB2ENSR,
		EnableFlag:     stm32.RCC_APB2ENR_TIM15EN,
		Device:         stm32.TIM15,
		busFreq:        APB2_TIM_FREQ,
	}

	TIM16 = TIM{
		EnableRegister: &stm32.RCC.APB2ENSR,
		EnableFlag:     stm32.RCC_APB2ENR_TIM16EN,
		Device:         stm32.TIM16,
		busFreq:        APB2_TIM_FREQ,
	}

	TIM17 = TIM{
		EnableRegister: &stm32.RCC.APB2ENSR,
		EnableFlag:     stm32.RCC_APB2ENR_TIM17EN,
		Device:         stm32.TIM17,
		busFreq:        APB2_TIM_FREQ,
	}
)

func (t *TIM) registerUPInterrupt() interrupt.Interrupt {
	switch t {
	case &TIM1:
		return interrupt.New(stm32.IRQ_TIM1_UP, TIM1.handleUPInterrupt)
	case &TIM2:
		return interrupt.New(stm32.IRQ_TIM2, TIM2.handleUPInterrupt)
	case &TIM3:
		return interrupt.New(stm32.IRQ_TIM3, TIM3.handleUPInterrupt)
	case &TIM4:
		return interrupt.New(stm32.IRQ_TIM4, TIM4.handleUPInterrupt)
	case &TIM5:
		return interrupt.New(stm32.IRQ_TIM5, TIM5.handleUPInterrupt)
	case &TIM6:
		return interrupt.New(stm32.IRQ_TIM6, TIM6.handleUPInterrupt)
	case &TIM7:
		return interrupt.New(stm32.IRQ_TIM7, TIM7.handleUPInterrupt)
	case &TIM8:
		return interrupt.New(stm32.IRQ_TIM8_UP, TIM8.handleUPInterrupt)
	case &TIM15:
		return interrupt.New(stm32.IRQ_TIM15, TIM15.handleUPInterrupt)
	case &TIM16:
		return interrupt.New(stm32.IRQ_TIM16, TIM16.handleUPInterrupt)
	case &TIM17:
		return interrupt.New(stm32.IRQ_TIM17, TIM17.handleUPInterrupt)
	}
	return interrupt.Interrupt{}
}

func (t *TIM) registerOCInterrupt() interrupt.Interrupt {
	switch t {
	case &TIM1:
		return interrupt.New(stm32.IRQ_TIM1_CC, TIM1.handleOCInterrupt)
	case &TIM2:
		return interrupt.New(stm32.IRQ_TIM2, TIM2.handleOCInterrupt)
	case &TIM3:
		return interrupt.New(stm32.IRQ_TIM3, TIM3.handleOCInterrupt)
	case &TIM4:
		return interrupt.New(stm32.IRQ_TIM4, TIM4.handleOCInterrupt)
	case &TIM5:
		return interrupt.New(stm32.IRQ_TIM5, TIM5.handleOCInterrupt)
	case &TIM6:
		return interrupt.New(stm32.IRQ_TIM6, TIM6.handleOCInterrupt)
	case &TIM7:
		return interrupt.New(stm32.IRQ_TIM7, TIM7.handleOCInterrupt)
	case &TIM8:
		return interrupt.New(stm32.IRQ_TIM8_CC, TIM8.handleOCInterrupt)
	case &TIM15:
		return interrupt.New(stm32.IRQ_TIM15, TIM15.handleOCInterrupt)
	case &TIM16:
		return interrupt.New(stm32.IRQ_TIM16, TIM16.handleOCInterrupt)
	case &TIM17:
		return interrupt.New(stm32.IRQ_TIM17, TIM17.handleOCInterrupt)
	}
	return interrupt.Interrupt{}
}

func (t *TIM) enableMainOutput() {
	t.Device.BDTR.SetBits(stm32.TIM_BDTR_MOE)
}

type arrtype = uint32
type arrRegType = volatile.Register32
type psctype = uint16

const (
	ARR_MAX = 0x10000
	PSC_MAX = 0x10000
)

func initRNG() {
	stm32.RCC.AHB3ENSR.Set(stm32.RCC_AHB3ENR_RNGEN)
	stm32.RNG.CR.SetBits(stm32.RNG_CR_RNGEN)
}
