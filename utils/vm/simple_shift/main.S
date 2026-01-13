.section .text
.globl _start

_start:
    lui a3, %hi(0x80001000)
    addi a3, a3, %lo(0x80001000)

    # SLLI 测试: 0x123 << 4 = 0x1230
    li a0, 0x123
    slli a1, a0, 4
    sw a1, 0(a3)

    # SRLI 测试: 0x123 >> 4 = 0x12
    li a0, 0x123
    srli a1, a0, 4
    sw a1, 4(a3)

    # SRAI 测试: -20 >> 2 = -5 (0xFFFFFFEC >> 2 = 0xFFFFFFFB)
    li a0, -20
    srai a1, a0, 2
    sw a1, 8(a3)

    # SLL 测试: 0x123 << 5
    li a0, 0x123
    li a4, 5
    sll a1, a0, a4
    sw a1, 12(a3)

    # SRL 测试: 0x123 >> 5
    li a0, 0x123
    li a4, 5
    srl a1, a0, a4
    sw a1, 16(a3)

    # SRA 测试: -20 >> 3
    li a0, -20
    li a4, 3
    sra a1, a0, a4
    sw a1, 20(a3)

hang:
    j hang
