.section .text
.globl _start

_start:
    # 简单的加法: 5 + 10 = 15
    li  a0, 5
    li  a1, 10
    add a2, a0, a1

    # 将结果 (15) 存储在内存地址 0x80001000
    lui a3, %hi(0x80001000)      # 加载地址的高 20 位
    addi a3, a3, %lo(0x80001000) # 添加低 12 位
    sw  a2, 0(a3)               # 将字 a2 存储在 a3 中的地址

# 无限循环以使 VM 挂起，如测试所预期的那样
hang:
    j hang
