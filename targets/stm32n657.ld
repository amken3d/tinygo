
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

/* Absolute symbols used by stedgeai-generated network.c when an STM32N6
 * AI model is linked into the binary. These name the start of each
 * memory pool the AtoNN compiler may reference. They allocate no space —
 * they're just address constants the linker resolves for the network.c
 * `mpool_reloc_info[]` table. Harmless when no model is linked. */
PROVIDE(_mem_pool_AXIFLEXMEM_network = 0x34000000);
PROVIDE(_mem_pool_AXISRAM1_network   = 0x34080000);
PROVIDE(_mem_pool_AXISRAM2_network   = 0x3419C000);
PROVIDE(_mem_pool_AXISRAM3_network   = 0x34200000);
PROVIDE(_mem_pool_AXISRAM4_network   = 0x34270000);
PROVIDE(_mem_pool_AXISRAM5_network   = 0x342E0000);
PROVIDE(_mem_pool_AXISRAM6_network   = 0x34350000);
PROVIDE(_mem_pool_xSPI2_network      = 0x70380000);

INCLUDE "targets/arm.ld"
