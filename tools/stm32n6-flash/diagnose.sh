#!/bin/bash
# Flash a TinyGo ELF, let it run briefly, then halt and dump TIM16 / NVIC /
# VTOR / tick-counter state. Use this when time.Sleep hangs — the register
# dump pinpoints which link in the chain is broken:
#
#   RCC.APB2ENR bit 17 (TIM16EN)   : TIM16 clock gate
#   TIM16.CR1.CEN bit 0            : counter running
#   TIM16.DIER.UIE bit 0           : update (overflow) interrupt enabled
#   TIM16.SR.UIF bit 0             : update flag (latched; set each overflow)
#   TIM16.CNT (counts continually if clocked + CEN)
#   SCB.VTOR                       : should be 0x34180400
#   NVIC.ISER0..3                  : TIM16 IRQ = 133 → ISER4 bit (133-128)=5
#   ticks variable                 : incremented by handleTick

set -e

ELF="${1:?usage: $0 path/to/app.elf}"
VECTOR_BASE="${STM32N6_VECTOR:-0x34180400}"
RUN_SECONDS="${STM32N6_RUN_SECONDS:-3}"

ST_OOCD="${STM32N6_OPENOCD:-/opt/st/stm32cubeide_2.1.1/plugins/com.st.stm32cube.ide.mcu.externaltools.openocd.linux64_2.4.400.202601091506/tools/bin/openocd}"
ST_SCRIPTS="${STM32N6_OPENOCD_SCRIPTS:-/opt/st/stm32cubeide_2.1.1/plugins/com.st.stm32cube.ide.mcu.debug.openocd_2.3.300.202602021527/resources/openocd/st_scripts}"

PC_ADDR=$(printf "0x%x" $((VECTOR_BASE + 4)))

# TIM16 base (NS alias): 0x42004400 per the SVD; APB2ENR = 0x4602826c.
# NVIC ISER base = 0xE000E100, each register covers 32 IRQs.
# TIM16 IRQ number = 133 → ISER4 (IRQ 128..159), bit 5.

