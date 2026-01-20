//go:build ignore

void _start() {
    volatile int *result_ptr = (int *)0x80001000;
    *result_ptr = 10 - 5;
    while(1) {}
}
