//go:build ignore

void _start() {
    volatile int *result_ptr = (int *)0x80001000;

    // ANDI: 12 & 10 = 8
    result_ptr[0] = 12 & 10;

    // ORI: 12 | 10 = 14
    result_ptr[1] = 12 | 10;

    // XORI: 12 ^ 6 = 10
    result_ptr[2] = 12 ^ 6;

    // AND: 12 & 10 = 8
    int a = 12, b = 10;
    result_ptr[3] = a & b;

    // OR: 12 | 10 = 14
    result_ptr[4] = a | b;

    // XOR: 12 ^ 10 = 6
    result_ptr[5] = a ^ b;

    while(1) {}
}