exec "$ST_OOCD" \
    -s "$ST_SCRIPTS" \
    -f interface/stlink-dap.cfg \
    -c "transport select dapdirect_swd" \
    -f target/stm32n6x.cfg \
    -c init \
    -c "reset halt" \
    -c "load_image $ELF" \
    -c "set sp [mrw $VECTOR_BASE]" \
    -c "set pc [mrw $PC_ADDR]" \
    -c "reg msp \$sp" \
    -c "resume \$pc" \
    -c "sleep [expr {$RUN_SECONDS * 1000}]" \
    -c "halt" \
    -c "echo {}" \
    -c "echo {=== probe state after N seconds of execution ===}" \
    -c "echo {RCC.APB2ENR @ 0x4602826c (TIM16EN = bit 17 = 0x20000):}" \
    -c "mdw 0x4602826c 1" \
    -c "echo {TIM16 @ 0x42004400 — CR1,CR2,SMCR,DIER,SR (5 words):}" \
    -c "mdw 0x42004400 5" \
    -c "echo {TIM16.CNT,PSC,ARR @ 0x42004424 (3 words):}" \
    -c "mdw 0x42004424 3" \
    -c "echo {SCB.VTOR @ 0xE000ED08 — should be 0x34180400:}" \
    -c "mdw 0xE000ED08 1" \
    -c "echo {NVIC.ISER4 @ 0xE000E110 — bit 5 = TIM16 IRQ 133 enable:}" \
    -c "mdw 0xE000E110 1" \
    -c "echo {NVIC.ISPR4 @ 0xE000E210 — bit 5 = TIM16 IRQ pending:}" \
    -c "mdw 0xE000E210 1" \
    -c "echo {NVIC.ITNS4 @ 0xE000E390 — bit 5 = TIM16 IRQ targets NS (1) vs S (0):}" \
    -c "mdw 0xE000E390 1" \
    -c "echo {NVIC.IPR33 @ 0xE000E484 — TIM16 IRQ 133 priority (byte 1 of word):}" \
    -c "mdw 0xE000E484 1" \
    -c "echo {SCB.ICSR @ 0xE000ED04 — active/pending:}" \
    -c "mdw 0xE000ED04 1" \
    -c "echo {SCB.AIRCR @ 0xE000ED0C — PRIS bit 14, BFHFNMINS bit 13, priority grouping bits 10:8:}" \
    -c "mdw 0xE000ED0C 1" \
    -c "echo {SCB.SCR @ 0xE000ED10 — SEVONPEND bit 4:}" \
    -c "mdw 0xE000ED10 1" \
    -c "echo {SCB.SHCSR @ 0xE000ED24:}" \
    -c "mdw 0xE000ED24 1" \
    -c "echo {SCB.CFSR/HFSR @ 0xE000ED28 (2 words):}" \
    -c "mdw 0xE000ED28 2" \
    -c "echo {NVIC.IABR4 @ 0xE000E310 — bit 5 = TIM16 IRQ currently active (in handler):}" \
    -c "mdw 0xE000E310 1" \
    -c "echo {MPU.CTRL @ 0xE000ED94 — bit 0 = enable:}" \
    -c "mdw 0xE000ED94 1" \
    -c "echo {SAU.CTRL @ 0xE000EDD0 — bit 0 = enable, bit 1 = ALLNS:}" \
    -c "mdw 0xE000EDD0 1" \
    -c "echo {SAU.TYPE @ 0xE000EDD4 — SREGION count:}" \
    -c "mdw 0xE000EDD4 1" \
    -c "echo {DSCSR @ 0xE000EE08 — bit 16 = CDS (1 = CPU in Secure state):}" \
    -c "mdw 0xE000EE08 1" \
    -c "echo {CCR @ 0xE000ED14:}" \
    -c "mdw 0xE000ED14 1" \
    -c "echo {RIFSC.RISC_SECCFGR0..5 @ 0x54024010 (Secure alias) — peripheral security (1 = Secure):}" \
    -c "mdw 0x54024010 6" \
    -c "echo {Same via NS alias 0x44024010 for comparison:}" \
    -c "mdw 0x44024010 6" \
    -c "echo {RIFSC.RISC_RCFGLOCKR0..5 @ 0x54024050 — lock bits (1 = frozen):}" \
    -c "mdw 0x54024050 6" \
    -c "echo {NS bank: NVIC_NS.ISER4 @ 0xE002E110 / ISPR4 @ 0xE002E210 / VTOR_NS @ 0xE002ED08:}" \
    -c "mdw 0xE002E110 1" \
    -c "mdw 0xE002E210 1" \
    -c "mdw 0xE002ED08 1" \
    -c "echo {DEMCR @ 0xE000ED30:}" \
    -c "mdw 0xE000ED30 1" \
    -c "echo {Write ISPR4 bit 8 and read back to verify write sticks:}" \
    -c "mww 0xE000E210 0x00000100" \
    -c "mdw 0xE000E210 1" \
    -c "echo {Vector table @ 0x34180400, first 16 entries (core exceptions):}" \
    -c "mdw 0x34180400 16" \
    -c "echo {TIM16 vector slot — 0x34180400 + 149*4 = 0x34180654 (should NOT match Default_Handler):}" \
    -c "mdw 0x34180654 1" \
    -c "echo {NMI_Handler vector — 0x34180408 (for comparison, expected weak alias):}" \
    -c "mdw 0x34180408 1" \
    -c "echo {HardFault_Handler vector — 0x3418040C:}" \
    -c "mdw 0x3418040C 1" \
    -c "echo {Current PC / PSR / MSP / PRIMASK / BASEPRI / FAULTMASK / CONTROL:}" \
    -c "reg pc" \
    -c "reg xpsr" \
    -c "reg msp" \
    -c "reg primask" \
    -c "reg basepri" \
    -c "reg faultmask" \
    -c "reg control" \
    -c shutdown
