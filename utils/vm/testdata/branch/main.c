//go:build ignore

void _start() {
    volatile int *result_ptr = (int *)0x80001000;

    // --- BEQ a0, a1, target ---
    // Test 1: Branch taken (5 == 5)
    int a = 5, b = 5;
    if (a == b) {
        result_ptr[0] = 1;
    } else {
        result_ptr[0] = 0; // Should not happen
    }

    // Test 2: Branch not taken (5 != 10)
    a = 5; b = 10;
    if (a == b) {
        result_ptr[1] = 0; // Should not happen
    } else {
        result_ptr[1] = 2;
    }

    // --- BNE a0, a1, target ---
    // Test 3: Branch taken (5 != 10)
    a = 5; b = 10;
    if (a != b) {
        result_ptr[2] = 3;
    } else {
        result_ptr[2] = 0; // Should not happen
    }

    // Test 4: Branch not taken (5 == 5)
    a = 5; b = 5;
    if (a != b) {
        result_ptr[3] = 0; // Should not happen
    } else {
        result_ptr[3] = 4;
    }

    // --- BLT a0, a1, target ---
    // Test 5: Branch taken (-10 < 5)
    int signed_a = -10, signed_b = 5;
    if (signed_a < signed_b) {
        result_ptr[4] = 5;
    } else {
        result_ptr[4] = 0; // Should not happen
    }

    // Test 6: Branch not taken (10 < 5)
    signed_a = 10; signed_b = 5;
    if (signed_a < signed_b) {
        result_ptr[5] = 0; // Should not happen
    } else {
        result_ptr[5] = 6;
    }

    // --- BGE a0, a1, target ---
    // Test 7: Branch taken (10 >= 5)
    signed_a = 10; signed_b = 5;
    if (signed_a >= signed_b) {
        result_ptr[6] = 7;
    } else {
        result_ptr[6] = 0; // Should not happen
    }

    // Test 8: Branch not taken (-10 >= 5)
    signed_a = -10; signed_b = 5;
    if (signed_a >= signed_b) {
        result_ptr[7] = 0; // Should not happen
    } else {
        result_ptr[7] = 8;
    }

    // --- BLTU a0, a1, target ---
    // Test 9: Branch taken (10 < 20)
    unsigned int unsigned_a = 10, unsigned_b = 20;
    if (unsigned_a < unsigned_b) {
        result_ptr[8] = 9;
    } else {
        result_ptr[8] = 0; // Should not happen
    }

    // Test 10: Branch not taken (-1 (unsigned) > 20)
    // In C, -1 assigned to unsigned int becomes the largest unsigned value
    unsigned_a = -1; unsigned_b = 20;
    if (unsigned_a < unsigned_b) {
        result_ptr[9] = 0; // Should not happen
    } else {
        result_ptr[9] = 10;
    }

    // --- BGEU a0, a1, target ---
    // Test 11: Branch taken (-1 (unsigned) >= 20)
    unsigned_a = -1; unsigned_b = 20;
    if (unsigned_a >= unsigned_b) {
        result_ptr[10] = 11;
    } else {
        result_ptr[10] = 0; // Should not happen
    }

    // Test 12: Branch not taken (10 >= 20)
    unsigned_a = 10; unsigned_b = 20;
    if (unsigned_a >= unsigned_b) {
        result_ptr[11] = 0; // Should not happen
    } else {
        result_ptr[11] = 12;
    }

    while(1) {}
}
