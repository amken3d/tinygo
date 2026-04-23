#!/bin/bash
# Flash and run a TinyGo ELF on STM32N6 via ST's openocd.
#
# On STM32N6 the boot ROM runs before any application code and — if no
# signed FSBL image is present in external flash — enters a DFU wait loop
# that locks down AXISRAM against debug writes. Once that happens, probe-rs
# and STM32CubeProgrammer both fail to land our binary.
#
# The trick is to halt the CPU *at the reset vector* via the Cortex-M
# DEMCR.VC_CORERESET bit — no ROM instruction executes. ST's openocd N6
# script configures this correctly (two APs: mem_ap for ROM debug + CPU AP,
# with TrustZone-aware access mode detection). Once halted, we load our
# ELF directly into SRAM, read SP and Reset_Handler from our own vector
# table, set MSP, and resume. ROM stays dormant.

set -e

ELF="${1:?usage: $0 path/to/app.elf}"
VECTOR_BASE="${STM32N6_VECTOR:-0x34180400}"

# Paths to ST's bundled openocd and its chip scripts (from CubeIDE install).
# Override via env if the CubeIDE layout changes.
ST_OOCD="${STM32N6_OPENOCD:-/opt/st/stm32cubeide_2.1.1/plugins/com.st.stm32cube.ide.mcu.externaltools.openocd.linux64_2.4.400.202601091506/tools/bin/openocd}"
ST_SCRIPTS="${STM32N6_OPENOCD_SCRIPTS:-/opt/st/stm32cubeide_2.1.1/plugins/com.st.stm32cube.ide.mcu.debug.openocd_2.3.300.202602021527/resources/openocd/st_scripts}"

if [ ! -x "$ST_OOCD" ]; then
    echo "Error: ST openocd not found at $ST_OOCD" >&2
    echo "Set STM32N6_OPENOCD to the openocd binary path." >&2
    exit 1
fi
if [ ! -d "$ST_SCRIPTS" ]; then
    echo "Error: ST openocd scripts not found at $ST_SCRIPTS" >&2
    echo "Set STM32N6_OPENOCD_SCRIPTS to the st_scripts directory." >&2
    exit 1
fi

PC_ADDR=$(printf "0x%x" $((VECTOR_BASE + 4)))

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
    -c shutdown
