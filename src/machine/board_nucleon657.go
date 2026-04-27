//go:build nucleon657

package machine

import (
	"device/stm32"
	"runtime/interrupt"
)

// Pin mappings for the NUCLEO-N657X0-Q board. Matches ST's stm32n6xx_nucleo
// BSP (https://github.com/STMicroelectronics/stm32n6xx-nucleo-bsp).
const (
	LED_BLUE    = PG8
	LED_RED     = PG10
	LED_GREEN   = PG0
	LED_BUILTIN = LED_BLUE
	LED         = LED_BUILTIN
)

const (
	BUTTON      = BUTTON_USER
	BUTTON_USER = PC13
)

// UART pins. USART1 on PE5/PE6 (AF7) is wired to the ST-LINK Virtual COM Port.
const (
	UART_TX_PIN = PE5
	UART_RX_PIN = PE6
	UART_ALT_FN = 7 // GPIO_AF7_USART1
)

var (
	UART1  = &_UART1
	_UART1 = UART{
		Buffer:            NewRingBuffer(),
		Bus:               stm32.USART1,
		TxAltFuncSelector: UART_ALT_FN,
		RxAltFuncSelector: UART_ALT_FN,
	}
	DefaultUART = UART1
)

// I2C2 is wired to the camera connector (CN7) on the NUCLEO-N657X0-Q.
// SCL/SDA are PB10/PB11 with alt function 4 (matches ST's BSP
// stm32n6xx_nucleo_bus.h). The IMX335 sensor lives at I2C address 0x34.
//
// I2C0_SCL_PIN/I2C0_SDA_PIN are the default-pin fallback the i2c_revb
// driver uses when Configure() is called without explicit SCL/SDA;
// alias them to the I2C2 pins so a bare config works.
const (
	I2C2_SCL_PIN = PB10
	I2C2_SDA_PIN = PB11
	I2C2_ALT_FN  = 4 // GPIO_AF4_I2C2

	I2C0_SCL_PIN = I2C2_SCL_PIN
	I2C0_SDA_PIN = I2C2_SDA_PIN
)

var (
	I2C2  = &_I2C2
	_I2C2 = I2C{
		Bus:             stm32.I2C2,
		AltFuncSelector: I2C2_ALT_FN,
	}
)

func init() {
	UART1.Interrupt = interrupt.New(stm32.IRQ_USART1, _UART1.handleInterrupt)
}

// USB identification strings + IDs used when the USB device stack is
// brought up via machine.USBDev.Configure. 0x0483 is ST's assigned VID;
// the PID is an arbitrary in-house value for development. Override by
// setting usb.VendorID / usb.ProductID / usb.Manufacturer / usb.Product
// from application code before calling Configure.
const (
	usb_STRING_PRODUCT      = "Tinygo uvc driver on NUCLEO-N657"
	usb_STRING_MANUFACTURER = "STMicroelectronics"
)

var (
	usb_VID uint16 = 0x0483
	usb_PID uint16 = 0x5740
)
