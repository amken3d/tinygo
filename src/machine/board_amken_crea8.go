//go:build crea8

// Crea8 Motion Control Board - RP2350B based motor control board
// https://www.amken3d.com

package machine

// GPIO pins with functional names
const (
	// RPi Communication pins (UART0 with flow control or SPI0 via jumpers)
	RPI_TX   Pin = GPIO0 // TX to RPi / SPI0 MISO
	RPI_RX   Pin = GPIO1 // RX from RPi / SPI0 CS
	RPI_CTS  Pin = GPIO2 // CTS / SPI0 SCK
	RPI_RTS  Pin = GPIO3 // RTS / SPI0 MOSI
	COM_MISO Pin = GPIO0 // SPI0 MISO (when in SPI mode)
	COM_CS   Pin = GPIO1 // SPI0 CS (when in SPI mode)
	COM_SCK  Pin = GPIO2 // SPI0 SCK (when in SPI mode)
	COM_MOSI Pin = GPIO3 // SPI0 MOSI (when in SPI mode)

	// I2C pins for TCA6408 I/O expander
	MCU_SDA Pin = GPIO4
	MCU_SCL Pin = GPIO5

	// High Power PWM MOSFETs
	HPMOS1 Pin = GPIO6
	HPMOS2 Pin = GPIO7

	// Low Power PWM MOSFETs
	LPMOS1 Pin = GPIO8
	LPMOS2 Pin = GPIO11
	LPMOS3 Pin = GPIO21
	LPMOS4 Pin = GPIO47

	// Stepper Motor 1
	STEP1 Pin = GPIO10
	DIR1  Pin = GPIO9

	// Stepper Motor 2
	STEP2 Pin = GPIO12
	DIR2  Pin = GPIO13

	// Stepper Motor 3
	STEP3 Pin = GPIO14
	DIR3  Pin = GPIO15

	// Stepper Motor 4
	STEP4 Pin = GPIO16
	DIR4  Pin = GPIO17

	// Stepper Motor 5
	STEP5 Pin = GPIO18
	DIR5  Pin = GPIO19
	CS5   Pin = GPIO20

	// Stepper Motor 6
	STEP6 Pin = GPIO22
	DIR6  Pin = GPIO23
	CS6   Pin = GPIO24

	// Stepper Motor 7
	STEP7 Pin = GPIO26
	DIR7  Pin = GPIO25
	CS7   Pin = GPIO27

	// Stepper driver chip selects
	MOTOR_CS1 Pin = GPIO34
	MOTOR_CS2 Pin = GPIO35
	MOTOR_CS3 Pin = GPIO38
	MOTOR_CS4 Pin = GPIO39
	MOTOR_CS5 Pin = GPIO20
	MOTOR_CS6 Pin = GPIO24
	MOTOR_CS7 Pin = GPIO27

	// SN74HC595 Shift Register Enable pins (directly control TCA6408)
	ENABLE1 Pin = GPIO33
	ENABLE2 Pin = GPIO32
	ENABLE3 Pin = GPIO31

	// I/O Expander interrupt
	IO_INT Pin = GPIO28

	// General purpose inputs
	INPUT1 Pin = GPIO29
	INPUT2 Pin = GPIO30

	// Secondary UART
	UART_TX Pin = GPIO36
	UART_RX Pin = GPIO37

	// Motor Driver SPI bus
	MOTOR_MISO Pin = GPIO40
	MOTOR_SCK  Pin = GPIO42
	MOTOR_MOSI Pin = GPIO43

	// NeoPixel headers
	NEO1     Pin = GPIO41
	NEO2     Pin = GPIO44
	NEOPIXEL Pin = GPIO41 // Default NeoPixel alias
	WS2812   Pin = GPIO41

	// ADC inputs
	ADC_1 Pin = GPIO46
	ADC_2 Pin = GPIO45

	// LED - using NEO1 as there's no onboard LED
	LED Pin = GPIO41

	// Onboard crystal oscillator frequency, in MHz.
	xoscFreq = 12 // MHz
)

