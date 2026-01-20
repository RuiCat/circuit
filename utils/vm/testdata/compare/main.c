//go:build ignore

void _start() {
    volatile int *result_ptr = (int *)0x80001000;

    // SLTI: Set if less than immediate (signed)
    result_ptr[0] = (10 < 20) ? 1 : 0;  // 10 < 20 -> 1
    result_ptr[1] = (20 < 10) ? 1 : 0;  // 20 < 10 -> 0

    // SLTIU: Set if less than immediate (unsigned)
    result_ptr[2] = ((unsigned int)10 < (unsigned int)20) ? 1 : 0; // 10 < 20 -> 1
    result_ptr[3] = ((unsigned int)-1 < (unsigned int)10) ? 1 : 0; // 0xFFFFFFFF < 10 -> 0

    // SLT: Set if less than (signed)
    int a = 10, b = 20;
    result_ptr[4] = (a < b) ? 1 : 0; // 10 < 20 -> 1
    a = -10; b = -20;
    result_ptr[5] = (a < b) ? 1 : 0; // -10 < -20 -> 0

    // SLTU: Set if less than (unsigned)
    unsigned int ua = 10, ub = 20;
    result_ptr[6] = (ua < ub) ? 1 : 0; // 10 < 20 -> 1
    ua = -1; ub = 20;
    result_ptr[7] = (ua < ub) ? 1 : 0; // 0xFFFFFFFF < 20 -> 0

    while(1) {}
}
