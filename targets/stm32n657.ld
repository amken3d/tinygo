
/*
 * STM32N657 has no user-accessible internal flash. The application runs
 * directly from SRAM, loaded via ST-LINK/SWD (STM32CubeProgrammer or
 * probe-rs). ROM bootloader scratch usage is confined to AXISRAM1 and the
 * bottom half of AXISRAM2, so everything goes into AXISRAM2 upper half —
 * the region ST's own FSBL reference uses.
 *
 *   FLASH_TEXT (rx)  : 0x34180400 .. 0x341BFFFF  (255 KB code/rodata)
 *   RAM        (xrw) : 0x341C0000 .. 0x341FFFFF  (256 KB data/bss/stack)
 *
 * The 1 KB gap at 0x34180000 .. 0x341803FF is ST's convention for a boot
 * header in signed-image flows; for dev-boot SWD loading it simply stays
 * empty.
 */

MEMORY
{
    FLASH_TEXT (rx) : ORIGIN = 0x34180400, LENGTH = 255K
    RAM (xrw)       : ORIGIN = 0x341C0000, LENGTH = 256K
}

_stack_size = 8K;

INCLUDE "targets/arm.ld"
