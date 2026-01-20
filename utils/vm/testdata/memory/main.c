//go:build ignore

#include <stdint.h>

void _start() {
    // Pointers to memory-mapped regions
    volatile uint8_t *scratchpad = (uint8_t *)0x80001000;
    volatile uint32_t *result_area = (uint32_t *)0x80001100;

    // --- Prepare and store initial data to scratchpad ---
    // Store word 0x5678ABCD
    *(volatile uint32_t *)(scratchpad + 0) = 0x5678ABCD;
    // Store half-word 0xDEFA
    *(volatile uint16_t *)(scratchpad + 4) = 0xDEFA;
    // Store byte 0x8A
    *(volatile uint8_t *)(scratchpad + 6) = 0x8A;


    // --- Load data from scratchpad and store to result area ---
    // LW: Load Word
    result_area[0] = *(volatile int32_t *)(scratchpad + 0);

    // LH: Load Half-word (signed)
    result_area[1] = *(volatile int16_t *)(scratchpad + 4);

    // LHU: Load Half-word (unsigned)
    result_area[2] = *(volatile uint16_t *)(scratchpad + 4);

    // LB: Load Byte (signed)
    result_area[3] = *(volatile int8_t *)(scratchpad + 6);

    // LBU: Load Byte (unsigned)
    result_area[4] = *(volatile uint8_t *)(scratchpad + 6);


    // --- Test store instructions into the result area ---
    volatile uint8_t *result_bytes = (uint8_t *)result_area;

    // SB: Store Byte 0x8A
    *(result_bytes + 20) = (int8_t)result_area[3]; // Use the value from LB

    // SH: Store Half-word 0xDEFA
    *(volatile uint16_t *)(result_bytes + 22) = (int16_t)result_area[1]; // Use the value from LH

    // SW: Store Word 0x5678ABCD
    *(volatile uint32_t *)(result_bytes + 24) = result_area[0]; // Use the value from LW

    while(1) {}
}
