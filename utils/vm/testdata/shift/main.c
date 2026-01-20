//go:build ignore

#include <stdint.h>

void _start() {
    volatile int32_t *result_ptr = (int32_t *)0x80001000;

    // SLLI: Logical left shift immediate
    result_ptr[0] = (int32_t)(0x123 << 4);

    // SRLI: Logical right shift immediate
    result_ptr[1] = (int32_t)((uint32_t)0x123 >> 4);

    // SRAI: Arithmetic right shift immediate
    result_ptr[2] = (int32_t)(-20 >> 2);

    // SLL: Logical left shift
    int32_t a = 0x123;
    int32_t shift_amount = 5;
    result_ptr[3] = a << shift_amount;

    // SRL: Logical right shift
    result_ptr[4] = (int32_t)((uint32_t)a >> shift_amount);

    // SRA: Arithmetic right shift
    int32_t b = -20;
    shift_amount = 3;
    result_ptr[5] = b >> shift_amount;

    while(1) {}
}
