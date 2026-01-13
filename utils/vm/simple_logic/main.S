.section .text
.globl _start

_start:
    # ANDI 测试: 0b1100 & 0b1010 = 0b1000 (12 & 10 = 8)
    li a0, 12
    andi a1, a0, 10
    lui a3, %hi(0x80001000)
    addi a3, a3, %lo(0x80001000)
    sw a1, 0(a3)

    # ORI 测试: 0b1100 | 0b1010 = 0b1110 (12 | 10 = 14)
    li a0, 12
    ori a1, a0, 10
    sw a1, 4(a3)

    # XORI 测试: 0b1100 ^ 0b1010 = 0b0110 (12 ^ 10 = 6)
    li a0, 12
    xori a1, a0, 6
    sw a1, 8(a3)

    # AND 测试: 0b1100 & 0b1010 = 0b1000 (12 & 10 = 8)
    li a0, 12
    li a4, 10
    and a1, a0, a4
    sw a1, 12(a3)

    # OR 测试: 0b1100 | 0b1010 = 0b1110 (12 | 10 = 14)
    li a0, 12
    li a4, 10
    or a1, a0, a4
    sw a1, 16(a3)

    # XOR 测试: 0b1100 ^ 0b1010 = 0b0110 (12 ^ 10 = 6)
    li a0, 12
    li a4, 10
    xor a1, a0, a4
    sw a1, 20(a3)

hang:
    j hang