// Generic GPIO pin aliases
const (
	GP0  Pin = GPIO0
	GP1  Pin = GPIO1
	GP2  Pin = GPIO2
	GP3  Pin = GPIO3
	GP4  Pin = GPIO4
	GP5  Pin = GPIO5
	GP6  Pin = GPIO6
	GP7  Pin = GPIO7
	GP8  Pin = GPIO8
	GP9  Pin = GPIO9
	GP10 Pin = GPIO10
	GP11 Pin = GPIO11
	GP12 Pin = GPIO12
	GP13 Pin = GPIO13
	GP14 Pin = GPIO14
	GP15 Pin = GPIO15
	GP16 Pin = GPIO16
	GP17 Pin = GPIO17
	GP18 Pin = GPIO18
	GP19 Pin = GPIO19
	GP20 Pin = GPIO20
	GP21 Pin = GPIO21
	GP22 Pin = GPIO22
	GP23 Pin = GPIO23
	GP24 Pin = GPIO24
	GP25 Pin = GPIO25
	GP26 Pin = GPIO26
	GP27 Pin = GPIO27
	GP28 Pin = GPIO28
	GP29 Pin = GPIO29
	GP30 Pin = GPIO30
	GP31 Pin = GPIO31
	GP32 Pin = GPIO32
	GP33 Pin = GPIO33
	GP34 Pin = GPIO34
	GP35 Pin = GPIO35
	GP36 Pin = GPIO36
	GP37 Pin = GPIO37
	GP38 Pin = GPIO38
	GP39 Pin = GPIO39
	GP40 Pin = GPIO40
	GP41 Pin = GPIO41
	GP42 Pin = GPIO42
	GP43 Pin = GPIO43
	GP44 Pin = GPIO44
	GP45 Pin = GPIO45
	GP46 Pin = GPIO46
	GP47 Pin = GPIO47
)

// I2C pins - MCU I2C bus for TCA6408 I/O expander
const (
	I2C0_SDA_PIN = MCU_SDA // GPIO4
	I2C0_SCL_PIN = MCU_SCL // GPIO5

	I2C1_SDA_PIN = GPIO6 // Alternative I2C1 (shared with HPMOS1)
	I2C1_SCL_PIN = GPIO7 // Alternative I2C1 (shared with HPMOS2)

	SDA_PIN = I2C0_SDA_PIN
	SCL_PIN = I2C0_SCL_PIN
)

// SPI default pins - Motor driver SPI bus
const (
	// SPI0 - RPi Communication
	SPI0_SCK_PIN = COM_SCK  // GPIO2
	SPI0_SDO_PIN = COM_MOSI // GPIO3 (Tx/MOSI)
	SPI0_SDI_PIN = COM_MISO // GPIO0 (Rx/MISO)

	// SPI1 - Motor drivers
	SPI1_SCK_PIN = MOTOR_SCK  // GPIO42
	SPI1_SDO_PIN = MOTOR_MOSI // GPIO43 (Tx/MOSI)
	SPI1_SDI_PIN = MOTOR_MISO // GPIO40 (Rx/MISO)

	// Default SPI aliases point to motor driver SPI
	MOSI Pin = MOTOR_MOSI
	MISO Pin = MOTOR_MISO
	SCK  Pin = MOTOR_SCK
)

// UART pins
const (
	// UART0 - RPi Communication
	UART0_TX_PIN = RPI_TX // GPIO0
	UART0_RX_PIN = RPI_RX // GPIO1

	// UART1 - General purpose
	UART1_TX_PIN = UART_TX // GPIO36
	UART1_RX_PIN = UART_RX // GPIO37

	// Default UART pins point to UART1 (since UART0 is for RPi comms)
	UART_TX_PIN = UART1_TX_PIN
	UART_RX_PIN = UART1_RX_PIN

	// TX/RX aliases
	TX Pin = UART_TX
	RX Pin = UART_RX
)

// DefaultUART is UART1 since UART0 is dedicated to RPi communication
var DefaultUART = UART1

// USB identifiers
const (
	usb_STRING_PRODUCT      = "Crea8 Motion Control Board"
	usb_STRING_MANUFACTURER = "Amken LLC."
)

var (
	usb_VID uint16 = 0x2E8A
	usb_PID uint16 = 0x10EE
)
