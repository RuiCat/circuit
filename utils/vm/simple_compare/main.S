.section .text
.globl _start

_start:
    lui a3, %hi(0x80001000)
    addi a3, a3, %lo(0x80001000)

    # SLTI 测试: 10 < 20 -> 1
    li a0, 10
    slti a1, a0, 20
    sw a1, 0(a3)

    # SLTI 测试: 20 < 10 -> 0
    li a0, 20
    slti a1, a0, 10
    sw a1, 4(a3)

    # SLTIU 测试: 10 < 20 -> 1
    li a0, 10
    sltiu a1, a0, 20
    sw a1, 8(a3)

    # SLTIU 测试: -1 (0xFFFFFFFF) < 10 -> 0 (unsigned)
    li a0, -1
    sltiu a1, a0, 10
    sw a1, 12(a3)

    # SLT 测试: 10 < 20 -> 1
    li a0, 10
    li a4, 20
    slt a1, a0, a4
    sw a1, 16(a3)

    # SLT 测试: -10 < -20 -> 0
    li a0, -10
    li a4, -20
    slt a1, a0, a4
    sw a1, 20(a3)

    # SLTU 测试: 10 < 20 -> 1
    li a0, 10
    li a4, 20
    sltu a1, a0, a4
    sw a1, 24(a3)

    # SLTU 测试: -1 (unsigned) < 20 (unsigned) -> 0
    li a0, -1
    li a4, 20
    sltu a1, a0, a4
    sw a1, 28(a3)


hang:
    j hang
