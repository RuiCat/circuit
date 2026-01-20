//go:build ignore

// These will be in .data
const float val1_f32 = 3.14f;
const float val2_f32 = 1.57f;
const int val3_int = 42;
const float val4_f32 = -3.14f;
const float val5_f32 = 0.0f;
const float val6_f32 = -0.0f;

// This will be in .bss if not initialized. The test finds it there.
volatile float result_area[20];

// Helper for fclass.s logic
int fclass_s(float f) {
    union { float f; int i; } u;
    u.f = f;
    int bits = u.i;
    int exponent = (bits >> 23) & 0xFF;
    int mantissa = bits & 0x7FFFFF;

    if (exponent == 0xFF) {
        if (mantissa == 0) { // Infinity
            return (bits >> 31) ? (1 << 0) : (1 << 7);
        } else { // NaN
            return (mantissa & (1 << 22)) ? (1 << 9) : (1 << 8); // Quiet or Signaling
        }
    } else if (exponent == 0) {
        if (mantissa == 0) { // Zero
            return (bits >> 31) ? (1 << 3) : (1 << 4);
        } else { // Subnormal
            return (bits >> 31) ? (1 << 2) : (1 << 5);
        }
    } else { // Normal
        return (bits >> 31) ? (1 << 1) : (1 << 6);
    }
}

void _start() {
    volatile int *int_result_ptr = (volatile int *)result_area;
    union { int i; float f; } u;

    // --- 1. Arithmetic ---
    result_area[0] = val1_f32 + val2_f32;    // FADD
    result_area[1] = val1_f32 - val2_f32;    // FSUB
    result_area[2] = val1_f32 * val2_f32;    // FMUL
    result_area[3] = val1_f32 / val2_f32;    // FDIV
    result_area[4] = __builtin_sqrtf(val1_f32); // FSQRT

    // --- 2. Conversion & Moves ---
    result_area[5] = (float)val3_int;      // FCVT.S.W
    int_result_ptr[6] = (int)val1_f32;      // FCVT.W.S

    // FMV.W.X: Store integer bits into float memory location, test reads it as int.
    int_result_ptr[7] = val3_int;

    // FMV.X.W: Store float bits into integer memory location.
    u.f = val1_f32;
    int_result_ptr[8] = u.i;

    // --- 3. Comparison ---
    int_result_ptr[9] = (val1_f32 == val2_f32); // FEQ (false)
    int_result_ptr[10] = (val1_f32 == val1_f32); // FEQ (true)

    // --- 4. New instructions ---
    result_area[11] = __builtin_fminf(val1_f32, val2_f32); // FMIN.S
    result_area[12] = __builtin_fmaxf(val1_f32, val4_f32); // FMAX.S
    result_area[13] = __builtin_fminf(val5_f32, val6_f32); // FMIN.S with +/- 0.0

    int_result_ptr[14] = fclass_s(val1_f32); // FCLASS.S (pos normal)
    int_result_ptr[15] = fclass_s(val6_f32); // FCLASS.S (neg zero)

    // FCVT.S.D / FCVT.D.S
    double temp_d = (double)val1_f32;
    result_area[16] = (float)temp_d;

    while(1) {}
}
